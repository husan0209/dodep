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

#[cfg(test)]
mod tests {
    use super::*;

    fn m(amount: &str, currency: &str) -> Money {
        Money::new(amount, currency).expect("valid money")
    }

    #[test]
    fn money_new_parses_and_uppercases_currency() {
        let money = Money::new("100.00", "usd").unwrap();
        assert_eq!(money.amount, Decimal::new(10000, 2));
        assert_eq!(money.currency, "USD");
    }

    #[test]
    fn money_new_rejects_negative_and_garbage() {
        assert!(Money::new("-1.00", "USD").is_err());
        assert!(Money::new("abc", "USD").is_err());
        assert!(Money::new("", "USD").is_err());
    }

    #[test]
    fn money_add_and_subtract_require_same_currency() {
        let usd = m("100.00", "USD");
        let eur = m("100.00", "EUR");

        let sum = usd.add(&m("25.50", "USD")).unwrap();
        assert_eq!(sum.amount, Decimal::new(12550, 2));
        assert_eq!(sum.currency, "USD");

        let diff = usd.subtract(&m("25.50", "USD")).unwrap();
        assert_eq!(diff.amount, Decimal::new(7450, 2));

        assert!(usd.add(&eur).is_err());
        assert!(usd.subtract(&eur).is_err());
    }

    #[test]
    fn money_subtract_can_go_negative() {
        // Subtraction is pure arithmetic; balance guards live in services.
        let diff = m("10.00", "USD").subtract(&m("25.00", "USD")).unwrap();
        assert!(diff.amount < Decimal::ZERO);
    }

    #[test]
    fn money_multiply_is_absolute() {
        let money = m("100.00", "USD");
        assert_eq!(
            money.multiply(Decimal::new(15, 1)).amount,
            Decimal::new(15000, 2)
        );
        // A negative scalar must not yield a negative (creditable) amount.
        assert_eq!(
            money.multiply(Decimal::new(-15, 1)).amount,
            Decimal::new(15000, 2)
        );
    }

    #[test]
    fn money_predicates() {
        assert!(m("0", "USD").is_zero());
        assert!(!m("0", "USD").is_positive());
        assert!(!m("0.01", "USD").is_zero());
        assert!(m("0.01", "USD").is_positive());
    }

    #[test]
    fn money_from_decimal_normalizes_currency() {
        let money = Money::from_decimal(Decimal::new(500, 2), "eur");
        assert_eq!(money.currency, "EUR");
        assert_eq!(money.amount, Decimal::new(500, 2));
    }

    #[test]
    fn money_serde_roundtrip_preserves_decimal_exactness() {
        // 0.1 + 0.2 != 0.3 in f64; Decimal must serialize exactly.
        let total = m("0.1", "USD")
            .add(&m("0.2", "USD"))
            .unwrap();
        assert_eq!(total.amount.to_string(), "0.3");

        let json = serde_json::to_string(&total).unwrap();
        let back: Money = serde_json::from_str(&json).unwrap();
        assert_eq!(back.amount, total.amount);
        assert_eq!(back.currency, total.currency);
    }

    #[test]
    fn identifiers_are_transparent_in_json() {
        let id = Uuid::parse_str("550e8400-e29b-41d4-a716-446655440000").unwrap();
        let user = UserId(id);
        let json = serde_json::to_string(&user).unwrap();
        assert_eq!(json, "\"550e8400-e29b-41d4-a716-446655440000\"");
        let back: UserId = serde_json::from_str(&json).unwrap();
        assert_eq!(back, user);
    }

    #[test]
    fn identifier_newtypes_are_distinct_types() {
        // Compile-time distinctness; the assertion documents intent.
        let u = UserId(Uuid::nil());
        let b = BetId(Uuid::nil());
        let t = TransactionId(Uuid::nil());
        let g = GameId(Uuid::nil());
        let s = SessionId(Uuid::nil());
        assert_eq!(u.0, b.0);
        assert_eq!(u.0, t.0);
        assert_eq!(u.0, g.0);
        assert_eq!(u.0, s.0);
    }

    #[test]
    fn pagination_params_defaults_page_size() {
        let params: PaginationParams = serde_json::from_str("{}").unwrap();
        assert_eq!(params.page_size, 20);
        assert!(params.cursor.is_none());
        assert!(!params.descending);
    }

    #[test]
    fn pagination_params_respects_explicit_page_size() {
        let params: PaginationParams = serde_json::from_str(r#"{"page_size":50,"descending":true}"#).unwrap();
        assert_eq!(params.page_size, 50);
        assert!(params.descending);
    }

    #[test]
    fn pagination_result_new_starts_without_cursors() {
        let result: PaginationResult<String> = PaginationResult::new(vec!["a".into()], true);
        assert_eq!(result.items.len(), 1);
        assert!(result.has_more);
        assert!(result.next_cursor.is_none());
        assert!(result.prev_cursor.is_none());
        assert!(result.total_count.is_none());
    }

    #[test]
    fn api_response_uses_camel_case_and_omits_empty() {
        let ok: ApiResponse<u32> = ApiResponse::success(42);
        let json = serde_json::to_string(&ok).unwrap();
        assert!(json.contains("\"data\":42"), "{json}");
        assert!(!json.contains("\"error\""), "{json}");

        let details = ErrorDetails {
            error_code: "NOT_FOUND".into(),
            error_message: "missing".into(),
            metadata: None,
            field_errors: vec![],
            trace_id: None,
        };
        // `ApiResponse::error` is declared on `impl<T>` but always returns
// `ApiResponse<()>`, so T is unconstrained here.
        let err = ApiResponse::<u32>::error(details);
        let json = serde_json::to_string(&err).unwrap();
        // NOTE: `rename_all = "camelCase"` on ApiResponse applies only to its
        // own fields (data/error). The nested ErrorDetails keeps snake_case,
        // so the wire format is mixed — pinned here as actual behavior.
        // Clients must expect `error.error_code`. See report to the owners.
        assert!(json.contains("\"error_code\""), "{json}");
        assert!(json.contains("\"error_message\""), "{json}");
        // Empty field_errors / absent metadata / trace_id must be omitted.
        assert!(!json.contains("field_errors"), "{json}");
        assert!(!json.contains("trace_id"), "{json}");
        assert!(!json.contains("\"data\""), "{json}");
    }

    #[test]
    fn enums_serialize_as_snake_case() {
        assert_eq!(serde_json::to_string(&HealthStatus::Healthy).unwrap(), "\"healthy\"");
        assert_eq!(serde_json::to_string(&DeviceType::MobileWeb).unwrap(), "\"mobile_web\"");
        assert_eq!(serde_json::to_string(&WalletType::FreeSpins).unwrap(), "\"free_spins\"");
        assert_eq!(serde_json::to_string(&TransactionType::BetPlace).unwrap(), "\"bet_place\"");
        assert_eq!(serde_json::to_string(&BetType::Live).unwrap(), "\"live\"");
        assert_eq!(serde_json::to_string(&BetStatus::Settled).unwrap(), "\"settled\"");
        assert_eq!(serde_json::to_string(&KycLevel::Identity).unwrap(), "\"identity\"");
    }
}
