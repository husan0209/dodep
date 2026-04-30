# SKILL #19 — go-error-handling.skill.md

```markdown
# go-error-handling.skill.md
# GAMBLING PLATFORM — GO ERROR HANDLING PATTERNS
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: Go Business Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

Go uses explicit error returns — no exceptions, no panics for flow control.
Errors are values. Wrap with context, match with errors.Is/As.

Three error categories:
  Sentinel errors → compared with errors.Is() (domain errors)
  Typed errors → inspected with errors.As() (validation, detailed)
  Opaque errors → wrapped with fmt.Errorf("context: %w", err)

# ============================================================
# SECTION 2: DOMAIN ERRORS (sentinel)
# ============================================================

```go
// internal/domain/errors.go

package domain

import "errors"

// ── Sentinel errors — compared with errors.Is() ──

// Authentication
var (
    ErrInvalidCredentials  = errors.New("invalid credentials")
    ErrAccountLocked       = errors.New("account locked")
    ErrAccountSuspended    = errors.New("account suspended")
    ErrSelfExcluded        = errors.New("self-excluded")
    ErrTokenExpired        = errors.New("token expired")
    ErrTokenInvalid        = errors.New("token invalid")
    Err2FARequired         = errors.New("2FA required")
    Err2FAInvalid          = errors.New("invalid 2FA code")
)

// User
var (
    ErrUserNotFound   = errors.New("user not found")
    ErrEmailExists    = errors.New("email exists")
    ErrPhoneExists    = errors.New("phone exists")
    ErrCountryBlocked = errors.New("country blocked")
)

// Wallet
var (
    ErrInsufficientBalance  = errors.New("insufficient balance")
    ErrConcurrencyConflict  = errors.New("concurrency conflict")
    ErrWalletLocked         = errors.New("wallet locked")
)

// Betting
var (
    ErrEventSuspended    = errors.New("event suspended")
    ErrMarketClosed      = errors.New("market closed")
    ErrOddsChanged       = errors.New("odds changed")
    ErrBetAlreadySettled = errors.New("bet already settled")
    ErrCashoutUnavailable = errors.New("cashout unavailable")
)

// Payment
var (
    ErrPaymentDeclined     = errors.New("payment declined")
    ErrKYCRequired         = errors.New("KYC required")
    ErrDepositLimitExceeded = errors.New("deposit limit exceeded")
    ErrWageringIncomplete  = errors.New("wagering incomplete")
)

// Generic
var (
    ErrNotFound    = errors.New("not found")
    ErrConflict    = errors.New("conflict")
    ErrForbidden   = errors.New("forbidden")
    ErrRateLimited = errors.New("rate limited")
)
============================================================
SECTION 3: TYPED ERRORS (with details)
============================================================
Go

// ── Validation error with field details ──

type ValidationError struct {
    Fields []FieldError
}

type FieldError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
}

func (e *ValidationError) Error() string {
    if len(e.Fields) == 0 {
        return "validation failed"
    }
    return fmt.Sprintf("validation: %s — %s", e.Fields[0].Field, e.Fields[0].Message)
}

func NewValidationError(fields ...FieldError) *ValidationError {
    return &ValidationError{Fields: fields}
}

// ── Detailed error with arbitrary context ──

type DetailedError struct {
    Err     error
    Details map[string]interface{}
}

func (e *DetailedError) Error() string { return e.Err.Error() }
func (e *DetailedError) Unwrap() error { return e.Err }

func WithDetails(err error, details map[string]interface{}) error {
    return &DetailedError{Err: err, Details: details}
}

// Usage:
// return WithDetails(ErrInsufficientBalance, map[string]interface{}{
//     "required": "100.00",
//     "available": "50.00",
// })
============================================================
SECTION 4: ERROR WRAPPING
============================================================
Go

// ── ALWAYS wrap with context using %w ──

// ✅ GOOD: Adds context, preserves error chain
func (s *AuthService) Login(ctx context.Context, email, password string) (*Token, error) {
    user, err := s.userRepo.GetByEmail(ctx, email)
    if err != nil {
        return nil, fmt.Errorf("get user by email: %w", err) // wraps with context
    }
    if user == nil {
        return nil, ErrInvalidCredentials // sentinel — no wrapping needed
    }
    
    if !verifyPassword(password, user.PasswordHash) {
        return nil, ErrInvalidCredentials
    }
    
    token, err := s.tokenSvc.Generate(user)
    if err != nil {
        return nil, fmt.Errorf("generate token: %w", err)
    }
    
    return token, nil
}

// ❌ BAD: No context
func (s *AuthService) Login(...) (*Token, error) {
    user, err := s.userRepo.GetByEmail(ctx, email)
    if err != nil {
        return nil, err // ❌ no context — hard to trace in logs
    }
}

// ❌ BAD: Using %v instead of %w (breaks errors.Is chain)
func (s *AuthService) Login(...) (*Token, error) {
    user, err := s.userRepo.GetByEmail(ctx, email)
    if err != nil {
        return nil, fmt.Errorf("get user: %v", err) // ❌ %v wraps as string, not error
    }
}

// ❌ BAD: Wrapping sentinel errors (double wrapping)
if user == nil {
    return nil, fmt.Errorf("login failed: %w", ErrInvalidCredentials) // ❌ unnecessary wrap
}
// Sentinel errors should be returned directly, not wrapped
============================================================
SECTION 5: ERROR CHECKING
============================================================
Go

// ── errors.Is() for sentinel errors ──
if errors.Is(err, domain.ErrUserNotFound) {
    return respondError(c, 404, "NOT_FOUND", "User not found")
}

// ── errors.As() for typed errors ──
var validationErr *domain.ValidationError
if errors.As(err, &validationErr) {
    return respondErrorWithDetails(c, 400, "VALIDATION_FAILED",
        "Validation failed", validationErr.Fields)
}

var detailedErr *domain.DetailedError
if errors.As(err, &detailedErr) {
    // Access details
    required := detailedErr.Details["required"]
    // Match inner error
    if errors.Is(detailedErr.Err, domain.ErrInsufficientBalance) {
        return respondErrorWithDetails(c, 422, "INSUFFICIENT_BALANCE",
            "Insufficient balance", detailedErr.Details)
    }
}
============================================================
SECTION 6: HANDLER ERROR MAPPING
============================================================
Go

// Central error mapping — single place for all domain → HTTP mappings

func (h *Handler) mapError(c *fiber.Ctx, err error) error {
    // Typed errors first (errors.As)
    var validationErr *domain.ValidationError
    if errors.As(err, &validationErr) {
        return respondErrorWithDetails(c, 400, "VALIDATION_FAILED",
            "Validation failed", validationErr.Fields)
    }

    // Unwrap DetailedError for sentinel matching
    var detailedErr *domain.DetailedError
    var details interface{}
    if errors.As(err, &detailedErr) {
        details = detailedErr.Details
        err = detailedErr.Err
    }

    // Sentinel errors
    switch {
    // 401
    case errors.Is(err, domain.ErrInvalidCredentials):
        return respondError(c, 401, "INVALID_CREDENTIALS", "Invalid email or password")
    case errors.Is(err, domain.ErrTokenExpired):
        return respondError(c, 401, "TOKEN_EXPIRED", "Token has expired")
    case errors.Is(err, domain.ErrTokenInvalid):
        return respondError(c, 401, "TOKEN_INVALID", "Invalid token")

    // 403
    case errors.Is(err, domain.ErrAccountLocked):
        return respondError(c, 403, "ACCOUNT_LOCKED", "Account is temporarily locked")
    case errors.Is(err, domain.ErrSelfExcluded):
        return respondError(c, 403, "SELF_EXCLUDED", "Account is self-excluded")
    case errors.Is(err, domain.ErrForbidden):
        return respondError(c, 403, "FORBIDDEN", "Access denied")
    case errors.Is(err, domain.ErrCountryBlocked):
        return respondError(c, 403, "COUNTRY_BLOCKED", "Not available in your country")

    // 404
    case errors.Is(err, domain.ErrNotFound),
         errors.Is(err, domain.ErrUserNotFound):
        return respondError(c, 404, "NOT_FOUND", "Resource not found")

    // 409
    case errors.Is(err, domain.ErrEmailExists):
        return respondError(c, 409, "EMAIL_EXISTS", "Email already registered")
    case errors.Is(err, domain.ErrConflict):
        return respondError(c, 409, "CONFLICT", "Resource conflict")

    // 422
    case errors.Is(err, domain.ErrInsufficientBalance):
        return respondErrorWithDetails(c, 422, "INSUFFICIENT_BALANCE",
            "Insufficient balance", details)
    case errors.Is(err, domain.ErrKYCRequired):
        return respondError(c, 422, "KYC_REQUIRED", "KYC verification required")
    case errors.Is(err, domain.ErrDepositLimitExceeded):
        return respondErrorWithDetails(c, 422, "DEPOSIT_LIMIT",
            "Deposit limit exceeded", details)

    // 429
    case errors.Is(err, domain.ErrRateLimited):
        c.Set("Retry-After", "60")
        return respondError(c, 429, "RATE_LIMITED", "Too many requests")

    // 500
    default:
        log.Error().Err(err).Str("request_id", getRequestID(c)).Msg("Internal error")
        return respondError(c, 500, "INTERNAL_ERROR", "An internal error occurred")
    }
}
============================================================
SECTION 7: RULES
============================================================
text

1. ALWAYS return error as last return value
2. ALWAYS check returned errors (never _ = mightFail())
   Exception: best-effort operations with explicit comment
   _ = cache.Del(ctx, key) // best-effort cleanup
3. ALWAYS wrap with fmt.Errorf("context: %w", err) for opaque errors
4. NEVER wrap sentinel errors — return them directly
5. NEVER use panic() for error handling (only truly unrecoverable in init)
6. NEVER return generic errors.New("something failed") — use domain sentinels
7. NEVER log AND return the same error (log at top level only)
8. NEVER expose internal error messages to API clients
9. Use errors.Is() for sentinel, errors.As() for typed
10. Map ALL errors to HTTP in ONE place (mapError function)
============================================================
SECTION 8: ANTI-PATTERNS
============================================================
Go

// ❌ BAD: Log and return (double logging)
func (s *Service) DoSomething() error {
    err := s.repo.Save(ctx, data)
    if err != nil {
        log.Error().Err(err).Msg("Failed to save") // ❌ logged here
        return err                                    // ❌ AND logged again in handler
    }
}

// ✅ GOOD: Return with context, log once at top level
func (s *Service) DoSomething() error {
    if err := s.repo.Save(ctx, data); err != nil {
        return fmt.Errorf("save data: %w", err) // handler or mapError will log
    }
    return nil
}

// ❌ BAD: Swallowing errors
result, _ := s.repo.GetByID(ctx, id) // ❌ error ignored!

// ✅ GOOD: Handle or propagate
result, err := s.repo.GetByID(ctx, id)
if err != nil {
    return nil, fmt.Errorf("get by id: %w", err)
}

// ❌ BAD: String comparison for errors
if err.Error() == "user not found" { // ❌ brittle

// ✅ GOOD: Sentinel comparison
if errors.Is(err, domain.ErrUserNotFound) { // ✅ robust