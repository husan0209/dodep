package config

import (
	"fmt"
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
	JWTEd25519PrivateKeyBase64 string
	JWTEd25519PublicKeyBase64  string

	// Google OAuth
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURI  string
	WebAppURL          string

	// Environment
	Env string
}

func Load() *Config {
	const defaultJWTSecret = "change-me-in-production"

	grpcPort, _ := strconv.Atoi(getEnv("GRPC_PORT", "9090"))
	httpPort := getEnv("PORT", "8080")

	cfg := &Config{
		GRPCPort: grpcPort,
		HTTPPort: httpPort,

		// Database
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:changeme@localhost:5433/opus_casino?sslmode=disable"),

		// Redis
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", "changeme"),
		RedisDB:       getIntEnv("REDIS_DB", 0),

		// JWT
		JWTSecretKey: getEnv("JWT_SECRET_KEY", defaultJWTSecret),
		JWTEd25519PrivateKeyBase64: getEnv("JWT_ED25519_PRIVATE_KEY", ""),
		JWTEd25519PublicKeyBase64:  getEnv("JWT_ED25519_PUBLIC_KEY", ""),

		// Google OAuth
		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURI:  getEnv("GOOGLE_REDIRECT_URI", fmt.Sprintf("http://localhost:%s/api/v1/auth/google/callback", httpPort)),
		WebAppURL:          getEnv("WEB_APP_URL", "http://localhost:3000"),

		// Environment
		Env: getEnv("APP_ENV", "development"),
	}

	// Fail-fast in non-development environments if the JWT secret is unset / default.
	// This prevents accidentally signing tokens with a public, known string.
	if cfg.Env != "development" && (cfg.JWTSecretKey == "" || cfg.JWTSecretKey == defaultJWTSecret) {
		// If Ed25519 keys are not configured, we refuse to start.
		// HS256 is allowed only for development fallback.
		if cfg.JWTEd25519PrivateKeyBase64 == "" || cfg.JWTEd25519PublicKeyBase64 == "" {
			panic("JWT_ED25519_PRIVATE_KEY and JWT_ED25519_PUBLIC_KEY must be set when APP_ENV != development")
		}
	}

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		if value != "" {
			return value
		}
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
