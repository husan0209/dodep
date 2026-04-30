use axum::{
    http::StatusCode,
    response::{IntoResponse, Response},
    Json,
};
use serde::Serialize;
use thiserror::Error;

#[derive(Debug, Error)]
pub enum AppError {
    #[error("Unauthorized: {reason}")]
    Unauthorized { reason: String },

    #[error("Too many subscriptions: max {max}, current {current}")]
    TooManySubscriptions { max: usize, current: usize },

    #[error("Rate limited, retry after {retry_after_secs}s")]
    RateLimited { retry_after_secs: u64 },

    #[error("Connection limit reached: {current}/{max}")]
    ConnectionLimitReached { current: u64, max: u64 },

    #[error("Invalid topic: {0}")]
    InvalidTopic(String),

    #[error("Kafka consumer error: {0}")]
    KafkaError(String),

    #[error("Internal error")]
    Internal(#[from] anyhow::Error),
}

#[derive(Debug, Serialize)]
struct ErrorResponse {
    error: ErrorBody,
}

#[derive(Debug, Serialize)]
struct ErrorBody {
    code: &'static str,
    message: String,
}

impl IntoResponse for AppError {
    fn into_response(self) -> Response {
        let (status, code, message) = match &self {
            AppError::Unauthorized { reason } => (
                StatusCode::UNAUTHORIZED,
                "WS_UNAUTHORIZED",
                reason.clone(),
            ),
            AppError::TooManySubscriptions { max, current } => (
                StatusCode::BAD_REQUEST,
                "WS_TOO_MANY_SUBSCRIPTIONS",
                format!("Max {max}, current {current}"),
            ),
            AppError::RateLimited { retry_after_secs } => (
                StatusCode::TOO_MANY_REQUESTS,
                "WS_RATE_LIMITED",
                format!("Retry after {retry_after_secs}s"),
            ),
            AppError::ConnectionLimitReached { current, max } => (
                StatusCode::SERVICE_UNAVAILABLE,
                "WS_CONNECTION_LIMIT",
                format!("{current}/{max}"),
            ),
            AppError::InvalidTopic(msg) => (
                StatusCode::BAD_REQUEST,
                "WS_INVALID_TOPIC",
                msg.clone(),
            ),
            AppError::KafkaError(e) => {
                tracing::error!(error = %e, "Kafka error");
                (
                    StatusCode::INTERNAL_SERVER_ERROR,
                    "WS_KAFKA_ERROR",
                    "Event stream error".to_string(),
                )
            }
            AppError::Internal(e) => {
                tracing::error!(error = %e, "Internal error");
                (
                    StatusCode::INTERNAL_SERVER_ERROR,
                    "INTERNAL_ERROR",
                    "Internal error".to_string(),
                )
            }
        };

        let body = ErrorResponse {
            error: ErrorBody { code, message },
        };

        (status, Json(body)).into_response()
    }
}
