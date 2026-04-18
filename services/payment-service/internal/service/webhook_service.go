package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/platform/services/payment-service/internal/client"
	"github.com/platform/services/payment-service/internal/domain"
	"github.com/platform/services/payment-service/internal/repository"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// WebhookService handles webhook processing
type WebhookService struct {
	paymentRepo     repository.PaymentRepository
	withdrawalRepo  repository.WithdrawalRepository
	idempotencyRepo repository.IdempotencyRepository
	auditLogRepo    repository.AuditLogRepository
	nowpayments     *client.NOWPaymentsClient
	wallet          *client.WalletClient
	logger          *zap.Logger
}

// NewWebhookService creates a new webhook service
func NewWebhookService(
	paymentRepo repository.PaymentRepository,
	withdrawalRepo repository.WithdrawalRepository,
	idempotencyRepo repository.IdempotencyRepository,
	auditLogRepo repository.AuditLogRepository,
	nowpayments *client.NOWPaymentsClient,
	wallet *client.WalletClient,
	logger *zap.Logger,
) *WebhookService {
	return &WebhookService{
		paymentRepo:     paymentRepo,
		withdrawalRepo:  withdrawalRepo,
		idempotencyRepo: idempotencyRepo,
		auditLogRepo:    auditLogRepo,
		nowpayments:     nowpayments,
		wallet:          wallet,
		logger:          logger,
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
		s.logger.Warn("Invalid webhook signature")
		return nil, domain.NewDetailedError(domain.ErrWebhookSignatureInvalid, domain.ErrCodeWebhookSignatureInvalid)
	}

	// Parse payload
	var payload client.WebhookPayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		return nil, fmt.Errorf("parse webhook payload: %w", err)
	}

	s.logger.Info("Processing deposit webhook",
		zap.String("payment_id", payload.PaymentID),
		zap.String("status", payload.PaymentStatus),
	)

	// Check idempotency
	idempotencyKey := "webhook:deposit:" + payload.PaymentID
	if processed, _ := s.idempotencyRepo.Get(ctx, idempotencyKey); processed != nil {
		s.logger.Info("Webhook already processed", zap.String("payment_id", payload.PaymentID))
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
				s.logger.Error("Failed to update payment status", zap.Error(err))
			}
		}
	}

	// Mark as processed
	s.idempotencyRepo.Set(ctx, idempotencyKey, []byte("processed"), 86400)

	// Log audit
	s.logAudit(ctx, payment.UserID, "deposit", payment.ID, payment.PaymentID, string(payment.Status), string(newStatus), payload.OutcomeAmount)

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
		s.logger.Warn("Invalid webhook signature")
		return nil, domain.NewDetailedError(domain.ErrWebhookSignatureInvalid, domain.ErrCodeWebhookSignatureInvalid)
	}

	// Parse payload
	var payload client.WebhookPayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		return nil, fmt.Errorf("parse webhook payload: %w", err)
	}

	s.logger.Info("Processing withdrawal webhook",
		zap.String("payment_id", payload.PaymentID),
		zap.String("status", payload.PaymentStatus),
	)

	// Check idempotency
	idempotencyKey := "webhook:withdrawal:" + payload.PaymentID
	if processed, _ := s.idempotencyRepo.Get(ctx, idempotencyKey); processed != nil {
		s.logger.Info("Webhook already processed", zap.String("withdrawal_id", payload.PaymentID))
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
		s.logger.Error("Failed to credit wallet", zap.Error(err), zap.String("payment_id", payment.PaymentID))
		return fmt.Errorf("credit wallet: %w", err)
	}

	// Update payment status
	if err := s.paymentRepo.UpdateStatus(ctx, payment.ID, payment.Status, domain.PaymentStatusFinished); err != nil {
		s.logger.Error("Failed to update payment status", zap.Error(err))
		return err
	}

	// Update actual amount if different
	if !payload.OutcomeAmount.IsZero() && !payload.OutcomeAmount.Equal(payment.RequestedAmount) {
		s.paymentRepo.UpdateActualAmount(ctx, payment.ID, payload.OutcomeAmount)
	}

	s.logger.Info("Deposit completed",
		zap.Int64("user_id", payment.UserID),
		zap.String("payment_id", payment.PaymentID),
		zap.String("amount", payment.FiatAmount.String()),
	)

	return nil
}

// handleDepositFailed handles a failed deposit
func (s *WebhookService) handleDepositFailed(ctx context.Context, payment *domain.Payment, status domain.PaymentStatus) error {
	if err := s.paymentRepo.UpdateStatus(ctx, payment.ID, payment.Status, status); err != nil {
		return err
	}

	s.logger.Info("Deposit failed",
		zap.Int64("user_id", payment.UserID),
		zap.String("payment_id", payment.PaymentID),
		zap.String("status", string(status)),
	)

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
		s.logger.Error("Failed to finalize debit", zap.Error(err), zap.String("withdrawal_id", withdrawal.WithdrawalID))
		return fmt.Errorf("finalize debit: %w", err)
	}

	// Update withdrawal status
	if err := s.withdrawalRepo.UpdateStatus(ctx, withdrawal.ID, withdrawal.Status, domain.WithdrawalStatusFinished); err != nil {
		return err
	}

	s.logger.Info("Withdrawal completed",
		zap.Int64("user_id", withdrawal.UserID),
		zap.String("withdrawal_id", withdrawal.WithdrawalID),
		zap.String("amount", withdrawal.FiatAmount.String()),
	)

	return nil
}

// handleWithdrawalFailed handles a failed withdrawal
func (s *WebhookService) handleWithdrawalFailed(ctx context.Context, withdrawal *domain.Withdrawal) error {
	// Unlock funds
	if err := s.wallet.UnlockFunds(ctx, withdrawal.WithdrawalID, "withdrawal_unlock:"+withdrawal.WithdrawalID); err != nil {
		s.logger.Error("Failed to unlock funds", zap.Error(err), zap.String("withdrawal_id", withdrawal.WithdrawalID))
	}

	// Update withdrawal status
	if err := s.withdrawalRepo.UpdateStatus(ctx, withdrawal.ID, withdrawal.Status, domain.WithdrawalStatusFailed); err != nil {
		return err
	}

	s.logger.Info("Withdrawal failed, funds unlocked",
		zap.Int64("user_id", withdrawal.UserID),
		zap.String("withdrawal_id", withdrawal.WithdrawalID),
	)

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
	log := &repository.AuditLog{
		UserID:         userID,
		OperationType:  opType,
		OperationID:    &opID,
		ReferenceType:  opType,
		ReferenceID:    refID,
		PreviousStatus: prevStatus,
		NewStatus:      newStatus,
		Amount:         amount,
	}
	if err := s.auditLogRepo.Create(ctx, log); err != nil {
		s.logger.Error("Failed to create audit log", zap.Error(err))
	}
}
