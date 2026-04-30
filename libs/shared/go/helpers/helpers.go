// Package helpers provides helper functions for Opus Casino platform.
package helpers

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/opus-casino/shared/go/types"
)

// FormatMoney formats money for display
func FormatMoney(money types.Money, locale string) string {
	symbol := getCurrencySymbol(money.Currency)
	return fmt.Sprintf("%s%s", symbol, money.Amount.StringFixed(2))
}

// getCurrencySymbol returns currency symbol
func getCurrencySymbol(currency string) string {
	symbols := map[string]string{
		"USD": "$",
		"EUR": "€",
		"GBP": "£",
		"RUB": "₽",
		"JPY": "¥",
	}
	if symbol, ok := symbols[currency]; ok {
		return symbol
	}
	return currency + " "
}

// ParseMoney parses money string to Money object
func ParseMoney(amount string, currency string) (types.Money, error) {
	return types.NewMoney(amount, currency)
}

// AddMoney adds two money amounts (must be same currency)
func AddMoney(a, b types.Money) (types.Money, error) {
	return a.Add(b)
}

// SubtractMoney subtracts two money amounts (must be same currency)
func SubtractMoney(a, b types.Money) (types.Money, error) {
	return a.Subtract(b)
}

// MultiplyMoney multiplies money by a scalar
func MultiplyMoney(money types.Money, scalar decimal.Decimal) types.Money {
	return money.Multiply(scalar)
}

// CompareMoney compares two money amounts
// Returns: -1 if a < b, 0 if a == b, 1 if a > b
func CompareMoney(a, b types.Money) (int, error) {
	if a.Currency != b.Currency {
		return 0, types.ErrCurrencyMismatch
	}

	if a.Amount.LessThan(b.Amount) {
		return -1, nil
	} else if a.Amount.GreaterThan(b.Amount) {
		return 1, nil
	}
	return 0, nil
}

// GenerateUUID generates UUID v4
func GenerateUUID() string {
	return uuid.New().String()
}

// NowMs returns current timestamp in milliseconds
func NowMs() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

// NowISO returns current ISO 8601 timestamp
func NowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// DeepClone deep clones a serializable object
func DeepClone[T any](obj T) (*T, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	var cloned T
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil, err
	}

	return &cloned, nil
}

// CalculatePercentage calculates percentage
func CalculatePercentage(part, total decimal.Decimal) *decimal.Decimal {
	if total.IsZero() {
		return nil
	}
	result := part.Div(total).Mul(decimal.NewFromInt(100))
	return &result
}

// ClampDecimal clamps a decimal value between min and max
func ClampDecimal(value, min, max decimal.Decimal) decimal.Decimal {
	if value.LessThan(min) {
		return min
	}
	if value.GreaterThan(max) {
		return max
	}
	return value
}

// RoundDecimal rounds decimal to specified precision
func RoundDecimal(value decimal.Decimal, precision int32) decimal.Decimal {
	return value.Round(precision)
}

// RetryConfig for retry operation
type RetryConfig struct {
	MaxRetries   uint
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

// DefaultRetryConfig returns default retry config
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
	}
}

// Retry executes operation with exponential backoff
func Retry[T any](operation func() (T, error), config RetryConfig) (T, error) {
	var lastErr error
	delay := config.InitialDelay

	for attempt := uint(0); attempt <= config.MaxRetries; attempt++ {
		result, err := operation()
		if err == nil {
			return result, nil
		}

		lastErr = err

		if attempt == config.MaxRetries {
			break
		}

		time.Sleep(delay)
		delay = time.Duration(float64(delay) * config.Multiplier)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}
	}

	var zero T
	return zero, lastErr
}

// Debouncer for rate limiting
type Debouncer struct {
	delay time.Duration
	lastCall time.Time
}

// NewDebouncer creates a new debouncer
func NewDebouncer(delay time.Duration) *Debouncer {
	return &Debouncer{
		delay: delay,
	}
}

// ShouldAllow checks if operation should be allowed
func (d *Debouncer) ShouldAllow() bool {
	now := time.Now()
	if now.Sub(d.lastCall) < d.delay {
		return false
	}
	d.lastCall = now
	return true
}

// Throttler for rate limiting
type Throttler struct {
	limit time.Duration
	lastExecution time.Time
}

// NewThrottler creates a new throttler
func NewThrottler(limit time.Duration) *Throttler {
	return &Throttler{
		limit: limit,
	}
}

// ShouldAllow checks if operation should be allowed
func (t *Throttler) ShouldAllow() bool {
	now := time.Now()
	if now.Sub(t.lastExecution) < t.limit {
		return false
	}
	t.lastExecution = now
	return true
}
