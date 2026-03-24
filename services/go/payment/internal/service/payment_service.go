package service

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/opus-casino/payment/internal/domain"
	"github.com/opus-casino/payment/internal/repository"
)

type PaymentService struct {
	repo *repository.PaymentRepository
	log  *zap.Logger
}

func NewPaymentService(repo *repository.PaymentRepository, log *zap.Logger) *PaymentService {
	return &PaymentService{repo: repo, log: log}
}

func (s *PaymentService) CreateDeposit(ctx context.Context, req *domain.CreateDepositRequest) (*domain.Deposit, error) {
	// Check idempotency
	exists, err := s.repo.CheckIdempotency(ctx, req.IdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("check idempotency: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("duplicate request")
	}

	deposit, err := s.repo.CreateDeposit(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create deposit: %w", err)
	}

	s.log.Info("Deposit created", zap.String("deposit_id", deposit.ID), zap.Int64("user_id", req.UserID), zap.String("amount", req.Amount))

	// In production: call PSP here to get redirect_url / QR code
	// For now: mark as processing and simulate completion
	s.repo.UpdateDepositStatus(ctx, deposit.ID, domain.TransactionStatusProcessing, nil)

	return deposit, nil
}

func (s *PaymentService) GetDeposit(ctx context.Context, userID int64, depositID string) (*domain.Deposit, error) {
	deposit, err := s.repo.GetDeposit(ctx, userID, depositID)
	if err != nil {
		return nil, fmt.Errorf("get deposit: %w", err)
	}
	if deposit == nil {
		return nil, fmt.Errorf("deposit not found")
	}
	return deposit, nil
}

func (s *PaymentService) ListDeposits(ctx context.Context, userID int64, limit, offset int) ([]*domain.Deposit, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repo.ListDeposits(ctx, userID, limit, offset)
}

func (s *PaymentService) RequestWithdrawal(ctx context.Context, req *domain.RequestWithdrawalRequest) (*domain.Withdrawal, error) {
	// Check idempotency
	exists, err := s.repo.CheckIdempotency(ctx, req.IdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("check idempotency: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("duplicate request")
	}

	// In production: check KYC level >= 2, check limits, fraud check, lock funds in wallet
	withdrawal, err := s.repo.CreateWithdrawal(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create withdrawal: %w", err)
	}

	s.log.Info("Withdrawal requested", zap.String("withdrawal_id", withdrawal.ID), zap.Int64("user_id", req.UserID), zap.String("amount", req.Amount))

	return withdrawal, nil
}

func (s *PaymentService) GetWithdrawal(ctx context.Context, userID int64, withdrawalID string) (*domain.Withdrawal, error) {
	withdrawal, err := s.repo.GetWithdrawal(ctx, userID, withdrawalID)
	if err != nil {
		return nil, fmt.Errorf("get withdrawal: %w", err)
	}
	if withdrawal == nil {
		return nil, fmt.Errorf("withdrawal not found")
	}
	return withdrawal, nil
}

func (s *PaymentService) ListWithdrawals(ctx context.Context, userID int64, limit, offset int) ([]*domain.Withdrawal, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repo.ListWithdrawals(ctx, userID, limit, offset)
}

func (s *PaymentService) CancelWithdrawal(ctx context.Context, userID int64, withdrawalID string) error {
	if err := s.repo.CancelWithdrawal(ctx, userID, withdrawalID); err != nil {
		return fmt.Errorf("cancel withdrawal: %w", err)
	}
	s.log.Info("Withdrawal cancelled", zap.String("withdrawal_id", withdrawalID), zap.Int64("user_id", userID))
	return nil
}

func (s *PaymentService) GetPaymentMethods(ctx context.Context, userID int64) ([]*domain.PaymentMethodInfo, error) {
	return s.repo.GetPaymentMethods(ctx, userID)
}

func (s *PaymentService) HandleWebhook(ctx context.Context, event *domain.WebhookEvent) error {
	s.log.Info("Webhook received", zap.String("provider", event.Provider), zap.String("event", event.EventType), zap.String("tx_id", event.TransactionID))

	// In production: verify signature, update deposit/withdrawal status, credit/debit wallet
	if err := s.repo.RecordWebhookEvent(ctx, event); err != nil {
		return fmt.Errorf("record webhook: %w", err)
	}

	return nil
}
