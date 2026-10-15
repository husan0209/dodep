# SKILL #12 — rust-error-handling.skill.md

```markdown
# rust-error-handling.skill.md
# GAMBLING PLATFORM — RUST ERROR HANDLING PATTERNS
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: Rust Core Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

Rust has no exceptions. Every error is explicit via Result<T, E>.
This is an advantage — no hidden control flow, no surprise panics.

Our stack: thiserror for library errors, anyhow for application errors,
custom AppError for HTTP/gRPC responses.

# ============================================================
# SECTION 2: ERROR TYPE HIERARCHY
# ============================================================

```text
ERROR LAYERS:

  Library errors (sqlx::Error, fred::Error, tonic::Status)
      ↓ converted via From<> impl
  Domain errors (AppError enum)
      ↓ converted via IntoResponse impl
  HTTP response (StatusCode + JSON body)
      ↓ OR via Into<tonic::Status> impl
  gRPC response (Status code + message)
Rust

// src/errors.rs — THE error type for the entire service
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response, Json};
use rust_decimal::Decimal;
use thiserror::Error;

#[derive(Debug, Error)]
pub enum AppError {
    // ── Client errors (4xx) ──
    
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
    
    #[error("Rate limit exceeded")]
    RateLimited,
    
    // ── Domain-specific errors (4xx, mapped per domain) ──
    
    #[error("Insufficient balance: need {required}, have {available}")]
    InsufficientBalance { required: Decimal, available: Decimal },
    
    #[error("Bet odds changed")]
    OddsChanged { submitted: Decimal, current: Decimal },
    
    #[error("Market closed")]
    MarketClosed { market_id: i64 },
    
    #[error("Event suspended")]
    EventSuspended { event_id: i64 },
    
    #[error("Bet already settled")]
    AlreadySettled,
    
    #[error("Self-excluded")]
    SelfExcluded,
    
    #[error("Concurrency conflict")]
    ConcurrencyConflict,
    
    // ── Server errors (5xx) ──
    
    #[error("Service unavailable: {0}")]
    ServiceUnavailable(String),
    
    #[error("Internal error")]
    Internal(#[from] anyhow::Error),
    
    #[error("Database error")]
    Database(#[from] sqlx::Error),
    
    #[error("Cache error")]
    Cache(#[from] fred::error::RedisError),
}
============================================================
SECTION 3: HTTP RESPONSE MAPPING
============================================================
Rust

impl IntoResponse for AppError {
    fn into_response(self) -> Response {
        let (status, code, message, details) = match &self {
            // 400
            AppError::Validation(fields) => (
                StatusCode::BAD_REQUEST, "VALIDATION_FAILED",
                "Validation failed".into(),
                Some(serde_json::to_value(fields).unwrap_or_default()),
            ),
            
            // 401
            AppError::Unauthorized { reason } => (
                StatusCode::UNAUTHORIZED, "UNAUTHORIZED", reason.clone(), None,
            ),
            
            // 403
            AppError::Forbidden { .. } => (
                StatusCode::FORBIDDEN, "FORBIDDEN", self.to_string(), None,
            ),
            AppError::SelfExcluded => (
                StatusCode::FORBIDDEN, "SELF_EXCLUDED",
                "Account is self-excluded".into(), None,
            ),
            
            // 404
            AppError::NotFound { .. } => (
                StatusCode::NOT_FOUND, "NOT_FOUND", self.to_string(), None,
            ),
            
            // 409
            AppError::Conflict { .. } | AppError::AlreadySettled | AppError::ConcurrencyConflict => (
                StatusCode::CONFLICT, "CONFLICT", self.to_string(), None,
            ),
            AppError::OddsChanged { submitted, current } => (
                StatusCode::CONFLICT, "ODDS_CHANGED",
                "Odds have changed".into(),
                Some(serde_json::json!({
                    "submitted": submitted.to_string(),
                    "current": current.to_string(),
                })),
            ),
            
            // 422
            AppError::InsufficientBalance { required, available } => (
                StatusCode::UNPROCESSABLE_ENTITY, "INSUFFICIENT_BALANCE",
                "Insufficient balance".into(),
                Some(serde_json::json!({
                    "required": required.to_string(),
                    "available": available.to_string(),
                })),
            ),
            AppError::MarketClosed { .. } | AppError::EventSuspended { .. } => (
                StatusCode::UNPROCESSABLE_ENTITY, "MARKET_UNAVAILABLE",
                self.to_string(), None,
            ),
            
            // 429
            AppError::RateLimited => (
                StatusCode::TOO_MANY_REQUESTS, "RATE_LIMITED",
                "Too many requests".into(), None,
            ),
            
            // 503
            AppError::ServiceUnavailable(svc) => (
                StatusCode::SERVICE_UNAVAILABLE, "SERVICE_UNAVAILABLE",
                format!("{svc} is temporarily unavailable"), None,
            ),
            
            // 500 — NEVER expose details
            AppError::Internal(e) => {
                tracing::error!(error = %e, "Internal error");
                (StatusCode::INTERNAL_SERVER_ERROR, "INTERNAL_ERROR",
                 "An internal error occurred".into(), None)
            }
            AppError::Database(e) => {
                tracing::error!(error = %e, "Database error");
                (StatusCode::INTERNAL_SERVER_ERROR, "INTERNAL_ERROR",
                 "An internal error occurred".into(), None)
            }
            AppError::Cache(e) => {
                tracing::error!(error = %e, "Cache error");
                (StatusCode::INTERNAL_SERVER_ERROR, "INTERNAL_ERROR",
                 "An internal error occurred".into(), None)
            }
        };

        let body = serde_json::json!({
            "error": { "code": code, "message": message, "details": details },
            "meta": { "timestamp": chrono::Utc::now().to_rfc3339() }
        });

        (status, Json(body)).into_response()
    }
}
============================================================
SECTION 4: gRPC STATUS MAPPING
============================================================
Rust

impl From<AppError> for tonic::Status {
    fn from(err: AppError) -> Self {
        match err {
            AppError::Validation(_) => tonic::Status::invalid_argument(err.to_string()),
            AppError::Unauthorized { .. } => tonic::Status::unauthenticated(err.to_string()),
            AppError::Forbidden { .. } | AppError::SelfExcluded => {
                tonic::Status::permission_denied(err.to_string())
            }
            AppError::NotFound { .. } => tonic::Status::not_found(err.to_string()),
            AppError::Conflict { .. } | AppError::AlreadySettled => {
                tonic::Status::already_exists(err.to_string())
            }
            AppError::InsufficientBalance { .. } | AppError::MarketClosed { .. }
            | AppError::EventSuspended { .. } => {
                tonic::Status::failed_precondition(err.to_string())
            }
            AppError::OddsChanged { .. } | AppError::ConcurrencyConflict => {
                tonic::Status::aborted(err.to_string())
            }
            AppError::RateLimited => tonic::Status::resource_exhausted(err.to_string()),
            AppError::ServiceUnavailable(_) => tonic::Status::unavailable(err.to_string()),
            _ => {
                tracing::error!(error = %err, "Internal gRPC error");
                tonic::Status::internal("Internal error")
            }
        }
    }
}
============================================================
SECTION 5: CONVERSION TRAITS
============================================================
Rust

// validator → AppError
impl From<validator::ValidationErrors> for AppError {
    fn from(errors: validator::ValidationErrors) -> Self {
        let fields = errors.field_errors().into_iter().flat_map(|(field, errs)| {
            errs.iter().map(move |e| FieldError {
                field: field.to_string(),
                message: e.message.as_ref()
                    .map(|m| m.to_string())
                    .unwrap_or_else(|| format!("Invalid value for {field}")),
            })
        }).collect();
        AppError::Validation(fields)
    }
}

// sqlx → AppError (auto via #[from], but we can add context)
// For specific handling:
pub fn map_db_error(err: sqlx::Error) -> AppError {
    match &err {
        sqlx::Error::RowNotFound => AppError::NotFound {
            entity: "Record".into(),
            id: "unknown".into(),
        },
        sqlx::Error::Database(db_err) => {
            if let Some(code) = db_err.code() {
                match code.as_ref() {
                    "23505" => AppError::Conflict { reason: "Duplicate entry".into() },
                    "23514" => AppError::Validation(vec![FieldError {
                        field: "unknown".into(),
                        message: "Constraint violation".into(),
                    }]),
                    _ => AppError::Database(err),
                }
            } else {
                AppError::Database(err)
            }
        }
        _ => AppError::Database(err),
    }
}
============================================================
SECTION 6: ERROR HANDLING RULES
============================================================
text

1. EVERY function that can fail returns Result<T, E>
2. Use ? operator for propagation (not match everywhere)
3. Add context with .map_err() or anyhow::Context
4. NEVER use .unwrap() in production (except after infallible check)
5. NEVER use .expect() in production (except in init/startup)
6. NEVER use panic!() for error handling
7. NEVER silently ignore errors: _ = might_fail()
   ✅ OK to explicitly ignore: let _ = cache.del(key).await; // best-effort
8. Log at ERROR level: only 500s and unexpected failures
9. Log at WARN level: retries, fallbacks, degraded operation
10. NEVER expose internal error details in API responses
Rust

// ✅ GOOD: Using ? with context
pub async fn get_user_balance(pool: &PgPool, user_id: i64) -> Result<Decimal, AppError> {
    let wallet = sqlx::query_as!(Wallet, "SELECT * FROM wallets WHERE user_id = $1", user_id)
        .fetch_optional(pool)
        .await?   // sqlx::Error → AppError via From
        .ok_or(AppError::NotFound { entity: "Wallet".into(), id: user_id.to_string() })?;
    
    Ok(wallet.balance)
}

// ✅ GOOD: Explicit ignore with comment
let _ = cache.del(&key).await; // best-effort cache invalidation

// ✅ GOOD: Logging non-critical errors
if let Err(e) = producer.publish("bets.placed", &event).await {
    tracing::warn!(error = %e, "Failed to publish event, will retry via outbox");
}

// ❌ BAD: Silent ignore
cache.del(&key).await;  // error silently dropped!

// ❌ BAD: unwrap in production
let user = get_user(id).await.unwrap();  // panics on error!

// ❌ BAD: Exposing internals
AppError::Internal(anyhow!("SQL error: {}", sql_error.message()))  // leaks SQL
============================================================
SECTION 7: FIELD ERROR HELPER
============================================================
Rust

#[derive(Debug, Clone, Serialize)]
pub struct FieldError {
    pub field: String,
    pub message: String,
}

impl FieldError {
    pub fn new(field: impl Into<String>, message: impl Into<String>) -> Self {
        Self { field: field.into(), message: message.into() }
    }
}

// Usage in service layer
fn validate_stake(stake: Decimal) -> Result<(), AppError> {
    let mut errors = vec![];
    
    if stake <= Decimal::ZERO {
        errors.push(FieldError::new("stake", "Must be positive"));
    }
    if stake > dec!(100_000) {
        errors.push(FieldError::new("stake", "Exceeds maximum of 100,000"));
    }
    
    if errors.is_empty() { Ok(()) } else { Err(AppError::Validation(errors)) }
}
============================================================
SECTION 8: TESTING ERRORS
============================================================
Rust

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_insufficient_balance_returns_422() {
        let err = AppError::InsufficientBalance {
            required: dec!(100),
            available: dec!(50),
        };
        let response = err.into_response();
        assert_eq!(response.status(), StatusCode::UNPROCESSABLE_ENTITY);
    }

    #[test]
    fn test_internal_error_hides_details() {
        let err = AppError::Internal(anyhow::anyhow!("secret SQL details"));
        let response = err.into_response();
        assert_eq!(response.status(), StatusCode::INTERNAL_SERVER_ERROR);
        
        let body = response_body_string(response);
        assert!(!body.contains("secret SQL"));
        assert!(body.contains("INTERNAL_ERROR"));
    }

    #[test]
    fn test_validation_error_includes_fields() {
        let err = AppError::Validation(vec![
            FieldError::new("stake", "Must be positive"),
            FieldError::new("odds", "Must be >= 1.01"),
        ]);
        let response = err.into_response();
        assert_eq!(response.status(), StatusCode::BAD_REQUEST);
        
        let body = response_body_string(response);
        assert!(body.contains("stake"));
        assert!(body.contains("odds"));
    }

    #[test]
    fn test_grpc_status_mapping() {
        let err = AppError::NotFound { entity: "Bet".into(), id: "123".into() };
        let status: tonic::Status = err.into();
        assert_eq!(status.code(), tonic::Code::NotFound);
    }
}