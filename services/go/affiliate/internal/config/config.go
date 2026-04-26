package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds application configuration loaded from environment variables.
type Config struct {
	// Server
	GRPCPort int
	HTTPPort string

	// Database
	DatabaseURL string

	// Redis
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// JWT (shared secret with auth service)
	JWTSecretKey string

	// Environment
	Env string

	// Outbox worker
	OutboxPollInterval time.Duration
	OutboxBatchSize    int
}

// Load creates a Config from environment variables with sensible defaults.
func Load() *Config {
	grpcPort, _ := strconv.Atoi(getEnv("GRPC_PORT", "50061"))

	return &Config{
		GRPCPort: grpcPort,
		HTTPPort: getEnv("PORT", "8090"),

		// Database
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/opus_casino?sslmode=disable"),

		// Redis
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getIntEnv("REDIS_DB", 0),

		// JWT — must match auth service JWT_SECRET_KEY
		JWTSecretKey: getEnv("JWT_SECRET_KEY", "change-me-in-production"),

		// Environment
		Env: getEnv("APP_ENV", "development"),

		// Outbox worker
		OutboxPollInterval: getDurationEnv("OUTBOX_POLL_INTERVAL", 2*time.Second),
		OutboxBatchSize:    getIntEnv("OUTBOX_BATCH_SIZE", 100),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if durationValue, err := time.ParseDuration(value); err == nil {
			return durationValue
		}
	}
	return defaultValue
}
