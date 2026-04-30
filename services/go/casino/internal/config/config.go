package config

import (
	"os"
	"strconv"
	"time"

	"github.com/opus-casino/casino/internal/provider/amatic"
	"github.com/opus-casino/casino/internal/provider/amusnet"
	"github.com/opus-casino/casino/internal/provider/pgsoft"
	"github.com/opus-casino/casino/internal/provider/pragmatic"
)

// Config holds all casino service configuration.
type Config struct {
	// Server
	GRPCPort int
	HTTPPort string

	// Database
	DatabaseURL string

	// Redis / DragonflyDB
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// gRPC upstreams
	WalletGRPCAddr string
	UserGRPCAddr   string

	// JWT Ed25519 public key (base64-encoded DER)
	JWTPublicKey string

	// Environment
	Env string

	// Slot Providers
	Pragmatic pragmatic.Config
	PGSoft    pgsoft.Config
	Amatic    amatic.Config
	Amusnet   amusnet.Config
}

// Load reads configuration from environment variables.
func Load() *Config {
	grpcPort, _ := strconv.Atoi(getEnv("GRPC_PORT", "50057"))
	replayWindow, _ := strconv.Atoi(getEnv("PRAGMATIC_REPLAY_WINDOW_SEC", "180"))

	return &Config{
		GRPCPort: grpcPort,
		HTTPPort: getEnv("PORT", "8086"),

		DatabaseURL:    getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/opus_casino?sslmode=disable"),
		RedisAddr:      getEnv("REDIS_HOST", "localhost") + ":" + getEnv("REDIS_PORT", "6379"),
		RedisPassword:  getEnv("REDIS_PASSWORD", ""),
		RedisDB:        getIntEnv("REDIS_DB", 0),
		WalletGRPCAddr: getEnv("WALLET_GRPC_ADDR", "localhost:50053"),
		UserGRPCAddr:   getEnv("USER_GRPC_ADDR", "localhost:50052"),
		JWTPublicKey:   getEnv("JWT_ED25519_PUBLIC_KEY", ""),
		Env:            getEnv("APP_ENV", "development"),

		Pragmatic: pragmatic.Config{
			Enabled:         getBoolEnv("PRAGMATIC_ENABLED", false),
			AgentID:         getEnv("PRAGMATIC_AGENT_ID", ""),
			SecretKey:       getEnv("PRAGMATIC_SECRET_KEY", ""),
			APIURL:          getEnv("PRAGMATIC_API_URL", "https://api.prerelease-env.biz"),
			HTTPTimeout:     10 * time.Second,
			ReplayWindowSec: replayWindow,
		},

		PGSoft: pgsoft.Config{
			Enabled:         getBoolEnv("PGSOFT_ENABLED", false),
			OperatorToken:   getEnv("PGSOFT_OPERATOR_TOKEN", ""),
			SecretKey:       getEnv("PGSOFT_SECRET_KEY", ""),
			APIURL:          getEnv("PGSOFT_API_URL", "https://api.pgsoft-games.com"),
			HTTPTimeout:     10 * time.Second,
			ReplayWindowSec: getIntEnv("PGSOFT_REPLAY_WINDOW_SEC", replayWindow),
		},

		Amatic: amatic.Config{
			Enabled:         getBoolEnv("AMATIC_ENABLED", false),
			OperatorID:      getEnv("AMATIC_OPERATOR_ID", ""),
			APIPassword:     getEnv("AMATIC_API_PASSWORD", ""),
			SecretKey:       getEnv("AMATIC_SECRET_KEY", ""),
			APIURL:          getEnv("AMATIC_API_URL", "https://api.amatic-industries.com"),
			HTTPTimeout:     10 * time.Second,
			ReplayWindowSec: getIntEnv("AMATIC_REPLAY_WINDOW_SEC", replayWindow),
		},

		Amusnet: amusnet.Config{
			Enabled:         getBoolEnv("AMUSNET_ENABLED", false),
			OperatorID:      getEnv("AMUSNET_OPERATOR_ID", ""),
			SecretKey:       getEnv("AMUSNET_SECRET_KEY", ""),
			APIURL:          getEnv("AMUSNET_API_URL", "https://api.amusnet.com"),
			ClientCertPath:  getEnv("AMUSNET_CLIENT_CERT_PATH", ""),
			ClientKeyPath:   getEnv("AMUSNET_CLIENT_KEY_PATH", ""),
			HTTPTimeout:     15 * time.Second,
			ReplayWindowSec: getIntEnv("AMUSNET_REPLAY_WINDOW_SEC", replayWindow),
		},
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

func getBoolEnv(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}
