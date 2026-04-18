module github.com/platform/services/payment-service

go 1.22

require (
	github.com/go-playground/validator/v10 v10.19.0
	github.com/gofiber/fiber/v2 v2.52.4
	github.com/google/uuid v1.6.0
	github.com/prometheus/client_golang v1.19.0
	github.com/redis/go-redis/v9 v9.5.1
	github.com/shopspring/decimal v1.4.0
	github.com/spf13/viper v1.18.2
	github.com/twmb/franz-go v1.15.4
	go.opentelemetry.io/otel v1.26.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.26.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.26.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.26.0
	go.opentelemetry.io/otel/sdk v1.26.0
	go.opentelemetry.io/otel/trace v1.26.0
	go.uber.org/zap v1.27.0
	google.golang.org/grpc v1.63.2
	google.golang.org/protobuf v1.33.0
	gorm.io/driver/postgres v1.5.7
	gorm.io/gorm v1.25.7
)
