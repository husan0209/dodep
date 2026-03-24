pub mod server;

use std::pin::Pin;
use std::sync::Arc;

use tokio::sync::mpsc;
use tonic::{Request, Response, Status, Streaming};
use tracing::{info, warn};

use crate::domain::bet::*;
use crate::services::bet_service::BetService;
use crate::services::settlement_service::SettlementService;

// Include generated protobuf code
// In production: tonic::include_proto!("opuscasino.betting");
// For now, we define the service trait manually

pub mod opuscasino {
    pub mod betting {
        tonic::include_proto!("opuscasino.betting");
    }
}

pub use opuscasino::betting::*;

#[derive(Clone)]
pub struct BettingEngineService {
    bet_service: BetService,
    settlement_service: SettlementService,
}

impl BettingEngineService {
    pub fn new(bet_service: BetService, settlement_service: SettlementService) -> Self {
        Self {
            bet_service,
            settlement_service,
        }
    }
}

#[tonic::async_trait]
impl betting_engine_server::BettingEngine for BettingEngineService {
    async fn place_bet(
        &self,
        request: Request<PlaceBetRequest>,
    ) -> Result<Response<PlaceBetResponse>, Status> {
        let req = request.into_inner();
        let user_id_str = req
            .user_id
            .as_ref()
            .map(|u| u.value.clone())
            .unwrap_or_default();
        let user_id: i64 = user_id_str.parse().map_err(|_| {
            Status::invalid_argument("Invalid user_id")
        })?;

        let stake_str = req
            .stake
            .as_ref()
            .map(|a| a.value.clone())
            .unwrap_or_else(|| "0".into());
        let stake: rust_decimal::Decimal = stake_str.parse().map_err(|_| {
            Status::invalid_argument("Invalid stake amount")
        })?;

        let odds: rust_decimal::Decimal = req.odds.parse().map_err(|_| {
            Status::invalid_argument("Invalid odds")
        })?;

        let currency = req
            .stake
            .as_ref()
            .map(|a| a.currency.clone())
            .unwrap_or_else(|| "USD".into());

        let selection = crate::domain::bet::SelectionRequest {
            event_id: req.event_id.parse().unwrap_or(0),
            market_id: req.market_id.parse().unwrap_or(0),
            outcome_id: req.selection_id.parse().unwrap_or(0),
            odds,
        };

        let place_req = PlaceBetRequest {
            bet_type: BetType::Single,
            selections: vec![selection],
            stake,
            currency_code: currency,
            idempotency_key: uuid::Uuid::new_v4(),
            accept_odds_changes: AcceptOddsChanges::default(),
            ip_address: None,
            device_fingerprint: None,
        };

        match self.bet_service.place_bet(UserId(user_id), place_req).await {
            Ok(bet) => Ok(Response::new(PlaceBetResponse {
                bet: Some(convert_bet_to_proto(&bet)),
                error_code: String::new(),
                error_message: String::new(),
            })),
            Err(e) => Ok(Response::new(PlaceBetResponse {
                bet: None,
                error_code: "BET_ERROR".into(),
                error_message: e.to_string(),
            })),
        }
    }

    async fn cancel_bet(
        &self,
        request: Request<CancelBetRequest>,
    ) -> Result<Response<CancelBetResponse>, Status> {
        let req = request.into_inner();
        let bet_id_str = req.bet_id.as_ref().map(|b| b.value.clone()).unwrap_or_default();
        let bet_id: i64 = bet_id_str.parse().map_err(|_| Status::invalid_argument("Invalid bet_id"))?;

        match self.settlement_service.void_bet(BetId(bet_id)).await {
            Ok(_) => Ok(Response::new(CancelBetResponse {
                success: true,
                error_code: String::new(),
                error_message: String::new(),
            })),
            Err(e) => Ok(Response::new(CancelBetResponse {
                success: false,
                error_code: "CANCEL_ERROR".into(),
                error_message: e.to_string(),
            })),
        }
    }

    async fn get_bet(
        &self,
        request: Request<GetBetRequest>,
    ) -> Result<Response<GetBetResponse>, Status> {
        let req = request.into_inner();
        let bet_id_str = req.bet_id.as_ref().map(|b| b.value.clone()).unwrap_or_default();
        let bet_id: i64 = bet_id_str.parse().map_err(|_| Status::invalid_argument("Invalid bet_id"))?;

        // gRPC: get without user_id check (internal service call)
        // For now return error — needs user context
        Err(Status::unimplemented("Use REST API for bet queries with user context"))
    }

    async fn get_user_bets(
        &self,
        request: Request<GetUserBetsRequest>,
    ) -> Result<Response<GetUserBetsResponse>, Status> {
        let req = request.into_inner();
        let user_id_str = req.user_id.as_ref().map(|u| u.value.clone()).unwrap_or_default();
        let user_id: i64 = user_id_str.parse().map_err(|_| Status::invalid_argument("Invalid user_id"))?;

        let limit = if req.limit > 0 { req.limit as i64 } else { 20 };

        match self
            .bet_service
            .get_history(UserId(user_id), limit, None, None)
            .await
        {
            Ok(result) => {
                let bets: Vec<opuscasino::Bet> = result
                    .data
                    .iter()
                    .map(convert_bet_resp_to_proto)
                    .collect();

                Ok(Response::new(GetUserBetsResponse {
                    bets,
                    total: result.total as i32,
                }))
            }
            Err(e) => Err(Status::internal(e.to_string())),
        }
    }

    async fn settle_bet(
        &self,
        request: Request<SettleBetRequest>,
    ) -> Result<Response<SettleBetResponse>, Status> {
        let req = request.into_inner();
        let bet_id_str = req.bet_id.as_ref().map(|b| b.value.clone()).unwrap_or_default();
        let bet_id: i64 = bet_id_str.parse().map_err(|_| Status::invalid_argument("Invalid bet_id"))?;

        let actual_win_str = req
            .actual_win
            .as_ref()
            .map(|a| a.value.clone())
            .unwrap_or_else(|| "0".into());
        let actual_win: rust_decimal::Decimal = actual_win_str.parse().map_err(|_| {
            Status::invalid_argument("Invalid actual_win amount")
        })?;

        match self
            .settlement_service
            .settle_bet(BetId(bet_id), &req.result, actual_win)
            .await
        {
            Ok(_) => Ok(Response::new(SettleBetResponse {
                success: true,
                error_code: String::new(),
                error_message: String::new(),
            })),
            Err(e) => Ok(Response::new(SettleBetResponse {
                success: false,
                error_code: "SETTLE_ERROR".into(),
                error_message: e.to_string(),
            })),
        }
    }

    type StreamOddsStream = Pin<Box<dyn tokio::stream::Stream<Item = Result<OddsUpdate, Status>> + Send>>;

    async fn stream_odds(
        &self,
        _request: Request<OddsStreamRequest>,
    ) -> Result<Response<Self::StreamOddsStream>, Status> {
        Err(Status::unimplemented("Odds streaming handled by WebSocket gateway"))
    }
}

fn convert_bet_to_proto(bet: &Bet) -> opuscasino::Bet {
    opuscasino::Bet {
        id: Some(opuscasino::BetId {
            value: bet.id.0.to_string(),
        }),
        user_id: Some(opuscasino::UserId {
            value: bet.user_id.0.to_string(),
        }),
        status: opuscasino::BetStatus::BetStatusPending as i32,
        stake: Some(opuscasino::Amount {
            value: bet.stake.to_string(),
            currency: bet.currency_code.clone(),
        }),
        potential_win: Some(opuscasino::Amount {
            value: bet.potential_win.to_string(),
            currency: bet.currency_code.clone(),
        }),
        actual_win: Some(opuscasino::Amount {
            value: bet.actual_win.to_string(),
            currency: bet.currency_code.clone(),
        }),
        odds: bet.combined_odds.to_string(),
        placed_at: Some(opuscasino::Timestamp {
            unix_ms: bet.placed_at.timestamp_millis(),
        }),
        settled_at: bet.settled_at.map(|t| opuscasino::Timestamp {
            unix_ms: t.timestamp_millis(),
        }),
        ..Default::default()
    }
}

fn convert_bet_resp_to_proto(bet: &BetResponse) -> opuscasino::Bet {
    opuscasino::Bet {
        id: Some(opuscasino::BetId {
            value: bet.bet_id.to_string(),
        }),
        user_id: Some(opuscasino::UserId {
            value: bet.user_id.to_string(),
        }),
        odds: bet.odds.clone(),
        ..Default::default()
    }
}
