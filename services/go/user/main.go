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

	"github.com/opus-casino/user/internal/config"
	"github.com/opus-casino/user/internal/handlers"
	"github.com/opus-casino/user/internal/repository"
	"github.com/opus-casino/user/internal/service"
	pb "github.com/opus-casino/proto/gen/go/user/v1"
)

func main() {
	cfg := config.Load()
	log, _ := zap.NewProduction()
	defer log.Sync()

	dbPool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer dbPool.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB,
	})
	defer rdb.Close()

	userRepo := repository.NewUserRepository(dbPool)
	userService := service.NewUserService(userRepo, log)

	grpcServer := grpc.NewServer()
	pb.RegisterUserServiceServer(grpcServer, handlers.NewUserGRPCHandler(userService, log))
	reflection.Register(grpcServer)

	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
		if err != nil {
			log.Fatal("Failed to listen on gRPC port", zap.Error(err))
		}
		log.Info("Starting gRPC server", zap.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("Failed to serve gRPC", zap.Error(err))
		}
	}()

	app := fiber.New(fiber.Config{AppName: "User Service v1.0.0"})
	app.Use(recover.New(), logger.New())
	app.Get("/health", func(c *fiber.Ctx) error { return c.SendString("ok") })
	app.Get("/ready", func(c *fiber.Ctx) error { return c.SendString("ready") })

	setupRoutes(app, userService)

	go func() {
		log.Info("Starting HTTP server", zap.String("port", cfg.HTTPPort))
		if err := app.Listen(":" + cfg.HTTPPort); err != nil {
			log.Error("Failed to start HTTP", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("Shutting down User Service...")
	grpcServer.GracefulStop()
	app.Shutdown()
	log.Info("User Service stopped")
}
