package main

import (
	"context"
	"fmt"
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

	"github.com/opus-casino/notification/internal/config"
	"github.com/opus-casino/notification/internal/handlers"
	pb "github.com/opus-casino/proto/gen/go/notification/v1"
	"github.com/opus-casino/notification/internal/repository"
	"github.com/opus-casino/notification/internal/service"
	"github.com/opus-casino/notification/internal/consumer"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize logger
	zapLogger, _ := zap.NewProduction()
	defer zapLogger.Sync()

	// Initialize database connection
	dbPool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		zapLogger.Fatal("Failed to connect to database", zap.Error(err))
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
	notifRepo := repository.NewNotificationRepository(dbPool, rdb)

	// Initialize service
	notificationService := service.NewNotificationService(notifRepo, zapLogger)

	// Initialize gRPC server
	grpcServer := grpc.NewServer()
	pb.RegisterNotificationServiceServer(grpcServer, handlers.NewNotificationGRPCHandler(notificationService))
	reflection.Register(grpcServer)

	// Start gRPC server
	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
		if err != nil {
			zapLogger.Fatal("Failed to listen on gRPC port", zap.Error(err))
		}
		zapLogger.Info("Starting gRPC server", zap.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			zapLogger.Error("Failed to serve gRPC server", zap.Error(err))
		}
	}()

	// Initialize Fiber app
	app := fiber.New(fiber.Config{
		AppName: "Notification Service v1.0.0",
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
			port = "8081"
		}
		zapLogger.Info("Starting HTTP server", zap.String("port", port))
		if err := app.Listen(":" + port); err != nil {
			zapLogger.Error("Failed to start HTTP server", zap.Error(err))
		}
	}()

	// Start Redpanda consumer
	if cfg.RedpandaEnabled {
		consumer := consumer.NewRedpandaConsumer(cfg.RedpandaBrokers, notificationService, zapLogger)
		go func() {
			zapLogger.Info("Starting Redpanda consumer")
			if err := consumer.Start(context.Background()); err != nil {
				zapLogger.Error("Failed to start Redpanda consumer", zap.Error(err))
			}
		}()
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	zapLogger.Info("Shutting down Notification Service...")
	grpcServer.GracefulStop()
}

func healthHandler(c *fiber.Ctx) error {
	return c.SendString("ok")
}

func readyHandler(c *fiber.Ctx) error {
	return c.SendString("ready")
}
