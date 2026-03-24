package config

import (
	"os"
	"strconv"
	"strings"
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

	// Redpanda/Kafka
	RedpandaEnabled bool
	RedpandaBrokers []string

	// Email
	EmailEnabled    bool
	EmailProvider   string
	EmailFrom       string
	EmailAPIKey     string
	EmailAPISecret  string

	// SMS
	SMSEnabled   bool
	SMSProvider  string
	SMSAPIKey    string
	SMSAPISecret string

	// Push
	PushEnabled      bool
	PushFirebaseKey  string

	// Environment
	Env string
}

func Load() *Config {
	grpcPort, _ := strconv.Atoi(getEnv("GRPC_PORT", "9091"))

	// Parse Redpanda brokers
	redpandaBrokersStr := getEnv("REDPANDA_BROKERS", "localhost:9092")
	redpandaBrokers := strings.Split(redpandaBrokersStr, ",")

	return &Config{
		GRPCPort: grpcPort,
		HTTPPort: getEnv("PORT", "8081"),

		// Database
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/opus_casino?sslmode=disable"),

		// Redis
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getIntEnv("REDIS_DB", 0),

		// Redpanda
		RedpandaEnabled: getEnv("REDPANDA_ENABLED", "true") == "true",
		RedpandaBrokers: redpandaBrokers,

		// Email
		EmailEnabled:   getEnv("EMAIL_ENABLED", "true") == "true",
		EmailProvider:  getEnv("EMAIL_PROVIDER", "sendgrid"),
		EmailFrom:      getEnv("EMAIL_FROM", "noreply@opuscasino.com"),
		EmailAPIKey:    getEnv("EMAIL_API_KEY", ""),
		EmailAPISecret: getEnv("EMAIL_API_SECRET", ""),

		// SMS
		SMSEnabled:   getEnv("SMS_ENABLED", "false") == "true",
		SMSProvider:  getEnv("SMS_PROVIDER", "twilio"),
		SMSAPIKey:    getEnv("SMS_API_KEY", ""),
		SMSAPISecret: getEnv("SMS_API_SECRET", ""),

		// Push
		PushEnabled:      getEnv("PUSH_ENABLED", "true") == "true",
		PushFirebaseKey:  getEnv("PUSH_FIREBASE_KEY", ""),

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
