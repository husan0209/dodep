//! Configuration for Wallet Core Service

use serde::Deserialize;
use std::env;

#[derive(Debug, Clone, Deserialize)]
pub struct Config {
    pub app: AppConfig,
    pub database: DatabaseConfig,
    pub redis: RedisConfig,
    pub grpc: GrpcConfig,
    pub http: HttpConfig,
    pub metrics: MetricsConfig,
    pub tracing: TracingConfig,
}

#[derive(Debug, Clone, Deserialize)]
pub struct AppConfig {
    pub env: String,
    pub name: String,
    pub version: String,
}

#[derive(Debug, Clone, Deserialize)]
pub struct DatabaseConfig {
    pub host: String,
    pub port: u16,
    pub database: String,
    pub username: String,
    pub password: String,
    pub max_connections: u32,
    pub min_connections: u32,
    pub connect_timeout_secs: u64,
    pub idle_timeout_secs: u64,
}

impl DatabaseConfig {
    pub fn connection_string(&self) -> String {
        format!(
            "postgres://{}:{}@{}:{}/{}",
            self.username, self.password, self.host, self.port, self.database
        )
    }
}

#[derive(Debug, Clone, Deserialize)]
pub struct RedisConfig {
    pub host: String,
    pub port: u16,
    pub password: Option<String>,
    pub db: u8,
    pub max_connections: u32,
}

impl RedisConfig {
    pub fn connection_string(&self) -> String {
        if let Some(password) = &self.password {
            format!("redis://:{}@{}:{}/{}", password, self.host, self.port, self.db)
        } else {
            format!("redis://{}:{}/{}", self.host, self.port, self.db)
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
pub struct GrpcConfig {
    pub addr: String,
    pub max_message_size_mb: usize,
}

#[derive(Debug, Clone, Deserialize)]
pub struct HttpConfig {
    pub addr: String,
}

#[derive(Debug, Clone, Deserialize)]
pub struct MetricsConfig {
    pub enabled: bool,
    pub addr: String,
}

#[derive(Debug, Clone, Deserialize)]
pub struct TracingConfig {
    pub enabled: bool,
    pub otlp_endpoint: String,
    pub service_name: String,
}

impl Config {
    pub fn load() -> Result<Self, config::ConfigError> {
        // Load from .env file
        let _ = dotenvy::dotenv();
        
        let config_builder = config::Config::builder()
            // Start with defaults
            .set_default("app.env", "development")?
            .set_default("app.name", "wallet-core")?
            .set_default("app.version", "0.1.0")?
            
            // Database defaults
            .set_default("database.host", "localhost")?
            .set_default("database.port", 5432)?
            .set_default("database.database", "opus_casino")?
            .set_default("database.username", "postgres")?
            .set_default("database.password", "postgres")?
            .set_default("database.max_connections", 20)?
            .set_default("database.min_connections", 5)?
            .set_default("database.connect_timeout_secs", 30)?
            .set_default("database.idle_timeout_secs", 600)?
            
            // Redis defaults
            .set_default("redis.host", "localhost")?
            .set_default("redis.port", 6379)?
            .set_default("redis.db", 0)?
            .set_default("redis.max_connections", 10)?
            
            // gRPC defaults
            .set_default("grpc.addr", "0.0.0.0:50053")?
            .set_default("grpc.max_message_size_mb", 4)?
            
            // HTTP defaults
            .set_default("http.addr", "0.0.0.0:3003")?
            
            // Metrics defaults
            .set_default("metrics.enabled", true)?
            .set_default("metrics.addr", "0.0.0.0:9003")?
            
            // Tracing defaults
            .set_default("tracing.enabled", true)?
            .set_default("tracing.otlp_endpoint", "http://localhost:4317")?
            .set_default("tracing.service_name", "wallet-core")?
            
            // Override with environment variables
            .add_source(config::Environment::with_prefix("WALLET").separator("__"))
            .add_source(config::Environment::with_prefix("APP").separator("__"))
            .add_source(config::Environment::with_prefix("DATABASE").separator("__"))
            .add_source(config::Environment::with_prefix("REDIS").separator("__"))
            .add_source(config::Environment::with_prefix("GRPC").separator("__"))
            .add_source(config::Environment::with_prefix("HTTP").separator("__"))
            .add_source(config::Environment::with_prefix("METRICS").separator("__"))
            .add_source(config::Environment::with_prefix("TRACING").separator("__"));
        
        config_builder.build()?.try_deserialize()
    }
    
    pub fn is_development(&self) -> bool {
        self.app.env == "development"
    }
    
    pub fn is_production(&self) -> bool {
        self.app.env == "production"
    }
}
