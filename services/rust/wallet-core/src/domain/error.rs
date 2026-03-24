//! Domain error types

use thiserror::Error;
use opus_shared::AppError;

/// Wallet domain errors
#[derive(Debug, Error)]
pub enum WalletError {
    #[error("Wallet not found for user {user_id:?}")]
    NotFound { user_id: uuid::Uuid },
    
    #[error("Wallet already exists for user {user_id:?} and type {wallet_type:?}")]
    AlreadyExists { 
        user_id: uuid::Uuid,
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
    
    #[error("Negative balance not allowed")]
    NegativeBalanceNotAllowed,
    
    #[error("Lock reference already exists: {0}")]
    LockReferenceExists(String),
    
    #[error("Lock reference not found: {0}")]
    LockReferenceNotFound(String),
}

impl From<WalletError> for AppError {
    fn from(err: WalletError) -> Self {
        match err {
            WalletError::NotFound { .. } => AppError::NotFound(err.to_string()),
            WalletError::AlreadyExists { .. } => AppError::AlreadyExists(err.to_string()),
            WalletError::InsufficientBalance { .. } => AppError::InsufficientBalance(err.to_string()),
            WalletError::InsufficientAvailableBalance { .. } => {
                AppError::InsufficientBalance(err.to_string())
            }
            WalletError::CurrencyMismatch { .. } => AppError::InvalidArgument(err.to_string()),
            WalletError::WalletLocked => AppError::BusinessRuleViolation(err.to_string()),
            WalletError::WalletInactive => AppError::BusinessRuleViolation(err.to_string()),
            WalletError::InvalidAmount(_) => AppError::InvalidArgument(err.to_string()),
            WalletError::NegativeBalanceNotAllowed => AppError::BusinessRuleViolation(err.to_string()),
            WalletError::LockReferenceExists(_) => AppError::AlreadyExists(err.to_string()),
            WalletError::LockReferenceNotFound(_) => AppError::NotFound(err.to_string()),
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
            TransactionError::AlreadyProcessed(_) => AppError::BusinessRuleViolation(err.to_string()),
            TransactionError::DuplicateIdempotencyKey(_) => AppError::AlreadyExists(err.to_string()),
            TransactionError::InvalidState(_) => AppError::BusinessRuleViolation(err.to_string()),
            TransactionError::ReferenceAlreadyLinked(_) => AppError::BusinessRuleViolation(err.to_string()),
        }
    }
}
