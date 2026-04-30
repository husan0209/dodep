# opus-shared — Rust Shared Library

[![Crates.io](https://img.shields.io/badge/crates.io-internal-blue)]()
[![Documentation](https://img.shields.io/badge/docs-latest-blue)]()

Shared libraries and utilities for the Opus Casino gambling platform.

## Features

- **Types**: Strongly-typed identifiers (`UserId`, `BetId`, etc.), `Money` with decimal precision
- **Validators**: Email, UUID, password, odds, phone, and more
- **Constants**: Currency codes, rate limits, betting limits, error codes
- **Helpers**: Money arithmetic, UUID generation, retry logic, debounce/throttle
- **Error Handling**: Comprehensive error types with serialization support

## Installation

Add to `Cargo.toml`:

```toml
[dependencies]
opus-shared = { path = "../../libs/shared/rust" }
```

Or from git:

```toml
[dependencies]
opus-shared = { git = "https://github.com/opus-casino/opus-casino.git", subpackage = "libs/shared/rust" }
```

## Usage

### Types

```rust
use opus_shared::{UserId, Money, BetStatus, PaginationParams};

let user_id = UserId::new_v4();
let balance = Money::new("100.00", "USD")?;
let status = BetStatus::Pending;

let pagination = PaginationParams {
    page_size: 20,
    cursor: None,
    sort_by: Some("created_at".to_string()),
    descending: true,
};
```

### Validators

```rust
use opus_shared::{
    is_valid_email, is_valid_uuid, is_valid_password,
    is_valid_odds, is_valid_phone, is_valid_country_code
};

assert!(is_valid_email("user@example.com"));
assert!(is_valid_uuid("550e8400-e29b-41d4-a716-446655440000"));
assert!(is_valid_password("SecureP@ss123"));
assert!(is_valid_odds("2.50"));
assert!(is_valid_phone("+1234567890"));
assert!(is_valid_country_code("US"));
```

### Constants

```rust
use opus_shared::constants::{
    currencies, rate_limits, bet_limits, error_codes
};

println!("USD: {}", currencies::USD);
println!("Login attempts: {}", rate_limits::LOGIN_ATTEMPTS);
println!("Min stake: {}", bet_limits::MIN_STAKE);
println!("Auth error: {}", error_codes::AUTH_INVALID_CREDENTIALS);
```

### Helpers

```rust
use opus_shared::{
    format_money, generate_uuid, add_money,
    retry, Debouncer, Throttler
};

// Money formatting
let money = Money::new("100.50", "USD")?;
println!("{}", format_money(&money, "en-US"));  // $100.50

// UUID generation
let id = generate_uuid();

// Money arithmetic
let a = Money::new("100.00", "USD")?;
let b = Money::new("50.00", "USD")?;
let sum = add_money(&a, &b)?;

// Retry with backoff
let result = retry(
    || fetch_data(),
    3,      // max retries
    100,    // initial delay ms
    10000,  // max delay ms
    2.0,    // multiplier
).await?;

// Rate limiting
let debouncer = Debouncer::new(Duration::from_millis(300));
if debouncer.should_allow() {
    // Process request
}
```

### Error Handling

```rust
use opus_shared::{AppError, AppResult, ErrorDetails, FieldError};
use opus_shared::{validation_error, not_found, invalid_argument};

fn get_user(id: UserId) -> AppResult<User> {
    let user = db.find_user(id).await
        .map_err(|_| not_found!("User", id))?;
    Ok(user)
}

fn validate_bet(amount: &Money) -> AppResult<()> {
    if amount.amount < MIN_STAKE {
        return Err(validation_error!("Bet amount below minimum"));
    }
    Ok(())
}

// Convert to API response
let error = AppError::NotFound("User not found".to_string());
let details: ErrorDetails = error.into();
```

## API Reference

### Types

- `UserId`, `BetId`, `TransactionId`, `GameId`, `SessionId` — UUID-based identifiers
- `Money` — Decimal-precision monetary amount
- `PaginationParams`, `PaginationResult<T>` — Cursor-based pagination
- `DateRange` — Date range filter
- `ErrorDetails`, `FieldError` — Error types
- `ApiResponse<T>` — API response wrapper
- `HealthCheckResponse`, `HealthStatus` — Health check types
- `DeviceType`, `WalletType`, `TransactionType`, `BetType`, `BetStatus`, `KycLevel` — Enums

### Validators

- `is_valid_uuid(uuid)` — Validate UUID v4
- `is_valid_email(email)` — Validate email
- `is_valid_country_code(code)` — Validate ISO 3166-1 alpha-2
- `is_valid_currency_code(code)` — Validate ISO 4217
- `is_valid_money_amount(amount)` — Validate money string
- `is_valid_password(password)` — Validate password strength
- `is_valid_phone(phone)` — Validate E.164 phone
- `is_valid_odds(odds)` — Validate decimal odds
- `is_valid_percentage(value)` — Validate percentage
- `is_valid_ip(ip)` — Validate IPv4/IPv6
- `is_valid_date(date)` — Validate ISO date
- `is_valid_username(username)` — Validate username

### Constants

See `opus_shared::constants` module for:
- Currency codes
- Restricted countries
- Rate limits
- Betting limits
- Payment limits
- Session settings
- Error codes

### Helpers

- `format_money(money, locale)` — Format money for display
- `parse_money(amount, currency)` — Parse to Money object
- `add_money(a, b)`, `subtract_money(a, b)` — Money arithmetic
- `multiply_money(money, scalar)` — Multiply money
- `compare_money(a, b)` — Compare money amounts
- `generate_uuid()` — Generate UUID v4
- `now_ms()`, `now_iso()` — Current timestamp
- `retry(operation, ...)` — Retry with exponential backoff
- `Debouncer`, `Throttler` — Rate limiting helpers

## Development

```bash
# Build
cargo build

# Run tests
cargo test

# Run clippy
cargo clippy -- -D warnings

# Format code
cargo fmt

# Generate docs
cargo doc --open
```

## License

Proprietary — все права защищены
