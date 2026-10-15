# SKILL #11 — rust-websocket.skill.md

```markdown
# rust-websocket.skill.md
# GAMBLING PLATFORM — RUST WEBSOCKET PATTERNS
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: Rust Core Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

WebSocket gateway delivers real-time updates to clients:
- Live odds changes (every 1-5 seconds per event)
- Live scores and match statistics
- User bet status updates
- Balance changes

Target: 100K concurrent connections per instance.
Memory: < 5KB per connection overhead.
Latency: < 50ms from Redpanda message to client delivery.

# ============================================================
# SECTION 2: CONNECTION ARCHITECTURE
# ============================================================

```text
┌──────────┐     ┌─────────────────┐     ┌──────────────┐
│  Client  │────▶│  WS Gateway     │◀────│  Redpanda    │
│  (Web/   │ WSS │  (Rust/Axum)    │     │  Consumer    │
│   Mobile)│◀────│                 │     │              │
└──────────┘     │  ┌───────────┐  │     └──────────────┘
                 │  │Subscription│  │
                 │  │   Map      │  │
                 │  │(DashMap)   │  │
                 │  └───────────┘  │
                 └─────────────────┘

Per instance: up to 100K connections
Instances: 5-20 (auto-scaled by connection count)
Sticky sessions: NOT required (each instance independent)
============================================================
SECTION 3: CONNECTION HANDLING
============================================================
Rust

use axum::{
    extract::{ws::{Message, WebSocket, WebSocketUpgrade}, State, Query},
    response::Response,
};
use dashmap::DashMap;
use std::sync::Arc;
use tokio::sync::mpsc;

/// Subscription topics a client can subscribe to
#[derive(Debug, Clone, Hash, Eq, PartialEq)]
pub enum Topic {
    EventOdds(i64),       // odds updates for event_id
    EventStats(i64),      // live stats for event_id
    SportScores(i64),     // live scores for sport_id
    UserBets(i64),        // bet status for user_id
    UserBalance(i64),     // balance changes for user_id
}

/// Per-connection sender (cheaply cloneable)
type ClientSender = mpsc::Sender<Message>;

/// Global subscription map: topic → set of client senders
pub struct SubscriptionManager {
    subscriptions: DashMap<Topic, Vec<ClientSender>>,
    connection_count: Arc<std::sync::atomic::AtomicU64>,
}

impl SubscriptionManager {
    pub fn new() -> Self {
        Self {
            subscriptions: DashMap::new(),
            connection_count: Arc::new(std::sync::atomic::AtomicU64::new(0)),
        }
    }

    pub fn subscribe(&self, topic: Topic, sender: ClientSender) {
        self.subscriptions
            .entry(topic)
            .or_default()
            .push(sender);
    }

    pub fn unsubscribe(&self, topic: &Topic, sender: &ClientSender) {
        if let Some(mut senders) = self.subscriptions.get_mut(topic) {
            senders.retain(|s| !s.is_closed() && !Arc::ptr_eq(&Arc::new(s), &Arc::new(sender)));
        }
    }

    /// Broadcast message to all subscribers of a topic
    pub async fn broadcast(&self, topic: &Topic, message: &[u8]) {
        if let Some(mut senders) = self.subscriptions.get_mut(topic) {
            let msg = Message::Binary(message.to_vec());
            
            // Remove closed senders and send to active ones
            senders.retain(|sender| {
                if sender.is_closed() {
                    return false;
                }
                // try_send: non-blocking, drops if buffer full
                sender.try_send(msg.clone()).is_ok()
            });
        }
    }

    pub fn connection_count(&self) -> u64 {
        self.connection_count.load(std::sync::atomic::Ordering::Relaxed)
    }
}
============================================================
SECTION 4: UPGRADE AND MESSAGE LOOP
============================================================
Rust

/// WebSocket upgrade handler
pub async fn ws_handler(
    ws: WebSocketUpgrade,
    State(state): State<AppState>,
    Query(params): Query<WsConnectParams>,
) -> Response {
    // Validate JWT before upgrade
    let claims = match state.token_service().validate(&params.token) {
        Ok(c) => c,
        Err(_) => return (axum::http::StatusCode::UNAUTHORIZED, "Invalid token").into_response(),
    };

    ws.on_upgrade(move |socket| handle_connection(socket, state, claims))
}

#[derive(Deserialize)]
pub struct WsConnectParams {
    token: String,
}

async fn handle_connection(socket: WebSocket, state: AppState, claims: TokenClaims) {
    let (mut ws_sender, mut ws_receiver) = socket.split();
    let (tx, mut rx) = mpsc::channel::<Message>(256); // per-client buffer
    
    let user_id = claims.sub;
    let sub_mgr = state.subscription_manager();
    
    // Track connection
    sub_mgr.connection_count.fetch_add(1, std::sync::atomic::Ordering::Relaxed);
    metrics::WS_CONNECTIONS.inc();
    
    tracing::info!(user_id, "WebSocket connected");

    // Auto-subscribe to user-specific topics
    sub_mgr.subscribe(Topic::UserBets(user_id), tx.clone());
    sub_mgr.subscribe(Topic::UserBalance(user_id), tx.clone());

    // Task 1: Forward messages from channel to WebSocket
    let send_task = tokio::spawn(async move {
        while let Some(msg) = rx.recv().await {
            if ws_sender.send(msg).await.is_err() {
                break; // connection closed
            }
        }
    });

    // Task 2: Read client messages (subscriptions, pings)
    let recv_task = {
        let tx = tx.clone();
        let sub_mgr = sub_mgr.clone();
        tokio::spawn(async move {
            while let Some(Ok(msg)) = ws_receiver.next().await {
                match msg {
                    Message::Text(text) => {
                        handle_client_message(&text, &sub_mgr, &tx, user_id).await;
                    }
                    Message::Ping(data) => {
                        let _ = tx.send(Message::Pong(data)).await;
                    }
                    Message::Close(_) => break,
                    _ => {}
                }
            }
        })
    };

    // Wait for either task to finish (connection close)
    tokio::select! {
        _ = send_task => {}
        _ = recv_task => {}
    }

    // Cleanup
    sub_mgr.connection_count.fetch_sub(1, std::sync::atomic::Ordering::Relaxed);
    metrics::WS_CONNECTIONS.dec();
    tracing::info!(user_id, "WebSocket disconnected");
}
============================================================
SECTION 5: CLIENT MESSAGE PROTOCOL
============================================================
Rust

// Client sends JSON messages to subscribe/unsubscribe

#[derive(Deserialize)]
#[serde(tag = "action")]
enum ClientMessage {
    #[serde(rename = "subscribe")]
    Subscribe { topics: Vec<TopicRequest> },
    #[serde(rename = "unsubscribe")]
    Unsubscribe { topics: Vec<TopicRequest> },
    #[serde(rename = "ping")]
    Ping,
}

#[derive(Deserialize)]
struct TopicRequest {
    #[serde(rename = "type")]
    topic_type: String,  // "event_odds", "sport_scores"
    id: i64,
}

async fn handle_client_message(
    text: &str,
    sub_mgr: &SubscriptionManager,
    sender: &ClientSender,
    user_id: i64,
) {
    let msg: ClientMessage = match serde_json::from_str(text) {
        Ok(m) => m,
        Err(_) => return, // ignore invalid messages
    };

    match msg {
        ClientMessage::Subscribe { topics } => {
            // Limit: max 50 subscriptions per connection
            let current = sub_mgr.user_subscription_count(user_id);
            let allowed = (50 - current).min(topics.len());
            
            for topic_req in topics.into_iter().take(allowed) {
                if let Some(topic) = parse_topic(&topic_req) {
                    sub_mgr.subscribe(topic, sender.clone());
                }
            }
        }
        ClientMessage::Unsubscribe { topics } => {
            for topic_req in topics {
                if let Some(topic) = parse_topic(&topic_req) {
                    sub_mgr.unsubscribe(&topic, sender);
                }
            }
        }
        ClientMessage::Ping => {
            let _ = sender.try_send(Message::Text(r#"{"action":"pong"}"#.into()));
        }
    }
}

fn parse_topic(req: &TopicRequest) -> Option<Topic> {
    match req.topic_type.as_str() {
        "event_odds" => Some(Topic::EventOdds(req.id)),
        "event_stats" => Some(Topic::EventStats(req.id)),
        "sport_scores" => Some(Topic::SportScores(req.id)),
        _ => None,
    }
}
============================================================
SECTION 6: REDPANDA CONSUMER → BROADCAST
============================================================
Rust

/// Consume odds updates from Redpanda and broadcast to subscribers
pub async fn start_odds_broadcaster(
    sub_mgr: Arc<SubscriptionManager>,
    redpanda_config: &RedpandaConfig,
) {
    let consumer = create_consumer(redpanda_config, "ws-gateway", &["events.odds_updated"]);
    
    loop {
        match consumer.poll(Duration::from_millis(100)).await {
            Some(Ok(message)) => {
                let event_id = extract_key_as_i64(&message);
                let topic = Topic::EventOdds(event_id);
                
                // Broadcast raw protobuf bytes to all subscribers
                sub_mgr.broadcast(&topic, message.payload()).await;
                
                metrics::WS_MESSAGES_SENT.inc();
            }
            Some(Err(e)) => {
                tracing::error!(error = %e, "Redpanda consume error");
            }
            None => {} // no message, loop again
        }
    }
}
============================================================
SECTION 7: ANTI-PATTERNS
============================================================
text

❌ NEVER accept WebSocket without JWT validation BEFORE upgrade
❌ NEVER allow unlimited subscriptions per connection (cap at 50)
❌ NEVER use unbounded channel per client (memory bomb with slow clients)
❌ NEVER broadcast by iterating all connections (use topic-based fan-out)
❌ NEVER send JSON for high-frequency data (use Protobuf binary)
❌ NEVER skip heartbeat/ping-pong (detect dead connections)
❌ NEVER hold subscription lock during broadcast (use DashMap for lock-free)
❌ NEVER store WebSocket state in external DB (in-memory only, stateless instances)
❌ NEVER block the broadcast loop on a slow client (use try_send, drop if full)
============================================================
SECTION 8: TESTING
============================================================
text

MUST TEST:
  ✅ Connection with valid JWT succeeds, invalid JWT rejected
  ✅ Subscribe → receive matching events, not unrelated events
  ✅ Unsubscribe → stop receiving events for that topic
  ✅ 50 subscription limit enforced
  ✅ Slow client doesn't block other clients (try_send drops)
  ✅ Disconnected client cleaned up from subscription map
  ✅ 100K concurrent connections (load test with k6 WebSocket)
  ✅ Message delivery latency < 50ms (Redpanda → client)
  ✅ Ping/pong heartbeat keeps connection alive