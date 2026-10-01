package config

import (
	"fmt"
	"os"
	"strconv"
)

// defaultJWTSecret is the placeholder shipped for local development. Tokens
// verified against it would be forgeable by anyone who has read the
// repository, so it is rejected outside development.
const defaultJWTSecret = "change-me-in-production"

// MinJWTSecretLength mirrors auth.MinHS256SecretLength.
const MinJWTSecretLength = 32

type Config struct {
	GRPCPort int
	HTTPPort string

	DatabaseURL string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	JWTSecretKey string
	// JWTEd25519PublicKey is the base64 Ed25519 public key used to verify
	// tokens when the auth service runs in EdDSA mode instead of HS256.
	JWTEd25519PublicKey string
	Env                 string
}

func Load() *Config {
	grpcPort, _ := strconv.Atoi(getEnv("GRPC_PORT", "9092"))

	return &Config{
		GRPCPort:            grpcPort,
		HTTPPort:            getEnv("PORT", "8082"),
		DatabaseURL:         getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/opus_casino?sslmode=disable"),
		RedisAddr:           getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:       getEnv("REDIS_PASSWORD", ""),
		RedisDB:             getIntEnv("REDIS_DB", 0),
		JWTSecretKey:        getEnv("JWT_SECRET_KEY", defaultJWTSecret),
		JWTEd25519PublicKey: getEnv("JWT_ED25519_PUBLIC_KEY", ""),
		Env:                 getEnv("APP_ENV", "development"),
	}
}

// Validate reports configuration that would leave the API either
// unauthenticated or trivially forgeable.
//
// This service authenticates every player request, so an unusable JWT key is
// not a degraded-but-usable state: it means either every request is rejected
// (the service looks healthy but does nothing) or, worse, tokens verify
// against the well-known placeholder. Both must stop the process at startup
// outside development, which is how the auth service already behaves.
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	if c.Env == "development" {
		return nil
	}

	// A secret that is present but weak is always a misconfiguration and is
	// reported precisely, so an operator knows to lengthen the key rather than
	// to look for a different setting. Checked before the "nothing usable"
	// branch, which would otherwise swallow it into a generic message.
	secretPresent := c.JWTSecretKey != "" && c.JWTSecretKey != defaultJWTSecret
	if secretPresent && len(c.JWTSecretKey) < MinJWTSecretLength {
		return fmt.Errorf("JWT_SECRET_KEY is too short: got %d bytes, need at least %d",
			len(c.JWTSecretKey), MinJWTSecretLength)
	}

	if !secretPresent && c.JWTEd25519PublicKey == "" {
		return fmt.Errorf(
			"JWT_SECRET_KEY must be set to a value of at least %d bytes (not the default placeholder) "+
				"when APP_ENV=%q, or JWT_ED25519_PUBLIC_KEY must be set for EdDSA verification",
			MinJWTSecretLength, c.Env)
	}
	return nil
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
