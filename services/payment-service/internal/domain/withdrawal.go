package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Withdrawal represents a crypto withdrawal request
type Withdrawal struct {
	ID             int64           `gorm:"primaryKey;autoIncrement"`
	UUID           uuid.UUID       `gorm:"type:uuid;uniqueIndex;not null"`
	UserID         int64           `gorm:"not null;index"`
	WithdrawalID   string          `gorm:"uniqueIndex;size:100;not null"` // NOWPayments withdrawal_id
	IdempotencyKey string          `gorm:"uniqueIndex;size:255;not null"`

	// Amounts
	Amount       decimal.Decimal `gorm:"type:numeric(18,8);not null"`
	FiatAmount   decimal.Decimal `gorm:"type:numeric(18,2);not null"`
	FiatCurrency string          `gorm:"size:3;not null;default:'USD'"`

	// Crypto details
	CryptoCurrency string `gorm:"size:20;not null"`
	Address        string `gorm:"size:255;not null"` // Destination address

	// Status
	Status WithdrawalStatus `gorm:"type:withdrawal_status;not null;default:'processing'"`

	// Timestamps
	CreatedAt   time.Time  `gorm:"not null;default:now()"`
	UpdatedAt   time.Time  `gorm:"not null;default:now()"`
	CompletedAt *time.Time `gorm:""`

	// Metadata
	IPAddress string `gorm:"type:inet"`
	UserAgent string `gorm:"size:500"`
}

// TableName specifies the table name for Withdrawal
func (Withdrawal) TableName() string {
	return "withdrawals"
}

// WithdrawalStatus represents the status of a withdrawal
type WithdrawalStatus string

const (
	WithdrawalStatusProcessing WithdrawalStatus = "processing"
	WithdrawalStatusSending    WithdrawalStatus = "sending"
	WithdrawalStatusSent       WithdrawalStatus = "sent"
	WithdrawalStatusFinished   WithdrawalStatus = "finished"
	WithdrawalStatusFailed     WithdrawalStatus = "failed"
	WithdrawalStatusCancelled  WithdrawalStatus = "cancelled"
)

// IsFinal returns true if the status is final (no further transitions)
func (s WithdrawalStatus) IsFinal() bool {
	return s == WithdrawalStatusFinished ||
		s == WithdrawalStatusFailed ||
		s == WithdrawalStatusCancelled
}

// IsSuccess returns true if the withdrawal was successful
func (s WithdrawalStatus) IsSuccess() bool {
	return s == WithdrawalStatusFinished
}

// validWithdrawalTransitions defines allowed status transitions
var withdrawalTransitions = map[WithdrawalStatus][]WithdrawalStatus{
	WithdrawalStatusProcessing: {WithdrawalStatusSending, WithdrawalStatusFailed, WithdrawalStatusCancelled},
	WithdrawalStatusSending:    {WithdrawalStatusSent, WithdrawalStatusFailed},
	WithdrawalStatusSent:       {WithdrawalStatusFinished, WithdrawalStatusFailed},
}

// CanTransitionTo checks if transition to target status is allowed
func (s WithdrawalStatus) CanTransitionTo(target WithdrawalStatus) bool {
	allowed, ok := withdrawalTransitions[s]
	if !ok {
		return false
	}
	for _, t := range allowed {
		if t == target {
			return true
		}
	}
	return false
}

// BeforeCreate sets UUID before creating a new withdrawal
func (w *Withdrawal) BeforeCreate() error {
	if w.UUID == uuid.Nil {
		w.UUID = uuid.New()
	}
	return nil
}
