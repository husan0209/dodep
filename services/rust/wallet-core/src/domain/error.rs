//! Domain error types

use opus_shared::AppError;
use thiserror::Error;

/// Wallet domain errors
#[derive(Debug, Error)]
pub enum WalletError {
    #[error("Wallet not found for user {user_id}")]
    NotFound { user_id: i64 },

    #[error("Wallet already exists for user {user_id} and type {wallet_type:?}")]
    AlreadyExists {
        user_id: i64,
        wallet_type: crate::domain::WalletType,
    },

    #[error("Insufficient balance: required {required}, available {available}")]
    InsufficientBalance {
        required: rust_decimal::Decimal,
        available: rust_decimal::Decimal,
    },

    #[error("Insufficient available balance: required {required}, available {available}")]
    InsufficientAvailableBalance {
        required: rust_decimal::Decimal,
        available: rust_decimal::Decimal,
    },

    #[error("Currency mismatch: {from} != {to}")]
    CurrencyMismatch { from: String, to: String },

    #[error("Wallet is locked")]
    WalletLocked,

    #[error("Wallet is inactive")]
    WalletInactive,

    #[error("Invalid amount: {0}")]
    InvalidAmount(String),

    #[error("Invalid argument: {0}")]
    InvalidArgument(String),

    #[error("Business rule violation: {0}")]
    BusinessRuleViolation(String),

    #[error("Negative balance not allowed")]
    NegativeBalanceNotAllowed,

    #[error("Lock reference already exists: {0}")]
    LockReferenceExists(String),

    #[error("Lock reference not found: {0}")]
    LockReferenceNotFound(String),

    /// The lock exists but is no longer active (already unlocked or consumed).
    /// Terminal state, so callers must treat it as "already done".
    #[error("Lock reference already settled: {0}")]
    LockAlreadySettled(String),

    /// Lock/unlock/consume targeted a lock owned by a different user.
    #[error("Lock reference {reference} does not belong to user {user_id}")]
    LockOwnershipMismatch { reference: String, user_id: i64 },

    /// A lock was released/settled for more than the wallet holds as locked,
    /// which means the ledger and the locks diverged.
    #[error("Fund lock amount {locked} exceeds wallet locked balance {available}")]
    LockedBalanceUnderflow {
        locked: rust_decimal::Decimal,
        available: rust_decimal::Decimal,
    },

    #[error("Database error: {0}")]
    DatabaseError(String),

    #[error("Concurrency conflict")]
    ConcurrencyConflict,

    #[error("Transaction not found: {0}")]
    TransactionNotFound(String),

    #[error("Redis error: {0}")]
    Redis(#[from] redis::RedisError),

    #[error("SQL error: {0}")]
    Sqlx(#[from] sqlx::Error),
}

impl From<WalletError> for AppError {
    fn from(err: WalletError) -> Self {
        match err {
            WalletError::NotFound { .. } => AppError::NotFound(err.to_string()),
            WalletError::AlreadyExists { .. } => AppError::AlreadyExists(err.to_string()),
            WalletError::InsufficientBalance { .. } => {
                AppError::InsufficientBalance(err.to_string())
            }
            WalletError::InsufficientAvailableBalance { .. } => {
                AppError::InsufficientBalance(err.to_string())
            }
            WalletError::CurrencyMismatch { .. } => AppError::InvalidArgument(err.to_string()),
            WalletError::WalletLocked => AppError::BusinessRuleViolation(err.to_string()),
            WalletError::WalletInactive => AppError::BusinessRuleViolation(err.to_string()),
            WalletError::InvalidAmount(_) => AppError::InvalidArgument(err.to_string()),
            WalletError::InvalidArgument(_) => AppError::InvalidArgument(err.to_string()),
            WalletError::BusinessRuleViolation(_) => {
                AppError::BusinessRuleViolation(err.to_string())
            }
            WalletError::NegativeBalanceNotAllowed => {
                AppError::BusinessRuleViolation(err.to_string())
            }
            WalletError::LockReferenceExists(_) => AppError::AlreadyExists(err.to_string()),
            WalletError::LockReferenceNotFound(_) => AppError::NotFound(err.to_string()),
            WalletError::LockAlreadySettled(_) => AppError::BusinessRuleViolation(err.to_string()),
            WalletError::LockOwnershipMismatch { .. } => AppError::AuthzError(err.to_string()),
            WalletError::LockedBalanceUnderflow { .. } => {
                AppError::BusinessRuleViolation(err.to_string())
            }
            WalletError::DatabaseError(msg) => AppError::InternalError(msg),
            WalletError::ConcurrencyConflict => {
                AppError::BusinessRuleViolation("Concurrency conflict".to_string())
            }
            WalletError::TransactionNotFound(msg) => AppError::NotFound(msg),
            WalletError::Redis(e) => AppError::RedisError(e),
            WalletError::Sqlx(e) => AppError::DatabaseError(e),
        }
    }
}

/// Transaction domain errors
#[derive(Debug, Error)]
pub enum TransactionError {
    #[error("Transaction not found: {0}")]
    NotFound(String),

    #[error("Transaction already processed: {0}")]
    AlreadyProcessed(String),

    #[error("Duplicate idempotency key: {0}")]
    DuplicateIdempotencyKey(String),

    #[error("Invalid transaction state: {0}")]
    InvalidState(String),

    #[error("Reference already linked to transaction: {0}")]
    ReferenceAlreadyLinked(String),
}

impl From<TransactionError> for AppError {
    fn from(err: TransactionError) -> Self {
        match err {
            TransactionError::NotFound(_) => AppError::NotFound(err.to_string()),
            TransactionError::AlreadyProcessed(_) => {
                AppError::BusinessRuleViolation(err.to_string())
            }
            TransactionError::DuplicateIdempotencyKey(_) => {
                AppError::AlreadyExists(err.to_string())
            }
            TransactionError::InvalidState(_) => AppError::BusinessRuleViolation(err.to_string()),
            TransactionError::ReferenceAlreadyLinked(_) => {
                AppError::BusinessRuleViolation(err.to_string())
            }
        }
    }
}
