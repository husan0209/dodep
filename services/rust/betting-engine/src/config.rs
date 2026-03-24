use serde::Deserialize;

#[derive(Debug, Clone, Deserialize)]
pub struct AppConfig {
    #[serde(default = "default_service_name")]
    pub service_name: String,

    #[serde(default = "default_http_port")]
    pub http_port: u16,

    #[serde(default = "default_grpc_port")]
    pub grpc_port: u16,

    pub database_url: String,

    #[serde(default = "default_db_max_conn")]
    pub db_max_connections: u32,

    pub cache_url: String,

    pub redpanda_brokers: String,
}

impl AppConfig {
    pub fn load() -> anyhow::Result<Self> {
        dotenvy::dotenv().ok();

        let config = config::Config::builder()
            .add_source(config::Environment::with_prefix("APP").separator("__"))
            .build()?;

        let app_config: Self = config.try_deserialize()?;
        Ok(app_config)
    }
}

fn default_service_name() -> String {
    "betting-engine".into()
}
fn default_http_port() -> u16 {
    8080
}
fn default_grpc_port() -> u16 {
    9090
}
fn default_db_max_conn() -> u32 {
    30
}
