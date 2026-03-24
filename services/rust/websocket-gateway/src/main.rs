//! WebSocket Gateway - Real-time communication
//!
//! WebSocket gateway for Opus Casino platform.
//! Handles real-time odds, bet status, and balance updates.

use std::net::SocketAddr;

use tokio::signal;
use tracing::info;

mod api;
mod config;
mod domain;
mod errors;
mod infrastructure;
mod state;

use config::AppConfig;
use infrastructure::kafka_consumer::KafkaBroadcaster;
use state::AppState;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    tracing_subscriber::fmt()
        .with_target(false)
        .with_level(true)
        .json()
        .init();

    let config = AppConfig::load()?;

    info!(
        service = %config.service_name,
        http_port = config.http_port,
        max_connections = config.max_connections,
        "Starting WebSocket Gateway"
    );

    let kafka_broadcaster = KafkaBroadcaster::new(&config.redpanda_brokers)?;

    let state = AppState::new(config.clone(), kafka_broadcaster);

    let app = api::build(state);

    let addr = SocketAddr::from(([0, 0, 0, 0], config.http_port));
    info!(%addr, "HTTP server listening");

    let listener = tokio::net::TcpListener::bind(addr).await?;
    axum::serve(listener, app)
        .with_graceful_shutdown(shutdown_signal())
        .await?;

    info!("WebSocket Gateway stopped");
    Ok(())
}

async fn shutdown_signal() {
    let ctrl_c = async {
        signal::ctrl_c()
            .await
            .expect("failed to install Ctrl+C handler");
    };

    #[cfg(unix)]
    let terminate = async {
        signal::unix::signal(signal::unix::SignalKind::terminate())
            .expect("failed to install signal handler")
            .recv()
            .await;
    };

    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();

    tokio::select! {
        () = ctrl_c => info!("Received Ctrl+C"),
        () = terminate => info!("Received SIGTERM"),
    }
}
