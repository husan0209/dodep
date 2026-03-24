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
                .layer(CorsLayer::permissive()),
        )
        .with_state(state)
}
