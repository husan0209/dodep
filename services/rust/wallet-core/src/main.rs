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
use std::net::SocketAddr;
use std::sync::Arc;
use std::time::Duration;
use tracing::info;

use wallet_core::api::grpc::WalletGrpcService;
use wallet_core::api::http::create_router;
use wallet_core::api::AppState;
use wallet_core::config::Config;
use wallet_core::infrastructure::{create_db_pool, create_redis_client, OutboxRepository};
use wallet_core::proto::wallet::v1::wallet_core_service_server::WalletCoreServiceServer;
use wallet_core::service::{create_producer, OutboxRelay};
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
    let state = Arc::new(AppState {
        config: config.clone(),
        db_pool: db_pool.clone(),
        redis_client,
        metrics: metrics_state,
    });

    // Start gRPC server
    let grpc_addr: SocketAddr = config.grpc.addr.parse()?;
    let grpc_state = state.clone();
    let grpc_handle = tokio::spawn(async move {
        tonic::transport::Server::builder()
            .add_service(WalletCoreServiceServer::new(WalletGrpcService::new(
                grpc_state,
            )))
            .serve(grpc_addr)
            .await
            .map_err(|e| anyhow::anyhow!(e))
    });
    info!("gRPC server listening on {}", config.grpc.addr);

    // Start HTTP server (health, metrics, admin)
    let http_addr: SocketAddr = config.http.addr.parse()?;
    let http_state = state.clone();
    let http_handle = tokio::spawn(async move {
        let app = create_router(http_state);
        let listener = tokio::net::TcpListener::bind(http_addr).await?;
        axum::serve(listener, app)
            .await
            .map_err(|e| anyhow::anyhow!(e))
    });
    info!("HTTP server listening on {}", config.http.addr);

    // Start metrics exporter
    let metrics_handle = if config.metrics.enabled {
        let metrics_addr: SocketAddr = config.metrics.addr.parse()?;
        Some(tokio::spawn(async move {
            metrics_exporter_prometheus::PrometheusBuilder::new()
                .with_http_listener(metrics_addr)
                .install()
        }))
    } else {
        None
    };

    // Start transactional outbox relay (DB outbox -> Redpanda).
    // Rows accumulate safely in Postgres when the broker is unreachable.
    let relay_handle = if config.kafka.enabled {
        let outbox_repo = Arc::new(OutboxRepository::new(db_pool.clone()));
        match create_producer(&config.kafka.brokers) {
            Ok(producer) => {
                let relay = OutboxRelay::new(
                    producer,
                    outbox_repo,
                    Duration::from_secs(config.kafka.poll_interval_secs),
                    config.kafka.batch_size,
                    config.kafka.max_retries,
                );
                info!(brokers = %config.kafka.brokers, "Outbox relay enabled");
                Some(tokio::spawn(async move { relay.run().await }))
            }
            Err(e) => {
                info!(error = %e, "Outbox relay disabled: producer creation failed (outbox rows will accumulate)");
                None
            }
        }
    } else {
        info!("Outbox relay disabled by config");
        None
    };

    // Wait for shutdown signal
    tokio::select! {
        result = grpc_handle => {
            result??;
        }
        result = http_handle => {
            result??;
        }
        _ = tokio::signal::ctrl_c() => {
            info!("Shutdown signal received");
        }
    }

    if let Some(handle) = relay_handle {
        handle.abort();
    }
    if let Some(handle) = metrics_handle {
        handle.abort();
    }

    info!("Wallet Core Service stopped");
    Ok(())
}
