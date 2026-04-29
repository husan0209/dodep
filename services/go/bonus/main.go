package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"strings"

	"github.com/gofiber/fiber/v2"
	fiberrecover "github.com/gofiber/fiber/v2/middleware/recover"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/opus-casino/bonus/internal/consumer"
	"github.com/opus-casino/bonus/internal/domain"
	"github.com/opus-casino/bonus/internal/service"
)

func main() {
	appEnv := getEnv("APP_ENV", "development")
	var log *zap.Logger
	if appEnv == "development" {
		log, _ = zap.NewDevelopment()
	} else {
		log, _ = zap.NewProduction()
	}
	defer log.Sync()

	// ── Database ──────────────────────────────────────────────────────────
	db, err := gorm.Open(postgres.Open(getEnv("DATABASE_URL",
		"postgres://postgres:postgres@localhost:5432/opus_casino?sslmode=disable")), &gorm.Config{})
	if err != nil {
		log.Fatal("Bonus: failed to connect to database", zap.Error(err))
	}

	// Auto-migrate bonus table
	if err := db.AutoMigrate(&domain.Bonus{}); err != nil {
		log.Fatal("Bonus: migration failed", zap.Error(err))
	}

	// ── Bonus Service ──────────────────────────────────────────────────────
	welcomePct, _ := strconv.Atoi(getEnv("WELCOME_BONUS_PERCENTAGE", "100"))
	welcomeMax, _ := strconv.ParseFloat(getEnv("WELCOME_BONUS_MAX_AMOUNT", "200"), 64)
	welcomeWager, _ := strconv.Atoi(getEnv("WELCOME_BONUS_WAGERING_REQUIREMENT", "30"))
	welcomeExpiry, _ := strconv.Atoi(getEnv("WELCOME_BONUS_EXPIRY_DAYS", "30"))

	cfg := service.BonusConfig{
		WelcomePct:        welcomePct,
		WelcomeMaxUSD:     welcomeMax,
		WelcomeWagering:   welcomeWager,
		WelcomeExpiryDays: welcomeExpiry,
	}

	bonusSvc := service.NewBonusService(db, cfg, log)
	log.Info("Bonus service initialized", zap.Any("config", cfg))

	// ── Redpanda Consumer ──────────────────────────────────────────────────
	brokers := strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ",")
	payConsumer, err := consumer.NewPaymentConsumer(brokers, bonusSvc, log)
	if err != nil {
		log.Fatal("Bonus: failed to create payment consumer", zap.Error(err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := payConsumer.Start(ctx); err != nil && err != context.Canceled {
			log.Error("Bonus: consumer stopped", zap.Error(err))
		}
	}()

	// ── gRPC Server ───────────────────────────────────────────────────────
	grpcPort := getEnv("GRPC_PORT", "50056")
	grpcServer := grpc.NewServer()
	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthSrv)
	healthSrv.SetServingStatus("bonus-service", healthpb.HealthCheckResponse_SERVING)

	go func() {
		lis, err := net.Listen("tcp", ":"+grpcPort)
		if err != nil {
			log.Fatal("Bonus: failed to listen gRPC", zap.Error(err))
		}
		log.Info("Bonus: gRPC server started", zap.String("port", grpcPort))
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("Bonus: gRPC error", zap.Error(err))
		}
	}()

	// ── HTTP Server ────────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{
		AppName:      "Bonus Service v1.0.0",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	})
	app.Use(fiberrecover.New())

	httpPort := getEnv("PORT", "8088")
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "bonus"})
	})
	app.Get("/ready", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ready"})
	})

	// Bonus REST API
	api := app.Group("/api/v1/bonuses")
	api.Get("/", func(c *fiber.Ctx) error {
		userIDStr, ok := c.Locals("user_id").(string)
		if !ok {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		var userID int64
		fmt.Sscanf(userIDStr, "%d", &userID)

		limit, _ := strconv.Atoi(c.Query("limit", "20"))
		offset, _ := strconv.Atoi(c.Query("offset", "0"))

		bonuses, total, err := bonusSvc.ListBonuses(c.Context(), userID, limit, offset)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"bonuses": bonuses, "total": total})
	})
	api.Get("/active", func(c *fiber.Ctx) error {
		userIDStr, ok := c.Locals("user_id").(string)
		if !ok {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		var userID int64
		fmt.Sscanf(userIDStr, "%d", &userID)

		bonus, err := bonusSvc.GetActiveBonus(c.Context(), userID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(bonus)
	})

	go func() {
		log.Info("Bonus: HTTP server started", zap.String("port", httpPort))
		if err := app.Listen(":" + httpPort); err != nil {
			log.Error("Bonus: HTTP error", zap.Error(err))
		}
	}()

	// ── Graceful shutdown ──────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	cancel()
	grpcServer.GracefulStop()
	_ = app.ShutdownWithTimeout(10 * time.Second)
	log.Info("Bonus: shutdown complete")
}

func getEnv(key, defaultValue string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return defaultValue
}
