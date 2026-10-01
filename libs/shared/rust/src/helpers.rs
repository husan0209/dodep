//! Helper functions for Opus Casino platform

use crate::types::Money;
use rust_decimal::Decimal;
use std::time::Duration;
use uuid::Uuid;

/// Format money for display
///
/// `locale` is accepted for API compatibility but unused: this is a simple
/// symbol-prefixed formatter, not a localization engine. Unknown currencies
/// are rendered as `CODE <amount>` (space-separated) so they cannot be
/// confused with a known symbol.
pub fn format_money(money: &Money, _locale: &str) -> String {
    match money.currency.as_str() {
        "USD" => format!("${}", money.amount),
        "EUR" => format!("\u{20ac}{}", money.amount),
        "GBP" => format!("\u{a3}{}", money.amount),
        "RUB" => format!("\u{20bd}{}", money.amount),
        "JPY" => format!("\u{a5}{}", money.amount),
        other => format!("{} {}", other, money.amount),
    }
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
///
/// Requires the `async` feature (pulls in tokio). Without it the symbol does
/// not exist, so the crate keeps building with `default = []`.
#[cfg(any(feature = "async", feature = "full"))]
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

/// Round decimal to specified precision (half away from zero)
///
/// `Decimal::round_dp` uses banker's rounding (half to even), so `2.345` would
/// become `2.34`. The Go (`shopspring/decimal.Round`) and TypeScript
/// implementations of this platform round half away from zero, giving `2.35`.
/// Money rounding must not differ between services, so the strategy is set
/// explicitly here instead of relying on the crate default.
pub fn round_decimal(value: Decimal, precision: u32) -> Decimal {
    value.round_dp_with_strategy(precision, rust_decimal::RoundingStrategy::MidpointAwayFromZero)
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde::Serialize;

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

    #[test]
    fn test_compare_money_rejects_currency_mismatch() {
        let usd = Money::new("10.00", "USD").unwrap();
        let eur = Money::new("10.00", "EUR").unwrap();
        assert!(compare_money(&usd, &eur).is_err());
    }

    #[test]
    fn test_add_subtract_reject_currency_mismatch() {
        let usd = Money::new("10.00", "USD").unwrap();
        let eur = Money::new("10.00", "EUR").unwrap();
        assert!(add_money(&usd, &eur).is_err());
        assert!(subtract_money(&usd, &eur).is_err());
    }

    #[test]
    fn test_subtract_money() {
        let a = Money::new("100.00", "USD").unwrap();
        let b = Money::new("25.50", "USD").unwrap();
        let diff = subtract_money(&a, &b).unwrap();
        assert_eq!(diff.amount, Decimal::new(7450, 2));
    }

    #[test]
    fn test_multiply_money_is_absolute() {
        let a = Money::new("100.00", "USD").unwrap();
        let half = multiply_money(&a, Decimal::new(5, 1));
        assert_eq!(half.amount, Decimal::new(5000, 2));
        // Negative scalar must not produce a negative (i.e. creditable) amount.
        let neg = multiply_money(&a, Decimal::new(-5, 1));
        assert_eq!(neg.amount, Decimal::new(5000, 2));
    }

    #[test]
    fn test_parse_money_rejects_negative() {
        assert!(parse_money("-1.00", "USD").is_err());
        assert!(parse_money("abc", "USD").is_err());
    }

    #[test]
    fn test_format_money_unknown_currency_keeps_separator() {
        let kzt = Money::new("7.00", "KZT").unwrap();
        assert_eq!(format_money(&kzt, "en-US"), "KZT 7.00");
    }

    #[test]
    fn test_money_currency_is_uppercased() {
        let m = Money::new("1.00", "usd").unwrap();
        assert_eq!(m.currency, "USD");
    }

    #[test]
    fn test_money_predicates() {
        let zero = Money::new("0", "USD").unwrap();
        assert!(zero.is_zero());
        assert!(!zero.is_positive());
        let pos = Money::new("0.01", "USD").unwrap();
        assert!(!pos.is_zero());
        assert!(pos.is_positive());
    }

    #[test]
    fn test_now_helpers() {
        let a = now_ms();
        let b = now_ms();
        assert!(b >= a, "clock must not go backwards");
        assert!(chrono::DateTime::parse_from_rfc3339(&now_iso()).is_ok());
    }

    #[test]
    fn test_calculate_percentage() {
        let got = calculate_percentage(Decimal::new(25, 0), Decimal::new(200, 0)).unwrap();
        assert_eq!(got, Decimal::new(125, 1)); // 12.5
        assert!(calculate_percentage(Decimal::new(1, 0), Decimal::ZERO).is_none());
    }

    #[test]
    fn test_clamp_and_round() {
        assert_eq!(
            clamp_decimal(Decimal::new(5, 0), Decimal::new(10, 0), Decimal::new(20, 0)),
            Decimal::new(10, 0)
        );
        assert_eq!(
            clamp_decimal(Decimal::new(25, 0), Decimal::new(10, 0), Decimal::new(20, 0)),
            Decimal::new(20, 0)
        );
        assert_eq!(
            clamp_decimal(Decimal::new(15, 0), Decimal::new(10, 0), Decimal::new(20, 0)),
            Decimal::new(15, 0)
        );
        assert_eq!(round_decimal(Decimal::new(2345, 3), 2), Decimal::new(235, 2));
    }

    #[test]
    fn test_deep_clone_is_independent() {
        #[derive(Debug, Clone, PartialEq, Serialize, serde::Deserialize)]
        struct Doc {
            tags: Vec<String>,
        }
        let src = Doc { tags: vec!["a".into()] };
        let mut cloned = deep_clone(&src).unwrap();
        cloned.tags[0] = "mutated".into();
        assert_eq!(src.tags[0], "a", "clone must not share memory");
    }

    #[test]
    fn test_map_helpers() {
        let mut map = std::collections::HashMap::new();
        assert!(is_empty_map(&map));
        map.insert("a", 1);
        map.insert("b", 2);
        map.insert("c", 3);
        assert!(!is_empty_map(&map));

        let picked = pick_from_map(&map, &["a", "c"]);
        assert_eq!(picked.len(), 2);
        assert_eq!(picked.get("a"), Some(&1));
        assert_eq!(picked.get("c"), Some(&3));
        assert!(!picked.contains_key("b"));
    }

    #[test]
    fn test_debouncer_allows_then_denies() {
        let d = Debouncer::new(Duration::from_millis(50));
        assert!(d.should_allow());
        assert!(!d.should_allow());
        std::thread::sleep(Duration::from_millis(60));
        assert!(d.should_allow());
    }

    #[test]
    fn test_throttler_allows_then_denies() {
        let t = Throttler::new(Duration::from_millis(50));
        assert!(t.should_allow());
        assert!(!t.should_allow());
        std::thread::sleep(Duration::from_millis(60));
        assert!(t.should_allow());
    }

    #[cfg(any(feature = "async", feature = "full"))]
    #[tokio::test]
    async fn test_retry_succeeds_after_transient_failures() {
        use std::sync::atomic::{AtomicU32, Ordering};
        let calls = AtomicU32::new(0);
        let result: Result<&str, &str> = retry(
            || {
                let n = calls.fetch_add(1, Ordering::SeqCst) + 1;
                if n < 3 {
                    Err("flaky")
                } else {
                    Ok("ok")
                }
            },
            5,
            1,
            5,
            2.0,
        )
        .await;
        assert_eq!(result, Ok("ok"));
        assert_eq!(calls.load(Ordering::SeqCst), 3);
    }

    #[cfg(any(feature = "async", feature = "full"))]
    #[tokio::test]
    async fn test_retry_returns_last_error() {
        use std::sync::atomic::{AtomicU32, Ordering};
        let calls = AtomicU32::new(0);
        let result: Result<u32, &str> = retry(
            || {
                calls.fetch_add(1, Ordering::SeqCst);
                Err("always")
            },
            2,
            1,
            5,
            2.0,
        )
        .await;
        assert_eq!(result, Err("always"));
        assert_eq!(calls.load(Ordering::SeqCst), 3, "1 call + 2 retries");
    }

    #[cfg(any(feature = "async", feature = "full"))]
    #[tokio::test]
    async fn test_retry_respects_max_delay() {
        use std::time::Instant;
        let start = Instant::now();
        let result: Result<u32, &str> = retry(|| Err("always"), 5, 10, 15, 10.0).await;
        assert!(result.is_err());
        // Delays would be 10, 15(clamped), 15, 15, 15 => well under a second.
        assert!(start.elapsed() < Duration::from_secs(1));
    }
}
