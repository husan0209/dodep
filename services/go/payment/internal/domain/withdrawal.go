package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Withdrawal represents a crypto withdrawal request
type Withdrawal struct {
	ID             int64
	UUID           uuid.UUID
	UserID         int64
	WithdrawalID   string // NOWPayments withdrawal_id; empty until PSP payout executes
	IdempotencyKey string

	// Amounts
	Amount       decimal.Decimal
	FiatAmount   decimal.Decimal
	FiatCurrency string

	// Crypto details
	CryptoCurrency string
	Address        string // Destination address

	// Wallet hold created at request time; released on reject/cancel/failure.
	LockID string

	// Manual-review decision (set by Approve/Reject; empty for auto flow).
	DecidedBy      string
	DecidedAt      *time.Time
	DecisionReason string

	// Status
	Status WithdrawalStatus

	// Timestamps
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt *time.Time

	// Metadata
	IPAddress string
	UserAgent string
}

// WithdrawalStatus represents the status of a withdrawal
type WithdrawalStatus string

const (
	// WithdrawalStatusPendingReview awaits a finance-operator decision.
	// Funds are locked in the wallet; no PSP payout exists yet.
	WithdrawalStatusPendingReview WithdrawalStatus = "pending_review"
	// WithdrawalStatusApproved passed manual review; PSP payout is next.
	WithdrawalStatusApproved WithdrawalStatus = "approved"
	// WithdrawalStatusRejected was declined by review; funds were released.
	WithdrawalStatusRejected   WithdrawalStatus = "rejected"
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
		s == WithdrawalStatusCancelled ||
		s == WithdrawalStatusRejected
}

// IsSuccess returns true if the withdrawal was successful
func (s WithdrawalStatus) IsSuccess() bool {
	return s == WithdrawalStatusFinished
}

// validWithdrawalTransitions defines allowed status transitions.
// Review flow (risk-based): pending_review -> approved -> processing ...
//
//	\-> rejected | cancelled (terminal).
//
// PSP flow is unchanged: processing -> sending -> sent -> finished.
var withdrawalTransitions = map[WithdrawalStatus][]WithdrawalStatus{
	WithdrawalStatusPendingReview: {WithdrawalStatusApproved, WithdrawalStatusRejected, WithdrawalStatusCancelled},
	WithdrawalStatusApproved:      {WithdrawalStatusProcessing, WithdrawalStatusFailed, WithdrawalStatusCancelled},
	WithdrawalStatusProcessing:    {WithdrawalStatusSending, WithdrawalStatusFailed, WithdrawalStatusCancelled},
	WithdrawalStatusSending:       {WithdrawalStatusSent, WithdrawalStatusFailed},
	WithdrawalStatusSent:          {WithdrawalStatusFinished, WithdrawalStatusFailed},
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

// NewWithdrawal creates a new withdrawal with defaults
func NewWithdrawal() *Withdrawal {
	return &Withdrawal{
		UUID:         uuid.New(),
		Status:       WithdrawalStatusProcessing,
		FiatCurrency: "USD",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}
