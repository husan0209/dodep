pub mod producer;

use chrono::Utc;
use rdkafka::config::ClientConfig;
use rdkafka::producer::{FutureProducer, FutureRecord};
use rdkafka::util::Timeout;
use serde::Serialize;
use tracing::{info, warn, error};

#[derive(Clone)]
pub struct EventProducer {
    producer: FutureProducer,
}

#[derive(Debug, Serialize)]
pub struct BetPlacedEvent {
    pub event_id: String,
    pub timestamp: String,
    pub user_id: i64,
    pub bet_id: i64,
    pub bet_type: String,
    pub stake: String,
    pub odds: String,
    pub potential_win: String,
    pub currency: String,
}

#[derive(Debug, Serialize)]
pub struct BetSettledEvent {
    pub event_id: String,
    pub timestamp: String,
    pub user_id: i64,
    pub bet_id: i64,
    pub result: String,
    pub actual_win: String,
}

impl EventProducer {
    pub fn new(brokers: &str) -> Result<Self, String> {
        let producer: FutureProducer = ClientConfig::new()
            .set("bootstrap.servers", brokers)
            .set("message.timeout.ms", "5000")
            .set("acks", "all")
            .set("enable.idempotence", "true")
            .set("compression.type", "zstd")
            .set("linger.ms", "5")
            .set("batch.size", "65536")
            .set("retries", "3")
            .create()
            .map_err(|e| format!("Failed to create Kafka producer: {e}"))?;

        Ok(Self { producer })
    }

    pub async fn publish_bet_placed(
        &self,
        user_id: i64,
        bet_id: i64,
        bet_type: &str,
        stake: &str,
        odds: &str,
        potential_win: &str,
        currency: &str,
    ) {
        let event = BetPlacedEvent {
            event_id: uuid::Uuid::new_v4().to_string(),
            timestamp: Utc::now().to_rfc3339(),
            user_id,
            bet_id,
            bet_type: bet_type.to_string(),
            stake: stake.to_string(),
            odds: odds.to_string(),
            potential_win: potential_win.to_string(),
            currency: currency.to_string(),
        };

        let payload = serde_json::to_vec(&event).unwrap_or_default();
        let key = user_id.to_string();

        let record = FutureRecord::to("bets.bet.placed")
            .key(&key)
            .payload(&payload);

        match self.producer.send(record, Timeout::Never).await {
            Ok(_) => info!(
                topic = "bets.bet.placed",
                bet_id = bet_id,
                "Event published"
            ),
            Err((e, _)) => error!(
                topic = "bets.bet.placed",
                error = %e,
                "Failed to publish event"
            ),
        }
    }

    pub async fn publish_bet_settled(
        &self,
        user_id: i64,
        bet_id: i64,
        result: &str,
        actual_win: &str,
    ) {
        let event = BetSettledEvent {
            event_id: uuid::Uuid::new_v4().to_string(),
            timestamp: Utc::now().to_rfc3339(),
            user_id,
            bet_id,
            result: result.to_string(),
            actual_win: actual_win.to_string(),
        };

        let payload = serde_json::to_vec(&event).unwrap_or_default();
        let key = user_id.to_string();

        let record = FutureRecord::to("bets.bet.settled")
            .key(&key)
            .payload(&payload);

        match self.producer.send(record, Timeout::Never).await {
            Ok(_) => info!(
                topic = "bets.bet.settled",
                bet_id = bet_id,
                result = result,
                "Event published"
            ),
            Err((e, _)) => error!(
                topic = "bets.bet.settled",
                error = %e,
                "Failed to publish event"
            ),
        }
    }

    pub async fn publish_bet_voided(
        &self,
        user_id: i64,
        bet_id: i64,
    ) {
        let event = BetSettledEvent {
            event_id: uuid::Uuid::new_v4().to_string(),
            timestamp: Utc::now().to_rfc3339(),
            user_id,
            bet_id,
            result: "void".to_string(),
            actual_win: "0".to_string(),
        };

        let payload = serde_json::to_vec(&event).unwrap_or_default();
        let key = user_id.to_string();

        let record = FutureRecord::to("bets.bet.settled")
            .key(&key)
            .payload(&payload);

        match self.producer.send(record, Timeout::Never).await {
            Ok(_) => info!(topic = "bets.bet.settled", bet_id = bet_id, "Void event published"),
            Err((e, _)) => error!(topic = "bets.bet.settled", error = %e, "Failed to publish void event"),
        }
    }
}
