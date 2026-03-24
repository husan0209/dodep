use std::sync::Arc;
use std::time::Duration;

use rdkafka::consumer::{Consumer, StreamConsumer};
use rdkafka::ClientConfig;
use tokio::sync::broadcast;

use crate::domain::channel::{kafka_to_topic, Topic, KAFKA_TOPICS};

/// Message from Kafka to be broadcast
#[derive(Debug, Clone)]
pub struct KafkaMessage {
    pub topic: Topic,
    pub payload: Vec<u8>,
}

/// Kafka consumer that polls Redpanda and broadcasts to SubscriptionManager
pub struct KafkaBroadcaster {
    consumer: StreamConsumer,
}

impl KafkaBroadcaster {
    pub fn new(brokers: &str) -> anyhow::Result<Self> {
        let consumer: StreamConsumer = ClientConfig::new()
            .set("bootstrap.servers", brokers)
            .set("group.id", "ws-gateway")
            .set("auto.offset.reset", "latest")
            .set("enable.auto.commit", "true")
            .set("auto.commit.interval.ms", "5000")
            .set("session.timeout.ms", "30000")
            .set("max.poll.interval.ms", "300000")
            .create()
            .map_err(|e| anyhow::anyhow!("Failed to create Kafka consumer: {e}"))?;

        consumer
            .subscribe(KAFKA_TOPICS)
            .map_err(|e| anyhow::anyhow!("Failed to subscribe to topics: {e}"))?;

        Ok(Self { consumer })
    }

    /// Start consuming messages and send them via broadcast channel
    pub async fn run(&self, sender: broadcast::Sender<KafkaMessage>) {
        tracing::info!(topics = ?KAFKA_TOPICS, "Kafka consumer started");

        loop {
            match self.consumer.recv().await {
                Ok(message) => {
                    let kafka_topic = message.topic();
                    let key = message
                        .key()
                        .and_then(|k| std::str::from_utf8(k).ok())
                        .unwrap_or_default();

                    if let Some(topic) = kafka_to_topic(kafka_topic, key) {
                        if let Some(payload) = message.payload() {
                            let msg = KafkaMessage {
                                topic,
                                payload: payload.to_vec(),
                            };
                            // Ignore send errors (no active receivers)
                            let _ = sender.send(msg);
                        }
                    }
                }
                Err(e) => {
                    tracing::error!(error = %e, "Kafka consume error");
                    tokio::time::sleep(Duration::from_millis(100)).await;
                }
            }
        }
    }
}
