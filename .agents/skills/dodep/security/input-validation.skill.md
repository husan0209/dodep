## #47 input-validation.skill.md

```markdown
# input-validation.skill.md

## РОЛЬ
Ты валидируешь ВСЕ входные данные на каждом уровне
гемблинг-платформы. Невалидированные данные — уязвимость.

## КОНТЕКСТ
- Каждый HTTP/gRPC запрос валидируется ДО бизнес-логики
- Клиентская валидация = UX, серверная = security
- Финансовые суммы: строгая валидация (precision, range)
- Gambling-specific: odds range, stake limits, event validity

## VALIDATION LAYERS
┌─────────────────────────────────────────┐
│ Layer 1: CloudFlare WAF │
│ SQL injection, XSS, bot detection │
│ │
│ Layer 2: API Gateway (Kong) │
│ Request size, rate limit, schema │
│ │
│ Layer 3: Application (handler) │
│ Type check, format, range, business │
│ │
│ Layer 4: Domain (service) │
│ Business rules, state validation │
│ │
│ Layer 5: Database │
│ Constraints, CHECK, UNIQUE, FK │
└─────────────────────────────────────────┘

text


## RUST VALIDATION

```rust
use validator::{Validate, ValidationError};
use regex::Regex;
use lazy_static::lazy_static;

lazy_static! {
    static ref SAFE_STRING_RE: Regex = Regex::new(r"^[\w\s\-\.@]+$").unwrap();
    static ref PHONE_RE: Regex = Regex::new(r"^\+[1-9]\d{6,14}$").unwrap();
}

// Registration request
#[derive(Deserialize, Validate)]
pub struct RegisterRequest {
    #[validate(
        email(message = "Invalid email format"),
        length(max = 255, message = "Email too long")
    )]
    pub email: String,

    #[validate(
        length(min = 8, max = 128, message = "Password must be 8-128 chars"),
        custom = "validate_password_strength"
    )]
    pub password: String,

    #[validate(
        length(equal = 2, message = "Country code must be 2 chars"),
        custom = "validate_country_code"
    )]
    pub country_code: String,

    #[validate(
        length(equal = 3, message = "Currency code must be 3 chars"),
        custom = "validate_currency_code"
    )]
    pub currency_code: String,

    #[validate(custom = "validate_must_be_true")]
    pub age_confirmed: bool,

    #[validate(custom = "validate_must_be_true")]
    pub terms_accepted: bool,
}

fn validate_password_strength(password: &str) -> Result<(), ValidationError> {
    let has_upper = password.chars().any(|c| c.is_uppercase());
    let has_lower = password.chars().any(|c| c.is_lowercase());
    let has_digit = password.chars().any(|c| c.is_ascii_digit());
    let has_special = password.chars().any(|c| !c.is_alphanumeric());

    if !has_upper || !has_lower || !has_digit || !has_special {
        return Err(ValidationError::new("password_weak")
            .with_message("Password must have uppercase, lowercase, digit, special char".into()));
    }
    Ok(())
}

fn validate_country_code(code: &str) -> Result<(), ValidationError> {
    let blocked = ["US", "GB", "FR", "AU"];  // из конфига
    if blocked.contains(&code) {
        return Err(ValidationError::new("country_blocked"));
    }
    // Проверка ISO 3166-1
    if !iso_codes::COUNTRIES.contains(&code) {
        return Err(ValidationError::new("invalid_country"));
    }
    Ok(())
}

fn validate_must_be_true(value: &bool) -> Result<(), ValidationError> {
    if !value {
        return Err(ValidationError::new("must_be_true"));
    }
    Ok(())
}

// Bet placement request
#[derive(Deserialize, Validate)]
pub struct PlaceBetRequest {
    #[validate(custom = "validate_bet_type")]
    pub bet_type: String,

    #[validate(length(min = 1, max = 20, message = "1-20 selections allowed"))]
    #[validate]
    pub selections: Vec<SelectionInput>,

    #[validate(custom = "validate_stake")]
    pub stake: f64,

    #[validate(length(equal = 3))]
    pub currency: String,

    #[validate(custom = "validate_odds_acceptance")]
    pub accept_odds_changes: String,

    #[validate(custom = "validate_uuid")]
    pub idempotency_key: String,
}

#[derive(Deserialize, Validate)]
pub struct SelectionInput {
    #[validate(range(min = 1, message = "Invalid event_id"))]
    pub event_id: i64,

    #[validate(range(min = 1, message = "Invalid market_id"))]
    pub market_id: i64,

    #[validate(range(min = 1, message = "Invalid outcome_id"))]
    pub outcome_id: i64,

    #[validate(custom = "validate_odds")]
    pub odds: f64,
}

fn validate_stake(stake: &f64) -> Result<(), ValidationError> {
    if !stake.is_finite() || stake.is_nan() {
        return Err(ValidationError::new("invalid_number"));
    }
    if *stake < 0.01 {
        return Err(ValidationError::new("stake_too_low")
            .with_message("Minimum stake is 0.01".into()));
    }
    if *stake > 100_000.0 {
        return Err(ValidationError::new("stake_too_high")
            .with_message("Maximum stake is 100,000".into()));
    }
    // Проверка на precision (max 2 decimal places для фиата)
    let cents = (*stake * 100.0).round();
    if (cents - *stake * 100.0).abs() > 0.001 {
        return Err(ValidationError::new("stake_precision")
            .with_message("Maximum 2 decimal places".into()));
    }
    Ok(())
}

fn validate_odds(odds: &f64) -> Result<(), ValidationError> {
    if !odds.is_finite() || odds.is_nan() {
        return Err(ValidationError::new("invalid_odds"));
    }
    if *odds < 1.01 {
        return Err(ValidationError::new("odds_too_low"));
    }
    if *odds > 10_000.0 {
        return Err(ValidationError::new("odds_too_high"));
    }
    Ok(())
}

fn validate_uuid(value: &str) -> Result<(), ValidationError> {
    uuid::Uuid::parse_str(value)
        .map_err(|_| ValidationError::new("invalid_uuid"))?;
    Ok(())
}
GO VALIDATION
Go

import "github.com/go-playground/validator/v10"

var validate *validator.Validate

func init() {
    validate = validator.New()
    validate.RegisterValidation("safe_string", validateSafeString)
    validate.RegisterValidation("not_blocked_country", validateNotBlockedCountry)
}

type DepositRequest struct {
    Amount   float64 `json:"amount" validate:"required,gt=0,lte=100000"`
    Currency string  `json:"currency" validate:"required,len=3,iso4217"`
    Method   string  `json:"method" validate:"required,oneof=card bank_transfer crypto e_wallet"`
}

type WithdrawalRequest struct {
    Amount      float64 `json:"amount" validate:"required,gt=0,lte=50000"`
    Currency    string  `json:"currency" validate:"required,len=3"`
    Method      string  `json:"method" validate:"required,oneof=card bank_transfer crypto"`
    Destination string  `json:"destination" validate:"required,max=255"`
}

type SearchRequest struct {
    Query    string `json:"query" validate:"omitempty,max=100,safe_string"`
    Page     int    `json:"page" validate:"omitempty,min=1,max=1000"`
    PageSize int    `json:"page_size" validate:"omitempty,min=1,max=100"`
    SortBy   string `json:"sort_by" validate:"omitempty,oneof=created_at amount status"`
    Order    string `json:"order" validate:"omitempty,oneof=asc desc"`
}

func validateSafeString(fl validator.FieldLevel) bool {
    value := fl.Field().String()
    // Запретить SQL injection, XSS паттерны
    dangerous := []string{"<script", "DROP TABLE", "DELETE FROM",
        "INSERT INTO", "UPDATE SET", "UNION SELECT", "--", "/*", "*/"}
    lower := strings.ToLower(value)
    for _, d := range dangerous {
        if strings.Contains(lower, strings.ToLower(d)) {
            return false
        }
    }
    return true
}

// Validation middleware
func ValidateBody[T any](c *fiber.Ctx) (*T, error) {
    var body T
    if err := c.BodyParser(&body); err != nil {
        return nil, NewValidationError("INVALID_JSON", "Invalid request body")
    }
    if err := validate.Struct(&body); err != nil {
        errors := make([]FieldError, 0)
        for _, e := range err.(validator.ValidationErrors) {
            errors = append(errors, FieldError{
                Field:   e.Field(),
                Message: formatValidationError(e),
            })
        }
        return nil, NewValidationError("VALIDATION_FAILED", errors)
    }
    return &body, nil
}
FRONTEND VALIDATION (Zod)
TypeScript

import { z } from 'zod';

export const registerSchema = z.object({
  email: z
    .string()
    .email('Invalid email')
    .max(255)
    .transform(v => v.toLowerCase().trim()),

  password: z
    .string()
    .min(8, 'Minimum 8 characters')
    .max(128)
    .regex(/[A-Z]/, 'Must contain uppercase')
    .regex(/[a-z]/, 'Must contain lowercase')
    .regex(/[0-9]/, 'Must contain digit')
    .regex(/[^A-Za-z0-9]/, 'Must contain special character'),

  country: z
    .string()
    .length(2)
    .refine(v => !BLOCKED_COUNTRIES.includes(v), 'Country not available'),

  currency: z.string().length(3),

  ageConfirmed: z.literal(true, {
    errorMap: () => ({ message: 'You must be 18+' }),
  }),

  termsAccepted: z.literal(true, {
    errorMap: () => ({ message: 'Terms must be accepted' }),
  }),
});

export const placeBetSchema = z.object({
  betType: z.enum(['single', 'accumulator', 'system']),
  selections: z
    .array(z.object({
      eventId: z.number().positive(),
      marketId: z.number().positive(),
      outcomeId: z.number().positive(),
      odds: z.number().min(1.01).max(10000),
    }))
    .min(1)
    .max(20),
  stake: z
    .number()
    .positive()
    .max(100000)
    .multipleOf(0.01),  // max 2 decimal places
  currency: z.string().length(3),
  acceptOddsChanges: z.enum(['none', 'higher', 'any']),
  idempotencyKey: z.string().uuid(),
});

export const depositSchema = z.object({
  amount: z
    .number()
    .positive('Amount must be positive')
    .min(10, 'Minimum deposit is $10')
    .max(100000, 'Maximum deposit is $100,000')
    .multipleOf(0.01),
  currency: z.string().length(3),
  method: z.enum(['card', 'bank_transfer', 'crypto', 'e_wallet']),
});
SANITIZATION
Rust

// Rust — sanitize user input
pub fn sanitize_string(input: &str) -> String {
    input
        .trim()
        .chars()
        .filter(|c| !c.is_control())   // удалить control characters
        .take(1000)                      // ограничить длину
        .collect()
}

pub fn sanitize_search_query(input: &str) -> String {
    input
        .trim()
        .chars()
        .filter(|c| c.is_alphanumeric() || *c == ' ' || *c == '-' || *c == '.')
        .take(100)
        .collect()
}

// Для имён: разрешить Unicode буквы, пробелы, дефисы
pub fn sanitize_name(input: &str) -> String {
    input
        .trim()
        .chars()
        .filter(|c| c.is_alphabetic() || *c == ' ' || *c == '-' || *c == '\'')
        .take(100)
        .collect()
}
Go

// Go — sanitization
func SanitizeString(input string) string {
    // Remove null bytes
    input = strings.ReplaceAll(input, "\x00", "")
    // Trim
    input = strings.TrimSpace(input)
    // Limit length
    if len(input) > 1000 {
        input = input[:1000]
    }
    return input
}

func SanitizeForSQL(input string) string {
    // Это НЕ замена параметризованных запросов!
    // Это дополнительный слой для строк, используемых в LIKE
    replacer := strings.NewReplacer(
        "%", "\\%",
        "_", "\\_",
    )
    return replacer.Replace(input)
}
АНТИПАТТЕРНЫ
Rust

// ❌ ПЛОХО: валидация после бизнес-логики
async fn place_bet(req: PlaceBetRequest) {
    let bet = service.place_bet(req).await?;  // может упасть с непонятной ошибкой
    req.validate()?;  // слишком поздно!
}

// ✅ ПРАВИЛЬНО: валидация ДО всего
async fn place_bet(req: PlaceBetRequest) {
    req.validate()?;  // первая строка
    let bet = service.place_bet(req).await?;
}

// ❌ ПЛОХО: доверять Content-Length без проверки
// Запрос 10GB → OOM

// ✅ ПРАВИЛЬНО: лимит на размер body
// Kong: request-size-limiting plugin (10MB max)
// Axum: ContentLengthLimit<Json<T>, 1_048_576>

// ❌ ПЛОХО: принимать любой тип файла
if file.size > 0 { upload(file); }

// ✅ ПРАВИЛЬНО: проверять magic bytes, не только расширение
let magic = &file_bytes[0..4];
match magic {
    [0xFF, 0xD8, 0xFF, ..] => "image/jpeg",
    [0x89, 0x50, 0x4E, 0x47] => "image/png",
    [0x25, 0x50, 0x44, 0x46] => "application/pdf",
    _ => return Err(Error::UnsupportedFileType),
}

// ❌ ПЛОХО: float для денег без проверки
let amount: f64 = req.amount;  // может быть NaN, Infinity, -0

// ✅ ПРАВИЛЬНО:
if !amount.is_finite() || amount.is_nan() || amount <= 0.0 {
    return Err(Error::InvalidAmount);
}