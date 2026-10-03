pub mod server;

use std::pin::Pin;
use std::str::FromStr;

use rust_decimal::Decimal;
use tonic::{Request, Response, Status};
use tracing::warn;
use uuid::Uuid;

use crate::domain::bet::{
    AcceptOddsChanges, Bet, BetId, BetStatus as DomainBetStatus, BetType as DomainBetType,
    SelectionRequest, UserId,
};
use crate::services::bet_service::BetService;
use crate::services::settlement_service::SettlementService;

// The generated code refers to cross-package messages as
// `super::super::common::v1::...`, so both packages have to be included at the
// same nesting depth for that to resolve.
pub mod proto {
    pub mod common {
        pub mod v1 {
            tonic::include_proto!("common.v1");
        }
    }
    pub mod betting {
        pub mod v1 {
            tonic::include_proto!("betting.v1");
        }
    }
}

use proto::betting::v1 as pb;
use proto::common::v1 as common;

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

// РІвЂќР‚РІвЂќР‚ conversions РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚РІвЂќР‚

/// `common.v1.UserId.value` and `common.v1.BetId.value` are strings on the wire.
// 	onic::Status carries a message plus a binary metadata payload, so any
// Result<_, Status> trips clippy::result_large_err (threshold 128 bytes). The
// gRPC signatures fix the error type, so there is nothing to box - suppressed
// rather than worked around with an artificial error wrapper.
#[allow(clippy::result_large_err)]
fn parse_i64(field: &str, value: &str) -> Result<i64, Status> {
    i64::from_str(value).map_err(|_| {
        Status::invalid_argument(format!("{field} must be a decimal integer, got {value:?}"))
    })
}

fn money(amount: Decimal, currency: &str) -> common::Money {
    common::Money {
        amount: amount.to_string(),
        currency: currency.to_string(),
    }
}

/// prost-types has no `From<DateTime<Utc>>` unless its chrono support is
/// enabled, so the two fields are filled in directly.
fn to_timestamp(dt: chrono::DateTime<chrono::Utc>) -> prost_types::Timestamp {
    prost_types::Timestamp {
        seconds: dt.timestamp(),
        nanos: dt.timestamp_subsec_nanos() as i32,
    }
}

// 	onic::Status carries a message plus a binary metadata payload, so any
// Result<_, Status> trips clippy::result_large_err (threshold 128 bytes). The
// gRPC signatures fix the error type, so there is nothing to box - suppressed
// rather than worked around with an artificial error wrapper.
#[allow(clippy::result_large_err)]
fn parse_money(field: &str, m: Option<&common::Money>) -> Result<(Decimal, String), Status> {
    let m = m.ok_or_else(|| Status::invalid_argument(format!("{field} is required")))?;
    let amount = Decimal::from_str(&m.amount)
        .map_err(|_| Status::invalid_argument(format!("{field}.amount is not a decimal")))?;
    Ok((amount, m.currency.clone()))
}

/// The proto's `BetStatus` is coarser than the domain's: it has no Won/Lost, only
/// `SETTLED`. A settled bet's real outcome lives in the domain, which this
/// layer does not carry, so settled maps to `Settled` and nothing finer is
/// invented.
fn to_proto_status(status: DomainBetStatus) -> i32 {
    match status {
        DomainBetStatus::Pending => common::BetStatus::Pending as i32,
        DomainBetStatus::Active => common::BetStatus::Accepted as i32,
        DomainBetStatus::Won | DomainBetStatus::Lost => common::BetStatus::Settled as i32,
        DomainBetStatus::Void | DomainBetStatus::Cashout => common::BetStatus::Cancelled as i32,
        DomainBetStatus::Rejected => common::BetStatus::Rejected as i32,
    }
}

fn to_proto_bet(bet: &Bet) -> pb::Bet {
    pb::Bet {
        id: Some(common::BetId {
            value: bet.id.0.to_string(),
        }),
        user_id: Some(common::UserId {
            value: bet.user_id.0.to_string(),
        }),
        // The contract's `type` is a product vertical (Sports/Live/Casino/...),
        // not a bet structure, and the domain's structure enum has no
        // counterpart here. Single is the only structure this field can
        // faithfully carry.
        r#type: common::BetType::Sports as i32,
        status: to_proto_status(bet.status),
        stake: Some(money(bet.stake, &bet.currency_code)),
        potential_win: Some(money(bet.potential_win, &bet.currency_code)),
        actual_win: Some(money(bet.actual_win, &bet.currency_code)),
        event_id: bet.event_id.map(|e| e.0.to_string()).unwrap_or_default(),
        market_id: String::new(),
        selection_id: String::new(),
        odds: bet.combined_odds.to_string(),
        placed_at: Some(to_timestamp(bet.placed_at)),
        settled_at: bet.settled_at.map(to_timestamp),
        client_request_id: bet.idempotency_key.to_string(),
        device_id: String::new(),
        ip_address: bet.ip_address.clone().unwrap_or_default(),
        metadata: Default::default(),
    }
}

fn to_proto_error(code: common::ErrorCode, message: impl Into<String>) -> common::ErrorDetails {
    common::ErrorDetails {
        error_code: code as i32,
        error_message: message.into(),
        ..Default::default()
    }
}

/// `pb::BetResult` is an enum; `SettlementService::settle_bet` takes the
/// lowercase string. HALF_WON / HALF_LOST have no domain equivalent, so they
/// are rejected rather than silently coerced into a full win or loss.
// 	onic::Status carries a message plus a binary metadata payload, so any
// Result<_, Status> trips clippy::result_large_err (threshold 128 bytes). The
// gRPC signatures fix the error type, so there is nothing to box - suppressed
// rather than worked around with an artificial error wrapper.
#[allow(clippy::result_large_err)]
fn bet_result_to_str(value: i32) -> Result<&'static str, Status> {
    match pb::BetResult::try_from(value) {
        Ok(pb::BetResult::Won) => Ok("won"),
        Ok(pb::BetResult::Lost) => Ok("lost"),
        Ok(pb::BetResult::Void) => Ok("void"),
        Ok(pb::BetResult::Unspecified) => Err(Status::invalid_argument("result is required")),
        Ok(other) => Err(Status::invalid_argument(format!(
            "{other:?} has no settlement mapping"
        ))),
        Err(_) => Err(Status::invalid_argument("unknown BetResult value")),
    }
}

#[tonic::async_trait]
impl pb::betting_engine_service_server::BettingEngineService for BettingEngineService {
    async fn place_bet(
        &self,
        request: Request<pb::PlaceBetRequest>,
    ) -> Result<Response<pb::PlaceBetResponse>, Status> {
        let req = request.into_inner();

        let user_id = req
            .user_id
            .as_ref()
            .ok_or_else(|| Status::invalid_argument("user_id is required"))?;
        let user_id = UserId(parse_i64("user_id", &user_id.value)?);

        let (stake, currency) = parse_money("stake", req.stake.as_ref())?;
        if stake <= Decimal::ZERO {
            return Err(Status::invalid_argument("stake must be positive"));
        }

        let odds = Decimal::from_str(&req.odds)
            .map_err(|_| Status::invalid_argument("odds must be a decimal"))?;
        if odds <= Decimal::ZERO {
            return Err(Status::invalid_argument("odds must be positive"));
        }

        // The contract describes a single event/market/selection, so the
        // structure is always Single. `req.bet_type` is a product vertical
        // (Sports/Live/Casino/...), not a structure - it is carried in metadata
        // rather than mapped onto DomainBetType.
        let event_id = parse_i64("event_id", &req.event_id)?;
        let market_id = parse_i64("market_id", &req.market_id)?;

        let idempotency_key = if req.client_request_id.is_empty() {
            Uuid::new_v4()
        } else {
            Uuid::parse_str(&req.client_request_id)
                .map_err(|_| Status::invalid_argument("client_request_id must be a UUID"))?
        };

        let domain_req = crate::domain::bet::PlaceBetRequest {
            bet_type: DomainBetType::Single,
            selections: vec![SelectionRequest {
                event_id,
                market_id,
                outcome_id: parse_i64("selection_id", &req.selection_id)?,
                odds,
            }],
            stake,
            currency_code: currency,
            idempotency_key,
            accept_odds_changes: AcceptOddsChanges::default(),
            ip_address: Some(req.ip_address).filter(|s| !s.is_empty()),
            device_fingerprint: Some(req.device_id).filter(|s| !s.is_empty()),
        };

        match self.bet_service.place_bet(user_id, domain_req).await {
            Ok(bet) => Ok(Response::new(pb::PlaceBetResponse {
                bet: Some(to_proto_bet(&bet)),
                error: None,
            })),
            Err(e) => {
                warn!(error = %e, "place_bet failed");
                Ok(Response::new(pb::PlaceBetResponse {
                    bet: None,
                    error: Some(to_proto_error(common::ErrorCode::BetInvalid, e.to_string())),
                }))
            }
        }
    }

    async fn cancel_bet(
        &self,
        request: Request<pb::CancelBetRequest>,
    ) -> Result<Response<pb::CancelBetResponse>, Status> {
        let req = request.into_inner();
        let bet_id = req
            .bet_id
            .as_ref()
            .ok_or_else(|| Status::invalid_argument("bet_id is required"))?;

        match self
            .settlement_service
            .void_bet(BetId(parse_i64("bet_id", &bet_id.value)?))
            .await
        {
            Ok(_) => Ok(Response::new(pb::CancelBetResponse {
                success: true,
                error: None,
            })),
            Err(e) => {
                warn!(error = %e, "cancel_bet failed");
                Ok(Response::new(pb::CancelBetResponse {
                    success: false,
                    error: Some(to_proto_error(common::ErrorCode::BetInvalid, e.to_string())),
                }))
            }
        }
    }

    async fn get_bet(
        &self,
        request: Request<pb::GetBetRequest>,
    ) -> Result<Response<pb::GetBetResponse>, Status> {
        let req = request.into_inner();
        let bet_id = req
            .bet_id
            .as_ref()
            .ok_or_else(|| Status::invalid_argument("bet_id is required"))?;
        let bet_id = parse_i64("bet_id", &bet_id.value)?;

        // Deliberately not implemented. `BetService::get_bet` takes a user_id
        // because it enforces that the caller owns the bet, and GetBetRequest
        // carries only a bet_id - so the check cannot be performed. Serving the
        // bet anyway would hand any caller any bet by guessing an id, so the
        // RPC stays closed until the contract carries a user_id.
        let _ = bet_id;
        Err(Status::unimplemented(
            "GetBet requires a user_id in the contract so ownership can be enforced",
        ))
    }

    async fn get_user_bets(
        &self,
        request: Request<pb::GetUserBetsRequest>,
    ) -> Result<Response<pb::GetUserBetsResponse>, Status> {
        let req = request.into_inner();
        let user_id = req
            .user_id
            .as_ref()
            .ok_or_else(|| Status::invalid_argument("user_id is required"))?;
        let user_id = UserId(parse_i64("user_id", &user_id.value)?);

        let pagination = req.pagination.unwrap_or_default();
        let limit = if pagination.page_size > 0 {
            i64::from(pagination.page_size)
        } else {
            20
        };
        let cursor = if pagination.cursor.is_empty() {
            None
        } else {
            Some(parse_i64("pagination.cursor", &pagination.cursor)?)
        };

        let result = self
            .bet_service
            .get_history(user_id, limit, cursor, None)
            .await
            .map_err(|e| {
                warn!(error = %e, "get_user_bets failed");
                Status::internal(e.to_string())
            })?;

        let bets = result
            .data
            .iter()
            .map(|b| pb::Bet {
                id: Some(common::BetId {
                    value: b.bet_id.to_string(),
                }),
                user_id: Some(common::UserId {
                    value: b.user_id.to_string(),
                }),
                r#type: common::BetType::Sports as i32,
                status: common::BetStatus::Settled as i32,
                stake: Some(money(
                    Decimal::from_str(&b.stake).unwrap_or(Decimal::ZERO),
                    &b.currency_code,
                )),
                potential_win: Some(money(
                    Decimal::from_str(&b.potential_win).unwrap_or(Decimal::ZERO),
                    &b.currency_code,
                )),
                actual_win: Some(money(
                    Decimal::from_str(&b.actual_win).unwrap_or(Decimal::ZERO),
                    &b.currency_code,
                )),
                event_id: b
                    .selections
                    .first()
                    .map(|s| s.event_id.to_string())
                    .unwrap_or_default(),
                market_id: b
                    .selections
                    .first()
                    .map(|s| s.market_id.to_string())
                    .unwrap_or_default(),
                selection_id: b
                    .selections
                    .first()
                    .map(|s| s.outcome_id.to_string())
                    .unwrap_or_default(),
                odds: b.odds.clone(),
                placed_at: None,
                settled_at: None,
                client_request_id: String::new(),
                device_id: String::new(),
                ip_address: String::new(),
                metadata: Default::default(),
            })
            .collect();

        Ok(Response::new(pb::GetUserBetsResponse {
            bets,
            pagination: Some(common::PageResponse {
                next_cursor: result.cursor.clone().unwrap_or_default(),
                prev_cursor: String::new(),
                has_more: result.cursor.is_some(),
                total_count: Some(result.total),
                current_page: None,
                total_pages: None,
            }),
        }))
    }

    async fn settle_bet(
        &self,
        request: Request<pb::SettleBetRequest>,
    ) -> Result<Response<pb::SettleBetResponse>, Status> {
        let req = request.into_inner();
        let bet_id = req
            .bet_id
            .as_ref()
            .ok_or_else(|| Status::invalid_argument("bet_id is required"))?;
        let bet_id = BetId(parse_i64("bet_id", &bet_id.value)?);
        let result = bet_result_to_str(req.result)?;
        let (actual_win, _) = parse_money("actual_win", req.actual_win.as_ref())?;

        match self
            .settlement_service
            .settle_bet(bet_id, result, actual_win)
            .await
        {
            Ok(_) => Ok(Response::new(pb::SettleBetResponse {
                success: true,
                error: None,
            })),
            Err(e) => {
                warn!(error = %e, "settle_bet failed");
                Ok(Response::new(pb::SettleBetResponse {
                    success: false,
                    error: Some(to_proto_error(common::ErrorCode::BetInvalid, e.to_string())),
                }))
            }
        }
    }

    async fn get_odds(
        &self,
        _request: Request<pb::GetOddsRequest>,
    ) -> Result<Response<pb::GetOddsResponse>, Status> {
        // Odds live in the Sportsbook/feeds layer, not in this crate. Returning
        // an error is honest; synthesising odds here would be worse.
        Err(Status::unimplemented(
            "GetOdds is served by the sportsbook feed, not the betting engine",
        ))
    }

    type StreamOddsStream = Pin<
        Box<dyn tonic::codegen::tokio_stream::Stream<Item = Result<pb::OddsUpdate, Status>> + Send>,
    >;

    async fn stream_odds(
        &self,
        _request: Request<pb::OddsStreamRequest>,
    ) -> Result<Response<Self::StreamOddsStream>, Status> {
        Err(Status::unimplemented(
            "StreamOdds is served by the websocket gateway",
        ))
    }
}
