# SKILL #7 — rust-axum-handlers.skill.md

```markdown
# rust-axum-handlers.skill.md
# GAMBLING PLATFORM — RUST AXUM HANDLER PATTERNS
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: Rust Core Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

All Rust HTTP services use Axum 0.8 (tokio-based).
Handlers are the THINNEST layer — extract input, call service, return output.
p99 target: < 5ms per handler.

# ============================================================
# SECTION 2: HANDLER RULES
# ============================================================

```text
1. Handler ONLY: extract → validate format → call service → return
2. Handler body: 3-10 lines (if longer, logic belongs in service)
3. Handler NEVER: queries DB, calls cache, contains business rules
4. Handler NEVER: imports repository module
5. Handler uses: axum extractors (State, Path, Query, Json, Extension)
6. Handler returns: Result<(StatusCode, Json<T>), AppError>
7. Handler decorates with: #[tracing::instrument] for observability
8. Handler validates: input FORMAT (via validator crate)
9. Handler does NOT validate: business rules (service does that)
10. Auth check: via middleware extractor (AuthUser)
============================================================
SECTION 3: EXTRACTOR PATTERNS
============================================================
Rust

use axum::{
    extract::{Json, Path, Query, State},
    http::StatusCode,
};
use uuid::Uuid;
use validator::Validate;

use crate::domain::*;
use crate::errors::AppError;
use crate::middleware::auth::AuthUser;
use crate::state::AppState;

// ── Path parameters ──
// Axum extracts from URL path segments

#[tracing::instrument(name = "get_bet", skip(state), fields(user_id = %user.id))]
pub async fn get_bet(
    State(state): State<AppState>,
    user: AuthUser,
    Path(bet_id): Path<i64>,
) -> Result<Json<BetResponse>, AppError> {
    let bet = state.bet_service().get_bet(user.id, BetId(bet_id)).await?;
    Ok(Json(BetResponse::from(bet)))
}

// ── Query parameters ──
// For filtering and pagination

#[derive(Debug, Deserialize, Validate)]
pub struct BetHistoryQuery {
    #[validate(range(min = 1, max = 100))]
    pub page_size: Option<i64>,
    pub cursor: Option<String>,
    pub status: Option<String>,
    pub sport: Option<String>,
}

#[tracing::instrument(name = "get_bet_history", skip(state), fields(user_id = %user.id))]
pub async fn get_bet_history(
    State(state): State<AppState>,
    user: AuthUser,
    Query(params): Query<BetHistoryQuery>,
) -> Result<Json<PaginatedResponse<BetResponse>>, AppError> {
    params.validate()?;
    let result = state.bet_service().get_history(user.id, params.into()).await?;
    Ok(Json(result))
}

// ── JSON body ──
// For POST/PUT/PATCH requests

#[tracing::instrument(name = "place_bet", skip(state, req), fields(user_id = %user.id))]
pub async fn place_bet(
    State(state): State<AppState>,
    user: AuthUser,
    Json(req): Json<PlaceBetRequest>,
) -> Result<(StatusCode, Json<BetResponse>), AppError> {
    req.validate()?;
    let bet = state.bet_service().place_bet(user.id, req).await?;
    Ok((StatusCode::CREATED, Json(BetResponse::from(bet))))
}

// ── No body (action endpoints) ──

#[tracing::instrument(name = "cashout_bet", skip(state), fields(user_id = %user.id))]
pub async fn cashout_bet(
    State(state): State<AppState>,
    user: AuthUser,
    Path(bet_id): Path<i64>,
) -> Result<Json<CashoutResponse>, AppError> {
    let result = state.cashout_service().cashout(user.id, BetId(bet_id)).await?;
    Ok(Json(result))
}

// ── Delete (returns 204) ──

pub async fn delete_session(
    State(state): State<AppState>,
    user: AuthUser,
    Path(session_id): Path<String>,
) -> Result<StatusCode, AppError> {
    state.session_service().delete(user.id, &session_id).await?;
    Ok(StatusCode::NO_CONTENT)
}
============================================================
SECTION 4: AUTH EXTRACTOR
============================================================
Rust

use axum::{
    async_trait,
    extract::FromRequestParts,
    http::{header, request::Parts},
};

/// Extracts authenticated user from JWT in Authorization header.
/// Implement as FromRequestParts so it works as handler parameter.
pub struct AuthUser {
    pub id: UserId,
    pub roles: Vec<String>,
    pub permissions: Vec<String>,
    pub token_id: String,
}

#[async_trait]
impl<S> FromRequestParts<S> for AuthUser
where
    S: Send + Sync,
    AppState: FromRef<S>,
{
    type Rejection = AppError;

    async fn from_request_parts(parts: &mut Parts, state: &S) -> Result<Self, Self::Rejection> {
        let state = AppState::from_ref(state);
        
        let auth_header = parts.headers
            .get(header::AUTHORIZATION)
            .and_then(|v| v.to_str().ok())
            .ok_or(AppError::Unauthorized { reason: "Missing Authorization header".into() })?;

        let token = auth_header
            .strip_prefix("Bearer ")
            .ok_or(AppError::Unauthorized { reason: "Invalid token format".into() })?;

        let claims = state.token_service()
            .validate_access_token(token)
            .map_err(|_| AppError::Unauthorized { reason: "Invalid token".into() })?;

        Ok(AuthUser {
            id: UserId(claims.sub),
            roles: claims.roles,
            permissions: claims.permissions,
            token_id: claims.jti,
        })
    }
}
============================================================
SECTION 5: ROUTER SETUP
============================================================
Rust

use axum::{
    middleware as axum_mw,
    routing::{get, post, delete},
    Router,
};
use tower::ServiceBuilder;
use tower_http::{
    catch_panic::CatchPanicLayer,
    compression::CompressionLayer,
    cors::CorsLayer,
    request_id::{MakeRequestUuid, SetRequestIdLayer, PropagateRequestIdLayer},
    timeout::TimeoutLayer,
    trace::TraceLayer,
};
use std::time::Duration;

pub fn build(state: AppState) -> Router {
    // Public routes (no auth)
    let health = Router::new()
        .route("/healthz", get(handlers::health::liveness))
        .route("/readyz", get(handlers::health::readiness))
        .route("/metrics", get(handlers::health::metrics));

    // Protected routes (require auth via AuthUser extractor)
    let bets = Router::new()
        .route("/api/v1/bets", post(handlers::bet::place_bet))
        .route("/api/v1/bets/:id", get(handlers::bet::get_bet))
        .route("/api/v1/bets/:id/cashout", post(handlers::bet::cashout_bet))
        .route("/api/v1/bets/history", get(handlers::bet::get_bet_history))
        .route("/api/v1/bets/active", get(handlers::bet::get_active_bets));

    // Combine with middleware
    Router::new()
        .merge(health)
        .merge(bets)
        .layer(
            ServiceBuilder::new()
                .layer(SetRequestIdLayer::x_request_id(MakeRequestUuid))
                .layer(PropagateRequestIdLayer::x_request_id())
                .layer(CatchPanicLayer::new())
                .layer(TimeoutLayer::new(Duration::from_secs(30)))
                .layer(CompressionLayer::new())
                .layer(TraceLayer::new_for_http())
        )
        .with_state(state)
}
============================================================
SECTION 6: REQUEST VALIDATION
============================================================
Rust

use serde::Deserialize;
use validator::Validate;
use rust_decimal::Decimal;
use uuid::Uuid;

#[derive(Debug, Deserialize, Validate)]
pub struct PlaceBetRequest {
    pub bet_type: BetType,
    
    #[validate(length(min = 1, max = 20, message = "1-20 selections required"))]
    pub selections: Vec<SelectionRequest>,
    
    #[validate(custom(function = "validate_positive_decimal"))]
    pub stake: Decimal,
    
    #[validate(length(equal = 3, message = "Currency code must be 3 characters"))]
    pub currency_code: String,
    
    pub idempotency_key: Uuid,
    
    #[serde(default)]
    pub accept_odds_changes: AcceptOddsChanges,
}

#[derive(Debug, Deserialize, Validate)]
pub struct SelectionRequest {
    pub event_id: i64,
    pub market_id: i64,
    pub outcome_id: i64,
    
    #[validate(custom(function = "validate_odds_range"))]
    pub odds: Decimal,
}

fn validate_positive_decimal(value: &Decimal) -> Result<(), validator::ValidationError> {
    if *value <= Decimal::ZERO {
        let mut err = validator::ValidationError::new("positive");
        err.message = Some("Must be positive".into());
        return Err(err);
    }
    Ok(())
}

fn validate_odds_range(value: &Decimal) -> Result<(), validator::ValidationError> {
    if *value < dec!(1.01) || *value > dec!(1001.0) {
        let mut err = validator::ValidationError::new("odds_range");
        err.message = Some("Odds must be between 1.01 and 1001.00".into());
        return Err(err);
    }
    Ok(())
}

// validator::ValidationErrors automatically converts to AppError
// via From<validator::ValidationErrors> for AppError implementation
============================================================
SECTION 7: HEALTH CHECK HANDLERS
============================================================
Rust

pub async fn liveness() -> StatusCode {
    StatusCode::OK
}

pub async fn readiness(State(state): State<AppState>) -> StatusCode {
    // Check all critical dependencies
    let db_ok = state.db_pool().acquire().await.is_ok();
    let cache_ok = state.cache().ping().await.is_ok();
    
    if db_ok && cache_ok {
        StatusCode::OK
    } else {
        StatusCode::SERVICE_UNAVAILABLE
    }
}

pub async fn metrics() -> String {
    // Prometheus metrics endpoint
    let encoder = prometheus::TextEncoder::new();
    let families = prometheus::gather();
    encoder.encode_to_string(&families).unwrap_or_default()
}
============================================================
SECTION 8: ANTI-PATTERNS
============================================================
Rust

// ❌ BAD: Business logic in handler
pub async fn place_bet(State(s): State<AppState>, Json(r): Json<Req>) -> Result<Json<Res>, AppError> {
    let balance = sqlx::query!("SELECT balance FROM wallets...").fetch_one(&s.pool).await?;
    if balance < r.stake { return Err(AppError::InsufficientBalance); } // ❌ logic in handler
    // ... 50 more lines
}

// ❌ BAD: Calling repo directly
pub async fn get_bet(State(s): State<AppState>, Path(id): Path<i64>) -> ... {
    let bet = s.bet_repo.get_by_id(id).await?; // ❌ handler → repo directly
}

// ❌ BAD: Not using extractors
pub async fn place_bet(req: Request<Body>) -> ... {
    let body = to_bytes(req.into_body()).await?; // ❌ manual parsing
    let data: Req = serde_json::from_slice(&body)?;
}

// ❌ BAD: Handler returning raw SQL errors
pub async fn get_bet(...) -> ... {
    let bet = sqlx::query!(...).fetch_one(&pool).await?; // ❌ sqlx error exposed
}

// ✅ GOOD: Thin handler
pub async fn place_bet(
    State(state): State<AppState>,
    user: AuthUser,
    Json(req): Json<PlaceBetRequest>,
) -> Result<(StatusCode, Json<BetResponse>), AppError> {
    req.validate()?;
    let bet = state.bet_service().place_bet(user.id, req).await?;
    Ok((StatusCode::CREATED, Json(BetResponse::from(bet))))
}