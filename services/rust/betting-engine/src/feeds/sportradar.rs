/// Sportradar Live Odds Feed — WebSocket client
///
/// Connects to Sportradar's live odds WebSocket, receives odds updates,
/// and caches them in DragonflyDB with configurable TTL.
///
/// Key format in cache:
///   `odds:live:{event_id}:{market_id}:{outcome_id}` → serialized OddsUpdate (JSON)
///   `odds:live:event:{event_id}` → list of market+outcome keys (for fan-out)
use std::time::Duration;

use fred::prelude::*;
use serde::{Deserialize, Serialize};
use tokio::time::sleep;
use tokio_tungstenite::{connect_async, tungstenite::Message};
use tracing::{error, info, warn};
use futures_util::{SinkExt, StreamExt};

/// An inbound odds update from Sportradar
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OddsUpdate {
    pub event_id:   String,
    pub market_id:  String,
    pub outcome_id: String,
    pub odds:       f64,
    pub status:     String,  // "active" | "suspended" | "settled"
    pub timestamp:  i64,     // Unix millis
}

/// Configuration for the Sportradar feed client
#[derive(Debug, Clone)]
pub struct SportradarConfig {
    pub api_key:        String,
    pub ws_url:         String,
    pub live_ttl_secs:  u64,
    pub enabled:        bool,
}

/// Live odds feed client
pub struct SportradarFeedClient {
    cfg:   SportradarConfig,
    redis: RedisClient,
}

impl SportradarFeedClient {
    /// Create a new feed client with a pre-connected Redis client.
    pub fn new(cfg: SportradarConfig, redis: RedisClient) -> Self {
        Self { cfg, redis }
    }

    /// Start the feed. Reconnects automatically on disconnect.
    /// Returns only when the task is cancelled via the provided abort handle.
    pub async fn run(&self) {
        if !self.cfg.enabled {
            info!("Sportradar feed disabled (SPORTRADAR_ENABLED=false)");
            return;
        }

        let url = format!("{}?api_key={}", self.cfg.ws_url, self.cfg.api_key);
        let mut backoff = Duration::from_secs(1);

        loop {
            info!("Connecting to Sportradar live odds feed: {}", self.cfg.ws_url);
            match connect_async(&url).await {
                Ok((ws_stream, _response)) => {
                    info!("Connected to Sportradar feed");
                    backoff = Duration::from_secs(1);
                    self.handle_stream(ws_stream).await;
                    warn!("Sportradar feed disconnected, reconnecting in {:?}", backoff);
                }
                Err(e) => {
                    error!("Sportradar connection error: {}", e);
                }
            }
            sleep(backoff).await;
            backoff = (backoff * 2).min(Duration::from_secs(60));
        }
    }

    async fn handle_stream<S>(&self, mut stream: S)
    where
        S: StreamExt<Item = Result<Message, tokio_tungstenite::tungstenite::Error>> + Unpin,
    {
        while let Some(msg) = stream.next().await {
            match msg {
                Ok(Message::Text(text)) => {
                    if let Err(e) = self.process_message(&text).await {
                        warn!("Error processing Sportradar message: {}", e);
                    }
                }
                Ok(Message::Ping(_)) => {
                    // tungstenite handles pong automatically
                }
                Ok(Message::Close(_)) => {
                    info!("Sportradar WebSocket closed");
                    break;
                }
                Err(e) => {
                    error!("Sportradar WebSocket error: {}", e);
                    break;
                }
                _ => {}
            }
        }
    }

    async fn process_message(&self, text: &str) -> anyhow::Result<()> {
        // Sportradar sends an array of OddsUpdates per message
        let updates: Vec<OddsUpdate> = serde_json::from_str(text).map_err(|e| {
            anyhow::anyhow!("Sportradar parse error: {e} — raw: {}", &text[..text.len().min(200)])
        })?;

        for update in updates {
            self.cache_odds(&update).await?;
        }

        Ok(())
    }

    async fn cache_odds(&self, update: &OddsUpdate) -> anyhow::Result<()> {
        let key = format!(
            "odds:live:{}:{}:{}",
            update.event_id, update.market_id, update.outcome_id
        );
        let value = serde_json::to_string(update)?;
        let ttl = self.cfg.live_ttl_secs as i64;

        self.redis
            .set::<(), _, _>(&key, value.as_str(), Some(Expiration::EX(ttl)), None, false)
            .await?;

        Ok(())
    }
}

/// Fetch cached odds for an event from DragonflyDB.
/// Returns a Vec of all cached OddsUpdate entries whose key matches the event.
pub async fn get_event_odds(
    redis: &RedisClient,
    event_id: &str,
) -> anyhow::Result<Vec<OddsUpdate>> {
    let pattern = format!("odds:live:{}:*", event_id);
    let keys: Vec<String> = redis.scan(pattern, Some(100), None).collect().await;

    let mut odds = Vec::with_capacity(keys.len());
    for key in &keys {
        if let Ok(Some(val)) = redis.get::<Option<String>, _>(key).await {
            if let Ok(update) = serde_json::from_str::<OddsUpdate>(&val) {
                odds.push(update);
            }
        }
    }

    Ok(odds)
}
