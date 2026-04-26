package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/opus-casino/payment/internal/client"
	"github.com/opus-casino/payment/internal/domain"
	"github.com/opus-casino/payment/internal/event"
	"github.com/opus-casino/payment/internal/repository"
	"github.com/rs/zerolog/log"
	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel/trace"
)

// WebhookService handles webhook processing
type WebhookService struct {
	paymentRepo     repository.PaymentRepository
	withdrawalRepo  repository.WithdrawalRepository
	idempotencyRepo repository.IdempotencyRepository
	auditLogRepo    repository.AuditLogRepository
	nowpayments     client.NOWPaymentsAPI
	wallet          client.WalletAPI
	producer        *event.Producer
	tracer          trace.Tracer
}

// NewWebhookService creates a new webhook service
func NewWebhookService(
	paymentRepo repository.PaymentRepository,
	withdrawalRepo repository.WithdrawalRepository,
	idempotencyRepo repository.IdempotencyRepository,
	auditLogRepo repository.AuditLogRepository,
	nowpayments client.NOWPaymentsAPI,
	wallet client.WalletAPI,
	producer *event.Producer,
	tracer trace.Tracer,
) *WebhookService {
	return &WebhookService{
		paymentRepo:     paymentRepo,
		withdrawalRepo:  withdrawalRepo,
		idempotencyRepo: idempotencyRepo,
		auditLogRepo:    auditLogRepo,
		nowpayments:     nowpayments,
		wallet:          wallet,
		producer:        producer,
		tracer:          tracer,
	}
}

// ProcessWebhookRequest represents a webhook processing request
type ProcessWebhookRequest struct {
	Payload   []byte
	Signature string
}

// ProcessWebhookResult represents a webhook processing result
type ProcessWebhookResult struct {
	Processed bool
	Type      string // "deposit" or "withdrawal"
	ID        string
	Status    string
}

// ProcessDepositWebhook processes a deposit webhook
func (s *WebhookService) ProcessDepositWebhook(ctx context.Context, req ProcessWebhookRequest) (*ProcessWebhookResult, error) {
	// Verify signature
	if !s.nowpayments.VerifyWebhookSignature(req.Payload, req.Signature) {
		log.Warn().Msg("Invalid webhook signature")
		return nil, domain.NewDetailedError(domain.ErrWebhookSignatureInvalid, domain.ErrCodeWebhookSignatureInvalid)
	}

	// Parse payload
	var payload client.WebhookPayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		return nil, fmt.Errorf("parse webhook payload: %w", err)
	}

	log.Info().
		Str("payment_id", payload.PaymentID).
		Str("status", payload.PaymentStatus).
		Msg("Processing deposit webhook")

	// Check idempotency
	idempotencyKey := "webhook:deposit:" + payload.PaymentID
	if processed, found, _ := s.idempotencyRepo.Get(ctx, idempotencyKey); found && processed != nil {
		log.Info().Str("payment_id", payload.PaymentID).Msg("Webhook already processed")
		return &ProcessWebhookResult{Processed: true, Type: "deposit", ID: payload.PaymentID, Status: payload.PaymentStatus}, nil
	}

	// Get payment
	payment, err := s.paymentRepo.GetByPaymentID(ctx, payload.PaymentID)
	if err != nil {
		return nil, fmt.Errorf("get payment: %w", err)
	}

	// Map NOWPayments status to domain status
	newStatus := s.mapPaymentStatus(payload.PaymentStatus)

	// Handle based on status
	switch newStatus {
	case domain.PaymentStatusFinished:
		if err := s.handleDepositFinished(ctx, payment, payload); err != nil {
			return nil, err
		}
	case domain.PaymentStatusFailed, domain.PaymentStatusExpired:
		if err := s.handleDepositFailed(ctx, payment, newStatus); err != nil {
			return nil, err
		}
	default:
		// Update status for intermediate states
		if payment.Status.CanTransitionTo(newStatus) {
			if err := s.paymentRepo.UpdateStatus(ctx, payment.ID, payment.Status, newStatus); err != nil {
				log.Error().Err(err).Msg("Failed to update payment status")
			}
		}
	}

	// Mark as processed
	s.idempotencyRepo.Set(ctx, idempotencyKey, []byte("processed"), 86400)

	// Log audit
	var outcomeAmount *decimal.Decimal
	if !payload.OutcomeAmount.IsZero() {
		amt := payload.OutcomeAmount
		outcomeAmount = &amt
	}
	s.logAudit(ctx, payment.UserID, "deposit", payment.ID, payment.PaymentID, string(payment.Status), string(newStatus), outcomeAmount)

	return &ProcessWebhookResult{
		Processed: true,
		Type:      "deposit",
		ID:        payload.PaymentID,
		Status:    string(newStatus),
	}, nil
}

// ProcessWithdrawalWebhook processes a withdrawal webhook
func (s *WebhookService) ProcessWithdrawalWebhook(ctx context.Context, req ProcessWebhookRequest) (*ProcessWebhookResult, error) {
	// Verify signature
	if !s.nowpayments.VerifyWebhookSignature(req.Payload, req.Signature) {
		log.Warn().Msg("Invalid webhook signature")
		return nil, domain.NewDetailedError(domain.ErrWebhookSignatureInvalid, domain.ErrCodeWebhookSignatureInvalid)
	}

	// Parse payload
	var payload client.WebhookPayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		return nil, fmt.Errorf("parse webhook payload: %w", err)
	}

	log.Info().
		Str("payment_id", payload.PaymentID).
		Str("status", payload.PaymentStatus).
		Msg("Processing withdrawal webhook")

	// Check idempotency
	idempotencyKey := "webhook:withdrawal:" + payload.PaymentID
	if processed, found, _ := s.idempotencyRepo.Get(ctx, idempotencyKey); found && processed != nil {
		log.Info().Str("withdrawal_id", payload.PaymentID).Msg("Webhook already processed")
		return &ProcessWebhookResult{Processed: true, Type: "withdrawal", ID: payload.PaymentID, Status: payload.PaymentStatus}, nil
	}

	// Get withdrawal
	withdrawal, err := s.withdrawalRepo.GetByWithdrawalID(ctx, payload.PaymentID)
	if err != nil {
		return nil, fmt.Errorf("get withdrawal: %w", err)
	}

	// Map NOWPayments status to domain status
	newStatus := s.mapWithdrawalStatus(payload.PaymentStatus)

	// Handle based on status
	switch newStatus {
	case domain.WithdrawalStatusFinished:
		if err := s.handleWithdrawalFinished(ctx, withdrawal); err != nil {
			return nil, err
		}
	case domain.WithdrawalStatusFailed:
		if err := s.handleWithdrawalFailed(ctx, withdrawal); err != nil {
			return nil, err
		}
	}

	// Mark as processed
	s.idempotencyRepo.Set(ctx, idempotencyKey, []byte("processed"), 86400)

	// Log audit
	s.logAudit(ctx, withdrawal.UserID, "withdrawal", withdrawal.ID, withdrawal.WithdrawalID, string(withdrawal.Status), string(newStatus), nil)

	return &ProcessWebhookResult{
		Processed: true,
		Type:      "withdrawal",
		ID:        payload.PaymentID,
		Status:    string(newStatus),
	}, nil
}

// handleDepositFinished handles a finished deposit
func (s *WebhookService) handleDepositFinished(ctx context.Context, payment *domain.Payment, payload client.WebhookPayload) error {
	// Credit wallet
	_, err := s.wallet.CreditWallet(ctx, client.CreditRequest{
		UserID:         payment.UserID,
		Currency:       "USD",
		Amount:         payment.FiatAmount,
		IdempotencyKey: "deposit:" + payment.PaymentID,
		ReferenceType:  "deposit",
		ReferenceID:    payment.PaymentID,
	})
	if err != nil {
		log.Error().Err(err).Str("payment_id", payment.PaymentID).Msg("Failed to credit wallet")
		return fmt.Errorf("credit wallet: %w", err)
	}

	// Update payment status
	if err := s.paymentRepo.UpdateStatus(ctx, payment.ID, payment.Status, domain.PaymentStatusFinished); err != nil {
		log.Error().Err(err).Msg("Failed to update payment status")
		return err
	}

	// Update actual amount if different
	if !payload.OutcomeAmount.IsZero() && !payload.OutcomeAmount.Equal(payment.RequestedAmount) {
		s.paymentRepo.UpdateActualAmount(ctx, payment.ID, payload.OutcomeAmount)
	}

	log.Info().
		Int64("user_id", payment.UserID).
		Str("payment_id", payment.PaymentID).
		Str("amount", payment.FiatAmount.String()).
		Msg("Deposit completed")

	return nil
}

// handleDepositFailed handles a failed deposit
func (s *WebhookService) handleDepositFailed(ctx context.Context, payment *domain.Payment, status domain.PaymentStatus) error {
	if err := s.paymentRepo.UpdateStatus(ctx, payment.ID, payment.Status, status); err != nil {
		return err
	}

	log.Info().
		Int64("user_id", payment.UserID).
		Str("payment_id", payment.PaymentID).
		Str("status", string(status)).
		Msg("Deposit failed")

	return nil
}

// handleWithdrawalFinished handles a finished withdrawal
func (s *WebhookService) handleWithdrawalFinished(ctx context.Context, withdrawal *domain.Withdrawal) error {
	// Finalize debit
	_, err := s.wallet.FinalizeDebit(ctx, client.FinalizeDebitRequest{
		UserID:         withdrawal.UserID,
		Currency:       "USD",
		Amount:         withdrawal.FiatAmount,
		IdempotencyKey: "withdrawal:" + withdrawal.WithdrawalID,
		ReferenceID:    withdrawal.WithdrawalID,
	})
	if err != nil {
		log.Error().Err(err).Str("withdrawal_id", withdrawal.WithdrawalID).Msg("Failed to finalize debit")
		return fmt.Errorf("finalize debit: %w", err)
	}

	// Update withdrawal status
	if err := s.withdrawalRepo.UpdateStatus(ctx, withdrawal.ID, withdrawal.Status, domain.WithdrawalStatusFinished); err != nil {
		return err
	}

	log.Info().
		Int64("user_id", withdrawal.UserID).
		Str("withdrawal_id", withdrawal.WithdrawalID).
		Str("amount", withdrawal.FiatAmount.String()).
		Msg("Withdrawal completed")

	return nil
}

// handleWithdrawalFailed handles a failed withdrawal
func (s *WebhookService) handleWithdrawalFailed(ctx context.Context, withdrawal *domain.Withdrawal) error {
	// Unlock funds
	if err := s.wallet.UnlockFunds(ctx, withdrawal.WithdrawalID, "withdrawal_unlock:"+withdrawal.WithdrawalID); err != nil {
		log.Error().Err(err).Str("withdrawal_id", withdrawal.WithdrawalID).Msg("Failed to unlock funds")
	}

	// Update withdrawal status
	if err := s.withdrawalRepo.UpdateStatus(ctx, withdrawal.ID, withdrawal.Status, domain.WithdrawalStatusFailed); err != nil {
		return err
	}

	log.Info().
		Int64("user_id", withdrawal.UserID).
		Str("withdrawal_id", withdrawal.WithdrawalID).
		Msg("Withdrawal failed, funds unlocked")

	return nil
}

// mapPaymentStatus maps NOWPayments status to domain status
func (s *WebhookService) mapPaymentStatus(status string) domain.PaymentStatus {
	statusMap := map[string]domain.PaymentStatus{
		"waiting":       domain.PaymentStatusWaiting,
		"confirming":    domain.PaymentStatusConfirming,
		"confirmed":     domain.PaymentStatusConfirmed,
		"sending":       domain.PaymentStatusSending,
		"partially_paid": domain.PaymentStatusPartiallyPaid,
		"finished":      domain.PaymentStatusFinished,
		"failed":        domain.PaymentStatusFailed,
		"expired":       domain.PaymentStatusExpired,
		"refunded":      domain.PaymentStatusRefunded,
	}
	if s, ok := statusMap[status]; ok {
		return s
	}
	return domain.PaymentStatusPending
}

// mapWithdrawalStatus maps NOWPayments status to domain status
func (s *WebhookService) mapWithdrawalStatus(status string) domain.WithdrawalStatus {
	statusMap := map[string]domain.WithdrawalStatus{
		"processing": domain.WithdrawalStatusProcessing,
		"sending":    domain.WithdrawalStatusSending,
		"sent":       domain.WithdrawalStatusSent,
		"finished":   domain.WithdrawalStatusFinished,
		"failed":     domain.WithdrawalStatusFailed,
		"cancelled":  domain.WithdrawalStatusCancelled,
	}
	if s, ok := statusMap[status]; ok {
		return s
	}
	return domain.WithdrawalStatusProcessing
}

// logAudit logs an audit entry
func (s *WebhookService) logAudit(ctx context.Context, userID int64, opType string, opID int64, refID, prevStatus, newStatus string, amount *decimal.Decimal) {
	if s.auditLogRepo == nil {
		return
	}

	auditEntry := &repository.AuditLog{
		UserID:         userID,
		OperationType:  opType,
		OperationID:    &opID,
		ReferenceType:  opType,
		ReferenceID:    refID,
		PreviousStatus: prevStatus,
		NewStatus:      newStatus,
		Amount:         amount,
	}
	if err := s.auditLogRepo.Create(ctx, auditEntry); err != nil {
		log.Error().Err(err).Msg("Failed to create audit log")
	}
}
