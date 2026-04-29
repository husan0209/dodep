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

    /// Base64-encoded Ed25519 public key used to validate JWTs.
    #[serde(default)]
    pub auth_ed25519_public_key: String,

    /// Comma-separated list of allowed CORS origins.
    #[serde(default)]
    pub cors_allow_origins: String,

    /// Sportradar API key for live odds feed.
    #[serde(default)]
    pub sportradar_api_key: String,

    /// Sportradar live odds WebSocket URL.
    #[serde(default = "default_sportradar_ws_url")]
    pub sportradar_live_odds_ws: String,

    /// Sportradar REST API base URL.
    #[serde(default = "default_sportradar_api_url")]
    pub sportradar_api_url: String,

    /// Whether live Sportradar feed is enabled.
    #[serde(default)]
    pub sportradar_enabled: bool,

    /// Odds cache TTL in seconds for live events.
    #[serde(default = "default_odds_live_ttl")]
    pub odds_live_ttl_secs: u64,

    /// Odds cache TTL in seconds for pre-match events.
    #[serde(default = "default_odds_prematch_ttl")]
    pub odds_prematch_ttl_secs: u64,
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
fn default_sportradar_ws_url() -> String {
    "wss://oddsfeed.sportradar.com".into()
}
fn default_sportradar_api_url() -> String {
    "https://api.sportradar.com".into()
}
fn default_odds_live_ttl() -> u64 {
    5   // 5 seconds for live events
}
fn default_odds_prematch_ttl() -> u64 {
    30  // 30 seconds for pre-match events
}
