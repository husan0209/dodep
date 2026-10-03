//! Wallet Core Service — Opus Casino
//!
//! Financial operations service managing:
//! - User wallets and balances
//! - Transactions (deposits, withdrawals, bets, wins)
//! - Fund locking/unlocking for pending bets
//! - Inter-wallet transfers
//! - Idempotent financial operations

#![warn(rust_2018_idioms)]
#![warn(clippy::all)]

use anyhow::Result;
use std::sync::Arc;
use tracing::info;

// Use the library crate rather than re-declaring every module, so the proto
// types (generated into `wallet_core::proto`) resolve consistently.
use wallet_core::api;
use wallet_core::api::grpc::WalletGrpcService;
use wallet_core::api::http::create_router;
use wallet_core::config::Config;
use wallet_core::infrastructure::{create_db_pool, create_redis_client};
use wallet_core::telemetry::{init_tracing, MetricsState};

#[tokio::main]
async fn main() -> Result<()> {
    // Load configuration
    let config = Config::load()?;

    // Initialize telemetry
    let _telemetry = init_tracing(&config);
    info!("Starting Wallet Core Service");
    info!("Environment: {}", config.app.env);

    // Initialize database pool
    let db_pool = create_db_pool(&config.database).await?;
    info!("Database connection pool created");

    // Initialize Redis client
    let redis_client = create_redis_client(&config.redis).await?;
    info!("Redis client created");

    // Create metrics state
    let metrics_state = Arc::new(MetricsState::new());

    // Create shared state
    let state = Arc::new(api::AppState {
        config: config.clone(),
        db_pool,
        redis_client,
        metrics: metrics_state,
    });

    // Start gRPC server
    let grpc_addr = config.grpc.addr.parse::<std::net::SocketAddr>()?;
    let grpc_state = state.clone();
    let grpc_handle =
        tokio::spawn(async move { WalletGrpcService::serve(grpc_addr, grpc_state).await });
    info!("gRPC server listening on {}", config.grpc.addr);

    // Start HTTP server (health, metrics, admin)
    // Annotate the type: the spawned closure's error type is inferred, so
    // `parse()` would otherwise default to `()`.
    let http_addr = config.http.addr.parse::<std::net::SocketAddr>()?;
    let http_state = state.clone();
    let http_handle = tokio::spawn(async move {
        let app = create_router(http_state);
        let listener = tokio::net::TcpListener::bind(http_addr).await?;
        axum::serve(listener, app).await
    });
    info!("HTTP server listening on {}", config.http.addr);

    // Start metrics exporter
    let metrics_handle = if config.metrics.enabled {
        // Resolve the address before spawning so a bad value is reported by
        // main (which returns `anyhow::Result`) instead of inside the task.
        let metrics_addr = config.metrics.addr.parse::<std::net::SocketAddr>()?;
        Some(tokio::spawn(async move {
            metrics_exporter_prometheus::PrometheusBuilder::new()
                .with_http_listener(metrics_addr)
                .install()
        }))
    } else {
        None
    };

    // Wait for shutdown signal
    tokio::select! {
        result = grpc_handle => {
            // The boxed error is unsized, so it cannot flow through `?`
            // directly; convert it to a message first.
            let inner = result?;
            if let Err(e) = inner {
                anyhow::bail!("gRPC server failed: {e}");
            }
        }
        result = http_handle => {
            result??;
        }
        _ = tokio::signal::ctrl_c() => {
            info!("Shutdown signal received");
        }
    }

    if let Some(handle) = metrics_handle {
        handle.abort();
    }

    info!("Wallet Core Service stopped");
    Ok(())
}
