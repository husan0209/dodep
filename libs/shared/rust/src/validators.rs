//! Validators for Opus Casino platform

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
pub fn is_valid_ip(ip: &str) -> bool {
    // IPv4
    static IPV4_REGEX: OnceLock<Regex> = OnceLock::new();
    let ipv4_regex = IPV4_REGEX.get_or_init(|| {
        Regex::new(r"^(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(?:\.(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}$").unwrap()
    });
    
    // IPv6 (simplified)
    static IPV6_REGEX: OnceLock<Regex> = OnceLock::new();
    let ipv6_regex = IPV6_REGEX.get_or_init(|| {
        Regex::new(r"^(?:[A-Fa-f0-9]{1,4}:){7}[A-Fa-f0-9]{1,4}$").unwrap()
    });
    
    ipv4_regex.is_match(ip) || ipv6_regex.is_match(ip)
}

/// Validate date string (ISO 8601 format: YYYY-MM-DD)
pub fn is_valid_date(date: &str) -> bool {
    static DATE_REGEX: OnceLock<Regex> = OnceLock::new();
    let regex = DATE_REGEX.get_or_init(|| {
        Regex::new(r"^\d{4}-\d{2}-\d{2}$").unwrap()
    });
    
    if !regex.is_match(date) {
        return false;
    }
    
    // Basic validation of date components
    let parts: Vec<&str> = date.split('-').collect();
    if parts.len() != 3 {
        return false;
    }
    
    let year: u32 = match parts[0].parse() {
        Ok(y) => y,
        Err(_) => return false,
    };
    
    let month: u32 = match parts[1].parse() {
        Ok(m) => m,
        Err(_) => return false,
    };
    
    let day: u32 = match parts[2].parse() {
        Ok(d) => d,
        Err(_) => return false,
    };
    
    year >= 1900 && year <= 2100 && month >= 1 && month <= 12 && day >= 1 && day <= 31
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
    }

    #[test]
    fn test_valid_odds() {
        assert!(is_valid_odds("1.50"));
        assert!(is_valid_odds("2.00"));
        assert!(is_valid_odds("100.00"));
    }

    #[test]
    fn test_invalid_odds() {
        assert!(!is_valid_odds("1.00"));  // Below minimum
        assert!(!is_valid_odds("1001.00"));  // Above maximum
        assert!(!is_valid_odds("invalid"));
    }
}
