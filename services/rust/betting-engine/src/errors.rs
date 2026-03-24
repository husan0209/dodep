use axum::{
    http::StatusCode,
    response::{IntoResponse, Response},
    Json,
};
use rust_decimal::Decimal;
use serde::Serialize;
use thiserror::Error;

use crate::domain::bet::{EventId, MarketId};

#[derive(Debug, Error)]
pub enum AppError {
    #[error("Validation failed")]
    Validation(Vec<FieldError>),

    #[error("Unauthorized: {reason}")]
    Unauthorized { reason: String },

    #[error("Forbidden: {reason}")]
    Forbidden { reason: String },

    #[error("{entity} not found: {id}")]
    NotFound { entity: String, id: String },

    #[error("Conflict: {reason}")]
    Conflict { reason: String },

    #[error("Event {event_id} not found")]
    BetEventNotFound { event_id: EventId },

    #[error("Event {event_id} is suspended")]
    BetEventSuspended { event_id: EventId },

    #[error("Market {market_id} is closed")]
    BetMarketClosed { market_id: MarketId },

    #[error("Odds changed for selection {index}: submitted {submitted}, current {current}")]
    BetOddsChanged {
        index: usize,
        submitted: Decimal,
        current: Decimal,
    },

    #[error("Stake {actual} below minimum {min}")]
    BetStakeTooLow { min: Decimal, actual: Decimal },

    #[error("Stake {actual} exceeds maximum {max}")]
    BetStakeTooHigh { max: Decimal, actual: Decimal },

    #[error("Max payout {max} exceeded by potential win {potential}")]
    BetMaxPayoutExceeded { max: Decimal, potential: Decimal },

    #[error("Bet rejected: {reason}")]
    BetRejected { reason: String },

    #[error("Bet already settled")]
    BetAlreadySettled,

    #[error("Cashout unavailable")]
    CashoutUnavailable,

    #[error("Insufficient balance: required {required}, available {available}")]
    InsufficientBalance { required: Decimal, available: Decimal },

    #[error("Service unavailable: {0}")]
    ServiceUnavailable(String),

    #[error("Rate limited, retry after {retry_after_secs}s")]
    RateLimited { retry_after_secs: u64 },

    #[error("Internal error")]
    Internal(#[from] anyhow::Error),

    #[error("Database error")]
    Database(#[from] sqlx::Error),

    #[error("Cache error")]
    Cache(String),
}

impl From<validator::ValidationErrors> for AppError {
    fn from(err: validator::ValidationErrors) -> Self {
        let fields: Vec<FieldError> = err
            .field_errors()
            .into_iter()
            .map(|(field, errors)| FieldError {
                field: field.to_string(),
                message: errors
                    .first()
                    .map(|e| e.message.as_ref().map(|m| m.to_string()).unwrap_or_default())
                    .unwrap_or_default(),
            })
            .collect();
        AppError::Validation(fields)
    }
}

#[derive(Debug, Serialize)]
pub struct FieldError {
    pub field: String,
    pub message: String,
}

#[derive(Debug, Serialize)]
struct ErrorResponse {
    error: ErrorBody,
}

#[derive(Debug, Serialize)]
struct ErrorBody {
    code: &'static str,
    message: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    details: Option<serde_json::Value>,
}

impl IntoResponse for AppError {
    fn into_response(self) -> Response {
        let (status, code, message, details) = match &self {
            AppError::Validation(fields) => (
                StatusCode::BAD_REQUEST,
                "VALIDATION_ERROR",
                "Validation failed".to_string(),
                Some(serde_json::to_value(fields).unwrap_or_default()),
            ),
            AppError::Unauthorized { reason } => (
                StatusCode::UNAUTHORIZED,
                "AUTH_UNAUTHORIZED",
                reason.clone(),
                None,
            ),
            AppError::Forbidden { reason } => (
                StatusCode::FORBIDDEN,
                "AUTH_FORBIDDEN",
                reason.clone(),
                None,
            ),
            AppError::NotFound { entity, id } => (
                StatusCode::NOT_FOUND,
                "NOT_FOUND",
                format!("{entity} not found"),
                Some(serde_json::json!({"id": id})),
            ),
            AppError::Conflict { reason } => (
                StatusCode::CONFLICT,
                "CONFLICT",
                reason.clone(),
                None,
            ),
            AppError::BetEventNotFound { event_id } => (
                StatusCode::NOT_FOUND,
                "BET_EVENT_NOT_FOUND",
                format!("Event {event_id} not found"),
                None,
            ),
            AppError::BetEventSuspended { event_id } => (
                StatusCode::UNPROCESSABLE_ENTITY,
                "BET_EVENT_SUSPENDED",
                format!("Event {event_id} suspended"),
                None,
            ),
            AppError::BetMarketClosed { market_id } => (
                StatusCode::UNPROCESSABLE_ENTITY,
                "BET_MARKET_CLOSED",
                format!("Market {market_id} closed"),
                None,
            ),
            AppError::BetOddsChanged { index, submitted, current } => (
                StatusCode::CONFLICT,
                "BET_ODDS_CHANGED",
                "Odds changed".to_string(),
                Some(serde_json::json!({
                    "selection_index": index,
                    "submitted": submitted.to_string(),
                    "current": current.to_string()
                })),
            ),
            AppError::BetStakeTooLow { min, actual } => (
                StatusCode::UNPROCESSABLE_ENTITY,
                "BET_STAKE_TOO_LOW",
                format!("Min stake {min}"),
                Some(serde_json::json!({"min": min.to_string(), "actual": actual.to_string()})),
            ),
            AppError::BetStakeTooHigh { max, actual } => (
                StatusCode::UNPROCESSABLE_ENTITY,
                "BET_STAKE_TOO_HIGH",
                format!("Max stake {max}"),
                Some(serde_json::json!({"max": max.to_string(), "actual": actual.to_string()})),
            ),
            AppError::BetMaxPayoutExceeded { max, potential } => (
                StatusCode::UNPROCESSABLE_ENTITY,
                "BET_MAX_PAYOUT_EXCEEDED",
                format!("Max payout {max}"),
                Some(serde_json::json!({"max_payout": max.to_string(), "potential_win": potential.to_string()})),
            ),
            AppError::BetRejected { reason } => (
                StatusCode::UNPROCESSABLE_ENTITY,
                "BET_REJECTED",
                reason.clone(),
                None,
            ),
            AppError::BetAlreadySettled => (
                StatusCode::CONFLICT,
                "BET_ALREADY_SETTLED",
                "Bet already settled".to_string(),
                None,
            ),
            AppError::CashoutUnavailable => (
                StatusCode::UNPROCESSABLE_ENTITY,
                "BET_CASHOUT_UNAVAILABLE",
                "Cashout unavailable".to_string(),
                None,
            ),
            AppError::InsufficientBalance { required, available } => (
                StatusCode::UNPROCESSABLE_ENTITY,
                "WALLET_INSUFFICIENT_BALANCE",
                "Insufficient balance".to_string(),
                Some(serde_json::json!({"required": required.to_string(), "available": available.to_string()})),
            ),
            AppError::RateLimited { retry_after_secs } => (
                StatusCode::TOO_MANY_REQUESTS,
                "RATE_LIMITED",
                format!("Retry after {retry_after_secs}s"),
                None,
            ),
            AppError::ServiceUnavailable(svc) => (
                StatusCode::SERVICE_UNAVAILABLE,
                "SERVICE_UNAVAILABLE",
                format!("Service unavailable: {svc}"),
                None,
            ),
            AppError::Internal(e) => {
                tracing::error!(error = %e, "Internal error");
                (
                    StatusCode::INTERNAL_SERVER_ERROR,
                    "INTERNAL_ERROR",
                    "Internal error".to_string(),
                    None,
                )
            }
            AppError::Database(e) => {
                tracing::error!(error = %e, "Database error");
                (
                    StatusCode::INTERNAL_SERVER_ERROR,
                    "DATABASE_ERROR",
                    "Database error".to_string(),
                    None,
                )
            }
            AppError::Cache(e) => {
                tracing::error!(error = %e, "Cache error");
                (
                    StatusCode::INTERNAL_SERVER_ERROR,
                    "CACHE_ERROR",
                    "Cache error".to_string(),
                    None,
                )
            }
        };

        let body = ErrorResponse {
            error: ErrorBody { code, message, details },
        };

        (status, Json(body)).into_response()
    }
}
