package repository

import (
	"context"
	"time"

	"github.com/opus-casino/payment/internal/domain"
	"github.com/shopspring/decimal"
)

// ListFilter holds pagination and filtering options
type ListFilter struct {
	Limit   int    `json:"limit"`
	Cursor  string `json:"cursor"`
	Status  string `json:"status,omitempty"`
	OrderBy string `json:"order_by,omitempty"`
}

// ListResult holds paginated results
type ListResult[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor"`
	HasMore    bool   `json:"has_more"`
}

// PaymentRepository manages payment records in PostgreSQL
type PaymentRepository interface {
	// Create creates a new payment record
	Create(ctx context.Context, payment *domain.Payment) error

	// GetByID retrieves a payment by internal ID
	GetByID(ctx context.Context, id int64) (*domain.Payment, error)

	// GetByPaymentID retrieves a payment by NOWPayments payment_id
	GetByPaymentID(ctx context.Context, paymentID string) (*domain.Payment, error)

	// GetByIDempotencyKey retrieves a payment by idempotency key
	GetByIDempotencyKey(ctx context.Context, key string) (*domain.Payment, error)

	// GetByUUID retrieves a payment by UUID
	GetByUUID(ctx context.Context, uuid string) (*domain.Payment, error)

	// UpdateStatus updates payment status with optimistic locking
	UpdateStatus(ctx context.Context, id int64, fromStatus, toStatus domain.PaymentStatus) error

	// UpdateActualAmount updates the actual received amount
	UpdateActualAmount(ctx context.Context, id int64, actualAmount decimal.Decimal) error

	// ListByUserID lists payments for a user with pagination
	ListByUserID(ctx context.Context, userID int64, filter ListFilter) (*ListResult[domain.Payment], error)

	// CountByUserIDStatus counts payments by user and status
	CountByUserIDStatus(ctx context.Context, userID int64, statuses []domain.PaymentStatus) (int64, error)
}

// WithdrawalRepository manages withdrawal records
type WithdrawalRepository interface {
	// Create creates a new withdrawal record
	Create(ctx context.Context, withdrawal *domain.Withdrawal) error

	// GetByID retrieves a withdrawal by internal ID
	GetByID(ctx context.Context, id int64) (*domain.Withdrawal, error)

	// GetByWithdrawalID retrieves a withdrawal by NOWPayments withdrawal_id
	GetByWithdrawalID(ctx context.Context, withdrawalID string) (*domain.Withdrawal, error)

	// GetByIDempotencyKey retrieves a withdrawal by idempotency key
	GetByIDempotencyKey(ctx context.Context, key string) (*domain.Withdrawal, error)

	// GetByUUID retrieves a withdrawal by UUID
	GetByUUID(ctx context.Context, uuid string) (*domain.Withdrawal, error)

	// UpdateStatus updates withdrawal status with optimistic locking
	UpdateStatus(ctx context.Context, id int64, fromStatus, toStatus domain.WithdrawalStatus) error

	// ListByUserID lists withdrawals for a user with pagination
	ListByUserID(ctx context.Context, userID int64, filter ListFilter) (*ListResult[domain.Withdrawal], error)

	// CountByUserIDStatus counts withdrawals by user and status
	CountByUserIDStatus(ctx context.Context, userID int64, statuses []domain.WithdrawalStatus) (int64, error)
}

// AuditLogRepository manages audit log records
type AuditLogRepository interface {
	// Create creates a new audit log entry
	Create(ctx context.Context, log *AuditLog) error

	// ListByUserID retrieves audit logs for a user
	ListByUserID(ctx context.Context, userID int64, filter ListFilter) (*ListResult[AuditLog], error)

	// ListByReference retrieves audit logs by reference
	ListByReference(ctx context.Context, refType, refID string, filter ListFilter) (*ListResult[AuditLog], error)
}

// AuditLog represents an audit log entry
type AuditLog struct {
	ID              int64         `gorm:"primaryKey;autoIncrement"`
	UserID          int64         `gorm:"not null;index"`
	OperationType   string        `gorm:"size:50;not null;index"`
	OperationID     *int64        `gorm:""`
	ReferenceType   string        `gorm:"size:50"`
	ReferenceID     string        `gorm:"size:100;index"`
	PreviousStatus  string        `gorm:"size:50"`
	NewStatus       string        `gorm:"size:50"`
	Amount          *decimal.Decimal `gorm:"type:numeric(18,8)"`
	Currency        string        `gorm:"size:20"`
	RequestDetails  map[string]interface{} `gorm:"type:jsonb"`
	ResponseDetails map[string]interface{} `gorm:"type:jsonb"`
	ErrorCode       string        `gorm:"size:50;index"`
	ErrorMessage    string        `gorm:"type:text"`
	TraceID         string        `gorm:"size:50;index"`
	CorrelationID   string        `gorm:"size:50"`
	CreatedAt       time.Time     `gorm:"not null;default:now();index"`
}

// TableName specifies the table name for AuditLog
func (AuditLog) TableName() string {
	return "payment_audit_logs"
}

// IdempotencyRepository manages idempotency keys in DragonflyDB
type IdempotencyRepository interface {
	// Get retrieves a cached response by key
	Get(ctx context.Context, key string) ([]byte, bool, error)

	// Set stores a response with TTL
	Set(ctx context.Context, key string, value []byte, ttlSeconds int) error

	// SetNX stores only if key doesn't exist (returns true if set)
	SetNX(ctx context.Context, key string, value []byte, ttlSeconds int) (bool, error)

	// Delete removes a key
	Delete(ctx context.Context, key string) error
}

// ExchangeRateRepository caches exchange rates
type ExchangeRateRepository interface {
	// Get retrieves a cached exchange rate
	Get(ctx context.Context, fromCurrency, toCurrency string) (*decimal.Decimal, error)

	// Set stores an exchange rate with TTL
	Set(ctx context.Context, fromCurrency, toCurrency string, rate decimal.Decimal, ttlSeconds int) error

	// Delete removes a cached rate
	Delete(ctx context.Context, fromCurrency, toCurrency string) error
}

// DailyLimitsRepository tracks daily cumulative amounts
type DailyLimitsRepository interface {
	// Increment adds amount to daily total and returns new total
	Increment(ctx context.Context, userID int64, operationType string, amount decimal.Decimal) (decimal.Decimal, error)

	// Get retrieves current daily total
	Get(ctx context.Context, userID int64, operationType string) (decimal.Decimal, error)

	// Reset clears daily totals for a user (for testing)
	Reset(ctx context.Context, userID int64, operationType string) error
}

// SupportedCurrenciesRepository caches supported currencies
type SupportedCurrenciesRepository interface {
	// Get retrieves cached supported currencies
	Get(ctx context.Context) ([]string, error)

	// Set stores supported currencies with TTL
	Set(ctx context.Context, currencies []string, ttlSeconds int) error
}
