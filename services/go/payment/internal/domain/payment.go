package domain

import (
	"time"
)

type TransactionStatus string

const (
	TransactionStatusPending    TransactionStatus = "pending"
	TransactionStatusProcessing TransactionStatus = "processing"
	TransactionStatusCompleted  TransactionStatus = "completed"
	TransactionStatusFailed     TransactionStatus = "failed"
	TransactionStatusCancelled  TransactionStatus = "cancelled"
	TransactionStatusRefunded   TransactionStatus = "refunded"
)

type PaymentMethodType string

const (
	PaymentMethodCreditCard    PaymentMethodType = "credit_card"
	PaymentMethodDebitCard     PaymentMethodType = "debit_card"
	PaymentMethodBankTransfer  PaymentMethodType = "bank_transfer"
	PaymentMethodEWallet       PaymentMethodType = "e_wallet"
	PaymentMethodCrypto        PaymentMethodType = "crypto"
	PaymentMethodPIX           PaymentMethodType = "pix"
	PaymentMethodUPI           PaymentMethodType = "upi"
)

type Deposit struct {
	ID                    string            `json:"id" db:"id"`
	UserID                int64             `json:"user_id" db:"user_id"`
	Amount                string            `json:"amount" db:"amount"`
	Fee                   string            `json:"fee" db:"fee"`
	NetAmount             string            `json:"net_amount" db:"net_amount"`
	Currency              string            `json:"currency" db:"currency"`
	PaymentMethodID       string            `json:"payment_method_id" db:"payment_method_id"`
	PaymentMethodType     PaymentMethodType `json:"payment_method_type" db:"payment_method_type"`
	PaymentProvider       string            `json:"payment_provider" db:"payment_provider"`
	ProviderTransactionID *string           `json:"provider_transaction_id,omitempty" db:"provider_transaction_id"`
	Status                TransactionStatus `json:"status" db:"status"`
	IdempotencyKey        string            `json:"idempotency_key" db:"idempotency_key"`
	CreatedAt             time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time         `json:"updated_at" db:"updated_at"`
	CompletedAt           *time.Time        `json:"completed_at,omitempty" db:"completed_at"`
	FailureReason         *string           `json:"failure_reason,omitempty" db:"failure_reason"`
	Metadata              string            `json:"metadata,omitempty" db:"metadata"`
}

type Withdrawal struct {
	ID                    string            `json:"id" db:"id"`
	UserID                int64             `json:"user_id" db:"user_id"`
	Amount                string            `json:"amount" db:"amount"`
	Fee                   string            `json:"fee" db:"fee"`
	NetAmount             string            `json:"net_amount" db:"net_amount"`
	Currency              string            `json:"currency" db:"currency"`
	PaymentMethodID       string            `json:"payment_method_id" db:"payment_method_id"`
	PaymentMethodType     PaymentMethodType `json:"payment_method_type" db:"payment_method_type"`
	PaymentProvider       string            `json:"payment_provider" db:"payment_provider"`
	ProviderTransactionID *string           `json:"provider_transaction_id,omitempty" db:"provider_transaction_id"`
	Status                TransactionStatus `json:"status" db:"status"`
	IdempotencyKey        string            `json:"idempotency_key" db:"idempotency_key"`
	CreatedAt             time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time         `json:"updated_at" db:"updated_at"`
	CompletedAt           *time.Time        `json:"completed_at,omitempty" db:"completed_at"`
	ApprovedAt            *time.Time        `json:"approved_at,omitempty" db:"approved_at"`
	CancelledAt           *time.Time        `json:"cancelled_at,omitempty" db:"cancelled_at"`
	FailureReason         *string           `json:"failure_reason,omitempty" db:"failure_reason"`
	Metadata              string            `json:"metadata,omitempty" db:"metadata"`
}

type CreateDepositRequest struct {
	UserID            int64  `json:"user_id"`
	Amount            string `json:"amount" validate:"required"`
	Currency          string `json:"currency" validate:"required,len=3"`
	PaymentMethodID   string `json:"payment_method_id" validate:"required"`
	PaymentProvider   string `json:"payment_provider" validate:"required"`
	IdempotencyKey    string `json:"idempotency_key" validate:"required,uuid"`
}

type RequestWithdrawalRequest struct {
	UserID          int64  `json:"user_id"`
	Amount          string `json:"amount" validate:"required"`
	Currency        string `json:"currency" validate:"required,len=3"`
	PaymentMethodID string `json:"payment_method_id" validate:"required"`
	PaymentProvider string `json:"payment_provider" validate:"required"`
	IdempotencyKey  string `json:"idempotency_key" validate:"required,uuid"`
}

type PaymentMethodInfo struct {
	ID                   string            `json:"id"`
	Type                 PaymentMethodType `json:"type"`
	Provider             string            `json:"provider"`
	DisplayName          string            `json:"display_name"`
	SupportedCurrencies  []string          `json:"supported_currencies"`
	MinAmount            string            `json:"min_amount"`
	MaxAmount            string            `json:"max_amount"`
	ProcessingTime       string            `json:"processing_time"`
}

type PaymentMethod struct {
	ID            string            `json:"id" db:"id"`
	UserID        int64             `json:"user_id" db:"user_id"`
	Type          PaymentMethodType `json:"type" db:"type"`
	Provider      string            `json:"provider" db:"provider"`
	Nickname      string            `json:"nickname" db:"nickname"`
	DisplayValue  string            `json:"display_value" db:"display_value"`
	IsDefault     bool              `json:"is_default" db:"is_default"`
	IsActive      bool              `json:"is_active" db:"is_active"`
	CreatedAt     time.Time         `json:"created_at" db:"created_at"`
	LastUsedAt    *time.Time        `json:"last_used_at,omitempty" db:"last_used_at"`
}

type WebhookEvent struct {
	Provider    string `json:"provider"`
	EventType   string `json:"event_type"`
	TransactionID string `json:"transaction_id"`
	Status      string `json:"status"`
	Amount      string `json:"amount"`
	Currency    string `json:"currency"`
	Signature   string `json:"signature"`
}
