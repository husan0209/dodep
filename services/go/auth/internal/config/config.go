package config

import (
	"os"
	"strconv"
)

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

	// JWT
	JWTSecretKey string

	// Environment
	Env string
}

func Load() *Config {
	grpcPort, _ := strconv.Atoi(getEnv("GRPC_PORT", "9090"))

	return &Config{
		GRPCPort: grpcPort,
		HTTPPort: getEnv("PORT", "8080"),

		// Database
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:changeme@localhost:5433/opus_casino?sslmode=disable"),

		// Redis
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", "changeme"),
		RedisDB:       getIntEnv("REDIS_DB", 0),

		// JWT
		JWTSecretKey: getEnv("JWT_SECRET_KEY", "change-me-in-production"),

		// Environment
		Env: getEnv("APP_ENV", "development"),
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
