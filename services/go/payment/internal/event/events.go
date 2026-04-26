package event

import (
	"time"

	"github.com/shopspring/decimal"
)

// Event types
const (
	EventTypeDepositCompleted   = "deposit.completed"
	EventTypeDepositFailed      = "deposit.failed"
	EventTypeWithdrawalCompleted = "withdrawal.completed"
	EventTypeWithdrawalFailed   = "withdrawal.failed"
	EventTypePaymentAudit       = "payment.audit"
)

// BaseEvent contains common event fields
type BaseEvent struct {
	EventType     string    `json:"event_type"`
	EventID       string    `json:"event_id"`
	Timestamp     time.Time `json:"timestamp"`
	TraceID       string    `json:"trace_id,omitempty"`
	CorrelationID string    `json:"correlation_id,omitempty"`
}

// DepositCompletedEvent is published when a deposit completes
type DepositCompletedEvent struct {
	BaseEvent
	UserID       int64           `json:"user_id"`
	PaymentID    string          `json:"payment_id"`
	PaymentUUID  string          `json:"payment_uuid"`
	Amount       decimal.Decimal `json:"amount"`
	FiatAmount   decimal.Decimal `json:"fiat_amount"`
	Currency     string          `json:"currency"`
	TransactionID string         `json:"transaction_id"`
}

// DepositFailedEvent is published when a deposit fails
type DepositFailedEvent struct {
	BaseEvent
	UserID      int64  `json:"user_id"`
	PaymentID   string `json:"payment_id"`
	PaymentUUID string `json:"payment_uuid"`
	Reason      string `json:"reason"`
	ErrorCode   string `json:"error_code,omitempty"`
}

// WithdrawalCompletedEvent is published when a withdrawal completes
type WithdrawalCompletedEvent struct {
	BaseEvent
	UserID        int64           `json:"user_id"`
	WithdrawalID  string          `json:"withdrawal_id"`
	WithdrawalUUID string         `json:"withdrawal_uuid"`
	Amount        decimal.Decimal `json:"amount"`
	FiatAmount    decimal.Decimal `json:"fiat_amount"`
	Currency      string          `json:"currency"`
	Address       string          `json:"address"`
	TransactionID string          `json:"transaction_id"`
}

// WithdrawalFailedEvent is published when a withdrawal fails
type WithdrawalFailedEvent struct {
	BaseEvent
	UserID         int64  `json:"user_id"`
	WithdrawalID   string `json:"withdrawal_id"`
	WithdrawalUUID string `json:"withdrawal_uuid"`
	Reason         string `json:"reason"`
	ErrorCode      string `json:"error_code,omitempty"`
	FundsUnlocked  bool   `json:"funds_unlocked"`
}

// PaymentAuditEvent is published for audit logging
type PaymentAuditEvent struct {
	BaseEvent
	UserID         int64                  `json:"user_id"`
	OperationType  string                 `json:"operation_type"`
	OperationID    string                 `json:"operation_id"`
	ReferenceType  string                 `json:"reference_type"`
	ReferenceID    string                 `json:"reference_id"`
	PreviousStatus string                 `json:"previous_status,omitempty"`
	NewStatus      string                 `json:"new_status,omitempty"`
	Amount         *decimal.Decimal       `json:"amount,omitempty"`
	Currency       string                 `json:"currency,omitempty"`
	Details        map[string]interface{} `json:"details,omitempty"`
	ErrorCode      string                 `json:"error_code,omitempty"`
	ErrorMessage   string                 `json:"error_message,omitempty"`
}

// NewDepositCompletedEvent creates a new deposit completed event
func NewDepositCompletedEvent(userID int64, paymentID, paymentUUID string, amount, fiatAmount decimal.Decimal, currency, transactionID string) *DepositCompletedEvent {
	return &DepositCompletedEvent{
		BaseEvent: BaseEvent{
			EventType: EventTypeDepositCompleted,
			Timestamp: time.Now(),
		},
		UserID:        userID,
		PaymentID:     paymentID,
		PaymentUUID:   paymentUUID,
		Amount:        amount,
		FiatAmount:    fiatAmount,
		Currency:      currency,
		TransactionID: transactionID,
	}
}

// NewWithdrawalCompletedEvent creates a new withdrawal completed event
func NewWithdrawalCompletedEvent(userID int64, withdrawalID, withdrawalUUID string, amount, fiatAmount decimal.Decimal, currency, address, transactionID string) *WithdrawalCompletedEvent {
	return &WithdrawalCompletedEvent{
		BaseEvent: BaseEvent{
			EventType: EventTypeWithdrawalCompleted,
			Timestamp: time.Now(),
		},
		UserID:         userID,
		WithdrawalID:   withdrawalID,
		WithdrawalUUID: withdrawalUUID,
		Amount:         amount,
		FiatAmount:     fiatAmount,
		Currency:       currency,
		Address:        address,
		TransactionID:  transactionID,
	}
}

// NewPaymentAuditEvent creates a new audit event
func NewPaymentAuditEvent(userID int64, opType, opID, refType, refID string) *PaymentAuditEvent {
	return &PaymentAuditEvent{
		BaseEvent: BaseEvent{
			EventType: EventTypePaymentAudit,
			Timestamp: time.Now(),
		},
		UserID:        userID,
		OperationType: opType,
		OperationID:   opID,
		ReferenceType: refType,
		ReferenceID:   refID,
	}
}
