use axum::{extract::State, http::StatusCode, Json};
use serde::Serialize;

use crate::state::AppState;

#[derive(Serialize)]
pub struct HealthResponse {
    pub status: &'static str,
}

pub async fn liveness() -> Json<HealthResponse> {
    Json(HealthResponse { status: "ok" })
}

pub async fn readiness(State(state): State<AppState>) -> StatusCode {
    match state.db_pool().acquire().await {
        Ok(_) => StatusCode::OK,
        Err(_) => StatusCode::SERVICE_UNAVAILABLE,
    }
}
