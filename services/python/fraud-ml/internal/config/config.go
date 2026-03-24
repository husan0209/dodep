package config

import (
	"os"
	"strconv"
)

type Config struct {
	// Server
	HTTPPort int

	// Database
	ClickHouseURL string
	PostgresURL   string

	// Redpanda
	RedpandaBrokers []string

	// ML Models
	ModelPath       string
	ModelUpdateFreq int // hours

	// Monitoring
	PrometheusPort int

	// Environment
	Env string
}

func Load() *Config {
	httpPort, _ := strconv.Atoi(getEnv("HTTP_PORT", "8000"))
	promPort, _ := strconv.Atoi(getEnv("PROMETHEUS_PORT", "9090"))
	modelUpdateFreq, _ := strconv.Atoi(getEnv("MODEL_UPDATE_FREQ", "24"))

	return &Config{
		HTTPPort: httpPort,

		// Database
		ClickHouseURL: getEnv("CLICKHOUSE_URL", "clickhouse://localhost:9000/opus_casino"),
		PostgresURL:   getEnv("DATABASE_URL", "postgres://localhost:5432/opus_casino"),

		// Redpanda
		RedpandaBrokers: []string{getEnv("REDPANDA_BROKERS", "localhost:9092")},

		// ML Models
		ModelPath:       getEnv("MODEL_PATH", "/app/models"),
		ModelUpdateFreq: modelUpdateFreq,

		// Monitoring
		PrometheusPort: promPort,

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
