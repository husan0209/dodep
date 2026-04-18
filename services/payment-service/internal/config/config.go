package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the payment service
type Config struct {
	Environment string        `mapstructure:"environment"`
	Server      ServerConfig  `mapstructure:"server"`
	Database    DBConfig      `mapstructure:"database"`
	Redis       RedisConfig   `mapstructure:"redis"`
	Kafka       KafkaConfig   `mapstructure:"kafka"`
	NOWPayments NOWPayments   `mapstructure:"nowpayments"`
	Wallet      GRPCClient    `mapstructure:"wallet"`
	User        GRPCClient    `mapstructure:"user"`
	Tracing     TracingConfig `mapstructure:"tracing"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port           int `mapstructure:"port"`
	ReadTimeoutSec int `mapstructure:"read_timeout_sec"`
	WriteTimeoutSec int `mapstructure:"write_timeout_sec"`
	IdleTimeoutSec int `mapstructure:"idle_timeout_sec"`
}

// DBConfig holds PostgreSQL database configuration
type DBConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	User            string `mapstructure:"user"`
	Password        string `mapstructure:"password"`
	Database        string `mapstructure:"database"`
	SSLMode         string `mapstructure:"ssl_mode"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime_sec"`
}

// DSN returns the PostgreSQL connection string
func (c DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode,
	)
}

// RedisConfig holds DragonflyDB/Redis configuration
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

// Addr returns the Redis address
func (c RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// KafkaConfig holds Redpanda/Kafka configuration
type KafkaConfig struct {
	Brokers       []string `mapstructure:"brokers"`
	TopicPrefix   string   `mapstructure:"topic_prefix"`
	Acks          string   `mapstructure:"acks"`
	Compression   string   `mapstructure:"compression"`
	RetryMax      int      `mapstructure:"retry_max"`
	RetryBackoff  int      `mapstructure:"retry_backoff_ms"`
}

// NOWPayments holds NOWPayments API configuration
type NOWPayments struct {
	BaseURL     string        `mapstructure:"base_url"`
	APIKey      string        `mapstructure:"api_key"`
	IPNSecret   string        `mapstructure:"ipn_secret"`
	Timeout     time.Duration `mapstructure:"timeout"`
	MaxRetries  int           `mapstructure:"max_retries"`
	RetryDelay  time.Duration `mapstructure:"retry_delay"`
}

// GRPCClient holds gRPC client configuration
type GRPCClient struct {
	Address           string        `mapstructure:"address"`
	Timeout           time.Duration `mapstructure:"timeout"`
	EnableTLS         bool          `mapstructure:"enable_tls"`
	MaxRecvMsgSize    int           `mapstructure:"max_recv_msg_size"`
	MaxSendMsgSize    int           `mapstructure:"max_send_msg_size"`
}

// TracingConfig holds OpenTelemetry tracing configuration
type TracingConfig struct {
	Enabled      bool    `mapstructure:"enabled"`
	Endpoint     string  `mapstructure:"endpoint"`
	Protocol     string  `mapstructure:"protocol"` // "grpc" or "http"
	SampleRate   float64 `mapstructure:"sample_rate"`
	Insecure     bool    `mapstructure:"insecure"`
	ServiceName  string  `mapstructure:"service_name"`
}

// Load reads configuration from file and environment variables
func Load() (*Config, error) {
	return LoadWithPath("")
}

// LoadWithPath reads configuration from a specific path
func LoadWithPath(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Read config file
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("default")
		v.SetConfigType("yaml")
		v.AddConfigPath("./config")
		v.AddConfigPath("/app/config")
	}

	// Environment variable overrides
	v.AutomaticEnv()
	v.SetEnvPrefix("PAYMENT")

	// Read config
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	// Load environment-specific config
	env := v.GetString("environment")
	if env != "" && env != "development" {
		v.SetConfigName(env)
		if err := v.MergeInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return nil, fmt.Errorf("merge %s config: %w", env, err)
			}
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("environment", "development")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout_sec", 10)
	v.SetDefault("server.write_timeout_sec", 10)
	v.SetDefault("server.idle_timeout_sec", 60)

	// Database defaults
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.password", "postgres")
	v.SetDefault("database.database", "payment_service")
	v.SetDefault("database.ssl_mode", "disable")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime_sec", 300)

	// Redis defaults
	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.pool_size", 100)

	// Kafka defaults
	v.SetDefault("kafka.brokers", []string{"localhost:9092"})
	v.SetDefault("kafka.topic_prefix", "payments")
	v.SetDefault("kafka.acks", "all")
	v.SetDefault("kafka.compression", "snappy")
	v.SetDefault("kafka.retry_max", 3)
	v.SetDefault("kafka.retry_backoff_ms", 100)

	// NOWPayments defaults
	v.SetDefault("nowpayments.base_url", "https://api.nowpayments.io/v1")
	v.SetDefault("nowpayments.timeout", 30*time.Second)
	v.SetDefault("nowpayments.max_retries", 3)
	v.SetDefault("nowpayments.retry_delay", 1*time.Second)

	// Wallet service defaults
	v.SetDefault("wallet.address", "localhost:50051")
	v.SetDefault("wallet.timeout", 5*time.Second)
	v.SetDefault("wallet.max_recv_msg_size", 4*1024*1024)
	v.SetDefault("wallet.max_send_msg_size", 4*1024*1024)

	// User service defaults
	v.SetDefault("user.address", "localhost:50052")
	v.SetDefault("user.timeout", 5*time.Second)
	v.SetDefault("user.max_recv_msg_size", 4*1024*1024)
	v.SetDefault("user.max_send_msg_size", 4*1024*1024)

	// Tracing defaults
	v.SetDefault("tracing.enabled", true)
	v.SetDefault("tracing.endpoint", "localhost:4317")
	v.SetDefault("tracing.protocol", "grpc")
	v.SetDefault("tracing.sample_rate", 1.0)
	v.SetDefault("tracing.insecure", true)
	v.SetDefault("tracing.service_name", "payment-service")
}
