package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/opus-casino/casino/internal/config"
	"github.com/opus-casino/casino/internal/handlers"
	pb "github.com/opus-casino/proto/gen/go/casino/v1"
	"github.com/opus-casino/casino/internal/repository"
	"github.com/opus-casino/casino/internal/service"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Initialize database connection
	dbPool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer dbPool.Close()

	// Initialize Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer rdb.Close()

	// Initialize repository
	casinoRepo := repository.NewCasinoRepository(dbPool, rdb)

	// Initialize service
	casinoService := service.NewCasinoService(casinoRepo, logger)

	// Initialize gRPC server
	grpcServer := grpc.NewServer()
	pb.RegisterCasinoServiceServer(grpcServer, handlers.NewCasinoGRPCHandler(casinoService))
	reflection.Register(grpcServer)

	// Start gRPC server
	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
		if err != nil {
			logger.Fatal("Failed to listen on gRPC port", zap.Error(err))
		}
		logger.Info("Starting gRPC server", zap.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("Failed to serve gRPC server", zap.Error(err))
		}
	}()

	// Initialize Fiber app
	app := fiber.New(fiber.Config{
		AppName: "Casino Service v1.0.0",
	})

	app.Use(recover.New())
	app.Use(logger.New())

	// Health checks
	app.Get("/health", healthHandler)
	app.Get("/ready", readyHandler)

	// Start HTTP server
	go func() {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		logger.Info("Starting HTTP server", zap.String("port", port))
		if err := app.Listen(":" + port); err != nil {
			logger.Error("Failed to start HTTP server", zap.Error(err))
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down Casino Service...")
	grpcServer.GracefulStop()
}

func healthHandler(c *fiber.Ctx) error {
	return c.SendString("ok")
}

func readyHandler(c *fiber.Ctx) error {
	return c.SendString("ready")
}
