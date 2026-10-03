pub mod server;

use std::pin::Pin;

use rust_decimal::prelude::ToPrimitive;
use rust_decimal::Decimal;
use tonic::{Request, Response, Status};

use crate::domain::bet::Bet as DomainBet;
use crate::domain::bet::{
    AcceptOddsChanges, BetId, BetResponse, BetStatus, BetType, SelectionRequest, UserId,
};
// Aliased: the proto request of the same name is in scope for the gRPC methods.
use crate::domain::bet::PlaceBetRequest as DomainPlaceBetRequest;
use crate::errors::AppError;
use crate::services::bet_service::BetService;
use crate::services::settlement_service::SettlementService;

/// Generated code for `betting/v1/betting.proto`.
pub mod proto {
    pub mod betting {
        pub mod v1 {
            tonic::include_proto!("betting.v1");
        }
    }
    pub mod common {
        pub mod v1 {
            tonic::include_proto!("common.v1");
        }
    }
}

use proto::betting::v1::betting_engine_service_server::{
    BettingEngineService as BettingEngineSvc, BettingEngineServiceServer,
};
use proto::betting::v1::{
    Bet as ProtoBet, BetResult, CancelBetRequest, CancelBetResponse, GetBetRequest, GetBetResponse,
    GetOddsRequest, GetOddsResponse, GetUserBetsRequest, GetUserBetsResponse, OddsStreamRequest,
    OddsUpdate, PlaceBetRequest, PlaceBetResponse, SettleBetRequest, SettleBetResponse,
};
use proto::common::v1::{BetId as ProtoBetId, ErrorDetails, Money, UserId as ProtoUserId};

/// gRPC adapter around the betting domain services.
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

    pub fn into_server(self) -> BettingEngineServiceServer<Self> {
        BettingEngineServiceServer::new(self)
    }
}

/// Invalid-argument detail returned by the request parsers.
///
/// `tonic::Status` is ~176 bytes, so returning it directly from these helpers
/// would bloat every `Result`; this small type converts at the call sites.
#[derive(Debug)]
struct InvalidArg(&'static str);

impl From<InvalidArg> for Status {
    fn from(err: InvalidArg) -> Self {
        Status::invalid_argument(err.0)
    }
}

/// Parse a `common.v1.UserId` / `common.v1.BetId` wrapper into the domain newtype.
fn parse_user_id(id: &Option<ProtoUserId>, field: &'static str) -> Result<i64, InvalidArg> {
    let value = id.as_ref().ok_or(InvalidArg(field))?;
    value.value.parse::<i64>().map_err(|_| InvalidArg(field))
}

fn parse_bet_id(id: &Option<ProtoBetId>) -> Result<i64, InvalidArg> {
    let value = id.as_ref().ok_or(InvalidArg("bet_id"))?;
    value.value.parse::<i64>().map_err(|_| InvalidArg("bet_id"))
}

fn parse_decimal(raw: &str, field: &'static str) -> Result<Decimal, InvalidArg> {
    if raw.trim().is_empty() {
        return Err(InvalidArg(field));
    }
    raw.parse::<Decimal>().map_err(|_| InvalidArg(field))
}

/// Identifiers arrive as decimal strings on the wire; accept both `12` and
/// `12.0` but reject fractional values.
fn parse_i64_id(raw: &str, field: &'static str) -> Result<i64, InvalidArg> {
    let value = parse_decimal(raw, field)?;
    if !value.fract().is_zero() {
        return Err(InvalidArg(field));
    }
    value.trunc().to_i64().ok_or(InvalidArg(field))
}

/// Build a `common.v1.ErrorDetails` payload.
fn proto_error(message: String) -> ErrorDetails {
    ErrorDetails {
        error_code: proto::common::v1::ErrorCode::InternalError as i32,
        error_message: message,
        metadata: Default::default(),
        field_errors: Vec::new(),
        trace_id: String::new(),
    }
}

fn money(amount: Decimal, currency: &str) -> Money {
    Money {
        amount: amount.to_string(),
        currency: currency.to_string(),
    }
}

fn proto_timestamp(ts: chrono::DateTime<chrono::Utc>) -> prost_types::Timestamp {
    prost_types::Timestamp {
        seconds: ts.timestamp(),
        nanos: ts.timestamp_subsec_nanos() as i32,
    }
}

/// Domain `BetStatus` -> proto `common.v1.BetStatus`.
fn proto_bet_status(status: &BetStatus) -> i32 {
    match status {
        BetStatus::Pending | BetStatus::Active => proto::common::v1::BetStatus::Pending as i32,
        BetStatus::Won | BetStatus::Lost | BetStatus::Cashout => {
            proto::common::v1::BetStatus::Settled as i32
        }
        BetStatus::Void => proto::common::v1::BetStatus::Cancelled as i32,
        BetStatus::Rejected => proto::common::v1::BetStatus::Rejected as i32,
    }
}

/// Domain error -> gRPC status, preserving the error message for callers.
fn app_err_to_status(err: AppError) -> Status {
    match err {
        AppError::Validation(_) | AppError::BetRejected { .. } => {
            Status::invalid_argument(err.to_string())
        }
        AppError::Unauthorized { .. } => Status::unauthenticated(err.to_string()),
        AppError::Forbidden { .. } => Status::permission_denied(err.to_string()),
        AppError::NotFound { .. } | AppError::BetEventNotFound { .. } => {
            Status::not_found(err.to_string())
        }
        AppError::Conflict { .. }
        | AppError::BetOddsChanged { .. }
        | AppError::BetAlreadySettled => Status::aborted(err.to_string()),
        AppError::ServiceUnavailable(_) => Status::unavailable(err.to_string()),
        AppError::RateLimited { .. } => Status::resource_exhausted(err.to_string()),
        AppError::InsufficientBalance { .. }
        | AppError::BetStakeTooLow { .. }
        | AppError::BetStakeTooHigh { .. }
        | AppError::BetMaxPayoutExceeded { .. }
        | AppError::BetEventSuspended { .. }
        | AppError::BetMarketClosed { .. }
        | AppError::CashoutUnavailable => Status::failed_precondition(err.to_string()),
        AppError::Internal(_) | AppError::Database(_) | AppError::Cache(_) => {
            Status::internal(err.to_string())
        }
    }
}

fn convert_bet_to_proto(bet: &DomainBet) -> ProtoBet {
    let currency = bet.currency_code.as_str();
    ProtoBet {
        id: Some(ProtoBetId {
            value: bet.id.0.to_string(),
        }),
        user_id: Some(ProtoUserId {
            value: bet.user_id.0.to_string(),
        }),
        r#type: proto::common::v1::BetType::Sports as i32,
        status: proto_bet_status(&bet.status),
        stake: Some(money(bet.stake, currency)),
        potential_win: Some(money(bet.potential_win, currency)),
        actual_win: Some(money(bet.actual_win, currency)),
        event_id: bet.event_id.map(|e| e.0.to_string()).unwrap_or_default(),
        market_id: bet
            .selections
            .first()
            .map(|s| s.market_id.0.to_string())
            .unwrap_or_default(),
        selection_id: bet
            .selections
            .first()
            .map(|s| s.outcome_id.0.to_string())
            .unwrap_or_default(),
        odds: bet.combined_odds.to_string(),
        placed_at: Some(proto_timestamp(bet.placed_at)),
        settled_at: bet.settled_at.map(proto_timestamp),
        client_request_id: bet.idempotency_key.to_string(),
        device_id: bet.device_fingerprint.clone().unwrap_or_default(),
        ip_address: bet.ip_address.clone().unwrap_or_default(),
        metadata: Default::default(),
    }
}

fn convert_bet_response_to_proto(resp: &BetResponse) -> ProtoBet {
    let currency = resp.currency_code.as_str();
    let mut bet = ProtoBet {
        id: Some(ProtoBetId {
            value: resp.bet_id.to_string(),
        }),
        user_id: Some(ProtoUserId {
            value: resp.user_id.to_string(),
        }),
        r#type: proto::common::v1::BetType::Sports as i32,
        stake: Some(money(
            resp.stake.parse::<Decimal>().unwrap_or(Decimal::ZERO),
            currency,
        )),
        potential_win: Some(money(
            resp.potential_win
                .parse::<Decimal>()
                .unwrap_or(Decimal::ZERO),
            currency,
        )),
        actual_win: Some(money(
            resp.actual_win.parse::<Decimal>().unwrap_or(Decimal::ZERO),
            currency,
        )),
        status: status_for_proto_string(&resp.status),
        odds: resp.odds.clone(),
        client_request_id: resp.bet_id.to_string(),
        metadata: Default::default(),
        ..Default::default()
    };

    if let Some(selection) = resp.selections.first() {
        bet.event_id = selection.event_id.to_string();
        bet.market_id = selection.market_id.to_string();
        bet.selection_id = selection.outcome_id.to_string();
    }

    bet
}

fn status_for_proto_string(status: &str) -> i32 {
    match status {
        "pending" | "active" => proto::common::v1::BetStatus::Pending as i32,
        "won" | "lost" | "cashout" => proto::common::v1::BetStatus::Settled as i32,
        "void" => proto::common::v1::BetStatus::Cancelled as i32,
        "rejected" => proto::common::v1::BetStatus::Rejected as i32,
        _ => proto::common::v1::BetStatus::Unspecified as i32,
    }
}

#[tonic::async_trait]
impl BettingEngineSvc for BettingEngineService {
    async fn place_bet(
        &self,
        request: Request<PlaceBetRequest>,
    ) -> Result<Response<PlaceBetResponse>, Status> {
        let req = request.into_inner();

        let user_id = parse_user_id(&req.user_id, "user_id")?;
        let odds = parse_decimal(&req.odds, "odds")?;

        let stake = match req.stake.as_ref() {
            Some(s) => parse_decimal(&s.amount, "stake")?,
            None => return Err(Status::invalid_argument("Missing stake")),
        };
        let currency = req
            .stake
            .as_ref()
            .map(|s| s.currency.clone())
            .unwrap_or_default();

        let place_req = DomainPlaceBetRequest {
            bet_type: BetType::Single,
            selections: vec![SelectionRequest {
                event_id: parse_i64_id(&req.event_id, "event_id")?,
                market_id: parse_i64_id(&req.market_id, "market_id")?,
                outcome_id: parse_i64_id(&req.selection_id, "selection_id")?,
                odds,
            }],
            stake,
            currency_code: currency,
            idempotency_key: uuid::Uuid::parse_str(&req.client_request_id)
                .unwrap_or_else(|_| uuid::Uuid::new_v4()),
            accept_odds_changes: AcceptOddsChanges::default(),
            ip_address: (!req.ip_address.is_empty()).then(|| req.ip_address.clone()),
            device_fingerprint: (!req.device_id.is_empty()).then(|| req.device_id.clone()),
        };

        match self.bet_service.place_bet(UserId(user_id), place_req).await {
            Ok(bet) => Ok(Response::new(PlaceBetResponse {
                bet: Some(convert_bet_to_proto(&bet)),
                error: None,
            })),
            Err(e) => Ok(Response::new(PlaceBetResponse {
                bet: None,
                error: Some(proto_error(e.to_string())),
            })),
        }
    }

    async fn cancel_bet(
        &self,
        request: Request<CancelBetRequest>,
    ) -> Result<Response<CancelBetResponse>, Status> {
        let req = request.into_inner();
        let bet_id = parse_bet_id(&req.bet_id)?;

        match self.settlement_service.void_bet(BetId(bet_id)).await {
            Ok(_) => Ok(Response::new(CancelBetResponse {
                success: true,
                error: None,
            })),
            Err(e) => Ok(Response::new(CancelBetResponse {
                success: false,
                error: Some(proto_error(e.to_string())),
            })),
        }
    }

    async fn get_bet(
        &self,
        request: Request<GetBetRequest>,
    ) -> Result<Response<GetBetResponse>, Status> {
        let req = request.into_inner();
        // Validate the request shape even though the lookup itself is not
        // implemented, so callers get INVALID_ARGUMENT instead of UNIMPLEMENTED
        // for malformed ids.
        parse_bet_id(&req.bet_id)?;

        // gRPC: get without user_id check (internal service call)
        Err(Status::unimplemented(
            "Use REST API for bet queries with user context",
        ))
    }

    async fn get_user_bets(
        &self,
        request: Request<GetUserBetsRequest>,
    ) -> Result<Response<GetUserBetsResponse>, Status> {
        let req = request.into_inner();
        let user_id = parse_user_id(&req.user_id, "user_id")?;

        // `PageRequest.page_size` documents a 1..=100 range; clamp to it.
        let limit = req
            .pagination
            .as_ref()
            .map(|p| p.page_size.clamp(1, 100) as i64)
            .unwrap_or(20);

        let result = self
            .bet_service
            .get_history(UserId(user_id), limit, None, None)
            .await
            .map_err(app_err_to_status)?;

        let bets: Vec<ProtoBet> = result
            .data
            .iter()
            .map(convert_bet_response_to_proto)
            .collect();
        let returned = bets.len();

        Ok(Response::new(GetUserBetsResponse {
            bets,
            pagination: Some(proto::common::v1::PageResponse {
                next_cursor: String::new(),
                prev_cursor: String::new(),
                has_more: (result.total as usize) > returned,
                total_count: Some(result.total),
                current_page: Some(1),
                total_pages: Some(if limit > 0 {
                    (((result.total + limit - 1) / limit).max(1)) as i32
                } else {
                    1
                }),
            }),
        }))
    }

    async fn settle_bet(
        &self,
        request: Request<SettleBetRequest>,
    ) -> Result<Response<SettleBetResponse>, Status> {
        let req = request.into_inner();
        let bet_id = parse_bet_id(&req.bet_id)?;

        let result = match BetResult::try_from(req.result) {
            Ok(BetResult::Won) => "won",
            Ok(BetResult::Lost) => "lost",
            Ok(BetResult::Void) => "void",
            Ok(BetResult::HalfWon) => "half_won",
            Ok(BetResult::HalfLost) => "half_lost",
            _ => return Err(Status::invalid_argument("Missing or unknown result")),
        };

        let actual_win = match req.actual_win.as_ref() {
            Some(m) => parse_decimal(&m.amount, "actual_win")?,
            None => Decimal::ZERO,
        };

        match self
            .settlement_service
            .settle_bet(BetId(bet_id), result, actual_win)
            .await
        {
            Ok(_) => Ok(Response::new(SettleBetResponse {
                success: true,
                error: None,
            })),
            Err(e) => Ok(Response::new(SettleBetResponse {
                success: false,
                error: Some(proto_error(e.to_string())),
            })),
        }
    }

    async fn get_odds(
        &self,
        request: Request<GetOddsRequest>,
    ) -> Result<Response<GetOddsResponse>, Status> {
        let req = request.into_inner();

        if req.event_id.is_empty() {
            return Err(Status::invalid_argument("Missing event_id"));
        }

        // Odds are served from the websocket gateway's feed; the gRPC surface is
        // not implemented by this service yet.
        Err(Status::unimplemented(
            "Odds are served by the WebSocket gateway",
        ))
    }

    type StreamOddsStream =
        Pin<Box<dyn tokio_stream::Stream<Item = Result<OddsUpdate, Status>> + Send>>;

    async fn stream_odds(
        &self,
        _request: Request<OddsStreamRequest>,
    ) -> Result<Response<Self::StreamOddsStream>, Status> {
        Err(Status::unimplemented(
            "Odds streaming handled by WebSocket gateway",
        ))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn domain_status_maps_to_proto_status() {
        assert_eq!(
            proto_bet_status(&BetStatus::Pending),
            proto::common::v1::BetStatus::Pending as i32
        );
        assert_eq!(
            proto_bet_status(&BetStatus::Won),
            proto::common::v1::BetStatus::Settled as i32
        );
        assert_eq!(
            proto_bet_status(&BetStatus::Void),
            proto::common::v1::BetStatus::Cancelled as i32
        );
    }

    #[test]
    fn status_for_proto_string_is_total() {
        assert_eq!(
            status_for_proto_string("won"),
            proto::common::v1::BetStatus::Settled as i32
        );
        assert_eq!(
            status_for_proto_string("nonsense"),
            proto::common::v1::BetStatus::Unspecified as i32
        );
    }

    #[test]
    fn decimal_parsing_rejects_blank_and_garbage() {
        assert!(parse_decimal("", "stake").is_err());
        assert!(parse_decimal("   ", "stake").is_err());
        assert!(parse_decimal("abc", "stake").is_err());
        assert_eq!(
            parse_decimal("10.50", "stake").unwrap(),
            "10.50".parse::<Decimal>().unwrap()
        );
    }

    #[test]
    fn money_encodes_amount_and_currency() {
        let m = money("12.34".parse::<Decimal>().unwrap(), "USD");
        assert_eq!(m.amount, "12.34");
        assert_eq!(m.currency, "USD");
    }

    #[test]
    fn parse_user_id_rejects_missing_and_non_numeric() {
        assert!(parse_user_id(&None, "user_id").is_err());
        let bad = Some(ProtoUserId {
            value: "not-a-number".into(),
        });
        assert!(parse_user_id(&bad, "user_id").is_err());
        let good = Some(ProtoUserId { value: "7".into() });
        assert_eq!(parse_user_id(&good, "user_id").unwrap(), 7);
    }
}
