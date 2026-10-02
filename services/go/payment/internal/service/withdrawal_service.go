package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/opus-casino/payment/internal/client"
	"github.com/opus-casino/payment/internal/domain"
	"github.com/opus-casino/payment/internal/event"
	"github.com/opus-casino/payment/internal/observability"
	"github.com/opus-casino/payment/internal/repository"
	"github.com/rs/zerolog/log"
	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel/trace"
)

// WithdrawalService handles withdrawal business logic
type WithdrawalService struct {
	withdrawalRepo   repository.WithdrawalRepository
	idempotencyRepo  repository.IdempotencyRepository
	exchangeRateRepo repository.ExchangeRateRepository
	dailyLimitsRepo  repository.DailyLimitsRepository
	nowpayments      client.NOWPaymentsAPI
	wallet           client.WalletAPI
	user             client.UserAPI
	producer         *event.Producer
	tracer           trace.Tracer
	ipnCallbackURL   string
}

// NewWithdrawalService creates a new withdrawal service
func NewWithdrawalService(
	withdrawalRepo repository.WithdrawalRepository,
	idempotencyRepo repository.IdempotencyRepository,
	exchangeRateRepo repository.ExchangeRateRepository,
	dailyLimitsRepo repository.DailyLimitsRepository,
	nowpayments client.NOWPaymentsAPI,
	wallet client.WalletAPI,
	user client.UserAPI,
	producer *event.Producer,
	tracer trace.Tracer,
	ipnCallbackURL string,
) *WithdrawalService {
	return &WithdrawalService{
		withdrawalRepo:   withdrawalRepo,
		idempotencyRepo:  idempotencyRepo,
		exchangeRateRepo: exchangeRateRepo,
		dailyLimitsRepo:  dailyLimitsRepo,
		nowpayments:      nowpayments,
		wallet:           wallet,
		user:             user,
		producer:         producer,
		tracer:           tracer,
		ipnCallbackURL:   ipnCallbackURL,
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
	// A replayed idempotency key is not a new withdrawal and must not be
	// counted again.
	if existingWithdrawal, err := s.withdrawalRepo.GetByIDempotencyKey(ctx, req.IdempotencyKey); err != nil {
		observability.RecordError("idempotency_lookup_failed", observability.OperationWithdrawal)
		return nil, fmt.Errorf("check idempotency: %w", err)
	} else if existingWithdrawal != nil {
		log.Info().
			Str("idempotency_key", req.IdempotencyKey).
			Str("withdrawal_id", existingWithdrawal.WithdrawalID).
			Msg("Returning existing withdrawal for idempotency key")
		return s.toResponse(existingWithdrawal), nil
	}

	// Validate KYC level (minimum level 2 for withdrawals)
	kycLevel, err := s.validateKYCLevel(ctx, req.UserID)
	if err != nil {
		observability.RecordError(metricErrorType(err), observability.OperationWithdrawal)
		return nil, err
	}

	// Validate currency
	if !req.Currency.IsWithdrawalSupported() {
		observability.RecordError("currency_not_supported", observability.OperationWithdrawal)
		return nil, domain.ErrorCurrencyNotSupported(string(req.Currency))
	}

	// Validate withdrawal limits
	if err := s.validateWithdrawalLimits(ctx, req.UserID, req.Amount); err != nil {
		observability.RecordError(metricErrorType(err), observability.OperationWithdrawal)
		return nil, err
	}

	// Get exchange rate for fiat amount
	fiatAmount, err := s.getFiatAmount(ctx, req.Amount, req.Currency)
	if err != nil {
		observability.RecordError("provider_error", observability.OperationWithdrawal)
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
		observability.RecordError(walletErrorType(err), observability.OperationWithdrawal)
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
			log.Error().
				Err(unlockErr).
				Str("lock_id", lockResult.LockID).
				Msg("Failed to unlock funds after payout failure")
			// The lock is stranded: the player cannot bet this balance and the
			// payout never happened. This needs a human, so it gets its own
			// label rather than being folded into provider_error.
			observability.RecordError("compensation_failed", observability.OperationWithdrawal)
		}
		observability.RecordError("provider_error", observability.OperationWithdrawal)
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
			log.Error().
				Err(unlockErr).
				Str("lock_id", lockResult.LockID).
				Msg("Failed to unlock funds after withdrawal create failure")
			observability.RecordError("compensation_failed", observability.OperationWithdrawal)
		}
		observability.RecordError("persistence_failed", observability.OperationWithdrawal)
		return nil, fmt.Errorf("create withdrawal: %w", err)
	}

	// Track daily limit
	if _, err := s.dailyLimitsRepo.Increment(ctx, req.UserID, "withdrawal", fiatAmount); err != nil {
		log.Warn().Err(err).Msg("Failed to track daily withdrawal limit")
		// The withdrawal exists and the payout is in flight, but the daily
		// limit was not charged, so the player can exceed their limit today.
		observability.RecordError("daily_limit_not_tracked", observability.OperationWithdrawal)
	}

	observability.RecordWithdrawal(
		observability.WithdrawalStatusProcessing,
		string(req.Currency),
		kycLevel,
		fiatAmount,
	)

	log.Info().
		Int64("user_id", req.UserID).
		Str("withdrawal_id", withdrawal.WithdrawalID).
		Str("amount", req.Amount.String()).
		Msg("Withdrawal created")

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

// validateKYCLevel validates minimum KYC level for withdrawals.
// It returns the KYC level so callers can label metrics with it instead of
// making a second gRPC round trip to User Service.
func (s *WithdrawalService) validateKYCLevel(ctx context.Context, userID int64) (int, error) {
	kycLevel, err := s.user.GetKYCLevel(ctx, userID)
	if err != nil {
		return observability.KYCLevelUnknown, fmt.Errorf("get KYC level: %w", err)
	}

	if kycLevel < 2 {
		return kycLevel, domain.ErrorKYCRequiredLevel(kycLevel)
	}

	return kycLevel, nil
}

// walletErrorType maps a Wallet Service gRPC failure onto a bounded
// error_type label. See metricErrorType for why the label must stay a constant
// set: the raw gRPC message is upstream text and must never become a label
// value, or one crafted error string would mint a new time series per request.
func walletErrorType(err error) string {
	if err == nil {
		return "internal_error"
	}
	if errors.Is(err, domain.ErrInsufficientBalance) {
		return "insufficient_balance"
	}
	if errors.Is(err, domain.ErrWalletLocked) {
		return "wallet_locked"
	}
	// WalletClient.mapError flattens gRPC codes to text, so match the stable
	// prefixes it emits rather than the upstream message.
	msg := err.Error()
	switch {
	case strings.Contains(msg, "not found"):
		return "wallet_not_found"
	case strings.Contains(msg, "invalid argument"):
		return "wallet_rejected_request"
	case strings.Contains(msg, "failed precondition"):
		return "wallet_locked"
	case strings.Contains(msg, "resource exhausted"):
		return "insufficient_balance"
	case strings.Contains(msg, "service unavailable"), strings.Contains(msg, "deadline exceeded"):
		return "wallet_unavailable"
	default:
		return "internal_error"
	}
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
		log.Warn().Err(err).Msg("Failed to get cached exchange rate")
	}

	if rate == nil {
		estResp, err := s.nowpayments.GetEstimatedPrice(ctx, amount, currency.NOWPaymentsCurrency(), "USD")
		if err != nil {
			return decimal.Zero, fmt.Errorf("get estimated price: %w", err)
		}
		rate = &estResp.EstimatedAmount

		if err := s.exchangeRateRepo.Set(ctx, currency.NOWPaymentsCurrency(), "USD", *rate, 60); err != nil {
			log.Warn().Err(err).Msg("Failed to cache exchange rate")
		}
	}

	return amount.Mul(*rate), nil
}

// getIPNCallbackURL returns the webhook callback URL configured via env/config
func (s *WithdrawalService) getIPNCallbackURL() string {
	return s.ipnCallbackURL
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
