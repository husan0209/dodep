//! Transactional outbox relay: delivers `outbox` rows to Redpanda.
//!
//! WalletService writes events to the `outbox` table inside the same DB
//! transaction as the balance update (see `wallet_service.rs`). This relay
//! polls unpublished rows and publishes them with at-least-once semantics:
//! a row is marked sent only after the broker acknowledges delivery.
//! Poison rows (retries over `max_retries`) are skipped with an error log
//! so they can be inspected via `OutboxRepository::get_dead_letter`.

use std::sync::Arc;
use std::time::Duration;

use rdkafka::producer::{FutureProducer, FutureRecord};
use rdkafka::ClientConfig;
use tracing::{error, info, warn};

use crate::domain::OutboxEvent;
use crate::infrastructure::repositories::OutboxRepository;

/// Builds an idempotent Redpanda producer (`acks=all`).
pub fn create_producer(brokers: &str) -> anyhow::Result<FutureProducer> {
    ClientConfig::new()
        .set("bootstrap.servers", brokers)
        .set("message.timeout.ms", "5000")
        .set("acks", "all")
        .set("enable.idempotence", "true")
        .create()
        .map_err(|e| anyhow::anyhow!("Failed to create Kafka producer: {e}"))
}

/// Background relay polling the outbox table and publishing to Redpanda.
pub struct OutboxRelay {
    producer: FutureProducer,
    repo: Arc<OutboxRepository>,
    poll_interval: Duration,
    batch_size: i64,
    max_retries: i32,
}

impl OutboxRelay {
    pub fn new(
        producer: FutureProducer,
        repo: Arc<OutboxRepository>,
        poll_interval: Duration,
        batch_size: i64,
        max_retries: i32,
    ) -> Self {
        Self {
            producer,
            repo,
            poll_interval,
            batch_size,
            max_retries,
        }
    }

    /// Run the relay loop until the task is aborted (graceful shutdown).
    pub async fn run(&self) {
        info!(
            poll_secs = self.poll_interval.as_secs(),
            batch = self.batch_size,
            "Outbox relay started"
        );
        let mut ticker = tokio::time::interval(self.poll_interval);
        loop {
            ticker.tick().await;
            if let Err(e) = self.relay_once().await {
                error!(error = %e, "Outbox relay batch failed");
            }
        }
    }

    /// Publish one batch of unpublished events. Returns delivered count.
    pub async fn relay_once(&self) -> anyhow::Result<usize> {
        let events = self.repo.get_unsent(self.batch_size).await?;
        let mut delivered = 0usize;
        for event in events {
            if event.retries > self.max_retries {
                error!(
                    outbox_id = event.id,
                    retries = event.retries,
                    topic = %event.topic,
                    "Outbox event exceeded max retries, skipping (dead letter)"
                );
                continue;
            }
            match publish_record(&self.producer, &event).await {
                Ok(()) => {
                    self.repo.mark_sent(event.id).await?;
                    delivered += 1;
                }
                Err(e) => {
                    warn!(
                        outbox_id = event.id,
                        error = %e,
                        "Failed to publish outbox event, will retry"
                    );
                    // Best-effort retry accounting; relay continues with the rest.
                    if let Err(db_err) = self.repo.increment_retry(event.id, &e.to_string()).await {
                        error!(outbox_id = event.id, error = %db_err, "Failed to record retry");
                    }
                }
            }
        }
        if delivered > 0 {
            info!(delivered, "Outbox relay delivered events");
        }
        Ok(delivered)
    }
}

/// Publish a single outbox row and wait for broker acknowledgement.
pub async fn publish_record(producer: &FutureProducer, event: &OutboxEvent) -> anyhow::Result<()> {
    let record = FutureRecord::to(&event.topic)
        .key(&event.event_key)
        .payload(&event.payload);
    producer
        .send(record, Duration::from_secs(5))
        .await
        .map_err(|(e, _msg)| anyhow::anyhow!("Delivery failed: {e}"))?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::domain::OutboxEvent;

    #[test]
    fn outbox_event_carries_topic_key_and_payload() {
        let payload = serde_json::to_vec(&serde_json::json!({"event": "wallet_credited"})).unwrap();
        let event = OutboxEvent::new("wallet.events".to_string(), "user-42".to_string(), payload);
        assert_eq!(event.topic, "wallet.events");
        assert_eq!(event.event_key, "user-42");
        assert!(!event.payload.is_empty());
        assert_eq!(event.retries, 0);
        assert!(event.sent_at.is_none());
    }

    #[test]
    fn producer_builds_offline_without_connecting() {
        // Client creation is lazy: no broker connection is attempted.
        let producer = create_producer("localhost:9092").expect("producer builds offline");
        let _ = producer;
    }
}
