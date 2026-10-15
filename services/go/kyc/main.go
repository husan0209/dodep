package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	fiberrecover "github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	pb "github.com/opus-casino/proto/gen/go/kyc/v1"

	"github.com/opus-casino/kyc/internal/cache"
	"github.com/opus-casino/kyc/internal/config"
	"github.com/opus-casino/kyc/internal/crypto"
	"github.com/opus-casino/kyc/internal/handlers"
	"github.com/opus-casino/kyc/internal/ratelimit"
	"github.com/opus-casino/kyc/internal/repository"
	"github.com/opus-casino/kyc/internal/screening"
	"github.com/opus-casino/kyc/internal/service"
)

// version is injected at build time (-ldflags "-X main.version=...").
var version = "dev"

// defaultHTTPPort is the CONVENTIONS.md port for kyc and the fallback when
// PORT is unset. Named so the health probe and the listener cannot drift.
const defaultHTTPPort = "8089"

// healthProbeHost is the loopback address the container healthcheck uses.
// A constant, never an env value: the probe must not be redirectable.
const healthProbeHost = "127.0.0.1"

func main() {
	// `app health` is the Docker HEALTHCHECK contract (see infra/docker/Dockerfile.go):
	// probe the local HTTP endpoint and exit 0/1 without booting the service.
	if len(os.Args) > 1 && os.Args[1] == "health" {
		os.Exit(runHealthProbe())
	}

	cfg := config.Load()

	var log *zap.Logger
	if cfg.Env == "development" {
		log, _ = zap.NewDevelopment()
	} else {
		log, _ = zap.NewProduction()
	}
	defer func() { _ = log.Sync() }()

	if err := cfg.Validate(); err != nil {
		log.Fatal("KYC: invalid configuration", zap.Error(err))
	}

	// ── Field encryption (PII at rest) ────────────────────────────────────
	var crypt *crypto.Service
	if key, err := crypto.ParseKey(cfg.FieldKey); err == nil {
		crypt, err = crypto.New(key)
		if err != nil {
			log.Fatal("KYC: field encryption init failed", zap.Error(err))
		}
	} else if cfg.Env == "development" {
		crypt, _ = crypto.Ephemeral()
		log.Warn("KYC: KYC_FIELD_KEY unset — ephemeral field key in use; ciphertexts will not survive restarts")
	} else {
		log.Fatal("KYC: KYC_FIELD_KEY is required outside development", zap.Error(err))
	}

	// ── Database (GORM + pgx driver via postgres dialect) ─────────────────
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatal("KYC: failed to connect to database", zap.Error(err))
	}
	if err := repository.AutoMigrate(db); err != nil {
		log.Fatal("KYC: migration failed", zap.Error(err))
	}

	// ── Cache (DragonflyDB/Redis; memory fallback for local dev) ───────────
	var statusCache cache.StatusCache
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, DB: cfg.RedisDB})
	redisCache := cache.NewRedis(rdb)
	if err := redisCache.Ping(context.Background()); err != nil {
		_ = rdb.Close()
		statusCache = cache.NewMemory()
		log.Warn("KYC: redis unreachable — in-memory status cache in use",
			zap.String("addr", cfg.RedisAddr), zap.Error(err))
	} else {
		statusCache = redisCache
		log.Info("KYC: status cache connected", zap.String("addr", cfg.RedisAddr))
	}
	defer func() { _ = statusCache.Close() }()

	// ── Layers: repository → service ──────────────────────────────────────
	repo := repository.NewKYCRepository(db)
	screener := screening.SelectProvider(cfg.ComplyAdvantageURL, cfg.ComplyAdvantageAPIKey)
	kycSvc := service.NewKYCService(repo, statusCache, screener, crypt, service.Config{
		WebhookSecret: cfg.SumsubWebhookSecret,
	}, log)
	log.Info("KYC service initialized",
		zap.String("version", version),
		zap.String("screening_provider", screener.Name()),
		zap.Bool("webhook_configured", cfg.SumsubWebhookSecret != ""))

	h := handlers.NewHandler(kycSvc, log)

	// ── gRPC server ───────────────────────────────────────────────────────
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcRecoveryInterceptor(log),
			grpcLoggingInterceptor(log),
		),
	)
	pb.RegisterKYCServiceServer(grpcServer, handlers.NewKYCGRPCHandler(kycSvc, log))
	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthSrv)
	healthSrv.SetServingStatus("kyc-service", healthpb.HealthCheckResponse_SERVING)
	if cfg.Env != "production" {
		reflection.Register(grpcServer)
	}

	go func() {
		lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
		if err != nil {
			log.Fatal("KYC: failed to listen gRPC", zap.Error(err))
		}
		log.Info("KYC: gRPC server started", zap.String("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("KYC: gRPC error", zap.Error(err))
		}
	}()

	// ── HTTP server (Fiber) ───────────────────────────────────────────────
	readLimiter := ratelimit.New(ratelimit.Config{Name: "read", Limit: 300, Window: time.Minute})
	writeLimiter := ratelimit.New(ratelimit.Config{Name: "write", Limit: 30, Window: time.Minute})
	webhookLimiter := ratelimit.New(ratelimit.Config{Name: "webhook", Limit: 60, Window: time.Minute})
	defer readLimiter.Close()
	defer writeLimiter.Close()
	defer webhookLimiter.Close()

	app := fiber.New(fiber.Config{
		AppName:      "KYC Service v1.0.0",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		BodyLimit:    1024 * 1024,
		IdleTimeout:  60 * time.Second,
	})
	app.Use(requestIDMiddleware())
	app.Use(fiberrecover.New())
	app.Use(SecurityHeaders())
	allowedOrigins := corsOriginsFromEnv()
	app.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Request-ID, X-Idempotency-Key, X-Webhook-Signature",
		AllowCredentials: allowedOrigins != "",
	}))

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "kyc"})
	})
	app.Get("/ready", func(c *fiber.Ctx) error {
		sqlDB, err := db.DB()
		if err != nil {
			return c.Status(503).JSON(fiber.Map{"status": "not ready", "reason": "db handle"})
		}
		if err := sqlDB.PingContext(c.Context()); err != nil {
			return c.Status(503).JSON(fiber.Map{"status": "not ready", "reason": "db ping"})
		}
		return c.JSON(fiber.Map{"status": "ready"})
	})

	// Authenticated user + admin API: user_id always comes from JWT.
	kycAPI := app.Group("/api/v1/kyc",
		AuthMiddleware(cfg.JWTSecretKey),
		readLimiter.Middleware(ratelimit.ByUserOrIP),
	)
	setupRoutesOnGroup(kycAPI, h, writeLimiter)
	setupWebhookRoutes(app, h, webhookLimiter)

	go func() {
		log.Info("KYC: HTTP server started", zap.String("port", cfg.HTTPPort))
		if err := app.Listen(":" + cfg.HTTPPort); err != nil {
			log.Error("KYC: HTTP error", zap.Error(err))
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("KYC: shutting down...")
	healthSrv.Shutdown()
	_ = app.ShutdownWithTimeout(10 * time.Second)
	grpcServer.GracefulStop()
	log.Info("KYC: shutdown complete")
}

// requestIDMiddleware propagates or generates X-Request-ID for log correlation.
func requestIDMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		rid := c.Get("X-Request-ID")
		if rid == "" {
			rid = uuid.NewString()
		} else if len(rid) > 128 {
			rid = rid[:128]
		}
		c.Set("X-Request-ID", rid)
		c.Response().Header.Set("X-Request-ID", rid)
		return c.Next()
	}
}

// runHealthProbe GETs the local /health endpoint for Docker HEALTHCHECK.
func runHealthProbe() int {
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = defaultHTTPPort
	}
	portNum, err := strconv.Atoi(port)
	if err != nil || portNum < 1 || portNum > 65535 {
		return 1
	}
	client := &http.Client{Timeout: 5 * time.Second}
	//nolint:gosec // G704: host is a constant; port validated to [1,65535] above.
	probeURL := fmt.Sprintf("http://%s:%d/health", healthProbeHost, portNum)
	// #nosec G704 -- validated port, loopback-only target.
	resp, err := client.Get(probeURL)
	if err != nil {
		return 1
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

// corsOriginsFromEnv reads $CORS_ORIGINS allowlist; "*" is stripped.
func corsOriginsFromEnv() string {
	raw := strings.TrimSpace(os.Getenv("CORS_ORIGINS"))
	if raw == "" {
		return "http://localhost:3000"
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" && t != "*" {
			out = append(out, t)
		}
	}
	return strings.Join(out, ", ")
}
