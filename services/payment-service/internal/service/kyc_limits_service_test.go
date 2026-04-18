package service

import (
	"context"
	"errors"
	"testing"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// Helper function to create a test KYC limits service
func newTestKYCLimitsService(dailyLimitsRepo *MockDailyLimitsRepository) *KYCLimitsService {
	return NewKYCLimitsService(
		dailyLimitsRepo,
		zap.NewNop(),
	)
}

func TestCheckDepositLimit_Allowed(t *testing.T) {
	ctx := context.Background()

	dailyLimitsRepo := &MockDailyLimitsRepository{
		GetFunc: func(ctx context.Context, userID int64, operationType string) (decimal.Decimal, error) {
			// User has used $100 of their $500 limit (KYC level 0)
			return decimal.NewFromFloat(100.0), nil
		},
	}

	service := newTestKYCLimitsService(dailyLimitsRepo)

	// Request $300 deposit (should be allowed: 100 + 300 = 400 < 500)
	result, err := service.CheckDepositLimit(ctx, 0, 12345, decimal.NewFromFloat(300.0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Allowed {
		t.Error("expected deposit to be allowed")
	}

	if !result.Used.Equal(decimal.NewFromFloat(100.0)) {
		t.Errorf("expected used 100, got %s", result.Used.String())
	}

	if !result.Limit.Equal(decimal.NewFromFloat(500.0)) {
		t.Errorf("expected limit 500, got %s", result.Limit.String())
	}

	if !result.Available.Equal(decimal.NewFromFloat(400.0)) {
		t.Errorf("expected available 400, got %s", result.Available.String())
	}
}

func TestCheckDepositLimit_Exceeded(t *testing.T) {
	ctx := context.Background()

	dailyLimitsRepo := &MockDailyLimitsRepository{
		GetFunc: func(ctx context.Context, userID int64, operationType string) (decimal.Decimal, error) {
			// User has used $450 of their $500 limit (KYC level 0)
			return decimal.NewFromFloat(450.0), nil
		},
	}

	service := newTestKYCLimitsService(dailyLimitsRepo)

	// Request $100 deposit (should be denied: 450 + 100 = 550 > 500)
	result, err := service.CheckDepositLimit(ctx, 0, 12345, decimal.NewFromFloat(100.0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Allowed {
		t.Error("expected deposit to be denied")
	}

	if !result.Used.Equal(decimal.NewFromFloat(450.0)) {
		t.Errorf("expected used 450, got %s", result.Used.String())
	}

	if !result.Limit.Equal(decimal.NewFromFloat(500.0)) {
		t.Errorf("expected limit 500, got %s", result.Limit.String())
	}
}

func TestCheckWithdrawalLimit_Allowed(t *testing.T) {
	ctx := context.Background()

	dailyLimitsRepo := &MockDailyLimitsRepository{
		GetFunc: func(ctx context.Context, userID int64, operationType string) (decimal.Decimal, error) {
			// User has used $200 of their $500 limit (KYC level 1)
			return decimal.NewFromFloat(200.0), nil
		},
	}

	service := newTestKYCLimitsService(dailyLimitsRepo)

	// Request $200 withdrawal (should be allowed: 200 + 200 = 400 < 500)
	result, err := service.CheckWithdrawalLimit(ctx, 1, 12345, decimal.NewFromFloat(200.0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Allowed {
		t.Error("expected withdrawal to be allowed")
	}

	if !result.Used.Equal(decimal.NewFromFloat(200.0)) {
		t.Errorf("expected used 200, got %s", result.Used.String())
	}

	if !result.Limit.Equal(decimal.NewFromFloat(500.0)) {
		t.Errorf("expected limit 500, got %s", result.Limit.String())
	}
}

func TestCheckWithdrawalLimit_Level0(t *testing.T) {
	ctx := context.Background()

	dailyLimitsRepo := &MockDailyLimitsRepository{}

	service := newTestKYCLimitsService(dailyLimitsRepo)

	// KYC level 0 cannot withdraw at all
	result, err := service.CheckWithdrawalLimit(ctx, 0, 12345, decimal.NewFromFloat(100.0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Allowed {
		t.Error("expected withdrawal to be denied for KYC level 0")
	}

	if !result.Limit.IsZero() {
		t.Errorf("expected limit 0 for KYC level 0, got %s", result.Limit.String())
	}

	if !result.Available.IsZero() {
		t.Errorf("expected available 0 for KYC level 0, got %s", result.Available.String())
	}
}

func TestCheckWithdrawalLimit_Exceeded(t *testing.T) {
	ctx := context.Background()

	dailyLimitsRepo := &MockDailyLimitsRepository{
		GetFunc: func(ctx context.Context, userID int64, operationType string) (decimal.Decimal, error) {
			// User has used $450 of their $500 limit (KYC level 1)
			return decimal.NewFromFloat(450.0), nil
		},
	}

	service := newTestKYCLimitsService(dailyLimitsRepo)

	// Request $100 withdrawal (should be denied: 450 + 100 = 550 > 500)
	result, err := service.CheckWithdrawalLimit(ctx, 1, 12345, decimal.NewFromFloat(100.0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Allowed {
		t.Error("expected withdrawal to be denied")
	}

	if !result.Used.Equal(decimal.NewFromFloat(450.0)) {
		t.Errorf("expected used 450, got %s", result.Used.String())
	}
}

func TestRecordDeposit(t *testing.T) {
	ctx := context.Background()

	var recordedAmount decimal.Decimal

	dailyLimitsRepo := &MockDailyLimitsRepository{
		IncrementFunc: func(ctx context.Context, userID int64, operationType string, amount decimal.Decimal) (decimal.Decimal, error) {
			recordedAmount = amount
			return decimal.NewFromFloat(200.0), nil
		},
	}

	service := newTestKYCLimitsService(dailyLimitsRepo)

	newTotal, err := service.RecordDeposit(ctx, 12345, decimal.NewFromFloat(100.0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !newTotal.Equal(decimal.NewFromFloat(200.0)) {
		t.Errorf("expected new total 200, got %s", newTotal.String())
	}

	if !recordedAmount.Equal(decimal.NewFromFloat(100.0)) {
		t.Errorf("expected recorded amount 100, got %s", recordedAmount.String())
	}
}

func TestRecordWithdrawal(t *testing.T) {
	ctx := context.Background()

	var recordedAmount decimal.Decimal

	dailyLimitsRepo := &MockDailyLimitsRepository{
		IncrementFunc: func(ctx context.Context, userID int64, operationType string, amount decimal.Decimal) (decimal.Decimal, error) {
			recordedAmount = amount
			return decimal.NewFromFloat(300.0), nil
		},
	}

	service := newTestKYCLimitsService(dailyLimitsRepo)

	newTotal, err := service.RecordWithdrawal(ctx, 12345, decimal.NewFromFloat(100.0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !newTotal.Equal(decimal.NewFromFloat(300.0)) {
		t.Errorf("expected new total 300, got %s", newTotal.String())
	}

	if !recordedAmount.Equal(decimal.NewFromFloat(100.0)) {
		t.Errorf("expected recorded amount 100, got %s", recordedAmount.String())
	}
}

func TestGetDepositLimit(t *testing.T) {
	service := newTestKYCLimitsService(nil)

	tests := []struct {
		name          string
		kycLevel      int
		expectedLimit float64
	}{
		{"KYC level 0", 0, 500},
		{"KYC level 1", 1, 2000},
		{"KYC level 2", 2, 10000},
		{"KYC level 3", 3, 50000},
		{"Unknown level defaults to 0", 99, 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit := service.GetDepositLimit(tt.kycLevel)
			if limit != tt.expectedLimit {
				t.Errorf("expected limit %f for KYC level %d, got %f", tt.expectedLimit, tt.kycLevel, limit)
			}
		})
	}
}

func TestGetWithdrawalLimit(t *testing.T) {
	service := newTestKYCLimitsService(nil)

	tests := []struct {
		name          string
		kycLevel      int
		expectedLimit float64
	}{
		{"KYC level 0 - no withdrawals", 0, 0},
		{"KYC level 1", 1, 500},
		{"KYC level 2", 2, 5000},
		{"KYC level 3", 3, 25000},
		{"Unknown level defaults to 0", 99, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit := service.GetWithdrawalLimit(tt.kycLevel)
			if limit != tt.expectedLimit {
				t.Errorf("expected limit %f for KYC level %d, got %f", tt.expectedLimit, tt.kycLevel, limit)
			}
		})
	}
}

func TestCheckDepositLimit_RepositoryError(t *testing.T) {
	ctx := context.Background()

	dailyLimitsRepo := &MockDailyLimitsRepository{
		GetFunc: func(ctx context.Context, userID int64, operationType string) (decimal.Decimal, error) {
			return decimal.Zero, errors.New("database error")
		},
	}

	service := newTestKYCLimitsService(dailyLimitsRepo)

	_, err := service.CheckDepositLimit(ctx, 0, 12345, decimal.NewFromFloat(100.0))
	if err == nil {
		t.Error("expected error from repository")
	}
}

func TestCheckWithdrawalLimit_RepositoryError(t *testing.T) {
	ctx := context.Background()

	dailyLimitsRepo := &MockDailyLimitsRepository{
		GetFunc: func(ctx context.Context, userID int64, operationType string) (decimal.Decimal, error) {
			return decimal.Zero, errors.New("database error")
		},
	}

	service := newTestKYCLimitsService(dailyLimitsRepo)

	_, err := service.CheckWithdrawalLimit(ctx, 1, 12345, decimal.NewFromFloat(100.0))
	if err == nil {
		t.Error("expected error from repository")
	}
}

func TestRecordDeposit_RepositoryError(t *testing.T) {
	ctx := context.Background()

	dailyLimitsRepo := &MockDailyLimitsRepository{
		IncrementFunc: func(ctx context.Context, userID int64, operationType string, amount decimal.Decimal) (decimal.Decimal, error) {
			return decimal.Zero, errors.New("database error")
		},
	}

	service := newTestKYCLimitsService(dailyLimitsRepo)

	_, err := service.RecordDeposit(ctx, 12345, decimal.NewFromFloat(100.0))
	if err == nil {
		t.Error("expected error from repository")
	}
}

func TestRecordWithdrawal_RepositoryError(t *testing.T) {
	ctx := context.Background()

	dailyLimitsRepo := &MockDailyLimitsRepository{
		IncrementFunc: func(ctx context.Context, userID int64, operationType string, amount decimal.Decimal) (decimal.Decimal, error) {
			return decimal.Zero, errors.New("database error")
		},
	}

	service := newTestKYCLimitsService(dailyLimitsRepo)

	_, err := service.RecordWithdrawal(ctx, 12345, decimal.NewFromFloat(100.0))
	if err == nil {
		t.Error("expected error from repository")
	}
}

func TestCheckDepositLimit_EdgeCaseExactLimit(t *testing.T) {
	ctx := context.Background()

	dailyLimitsRepo := &MockDailyLimitsRepository{
		GetFunc: func(ctx context.Context, userID int64, operationType string) (decimal.Decimal, error) {
			// User has used $400 of their $500 limit
			return decimal.NewFromFloat(400.0), nil
		},
	}

	service := newTestKYCLimitsService(dailyLimitsRepo)

	// Request exactly $100 (should be allowed: 400 + 100 = 500 = limit)
	result, err := service.CheckDepositLimit(ctx, 0, 12345, decimal.NewFromFloat(100.0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Allowed {
		t.Error("expected deposit to be allowed when exactly at limit")
	}
}

func TestCheckWithdrawalLimit_EdgeCaseExactLimit(t *testing.T) {
	ctx := context.Background()

	dailyLimitsRepo := &MockDailyLimitsRepository{
		GetFunc: func(ctx context.Context, userID int64, operationType string) (decimal.Decimal, error) {
			// User has used $400 of their $500 limit
			return decimal.NewFromFloat(400.0), nil
		},
	}

	service := newTestKYCLimitsService(dailyLimitsRepo)

	// Request exactly $100 (should be allowed: 400 + 100 = 500 = limit)
	result, err := service.CheckWithdrawalLimit(ctx, 1, 12345, decimal.NewFromFloat(100.0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Allowed {
		t.Error("expected withdrawal to be allowed when exactly at limit")
	}
}

func TestCheckDepositLimit_AllKYCLevels(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		kycLevel  int
		usedToday float64
		request   float64
		allowed   bool
	}{
		{"Level 0 - under limit", 0, 100, 300, true},
		{"Level 0 - at limit", 0, 400, 100, true},
		{"Level 0 - over limit", 0, 400, 200, false},
		{"Level 1 - under limit", 1, 1000, 500, true},
		{"Level 1 - at limit", 1, 1900, 100, true},
		{"Level 1 - over limit", 1, 1900, 200, false},
		{"Level 2 - under limit", 2, 5000, 4000, true},
		{"Level 2 - at limit", 2, 9900, 100, true},
		{"Level 2 - over limit", 2, 9900, 200, false},
		{"Level 3 - under limit", 3, 25000, 20000, true},
		{"Level 3 - at limit", 3, 49900, 100, true},
		{"Level 3 - over limit", 3, 49900, 200, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dailyLimitsRepo := &MockDailyLimitsRepository{
				GetFunc: func(ctx context.Context, userID int64, operationType string) (decimal.Decimal, error) {
					return decimal.NewFromFloat(tt.usedToday), nil
				},
			}

			service := newTestKYCLimitsService(dailyLimitsRepo)

			result, err := service.CheckDepositLimit(ctx, tt.kycLevel, 12345, decimal.NewFromFloat(tt.request))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Allowed != tt.allowed {
				t.Errorf("expected allowed=%v, got %v", tt.allowed, result.Allowed)
			}
		})
	}
}

func TestCheckWithdrawalLimit_AllKYCLevels(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		kycLevel  int
		usedToday float64
		request   float64
		allowed   bool
	}{
		{"Level 0 - no withdrawals", 0, 0, 100, false},
		{"Level 1 - under limit", 1, 200, 200, true},
		{"Level 1 - at limit", 1, 400, 100, true},
		{"Level 1 - over limit", 1, 400, 200, false},
		{"Level 2 - under limit", 2, 2000, 2000, true},
		{"Level 2 - at limit", 2, 4900, 100, true},
		{"Level 2 - over limit", 2, 4900, 200, false},
		{"Level 3 - under limit", 3, 10000, 10000, true},
		{"Level 3 - at limit", 3, 24900, 100, true},
		{"Level 3 - over limit", 3, 24900, 200, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dailyLimitsRepo := &MockDailyLimitsRepository{
				GetFunc: func(ctx context.Context, userID int64, operationType string) (decimal.Decimal, error) {
					return decimal.NewFromFloat(tt.usedToday), nil
				},
			}

			service := newTestKYCLimitsService(dailyLimitsRepo)

			result, err := service.CheckWithdrawalLimit(ctx, tt.kycLevel, 12345, decimal.NewFromFloat(tt.request))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Allowed != tt.allowed {
				t.Errorf("expected allowed=%v, got %v", tt.allowed, result.Allowed)
			}
		})
	}
}
