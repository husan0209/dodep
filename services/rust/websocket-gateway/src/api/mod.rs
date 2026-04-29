pub mod handlers;

use axum::{
    routing::get,
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

/// Build the CORS layer from $CORS_ORIGINS (comma-separated).
/// - "*" entries are stripped (wildcard is unsafe with credentials).
/// - Empty / unset → no allowed origins (browser fetch denied; WS handshake
///   from a configured origin still works because it uses `Origin` header
///   match, which we explicitly allowlist below).
fn cors_layer_from_env() -> CorsLayer {
    let raw = std::env::var("CORS_ORIGINS").unwrap_or_default();
    let origins: Vec<axum::http::HeaderValue> = raw
        .split(',')
        .map(str::trim)
        .filter(|s| !s.is_empty() && *s != "*")
        .filter_map(|s| s.parse().ok())
        .collect();

    if origins.is_empty() {
        // Safe default: deny browser CORS (does not block server-to-server).
        CorsLayer::new()
    } else {
        CorsLayer::new()
            .allow_origin(origins)
            .allow_credentials(true)
            .allow_headers([
                axum::http::header::AUTHORIZATION,
                axum::http::header::CONTENT_TYPE,
                axum::http::header::ACCEPT,
            ])
    }
}

pub fn build(state: AppState) -> Router {
    let health = Router::new()
        .route("/healthz", get(handlers::health_handler::liveness))
        .route("/readyz", get(handlers::health_handler::readiness));

    let ws = Router::new()
        .route("/ws", get(handlers::ws_handler::ws_handler));

    Router::new()
        .merge(health)
        .merge(ws)
        .layer(
            ServiceBuilder::new()
                .layer(SetRequestIdLayer::x_request_id(MakeRequestUuid))
                .layer(PropagateRequestIdLayer::x_request_id())
                .layer(CompressionLayer::new())
                .layer(TraceLayer::new_for_http())
                .layer(cors_layer_from_env()),
        )
        .with_state(state)
}
