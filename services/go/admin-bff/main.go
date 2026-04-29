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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/opus-casino/admin-bff/internal/client"
	"github.com/opus-casino/admin-bff/internal/config"
	"github.com/opus-casino/admin-bff/internal/handlers"
	"github.com/opus-casino/admin-bff/internal/middleware"
	"github.com/opus-casino/admin-bff/internal/service"
)

// corsOriginsFromEnv reads $CORS_ORIGINS (comma-separated), strips "*",
// and falls back to devDefault when unset.
func corsOriginsFromEnv(devDefault string) string {
	raw := strings.TrimSpace(os.Getenv("CORS_ORIGINS"))
	if raw == "" {
		raw = devDefault
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
	cfg := config.Load()

	log, _ := zap.NewProduction()
	defer log.Sync()

	dbPool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer dbPool.Close()
	log.Info("Database connected")

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to init GORM", zap.Error(err))
	}

	// Dev-only: skip AutoMigrate to avoid enum-type errors; run SQL migrations instead.
	log.Info("Database migration skipped in dev (run SQL migrations manually)")

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer rdb.Close()
	log.Info("Redis connected")

	app := fiber.New(fiber.Config{
		AppName: "Admin BFF v1.0.0",
	})

	app.Use(recover.New())
	// CORS: allowlist from $CORS_ORIGINS (comma-separated). Wildcard "*"
	// is stripped. Dev fallback covers local admin SPA on :3001.
	// In production set $CORS_ORIGINS explicitly via Helm values.
	allowedOrigins := corsOriginsFromEnv("http://localhost:3001, http://127.0.0.1:3001")
	app.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Request-ID, X-Idempotency-Key",
		AllowCredentials: allowedOrigins != "",
	}))
	app.Use(logger.New())

	app.Get("/health", func(c *fiber.Ctx) error { return c.SendString("ok") })
	app.Get("/ready", func(c *fiber.Ctx) error { return c.SendString("ready") })

	// gRPC clients
	userClient, err := client.NewUserClient(cfg.UserService.Address, cfg.UserService.Timeout)
	if err != nil {
		log.Fatal("Failed to connect to user service", zap.Error(err))
	}
	defer userClient.Close()
	log.Info("User service client ready")

	paymentClient, err := client.NewPaymentClient(cfg.PaymentService.Address, cfg.PaymentService.Timeout)
	if err != nil {
		log.Fatal("Failed to connect to payment service", zap.Error(err))
	}
	defer paymentClient.Close()
	log.Info("Payment service client ready")

	bettingClient, err := client.NewBettingClient(cfg.BettingEngine.Address, cfg.BettingEngine.Timeout)
	if err != nil {
		log.Fatal("Failed to connect to betting engine", zap.Error(err))
	}
	defer bettingClient.Close()
	log.Info("Betting engine client ready")

	// Services
	auditSvc := service.NewAuditService(db, log)
	usersSvc := service.NewUsersService(userClient, auditSvc)
	financeSvc := service.NewFinanceService(paymentClient, auditSvc)

	// WebSocket / SSE hub — starts background goroutines for metric push.
	redpandaBrokers := cfg.RedpandaBrokers
	if !cfg.RedpandaEnabled {
		redpandaBrokers = nil
	}
	wsHub := handlers.NewWSHub(rdb, log, redpandaBrokers)
	wsHubCtx, wsHubCancel := context.WithCancel(context.Background())
	defer wsHubCancel()
	wsHub.Start(wsHubCtx)

	// Public admin auth routes — login & refresh MUST NOT require an existing
	// JWT (chicken-and-egg). Mounted BEFORE the protected /admin group so that
	// the AdminAuth middleware does not block credential exchange.
	publicAdminAuth := app.Group("/admin/auth")
	// Apply login-specific rate limiting BEFORE handler.
	publicAdminAuth.Use(middleware.RateLimitLogin(rdb))
	handlers.RegisterPublicAdminAuthRoutes(publicAdminAuth, db, log, cfg.JWTSecretKey)

	// Admin routes — protected by: JWT auth → maintenance check → IP whitelist → API rate limit.
	admin := app.Group("/admin",
		middleware.AdminAuth(cfg.JWTSecretKey, dbPool),
		middleware.MaintenanceMode(rdb),
		middleware.IPWhitelist(db),
		middleware.RateLimitAPI(rdb),
	)

	handlers.RegisterProtectedAdminAuthRoutes(admin, db, log, cfg.JWTSecretKey)
	handlers.RegisterDashboardRoutes(admin, db, log, auditSvc)
	handlers.RegisterUserRoutes(admin, usersSvc, log)
	handlers.RegisterFinanceRoutes(admin, financeSvc, db, log)
	handlers.RegisterPlayerRoutes(admin, db, log, auditSvc)
	handlers.RegisterCRMRoutes(admin, db, log, auditSvc)
	handlers.RegisterWSRoutes(admin, wsHub, log)

	handlers.RegisterKycRoutes(admin, db, log)
	handlers.RegisterPaymentRoutes(admin, db, log)
	handlers.RegisterSupportRoutes(admin, db, log)
	handlers.RegisterDepositRoutes(admin, db)
	// Apply withdrawal-specific rate limiting
	admin.Post("/withdrawals/:id/approve", middleware.RateLimitWithdrawalApprove(rdb))
	admin.Post("/players/:id/adjust-balance", middleware.RateLimitBalanceAdjust(rdb))
	handlers.RegisterWithdrawalRoutes(admin, db)
	handlers.RegisterBonusRoutes(admin, db)
	handlers.RegisterRiskRoutes(admin, db, log, auditSvc)
	handlers.RegisterSportsRoutes(admin, db, log, bettingClient, auditSvc)
	handlers.RegisterSettingsRoutes(admin, db, log)
	handlers.RegisterReportsRoutes(admin, db, log)
	handlers.RegisterRegulatoryRoutes(admin, db, log)
	handlers.RegisterCasinoRoutes(admin, db, log, auditSvc)
	handlers.RegisterAffiliateRoutes(admin, db, log, auditSvc)

	go func() {
		port := cfg.HTTPPort
		log.Info("Starting Admin BFF HTTP server", zap.String("port", port))
		if err := app.Listen(":" + port); err != nil {
			log.Error("Failed to start HTTP server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Admin BFF...")
	if err := app.Shutdown(); err != nil {
		log.Error("Failed to shutdown", zap.Error(err))
	}
	log.Info("Admin BFF stopped")
}
