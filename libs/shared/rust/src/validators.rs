//! Validators for Opus Casino platform

use chrono::Datelike;
use regex::Regex;
use std::sync::OnceLock;

/// Validate UUID v4
pub fn is_valid_uuid(uuid: &str) -> bool {
    static UUID_REGEX: OnceLock<Regex> = OnceLock::new();
    let regex = UUID_REGEX.get_or_init(|| {
        Regex::new(r"^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$").unwrap()
    });
    regex.is_match(&uuid.to_lowercase())
}

/// Validate email address
pub fn is_valid_email(email: &str) -> bool {
    static EMAIL_REGEX: OnceLock<Regex> = OnceLock::new();
    let regex = EMAIL_REGEX.get_or_init(|| {
        Regex::new(r"^[^\s@]+@[^\s@]+\.[^\s@]+$").unwrap()
    });
    regex.is_match(email)
}

/// Validate country code (ISO 3166-1 alpha-2)
pub fn is_valid_country_code(code: &str) -> bool {
    code.len() == 2 && code.chars().all(|c| c.is_ascii_uppercase())
}

/// Validate currency code (ISO 4217)
pub fn is_valid_currency_code(code: &str) -> bool {
    code.len() == 3 && code.chars().all(|c| c.is_ascii_uppercase())
}

/// Validate money amount string
pub fn is_valid_money_amount(amount: &str) -> bool {
    static AMOUNT_REGEX: OnceLock<Regex> = OnceLock::new();
    let regex = AMOUNT_REGEX.get_or_init(|| {
        Regex::new(r"^\d+(\.\d{1,2})?$").unwrap()
    });
    regex.is_match(amount)
}

/// Validate password strength
/// Requirements:
/// - At least 8 characters
/// - At least one uppercase letter
/// - At least one lowercase letter
/// - At least one number
/// - At least one special character
pub fn is_valid_password(password: &str) -> bool {
    if password.len() < 8 {
        return false;
    }
    
    let has_upper = password.chars().any(|c| c.is_ascii_uppercase());
    let has_lower = password.chars().any(|c| c.is_ascii_lowercase());
    let has_digit = password.chars().any(|c| c.is_ascii_digit());
    let has_special = password.chars().any(|c| !c.is_alphanumeric());
    
    has_upper && has_lower && has_digit && has_special
}

/// Validate phone number (E.164 format)
pub fn is_valid_phone(phone: &str) -> bool {
    static PHONE_REGEX: OnceLock<Regex> = OnceLock::new();
    let regex = PHONE_REGEX.get_or_init(|| {
        Regex::new(r"^\+[1-9]\d{1,14}$").unwrap()
    });
    regex.is_match(phone)
}

/// Validate odds format (decimal)
pub fn is_valid_odds(odds: &str) -> bool {
    static ODDS_REGEX: OnceLock<Regex> = OnceLock::new();
    let regex = ODDS_REGEX.get_or_init(|| {
        Regex::new(r"^\d+(\.\d+)?$").unwrap()
    });
    
    if !regex.is_match(odds) {
        return false;
    }
    
    if let Ok(odds_value) = odds.parse::<f64>() {
        return odds_value >= 1.01 && odds_value <= 1000.0;
    }
    
    false
}

/// Validate percentage (0-100)
pub fn is_valid_percentage(value: f64) -> bool {
    value.is_finite() && value >= 0.0 && value <= 100.0
}

/// Validate IP address (IPv4 or IPv6)
///
/// Uses `std::net::IpAddr::from_str`, which is the canonical parser and accepts
/// every valid textual form — including compressed IPv6 (`::1`, `2001:db8::1`)
/// and IPv4-mapped IPv6. A hand-rolled regex could only cover a subset: the
/// previous implementation rejected compressed IPv6, silently disagreeing with
/// the Go and TypeScript validators on the same platform.
pub fn is_valid_ip(ip: &str) -> bool {
    ip.parse::<std::net::IpAddr>().is_ok()
}

/// Validate date string (ISO 8601 format: YYYY-MM-DD)
///
/// Rejects impossible calendar dates (e.g. `2026-02-30`, `2025-02-29`): a
/// range check on the day component alone would accept them. Bounded to
/// 1900-2100 to match the other language implementations of this validator.
pub fn is_valid_date(date: &str) -> bool {
    static DATE_REGEX: OnceLock<Regex> = OnceLock::new();
    let regex = DATE_REGEX.get_or_init(|| {
        Regex::new(r"^\d{4}-\d{2}-\d{2}$").unwrap()
    });

    if !regex.is_match(date) {
        return false;
    }

    let Ok(naive) = chrono::NaiveDate::parse_from_str(date, "%Y-%m-%d") else {
        return false;
    };

    (1900..=2100).contains(&naive.year())
}

/// Validate username
/// Requirements:
/// - 3-20 characters
/// - Only alphanumeric characters and underscores
/// - Must start with a letter
pub fn is_valid_username(username: &str) -> bool {
    static USERNAME_REGEX: OnceLock<Regex> = OnceLock::new();
    let regex = USERNAME_REGEX.get_or_init(|| {
        Regex::new(r"^[a-zA-Z][a-zA-Z0-9_]{2,19}$").unwrap()
    });
    regex.is_match(username)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_valid_uuid() {
        assert!(is_valid_uuid("550e8400-e29b-41d4-a716-446655440000"));
        assert!(is_valid_uuid("550E8400-E29B-41D4-A716-446655440000"));
    }

    #[test]
    fn test_invalid_uuid() {
        assert!(!is_valid_uuid("not-a-uuid"));
        assert!(!is_valid_uuid("550e8400-e29b-41d4-a716"));
        assert!(!is_valid_uuid(""));
        // v1 uuid: version nibble must be 4
        assert!(!is_valid_uuid("550e8400-e29b-11d4-a716-446655440000"));
    }

    #[test]
    fn test_valid_email() {
        assert!(is_valid_email("user@example.com"));
        assert!(is_valid_email("test.user+tag@domain.co.uk"));
    }

    #[test]
    fn test_invalid_email() {
        assert!(!is_valid_email("invalid"));
        assert!(!is_valid_email("@example.com"));
        assert!(!is_valid_email("user@"));
        assert!(!is_valid_email(""));
        assert!(!is_valid_email("a b@c.com"));
    }

    #[test]
    fn test_country_and_currency_codes() {
        for c in ["US", "UA", "DE"] {
            assert!(is_valid_country_code(c), "{c} must be valid");
        }
        for c in ["", "U", "USA", "us", "U1"] {
            assert!(!is_valid_country_code(c), "{c} must be invalid");
        }
        for c in ["USD", "EUR", "BTC"] {
            assert!(is_valid_currency_code(c), "{c} must be valid");
        }
        for c in ["", "US", "USDD", "usd", "U1D"] {
            assert!(!is_valid_currency_code(c), "{c} must be invalid");
        }
    }

    #[test]
    fn test_money_amount() {
        for a in ["0", "10", "100.00", "0.99", "999999999.99"] {
            assert!(is_valid_money_amount(a), "{a} must be valid");
        }
        for a in ["", "-5", "10.123", "abc", "10,00", " 10"] {
            assert!(!is_valid_money_amount(a), "{a} must be invalid");
        }
    }

    #[test]
    fn test_valid_password() {
        assert!(is_valid_password("SecureP@ss123"));
        assert!(is_valid_password("MyP@ssw0rd!"));
    }

    #[test]
    fn test_invalid_password() {
        assert!(!is_valid_password("weak"));
        assert!(!is_valid_password("nouppercase1!"));
        assert!(!is_valid_password("NOLOWERCASE1!"));
        assert!(!is_valid_password("NoSpecial1"));
        assert!(!is_valid_password("NoDigit@!"));
        assert!(!is_valid_password(""));
    }

    #[test]
    fn test_valid_odds() {
        assert!(is_valid_odds("1.50"));
        assert!(is_valid_odds("2.00"));
        assert!(is_valid_odds("100.00"));
        assert!(is_valid_odds("1.01"));
        assert!(is_valid_odds("1000"));
    }

    #[test]
    fn test_invalid_odds() {
        assert!(!is_valid_odds("1.00")); // Below minimum
        assert!(!is_valid_odds("1000.01")); // Above maximum
        assert!(!is_valid_odds("1001.00"));
        assert!(!is_valid_odds("invalid"));
        assert!(!is_valid_odds("-2"));
        assert!(!is_valid_odds(""));
    }

    #[test]
    fn test_percentage() {
        for v in [0.0, 50.5, 100.0] {
            assert!(is_valid_percentage(v));
        }
        for v in [-0.1, 100.1, f64::NAN, f64::INFINITY] {
            assert!(!is_valid_percentage(v));
        }
    }

    #[test]
    fn test_ip() {
        for ip in ["127.0.0.1", "192.168.1.1", "2001:db8::1"] {
            assert!(is_valid_ip(ip), "{ip} must be valid");
        }
        for ip in ["", "999.1.1.1", "abc"] {
            assert!(!is_valid_ip(ip), "{ip} must be invalid");
        }
    }

    #[test]
    fn test_date_rejects_impossible_calendar_days() {
        assert!(is_valid_date("2026-09-30"));
        assert!(is_valid_date("2024-02-29")); // leap year
        assert!(!is_valid_date("2025-02-29")); // not a leap year
        assert!(!is_valid_date("2026-02-30"));
        assert!(!is_valid_date("2026-04-31"));
        assert!(!is_valid_date("2026-13-01"));
        assert!(!is_valid_date("2026-00-10"));
        assert!(!is_valid_date("2026-01-00"));
        assert!(!is_valid_date("30-09-2026"));
        assert!(!is_valid_date("2026-9-3"));
        assert!(!is_valid_date(""));
        assert!(!is_valid_date("1899-01-01"));
        assert!(!is_valid_date("2101-01-01"));
    }

    #[test]
    fn test_username() {
        for u in ["player1", "A_bc123", "abcdefghijklmnopqrst"] {
            assert!(is_valid_username(u), "{u} must be valid");
        }
        for u in ["", "ab", "1abc", "a", "abcdefghijklmnopqrstu", "a-b"] {
            assert!(!is_valid_username(u), "{u} must be invalid");
        }
    }
}
