use axum::{
    extract::{
        ws::{Message, WebSocket, WebSocketUpgrade},
        Query, State,
    },
    response::{IntoResponse, Response},
};
use serde::Deserialize;
use tokio::sync::mpsc;

use crate::domain::channel::{
    parse_topic, ClientMessage, ServerMessage, Topic, TopicResponse,
};
use crate::infrastructure::kafka_consumer::KafkaMessage;
use crate::state::AppState;

#[derive(Debug, Deserialize)]
pub struct WsConnectParams {
    token: String,
}

/// WebSocket upgrade handler
pub async fn ws_handler(
    ws: WebSocketUpgrade,
    State(state): State<AppState>,
    Query(params): Query<WsConnectParams>,
) -> Response {
    // Validate token before upgrade
    // In production: call Auth Service gRPC or validate JWT locally
    // For now: simple token presence check
    if params.token.is_empty() {
        return (
            axum::http::StatusCode::UNAUTHORIZED,
            "Missing token",
        )
            .into_response();
    }

    // Extract user_id from token (placeholder — in production decode JWT)
    // Format: "user_{id}" for testing, or actual JWT decode
    let user_id = extract_user_id(&params.token);

    ws.on_upgrade(move |socket| handle_connection(socket, state, user_id))
}

fn extract_user_id(token: &str) -> i64 {
    // Placeholder: parse "user_123" format
    // Production: decode JWT Ed25519, extract sub claim
    token
        .strip_prefix("user_")
        .and_then(|s| s.parse().ok())
        .unwrap_or(0)
}

async fn handle_connection(socket: WebSocket, state: AppState, user_id: i64) {
    let config = state.config();
    let max_conns = config.max_connections;

    // Check connection limit
    let current = state.subscription_manager().connection_count();
    if current >= max_conns {
        tracing::warn!(current, max_conns, "Connection limit reached");
        let _ = socket
            .send(Message::Text(
                serde_json::to_string(&ServerMessage::Error {
                    code: "WS_CONNECTION_LIMIT".into(),
                    message: "Server at capacity".into(),
                })
                .unwrap_or_default(),
            ))
            .await;
        return;
    }

    let (mut ws_sender, mut ws_receiver) = socket.split();
    let (tx, mut rx) = mpsc::channel::<Message>(config.channel_buffer_size);

    let sub_mgr = state.subscription_manager();
    let max_subs = config.max_subscriptions_per_connection;

    // Track connection
    sub_mgr.increment_connections();
    tracing::info!(user_id, "WebSocket connected");

    // Auto-subscribe to user-specific topics
    if user_id > 0 {
        sub_mgr.subscribe(Topic::UserBets(user_id), tx.clone());
        sub_mgr.subscribe(Topic::UserBalance(user_id), tx.clone());
    }

    // Task 1: Forward messages from channel to WebSocket
    let send_task = tokio::spawn(async move {
        while let Some(msg) = rx.recv().await {
            if ws_sender.send(msg).await.is_err() {
                break;
            }
        }
    });

    // Task 2: Read client messages (subscriptions, pings)
    let tx_clone = tx.clone();
    let sub_mgr_clone = sub_mgr.clone();
    let recv_task = tokio::spawn(async move {
        while let Some(Ok(msg)) = ws_receiver.next().await {
            match msg {
                Message::Text(text) => {
                    handle_client_message(
                        &text,
                        &sub_mgr_clone,
                        &tx_clone,
                        user_id,
                        max_subs,
                    )
                    .await;
                }
                Message::Ping(data) => {
                    let _ = tx_clone.send(Message::Pong(data)).await;
                }
                Message::Close(_) => break,
                _ => {}
            }
        }
    });

    // Task 3: Listen for Kafka broadcasts and forward to this client
    let kafka_task = {
        let mut kafka_rx = state.kafka_receiver();
        let tx_clone = tx.clone();
        tokio::spawn(async move {
            while let Ok(msg) = kafka_rx.recv().await {
                let _ = tx_clone
                    .send(Message::Binary(msg.payload))
                    .await;
            }
        })
    };

    // Wait for any task to finish (connection close)
    tokio::select! {
        _ = send_task => {}
        _ = recv_task => {}
        _ = kafka_task => {}
    }

    // Cleanup
    if user_id > 0 {
        sub_mgr.unsubscribe(&Topic::UserBets(user_id), &tx);
        sub_mgr.unsubscribe(&Topic::UserBalance(user_id), &tx);
    }
    sub_mgr.decrement_connections();

    tracing::info!(user_id, "WebSocket disconnected");
}

async fn handle_client_message(
    text: &str,
    sub_mgr: &crate::infrastructure::connection_manager::SubscriptionManager,
    sender: &mpsc::Sender<Message>,
    user_id: i64,
    max_subs: usize,
) {
    let msg: ClientMessage = match serde_json::from_str(text) {
        Ok(m) => m,
        Err(_) => {
            let _ = sender
                .try_send(Message::Text(
                    serde_json::to_string(&ServerMessage::Error {
                        code: "WS_INVALID_MESSAGE".into(),
                        message: "Invalid JSON".into(),
                    })
                    .unwrap_or_default(),
                ));
            return;
        }
    };

    match msg {
        ClientMessage::Subscribe { topics } => {
            let mut subscribed = Vec::new();
            let current = sub_mgr.total_subscriptions();
            let allowed = max_subs.saturating_sub(current).min(topics.len());

            for topic_req in topics.into_iter().take(allowed) {
                if let Some(topic) = parse_topic(&topic_req) {
                    sub_mgr.subscribe(topic, sender.clone());
                    subscribed.push(TopicResponse {
                        topic_type: topic_req.topic_type,
                        id: topic_req.id,
                    });
                }
            }

            let _ = sender
                .try_send(Message::Text(
                    serde_json::to_string(&ServerMessage::Subscribed {
                        topics: subscribed,
                    })
                    .unwrap_or_default(),
                ));
        }
        ClientMessage::Unsubscribe { topics } => {
            let mut unsubscribed = Vec::new();
            for topic_req in topics {
                if let Some(topic) = parse_topic(&topic_req) {
                    sub_mgr.unsubscribe(&topic, sender);
                    unsubscribed.push(TopicResponse {
                        topic_type: topic_req.topic_type,
                        id: topic_req.id,
                    });
                }
            }

            let _ = sender
                .try_send(Message::Text(
                    serde_json::to_string(&ServerMessage::Unsubscribed {
                        topics: unsubscribed,
                    })
                    .unwrap_or_default(),
                ));
        }
        ClientMessage::Ping => {
            let _ = sender
                .try_send(Message::Text(
                    serde_json::to_string(&ServerMessage::Pong).unwrap_or_default(),
                ));
        }
    }
}
