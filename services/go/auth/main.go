package main

import (
	"context"
	"fmt"
	"net"
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
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/opus-casino/auth/internal/config"
	"github.com/opus-casino/auth/internal/crypto"
	"github.com/opus-casino/auth/internal/handlers"
	"github.com/opus-casino/auth/internal/repository"
	"github.com/opus-casino/auth/internal/service"
	pb "github.com/opus-casino/proto/gen/go/auth/v1"
)

// corsOriginsFromEnv reads $CORS_ORIGINS (comma-separated) and rejects "*".
// If unset, returns devDefault (used only in non-production environments).
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
	// Load configuration
	cfg := config.Load()

	// Initialize logger
	log, _ := zap.NewProduction()
	defer func() { _ = log.Sync() }()

	// Initialize database connection
	dbPool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer dbPool.Close()

	log.Info("Database connected")

	// Initialize Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer rdb.Close()

	log.Info("Redis connected")

	// Initialize repository
	authRepo := repository.NewAuthRepository(dbPool, rdb)

	// Initialize service
	jwtConfig, err := crypto.NewJWTConfigFromEnv(cfg.Env, cfg.JWTSecretKey, cfg.JWTEd25519PrivateKeyBase64, cfg.JWTEd25519PublicKeyBase64)
	if err != nil {
		log.Fatal("Failed to initialize JWT config", zap.Error(err))
	}
	authService := service.NewAuthService(authRepo, jwtConfig, log)
	authService.ConfigureGoogleOAuth(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURI, cfg.WebAppURL)

	// Initialize gRPC server
	grpcServer := grpc.NewServer()
	pb.RegisterAuthServiceServer(grpcServer, handlers.NewAuthGRPCHandler(authService, log))
	reflection.Register(grpcServer)

	// Start gRPC server
	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
		if err != nil {
			log.Fatal("Failed to listen on gRPC port", zap.Error(err))
		}
		log.Info("Starting gRPC server", zap.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("Failed to serve gRPC server", zap.Error(err))
		}
	}()

	// Initialize Fiber app
	app := fiber.New(fiber.Config{
		AppName: "Auth Service v1.0.0",
	})

	app.Use(recover.New())
	// CORS: allowlist from $CORS_ORIGINS (comma-separated). Wildcard "*" is
	// stripped. Dev fallback covers local web (3000) and admin (3001) origins.
	// In production set $CORS_ORIGINS explicitly via Helm values.
	devDefault := "http://localhost:3000, http://127.0.0.1:3000, http://0.0.0.0:3000, http://web:3000, http://localhost:3001, http://127.0.0.1:3001"
	allowedOrigins := corsOriginsFromEnv(devDefault)
	app.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Request-ID, X-Idempotency-Key",
		AllowCredentials: allowedOrigins != "",
	}))
	app.Use(logger.New())

	// Health checks
	app.Get("/health", healthHandler)
	app.Get("/ready", readyHandler)

	// Setup routes
	setupRoutes(app, authService)

	// Start HTTP server
	go func() {
		port := cfg.HTTPPort
		log.Info("Starting HTTP server", zap.String("port", port))
		if err := app.Listen(":" + port); err != nil {
			log.Error("Failed to start HTTP server", zap.Error(err))
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Auth Service...")
	grpcServer.GracefulStop()
	if err := app.Shutdown(); err != nil {
		log.Error("Failed to shutdown HTTP server", zap.Error(err))
	}
	log.Info("Auth Service stopped")
}

func healthHandler(c *fiber.Ctx) error {
	return c.SendString("ok")
}

func readyHandler(c *fiber.Ctx) error {
	return c.SendString("ready")
}
