//! Typed event publisher for Redpanda.
//!
//! Publishes [`WalletEvent`] / [`TransactionEvent`] through a real
//! `rdkafka::FutureProducer` with broker acknowledgement. Use
//! [`NoOpEventPublisher`] in tests or when the broker is disabled.

use rdkafka::producer::{FutureProducer, FutureRecord};
use std::time::Duration;
use tracing::info;

use crate::domain::{TransactionEvent, WalletEvent};

/// Event publisher trait
#[async_trait::async_trait]
pub trait EventPublisherTrait: Send + Sync {
    async fn publish_wallet_event(&self, event: WalletEvent) -> anyhow::Result<()>;
    async fn publish_transaction_event(&self, event: TransactionEvent) -> anyhow::Result<()>;
}

/// Real Redpanda-backed event publisher.
pub struct EventPublisher {
    producer: FutureProducer,
}

impl EventPublisher {
    pub const WALLET_EVENTS_TOPIC: &'static str = "wallet.events";
    pub const TRANSACTION_EVENTS_TOPIC: &'static str = "transaction.events";

    pub fn new(producer: FutureProducer) -> Self {
        Self { producer }
    }

    async fn send(&self, topic: &str, key: &str, payload: &[u8]) -> anyhow::Result<()> {
        self.producer
            .send(
                FutureRecord::to(topic).key(key).payload(payload),
                Duration::from_secs(5),
            )
            .await
            .map_err(|(e, _msg)| anyhow::anyhow!("Delivery to {topic} failed: {e}"))?;
        Ok(())
    }
}

fn wallet_event_key(event: &WalletEvent) -> String {
    match event {
        WalletEvent::WalletCreated(e) => e.user_id.to_string(),
        WalletEvent::BalanceUpdated(e) => e.user_id.to_string(),
        WalletEvent::FundLocked(e) => e.user_id.to_string(),
        WalletEvent::FundUnlocked(e) => e.user_id.to_string(),
    }
}

fn transaction_event_key(event: &TransactionEvent) -> String {
    match event {
        TransactionEvent::TransactionCreated(e) => e.user_id.to_string(),
        TransactionEvent::TransactionCompleted(e) => e.user_id.to_string(),
        TransactionEvent::TransactionFailed(e) => e.user_id.to_string(),
    }
}

fn wallet_event_type(event: &WalletEvent) -> &'static str {
    match event {
        WalletEvent::WalletCreated(_) => "wallet_created",
        WalletEvent::BalanceUpdated(_) => "balance_updated",
        WalletEvent::FundLocked(_) => "fund_locked",
        WalletEvent::FundUnlocked(_) => "fund_unlocked",
    }
}

fn transaction_event_type(event: &TransactionEvent) -> &'static str {
    match event {
        TransactionEvent::TransactionCreated(_) => "transaction_created",
        TransactionEvent::TransactionCompleted(_) => "transaction_completed",
        TransactionEvent::TransactionFailed(_) => "transaction_failed",
    }
}

#[async_trait::async_trait]
impl EventPublisherTrait for EventPublisher {
    async fn publish_wallet_event(&self, event: WalletEvent) -> anyhow::Result<()> {
        let payload = serde_json::to_vec(&event)?;
        self.send(
            Self::WALLET_EVENTS_TOPIC,
            &wallet_event_key(&event),
            &payload,
        )
        .await?;
        info!(
            topic = Self::WALLET_EVENTS_TOPIC,
            event_type = wallet_event_type(&event),
            "Published wallet event"
        );
        Ok(())
    }

    async fn publish_transaction_event(&self, event: TransactionEvent) -> anyhow::Result<()> {
        let payload = serde_json::to_vec(&event)?;
        self.send(
            Self::TRANSACTION_EVENTS_TOPIC,
            &transaction_event_key(&event),
            &payload,
        )
        .await?;
        info!(
            topic = Self::TRANSACTION_EVENTS_TOPIC,
            event_type = transaction_event_type(&event),
            "Published transaction event"
        );
        Ok(())
    }
}

/// No-op event publisher for testing and broker-disabled mode.
pub struct NoOpEventPublisher;

#[async_trait::async_trait]
impl EventPublisherTrait for NoOpEventPublisher {
    async fn publish_wallet_event(&self, _event: WalletEvent) -> anyhow::Result<()> {
        Ok(())
    }

    async fn publish_transaction_event(&self, _event: TransactionEvent) -> anyhow::Result<()> {
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::domain::{
        BalanceUpdatedEvent, TransactionCompletedEvent, TransactionStatus, TransactionType,
        WalletType,
    };
    use rust_decimal::Decimal;
    use uuid::Uuid;

    #[test]
    fn wallet_events_serialize_with_type_tag() {
        let event = WalletEvent::BalanceUpdated(BalanceUpdatedEvent::new(
            Uuid::new_v4(),
            42,
            WalletType::Main,
            Decimal::new(100, 2),
            Decimal::new(150, 2),
            Uuid::new_v4(),
        ));
        let v: serde_json::Value = serde_json::to_value(&event).unwrap();
        assert_eq!(v["type"], "balance_updated");
        assert_eq!(wallet_event_type(&event), "balance_updated");
    }

    #[test]
    fn transaction_events_serialize_with_type_tag() {
        let event = TransactionEvent::TransactionCompleted(TransactionCompletedEvent::new(
            Uuid::new_v4(),
            42,
            Uuid::new_v4(),
            WalletType::Main,
            TransactionType::Deposit,
            Decimal::new(50, 2),
            "USD".to_string(),
        ));
        let v: serde_json::Value = serde_json::to_value(&event).unwrap();
        assert_eq!(v["type"], "transaction_completed");
        let _ = TransactionStatus::Completed;
    }
}
