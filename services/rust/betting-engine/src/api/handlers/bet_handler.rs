use axum::{
    Extension,
    extract::{Path, Query, State},
    http::StatusCode,
    Json,
};
use serde::Deserialize;
use validator::Validate;

use crate::domain::bet::*;
use crate::errors::AppError;
use crate::middleware::auth::{require_admin, AuthUser};
use crate::services::cashout_service::CashoutResponse;
use crate::state::AppState;

#[derive(Debug, Deserialize, Validate)]
pub struct HistoryQuery {
    #[validate(range(min = 1, max = 100))]
    pub limit: Option<i64>,
    pub cursor: Option<i64>,
    pub status: Option<String>,
}

#[derive(Debug, Deserialize)]
pub struct SettleRequest {
    pub result: String,
    pub actual_win: rust_decimal::Decimal,
}

#[tracing::instrument(name = "handler.place_bet", skip(state, req), fields(user_id = %user_id))]
pub async fn place_bet(
    State(state): State<AppState>,
    Extension(user): Extension<AuthUser>,
    Path(user_id): Path<i64>,
    Json(req): Json<PlaceBetRequest>,
) -> Result<(StatusCode, Json<BetResponse>), AppError> {
    if user.id != user_id {
        return Err(AppError::Forbidden { reason: "forbidden".into() });
    }
    let bet = state
        .bet_service()
        .place_bet(UserId(user_id), req)
        .await?;
    Ok((StatusCode::CREATED, Json(BetResponse::from(bet))))
}

#[tracing::instrument(name = "handler.get_bet", skip(state), fields(user_id = %user_id, bet_id = %bet_id))]
pub async fn get_bet(
    State(state): State<AppState>,
    Extension(user): Extension<AuthUser>,
    Path((user_id, bet_id)): Path<(i64, i64)>,
) -> Result<Json<BetResponse>, AppError> {
    if user.id != user_id {
        return Err(AppError::Forbidden { reason: "forbidden".into() });
    }
    let bet = state
        .bet_service()
        .get_bet(UserId(user_id), BetId(bet_id))
        .await?;
    Ok(Json(BetResponse::from(bet)))
}

#[tracing::instrument(name = "handler.get_history", skip(state), fields(user_id = %user_id))]
pub async fn get_history(
    State(state): State<AppState>,
    Extension(user): Extension<AuthUser>,
    Path(user_id): Path<i64>,
    Query(query): Query<HistoryQuery>,
) -> Result<Json<PaginatedResponse<BetResponse>>, AppError> {
    if user.id != user_id {
        return Err(AppError::Forbidden { reason: "forbidden".into() });
    }
    let limit = query.limit.unwrap_or(20).clamp(1, 100);
    let status_filter = query.status.as_deref().and_then(|s| match s {
        "pending" => Some(BetStatus::Pending),
        "active" => Some(BetStatus::Active),
        "won" => Some(BetStatus::Won),
        "lost" => Some(BetStatus::Lost),
        "void" => Some(BetStatus::Void),
        "cashout" => Some(BetStatus::Cashout),
        _ => None,
    });

    let result = state
        .bet_service()
        .get_history(UserId(user_id), limit, query.cursor, status_filter)
        .await?;
    Ok(Json(result))
}

#[tracing::instrument(name = "handler.cashout_bet", skip(state), fields(user_id = %user_id, bet_id = %bet_id))]
pub async fn cashout_bet(
    State(state): State<AppState>,
    Extension(user): Extension<AuthUser>,
    Path((user_id, bet_id)): Path<(i64, i64)>,
) -> Result<Json<CashoutResponse>, AppError> {
    if user.id != user_id {
        return Err(AppError::Forbidden { reason: "forbidden".into() });
    }
    let result = state
        .cashout_service()
        .cashout(UserId(user_id), BetId(bet_id))
        .await?;
    Ok(Json(result))
}

#[tracing::instrument(name = "handler.settle_bet", skip(state), fields(bet_id = %bet_id))]
pub async fn settle_bet(
    State(state): State<AppState>,
    Extension(user): Extension<AuthUser>,
    Path(bet_id): Path<i64>,
    Json(req): Json<SettleRequest>,
) -> Result<Json<BetResponse>, AppError> {
    require_admin(&user).map_err(|_| AppError::Forbidden { reason: "forbidden".into() })?;
    let bet = state
        .settlement_service()
        .settle_bet(BetId(bet_id), &req.result, req.actual_win)
        .await?;
    Ok(Json(BetResponse::from(bet)))
}

#[tracing::instrument(name = "handler.void_bet", skip(state), fields(bet_id = %bet_id))]
pub async fn void_bet(
    State(state): State<AppState>,
    Extension(user): Extension<AuthUser>,
    Path(bet_id): Path<i64>,
) -> Result<Json<BetResponse>, AppError> {
    require_admin(&user).map_err(|_| AppError::Forbidden { reason: "forbidden".into() })?;
    let bet = state.settlement_service().void_bet(BetId(bet_id)).await?;
    Ok(Json(BetResponse::from(bet)))
}
