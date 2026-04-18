package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Payment represents a crypto deposit payment
type Payment struct {
	ID             int64            `gorm:"primaryKey;autoIncrement"`
	UUID           uuid.UUID        `gorm:"type:uuid;uniqueIndex;not null"`
	UserID         int64            `gorm:"not null;index"`
	PaymentID      string           `gorm:"uniqueIndex;size:100;not null"` // NOWPayments payment_id
	IdempotencyKey string           `gorm:"uniqueIndex;size:255;not null"`

	// Amounts
	RequestedAmount decimal.Decimal `gorm:"type:numeric(18,8);not null"`
	ActualAmount    *decimal.Decimal `gorm:"type:numeric(18,8)"`
	FiatAmount      decimal.Decimal `gorm:"type:numeric(18,2);not null"`
	FiatCurrency    string          `gorm:"size:3;not null;default:'USD'"`

	// Crypto details
	CryptoCurrency string           `gorm:"size:20;not null"`
	PayAddress     string           `gorm:"size:255;not null"`
	PayAmount      *decimal.Decimal `gorm:"type:numeric(18,8)"`

	// Status
	Status PaymentStatus `gorm:"type:payment_status;not null;default:'pending'"`

	// Timestamps
	CreatedAt   time.Time  `gorm:"not null;default:now()"`
	UpdatedAt   time.Time  `gorm:"not null;default:now()"`
	CompletedAt *time.Time `gorm:""`
	ExpiresAt   *time.Time `gorm:""`

	// Metadata
	IPAddress string `gorm:"type:inet"`
	UserAgent string `gorm:"size:500"`
}

// TableName specifies the table name for Payment
func (Payment) TableName() string {
	return "payments"
}

// PaymentStatus represents the status of a payment
type PaymentStatus string

const (
	PaymentStatusPending      PaymentStatus = "pending"
	PaymentStatusWaiting      PaymentStatus = "waiting"
	PaymentStatusConfirming   PaymentStatus = "confirming"
	PaymentStatusConfirmed    PaymentStatus = "confirmed"
	PaymentStatusSending      PaymentStatus = "sending"
	PaymentStatusPartiallyPaid PaymentStatus = "partially_paid"
	PaymentStatusFinished     PaymentStatus = "finished"
	PaymentStatusFailed       PaymentStatus = "failed"
	PaymentStatusExpired      PaymentStatus = "expired"
	PaymentStatusRefunded     PaymentStatus = "refunded"
)

// IsFinal returns true if the status is final (no further transitions)
func (s PaymentStatus) IsFinal() bool {
	return s == PaymentStatusFinished ||
		s == PaymentStatusFailed ||
		s == PaymentStatusExpired ||
		s == PaymentStatusRefunded
}

// IsSuccess returns true if the payment was successful
func (s PaymentStatus) IsSuccess() bool {
	return s == PaymentStatusFinished
}

// validTransitions defines allowed status transitions
var paymentTransitions = map[PaymentStatus][]PaymentStatus{
	PaymentStatusPending:       {PaymentStatusWaiting, PaymentStatusFailed, PaymentStatusExpired},
	PaymentStatusWaiting:       {PaymentStatusConfirming, PaymentStatusPartiallyPaid, PaymentStatusFailed, PaymentStatusExpired},
	PaymentStatusConfirming:    {PaymentStatusConfirmed, PaymentStatusPartiallyPaid, PaymentStatusFailed},
	PaymentStatusConfirmed:     {PaymentStatusSending, PaymentStatusFinished},
	PaymentStatusSending:       {PaymentStatusFinished, PaymentStatusFailed},
	PaymentStatusPartiallyPaid: {PaymentStatusFinished, PaymentStatusFailed},
}

// CanTransitionTo checks if transition to target status is allowed
func (s PaymentStatus) CanTransitionTo(target PaymentStatus) bool {
	allowed, ok := paymentTransitions[s]
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

// BeforeCreate sets UUID before creating a new payment
func (p *Payment) BeforeCreate() error {
	if p.UUID == uuid.Nil {
		p.UUID = uuid.New()
	}
	return nil
}
