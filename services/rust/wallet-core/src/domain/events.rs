//! Domain events for Wallet Core

use chrono::{DateTime, Utc};
use rust_decimal::Decimal;
use serde::{Deserialize, Serialize};
use uuid::Uuid;

use super::{TransactionStatus, TransactionType, WalletType};

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

/// Shared payload for the transaction lifecycle events.
#[derive(Debug, Clone)]
pub struct TransactionEventSubject {
    pub transaction_id: Uuid,
    pub user_id: Uuid,
    pub wallet_id: Uuid,
    pub wallet_type: WalletType,
    pub transaction_type: TransactionType,
    pub amount: Decimal,
    pub currency: String,
}

impl TransactionCreatedEvent {
    pub fn new(subject: TransactionEventSubject, created: TransactionCreated) -> Self {
        Self {
            event_id: Uuid::new_v4(),
            timestamp: Utc::now(),
            transaction_id: subject.transaction_id,
            user_id: subject.user_id,
            wallet_id: subject.wallet_id,
            wallet_type: subject.wallet_type,
            transaction_type: subject.transaction_type,
            amount: subject.amount,
            currency: subject.currency,
            status: created.status,
            reference_id: created.reference_id,
            reference_type: created.reference_type,
            idempotency_key: created.idempotency_key,
        }
    }
}

/// Extra fields carried by a `transaction.created` event.
#[derive(Debug, Clone)]
pub struct TransactionCreated {
    pub status: TransactionStatus,
    pub reference_id: Option<Uuid>,
    pub reference_type: Option<String>,
    pub idempotency_key: Option<String>,
}

impl TransactionCompletedEvent {
    pub fn new(subject: TransactionEventSubject) -> Self {
        Self {
            event_id: Uuid::new_v4(),
            timestamp: Utc::now(),
            transaction_id: subject.transaction_id,
            user_id: subject.user_id,
            wallet_id: subject.wallet_id,
            wallet_type: subject.wallet_type,
            transaction_type: subject.transaction_type,
            amount: subject.amount,
            currency: subject.currency,
            completed_at: Utc::now(),
        }
    }
}

impl TransactionFailedEvent {
    pub fn new(subject: TransactionEventSubject, error: TransactionFailure) -> Self {
        let TransactionFailure {
            error_code,
            error_message,
        } = error;
        Self {
            event_id: Uuid::new_v4(),
            timestamp: Utc::now(),
            transaction_id: subject.transaction_id,
            user_id: subject.user_id,
            wallet_id: subject.wallet_id,
            wallet_type: subject.wallet_type,
            transaction_type: subject.transaction_type,
            amount: subject.amount,
            currency: subject.currency,
            error_code,
            error_message,
            failed_at: Utc::now(),
        }
    }
}

/// Failure detail carried by a `transaction.failed` event.
#[derive(Debug, Clone)]
pub struct TransactionFailure {
    pub error_code: String,
    pub error_message: String,
}
