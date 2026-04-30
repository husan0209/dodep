package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/opus-casino/casino/internal/client"
	"github.com/opus-casino/casino/internal/config"
	"github.com/opus-casino/casino/internal/handlers"
	"github.com/opus-casino/casino/internal/provider"
	"github.com/opus-casino/casino/internal/provider/amatic"
	"github.com/opus-casino/casino/internal/provider/amusnet"
	pgsoft_provider "github.com/opus-casino/casino/internal/provider/pgsoft"
	"github.com/opus-casino/casino/internal/provider/pragmatic"
	"github.com/opus-casino/casino/internal/repository"
	"github.com/opus-casino/casino/internal/service"
)

func main() {
	cfg := config.Load()

	log, _ := zap.NewProduction()
	if cfg.Env == "development" {
		log, _ = zap.NewDevelopment()
	}
	defer log.Sync()

	// ── Database (GORM + pgx) ──────────────────────────────────────────────
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}

	// ── DragonflyDB / Redis ───────────────────────────────────────────────
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer rdb.Close()

	// ── Repository ────────────────────────────────────────────────────────
	casinoRepo := repository.NewCasinoRepository(db, rdb)

	// ── Provider Registry ─────────────────────────────────────────────────
	registry := provider.NewRegistry()

	if cfg.Pragmatic.Enabled {
		registry.Register(pragmatic.New(cfg.Pragmatic, log))
		log.Info("Casino: Pragmatic Play enabled")
	}
	if cfg.PGSoft.Enabled {
		registry.Register(pgsoft_provider.New(cfg.PGSoft, log))
		log.Info("Casino: PG Soft enabled")
	}
	if cfg.Amatic.Enabled {
		registry.Register(amatic.New(cfg.Amatic, log))
		log.Info("Casino: Amatic enabled")
	}
	if cfg.Amusnet.Enabled {
		amusnetAdapter, err := amusnet.New(cfg.Amusnet, log)
		if err != nil {
			log.Fatal("Casino: failed to init Amusnet adapter", zap.Error(err))
		}
		registry.Register(amusnetAdapter)
		log.Info("Casino: Amusnet enabled")
	}

	// ── gRPC Clients ──────────────────────────────────────────────────────
	walletClient, err := client.NewWalletClient(client.WalletClientConfig{
		Address: cfg.WalletGRPCAddr,
		Timeout: 5 * time.Second,
	}, log)
	if err != nil {
		log.Fatal("Casino: failed to connect to wallet-core", zap.Error(err))
	}

	userClient, err := client.NewUserClient(client.UserClientConfig{
		Address: cfg.UserGRPCAddr,
		Timeout: 5 * time.Second,
	}, log)
	if err != nil {
		log.Fatal("Casino: failed to connect to user service", zap.Error(err))
	}

	// ── Services ──────────────────────────────────────────────────────────
	casinoSvc := service.NewCasinoService(casinoRepo, registry, walletClient, userClient, log)

	integrationSvc := service.NewIntegrationService(
		registry,
		walletClient,
		repository.NewPlayerMapper(db),
		rdb,
		log,
	)

	// ── gRPC Server ───────────────────────────────────────────────────────
	grpcServer := grpc.NewServer()

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthSrv)
	healthSrv.SetServingStatus("casino-service", healthpb.HealthCheckResponse_SERVING)

	grpcHandler := handlers.NewCasinoGRPCHandler(casinoSvc)
	// pb.RegisterCasinoServiceServer(grpcServer, grpcHandler)  // Enable once proto codegen is wired
	_ = grpcHandler
	reflection.Register(grpcServer)

	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
		if err != nil {
			log.Fatal("Casino: failed to listen gRPC", zap.Error(err))
		}
		log.Info("Casino: gRPC server started", zap.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("Casino: gRPC server error", zap.Error(err))
		}
	}()

	// ── Fiber HTTP Server ──────────────────────────────────────────────────
	app := fiber.New(fiber.Config{
		AppName:       "Casino Service v1.0.0",
		ReadTimeout:   10 * time.Second,
		WriteTimeout:  10 * time.Second,
		IdleTimeout:   60 * time.Second,
		ErrorHandler:  jsonErrorHandler,
	})

	app.Use(recover.New())
	app.Use(fiberlogger.New())

	// Health checks
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "casino"})
	})
	app.Get("/ready", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ready"})
	})

	// Prometheus metrics (served on separate port 9186 to avoid Fiber type conflicts)
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		if err := http.ListenAndServe(":9186", mux); err != nil {
			log.Error("Casino: metrics server error", zap.Error(err))
		}
	}()

	// Casino REST API (authenticated routes)
	api := app.Group("/api/v1")
	casinoRoutes := api.Group("/casino")
	casinoRoutes.Get("/games", handlers.NewCasinoHTTPHandler(casinoSvc, log).GetGames)
	casinoRoutes.Get("/games/:id", handlers.NewCasinoHTTPHandler(casinoSvc, log).GetGame)
	casinoRoutes.Post("/games/launch", handlers.NewCasinoHTTPHandler(casinoSvc, log).LaunchGame)
	casinoRoutes.Get("/providers", handlers.NewCasinoHTTPHandler(casinoSvc, log).GetProviders)
	casinoRoutes.Get("/sessions/:id", handlers.NewCasinoHTTPHandler(casinoSvc, log).GetSession)
	casinoRoutes.Post("/sessions/:id/end", handlers.NewCasinoHTTPHandler(casinoSvc, log).EndSession)
	casinoRoutes.Get("/history", handlers.NewCasinoHTTPHandler(casinoSvc, log).GetHistory)

	// Provider callback routes (no JWT — secured by nginx IP allowlist + HMAC)
	providerHandler := handlers.NewProviderHandler(integrationSvc, log)
	providerHandler.RegisterRoutes(app)

	go func() {
		log.Info("Casino: HTTP server started", zap.String("port", cfg.HTTPPort))
		if err := app.Listen(":" + cfg.HTTPPort); err != nil {
			log.Error("Casino: HTTP server error", zap.Error(err))
		}
	}()

	// ── Graceful shutdown ──────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Casino: shutting down...")
	grpcServer.GracefulStop()
	_ = app.ShutdownWithTimeout(10 * time.Second)
	log.Info("Casino: shutdown complete")
}

func jsonErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}
	return c.Status(code).JSON(fiber.Map{
		"error":   1,
		"message": err.Error(),
	})
}
