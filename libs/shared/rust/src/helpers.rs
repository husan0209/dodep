//! Helper functions for Opus Casino platform

use crate::types::Money;
use rust_decimal::Decimal;
use std::time::Duration;
use uuid::Uuid;

/// Format money for display
pub fn format_money(money: &Money, locale: &str) -> String {
    // Simple formatting - in production, use a proper localization library
    let symbol = match money.currency.as_str() {
        "USD" => "$",
        "EUR" => "€",
        "GBP" => "£",
        "RUB" => "₽",
        "JPY" => "¥",
        _ => &money.currency,
    };
    
    format!("{}{}", symbol, money.amount)
}

/// Parse money string to Money object
pub fn parse_money(amount: &str, currency: &str) -> Result<Money, String> {
    Money::new(amount, currency)
}

/// Add two money amounts (must be same currency)
pub fn add_money(a: &Money, b: &Money) -> Result<Money, String> {
    a.add(b)
}

/// Subtract two money amounts (must be same currency)
pub fn subtract_money(a: &Money, b: &Money) -> Result<Money, String> {
    a.subtract(b)
}

/// Multiply money by a scalar
pub fn multiply_money(money: &Money, scalar: Decimal) -> Money {
    money.multiply(scalar)
}

/// Compare two money amounts
/// Returns: -1 if a < b, 0 if a == b, 1 if a > b
pub fn compare_money(a: &Money, b: &Money) -> Result<i8, String> {
    if a.currency != b.currency {
        return Err(format!(
            "Currency mismatch: {} != {}",
            a.currency, b.currency
        ));
    }
    
    Ok(if a.amount < b.amount {
        -1
    } else if a.amount > b.amount {
        1
    } else {
        0
    })
}

/// Generate UUID v4
pub fn generate_uuid() -> Uuid {
    Uuid::new_v4()
}

/// Get current timestamp in milliseconds
pub fn now_ms() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_millis() as u64
}

/// Get current ISO 8601 timestamp
pub fn now_iso() -> String {
    chrono::Utc::now().to_rfc3339()
}

/// Retry a function with exponential backoff
pub async fn retry<T, F, E>(
    mut operation: F,
    max_retries: u32,
    initial_delay_ms: u64,
    max_delay_ms: u64,
    multiplier: f64,
) -> Result<T, E>
where
    F: FnMut() -> Result<T, E>,
{
    let mut delay = initial_delay_ms;
    let mut last_error: Option<E> = None;
    
    for attempt in 0..=max_retries {
        match operation() {
            Ok(result) => return Ok(result),
            Err(error) => {
                last_error = Some(error);
                
                if attempt == max_retries {
                    break;
                }
                
                tokio::time::sleep(Duration::from_millis(delay)).await;
                delay = (delay as f64 * multiplier).min(max_delay_ms as f64) as u64;
            }
        }
    }
    
    Err(last_error.unwrap())
}

/// Debounce helper - delays execution until no new calls for specified duration
/// Useful for rate limiting
pub struct Debouncer {
    last_call: std::sync::Mutex<Option<std::time::Instant>>,
    delay: Duration,
}

impl Debouncer {
    pub fn new(delay: Duration) -> Self {
        Self {
            last_call: std::sync::Mutex::new(None),
            delay,
        }
    }
    
    pub fn should_allow(&self) -> bool {
        let now = std::time::Instant::now();
        let mut last_call = self.last_call.lock().unwrap();
        
        if let Some(last) = *last_call {
            if now.duration_since(last) < self.delay {
                return false;
            }
        }
        
        *last_call = Some(now);
        true
    }
}

/// Throttle helper - limits execution to once per specified duration
pub struct Throttler {
    last_execution: std::sync::Mutex<Option<std::time::Instant>>,
    limit: Duration,
}

impl Throttler {
    pub fn new(limit: Duration) -> Self {
        Self {
            last_execution: std::sync::Mutex::new(None),
            limit,
        }
    }
    
    pub fn should_allow(&self) -> bool {
        let now = std::time::Instant::now();
        let mut last_execution = self.last_execution.lock().unwrap();
        
        if let Some(last) = *last_execution {
            if now.duration_since(last) < self.limit {
                return false;
            }
        }
        
        *last_execution = Some(now);
        true
    }
}

/// Deep clone a serde-serializable object
pub fn deep_clone<T: serde::Serialize + for<'de> serde::Deserialize<'de>>(
    obj: &T,
) -> Result<T, serde_json::Error> {
    let json = serde_json::to_value(obj)?;
    serde_json::from_value(json)
}

/// Check if a HashMap is empty
pub fn is_empty_map<K, V>(map: &std::collections::HashMap<K, V>) -> bool {
    map.is_empty()
}

/// Pick specific keys from a HashMap
pub fn pick_from_map<K: Eq + std::hash::Hash + Clone, V: Clone>(
    map: &std::collections::HashMap<K, V>,
    keys: &[K],
) -> std::collections::HashMap<K, V> {
    map.iter()
        .filter(|(k, _)| keys.contains(k))
        .map(|(k, v)| (k.clone(), v.clone()))
        .collect()
}

/// Calculate percentage
pub fn calculate_percentage(part: Decimal, total: Decimal) -> Option<Decimal> {
    if total.is_zero() {
        return None;
    }
    Some((part / total) * Decimal::new(100, 0))
}

/// Clamp a decimal value between min and max
pub fn clamp_decimal(value: Decimal, min: Decimal, max: Decimal) -> Decimal {
    value.max(min).min(max)
}

/// Round decimal to specified precision
pub fn round_decimal(value: Decimal, precision: u32) -> Decimal {
    value.round_dp(precision)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_format_money() {
        let money = Money::new("100.50", "USD").unwrap();
        assert_eq!(format_money(&money, "en-US"), "$100.50");
    }

    #[test]
    fn test_generate_uuid() {
        let uuid1 = generate_uuid();
        let uuid2 = generate_uuid();
        assert_ne!(uuid1, uuid2);
    }

    #[test]
    fn test_add_money() {
        let a = Money::new("100.00", "USD").unwrap();
        let b = Money::new("50.00", "USD").unwrap();
        let sum = add_money(&a, &b).unwrap();
        assert_eq!(sum.amount, Decimal::new(15000, 2));
    }

    #[test]
    fn test_compare_money() {
        let a = Money::new("100.00", "USD").unwrap();
        let b = Money::new("50.00", "USD").unwrap();
        let c = Money::new("100.00", "USD").unwrap();
        
        assert_eq!(compare_money(&a, &b).unwrap(), 1);
        assert_eq!(compare_money(&b, &a).unwrap(), -1);
        assert_eq!(compare_money(&a, &c).unwrap(), 0);
    }
}
