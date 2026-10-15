package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds KYC Service runtime configuration.
// All values come from environment with safe local defaults.
// Production must override secrets and URLs via env / Vault (NEVER-5).
type Config struct {
	Env         string
	HTTPPort    string
	GRPCPort    string
	DatabaseURL string
	RedisAddr   string
	RedisDB     int

	// JWT shared secret for user tokens (HS256, issued by auth-service).
	JWTSecretKey string

	// Sumsub (verification provider) settings.
	SumsubAPIKey        string
	SumsubAPISecret     string
	SumsubWebhookSecret string
	SumsubBaseURL       string

	// ComplyAdvantage (PEP/sanctions screening) settings.
	// Empty API key = stub provider (documented in docs/api/kyc.md).
	ComplyAdvantageAPIKey string
	ComplyAdvantageURL    string

	// FieldKey is the AES-256 key for PII field encryption (base64 or hex,
	// 32 bytes). Empty in development uses an ephemeral key and logs loudly;
	// production refuses to boot without it.
	FieldKey string
}

// Load reads configuration from environment.
func Load() *Config {
	return &Config{
		Env:          getEnv("APP_ENV", "development"),
		HTTPPort:     getEnv("PORT", "8089"),
		GRPCPort:     getEnv("GRPC_PORT", "50059"),
		DatabaseURL:  getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/opus_casino?sslmode=disable"),
		RedisAddr:    getEnv("REDIS_ADDR", "localhost:6379"),
		RedisDB:      getIntEnv("REDIS_DB", 0),
		JWTSecretKey: getEnv("JWT_SECRET_KEY", "change-me-in-production"),

		SumsubAPIKey:        getEnv("SUMSUB_API_KEY", ""),
		SumsubAPISecret:     getEnv("SUMSUB_API_SECRET", ""),
		SumsubWebhookSecret: getEnv("SUMSUB_WEBHOOK_SECRET", ""),
		SumsubBaseURL:       getEnv("SUMSUB_BASE_URL", "https://api.sumsub.com"),

		ComplyAdvantageAPIKey: getEnv("COMPLYADVANTAGE_API_KEY", ""),
		ComplyAdvantageURL:    getEnv("COMPLYADVANTAGE_URL", "https://api.complyadvantage.com"),

		FieldKey: getEnv("KYC_FIELD_KEY", ""),
	}
}

// Validate reports configuration that must stop the process at startup.
// Non-development environments require a real JWT secret and a field key.
func (c *Config) Validate() error {
	if c == nil {
		return errNil("config is nil")
	}
	if c.HTTPPort == c.GRPCPort {
		return errNil("http and grpc ports must differ")
	}
	if c.Env == "development" {
		return nil
	}
	if c.JWTSecretKey == "" || c.JWTSecretKey == "change-me-in-production" {
		return errNil("JWT_SECRET_KEY must be set outside development")
	}
	if len(c.JWTSecretKey) < 32 {
		return errNil("JWT_SECRET_KEY must be at least 32 bytes outside development")
	}
	if c.FieldKey == "" {
		return errNil("KYC_FIELD_KEY must be set outside development")
	}
	return nil
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return def
}

func getIntEnv(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}

type configError string

func (e configError) Error() string { return string(e) }

func errNil(msg string) error { return configError(msg) }
