package main

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/opus-casino/affiliate/internal/config"
	"github.com/opus-casino/affiliate/internal/event"
	"github.com/opus-casino/affiliate/internal/repository"
	"github.com/opus-casino/affiliate/internal/service"
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
	publisher := event.NewLogPublisher(log)
	outboxWorker := event.NewOutboxWorker(repo, publisher, log, cfg.OutboxPollInterval, cfg.OutboxBatchSize)

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
	if err := app.Shutdown(); err != nil {
		log.Error("Failed to shutdown HTTP server", zap.Error(err))
	}
	log.Info("Affiliate Service stopped")
}

func healthHandler(c *fiber.Ctx) error {
	return c.SendString("ok")
}

func readyHandler(c *fiber.Ctx) error {
	return c.SendString("ready")
}
