// Package types provides shared types for Opus Casino platform.
package types

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// UserId represents a unique user identifier (UUID v4)
type UserId struct {
	Value uuid.UUID `json:"value"`
}

// NewUserId creates a new UserId
func NewUserId() UserId {
	return UserId{Value: uuid.New()}
}

// ParseUserId parses a string into UserId
func ParseUserId(s string) (UserId, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return UserId{}, err
	}
	return UserId{Value: id}, nil
}

// String returns the string representation
func (id UserId) String() string {
	return id.Value.String()
}

// MarshalJSON implements json.Marshaler
func (id UserId) MarshalJSON() ([]byte, error) {
	return json.Marshal(id.Value.String())
}

// UnmarshalJSON implements json.Unmarshaler
func (id *UserId) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := uuid.Parse(s)
	if err != nil {
		return err
	}
	id.Value = parsed
	return nil
}

// BetId represents a unique bet identifier (UUID v4)
type BetId struct {
	Value uuid.UUID `json:"value"`
}

// NewBetId creates a new BetId
func NewBetId() BetId {
	return BetId{Value: uuid.New()}
}

// ParseBetId parses a string into BetId
func ParseBetId(s string) (BetId, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return BetId{}, err
	}
	return BetId{Value: id}, nil
}

// String returns the string representation
func (id BetId) String() string {
	return id.Value.String()
}

// TransactionId represents a unique transaction identifier (UUID v4)
type TransactionId struct {
	Value uuid.UUID `json:"value"`
}

// NewTransactionId creates a new TransactionId
func NewTransactionId() TransactionId {
	return TransactionId{Value: uuid.New()}
}

// ParseTransactionId parses a string into TransactionId
func ParseTransactionId(s string) (TransactionId, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return TransactionId{}, err
	}
	return TransactionId{Value: id}, nil
}

// String returns the string representation
func (id TransactionId) String() string {
	return id.Value.String()
}

// GameId represents a unique game identifier (UUID v4)
type GameId struct {
	Value uuid.UUID `json:"value"`
}

// NewGameId creates a new GameId
func NewGameId() GameId {
	return GameId{Value: uuid.New()}
}

// ParseGameId parses a string into GameId
func ParseGameId(s string) (GameId, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return GameId{}, err
	}
	return GameId{Value: id}, nil
}

// String returns the string representation
func (id GameId) String() string {
	return id.Value.String()
}

// SessionId represents a unique session identifier (UUID v4)
type SessionId struct {
	Value uuid.UUID `json:"value"`
}

// NewSessionId creates a new SessionId
func NewSessionId() SessionId {
	return SessionId{Value: uuid.New()}
}

// ParseSessionId parses a string into SessionId
func ParseSessionId(s string) (SessionId, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return SessionId{}, err
	}
	return SessionId{Value: id}, nil
}

// String returns the string representation
func (id SessionId) String() string {
	return id.Value.String()
}

// Money represents a monetary amount with currency
// Amount is stored as decimal to avoid floating point precision issues
type Money struct {
	Amount   decimal.Decimal `json:"amount"`
	Currency string          `json:"currency"` // ISO 4217 currency code
}

// NewMoney creates a new Money instance
func NewMoney(amount string, currency string) (Money, error) {
	dec, err := decimal.NewFromString(amount)
	if err != nil {
		return Money{}, err
	}
	if dec.LessThan(decimal.Zero) {
		return Money{}, ErrNegativeAmount
	}
	return Money{
		Amount:   dec,
		Currency: currency,
	}, nil
}

// MustNewMoney creates a new Money instance, panics on error
func MustNewMoney(amount string, currency string) Money {
	m, err := NewMoney(amount, currency)
	if err != nil {
		panic(err)
	}
	return m
}

// Add adds two money amounts (must be same currency)
func (m Money) Add(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, ErrCurrencyMismatch
	}
	return Money{
		Amount:   m.Amount.Add(other.Amount),
		Currency: m.Currency,
	}, nil
}

// Subtract subtracts two money amounts (must be same currency)
func (m Money) Subtract(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, ErrCurrencyMismatch
	}
	return Money{
		Amount:   m.Amount.Sub(other.Amount),
		Currency: m.Currency,
	}, nil
}

// Multiply multiplies money by a scalar
func (m Money) Multiply(scalar decimal.Decimal) Money {
	return Money{
		Amount:   m.Amount.Mul(scalar).Abs(),
		Currency: m.Currency,
	}
}

// IsZero checks if amount is zero
func (m Money) IsZero() bool {
	return m.Amount.IsZero()
}

// IsPositive checks if amount is positive
func (m Money) IsPositive() bool {
	return m.Amount.GreaterThan(decimal.Zero)
}

// String returns string representation
func (m Money) String() string {
	return m.Amount.String() + " " + m.Currency
}

// PaginationParams for cursor-based pagination
type PaginationParams struct {
	PageSize   int32  `json:"page_size,omitempty"`
	Cursor     string `json:"cursor,omitempty"`
	SortBy     string `json:"sort_by,omitempty"`
	Descending bool   `json:"descending,omitempty"`
}

// DefaultPaginationParams returns default pagination params
func DefaultPaginationParams() PaginationParams {
	return PaginationParams{
		PageSize: 20,
	}
}

// PaginationResult for cursor-based pagination
type PaginationResult[T any] struct {
	Items       []T    `json:"items"`
	NextCursor  string `json:"next_cursor,omitempty"`
	PrevCursor  string `json:"prev_cursor,omitempty"`
	HasMore     bool   `json:"has_more"`
	TotalCount  *int64 `json:"total_count,omitempty"`
}

// DateRange filter
type DateRange struct {
	From *time.Time `json:"from,omitempty"`
	To   *time.Time `json:"to,omitempty"`
}

// ErrorDetails for API responses
type ErrorDetails struct {
	ErrorCode    string                 `json:"error_code"`
	ErrorMessage string                 `json:"error_message"`
	Metadata     map[string]string      `json:"metadata,omitempty"`
	FieldErrors  []FieldError           `json:"field_errors,omitempty"`
	TraceID      string                 `json:"trace_id,omitempty"`
}

// FieldError represents a validation error for a specific field
type FieldError struct {
	Field        string `json:"field"`
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
}

// ApiResponse wrapper
type ApiResponse[T any] struct {
	Data  *T           `json:"data,omitempty"`
	Error *ErrorDetails `json:"error,omitempty"`
}

// Success creates a success response
func Success[T any](data T) ApiResponse[T] {
	return ApiResponse[T]{
		Data: &data,
	}
}

// Error creates an error response
func Error[T any](err ErrorDetails) ApiResponse[T] {
	return ApiResponse[T]{
		Error: &err,
	}
}

// HealthStatus enum
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// DeviceType enum
type DeviceType string

const (
	DeviceTypeWeb       DeviceType = "web"
	DeviceTypeMobileWeb DeviceType = "mobile_web"
	DeviceTypeIOS       DeviceType = "ios"
	DeviceTypeAndroid   DeviceType = "android"
	DeviceTypeDesktop   DeviceType = "desktop"
)

// WalletType enum
type WalletType string

const (
	WalletTypeMain       WalletType = "main"
	WalletTypeBonus      WalletType = "bonus"
	WalletTypeFreeSpins  WalletType = "free_spins"
	WalletTypeCashback   WalletType = "cashback"
)

// TransactionType enum
type TransactionType string

const (
	TransactionTypeDeposit       TransactionType = "deposit"
	TransactionTypeWithdrawal    TransactionType = "withdrawal"
	TransactionTypeBetPlace      TransactionType = "bet_place"
	TransactionTypeBetWin        TransactionType = "bet_win"
	TransactionTypeBetRefund     TransactionType = "bet_refund"
	TransactionTypeBonusCredit   TransactionType = "bonus_credit"
	TransactionTypeBonusDebit    TransactionType = "bonus_debit"
	TransactionTypeTransfer      TransactionType = "transfer"
	TransactionTypeAdjustment    TransactionType = "adjustment"
)

// BetType enum
type BetType string

const (
	BetTypeSports   BetType = "sports"
	BetTypeLive     BetType = "live"
	BetTypeCasino   BetType = "casino"
	BetTypeLottery  BetType = "lottery"
	BetTypeVirtual  BetType = "virtual"
)

// BetStatus enum
type BetStatus string

const (
	BetStatusPending   BetStatus = "pending"
	BetStatusAccepted  BetStatus = "accepted"
	BetStatusSettled   BetStatus = "settled"
	BetStatusCancelled BetStatus = "cancelled"
	BetStatusRejected  BetStatus = "rejected"
)

// KycLevel enum
type KycLevel string

const (
	KycLevelNone      KycLevel = "none"
	KycLevelBasic     KycLevel = "basic"
	KycLevelIdentity  KycLevel = "identity"
	KycLevelEnhanced  KycLevel = "enhanced"
	KycLevelVip       KycLevel = "vip"
)
