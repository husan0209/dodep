//! Betting Engine Service
//!
//! Handles bet placement, settlement, and cashout.
//! Critical path — p99 target: < 5ms.

use std::net::SocketAddr;

use betting_engine::api;
use betting_engine::config::AppConfig;
use betting_engine::grpc::server::start_grpc;
use betting_engine::services::bet_service::BetService;
use betting_engine::services::settlement_service::SettlementService;
use betting_engine::state::AppState;
use tokio::signal;
use tracing::info;

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
        grpc_port = config.grpc_port,
        "Starting Betting Engine"
    );

    let db_pool = sqlx::postgres::PgPoolOptions::new()
        .max_connections(config.db_max_connections)
        .connect(&config.database_url)
        .await?;

    info!("Database connected");

    let grpc_port = config.grpc_port;
    let state = AppState::new(config.clone(), db_pool);

    let app = api::build(state.clone());

    let addr = SocketAddr::from(([0, 0, 0, 0], config.http_port));
    info!(%addr, "HTTP server listening");

    // gRPC runs alongside HTTP; share the state's services so both transports
    // observe the same pool and repository.
    let grpc_handle = tokio::spawn(async move {
        let repo = state.bet_repository();
        let result = start_grpc(
            grpc_port,
            BetService::new(repo.clone()),
            SettlementService::new(repo),
        )
        .await;
        if let Err(e) = result {
            tracing::error!(error = %e, "gRPC server exited");
        }
    });

    let listener = tokio::net::TcpListener::bind(addr).await?;
    axum::serve(listener, app)
        .with_graceful_shutdown(shutdown_signal())
        .await?;

    grpc_handle.abort();
    info!("Betting Engine stopped");
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
