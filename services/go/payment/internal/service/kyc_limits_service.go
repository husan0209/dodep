package service

import (
	"context"
	"fmt"

	"github.com/opus-casino/payment/internal/domain"
	"github.com/opus-casino/payment/internal/repository"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// KYCLimitsService handles KYC level-based deposit and withdrawal limit checking
type KYCLimitsService struct {
	dailyLimitsRepo repository.DailyLimitsRepository
	logger          *zap.Logger
}

// KYCLimitsConfig holds configuration for KYC limits
type KYCLimitsConfig struct {
	// Deposit limits by KYC level (0-3) in USD
	DepositLimits map[int]float64
	// Withdrawal limits by KYC level (0-3) in USD
	WithdrawalLimits map[int]float64
}

// DefaultKYCLimits returns the default KYC limits configuration
func DefaultKYCLimits() KYCLimitsConfig {
	return KYCLimitsConfig{
		DepositLimits: map[int]float64{
			0: 500,     // Level 0: $500/day
			1: 2000,    // Level 1: $2,000/day
			2: 10000,   // Level 2: $10,000/day
			3: 50000,   // Level 3: $50,000/day
		},
		WithdrawalLimits: map[int]float64{
			0: 0,      // Level 0: $0/day (no withdrawals allowed)
			1: 500,    // Level 1: $500/day
			2: 5000,   // Level 2: $5,000/day
			3: 25000,  // Level 3: $25,000/day
		},
	}
}

// NewKYCLimitsService creates a new KYC limits service
func NewKYCLimitsService(
	dailyLimitsRepo repository.DailyLimitsRepository,
	logger *zap.Logger,
) *KYCLimitsService {
	return &KYCLimitsService{
		dailyLimitsRepo: dailyLimitsRepo,
		logger:          logger,
	}
}

// LimitCheckResult represents the result of a limit check
type LimitCheckResult struct {
	Allowed   bool
	Used      decimal.Decimal
	Limit     decimal.Decimal
	Available decimal.Decimal
}

// CheckDepositLimit checks if a deposit is within the user's KYC level limits
func (s *KYCLimitsService) CheckDepositLimit(
	ctx context.Context,
	kycLevel int,
	userID int64,
	amount decimal.Decimal,
) (*LimitCheckResult, error) {
	// Get deposit limit for KYC level
	limit := s.GetDepositLimit(kycLevel)

	// Get today's cumulative deposits
	used, err := s.dailyLimitsRepo.Get(ctx, userID, "deposit")
	if err != nil {
		return nil, fmt.Errorf("get daily deposits: %w", err)
	}

	limitDec := decimal.NewFromFloat(limit)
	available := limitDec.Sub(used)

	// Check if exceeds limit
	allowed := used.Add(amount).LessThanOrEqual(limitDec)

	result := &LimitCheckResult{
		Allowed:   allowed,
		Used:      used,
		Limit:     limitDec,
		Available: available,
	}

	if !allowed {
		s.logger.Warn("Deposit limit exceeded",
			zap.Int64("user_id", userID),
			zap.Int("kyc_level", kycLevel),
			zap.String("amount", amount.String()),
			zap.String("used", used.String()),
			zap.String("limit", limitDec.String()),
		)
	}

	return result, nil
}

// CheckWithdrawalLimit checks if a withdrawal is within the user's KYC level limits
func (s *KYCLimitsService) CheckWithdrawalLimit(
	ctx context.Context,
	kycLevel int,
	userID int64,
	amount decimal.Decimal,
) (*LimitCheckResult, error) {
	// Get withdrawal limit for KYC level
	limit := s.GetWithdrawalLimit(kycLevel)

	// Level 0 cannot withdraw at all
	if limit == 0 {
		s.logger.Warn("Withdrawal not allowed for KYC level 0",
			zap.Int64("user_id", userID),
			zap.Int("kyc_level", kycLevel),
		)
		return &LimitCheckResult{
			Allowed:   false,
			Used:      decimal.Zero,
			Limit:     decimal.Zero,
			Available: decimal.Zero,
		}, nil
	}

	// Get today's cumulative withdrawals
	used, err := s.dailyLimitsRepo.Get(ctx, userID, "withdrawal")
	if err != nil {
		return nil, fmt.Errorf("get daily withdrawals: %w", err)
	}

	limitDec := decimal.NewFromFloat(limit)
	available := limitDec.Sub(used)

	// Check if exceeds limit
	allowed := used.Add(amount).LessThanOrEqual(limitDec)

	result := &LimitCheckResult{
		Allowed:   allowed,
		Used:      used,
		Limit:     limitDec,
		Available: available,
	}

	if !allowed {
		s.logger.Warn("Withdrawal limit exceeded",
			zap.Int64("user_id", userID),
			zap.Int("kyc_level", kycLevel),
			zap.String("amount", amount.String()),
			zap.String("used", used.String()),
			zap.String("limit", limitDec.String()),
		)
	}

	return result, nil
}

// RecordDeposit records a deposit amount against the user's daily limit
func (s *KYCLimitsService) RecordDeposit(
	ctx context.Context,
	userID int64,
	amount decimal.Decimal,
) (decimal.Decimal, error) {
	newTotal, err := s.dailyLimitsRepo.Increment(ctx, userID, "deposit", amount)
	if err != nil {
		return decimal.Zero, fmt.Errorf("record deposit: %w", err)
	}

	s.logger.Info("Deposit recorded against daily limit",
		zap.Int64("user_id", userID),
		zap.String("amount", amount.String()),
		zap.String("new_total", newTotal.String()),
	)

	return newTotal, nil
}

// RecordWithdrawal records a withdrawal amount against the user's daily limit
func (s *KYCLimitsService) RecordWithdrawal(
	ctx context.Context,
	userID int64,
	amount decimal.Decimal,
) (decimal.Decimal, error) {
	newTotal, err := s.dailyLimitsRepo.Increment(ctx, userID, "withdrawal", amount)
	if err != nil {
		return decimal.Zero, fmt.Errorf("record withdrawal: %w", err)
	}

	s.logger.Info("Withdrawal recorded against daily limit",
		zap.Int64("user_id", userID),
		zap.String("amount", amount.String()),
		zap.String("new_total", newTotal.String()),
	)

	return newTotal, nil
}

// GetDepositLimit returns the daily deposit limit for a KYC level
func (s *KYCLimitsService) GetDepositLimit(kycLevel int) float64 {
	limits := DefaultKYCLimits().DepositLimits
	if limit, ok := limits[kycLevel]; ok {
		return limit
	}
	// Default to level 0 for unknown levels
	return limits[0]
}

// GetWithdrawalLimit returns the daily withdrawal limit for a KYC level
func (s *KYCLimitsService) GetWithdrawalLimit(kycLevel int) float64 {
	limits := DefaultKYCLimits().WithdrawalLimits
	if limit, ok := limits[kycLevel]; ok {
		return limit
	}
	// Default to level 0 for unknown levels
	return limits[0]
}

// ValidateDeposit checks deposit limits and returns an error if exceeded
func (s *KYCLimitsService) ValidateDeposit(
	ctx context.Context,
	kycLevel int,
	userID int64,
	amount decimal.Decimal,
) error {
	result, err := s.CheckDepositLimit(ctx, kycLevel, userID, amount)
	if err != nil {
		return err
	}

	if !result.Allowed {
		return domain.ErrorDailyLimitExceeded(
			result.Limit.InexactFloat64(),
			result.Used.InexactFloat64(),
			amount.InexactFloat64(),
		)
	}

	return nil
}

// ValidateWithdrawal checks withdrawal limits and returns an error if exceeded
func (s *KYCLimitsService) ValidateWithdrawal(
	ctx context.Context,
	kycLevel int,
	userID int64,
	amount decimal.Decimal,
) error {
	result, err := s.CheckWithdrawalLimit(ctx, kycLevel, userID, amount)
	if err != nil {
		return err
	}

	if !result.Allowed {
		return domain.ErrorDailyLimitExceeded(
			result.Limit.InexactFloat64(),
			result.Used.InexactFloat64(),
			amount.InexactFloat64(),
		)
	}

	return nil
}

// GetDailyUsage returns the current daily usage for a user
func (s *KYCLimitsService) GetDailyUsage(
	ctx context.Context,
	userID int64,
	operationType string,
) (decimal.Decimal, error) {
	return s.dailyLimitsRepo.Get(ctx, userID, operationType)
}
