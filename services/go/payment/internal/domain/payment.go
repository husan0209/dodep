package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Payment represents a crypto deposit payment
type Payment struct {
	ID             int64
	UUID           uuid.UUID
	UserID         int64
	PaymentID      string // NOWPayments payment_id
	IdempotencyKey string

	// Amounts
	RequestedAmount decimal.Decimal
	ActualAmount    *decimal.Decimal
	FiatAmount      decimal.Decimal
	FiatCurrency    string

	// Crypto details
	CryptoCurrency string
	PayAddress     string
	PayAmount      *decimal.Decimal

	// Status
	Status PaymentStatus

	// Timestamps
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt *time.Time
	ExpiresAt   *time.Time

	// Metadata
	IPAddress string
	UserAgent string
}

// PaymentStatus represents the status of a payment
type PaymentStatus string

const (
	PaymentStatusPending       PaymentStatus = "pending"
	PaymentStatusWaiting       PaymentStatus = "waiting"
	PaymentStatusConfirming    PaymentStatus = "confirming"
	PaymentStatusConfirmed     PaymentStatus = "confirmed"
	PaymentStatusSending       PaymentStatus = "sending"
	PaymentStatusPartiallyPaid PaymentStatus = "partially_paid"
	PaymentStatusFinished      PaymentStatus = "finished"
	PaymentStatusFailed        PaymentStatus = "failed"
	PaymentStatusExpired       PaymentStatus = "expired"
	PaymentStatusRefunded      PaymentStatus = "refunded"
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

// NewPayment creates a new payment with defaults
func NewPayment() *Payment {
	return &Payment{
		UUID:         uuid.New(),
		Status:       PaymentStatusPending,
		FiatCurrency: "USD",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}
