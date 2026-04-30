//! Shared types for Opus Casino platform

use chrono::{DateTime, Utc};
use rust_decimal::Decimal;
use serde::{Deserialize, Serialize};
use uuid::Uuid;

/// User identifier (UUID v4)
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(transparent)]
pub struct UserId(pub Uuid);

/// Bet identifier (UUID v4)
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(transparent)]
pub struct BetId(pub Uuid);

/// Transaction identifier (UUID v4)
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(transparent)]
pub struct TransactionId(pub Uuid);

/// Game identifier (UUID v4)
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(transparent)]
pub struct GameId(pub Uuid);

/// Session identifier (UUID v4)
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(transparent)]
pub struct SessionId(pub Uuid);

/// Monetary amount with currency
/// Amount is stored as Decimal to avoid floating point precision issues
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Money {
    pub amount: Decimal,
    pub currency: String, // ISO 4217 currency code
}

impl Money {
    /// Create a new Money instance
    pub fn new(amount: &str, currency: &str) -> Result<Self, String> {
        let amount = amount
            .parse::<Decimal>()
            .map_err(|e| format!("Invalid amount: {}", e))?;
        
        if amount < Decimal::ZERO {
            return Err("Amount cannot be negative".to_string());
        }
        
        Ok(Self {
            amount,
            currency: currency.to_uppercase(),
        })
    }

    /// Create Money from Decimal
    pub fn from_decimal(amount: Decimal, currency: &str) -> Self {
        Self {
            amount,
            currency: currency.to_uppercase(),
        }
    }

    /// Add two money amounts (must be same currency)
    pub fn add(&self, other: &Money) -> Result<Money, String> {
        if self.currency != other.currency {
            return Err(format!(
                "Currency mismatch: {} != {}",
                self.currency, other.currency
            ));
        }
        Ok(Money {
            amount: self.amount + other.amount,
            currency: self.currency.clone(),
        })
    }

    /// Subtract two money amounts (must be same currency)
    pub fn subtract(&self, other: &Money) -> Result<Money, String> {
        if self.currency != other.currency {
            return Err(format!(
                "Currency mismatch: {} != {}",
                self.currency, other.currency
            ));
        }
        Ok(Money {
            amount: self.amount - other.amount,
            currency: self.currency.clone(),
        })
    }

    /// Multiply money by a scalar
    pub fn multiply(&self, scalar: Decimal) -> Money {
        Money {
            amount: (self.amount * scalar).abs(),
            currency: self.currency.clone(),
        }
    }

    /// Check if amount is zero
    pub fn is_zero(&self) -> bool {
        self.amount == Decimal::ZERO
    }

    /// Check if amount is positive
    pub fn is_positive(&self) -> bool {
        self.amount > Decimal::ZERO
    }
}

/// Pagination parameters
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct PaginationParams {
    #[serde(default = "default_page_size")]
    pub page_size: i32,
    pub cursor: Option<String>,
    pub sort_by: Option<String>,
    #[serde(default)]
    pub descending: bool,
}

fn default_page_size() -> i32 {
    20
}

/// Pagination response
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PaginationResult<T> {
    pub items: Vec<T>,
    pub next_cursor: Option<String>,
    pub prev_cursor: Option<String>,
    pub has_more: bool,
    pub total_count: Option<i64>,
}

impl<T> PaginationResult<T> {
    pub fn new(items: Vec<T>, has_more: bool) -> Self {
        Self {
            items,
            has_more,
            next_cursor: None,
            prev_cursor: None,
            total_count: None,
        }
    }
}

/// Date range filter
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DateRange {
    pub from: Option<DateTime<Utc>>,
    pub to: Option<DateTime<Utc>>,
}

/// Error details
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ErrorDetails {
    pub error_code: String,
    pub error_message: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub metadata: Option<std::collections::HashMap<String, String>>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub field_errors: Vec<FieldError>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub trace_id: Option<String>,
}

/// Field validation error
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FieldError {
    pub field: String,
    pub error_code: String,
    pub error_message: String,
}

/// API response wrapper
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ApiResponse<T> {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub data: Option<T>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<ErrorDetails>,
}

impl<T> ApiResponse<T> {
    pub fn success(data: T) -> Self {
        Self {
            data: Some(data),
            error: None,
        }
    }

    pub fn error(error: ErrorDetails) -> ApiResponse<()> {
        ApiResponse {
            data: None,
            error: Some(error),
        }
    }
}

/// Health check status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum HealthStatus {
    Healthy,
    Degraded,
    Unhealthy,
}

/// Health check response
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HealthCheckResponse {
    pub service_name: String,
    pub status: HealthStatus,
    pub timestamp: DateTime<Utc>,
    pub version: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub components: Option<std::collections::HashMap<String, ComponentHealth>>,
}

/// Component health
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ComponentHealth {
    pub name: String,
    pub status: HealthStatus,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub message: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub latency_ms: Option<u64>,
}

/// Device type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum DeviceType {
    Web,
    MobileWeb,
    Ios,
    Android,
    Desktop,
}

/// Wallet type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum WalletType {
    Main,
    Bonus,
    FreeSpins,
    Cashback,
}

/// Transaction type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum TransactionType {
    Deposit,
    Withdrawal,
    BetPlace,
    BetWin,
    BetRefund,
    BonusCredit,
    BonusDebit,
    Transfer,
    Adjustment,
}

/// Bet type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum BetType {
    Sports,
    Live,
    Casino,
    Lottery,
    Virtual,
}

/// Bet status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum BetStatus {
    Pending,
    Accepted,
    Settled,
    Cancelled,
    Rejected,
}

/// KYC level
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum KycLevel {
    None,
    Basic,
    Identity,
    Enhanced,
    Vip,
}
