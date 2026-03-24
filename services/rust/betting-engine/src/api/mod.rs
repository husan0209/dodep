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

    Router::new()
        .merge(health)
        .merge(bets)
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
