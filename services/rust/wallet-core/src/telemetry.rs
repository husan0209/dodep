//! Telemetry - tracing and metrics

use crate::config::Config;
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt, EnvFilter};

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
    // EnvFilter directives use `_` where the crate target uses `-`, and the
    // service name is configurable, so build the default from it rather than
    // hardcoding "wallet_core".
    let service = config.tracing.service_name.replace('-', "_");
    let default_directive = format!("info,{}=debug", service);

    let env_filter =
        EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new(default_directive));

    let subscriber = tracing_subscriber::registry()
        .with(env_filter)
        .with(tracing_subscriber::fmt::layer().json());

    subscriber.set_default()
}
