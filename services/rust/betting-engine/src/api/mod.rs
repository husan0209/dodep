pub mod handlers;

use axum::{
    routing::{get, post},
    Router,
};
use tower::ServiceBuilder;
use tower_http::{
    compression::CompressionLayer,
    cors::CorsLayer,
    request_id::{MakeRequestUuid, PropagateRequestIdLayer, SetRequestIdLayer},
    trace::TraceLayer,
};

use crate::state::AppState;
use crate::middleware::auth;

pub fn build(state: AppState) -> Router {
    let health = Router::new()
        .route("/healthz", get(handlers::health_handler::liveness))
        .route("/readyz", get(handlers::health_handler::readiness));

    let bets = Router::new()
        .route(
            "/api/v1/users/:user_id/bets",
            post(handlers::bet_handler::place_bet)
                .get(handlers::bet_handler::get_history),
        )
        .route(
            "/api/v1/users/:user_id/bets/:bet_id",
            get(handlers::bet_handler::get_bet),
        )
        .route(
            "/api/v1/users/:user_id/bets/:bet_id/cashout",
            post(handlers::bet_handler::cashout_bet),
        )
        .route(
            "/api/v1/bets/:bet_id/settle",
            post(handlers::bet_handler::settle_bet),
        )
        .route(
            "/api/v1/bets/:bet_id/void",
            post(handlers::bet_handler::void_bet),
        );

    // CORS allowlist (safe default: allow none unless configured).
    // Wildcard "*" is explicitly rejected — it is unsafe with credentials
    // and there is no use case for it on a betting API.
    let allowed_origins: Vec<String> = state
        .config()
        .cors_allow_origins
        .split(',')
        .map(|s| s.trim().to_string())
        .filter(|s| !s.is_empty() && s != "*")
        .collect();

    let cors = if allowed_origins.is_empty() {
        CorsLayer::new()
    } else {
        let origins = allowed_origins
            .into_iter()
            .filter_map(|o| o.parse().ok())
            .collect::<Vec<_>>();
        CorsLayer::new()
            .allow_origin(origins)
            .allow_credentials(true)
            .allow_headers([
                axum::http::header::AUTHORIZATION,
                axum::http::header::CONTENT_TYPE,
                axum::http::header::ACCEPT,
                axum::http::HeaderName::from_static("x-request-id"),
                axum::http::HeaderName::from_static("x-idempotency-key"),
            ])
    };

    Router::new()
        .merge(health)
        .merge(
            bets.layer(axum::middleware::from_fn_with_state(
                state.clone(),
                auth::require_auth,
            )),
        )
        .layer(
            ServiceBuilder::new()
                .layer(SetRequestIdLayer::x_request_id(MakeRequestUuid))
                .layer(PropagateRequestIdLayer::x_request_id())
                .layer(CompressionLayer::new())
                .layer(TraceLayer::new_for_http())
                .layer(cors),
        )
        .with_state(state)
}
