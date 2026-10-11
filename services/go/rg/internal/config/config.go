package config

import (
	"os"
	"strconv"
)

// Config holds RG service configuration from environment variables.
// Ports: HTTP 8091, gRPC 50062 (next free after affiliate 8090/50061).
type Config struct {
	GRPCPort int
	HTTPPort string
	Env      string

	DatabaseURL string

	// JWTSecretKey is the shared HS256 secret with auth-service (dev fallback).
	JWTSecretKey string

	// AdminToken guards operator endpoints (manual exclusion, revoke review).
	AdminToken string

	// CoolingHours is the delay before a limit INCREASE takes effect
	// (regulatory range 24-72h). Decreases are always immediate.
	CoolingHours int

	// RevokeCoolingHours is the delay after a temporary exclusion expires
	// before the player may request revocation (positive action + cooling).
	RevokeCoolingHours int
}

// Load builds Config with production-safe defaults.
func Load() *Config {
	return &Config{
		GRPCPort: getIntEnv("GRPC_PORT", 50062),
		HTTPPort: getEnv("PORT", "8091"),
		Env:      getEnv("APP_ENV", "development"),

		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/opus_casino?sslmode=disable"),

		JWTSecretKey: getEnv("JWT_SECRET_KEY", "change-me-in-production"),
		AdminToken:   getEnv("RG_ADMIN_TOKEN", ""),

		CoolingHours:       getIntEnv("RG_COOLING_HOURS", 24),
		RevokeCoolingHours: getIntEnv("RG_REVOKE_COOLING_HOURS", 24),
	}
}

// Validate clamps regulatory parameters into their legal ranges.
func (c *Config) Validate() *Config {
	if c.CoolingHours < 24 {
		c.CoolingHours = 24
	}
	if c.CoolingHours > 72 {
		c.CoolingHours = 72
	}
	if c.RevokeCoolingHours < 24 {
		c.RevokeCoolingHours = 24
	}
	return c
}

func getEnv(key, defaultValue string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultValue
}
