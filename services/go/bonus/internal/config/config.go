package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
)

// Config holds Bonus Service runtime configuration.
// All values come from environment with safe local defaults.
// Production must override secrets and URLs via env / Vault.
type Config struct {
	Env          string
	HTTPPort     string
	GRPCPort     string
	DatabaseURL  string
	RedisAddr    string
	KafkaBrokers []string
	JWTSecretKey string

	// WalletGRPCAddr is wallet-core gRPC (CONVENTIONS: 50053).
	WalletGRPCAddr string

	// OpenTelemetry (observability standard: OTel SDK in every service).
	// Tracing is DISABLED unless OTEL_EXPORTER_OTLP_ENDPOINT is set, so local
	// development and tests never need a collector.
	OTLPEndpoint    string
	OTELServiceName string
	OTELSampleRatio float64
	OTELInsecure    bool

	WelcomePct        int
	WelcomeMaxUSD     decimal.Decimal
	WelcomeWagering   int
	WelcomeExpiryDays int
}

// Load reads configuration from environment.
func Load() *Config {
	return &Config{
		Env:          getEnv("APP_ENV", "development"),
		HTTPPort:     getEnv("PORT", "8088"),
		GRPCPort:     getEnv("GRPC_PORT", "50056"),
		DatabaseURL:  getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/opus_casino?sslmode=disable"),
		RedisAddr:    getEnv("REDIS_ADDR", "localhost:6379"),
		KafkaBrokers: splitCSV(getEnv("KAFKA_BROKERS", "localhost:9092")),
		JWTSecretKey: getEnv("JWT_SECRET_KEY", "change-me-in-production"),

		WalletGRPCAddr: getEnv("WALLET_GRPC_ADDR", "wallet-core:50053"),

		OTLPEndpoint:    getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		OTELServiceName: getEnv("OTEL_SERVICE_NAME", "bonus-service"),
		OTELSampleRatio: getFloatEnv("OTEL_TRACES_SAMPLER_ARG", 1.0),
		OTELInsecure:    getBoolEnv("OTEL_EXPORTER_OTLP_INSECURE", true),

		WelcomePct:        getIntEnv("WELCOME_BONUS_PERCENTAGE", 100),
		WelcomeMaxUSD:     getDecimalEnv("WELCOME_BONUS_MAX_AMOUNT", "200"),
		WelcomeWagering:   getIntEnv("WELCOME_BONUS_WAGERING_REQUIREMENT", 30),
		WelcomeExpiryDays: getIntEnv("WELCOME_BONUS_EXPIRY_DAYS", 30),
	}
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getIntEnv(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// getFloatEnv reads a float env var, clamping it to [0,1].
// Used for the trace sampler ratio — never for money (NEVER-6).
func getFloatEnv(key string, def float64) float64 {
	raw := getEnv(key, "")
	if raw == "" {
		return def
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v < 0 || v > 1 {
		return def
	}
	return v
}

func getBoolEnv(key string, def bool) bool {
	raw := strings.ToLower(strings.TrimSpace(getEnv(key, "")))
	if raw == "" {
		return def
	}
	return raw == "1" || raw == "true" || raw == "yes"
}

// getDecimalEnv parses money values from string env vars.
// NEVER use float64 for money (CONVENTIONS NEVER-6).
func getDecimalEnv(key, def string) decimal.Decimal {
	raw := getEnv(key, def)
	if d, err := decimal.NewFromString(raw); err == nil && !d.IsNegative() {
		return d
	}
	d, _ := decimal.NewFromString(def)
	return d
}

func splitCSV(raw string) []string {
	if raw == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i <= len(raw); i++ {
		if i == len(raw) || raw[i] == ',' {
			part := raw[start:i]
			trimmed := ""
			for _, r := range part {
				if r != ' ' && r != '\t' {
					trimmed += string(r)
				}
			}
			if trimmed != "" {
				out = append(out, trimmed)
			}
			start = i + 1
		}
	}
	return out
}
