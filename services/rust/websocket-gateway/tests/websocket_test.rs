//! Integration tests for WebSocket Gateway.
//!
//! The HTTP surface (liveness/readiness and the pre-upgrade auth gate) is
//! exercised in-process against the real router. Everything that would need a
//! live Redpanda broker or a 100k-connection load generator is covered at the
//! unit level instead (topic parsing/routing, subscription accounting and
//! broadcast fan-out), so these tests stay hermetic.

use std::time::Duration;

use axum::{
    body::Body,
    http::{Request, StatusCode},
};
use futures::{SinkExt, StreamExt};
use tower::ServiceExt;
use websocket_gateway::api;
use websocket_gateway::config::AppConfig;
use websocket_gateway::domain::channel::{
    kafka_to_topic, parse_topic, ClientMessage, Topic, TopicRequest, KAFKA_TOPICS,
};
use websocket_gateway::infrastructure::connection_manager::SubscriptionManager;
use websocket_gateway::infrastructure::kafka_consumer::KafkaBroadcaster;
use websocket_gateway::state::AppState;

/// Unused-broker config: `ClientConfig::create` and `subscribe` are lazy, so no
/// connection is attempted while these tests run.
const TEST_BROKERS: &str = "127.0.0.1:1";

fn test_config() -> AppConfig {
    AppConfig {
        service_name: "websocket-gateway-test".into(),
        http_port: 0,
        redpanda_brokers: TEST_BROKERS.into(),
        cache_url: "redis://127.0.0.1:1".into(),
        max_connections: 2,
        channel_buffer_size: 8,
        max_subscriptions_per_connection: 3,
    }
}

fn test_app() -> axum::Router {
    let config = test_config();
    let broadcaster = KafkaBroadcaster::new(&config.redpanda_brokers)
        .expect("KafkaBroadcaster::new must not require a live broker");
    api::build(AppState::new(config, broadcaster))
}

#[tokio::test]
async fn health_endpoints_return_ok() {
    let app = test_app();

    for path in ["/healthz", "/readyz"] {
        let response = app
            .clone()
            .oneshot(
                Request::builder()
                    .uri(path)
                    .body(Body::empty())
                    .expect("request builds"),
            )
            .await
            .expect("router responds");

        assert_eq!(
            response.status(),
            StatusCode::OK,
            "{path} must report healthy"
        );
    }
}

#[tokio::test]
async fn ws_upgrade_requires_token() {
    let app = test_app();

    // Missing and empty tokens are both refused with 401 by the auth gate,
    // before any upgrade happens.
    for uri in ["/ws", "/ws?token=", "/ws?token=&other=1"] {
        let response = app
            .clone()
            .oneshot(
                Request::builder()
                    .uri(uri)
                    .body(Body::empty())
                    .expect("request builds"),
            )
            .await
            .expect("router responds");
        let status = response.status();
        let body = axum::body::to_bytes(response.into_body(), 4096)
            .await
            .expect("body reads");
        assert_eq!(
            status,
            StatusCode::UNAUTHORIZED,
            "{uri} must be rejected before upgrade, got {status}: {}",
            String::from_utf8_lossy(&body)
        );
    }
}

/// A real 101 Switching Protocols handshake needs a live hyper connection
/// (`hyper::upgrade::OnUpgrade` only exists when a connection can actually be
/// upgraded), so this test serves the router on an ephemeral port and drives it
/// with a real WebSocket client.
async fn serve(app: axum::Router) -> std::net::SocketAddr {
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0")
        .await
        .expect("bind ephemeral port");
    let addr = listener.local_addr().expect("local addr");
    tokio::spawn(async move {
        let _ = axum::serve(listener, app).await;
    });
    addr
}

#[tokio::test]
async fn ws_upgrade_with_token_is_accepted_and_echoes_ping() {
    let addr = serve(test_app()).await;
    let url = format!("ws://{addr}/ws?token=user_7");

    let (mut socket, response) = tokio_tungstenite::connect_async(&url)
        .await
        .expect("handshake with a valid token must succeed");
    assert_eq!(
        response.status(),
        StatusCode::SWITCHING_PROTOCOLS,
        "a valid upgrade with a token must return 101"
    );

    socket
        .send(tokio_tungstenite::tungstenite::Message::Text(
            r#"{"action":"ping"}"#.into(),
        ))
        .await
        .expect("ping sends");

    let reply = tokio::time::timeout(Duration::from_secs(5), socket.next())
        .await
        .expect("server answers within 5s")
        .expect("a message arrives")
        .expect("message is not a protocol error");

    assert_eq!(
        reply.into_text().expect("text frame").as_str(),
        r#"{"action":"pong"}"#,
        "the server must answer ping with pong"
    );

    socket.close(None).await.expect("close cleanly");
}

#[tokio::test]
async fn ws_upgrade_without_token_is_refused() {
    let addr = serve(test_app()).await;
    let url = format!("ws://{addr}/ws?token=");
    let status = handshake_status(&url)
        .await
        .expect("server sent an HTTP response");
    assert_eq!(
        status,
        StatusCode::UNAUTHORIZED,
        "a handshake without a token must be refused with 401"
    );
}

/// Performs a WebSocket handshake and returns the HTTP status the server
/// replied with, for both accepted (101) and rejected handshakes.
async fn handshake_status(url: &str) -> Option<StatusCode> {
    match tokio_tungstenite::connect_async(url).await {
        Ok((mut socket, response)) => {
            let status = Some(response.status());
            let _ = socket.close(None).await;
            status
        }
        Err(tokio_tungstenite::tungstenite::Error::Http(response)) => Some(response.status()),
        Err(_) => None,
    }
}

#[tokio::test]
async fn ws_connection_limit_is_enforced() {
    let addr = serve(test_app()).await; // test_config() sets max_connections = 2

    let mut sockets = Vec::new();
    for _ in 0..2 {
        let (socket, response) =
            tokio_tungstenite::connect_async(format!("ws://{addr}/ws?token=user_1"))
                .await
                .expect("the first two connections are accepted");
        assert_eq!(response.status(), StatusCode::SWITCHING_PROTOCOLS);
        sockets.push(socket);
    }

    let status = handshake_status(&format!("ws://{addr}/ws?token=user_1"))
        .await
        .expect("server sent an HTTP response");
    assert_eq!(
        status,
        StatusCode::SERVICE_UNAVAILABLE,
        "exceeding max_connections must yield 503"
    );

    for mut socket in sockets {
        let _ = socket.close(None).await;
    }
}

#[tokio::test]
async fn unknown_route_is_not_found() {
    let app = test_app();
    let response = app
        .oneshot(
            Request::builder()
                .uri("/definitely-not-a-route")
                .body(Body::empty())
                .expect("request builds"),
        )
        .await
        .expect("router responds");

    assert_eq!(response.status(), StatusCode::NOT_FOUND);
}

#[test]
fn topic_requests_parse_into_known_topics() {
    let cases = [
        ("event_odds", 1_i64, Topic::EventOdds(1)),
        ("event_stats", 2, Topic::EventStats(2)),
        ("sport_scores", 3, Topic::SportScores(3)),
        ("user_bets", 4, Topic::UserBets(4)),
        ("user_balance", 5, Topic::UserBalance(5)),
    ];

    for (topic_type, id, expected) in cases {
        let req = TopicRequest {
            topic_type: topic_type.into(),
            id,
        };
        assert_eq!(parse_topic(&req), Some(expected), "{topic_type} must parse");
    }

    let unknown = TopicRequest {
        topic_type: "not_a_topic".into(),
        id: 9,
    };
    assert_eq!(
        parse_topic(&unknown),
        None,
        "unknown topic types must be rejected"
    );
}

#[test]
fn kafka_events_map_to_broadcast_topics() {
    // User-scoped Kafka topics are keyed by user id.
    for kafka_topic in ["bets.bet.placed", "bets.bet.settled", "bets.bet.cashout"] {
        assert_eq!(
            kafka_to_topic(kafka_topic, "42"),
            Some(Topic::UserBets(42)),
            "{kafka_topic} must route to the user's bet stream"
        );
    }

    assert_eq!(
        kafka_to_topic("events.odds_updated", "7"),
        Some(Topic::EventOdds(7))
    );
    assert_eq!(
        kafka_to_topic("analytics.events", "7"),
        Some(Topic::EventStats(7))
    );

    // Non-numeric keys and unsubscribed topics must not produce a Topic.
    assert_eq!(kafka_to_topic("events.odds_updated", "not-a-number"), None);
    assert_eq!(kafka_to_topic("some.unrelated.topic", "1"), None);
}

#[test]
fn subscription_manager_accounts_for_topics() {
    let mgr = SubscriptionManager::new();
    let (tx, _rx) = tokio::sync::mpsc::channel(4);
    let topic = Topic::EventOdds(1);

    assert_eq!(mgr.subscriber_count(&topic), 0);
    assert_eq!(mgr.total_subscriptions(), 0);

    mgr.subscribe(topic.clone(), tx.clone());
    mgr.subscribe(topic.clone(), tx);
    assert_eq!(mgr.subscriber_count(&topic), 2);
    assert_eq!(mgr.total_subscriptions(), 2);

    // Unsubscribing removes exactly the given sender.
    let (other_tx, _other_rx) = tokio::sync::mpsc::channel(4);
    mgr.subscribe(topic.clone(), other_tx.clone());
    assert_eq!(mgr.subscriber_count(&topic), 3);

    let (to_remove, _to_remove_rx) = tokio::sync::mpsc::channel(4);
    mgr.subscribe(topic.clone(), to_remove.clone());
    mgr.unsubscribe(&topic, &to_remove);
    assert_eq!(
        mgr.subscriber_count(&topic),
        3,
        "unsubscribe must only drop the matching sender"
    );
}

#[tokio::test]
async fn closed_subscribers_are_reaped_on_broadcast() {
    let mgr = SubscriptionManager::new();
    let topic = Topic::EventOdds(1);

    let (dropped_tx, dropped_rx) = tokio::sync::mpsc::channel(4);
    let (live_tx, mut live_rx) = tokio::sync::mpsc::channel(4);
    drop(dropped_rx);

    mgr.subscribe(topic.clone(), dropped_tx);
    mgr.subscribe(topic.clone(), live_tx);

    mgr.broadcast(&topic, b"payload");

    assert_eq!(
        mgr.subscriber_count(&topic),
        1,
        "closed senders must be dropped during broadcast"
    );
    assert_eq!(
        live_rx.recv().await,
        Some(axum::extract::ws::Message::Binary(b"payload".to_vec())),
        "the live subscriber must receive the broadcast payload"
    );
}

#[tokio::test]
async fn connection_count_tracks_increment_and_decrement() {
    let mgr = SubscriptionManager::new();
    assert_eq!(mgr.connection_count(), 0);

    mgr.increment_connections();
    mgr.increment_connections();
    assert_eq!(mgr.connection_count(), 2);

    mgr.decrement_connections();
    assert_eq!(mgr.connection_count(), 1);
}

#[test]
fn client_messages_use_action_tagged_json() {
    let subscribe: ClientMessage =
        serde_json::from_str(r#"{"action":"subscribe","topics":[{"type":"event_odds","id":1}]}"#)
            .expect("subscribe envelope deserializes");
    match subscribe {
        ClientMessage::Subscribe { topics } => {
            assert_eq!(topics.len(), 1);
            assert_eq!(topics[0].topic_type, "event_odds");
            assert_eq!(topics[0].id, 1);
        }
        other => panic!("expected Subscribe, got {other:?}"),
    }

    let ping: ClientMessage =
        serde_json::from_str(r#"{"action":"ping"}"#).expect("ping envelope deserializes");
    assert!(matches!(ping, ClientMessage::Ping));

    assert!(
        serde_json::from_str::<ClientMessage>(r#"{"action":"nope"}"#).is_err(),
        "unknown actions must be rejected"
    );
}

#[test]
fn subscribed_kafka_topics_are_declared() {
    assert!(
        KAFKA_TOPICS.contains(&"events.odds_updated"),
        "odds updates must be subscribed"
    );
    assert!(
        KAFKA_TOPICS.contains(&"bets.bet.placed"),
        "bet placements must be subscribed"
    );
    // Every subscribed topic must have a routing rule, otherwise messages are
    // silently dropped.
    for topic in KAFKA_TOPICS {
        assert!(
            kafka_to_topic(topic, "1").is_some(),
            "subscribed topic {topic} has no kafka_to_topic mapping"
        );
    }
}

// Load test scenarios (require k6 or similar)
//
// 100K concurrent connections:
//   - k6 script connecting 100K WS clients
//   - Subscribe to random event topics
//   - Measure latency from Redpanda to client delivery
//   - Target: p99 < 10ms, p50 < 5ms
//
// Message throughput:
//   - Send 1M messages/sec through Kafka
//   - Verify all subscribers receive messages
//   - Measure broadcast latency
//
// Reconnection:
//   - Disconnect 50% of clients
//   - Verify they can reconnect
//   - Verify subscriptions are re-established
