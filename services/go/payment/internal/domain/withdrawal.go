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
	WithdrawalID   string // NOWPayments withdrawal_id (empty until a payout exists)
	IdempotencyKey string

	// Amounts
	Amount       decimal.Decimal
	FiatAmount   decimal.Decimal
	FiatCurrency string

	// Crypto details
	CryptoCurrency string
	Address        string // Destination address

	// Status
	Status WithdrawalStatus

	// Manual-review columns (migrations 022/030). LockID ties the row to the
	// wallet lock taken at request time; DecidedBy/DecidedAt record the
	// operator decision ("auto" for the automated path); DecisionReason is
	// the mandatory rejection rationale; PayoutQueuedAt is the exactly-once
	// marker set when the dispatcher starts processing a decision.
	LockID         string
	DecidedBy      string
	DecidedAt      *time.Time
	DecisionReason string
	PayoutQueuedAt *time.Time

	// Timestamps
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt *time.Time

	// Metadata
	IPAddress string
	UserAgent string
}

// WithdrawalStatus represents the status of a withdrawal.
//
// The lifecycle splits in two:
//
//	instant:     processing → sending → sent → finished
//	review:      processing → pending_review → approved → sending → sent …
//	                                │            └→ rejected → cancelled
//	                                └→ held → pending_review | rejected
//
// pending_review/held/approved/rejected are the manual-review states written
// by the admin panel (finance operators) on the shared withdrawals table;
// the dispatcher in the payment service reacts to decided rows.
type WithdrawalStatus string

const (
	WithdrawalStatusProcessing    WithdrawalStatus = "processing"
	WithdrawalStatusPendingReview WithdrawalStatus = "pending_review"
	WithdrawalStatusHeld          WithdrawalStatus = "held"
	WithdrawalStatusApproved      WithdrawalStatus = "approved"
	WithdrawalStatusRejected      WithdrawalStatus = "rejected"
	WithdrawalStatusSending       WithdrawalStatus = "sending"
	WithdrawalStatusSent          WithdrawalStatus = "sent"
	WithdrawalStatusFinished      WithdrawalStatus = "finished"
	WithdrawalStatusFailed        WithdrawalStatus = "failed"
	WithdrawalStatusCancelled     WithdrawalStatus = "cancelled"
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

// validWithdrawalTransitions defines allowed status transitions
var withdrawalTransitions = map[WithdrawalStatus][]WithdrawalStatus{
	WithdrawalStatusProcessing: {WithdrawalStatusSending, WithdrawalStatusFailed, WithdrawalStatusCancelled, WithdrawalStatusPendingReview},
	// Review queue: the operator (or the automated approval path) decides.
	WithdrawalStatusPendingReview: {WithdrawalStatusApproved, WithdrawalStatusRejected, WithdrawalStatusHeld, WithdrawalStatusCancelled},
	WithdrawalStatusHeld:          {WithdrawalStatusPendingReview, WithdrawalStatusRejected, WithdrawalStatusCancelled},
	// Approved rows are picked up by the review dispatcher, which creates the
	// NOWPayments payout and moves the row to sending.
	WithdrawalStatusApproved: {WithdrawalStatusSending, WithdrawalStatusFailed},
	// Rejected rows end in cancelled once the dispatcher returns the locked
	// funds to the player's wallet.
	WithdrawalStatusRejected: {WithdrawalStatusCancelled},
	WithdrawalStatusSending:  {WithdrawalStatusSent, WithdrawalStatusFailed},
	WithdrawalStatusSent:     {WithdrawalStatusFinished, WithdrawalStatusFailed},
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
