//! Domain events for Wallet Core

use chrono::{DateTime, Utc};
use rust_decimal::Decimal;
use serde::{Deserialize, Serialize};
use uuid::Uuid;

use super::{WalletType, TransactionType, TransactionStatus};

/// Wallet domain events
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum WalletEvent {
    WalletCreated(WalletCreatedEvent),
    BalanceUpdated(BalanceUpdatedEvent),
    FundLocked(FundLockedEvent),
    FundUnlocked(FundUnlockedEvent),
}

/// Transaction domain events
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum TransactionEvent {
    TransactionCreated(TransactionCreatedEvent),
    TransactionCompleted(TransactionCompletedEvent),
    TransactionFailed(TransactionFailedEvent),
}

/// Wallet created event
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WalletCreatedEvent {
    pub event_id: Uuid,
    pub timestamp: DateTime<Utc>,
    pub wallet_id: Uuid,
    pub user_id: Uuid,
    pub wallet_type: WalletType,
    pub currency: String,
}

/// Balance updated event
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BalanceUpdatedEvent {
    pub event_id: Uuid,
    pub timestamp: DateTime<Utc>,
    pub wallet_id: Uuid,
    pub user_id: Uuid,
    pub wallet_type: WalletType,
    pub previous_balance: Decimal,
    pub new_balance: Decimal,
    pub change: Decimal,
    pub transaction_id: Uuid,
}

/// Fund locked event
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FundLockedEvent {
    pub event_id: Uuid,
    pub timestamp: DateTime<Utc>,
    pub wallet_id: Uuid,
    pub user_id: Uuid,
    pub amount: Decimal,
    pub reference_id: Uuid,
    pub reference_type: String,
}

/// Fund unlocked event
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FundUnlockedEvent {
    pub event_id: Uuid,
    pub timestamp: DateTime<Utc>,
    pub wallet_id: Uuid,
    pub user_id: Uuid,
    pub amount: Decimal,
    pub reference_id: Uuid,
    pub reference_type: String,
}

/// Transaction created event
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TransactionCreatedEvent {
    pub event_id: Uuid,
    pub timestamp: DateTime<Utc>,
    pub transaction_id: Uuid,
    pub user_id: Uuid,
    pub wallet_id: Uuid,
    pub wallet_type: WalletType,
    pub transaction_type: TransactionType,
    pub amount: Decimal,
    pub currency: String,
    pub status: TransactionStatus,
    pub reference_id: Option<Uuid>,
    pub reference_type: Option<String>,
    pub idempotency_key: Option<String>,
}

/// Transaction completed event
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TransactionCompletedEvent {
    pub event_id: Uuid,
    pub timestamp: DateTime<Utc>,
    pub transaction_id: Uuid,
    pub user_id: Uuid,
    pub wallet_id: Uuid,
    pub wallet_type: WalletType,
    pub transaction_type: TransactionType,
    pub amount: Decimal,
    pub currency: String,
    pub completed_at: DateTime<Utc>,
}

/// Transaction failed event
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TransactionFailedEvent {
    pub event_id: Uuid,
    pub timestamp: DateTime<Utc>,
    pub transaction_id: Uuid,
    pub user_id: Uuid,
    pub wallet_id: Uuid,
    pub wallet_type: WalletType,
    pub transaction_type: TransactionType,
    pub amount: Decimal,
    pub currency: String,
    pub error_code: String,
    pub error_message: String,
    pub failed_at: DateTime<Utc>,
}

impl WalletCreatedEvent {
    pub fn new(wallet_id: Uuid, user_id: Uuid, wallet_type: WalletType, currency: String) -> Self {
        Self {
            event_id: Uuid::new_v4(),
            timestamp: Utc::now(),
            wallet_id,
            user_id,
            wallet_type,
            currency,
        }
    }
}

impl BalanceUpdatedEvent {
    pub fn new(
        wallet_id: Uuid,
        user_id: Uuid,
        wallet_type: WalletType,
        previous_balance: Decimal,
        new_balance: Decimal,
        transaction_id: Uuid,
    ) -> Self {
        Self {
            event_id: Uuid::new_v4(),
            timestamp: Utc::now(),
            wallet_id,
            user_id,
            wallet_type,
            previous_balance,
            new_balance,
            change: new_balance - previous_balance,
            transaction_id,
        }
    }
}

impl FundLockedEvent {
    pub fn new(
        wallet_id: Uuid,
        user_id: Uuid,
        amount: Decimal,
        reference_id: Uuid,
        reference_type: String,
    ) -> Self {
        Self {
            event_id: Uuid::new_v4(),
            timestamp: Utc::now(),
            wallet_id,
            user_id,
            amount,
            reference_id,
            reference_type,
        }
    }
}

impl FundUnlockedEvent {
    pub fn new(
        wallet_id: Uuid,
        user_id: Uuid,
        amount: Decimal,
        reference_id: Uuid,
        reference_type: String,
    ) -> Self {
        Self {
            event_id: Uuid::new_v4(),
            timestamp: Utc::now(),
            wallet_id,
            user_id,
            amount,
            reference_id,
            reference_type,
        }
    }
}

impl TransactionCreatedEvent {
    pub fn new(
        transaction_id: Uuid,
        user_id: Uuid,
        wallet_id: Uuid,
        wallet_type: WalletType,
        transaction_type: TransactionType,
        amount: Decimal,
        currency: String,
        status: TransactionStatus,
        reference_id: Option<Uuid>,
        reference_type: Option<String>,
        idempotency_key: Option<String>,
    ) -> Self {
        Self {
            event_id: Uuid::new_v4(),
            timestamp: Utc::now(),
            transaction_id,
            user_id,
            wallet_id,
            wallet_type,
            transaction_type,
            amount,
            currency,
            status,
            reference_id,
            reference_type,
            idempotency_key,
        }
    }
}

impl TransactionCompletedEvent {
    pub fn new(
        transaction_id: Uuid,
        user_id: Uuid,
        wallet_id: Uuid,
        wallet_type: WalletType,
        transaction_type: TransactionType,
        amount: Decimal,
        currency: String,
    ) -> Self {
        Self {
            event_id: Uuid::new_v4(),
            timestamp: Utc::now(),
            transaction_id,
            user_id,
            wallet_id,
            wallet_type,
            transaction_type,
            amount,
            currency,
            completed_at: Utc::now(),
        }
    }
}

impl TransactionFailedEvent {
    pub fn new(
        transaction_id: Uuid,
        user_id: Uuid,
        wallet_id: Uuid,
        wallet_type: WalletType,
        transaction_type: TransactionType,
        amount: Decimal,
        currency: String,
        error_code: String,
        error_message: String,
    ) -> Self {
        Self {
            event_id: Uuid::new_v4(),
            timestamp: Utc::now(),
            transaction_id,
            user_id,
            wallet_id,
            wallet_type,
            transaction_type,
            amount,
            currency,
            error_code,
            error_message,
            failed_at: Utc::now(),
        }
    }
}
