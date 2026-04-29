package config

import (
	"os"
	"strings"
	"time"
)

type Config struct {
	HTTPPort        string
	DatabaseURL     string
	RedisAddr       string
	RedisPassword   string
	RedisDB         int
	JWTSecretKey    string
	ClickHouseDSN   string
	VaultAddr       string
	VaultToken      string
	VaultTransitKey string
	RedpandaEnabled bool
	RedpandaBrokers []string

	AuthService    ServiceConfig
	UserService    ServiceConfig
	PaymentService ServiceConfig
	BettingEngine  ServiceConfig
}

type ServiceConfig struct {
	Address        string
	Timeout        time.Duration
	MaxRecvMsgSize int
	MaxSendMsgSize int
}

func Load() *Config {
	redpandaBrokers := splitAndTrim(getEnv("REDPANDA_BROKERS", ""))
	return &Config{
		HTTPPort:        getEnv("HTTP_PORT", "8080"),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/opus?sslmode=disable"),
		RedisAddr:       getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:   getEnv("REDIS_PASSWORD", ""),
		RedisDB:         0,
		JWTSecretKey:    getEnv("JWT_SECRET_KEY", "dev-secret-change-me"),
		ClickHouseDSN:   getEnv("CLICKHOUSE_DSN", ""),
		VaultAddr:       getEnv("VAULT_ADDR", ""),
		VaultToken:      getEnv("VAULT_TOKEN", ""),
		VaultTransitKey: getEnv("VAULT_TRANSIT_KEY", ""),
		RedpandaEnabled: getEnv("REDPANDA_ENABLED", "false") == "true",
		RedpandaBrokers: redpandaBrokers,

		AuthService: ServiceConfig{
			Address: getEnv("AUTH_SERVICE_ADDR", "localhost:50051"),
			Timeout: parseDuration(getEnv("AUTH_SERVICE_TIMEOUT", "2s")),
		},
		UserService: ServiceConfig{
			Address: getEnv("USER_SERVICE_ADDR", "localhost:50052"),
			Timeout: parseDuration(getEnv("USER_SERVICE_TIMEOUT", "3s")),
		},
		PaymentService: ServiceConfig{
			Address: getEnv("PAYMENT_SERVICE_ADDR", "localhost:50055"),
			Timeout: parseDuration(getEnv("PAYMENT_SERVICE_TIMEOUT", "10s")),
		},
		BettingEngine: ServiceConfig{
			Address: getEnv("BETTING_ENGINE_ADDR", "localhost:50054"),
			Timeout: parseDuration(getEnv("BETTING_ENGINE_TIMEOUT", "10s")),
		},
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitAndTrim(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 5 * time.Second
	}
	return d
}
