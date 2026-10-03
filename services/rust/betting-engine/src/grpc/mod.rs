pub mod server;

use std::collections::HashMap;
use std::pin::Pin;

use chrono::{DateTime, Utc};
use rust_decimal::Decimal;
use tonic::{Request, Response, Status};
use uuid::Uuid;

use crate::domain::bet::{
    AcceptOddsChanges, Bet, BetId, BetResponse as DomainBetResponse, BetStatus as DomainBetStatus,
    BetType as DomainBetType, PlaceBetRequest as DomainPlaceBetRequest, SelectionRequest, UserId,
};
use crate::errors::AppError;
use crate::services::bet_service::BetService;
use crate::services::settlement_service::SettlementService;

// Generated from libs/proto. CONVENTIONS.md NEVER-3: a proto contract has exactly
// one home (libs/proto/) and adapters bind to it rather than redefining it.
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

pub use betting::v1::betting_engine_service_server;

use betting::v1::{
    Bet as PbBet, BetResult as PbResult, CancelBetRequest, CancelBetResponse, GetBetRequest,
    GetBetResponse, GetOddsRequest, GetOddsResponse, GetUserBetsRequest, GetUserBetsResponse,
    OddsStreamRequest, OddsUpdate, PlaceBetRequest, PlaceBetResponse, SettleBetRequest,
    SettleBetResponse,
};
use common::v1::{
    BetId as PbBetId, BetStatus as PbBetStatus, ErrorCode, ErrorDetails, FieldError, Money,
    PageResponse, UserId as PbUserId,
};

const DEFAULT_PAGE_SIZE: i64 = 20;

/// Handler for the `betting.v1.BettingEngineService` contract.
///
/// The generated trait carries the contract's name, so the handler takes the
/// conventional `BettingEngine` to avoid colliding with it.
#[derive(Clone)]
pub struct BettingEngine {
    bet_service: BetService,
    settlement_service: SettlementService,
}

impl BettingEngine {
    pub fn new(bet_service: BetService, settlement_service: SettlementService) -> Self {
        Self {
            bet_service,
            settlement_service,
        }
    }
}

#[tonic::async_trait]
impl betting_engine_service_server::BettingEngineService for BettingEngine {
    async fn place_bet(
        &self,
        request: Request<PlaceBetRequest>,
    ) -> Result<Response<PlaceBetResponse>, Status> {
        let req = request.into_inner();

        let user_id = require_user_id(req.user_id.as_ref())?;
        let stake = parse_decimal(req.stake.as_ref().map(|m| m.amount.as_str()), "stake")?;
        let odds = parse_decimal(Some(req.odds.as_str()), "odds")?;
        let event_id = parse_i64(&req.event_id, "event_id")?;
        let market_id = parse_i64(&req.market_id, "market_id")?;
        let selection_id = parse_i64(&req.selection_id, "selection_id")?;

        // The contract carries exactly one event/market/selection triple, so the
        // domain bet is structurally a single. `common.v1.BetType` is a *product*
        // enum (sports/live/casino/...) and has no structural counterpart in the
        // domain model, so it is not mapped here; the placement rules that depend
        // on it are applied inside the domain service.
        let domain_req = DomainPlaceBetRequest {
            bet_type: DomainBetType::Single,
            selections: vec![SelectionRequest {
                event_id,
                market_id,
                outcome_id: selection_id,
                odds,
            }],
            stake,
            currency_code: req
                .stake
                .as_ref()
                .map(|m| m.currency.clone())
                .filter(|c| !c.is_empty())
                .unwrap_or_else(|| "USD".to_string()),
            idempotency_key: Uuid::parse_str(&req.client_request_id)
                .unwrap_or_else(|_| Uuid::new_v4()),
            accept_odds_changes: AcceptOddsChanges::None,
            ip_address: non_empty(req.ip_address),
            device_fingerprint: non_empty(req.device_id),
        };

        match self
            .bet_service
            .place_bet(UserId(user_id), domain_req)
            .await
        {
            Ok(bet) => Ok(Response::new(PlaceBetResponse {
                bet: Some(bet_to_proto(&bet)),
                error: None,
            })),
            Err(err) => Ok(Response::new(PlaceBetResponse {
                bet: None,
                error: Some(error_details(&err)),
            })),
        }
    }

    async fn cancel_bet(
        &self,
        request: Request<CancelBetRequest>,
    ) -> Result<Response<CancelBetResponse>, Status> {
        let req = request.into_inner();
        let bet_id = parse_bet_id(req.bet_id.as_ref())?;

        match self.settlement_service.void_bet(BetId(bet_id)).await {
            Ok(_) => Ok(Response::new(CancelBetResponse {
                success: true,
                error: None,
            })),
            Err(err) => Ok(Response::new(CancelBetResponse {
                success: false,
                error: Some(error_details(&err)),
            })),
        }
    }

    async fn get_bet(
        &self,
        _request: Request<GetBetRequest>,
    ) -> Result<Response<GetBetResponse>, Status> {
        // GetBetRequest carries only a bet id. Reading a bet requires the owning
        // user id as well, and CONVENTIONS.md NEVER-7 forbids taking an identity
        // from a request field, so this cannot be served without an authenticated
        // principal. Callers use the REST API in src/api, which carries the JWT.
        Err(Status::unimplemented(
            "GetBet requires an authenticated principal; use the REST API",
        ))
    }

    async fn get_user_bets(
        &self,
        request: Request<GetUserBetsRequest>,
    ) -> Result<Response<GetUserBetsResponse>, Status> {
        let req = request.into_inner();
        let user_id = require_user_id(req.user_id.as_ref())?;

        let limit = req
            .pagination
            .as_ref()
            .map(|p| p.page_size)
            .filter(|size| *size > 0)
            .map_or(DEFAULT_PAGE_SIZE, |size| size as i64);

        let cursor = req
            .pagination
            .as_ref()
            .and_then(|p| non_empty(p.cursor.clone()))
            .and_then(|c| c.parse::<i64>().ok());

        let status = req
            .status
            .and_then(|raw| PbBetStatus::try_from(raw).ok())
            .map(to_domain_status);

        match self
            .bet_service
            .get_history(UserId(user_id), limit, cursor, status)
            .await
        {
            Ok(result) => {
                let bets = result.data.iter().map(bet_response_to_proto).collect();
                let from = cursor.unwrap_or(0);
                let next = from + limit;
                let per_page = limit.max(1);

                Ok(Response::new(GetUserBetsResponse {
                    bets,
                    pagination: Some(PageResponse {
                        next_cursor: result.cursor.unwrap_or_default(),
                        prev_cursor: String::new(),
                        has_more: next < result.total,
                        total_count: Some(result.total),
                        current_page: Some((from / per_page + 1) as i32),
                        // `i64::div_ceil` is still unstable on the pinned
                        // toolchain (rust#88581), so round up by hand.
                        total_pages: Some(
                            (result.total / per_page + i64::from(result.total % per_page != 0))
                                as i32,
                        ),
                    }),
                }))
            }
            Err(err) => Err(Status::internal(err.to_string())),
        }
    }

    async fn settle_bet(
        &self,
        request: Request<SettleBetRequest>,
    ) -> Result<Response<SettleBetResponse>, Status> {
        let req = request.into_inner();
        let bet_id = parse_bet_id(req.bet_id.as_ref())?;

        let result = match PbResult::try_from(req.result).ok() {
            Some(PbResult::Won) => "won",
            Some(PbResult::Lost) => "lost",
            Some(PbResult::Void) => "void",
            _ => {
                // UNSPECIFIED and the half-settlement outcomes have no
                // representation in the domain settlement rules.
                return Err(Status::invalid_argument(format!(
                    "unsupported bet result enum value {}",
                    req.result
                )));
            }
        };

        let actual_win = match req.actual_win.as_ref() {
            Some(money) => parse_decimal(Some(money.amount.as_str()), "actual_win")?,
            // A missing Money means "lost", which settles at zero.
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
            Err(err) => Ok(Response::new(SettleBetResponse {
                success: false,
                error: Some(error_details(&err)),
            })),
        }
    }

    async fn get_odds(
        &self,
        _request: Request<GetOddsRequest>,
    ) -> Result<Response<GetOddsResponse>, Status> {
        // Odds are served by the websocket gateway, which is the only component
        // holding a SportradarFeedClient. This service is never given one.
        Err(Status::unimplemented(
            "GetOdds is served by the websocket gateway",
        ))
    }

    type StreamOddsStream = Pin<
        Box<dyn tonic::codegen::tokio_stream::Stream<Item = Result<OddsUpdate, Status>> + Send>,
    >;

    async fn stream_odds(
        &self,
        _request: Request<OddsStreamRequest>,
    ) -> Result<Response<Self::StreamOddsStream>, Status> {
        Err(Status::unimplemented(
            "Odds streaming is served by the websocket gateway",
        ))
    }
}

// ---------------------------------------------------------------------------
// Parsing helpers
// ---------------------------------------------------------------------------

fn non_empty(value: String) -> Option<String> {
    if value.is_empty() {
        None
    } else {
        Some(value)
    }
}

fn parse_i64(raw: &str, field: &str) -> Result<i64, Status> {
    raw.trim().parse::<i64>().map_err(|_| {
        Status::invalid_argument(format!("{field} must be a base-10 integer, got {raw:?}"))
    })
}

fn parse_decimal(raw: Option<&str>, field: &str) -> Result<Decimal, Status> {
    let raw = raw.unwrap_or("").trim().to_string();
    if raw.is_empty() {
        return Err(Status::invalid_argument(format!("{field} is required")));
    }
    raw.parse::<Decimal>()
        .map_err(|_| Status::invalid_argument(format!("{field} must be a decimal string")))
}

fn require_user_id(user_id: Option<&PbUserId>) -> Result<i64, Status> {
    let value = user_id.map(|u| u.value.as_str()).unwrap_or_default().trim();
    if value.is_empty() {
        return Err(Status::invalid_argument("user_id is required"));
    }
    parse_i64(value, "user_id")
}

fn parse_bet_id(bet_id: Option<&PbBetId>) -> Result<i64, Status> {
    let value = bet_id.map(|b| b.value.as_str()).unwrap_or_default().trim();
    if value.is_empty() {
        return Err(Status::invalid_argument("bet_id is required"));
    }
    parse_i64(value, "bet_id")
}

// ---------------------------------------------------------------------------
// Error mapping
// ---------------------------------------------------------------------------

fn error_code(err: &AppError) -> ErrorCode {
    match err {
        AppError::Validation(_) => ErrorCode::ValidationError,
        AppError::Unauthorized { .. } => ErrorCode::AuthTokenInvalid,
        AppError::Forbidden { .. } => ErrorCode::PermissionDenied,
        AppError::NotFound { .. } => ErrorCode::BetNotFound,
        AppError::Conflict { .. } => ErrorCode::BetInvalid,
        AppError::BetEventNotFound { .. } => ErrorCode::BetSelectionInvalid,
        AppError::BetEventSuspended { .. } => ErrorCode::BetEventSuspended,
        AppError::BetMarketClosed { .. } => ErrorCode::BetSelectionInvalid,
        AppError::BetOddsChanged { .. } => ErrorCode::BetOddsChanged,
        AppError::BetStakeTooLow { .. } => ErrorCode::BetMinimumNotMet,
        AppError::BetStakeTooHigh { .. } | AppError::BetMaxPayoutExceeded { .. } => {
            ErrorCode::BetLimitExceeded
        }
        AppError::BetRejected { .. } | AppError::CashoutUnavailable => ErrorCode::BetInvalid,
        AppError::BetAlreadySettled => ErrorCode::BetAlreadySettled,
        AppError::InsufficientBalance { .. } => ErrorCode::InsufficientBalance,
        // The contract has no code for transport-level failures; the message
        // carries the detail and trace_id is left empty by design.
        AppError::ServiceUnavailable(_)
        | AppError::RateLimited { .. }
        | AppError::Internal(_)
        | AppError::Database(_)
        | AppError::Cache(_) => ErrorCode::Unspecified,
    }
}

fn error_details(err: &AppError) -> ErrorDetails {
    ErrorDetails {
        error_code: error_code(err) as i32,
        error_message: err.to_string(),
        metadata: HashMap::new(),
        field_errors: match err {
            AppError::Validation(fields) => fields
                .iter()
                .map(|f| FieldError {
                    field: f.field.clone(),
                    error_code: String::new(),
                    error_message: f.message.clone(),
                })
                .collect(),
            _ => Vec::new(),
        },
        trace_id: String::new(),
    }
}

// ---------------------------------------------------------------------------
// Enum mapping
// ---------------------------------------------------------------------------

fn pb_status(status: DomainBetStatus) -> PbBetStatus {
    match status {
        DomainBetStatus::Pending => PbBetStatus::Pending,
        DomainBetStatus::Active => PbBetStatus::Accepted,
        // A cashout closes the bet with a paid return, so it is a settlement.
        DomainBetStatus::Won | DomainBetStatus::Lost | DomainBetStatus::Cashout => {
            PbBetStatus::Settled
        }
        DomainBetStatus::Void => PbBetStatus::Cancelled,
        DomainBetStatus::Rejected => PbBetStatus::Rejected,
    }
}

fn to_domain_status(status: PbBetStatus) -> DomainBetStatus {
    match status {
        PbBetStatus::Pending => DomainBetStatus::Pending,
        PbBetStatus::Accepted => DomainBetStatus::Active,
        PbBetStatus::Settled => DomainBetStatus::Won,
        PbBetStatus::Cancelled => DomainBetStatus::Void,
        PbBetStatus::Rejected => DomainBetStatus::Rejected,
        PbBetStatus::Unspecified => DomainBetStatus::Pending,
    }
}

/// Domain bet types are structural (single/accumulator/...), the contract's
/// `common.v1.BetType` is a product line. They are not isomorphic, so the
/// adapter reports UNSPECIFIED rather than inventing a wrong product.
fn pb_bet_type(_bet_type: DomainBetType) -> i32 {
    common::v1::BetType::Unspecified as i32
}

// ---------------------------------------------------------------------------
// Timestamps
// ---------------------------------------------------------------------------

type PbTimestamp = prost_types::Timestamp;

fn to_timestamp(value: DateTime<Utc>) -> PbTimestamp {
    PbTimestamp {
        seconds: value.timestamp(),
        nanos: value.timestamp_subsec_nanos() as i32,
    }
}

fn parse_timestamp(raw: &str) -> Option<PbTimestamp> {
    DateTime::parse_from_rfc3339(raw)
        .ok()
        .map(|dt| to_timestamp(dt.with_timezone(&Utc)))
}

// ---------------------------------------------------------------------------
// Domain -> contract
// ---------------------------------------------------------------------------

fn money(amount: Decimal, currency: &str) -> Money {
    Money {
        amount: amount.to_string(),
        currency: currency.to_string(),
    }
}

fn bet_to_proto(bet: &Bet) -> PbBet {
    let selection = bet.selections.first();

    PbBet {
        id: Some(PbBetId {
            value: bet.id.0.to_string(),
        }),
        user_id: Some(PbUserId {
            value: bet.user_id.0.to_string(),
        }),
        r#type: pb_bet_type(bet.bet_type),
        status: pb_status(bet.status) as i32,
        stake: Some(money(bet.stake, &bet.currency_code)),
        potential_win: Some(money(bet.potential_win, &bet.currency_code)),
        actual_win: Some(money(bet.actual_win, &bet.currency_code)),
        event_id: bet
            .event_id
            .map(|id| id.0.to_string())
            .or_else(|| selection.map(|s| s.event_id.0.to_string()))
            .unwrap_or_default(),
        market_id: selection
            .map(|s| s.market_id.0.to_string())
            .unwrap_or_default(),
        selection_id: selection
            .map(|s| s.outcome_id.0.to_string())
            .unwrap_or_default(),
        odds: bet.combined_odds.to_string(),
        placed_at: Some(to_timestamp(bet.placed_at)),
        settled_at: bet.settled_at.map(to_timestamp),
        client_request_id: bet.idempotency_key.to_string(),
        device_id: bet.device_fingerprint.clone().unwrap_or_default(),
        ip_address: bet.ip_address.clone().unwrap_or_default(),
        metadata: HashMap::new(),
    }
}

fn bet_response_to_proto(bet: &DomainBetResponse) -> PbBet {
    let selection = bet.selections.first();
    let currency = bet.currency_code.as_str();

    PbBet {
        id: Some(PbBetId {
            value: bet.bet_id.to_string(),
        }),
        user_id: Some(PbUserId {
            value: bet.user_id.to_string(),
        }),
        r#type: pb_bet_type_from_str(&bet.bet_type),
        status: pb_status_from_str(&bet.status) as i32,
        stake: Some(money(parse_decimal_lossy(&bet.stake), currency)),
        potential_win: Some(money(parse_decimal_lossy(&bet.potential_win), currency)),
        actual_win: Some(money(parse_decimal_lossy(&bet.actual_win), currency)),
        event_id: selection
            .map(|s| s.event_id.to_string())
            .unwrap_or_default(),
        market_id: selection
            .map(|s| s.market_id.to_string())
            .unwrap_or_default(),
        selection_id: selection
            .map(|s| s.outcome_id.to_string())
            .unwrap_or_default(),
        odds: bet.odds.clone(),
        placed_at: parse_timestamp(&bet.placed_at),
        settled_at: bet.settled_at.as_deref().and_then(parse_timestamp),
        client_request_id: String::new(),
        device_id: String::new(),
        ip_address: String::new(),
        metadata: HashMap::new(),
    }
}

fn parse_decimal_lossy(raw: &str) -> Decimal {
    raw.trim().parse::<Decimal>().unwrap_or(Decimal::ZERO)
}

fn pb_bet_type_from_str(_raw: &str) -> i32 {
    common::v1::BetType::Unspecified as i32
}

fn pb_status_from_str(raw: &str) -> PbBetStatus {
    match raw {
        "pending" => PbBetStatus::Pending,
        "active" => PbBetStatus::Accepted,
        "won" | "lost" | "cashout" => PbBetStatus::Settled,
        "void" => PbBetStatus::Cancelled,
        "rejected" => PbBetStatus::Rejected,
        _ => PbBetStatus::Unspecified,
    }
}
