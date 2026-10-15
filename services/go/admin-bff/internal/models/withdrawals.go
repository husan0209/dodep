package models

import (
	"time"
)

// Withdrawal mirrors the shared `withdrawals` table owned by the payment
// service (migration 014) and extended with the manual-review columns
// (migration 022): status is TEXT with values
// pending_review | approved | rejected | processing | sending | sent |
// finished | failed | cancelled.
//
// The admin panel does NOT own this table's lifecycle: withdrawals arrive via
// the payment service (funds locked in the wallet at request time) and finance
// operators decide the review queue — approve moves the row toward payout,
// reject returns the funds. Both services share the same database, so the
// decision columns are written here and read by payment.
type Withdrawal struct {
	ID             int64  `gorm:"column:id;primary_key" json:"id"`
	UUID           string `gorm:"column:uuid;type:uuid;default:gen_random_uuid()" json:"uuid"`
	UserID         int64  `gorm:"column:user_id;not null;index" json:"user_id"`
	WithdrawalID   string `gorm:"column:withdrawal_id" json:"withdrawal_id,omitempty"`
	IdempotencyKey string `gorm:"column:idempotency_key;not null" json:"idempotency_key"`
	Amount         string `gorm:"column:amount;type:numeric(18,8);not null" json:"amount"`
	FiatAmount     string `gorm:"column:fiat_amount;type:numeric(18,2);not null" json:"fiat_amount"`
	FiatCurrency   string `gorm:"column:fiat_currency;type:char(3);not null;default:'USD'" json:"fiat_currency"`
	CryptoCurrency string `gorm:"column:crypto_currency;type:varchar(20);not null" json:"crypto_currency"`
	Address        string `gorm:"column:address;type:varchar(255);not null" json:"address"`
	Status         string `gorm:"column:status;not null;default:'processing'" json:"status"`

	// Review decision columns (migration 022). decided_by holds the admin id
	// (or "auto" for the automated approval path), decided_at the decision
	// timestamp and decision_reason the mandatory rejection rationale.
	LockID         string     `gorm:"column:lock_id" json:"lock_id,omitempty"`
	DecidedBy      string     `gorm:"column:decided_by" json:"decided_by,omitempty"`
	DecidedAt      *time.Time `gorm:"column:decided_at" json:"decided_at,omitempty"`
	DecisionReason string     `gorm:"column:decision_reason" json:"decision_reason,omitempty"`

	CreatedAt   time.Time  `gorm:"column:created_at;not null;default:now()" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;not null;default:now()" json:"updated_at"`
	CompletedAt *time.Time `gorm:"column:completed_at" json:"completed_at,omitempty"`

	// ReviewedBy is the admin-panel alias for DecidedBy ("Reviewed By"
	// column in the withdrawals UI). Not persisted.
	ReviewedBy string `gorm:"-" json:"reviewed_by,omitempty"`
}

// TableName pins the model to the shared payment withdrawals table.
func (Withdrawal) TableName() string { return "withdrawals" }

// IsPendingReview reports whether the withdrawal is awaiting an operator
// decision — the only state in which approve/reject/hold are allowed.
func (w *Withdrawal) IsPendingReview() bool { return w.Status == "pending_review" }
