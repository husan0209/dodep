package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/cors"
	fiberrecover "github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/opus-casino/bonus/internal/client"
	"github.com/opus-casino/bonus/internal/config"
	"github.com/opus-casino/bonus/internal/consumer"
	"github.com/opus-casino/bonus/internal/domain"
	"github.com/opus-casino/bonus/internal/handlers"
	"github.com/opus-casino/bonus/internal/ratelimit"
	"github.com/opus-casino/bonus/internal/repository"
	"github.com/opus-casino/bonus/internal/service"
	"github.com/opus-casino/bonus/internal/telemetry"
	pb "github.com/opus-casino/proto/gen/go/bonus/v1"
)

// version is injected at build time (-ldflags "-X main.version=...").
var version = "dev"

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

	// ── Tracing (opt-in: no OTEL_EXPORTER_OTLP_ENDPOINT → no-op tracer) ──
	// Initialised before any other component so DB/Kafka/HTTP/gRPC spans all
	// share one provider. The telemetry package owns its own env contract.
	traceCfg := telemetry.TracingConfigFromEnv("bonus-service")
	traceCfg.Environment = cfg.Env
	traceCfg.Version = version
	shutdownTracing, err := telemetry.InitTracing(context.Background(), traceCfg)
	if err != nil {
		// A broken collector endpoint must not take the service down in
		// development; in production it is a configuration bug worth failing
		// fast on.
		if cfg.Env == "production" {
			log.Fatal("Bonus: tracing initialisation failed", zap.Error(err))
		}
		log.Error("Bonus: tracing disabled after init failure", zap.Error(err))
	}
	log.Info("Bonus: tracing configured",
		zap.String("service", traceCfg.ServiceName),
		zap.Bool("enabled", traceCfg.Endpoint != ""),
		zap.Float64("sample_ratio", traceCfg.SampleRatio))
	defer func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracing(flushCtx); err != nil {
			log.Error("Bonus: tracing shutdown", zap.Error(err))
		}
	}()

	// ── Database (GORM + pgx driver) ──────────────────────────────────────
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatal("Bonus: failed to connect to database", zap.Error(err))
	}
	if err := db.AutoMigrate(&domain.Bonus{}); err != nil {
		log.Fatal("Bonus: migration failed", zap.Error(err))
	}

	// ── Telemetry: Prometheus registry + collectors ───────────────────────
	// A dedicated registry (not the global default) keeps the exposition
	// explicit and lets tests build isolated registries.
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	metrics := telemetry.New(registry)

	// ── Layers: repository → service ──────────────────────────────────────
	repo := repository.NewBonusRepository(db)
	bonusSvc := service.NewBonusServiceWithRepository(repo, service.BonusConfig{
		WelcomePct:        cfg.WelcomePct,
		WelcomeMaxUSD:     cfg.WelcomeMaxUSD,
		WelcomeWagering:   cfg.WelcomeWagering,
		WelcomeExpiryDays: cfg.WelcomeExpiryDays,
	}, log)
	bonusSvc.SetMetrics(metrics)
	log.Info("Bonus service initialized",
		zap.Int("welcome_pct", cfg.WelcomePct),
		zap.String("welcome_max_usd", cfg.WelcomeMaxUSD.String()),
		zap.Int("welcome_wagering", cfg.WelcomeWagering),
		zap.Int("welcome_expiry_days", cfg.WelcomeExpiryDays),
		zap.String("wallet_grpc_addr", cfg.WalletGRPCAddr))

	// ── Wallet client (wagering conversion credits, idempotent) ──────────
	walletClient, err := client.NewWalletClient(client.WalletClientConfig{
		Address: cfg.WalletGRPCAddr,
	}, log)
	if err != nil {
		log.Fatal("Bonus: failed to create wallet client", zap.Error(err))
	}
	// Deferred close: the gRPC connection must be released on shutdown,
	// otherwise the process lingers with an open client connection.
	defer func() {
		if err := walletClient.Close(); err != nil {
			log.Error("Bonus: wallet client close", zap.Error(err))
		}
	}()
	bonusSvc.SetWalletCrediter(walletClient)

	// ── Redpanda consumer (first-deposit → welcome bonus) ─────────────────
	payConsumer, err := consumer.NewPaymentConsumer(cfg.KafkaBrokers, bonusSvc, metrics, log)
	if err != nil {
		log.Fatal("Bonus: failed to create payment consumer", zap.Error(err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		if err := payConsumer.Start(ctx); err != nil && err != context.Canceled {
			log.Error("Bonus: consumer stopped", zap.Error(err))
		}
	}()

	// ── gRPC server ───────────────────────────────────────────────────────
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
		telemetry.UnaryRecoveryInterceptor(log),
		telemetry.UnaryTracingInterceptor(),
		telemetry.UnaryLoggingInterceptor(log),
		telemetry.UnaryMetricsInterceptor(metrics),
	))
	pb.RegisterBonusServiceServer(grpcServer, handlers.NewBonusGRPCHandler(bonusSvc, log))
	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthSrv)
	healthSrv.SetServingStatus("bonus-service", healthpb.HealthCheckResponse_SERVING)
	if cfg.Env != "production" {
		reflection.Register(grpcServer)
	}

	go func() {
		lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
		if err != nil {
			log.Fatal("Bonus: failed to listen gRPC", zap.Error(err))
		}
		log.Info("Bonus: gRPC server started", zap.String("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("Bonus: gRPC error", zap.Error(err))
		}
	}()

	// ── HTTP server (Fiber) ───────────────────────────────────────────────
	// Rate limits: generous reads, tight writes. The write budget protects the
	// money-adjacent endpoints (activate/cancel) from scripted loops.
	readLimiter := ratelimit.New(ratelimit.Config{Name: "read", Limit: 300, Window: time.Minute})
	writeLimiter := ratelimit.New(ratelimit.Config{Name: "write", Limit: 30, Window: time.Minute})
	defer readLimiter.Close()
	defer writeLimiter.Close()
	log.Info("Bonus: rate limits configured",
		zap.String("read_per_minute", "300"),
		zap.String("write_per_minute", "30"))

	app := fiber.New(fiber.Config{
		AppName:      "Bonus Service v1.0.0",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		// The API accepts tiny JSON documents only; a 1 MiB cap rejects
		// oversized bodies with 413 before any parsing or allocation.
		BodyLimit: 1024 * 1024,
		// Keep the listener from idling forever on keep-alive connections.
		IdleTimeout: 60 * time.Second,
	})
	// Order matters: request_id first (so every log line can be correlated),
	// then tracing (extracts upstream traceparent), then metrics, then recover.
	app.Use(telemetry.RequestIDMiddleware())
	app.Use(telemetry.TracingMiddleware())
	// Metrics middleware goes early so even auth-rejected and rate-limited
	// requests are observed (RED: rate, errors, duration).
	app.Use(telemetry.HTTPMiddleware(metrics))
	app.Use(fiberrecover.New())
	app.Use(SecurityHeaders())
	allowedOrigins := corsOriginsFromEnv()
	app.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Request-ID, X-Idempotency-Key",
		AllowCredentials: allowedOrigins != "",
	}))

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "bonus"})
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

	// Prometheus exposition. Served on the service HTTP port and left
	// unauthenticated on purpose: it is network-restricted by the Helm
	// NetworkPolicy (monitoring namespace only) and carries no PII, only
	// aggregate counters. Serving it here (rather than a side port) keeps
	// container/service port lists minimal.
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		Registry:          registry,
		EnableOpenMetrics: false,
	})))

	// Authenticated user API: user_id always comes from JWT (NEVER from body).
	bonusAPI := app.Group("/api/v1/bonuses",
		AuthMiddleware(cfg.JWTSecretKey),
		// Per-user abuse control on the partner API. Per-pod state by design:
		// global quotas live at the edge/mesh (see internal/ratelimit docs).
		readLimiter.Middleware(ratelimit.ByUserOrIP),
	)
	setupRoutesOnGroup(bonusAPI, bonusSvc, writeLimiter)

	go func() {
		log.Info("Bonus: HTTP server started", zap.String("port", cfg.HTTPPort))
		if err := app.Listen(":" + cfg.HTTPPort); err != nil {
			log.Error("Bonus: HTTP error", zap.Error(err))
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────
	// Order: stop accepting new work → drain in-flight requests → close
	// outbound clients → flush telemetry. Leaking the wallet-core gRPC
	// connection (or flushing spans after the servers died) both lose data.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Bonus: shutting down...")
	// Flip gRPC health to NOT_SERVING first: mesh clients (and the readiness
	// gate of callers) must stop sending work before the listeners close.
	healthSrv.Shutdown()
	cancel() // stop the Kafka consumer loop
	_ = app.ShutdownWithTimeout(10 * time.Second)
	grpcServer.GracefulStop()
	log.Info("Bonus: shutdown complete")
}

// runHealthProbe GETs the local /health endpoint for Docker HEALTHCHECK.
// It returns 0 when the service answers 200 OK, 1 otherwise.
func runHealthProbe() int {
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8088"
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/health", port))
	if err != nil {
		return 1
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

// corsOriginsFromEnv reads $CORS_ORIGINS allowlist; "*" is stripped (never allow wildcard).
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
