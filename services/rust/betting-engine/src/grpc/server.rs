use std::net::SocketAddr;

use tracing::info;

use super::BettingEngineService;

use crate::services::bet_service::BetService;
use crate::services::settlement_service::SettlementService;

pub async fn start_grpc(
    port: u16,
    bet_service: BetService,
    settlement_service: SettlementService,
) -> Result<(), Box<dyn std::error::Error>> {
    let addr: SocketAddr = format!("0.0.0.0:{port}").parse()?;
    let service = BettingEngineService::new(bet_service, settlement_service).into_server();

    info!(%addr, "gRPC server listening");

    tonic::transport::Server::builder()
        .add_service(service)
        .serve(addr)
        .await?;

    Ok(())
}
