//! Telemetry - tracing and metrics

use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt, EnvFilter};
use crate::config::Config;

/// Metrics state
pub struct MetricsState {
    // Add metrics counters here
}

impl MetricsState {
    pub fn new() -> Self {
        Self {}
    }
}

impl Default for MetricsState {
    fn default() -> Self {
        Self::new()
    }
}

/// Initialize tracing
pub fn init_tracing(config: &Config) -> tracing::subscriber::DefaultGuard {
    let env_filter = EnvFilter::try_from_default_env()
        .unwrap_or_else(|_| EnvFilter::new("info,wallet_core=debug"));
    
    let subscriber = tracing_subscriber::registry()
        .with(env_filter)
        .with(tracing_subscriber::fmt::layer().json());
    
    subscriber.set_default()
}
