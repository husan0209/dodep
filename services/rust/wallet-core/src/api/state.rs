//! Application state

use redis::Client as RedisClient;
use sqlx::PgPool;
use std::sync::Arc;

use crate::config::Config;
use crate::telemetry::MetricsState;

/// Shared application state
pub struct AppState {
    pub config: Config,
    pub db_pool: PgPool,
    pub redis_client: RedisClient,
    pub metrics: Arc<MetricsState>,
}
