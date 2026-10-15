//! API integration tests for the read-only sports feed.
//!
//! The backing `events` / `markets` tables do not exist yet, so these
//! tests pin the honest `501 Not Implemented` contract (status, error
//! shape, WebSocket fallback details) that mobile relies on instead of
//! hitting an empty route.
//!
//! No database is required: the handlers never touch the pool, so the
//! app is built with a lazy (never-connected) pool.

use axum::{
    body::Body,
    http::{Request, StatusCode},
};
use serde_json::Value;
use tower::ServiceExt;

use betting_engine::{api, config::AppConfig, state::AppState};

fn test_router() -> axum::Router {
    let config = AppConfig {
        service_name: "betting-engine-test".into(),
        http_port: 0,
        grpc_port: 0,
        database_url: String::new(),
        db_max_connections: 1,
        cache_url: String::new(),
        redpanda_brokers: "localhost:1".into(),
        auth_ed25519_public_key: String::new(),
        cors_allow_origins: String::new(),
        sportradar_api_key: String::new(),
        sportradar_live_odds_ws: "wss://localhost".into(),
        sportradar_api_url: "https://localhost".into(),
        sportradar_enabled: false,
        odds_live_ttl_secs: 5,
        odds_prematch_ttl_secs: 30,
    };
    // Lazy pool: never opens a connection (handlers return 501 before any I/O).
    let pool = sqlx::postgres::PgPoolOptions::new()
        .connect_lazy("postgres://localhost:1/nonexistent")
        .expect("test lazy pool URL must parse");
    api::build(AppState::new(config, pool))
}

async fn get_json(uri: &str) -> (StatusCode, Value) {
    let response = test_router()
        .oneshot(Request::builder().uri(uri).body(Body::empty()).unwrap())
        .await
        .unwrap();
    let status = response.status();
    let bytes = axum::body::to_bytes(response.into_body(), 1024 * 1024)
        .await
        .unwrap();
    let json: Value = serde_json::from_slice(&bytes).unwrap();
    (status, json)
}

#[tokio::test]
async fn test_list_sports_events_returns_501_contract() {
    let (status, json) = get_json("/api/v1/sports/events").await;

    assert_eq!(status, StatusCode::NOT_IMPLEMENTED);
    assert_eq!(json["error"]["code"], "SPORTS_FEED_UNAVAILABLE");
    assert_eq!(json["error"]["details"]["resource"], "sports_events");
    assert_eq!(json["error"]["details"]["retryable"], false);

    let ws = &json["error"]["details"]["ws_alternative"];
    assert_eq!(ws["redpanda_topic"], "events.odds_updated");
    assert_eq!(ws["topic_key"], "event_id");
    assert_eq!(
        ws["cache_key_pattern"],
        "odds:{event_id}:{market_id}:{outcome_id}"
    );
    assert_eq!(ws["ttl_live_secs"], 5);
    assert_eq!(ws["ttl_prematch_secs"], 30);
    assert!(ws["fields"]
        .as_array()
        .unwrap()
        .contains(&Value::String("odds".into())));
}

#[tokio::test]
async fn test_list_sports_events_accepts_filters() {
    // sport / live / pagination cursor must parse cleanly (still 501).
    let (status, json) = get_json("/api/v1/sports/events?sport=football&live=true&limit=5").await;

    assert_eq!(status, StatusCode::NOT_IMPLEMENTED);
    assert_eq!(json["error"]["code"], "SPORTS_FEED_UNAVAILABLE");
}

#[tokio::test]
async fn test_list_sports_events_clamps_oversize_limit() {
    // limit=500 is clamped to 100 internally — no 400, still the 501 contract.
    let (status, _) = get_json("/api/v1/sports/events?limit=500&cursor=99").await;
    assert_eq!(status, StatusCode::NOT_IMPLEMENTED);
}

#[tokio::test]
async fn test_get_sports_event_echoes_id() {
    let (status, json) = get_json("/api/v1/sports/events/evt-9").await;

    assert_eq!(status, StatusCode::NOT_IMPLEMENTED);
    assert_eq!(json["error"]["code"], "SPORTS_FEED_UNAVAILABLE");
    assert_eq!(json["error"]["details"]["resource"], "sports_event");
    assert_eq!(json["error"]["details"]["event_id"], "evt-9");
}

#[tokio::test]
async fn test_get_event_odds_echoes_id_with_ws_fallback() {
    let (status, json) = get_json("/api/v1/sports/events/evt-9/odds").await;

    assert_eq!(status, StatusCode::NOT_IMPLEMENTED);
    assert_eq!(json["error"]["code"], "SPORTS_FEED_UNAVAILABLE");
    assert_eq!(json["error"]["details"]["resource"], "event_odds");
    assert_eq!(json["error"]["details"]["event_id"], "evt-9");
    assert_eq!(
        json["error"]["details"]["ws_alternative"]["redpanda_topic"],
        "events.odds_updated"
    );
}
