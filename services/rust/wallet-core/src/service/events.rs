//! Event publisher for Redpanda/Kafka

use std::sync::Arc;
use tracing::{error, info};

use crate::domain::{WalletEvent, TransactionEvent};

/// Event publisher trait
#[async_trait::async_trait]
pub trait EventPublisherTrait: Send + Sync {
    async fn publish_wallet_event(&self, event: WalletEvent) -> anyhow::Result<()>;
    async fn publish_transaction_event(&self, event: TransactionEvent) -> anyhow::Result<()>;
}

/// Event publisher implementation
pub struct EventPublisher {
    // In production, this would be a Redpanda/Kafka producer
    // For now, it's a placeholder that logs events
    enabled: bool,
}

impl EventPublisher {
    pub fn new(enabled: bool) -> Self {
        Self { enabled }
    }
    
    /// Topic names
    pub const WALLET_EVENTS_TOPIC: &'static str = "wallet.events";
    pub const TRANSACTION_EVENTS_TOPIC: &'static str = "transaction.events";
}

#[async_trait::async_trait]
impl EventPublisherTrait for EventPublisher {
    async fn publish_wallet_event(&self, event: WalletEvent) -> anyhow::Result<()> {
        if !self.enabled {
            return Ok(());
        }
        
        // Serialize event
        let event_json = serde_json::to_string(&event)?;
        
        // In production, publish to Redpanda:
        // self.producer.send(Self::WALLET_EVENTS_TOPIC, event_json).await?;
        
        info!(
            topic = Self::WALLET_EVENTS_TOPIC,
            event_type = match &event {
                WalletEvent::WalletCreated(_) => "wallet_created",
                WalletEvent::BalanceUpdated(_) => "balance_updated",
                WalletEvent::FundLocked(_) => "fund_locked",
                WalletEvent::FundUnlocked(_) => "fund_unlocked",
            },
            "Published wallet event"
        );
        
        Ok(())
    }
    
    async fn publish_transaction_event(&self, event: TransactionEvent) -> anyhow::Result<()> {
        if !self.enabled {
            return Ok(());
        }
        
        // Serialize event
        let event_json = serde_json::to_string(&event)?;
        
        // In production, publish to Redpanda:
        // self.producer.send(Self::TRANSACTION_EVENTS_TOPIC, event_json).await?;
        
        info!(
            topic = Self::TRANSACTION_EVENTS_TOPIC,
            event_type = match &event {
                TransactionEvent::TransactionCreated(_) => "transaction_created",
                TransactionEvent::TransactionCompleted(_) => "transaction_completed",
                TransactionEvent::TransactionFailed(_) => "transaction_failed",
            },
            "Published transaction event"
        );
        
        Ok(())
    }
}

/// No-op event publisher for testing
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
