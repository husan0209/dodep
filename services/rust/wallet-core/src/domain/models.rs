//! Domain models for Wallet Core
//! 
//! Standards: wallet-financial-ops.skill.md
//! - Every financial operation creates debit+credit ledger entries
//! - Idempotency checked FIRST
//! - Optimistic locking with version

use chrono::{DateTime, Utc};
use rust_decimal::Decimal;
use serde::{Deserialize, Serialize};
use sqlx::Type;
use uuid::Uuid;

use super::{WalletType, TransactionType, TransactionStatus, LedgerEntryType, AccountType};

/// Wallet aggregate root
#[derive(Debug, Clone, Serialize, Deserialize, Type)]
pub struct Wallet {
    pub id: Uuid,
    pub user_id: Uuid,
    pub wallet_type: WalletType,
    pub currency: String,
    pub balance_available: Decimal,
    pub balance_locked: Decimal,
    pub balance_bonus: Decimal,
    pub version: i32,
    pub is_active: bool,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

impl Wallet {
    /// Create a new wallet
    pub fn new(user_id: Uuid, wallet_type: WalletType, currency: String) -> Self {
        let now = Utc::now();
        Self {
            id: Uuid::new_v4(),
            user_id,
            wallet_type,
            currency,
            balance_available: Decimal::ZERO,
            balance_locked: Decimal::ZERO,
            balance_bonus: Decimal::ZERO,
            version: 0,
            is_active: true,
            created_at: now,
            updated_at: now,
        }
    }
    
    /// Get total balance (available + locked + bonus)
    pub fn total_balance(&self) -> Decimal {
        self.balance_available + self.balance_locked + self.balance_bonus
    }
    
    /// Check if wallet has sufficient available balance
    pub fn has_available_balance(&self, amount: Decimal) -> bool {
        self.balance_available >= amount
    }
    
    /// Check if wallet has sufficient balance (available + bonus)
    pub fn has_sufficient_balance(&self, amount: Decimal) -> bool {
        self.balance_available + self.balance_bonus >= amount
    }
    
    /// Get account ID for ledger entries
    pub fn ledger_account_id(&self) -> String {
        format!("user_wallet:{}:{:?}:{}", self.user_id, self.wallet_type, self.currency)
    }
}

/// Balance snapshot
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Balance {
    pub available: Decimal,
    pub locked: Decimal,
    pub bonus: Decimal,
}

impl Balance {
    pub fn new(available: Decimal, locked: Decimal, bonus: Decimal) -> Self {
        Self { available, locked, bonus }
    }
    
    pub fn total(&self) -> Decimal {
        self.available + self.locked + self.bonus
    }
}

/// Transaction entity
#[derive(Debug, Clone, Serialize, Deserialize, Type)]
pub struct Transaction {
    pub id: Uuid,
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
    pub description: Option<String>,
    pub metadata: Option<serde_json::Value>,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
    pub completed_at: Option<DateTime<Utc>>,
}

impl Transaction {
    /// Create a new pending transaction
    pub fn new(
        user_id: Uuid,
        wallet_id: Uuid,
        wallet_type: WalletType,
        transaction_type: TransactionType,
        amount: Decimal,
        currency: String,
        reference_id: Option<Uuid>,
        reference_type: Option<String>,
        idempotency_key: Option<String>,
    ) -> Self {
        let now = Utc::now();
        Self {
            id: Uuid::new_v4(),
            user_id,
            wallet_id,
            wallet_type,
            transaction_type,
            amount,
            currency,
            status: TransactionStatus::Pending,
            reference_id,
            reference_type,
            idempotency_key,
            description: None,
            metadata: None,
            created_at: now,
            updated_at: now,
            completed_at: None,
        }
    }
    
    /// Mark transaction as completed
    pub fn complete(&mut self) {
        self.status = TransactionStatus::Completed;
        self.completed_at = Some(Utc::now());
        self.updated_at = Utc::now();
    }
    
    /// Mark transaction as failed
    pub fn fail(&mut self) {
        self.status = TransactionStatus::Failed;
        self.updated_at = Utc::now();
    }
    
    /// Mark transaction as cancelled
    pub fn cancel(&mut self) {
        self.status = TransactionStatus::Cancelled;
        self.updated_at = Utc::now();
    }
}

/// Fund lock for pending operations
#[derive(Debug, Clone, Serialize, Deserialize, Type)]
pub struct FundLock {
    pub id: Uuid,
    pub wallet_id: Uuid,
    pub user_id: Uuid,
    pub amount: Decimal,
    pub reference_id: Uuid,
    pub reference_type: String,
    pub is_active: bool,
    pub created_at: DateTime<Utc>,
    pub released_at: Option<DateTime<Utc>>,
}

impl FundLock {
    pub fn new(
        wallet_id: Uuid,
        user_id: Uuid,
        amount: Decimal,
        reference_id: Uuid,
        reference_type: String,
    ) -> Self {
        Self {
            id: Uuid::new_v4(),
            wallet_id,
            user_id,
            amount,
            reference_id,
            reference_type,
            is_active: true,
            created_at: Utc::now(),
            released_at: None,
        }
    }
    
    pub fn release(&mut self) {
        self.is_active = false;
        self.released_at = Some(Utc::now());
    }
}

/// Ledger entry for double-entry bookkeeping
/// 
/// CRITICAL: Every financial operation creates TWO entries:
/// - One DEBIT (money leaves account)
/// - One CREDIT (money enters account)
/// SUM(all debits) = SUM(all credits) — ALWAYS
#[derive(Debug, Clone, Serialize, Deserialize, Type)]
pub struct LedgerEntry {
    pub id: Uuid,
    pub transaction_id: Uuid,
    pub account_type: AccountType,
    pub account_id: String,  // e.g., "user_wallet:123:main:USD"
    pub entry_type: LedgerEntryType,
    pub amount: Decimal,
    pub currency: String,
    pub balance_after: Option<Decimal>,  // Snapshot of account balance
    pub reference_type: Option<String>,
    pub reference_id: Option<Uuid>,
    pub idempotency_key: Option<String>,
    pub created_at: DateTime<Utc>,
}

impl LedgerEntry {
    /// Create a debit entry (money leaves account)
    pub fn debit(
        transaction_id: Uuid,
        account_type: AccountType,
        account_id: String,
        amount: Decimal,
        currency: String,
        balance_after: Option<Decimal>,
    ) -> Self {
        Self {
            id: Uuid::new_v4(),
            transaction_id,
            account_type,
            account_id,
            entry_type: LedgerEntryType::Debit,
            amount,
            currency,
            balance_after,
            reference_type: None,
            reference_id: None,
            idempotency_key: None,
            created_at: Utc::now(),
        }
    }
    
    /// Create a credit entry (money enters account)
    pub fn credit(
        transaction_id: Uuid,
        account_type: AccountType,
        account_id: String,
        amount: Decimal,
        currency: String,
        balance_after: Option<Decimal>,
    ) -> Self {
        Self {
            id: Uuid::new_v4(),
            transaction_id,
            account_type,
            account_id,
            entry_type: LedgerEntryType::Credit,
            amount,
            currency,
            balance_after,
            reference_type: None,
            reference_id: None,
            idempotency_key: None,
            created_at: Utc::now(),
        }
    }
    
    /// Create a debit entry with reference
    pub fn debit_with_ref(
        transaction_id: Uuid,
        account_type: AccountType,
        account_id: String,
        amount: Decimal,
        currency: String,
        balance_after: Option<Decimal>,
        reference_type: String,
        reference_id: Uuid,
        idempotency_key: Option<String>,
    ) -> Self {
        Self {
            id: Uuid::new_v4(),
            transaction_id,
            account_type,
            account_id,
            entry_type: LedgerEntryType::Debit,
            amount,
            currency,
            balance_after,
            reference_type: Some(reference_type),
            reference_id: Some(reference_id),
            idempotency_key,
            created_at: Utc::now(),
        }
    }
    
    /// Create a credit entry with reference
    pub fn credit_with_ref(
        transaction_id: Uuid,
        account_type: AccountType,
        account_id: String,
        amount: Decimal,
        currency: String,
        balance_after: Option<Decimal>,
        reference_type: String,
        reference_id: Uuid,
        idempotency_key: Option<String>,
    ) -> Self {
        Self {
            id: Uuid::new_v4(),
            transaction_id,
            account_type,
            account_id,
            entry_type: LedgerEntryType::Credit,
            amount,
            currency,
            balance_after,
            reference_type: Some(reference_type),
            reference_id: Some(reference_id),
            idempotency_key,
            created_at: Utc::now(),
        }
    }
}

/// Ledger entry pair for double-entry bookkeeping
#[derive(Debug, Clone)]
pub struct LedgerPair {
    pub debit: LedgerEntry,
    pub credit: LedgerEntry,
}

impl LedgerPair {
    pub fn new(debit: LedgerEntry, credit: LedgerEntry) -> Self {
        // Validate: amounts must be equal
        assert_eq!(debit.amount, credit.amount, "Debit and credit amounts must be equal");
        assert_eq!(debit.currency, credit.currency, "Debit and credit currencies must be equal");
        Self { debit, credit }
    }
}

/// Outbox event for reliable event publishing
#[derive(Debug, Clone, Serialize, Deserialize, Type)]
pub struct OutboxEvent {
    pub id: i64,
    pub topic: String,
    pub event_key: String,
    pub payload: Vec<u8>,
    pub headers: serde_json::Value,
    pub created_at: DateTime<Utc>,
    pub sent_at: Option<DateTime<Utc>>,
    pub retries: i32,
    pub last_error: Option<String>,
}

impl OutboxEvent {
    pub fn new(topic: String, event_key: String, payload: Vec<u8>) -> Self {
        Self {
            id: 0,
            topic,
            event_key,
            payload,
            headers: serde_json::json!({}),
            created_at: Utc::now(),
            sent_at: None,
            retries: 0,
            last_error: None,
        }
    }
}

/// Reconciliation result
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ReconciliationResult {
    pub wallet_id: Uuid,
    pub user_id: Uuid,
    pub wallet_type: WalletType,
    pub currency: String,
    pub actual_balance: Decimal,
    pub expected_balance: Decimal,
    pub discrepancy: Decimal,
}

impl ReconciliationResult {
    /// Check if discrepancy is significant (> $0.01)
    pub fn is_significant(&self) -> bool {
        self.discrepancy > Decimal::new(1, 2)  // 0.01
    }
}
