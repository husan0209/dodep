//! Constants for Opus Casino platform

use rust_decimal::Decimal;
use std::sync::LazyLock;

/// Default pagination settings
pub mod pagination {
    pub const DEFAULT_PAGE_SIZE: i32 = 20;
    pub const MAX_PAGE_SIZE: i32 = 100;
    pub const MIN_PAGE_SIZE: i32 = 1;
}

/// Currency constants
pub mod currencies {
    pub const USD: &str = "USD";
    pub const EUR: &str = "EUR";
    pub const GBP: &str = "GBP";
    pub const RUB: &str = "RUB";
    pub const BRL: &str = "BRL";
    pub const INR: &str = "INR";
    pub const JPY: &str = "JPY";
    pub const CNY: &str = "CNY";
    pub const KRW: &str = "KRW";
    pub const CAD: &str = "CAD";
    pub const AUD: &str = "AUD";
    pub const CHF: &str = "CHF";
    pub const BTC: &str = "BTC";
    pub const ETH: &str = "ETH";
    pub const USDT: &str = "USDT";
}

/// Restricted countries (gambling restricted jurisdictions)
pub static RESTRICTED_COUNTRIES: &[&str] = &[
    "US", // United States (restricted states)
    "FR", // France (requires license)
    "IT", // Italy (requires license)
    "ES", // Spain (requires license)
    "NL", // Netherlands (requires license)
    "BE", // Belgium (requires license)
    "CH", // Switzerland (requires license)
    "AU", // Australia (interactive gambling act)
    "SG", // Singapore
    "HK", // Hong Kong
    "JP", // Japan (restrictions apply)
    "KR", // South Korea
    "CN", // China
    "RU", // Russia (restrictions apply)
];

/// Wallet types
pub mod wallet_types {
    pub const MAIN: &str = "main";
    pub const BONUS: &str = "bonus";
    pub const FREE_SPINS: &str = "free_spins";
    pub const CASHBACK: &str = "cashback";
}

/// Transaction types
pub mod transaction_types {
    pub const DEPOSIT: &str = "deposit";
    pub const WITHDRAWAL: &str = "withdrawal";
    pub const BET_PLACE: &str = "bet_place";
    pub const BET_WIN: &str = "bet_win";
    pub const BET_REFUND: &str = "bet_refund";
    pub const BONUS_CREDIT: &str = "bonus_credit";
    pub const BONUS_DEBIT: &str = "bonus_debit";
    pub const TRANSFER: &str = "transfer";
    pub const ADJUSTMENT: &str = "adjustment";
}

/// Bet types
pub mod bet_types {
    pub const SPORTS: &str = "sports";
    pub const LIVE: &str = "live";
    pub const CASINO: &str = "casino";
    pub const LOTTERY: &str = "lottery";
    pub const VIRTUAL: &str = "virtual";
}

/// Bet statuses
pub mod bet_statuses {
    pub const PENDING: &str = "pending";
    pub const ACCEPTED: &str = "accepted";
    pub const SETTLED: &str = "settled";
    pub const CANCELLED: &str = "cancelled";
    pub const REJECTED: &str = "rejected";
}

/// Bonus types
pub mod bonus_types {
    pub const WELCOME: &str = "welcome";
    pub const DEPOSIT: &str = "deposit";
    pub const NO_DEPOSIT: &str = "no_deposit";
    pub const FREE_SPINS: &str = "free_spins";
    pub const CASHBACK: &str = "cashback";
    pub const RELOAD: &str = "reload";
    pub const VIP: &str = "vip";
    pub const LOYALTY: &str = "loyalty";
    pub const TOURNAMENT: &str = "tournament";
}

/// KYC levels
pub mod kyc_levels {
    pub const NONE: &str = "none";
    pub const BASIC: &str = "basic";
    pub const IDENTITY: &str = "identity";
    pub const ENHANCED: &str = "enhanced";
    pub const VIP: &str = "vip";
}

/// Notification channels
pub mod notification_channels {
    pub const EMAIL: &str = "email";
    pub const SMS: &str = "sms";
    pub const PUSH: &str = "push";
    pub const IN_APP: &str = "in_app";
    pub const TELEGRAM: &str = "telegram";
    pub const WHATSAPP: &str = "whatsapp";
}

/// Notification types
pub mod notification_types {
    pub const WELCOME: &str = "welcome";
    pub const DEPOSIT_CONFIRMED: &str = "deposit_confirmed";
    pub const WITHDRAWAL_PROCESSED: &str = "withdrawal_processed";
    pub const BET_SETTLED: &str = "bet_settled";
    pub const BONUS_ACTIVATED: &str = "bonus_activated";
    pub const BONUS_EXPIRING: &str = "bonus_expiring";
    pub const KYC_STATUS: &str = "kyc_status";
    pub const SECURITY_ALERT: &str = "security_alert";
    pub const PROMOTION: &str = "promotion";
    pub const TOURNAMENT: &str = "tournament";
    pub const VIP_UPDATE: &str = "vip_update";
    pub const REALITY_CHECK: &str = "reality_check";
    pub const SYSTEM: &str = "system";
}

/// Device types
pub mod device_types {
    pub const WEB: &str = "web";
    pub const MOBILE_WEB: &str = "mobile_web";
    pub const IOS: &str = "ios";
    pub const ANDROID: &str = "android";
    pub const DESKTOP: &str = "desktop";
}

/// Rate limits
pub mod rate_limits {
    // Authentication
    pub const LOGIN_ATTEMPTS: u32 = 5;
    pub const LOGIN_WINDOW_MS: u64 = 15 * 60 * 1000; // 15 minutes
    
    // API
    pub const API_REQUESTS_PER_MINUTE: u32 = 100;
    pub const API_REQUESTS_PER_HOUR: u32 = 1000;
    
    // Betting
    pub const BETS_PER_SECOND: u32 = 10;
    
    // Withdrawal
    pub const WITHDRAWAL_REQUESTS_PER_DAY: u32 = 5;
    
    // Password reset
    pub const PASSWORD_RESET_PER_HOUR: u32 = 3;
    
    // 2FA
    pub const TOTP_WINDOW_SECONDS: u64 = 30;
    pub const TOTP_MAX_ATTEMPTS: u32 = 5;
}

/// Betting limits
pub mod bet_limits {
    use rust_decimal::Decimal;
    
    pub static MIN_STAKE: LazyLock<Decimal> = LazyLock::new(|| Decimal::new(10, 2)); // 0.10
    pub static MAX_STAKE: LazyLock<Decimal> = LazyLock::new(|| Decimal::new(1000000, 2)); // 10000.00
    pub const MAX_WIN_MULTIPLIER: u32 = 10000; // Max win = stake * multiplier
    pub static MAX_ODDS: LazyLock<Decimal> = LazyLock::new(|| Decimal::new(100000, 2)); // 1000.00
    pub static MIN_ODDS: LazyLock<Decimal> = LazyLock::new(|| Decimal::new(101, 2)); // 1.01
}

/// Payment limits
pub mod payment_limits {
    use rust_decimal::Decimal;
    
    pub static MIN_DEPOSIT: LazyLock<Decimal> = LazyLock::new(|| Decimal::new(100, 2)); // 1.00
    pub static MAX_DEPOSIT_DAILY: LazyLock<Decimal> = LazyLock::new(|| Decimal::new(1000000, 2)); // 10000.00
    pub static MIN_WITHDRAWAL: LazyLock<Decimal> = LazyLock::new(|| Decimal::new(1000, 2)); // 10.00
    pub static MAX_WITHDRAWAL_DAILY: LazyLock<Decimal> = LazyLock::new(|| Decimal::new(5000000, 2)); // 50000.00
    pub static MAX_WITHDRAWAL_MONTHLY: LazyLock<Decimal> = LazyLock::new(|| Decimal::new(50000000, 2)); // 500000.00
}

/// Session settings
pub mod session {
    pub const ACCESS_TOKEN_TTL_SECONDS: u64 = 900; // 15 minutes
    pub const REFRESH_TOKEN_TTL_SECONDS: u64 = 604800; // 7 days
    pub const SESSION_TTL_SECONDS: u64 = 2592000; // 30 days
    pub const MAX_SESSIONS_PER_USER: usize = 5;
}

/// Responsible gambling limits
pub mod responsible_gambling {
    pub const REALITY_CHECK_DEFAULT_MINUTES: u32 = 60;
    pub const SESSION_TIME_LIMIT_DEFAULT_MINUTES: u32 = 120;
    pub const SELF_EXCLUSION_MIN_DAYS: u32 = 6;
    pub const SELF_EXCLUSION_MAX_YEARS: u32 = 5;
    pub const COOLDOWN_PERIOD_DAYS: u32 = 24; // After limit decrease
}

/// Error codes
pub mod error_codes {
    // Authentication (1000-1999)
    pub const AUTH_INVALID_CREDENTIALS: &str = "AUTH_1001";
    pub const AUTH_TOKEN_EXPIRED: &str = "AUTH_1002";
    pub const AUTH_TOKEN_INVALID: &str = "AUTH_1003";
    pub const AUTH_2FA_REQUIRED: &str = "AUTH_1006";
    pub const AUTH_2FA_INVALID: &str = "AUTH_1007";
    pub const AUTH_ACCOUNT_LOCKED: &str = "AUTH_1008";
    
    // Wallet (5000-5999)
    pub const WALLET_NOT_FOUND: &str = "WALLET_5001";
    pub const INSUFFICIENT_BALANCE: &str = "WALLET_5002";
    pub const INSUFFICIENT_AVAILABLE_BALANCE: &str = "WALLET_5003";
    
    // Bet (7000-7999)
    pub const BET_NOT_FOUND: &str = "BET_7001";
    pub const BET_INVALID: &str = "BET_7002";
    pub const BET_ALREADY_SETTLED: &str = "BET_7003";
    pub const BET_LIMIT_EXCEEDED: &str = "BET_7005";
    pub const BET_ODDS_CHANGED: &str = "BET_7007";
    
    // System (11000-11999)
    pub const INTERNAL_ERROR: &str = "SYS_11001";
    pub const SERVICE_UNAVAILABLE: &str = "SYS_11002";
    pub const RATE_LIMIT_EXCEEDED: &str = "SYS_11005";
}
