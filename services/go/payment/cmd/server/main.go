package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/opus-casino/payment/internal/client"
	"github.com/opus-casino/payment/internal/config"
	"github.com/opus-casino/payment/internal/event"
	// grpcserver "github.com/opus-casino/payment/internal/grpc"
	"github.com/opus-casino/payment/internal/handler"
	"github.com/opus-casino/payment/internal/repository"
	"github.com/opus-casino/payment/internal/service"
)

func main() {
	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("Application failed")
	}
}

func run() error {
	// 1. Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 2. Configure structured logging
	zerolog.TimeFieldFormat = time.RFC3339Nano
	if cfg.Environment == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	log.Info().
		Str("service", "payment-service").
		Str("environment", cfg.Environment).
		Msg("Starting service")

	// 3. Initialize JWT validator
	if err := handler.InitJWTValidator(cfg.Auth.Ed25519PublicKey); err != nil {
		return fmt.Errorf("init jwt validator: %w", err)
	}

	// 4. Initialize tracing
	var tracer trace.Tracer
	var shutdownTracing func(context.Context) error

	if cfg.Tracing.Enabled {
		shutdownTracing, tracer, err = initTracing(cfg)
		if err != nil {
			return fmt.Errorf("init tracing: %w", err)
		}
		defer shutdownTracing(context.Background())
	}

	// 4. Initialize database
	db, err := initDatabase(cfg)
	if err != nil {
		return fmt.Errorf("init database: %w", err)
	}
	defer func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}()

	// 5. Initialize Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		PoolSize: cfg.Redis.PoolSize,
	})
	defer rdb.Close()

	// 5.5. Initialize Zap logger for dependencies
	zapLogger, _ := zap.NewProduction()
	if cfg.Environment == "development" {
		zapLogger, _ = zap.NewDevelopment()
	}
	defer zapLogger.Sync()

	// 6. Initialize event producer
	producer, err := event.NewProducer(event.ProducerConfig{
		Brokers: cfg.Kafka.Brokers,
		Topic:   cfg.Kafka.TopicPrefix,
	}, zapLogger)
	if err != nil {
		return fmt.Errorf("create producer: %w", err)
	}
	defer producer.Close()

	// 7. Build layers
	repos := buildRepositories(db, rdb)
	clients, err := buildClients(cfg, zapLogger, tracer)
	if err != nil {
		return fmt.Errorf("build clients: %w", err)
	}
	services := buildServices(cfg, repos, clients, producer, tracer)
	handlers := handler.New(
		services.Payment,
		services.Withdrawal,
		services.Webhook,
	)

	// 8. Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	// 9. Start HTTP server
	g.Go(func() error {
		return startHTTPServer(ctx, cfg, handlers)
	})

	// 10. Start gRPC server
	g.Go(func() error {
		return startGRPCServer(ctx, cfg, services)
	})

	// 11. Start metrics server
	g.Go(func() error {
		return startMetricsServer(ctx, cfg)
	})

	// 12. Wait for shutdown signal
	g.Go(func() error {
		return waitForShutdown(ctx, cancel)
	})

	log.Info().
		Int("http_port", cfg.Server.Port).
		Int("grpc_port", 50055).
		Int("metrics_port", 9104).
		Msg("Service started")

	return g.Wait()
}

func initTracing(cfg *config.Config) (func(context.Context) error, trace.Tracer, error) {
	ctx := context.Background()

	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Tracing.Endpoint),
	}
	if cfg.Tracing.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	
	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("create trace exporter: %w", err)
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.Tracing.ServiceName),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.Tracing.SampleRate)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tracer := tp.Tracer("payment-service")

	return tp.Shutdown, tracer, nil
}

func initDatabase(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Database,
		cfg.Database.SSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)

	return db, nil
}

type Repositories struct {
	Payment      repository.PaymentRepository
	Withdrawal   repository.WithdrawalRepository
	Idempotency  repository.IdempotencyRepository
	ExchangeRate repository.ExchangeRateRepository
	DailyLimits  repository.DailyLimitsRepository
	AuditLog     repository.AuditLogRepository
}

func buildRepositories(db *gorm.DB, rdb *redis.Client) *Repositories {
	return &Repositories{
		Payment:      repository.NewPaymentRepository(db),
		Withdrawal:   repository.NewWithdrawalRepository(db),
		Idempotency:  repository.NewIdempotencyRepository(rdb),
		ExchangeRate: repository.NewExchangeRateRepository(rdb),
		DailyLimits:  repository.NewDailyLimitsRepository(rdb),
		AuditLog:     repository.NewAuditLogRepository(db),
	}
}

type Clients struct {
	NOWPayments client.NOWPaymentsAPI
	Wallet      client.WalletAPI
	User        client.UserAPI
}

func buildClients(cfg *config.Config, logger *zap.Logger, tracer trace.Tracer) (*Clients, error) {
	nowPaymentsConfig := client.NOWPaymentsConfig{
		BaseURL:   cfg.NOWPayments.BaseURL,
		APIKey:    cfg.NOWPayments.APIKey,
		IPNSecret: cfg.NOWPayments.IPNSecret,
		Timeout:   cfg.NOWPayments.Timeout,
	}

	walletConfig := client.WalletClientConfig{
		Address: cfg.Wallet.Address,
		Timeout: cfg.Wallet.Timeout,
	}

	userConfig := client.UserClientConfig{
		Address: cfg.User.Address,
		Timeout: cfg.User.Timeout,
	}

	walletClient, err := client.NewWalletClient(walletConfig, logger, tracer)
	if err != nil {
		return nil, fmt.Errorf("create wallet client: %w", err)
	}

	userClient, err := client.NewUserClient(userConfig, logger, tracer)
	if err != nil {
		return nil, fmt.Errorf("create user client: %w", err)
	}

	return &Clients{
		NOWPayments: client.NewNOWPaymentsClient(nowPaymentsConfig, logger),
		Wallet:      walletClient,
		User:        userClient,
	}, nil
}

type Services struct {
	Payment    *service.PaymentService
	Withdrawal *service.WithdrawalService
	Webhook    *service.WebhookService
	KYCLimits  *service.KYCLimitsService
	Exchange   *service.ExchangeRateService
}

func buildServices(
	cfg *config.Config,
	repos *Repositories,
	clients *Clients,
	producer *event.Producer,
	tracer trace.Tracer,
) *Services {
	return &Services{
		Payment: service.NewPaymentService(
			repos.Payment,
			repos.Idempotency,
			repos.ExchangeRate,
			repos.DailyLimits,
			clients.NOWPayments,
			clients.Wallet,
			clients.User,
			producer,
			tracer,
			cfg.NOWPayments.IPNCallbackURL,
		),
		Withdrawal: service.NewWithdrawalService(
			repos.Withdrawal,
			repos.Idempotency,
			repos.ExchangeRate,
			repos.DailyLimits,
			clients.NOWPayments,
			clients.Wallet,
			clients.User,
			producer,
			tracer,
			cfg.NOWPayments.IPNCallbackURL,
		),
		Webhook: service.NewWebhookService(
			repos.Payment,
			repos.Withdrawal,
			repos.Idempotency,
			repos.AuditLog,
			clients.NOWPayments,
			clients.Wallet,
			producer,
			tracer,
		),
		KYCLimits: service.NewKYCLimitsService(repos.DailyLimits, nil),
		Exchange:  service.NewExchangeRateService(repos.ExchangeRate, clients.NOWPayments.(*client.NOWPaymentsClient), nil, service.DefaultExchangeRateConfig()),
	}
}

func startHTTPServer(ctx context.Context, cfg *config.Config, handlers *handler.Handlers) error {
	app := fiber.New(fiber.Config{
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSec) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSec) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeoutSec) * time.Second,
	})

	app.Use(recover.New())

	// Health endpoints
	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "healthy"})
	})
	app.Get("/readyz", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ready"})
	})

	// API routes
	api := app.Group("/api/v1")
	payments := api.Group("/payments")

	// Middleware
	payments.Use(handler.AuthMiddleware())
	payments.Use(handler.LoggingMiddleware())
	payments.Use(handler.RateLimitMiddleware())

	// Payment routes
	payments.Post("/deposit", handlers.Payment.InitiateDeposit)
	payments.Get("/:id", handlers.Payment.GetPayment)
	payments.Get("/history", handlers.Payment.GetPaymentHistory)

	// Withdrawal routes
	payments.Post("/withdraw", handlers.Withdrawal.InitiateWithdrawal)
	payments.Get("/withdrawals/history", handlers.Withdrawal.GetWithdrawalHistory)
	payments.Get("/withdrawals/:id", handlers.Withdrawal.GetWithdrawal)

	// Webhook routes (no auth)
	payments.Post("/webhooks/nowpayments", handlers.Webhook.ProcessNOWPaymentsWebhook)

	log.Info().Int("port", cfg.Server.Port).Msg("Starting HTTP server")

	if err := app.Listen(fmt.Sprintf(":%d", cfg.Server.Port)); err != nil {
		select {
		case <-ctx.Done():
			return nil // Context cancelled, normal shutdown
		default:
			return fmt.Errorf("http server: %w", err)
		}
	}
	return nil
}

func startGRPCServer(ctx context.Context, cfg *config.Config, services *Services) error {
	server := grpc.NewServer()

	// Health check
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus("payment-service", healthpb.HealthCheckResponse_SERVING)

	// TODO: Register payment gRPC service from proto
	// paymentpb.RegisterPaymentServiceServer(server, grpcserver.NewPaymentGRPCServer(services.Payment))

	addr := fmt.Sprintf(":%d", 50055)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen grpc: %w", err)
	}

	log.Info().Str("addr", addr).Msg("Starting gRPC server")

	if err := server.Serve(lis); err != nil {
		select {
		case <-ctx.Done():
			return nil // Context cancelled, normal shutdown
		default:
			return fmt.Errorf("grpc server: %w", err)
		}
	}
	return nil
}

func startMetricsServer(ctx context.Context, cfg *config.Config) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:    ":9104",
		Handler: mux,
	}

	log.Info().Str("addr", ":9104").Msg("Starting metrics server")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		select {
		case <-ctx.Done():
			return nil
		default:
			return fmt.Errorf("metrics server: %w", err)
		}
	}
	return nil
}

func waitForShutdown(ctx context.Context, cancel context.CancelFunc) error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
	case <-ctx.Done():
		log.Info().Msg("Context cancelled")
	}

	cancel()
	return nil
}
