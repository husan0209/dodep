package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/opus-casino/rg/internal/config"
	"github.com/opus-casino/rg/internal/handlers"
	"github.com/opus-casino/rg/internal/repository"
	"github.com/opus-casino/rg/internal/service"
)

func main() {
	// 1. Config (env-based; validated into regulatory ranges)
	cfg := config.Load().Validate()

	// 2. Logger
	var log *zap.Logger
	if cfg.Env == "development" {
		log, _ = zap.NewDevelopment()
	} else {
		log, _ = zap.NewProduction()
	}
	defer func() { _ = log.Sync() }()

	// 3. Database (GORM + pgx driver, per CONVENTIONS)
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatal("rg: failed to connect to database", zap.Error(err))
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("rg: failed to get sql.DB", zap.Error(err))
	}
	sqlDB.SetMaxOpenConns(30)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	if err := sqlDB.Ping(); err != nil {
		log.Fatal("rg: database ping failed", zap.Error(err))
	}
	log.Info("rg: database connected")

	// 4. Auto-migrate service-owned tables (CREATE TABLE / ADD COLUMN only)
	if err := repository.AutoMigrate(db); err != nil {
		log.Fatal("rg: migration failed", zap.Error(err))
	}

	// 5. Repository -> Service
	repo := repository.NewGormRepository(db)
	publisher := service.NewLogPublisher(log)
	rgService := service.NewRGService(repo, publisher, log, cfg.CoolingHours, cfg.RevokeCoolingHours)
	grpcHandler := handlers.NewRGGrpcHandler(rgService, log)
	_ = grpcHandler
	httpHandler := handlers.New(rgService)

	// 6. gRPC server (health + reflection; full RGService bindings land
	// with `buf generate` by PROTOBUF_CONTRACTS — see grpc_handler.go)
	grpcServer := grpc.NewServer()
	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthSrv)
	healthSrv.SetServingStatus("rg-service", healthpb.HealthCheckResponse_SERVING)
	reflection.Register(grpcServer)

	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
		if err != nil {
			log.Fatal("rg: failed to listen gRPC", zap.Error(err))
		}
		log.Info("rg: gRPC server started", zap.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("rg: gRPC error", zap.Error(err))
		}
	}()

	// 7. Fiber HTTP server
	app := fiber.New(fiber.Config{
		AppName:      "RG Service v1.0.0",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	})
	app.Use(RequestIDMiddleware())
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:3000, http://127.0.0.1:3000",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Request-ID, X-Admin-ID",
		AllowCredentials: true,
	}))

	// 8. Health endpoints (with DB check on readiness)
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "rg"})
	})
	app.Get("/ready", func(c *fiber.Ctx) error {
		if err := sqlDB.Ping(); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "not ready"})
		}
		return c.JSON(fiber.Map{"status": "ready", "service": "rg"})
	})

	// 9. Routes: user_id from JWT (AuthMiddleware), operators via AdminMiddleware
	setupRoutes(app, httpHandler, rgService, AuthMiddleware(cfg.JWTSecretKey, cfg.Env))
	setupAdminRoutes(app, httpHandler, rgService, AdminMiddleware(cfg.AdminToken))

	go func() {
		log.Info("rg: HTTP server started", zap.String("port", cfg.HTTPPort))
		if err := app.Listen(":" + cfg.HTTPPort); err != nil {
			log.Error("rg: HTTP error", zap.Error(err))
		}
	}()

	// 10. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("rg: shutting down...")
	grpcServer.GracefulStop()
	_ = app.ShutdownWithTimeout(10 * time.Second)
	log.Info("rg: shutdown complete")
}
