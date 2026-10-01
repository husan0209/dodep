package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/opus-casino/affiliate/internal/config"
	"github.com/opus-casino/affiliate/internal/consumer"
	"github.com/opus-casino/affiliate/internal/event"
	grpcserver "github.com/opus-casino/affiliate/internal/grpc"
	"github.com/opus-casino/affiliate/internal/repository"
	"github.com/opus-casino/affiliate/internal/service"
	pb "github.com/opus-casino/proto/gen/go/affiliate/v1"
)

// corsOriginsFromEnv reads $CORS_ORIGINS (comma-separated) and trims spaces.
// Falls back to http://localhost:3000 when unset so local dev works out of the
// box. Production must always set CORS_ORIGINS explicitly.
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

func main() {
	// 1. Load configuration
	cfg := config.Load()

	// 2. Initialize logger
	log, _ := zap.NewProduction()
	defer log.Sync()

	// 3. Initialize database (GORM + pgx driver, per CONVENTIONS)
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	log.Info("Database connected")

	// 4. Initialize repository → service
	repo := repository.NewGormAffiliateRepository(db, log)
	affiliateService := service.NewAffiliateService(repo, log)

	// Outbox delivery: real Redpanda producer when REDPANDA_BROKERS is set,
	// LogPublisher fallback for local dev without a broker.
	var publisher event.Publisher = event.NewLogPublisher(log)
	var closePublisher func()
	if brokers := splitBrokers(cfg.RedpandaBrokers); len(brokers) > 0 {
		rp, err := event.NewRedpandaPublisher(
			context.Background(),
			event.RedpandaPublisherConfig{Brokers: brokers},
			log,
		)
		if err != nil {
			log.Fatal("Failed to connect to Redpanda", zap.Error(err))
		}
		publisher = rp
		closePublisher = rp.Close
		log.Info("Outbox delivery via Redpanda", zap.Strings("brokers", brokers))
	} else {
		log.Info("REDPANDA_BROKERS unset: outbox delivery via LogPublisher")
	}
	outboxWorker := event.NewOutboxWorker(repo, publisher, log, cfg.OutboxPollInterval, cfg.OutboxBatchSize)

	// 4a. Initialize gRPC handler
	grpcHandler := grpcserver.NewAffiliateGRPCHandler(affiliateService, log)

	// 5. Initialize Fiber HTTP server
	app := fiber.New(fiber.Config{
		AppName: "Affiliate Service v1.0.0",
	})

	app.Use(recover.New())
	app.Use(logger.New())
	// CORS: explicit allowlist from $CORS_ORIGINS (comma-separated).
	// Wildcard "*" was a security misconfiguration (Phase 0.4) and is
	// stripped by corsOriginsFromEnv. Empty list = deny browser fetch
	// preflight (redirect endpoints under /r/* are unaffected — they are
	// browser-navigation, not subject to CORS).
	allowedOrigins := corsOriginsFromEnv()
	app.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Request-ID, X-Idempotency-Key",
		AllowCredentials: allowedOrigins != "",
	}))

	// 6. Health endpoints
	app.Get("/health", healthHandler)
	app.Get("/ready", readyHandler)

	// 7. Public routes (click tracking — no auth required)
	app.Get("/r/:affiliate_code", trackClickHandler(affiliateService))
	app.Get("/r/:affiliate_code/:campaign", trackClickHandler(affiliateService))

	// 8. Protected partner cabinet routes (JWT required)
	protected := app.Group("/api/v1/affiliate", AuthMiddleware(cfg.JWTSecretKey))
	setupRoutes(protected, affiliateService)

	// 9. Admin routes (Admin JWT required)
	adminGroup := app.Group("/admin", AdminMiddleware(cfg.JWTSecretKey))
	setupAdminRoutes(adminGroup, affiliateService)

	// 10. Start background workers
	workerCtx, cancelWorkers := context.WithCancel(context.Background())
	defer cancelWorkers()
	go outboxWorker.Run(workerCtx)

	// 10a. Start NGR consumer (if Redpanda is configured)
	if cfg.RedpandaBrokers != "" {
		go startNGRConsumer(workerCtx, cfg, affiliateService, log)
	}

	// 10b. Start gRPC server
	grpcListener, err := net.Listen("tcp", ":"+strconv.Itoa(cfg.GRPCPort))
	if err != nil {
		log.Fatal("Failed to listen for gRPC", zap.Error(err))
	}
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(16*1024*1024),
		grpc.MaxSendMsgSize(16*1024*1024),
	)
	pb.RegisterAffiliateServiceServer(grpcServer, grpcHandler)
	go func() {
		log.Info("Starting gRPC server", zap.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Error("gRPC server error", zap.Error(err))
		}
	}()

	// 11. Start HTTP server
	go func() {
		port := cfg.HTTPPort
		log.Info("Starting Affiliate HTTP server", zap.String("port", port))
		if err := app.Listen(":" + port); err != nil {
			log.Error("Failed to start HTTP server", zap.Error(err))
		}
	}()

	// 12. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Affiliate Service...")
	cancelWorkers()
	if closePublisher != nil {
		closePublisher()
	}
	grpcServer.GracefulStop()
	if err := app.Shutdown(); err != nil {
		log.Error("Failed to shutdown HTTP server", zap.Error(err))
	}
	log.Info("Affiliate Service stopped")
}

// splitBrokers parses a comma-separated broker list, dropping empties.
func splitBrokers(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func healthHandler(c *fiber.Ctx) error {
	return c.SendString("ok")
}

func readyHandler(c *fiber.Ctx) error {
	return c.SendString("ready")
}

// startNGRConsumer starts the Redpanda consumer for NGR events
func startNGRConsumer(ctx context.Context, cfg *config.Config, svc *service.AffiliateService, log *zap.Logger) {
	brokers := strings.Split(cfg.RedpandaBrokers, ",")
	consumerCfg := consumer.DefaultNGRConsumerConfig()
	consumerCfg.Brokers = brokers

	franzCfg := consumer.FranzConsumerConfig{
		Brokers: brokers,
		GroupID: consumerCfg.GroupID,
		Topics:  consumerCfg.Topics,
	}

	franzConsumer, err := consumer.NewFranzConsumer(ctx, franzCfg, log)
	if err != nil {
		log.Error("Failed to create Franz consumer", zap.Error(err))
		return
	}
	defer franzConsumer.Close()

	ngrConsumer := consumer.NewNGRConsumer(svc, log, consumerCfg)
	if err := ngrConsumer.Run(ctx, franzConsumer); err != nil && err != context.Canceled {
		log.Error("NGR consumer error", zap.Error(err))
	}
}
