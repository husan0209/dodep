//! HTTP API for health, metrics, and admin endpoints

use axum::{
    routing::{get, post},
    Router, Json, extract::State, http::StatusCode, response::IntoResponse,
};
use serde_json::json;
use std::sync::Arc;

use crate::api::AppState;

/// Create HTTP router
pub fn create_router(state: Arc<AppState>) -> Router {
    Router::new()
        .route("/health", get(health_check))
        .route("/ready", get(readiness_check))
        .route("/live", get(liveness_check))
        .route("/metrics", get(metrics))
        .with_state(state)
}

/// Health check handler
async fn health_check(State(state): State<Arc<AppState>>) -> impl IntoResponse {
    // Check database connection
    let db_healthy = sqlx::query("SELECT 1")
        .fetch_one(&state.db_pool)
        .await
        .is_ok();
    
    // Check Redis connection
    let redis_healthy = state.redis_client
        .get_multiplexed_tokio_connection()
        .await
        .is_ok();
    
    let status = if db_healthy && redis_healthy {
        StatusCode::OK
    } else {
        StatusCode::SERVICE_UNAVAILABLE
    };
    
    (status, Json(json!({
        "status": if status == StatusCode::OK { "healthy" } else { "unhealthy" },
        "database": if db_healthy { "ok" } else { "error" },
        "redis": if redis_healthy { "ok" } else { "error" },
        "version": state.config.app.version,
    })))
}

/// Readiness check handler
async fn readiness_check(State(state): State<Arc<AppState>>) -> impl IntoResponse {
    // Check if service is ready to accept traffic
    let ready = state.db_pool.is_connected();
    
    if ready {
        (StatusCode::OK, Json(json!({"ready": true})))
    } else {
        (StatusCode::SERVICE_UNAVAILABLE, Json(json!({"ready": false})))
    }
}

/// Liveness check handler
async fn liveness_check() -> impl IntoResponse {
    // Simple liveness check - just confirms the process is running
    (StatusCode::OK, Json(json!({"alive": true})))
}

/// Metrics handler
async fn metrics(State(state): State<Arc<AppState>>) -> impl IntoResponse {
    // Prometheus metrics are exposed on a separate port
    // This endpoint returns basic service metrics
    (StatusCode::OK, Json(json!({
        "service": state.config.app.name,
        "version": state.config.app.version,
        "environment": state.config.app.env,
    })))
}
