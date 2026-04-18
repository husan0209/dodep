package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/platform/services/payment-service/internal/client"
	"github.com/platform/services/payment-service/internal/domain"
	"github.com/platform/services/payment-service/internal/repository"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// WithdrawalService handles withdrawal business logic
type WithdrawalService struct {
	withdrawalRepo   repository.WithdrawalRepository
	idempotencyRepo  repository.IdempotencyRepository
	exchangeRateRepo repository.ExchangeRateRepository
	dailyLimitsRepo  repository.DailyLimitsRepository
	nowpayments      *client.NOWPaymentsClient
	wallet           *client.WalletClient
	user             *client.UserClient
	logger           *zap.Logger
}

// NewWithdrawalService creates a new withdrawal service
func NewWithdrawalService(
	withdrawalRepo repository.WithdrawalRepository,
	idempotencyRepo repository.IdempotencyRepository,
	exchangeRateRepo repository.ExchangeRateRepository,
	dailyLimitsRepo repository.DailyLimitsRepository,
	nowpayments *client.NOWPaymentsClient,
	wallet *client.WalletClient,
	user *client.UserClient,
	logger *zap.Logger,
) *WithdrawalService {
	return &WithdrawalService{
		withdrawalRepo:   withdrawalRepo,
		idempotencyRepo:  idempotencyRepo,
		exchangeRateRepo: exchangeRateRepo,
		dailyLimitsRepo:  dailyLimitsRepo,
		nowpayments:      nowpayments,
		wallet:           wallet,
		user:             user,
		logger:           logger,
	}
}

// InitiateWithdrawalRequest represents a withdrawal request
type InitiateWithdrawalRequest struct {
	UserID         int64
	Amount         decimal.Decimal
	Currency       domain.CryptoCurrency
	Address        string
	IdempotencyKey string
	IPAddress      string
	UserAgent      string
}

// InitiateWithdrawalResponse represents a withdrawal response
type InitiateWithdrawalResponse struct {
	WithdrawalUUID string
	WithdrawalID   string
	Amount         decimal.Decimal
	FiatAmount     decimal.Decimal
	Currency       string
	Address        string
	Status         string
}

// InitiateWithdrawal creates a new withdrawal request
func (s *WithdrawalService) InitiateWithdrawal(ctx context.Context, req InitiateWithdrawalRequest) (*InitiateWithdrawalResponse, error) {
	// Check idempotency
	if existingWithdrawal, err := s.withdrawalRepo.GetByIDempotencyKey(ctx, req.IdempotencyKey); err != nil {
		return nil, fmt.Errorf("check idempotency: %w", err)
	} else if existingWithdrawal != nil {
		s.logger.Info("Returning existing withdrawal for idempotency key",
			zap.String("idempotency_key", req.IdempotencyKey),
			zap.String("withdrawal_id", existingWithdrawal.WithdrawalID),
		)
		return s.toResponse(existingWithdrawal), nil
	}

	// Validate KYC level (minimum level 2 for withdrawals)
	if err := s.validateKYCLevel(ctx, req.UserID); err != nil {
		return nil, err
	}

	// Validate currency
	if !req.Currency.IsWithdrawalSupported() {
		return nil, domain.ErrorCurrencyNotSupported(string(req.Currency))
	}

	// Validate withdrawal limits
	if err := s.validateWithdrawalLimits(ctx, req.UserID, req.Amount); err != nil {
		return nil, err
	}

	// Get exchange rate for fiat amount
	fiatAmount, err := s.getFiatAmount(ctx, req.Amount, req.Currency)
	if err != nil {
		return nil, fmt.Errorf("get exchange rate: %w", err)
	}

	// Check and lock funds
	lockResult, err := s.wallet.LockFunds(ctx, client.LockRequest{
		UserID:         req.UserID,
		Currency:       "USD",
		Amount:         fiatAmount,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, fmt.Errorf("lock funds: %w", err)
	}

	// Create payout in NOWPayments
	npResp, err := s.nowpayments.CreatePayout(ctx, client.CreatePayoutRequest{
		WithdrawalID:   uuid.New().String(),
		Address:        req.Address,
		Currency:       req.Currency.NOWPaymentsCurrency(),
		Amount:         req.Amount,
		IPNCallbackURL: s.getIPNCallbackURL(),
	})

	if err != nil {
		// Compensating transaction: unlock funds
		if unlockErr := s.wallet.UnlockFunds(ctx, lockResult.LockID, req.IdempotencyKey+"_unlock"); unlockErr != nil {
			s.logger.Error("Failed to unlock funds after payout failure",
				zap.Error(unlockErr),
				zap.String("lock_id", lockResult.LockID),
			)
		}
		return nil, domain.ErrorProviderUnavailable("NOWPayments", err)
	}

	// Create withdrawal record
	withdrawal := &domain.Withdrawal{
		UUID:           uuid.New(),
		UserID:         req.UserID,
		WithdrawalID:   npResp.WithdrawalID,
		IdempotencyKey: req.IdempotencyKey,
		Amount:         req.Amount,
		FiatAmount:     fiatAmount,
		FiatCurrency:   "USD",
		CryptoCurrency: string(req.Currency),
		Address:        req.Address,
		Status:         domain.WithdrawalStatusProcessing,
		IPAddress:      req.IPAddress,
		UserAgent:      req.UserAgent,
	}

	if err := s.withdrawalRepo.Create(ctx, withdrawal); err != nil {
		// Compensating transaction: unlock funds
		if unlockErr := s.wallet.UnlockFunds(ctx, lockResult.LockID, req.IdempotencyKey+"_unlock"); unlockErr != nil {
			s.logger.Error("Failed to unlock funds after withdrawal create failure",
				zap.Error(unlockErr),
				zap.String("lock_id", lockResult.LockID),
			)
		}
		return nil, fmt.Errorf("create withdrawal: %w", err)
	}

	// Track daily limit
	if _, err := s.dailyLimitsRepo.Increment(ctx, req.UserID, "withdrawal", fiatAmount); err != nil {
		s.logger.Warn("Failed to track daily withdrawal limit", zap.Error(err))
	}

	s.logger.Info("Withdrawal created",
		zap.Int64("user_id", req.UserID),
		zap.String("withdrawal_id", withdrawal.WithdrawalID),
		zap.String("amount", req.Amount.String()),
	)

	return s.toResponse(withdrawal), nil
}

// GetWithdrawal retrieves a withdrawal by UUID
func (s *WithdrawalService) GetWithdrawal(ctx context.Context, withdrawalUUID string) (*domain.Withdrawal, error) {
	return s.withdrawalRepo.GetByUUID(ctx, withdrawalUUID)
}

// GetWithdrawalByID retrieves a withdrawal by internal ID
func (s *WithdrawalService) GetWithdrawalByID(ctx context.Context, id int64) (*domain.Withdrawal, error) {
	return s.withdrawalRepo.GetByID(ctx, id)
}

// GetWithdrawalByWithdrawalID retrieves a withdrawal by NOWPayments ID
func (s *WithdrawalService) GetWithdrawalByWithdrawalID(ctx context.Context, withdrawalID string) (*domain.Withdrawal, error) {
	return s.withdrawalRepo.GetByWithdrawalID(ctx, withdrawalID)
}

// ListWithdrawals lists withdrawals for a user
func (s *WithdrawalService) ListWithdrawals(ctx context.Context, req ListPaymentsRequest) (*repository.ListResult[domain.Withdrawal], error) {
	filter := repository.ListFilter{
		Limit:  req.Limit,
		Cursor: req.Cursor,
		Status: req.Status,
	}

	return s.withdrawalRepo.ListByUserID(ctx, req.UserID, filter)
}

// validateKYCLevel validates minimum KYC level for withdrawals
func (s *WithdrawalService) validateKYCLevel(ctx context.Context, userID int64) error {
	kycLevel, err := s.user.GetKYCLevel(ctx, userID)
	if err != nil {
		return fmt.Errorf("get KYC level: %w", err)
	}

	if kycLevel < 2 {
		return domain.ErrorKYCRequiredLevel(kycLevel)
	}

	return nil
}

// validateWithdrawalLimits validates daily withdrawal limits
func (s *WithdrawalService) validateWithdrawalLimits(ctx context.Context, userID int64, amount decimal.Decimal) error {
	// Get KYC level
	kycLevel, err := s.user.GetKYCLevel(ctx, userID)
	if err != nil {
		return fmt.Errorf("get KYC level: %w", err)
	}

	// Get daily limit for KYC level
	limit := s.getWithdrawalLimit(kycLevel)

	// Get today's cumulative withdrawals
	used, err := s.dailyLimitsRepo.Get(ctx, userID, "withdrawal")
	if err != nil {
		return fmt.Errorf("get daily withdrawals: %w", err)
	}

	// Check if exceeds limit
	if used.Add(amount).GreaterThan(decimal.NewFromFloat(limit)) {
		return domain.ErrorDailyLimitExceeded(limit, used.InexactFloat64(), amount.InexactFloat64())
	}

	return nil
}

// getWithdrawalLimit returns the daily withdrawal limit for a KYC level
func (s *WithdrawalService) getWithdrawalLimit(kycLevel int) float64 {
	limits := map[int]float64{
		0: 0,
		1: 500,
		2: 5000,
		3: 25000,
	}
	if limit, ok := limits[kycLevel]; ok {
		return limit
	}
	return 0
}

// getFiatAmount converts crypto amount to fiat
func (s *WithdrawalService) getFiatAmount(ctx context.Context, amount decimal.Decimal, currency domain.CryptoCurrency) (decimal.Decimal, error) {
	rate, err := s.exchangeRateRepo.Get(ctx, currency.NOWPaymentsCurrency(), "USD")
	if err != nil {
		s.logger.Warn("Failed to get cached exchange rate", zap.Error(err))
	}

	if rate == nil {
		estResp, err := s.nowpayments.GetEstimatedPrice(ctx, amount, currency.NOWPaymentsCurrency(), "USD")
		if err != nil {
			return decimal.Zero, fmt.Errorf("get estimated price: %w", err)
		}
		rate = &estResp.EstimatedAmount

		if err := s.exchangeRateRepo.Set(ctx, currency.NOWPaymentsCurrency(), "USD", *rate, 60); err != nil {
			s.logger.Warn("Failed to cache exchange rate", zap.Error(err))
		}
	}

	return amount.Mul(*rate), nil
}

// getIPNCallbackURL returns the webhook callback URL
func (s *WithdrawalService) getIPNCallbackURL() string {
	return "https://api.platform.com/api/v1/payments/webhooks/nowpayments"
}

// toResponse converts a withdrawal to response
func (s *WithdrawalService) toResponse(withdrawal *domain.Withdrawal) *InitiateWithdrawalResponse {
	return &InitiateWithdrawalResponse{
		WithdrawalUUID: withdrawal.UUID.String(),
		WithdrawalID:   withdrawal.WithdrawalID,
		Amount:         withdrawal.Amount,
		FiatAmount:     withdrawal.FiatAmount,
		Currency:       withdrawal.CryptoCurrency,
		Address:        withdrawal.Address,
		Status:         string(withdrawal.Status),
	}
}
