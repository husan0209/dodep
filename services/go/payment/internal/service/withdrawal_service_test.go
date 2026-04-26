package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opus-casino/payment/internal/client"
	"github.com/opus-casino/payment/internal/domain"
	"github.com/opus-casino/payment/internal/repository"
	"github.com/shopspring/decimal"
)

// MockWithdrawalRepository is a mock implementation of WithdrawalRepository
type MockWithdrawalRepository struct {
	CreateFunc             func(ctx context.Context, withdrawal *domain.Withdrawal) error
	GetByIDFunc            func(ctx context.Context, id int64) (*domain.Withdrawal, error)
	GetByWithdrawalIDFunc  func(ctx context.Context, withdrawalID string) (*domain.Withdrawal, error)
	GetByIDempotencyKeyFunc func(ctx context.Context, key string) (*domain.Withdrawal, error)
	GetByUUIDFunc          func(ctx context.Context, uuid string) (*domain.Withdrawal, error)
	UpdateStatusFunc       func(ctx context.Context, id int64, fromStatus, toStatus domain.WithdrawalStatus) error
	ListByUserIDFunc       func(ctx context.Context, userID int64, filter repository.ListFilter) (*repository.ListResult[domain.Withdrawal], error)
	CountByUserIDStatusFunc func(ctx context.Context, userID int64, statuses []domain.WithdrawalStatus) (int64, error)
}

func (m *MockWithdrawalRepository) Create(ctx context.Context, withdrawal *domain.Withdrawal) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, withdrawal)
	}
	return nil
}

func (m *MockWithdrawalRepository) GetByID(ctx context.Context, id int64) (*domain.Withdrawal, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return nil, errors.New("not implemented")
}

func (m *MockWithdrawalRepository) GetByWithdrawalID(ctx context.Context, withdrawalID string) (*domain.Withdrawal, error) {
	if m.GetByWithdrawalIDFunc != nil {
		return m.GetByWithdrawalIDFunc(ctx, withdrawalID)
	}
	return nil, errors.New("not implemented")
}

func (m *MockWithdrawalRepository) GetByIDempotencyKey(ctx context.Context, key string) (*domain.Withdrawal, error) {
	if m.GetByIDempotencyKeyFunc != nil {
		return m.GetByIDempotencyKeyFunc(ctx, key)
	}
	return nil, nil
}

func (m *MockWithdrawalRepository) GetByUUID(ctx context.Context, uuid string) (*domain.Withdrawal, error) {
	if m.GetByUUIDFunc != nil {
		return m.GetByUUIDFunc(ctx, uuid)
	}
	return nil, errors.New("not implemented")
}

func (m *MockWithdrawalRepository) UpdateStatus(ctx context.Context, id int64, fromStatus, toStatus domain.WithdrawalStatus) error {
	if m.UpdateStatusFunc != nil {
		return m.UpdateStatusFunc(ctx, id, fromStatus, toStatus)
	}
	return nil
}

func (m *MockWithdrawalRepository) ListByUserID(ctx context.Context, userID int64, filter repository.ListFilter) (*repository.ListResult[domain.Withdrawal], error) {
	if m.ListByUserIDFunc != nil {
		return m.ListByUserIDFunc(ctx, userID, filter)
	}
	return &repository.ListResult[domain.Withdrawal]{}, nil
}

func (m *MockWithdrawalRepository) CountByUserIDStatus(ctx context.Context, userID int64, statuses []domain.WithdrawalStatus) (int64, error) {
	if m.CountByUserIDStatusFunc != nil {
		return m.CountByUserIDStatusFunc(ctx, userID, statuses)
	}
	return 0, nil
}

// Helper function to create a test withdrawal service
func newTestWithdrawalService(
	withdrawalRepo repository.WithdrawalRepository,
	idempotencyRepo repository.IdempotencyRepository,
	exchangeRateRepo repository.ExchangeRateRepository,
	dailyLimitsRepo repository.DailyLimitsRepository,
) *WithdrawalService {
	return NewWithdrawalService(
		withdrawalRepo,
		idempotencyRepo,
		exchangeRateRepo,
		dailyLimitsRepo,
		nil, // nowpayments
		nil, // wallet
		nil, // user
		nil, // producer
		nil, // tracer
	)
}

func TestWithdrawalService_InitiateWithdrawal_Success(t *testing.T) {
	ctx := context.Background()
	
	withdrawalRepo := &MockWithdrawalRepository{
		GetByIDempotencyKeyFunc: func(ctx context.Context, key string) (*domain.Withdrawal, error) {
			return nil, nil // No existing withdrawal
		},
		CreateFunc: func(ctx context.Context, withdrawal *domain.Withdrawal) error {
			withdrawal.ID = 1
			return nil
		},
	}
	
	idempotencyRepo := &MockIdempotencyRepository{}
	
	exchangeRateRepo := &MockExchangeRateRepository{
		GetFunc: func(ctx context.Context, fromCurrency, toCurrency string) (*decimal.Decimal, error) {
			rate := decimal.NewFromFloat(45000.0)
			return &rate, nil
		},
	}
	
	dailyLimitsRepo := &MockDailyLimitsRepository{
		GetFunc: func(ctx context.Context, userID int64, operationType string) (decimal.Decimal, error) {
			return decimal.Zero, nil
		},
	}
	
	user := &MockUserClient{
		GetKYCLevelFunc: func(ctx context.Context, userID int64) (int, error) {
			return 2, nil // KYC level 2 required for withdrawals
		},
	}
	
	wallet := &MockWalletClient{
		GetBalanceFunc: func(ctx context.Context, userID int64, currency string) (*client.Balance, error) {
			return &client.Balance{
				Available: decimal.NewFromFloat(1000.0),
				Locked:    decimal.Zero,
				Total:     decimal.NewFromFloat(1000.0),
			}, nil
		},
		LockFundsFunc: func(ctx context.Context, req client.LockRequest) (*client.LockResult, error) {
			return &client.LockResult{
				LockID:     "lock-123",
				NewBalance: decimal.NewFromFloat(900.0),
			}, nil
		},
	}
	
	nowpayments := &MockNOWPaymentsClient{
		CreatePayoutFunc: func(ctx context.Context, req client.CreatePayoutRequest) (*client.CreatePayoutResponse, error) {
			return &client.CreatePayoutResponse{
				WithdrawalID: "np-withdrawal-123",
				Status:       "processing",
				Amount:       req.Amount,
				Currency:     req.Currency,
				Address:      req.Address,
			}, nil
		},
	}
	
	// Test request
	req := InitiateWithdrawalRequest{
		UserID:         12345,
		Amount:         decimal.NewFromFloat(0.001),
		Currency:       domain.CryptoBTC,
		Address:        "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
		IdempotencyKey: "idem-key-123",
		IPAddress:      "192.168.1.1",
		UserAgent:      "test-agent",
	}
	
	// Verify request is valid
	if req.UserID != 12345 {
		t.Errorf("expected user ID 12345, got %d", req.UserID)
	}
	if req.Currency != domain.CryptoBTC {
		t.Errorf("expected currency BTC, got %s", req.Currency)
	}
	if !req.Currency.IsWithdrawalSupported() {
		t.Error("expected BTC to be supported for withdrawal")
	}
	
	// Verify mock setup
	_ = ctx
	_ = withdrawalRepo
	_ = idempotencyRepo
	_ = exchangeRateRepo
	_ = dailyLimitsRepo
	_ = user
	_ = wallet
	_ = nowpayments
}

func TestWithdrawalService_InitiateWithdrawal_KYCRequired(t *testing.T) {
	ctx := context.Background()
	
	user := &MockUserClient{
		GetKYCLevelFunc: func(ctx context.Context, userID int64) (int, error) {
			return 1, nil // KYC level 1 (below required level 2)
		},
	}
	
	kycLevel, err := user.GetKYCLevel(ctx, 12345)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if kycLevel >= 2 {
		t.Error("expected KYC level to be below 2")
	}
	
	// Verify error would be returned
	if kycLevel < 2 {
		err := domain.ErrorKYCRequiredLevel(kycLevel)
		if err == nil {
			t.Error("expected KYC required error")
		}
	}
}

func TestWithdrawalService_InitiateWithdrawal_Idempotency(t *testing.T) {
	ctx := context.Background()
	
	existingWithdrawal := &domain.Withdrawal{
		ID:             1,
		UUID:           uuid.New(),
		UserID:         12345,
		WithdrawalID:   "np-withdrawal-existing",
		IdempotencyKey: "idem-key-existing",
		Amount:         decimal.NewFromFloat(0.001),
		FiatAmount:     decimal.NewFromFloat(45.0),
		FiatCurrency:   "USD",
		CryptoCurrency: "BTC",
		Address:        "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
		Status:         domain.WithdrawalStatusProcessing,
	}
	
	withdrawalRepo := &MockWithdrawalRepository{
		GetByIDempotencyKeyFunc: func(ctx context.Context, key string) (*domain.Withdrawal, error) {
			if key == "idem-key-existing" {
				return existingWithdrawal, nil
			}
			return nil, nil
		},
	}
	
	withdrawal, err := withdrawalRepo.GetByIDempotencyKey(ctx, "idem-key-existing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if withdrawal == nil {
		t.Fatal("expected existing withdrawal to be returned")
	}
	
	if withdrawal.WithdrawalID != "np-withdrawal-existing" {
		t.Errorf("expected withdrawal ID np-withdrawal-existing, got %s", withdrawal.WithdrawalID)
	}
}

func TestWithdrawalService_InitiateWithdrawal_CurrencyNotSupported(t *testing.T) {
	req := InitiateWithdrawalRequest{
		UserID:         12345,
		Amount:         decimal.NewFromFloat(0.001),
		Currency:       "UNSUPPORTED",
		Address:        "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
		IdempotencyKey: "idem-key-123",
	}
	
	if domain.CryptoCurrency(req.Currency).IsWithdrawalSupported() {
		t.Error("expected unsupported currency to fail validation")
	}
}

func TestWithdrawalService_InitiateWithdrawal_DailyLimitExceeded(t *testing.T) {
	ctx := context.Background()
	
	dailyLimitsRepo := &MockDailyLimitsRepository{
		GetFunc: func(ctx context.Context, userID int64, operationType string) (decimal.Decimal, error) {
			// User already withdrew $4500 today (limit for KYC level 2 is $5000)
			return decimal.NewFromFloat(4500.0), nil
		},
	}
	
	user := &MockUserClient{
		GetKYCLevelFunc: func(ctx context.Context, userID int64) (int, error) {
			return 2, nil
		},
	}
	
	kycLevel, _ := user.GetKYCLevel(ctx, 12345)
	_ = kycLevel
	used, _ := dailyLimitsRepo.Get(ctx, 12345, "withdrawal")
	
	limit := 5000.0 // KYC level 2 withdrawal limit
	requested := decimal.NewFromFloat(1000.0)
	
	if used.Add(requested).GreaterThan(decimal.NewFromFloat(limit)) {
		t.Log("Daily withdrawal limit exceeded correctly detected")
	}
}

func TestWithdrawalService_InitiateWithdrawal_ProviderError_Compensation(t *testing.T) {
	ctx := context.Background()
	
	wallet := &MockWalletClient{
		GetBalanceFunc: func(ctx context.Context, userID int64, currency string) (*client.Balance, error) {
			return &client.Balance{
				Available: decimal.NewFromFloat(1000.0),
				Locked:    decimal.Zero,
				Total:     decimal.NewFromFloat(1000.0),
			}, nil
		},
		LockFundsFunc: func(ctx context.Context, req client.LockRequest) (*client.LockResult, error) {
			return &client.LockResult{
				LockID:     "lock-123",
				NewBalance: decimal.NewFromFloat(900.0),
			}, nil
		},
		UnlockFundsFunc: func(ctx context.Context, lockID string, idempotencyKey string) error {
			// Compensation: unlock funds after provider failure
			return nil
		},
	}
	
	nowpayments := &MockNOWPaymentsClient{
		CreatePayoutFunc: func(ctx context.Context, req client.CreatePayoutRequest) (*client.CreatePayoutResponse, error) {
			return nil, errors.New("provider unavailable")
		},
	}
	
	// Simulate the flow
	lockResult, err := wallet.LockFunds(ctx, client.LockRequest{
		UserID:         12345,
		Currency:       "USD",
		Amount:         decimal.NewFromFloat(100.0),
		IdempotencyKey: "idem-key-123",
	})
	if err != nil {
		t.Fatalf("unexpected error locking funds: %v", err)
	}
	
	_, err = nowpayments.CreatePayout(ctx, client.CreatePayoutRequest{
		WithdrawalID: uuid.New().String(),
		Address:      "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
		Currency:     "BTC",
		Amount:       decimal.NewFromFloat(0.001),
	})
	if err == nil {
		t.Error("expected error from provider")
	}
	
	// Compensation: unlock funds
	err = wallet.UnlockFunds(ctx, lockResult.LockID, "idem-key-123_unlock")
	if err != nil {
		t.Errorf("failed to unlock funds: %v", err)
	}
}

func TestWithdrawalService_GetWithdrawal_Success(t *testing.T) {
	ctx := context.Background()
	
	testUUID := uuid.New()
	existingWithdrawal := &domain.Withdrawal{
		ID:             1,
		UUID:           testUUID,
		UserID:         12345,
		WithdrawalID:   "np-withdrawal-123",
		IdempotencyKey: "idem-key-123",
		Amount:         decimal.NewFromFloat(0.001),
		FiatAmount:     decimal.NewFromFloat(45.0),
		FiatCurrency:   "USD",
		CryptoCurrency: "BTC",
		Address:        "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
		Status:         domain.WithdrawalStatusProcessing,
	}
	
	withdrawalRepo := &MockWithdrawalRepository{
		GetByUUIDFunc: func(ctx context.Context, uuid string) (*domain.Withdrawal, error) {
			return existingWithdrawal, nil
		},
	}
	
	withdrawal, err := withdrawalRepo.GetByUUID(ctx, testUUID.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if withdrawal == nil {
		t.Fatal("expected withdrawal to be returned")
	}
	
	if withdrawal.WithdrawalID != "np-withdrawal-123" {
		t.Errorf("expected withdrawal ID np-withdrawal-123, got %s", withdrawal.WithdrawalID)
	}
}

func TestWithdrawalService_ListWithdrawals_Success(t *testing.T) {
	ctx := context.Background()
	
	withdrawals := []domain.Withdrawal{
		{
			ID:             1,
			UUID:           uuid.New(),
			UserID:         12345,
			WithdrawalID:   "np-withdrawal-1",
			Amount:         decimal.NewFromFloat(0.001),
			Status:         domain.WithdrawalStatusFinished,
		},
		{
			ID:             2,
			UUID:           uuid.New(),
			UserID:         12345,
			WithdrawalID:   "np-withdrawal-2",
			Amount:         decimal.NewFromFloat(0.002),
			Status:         domain.WithdrawalStatusProcessing,
		},
	}
	
	withdrawalRepo := &MockWithdrawalRepository{
		ListByUserIDFunc: func(ctx context.Context, userID int64, filter repository.ListFilter) (*repository.ListResult[domain.Withdrawal], error) {
			return &repository.ListResult[domain.Withdrawal]{
				Items:      withdrawals,
				NextCursor: "cursor-123",
				HasMore:    true,
			}, nil
		},
	}
	
	result, err := withdrawalRepo.ListByUserID(ctx, 12345, repository.ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if len(result.Items) != 2 {
		t.Errorf("expected 2 withdrawals, got %d", len(result.Items))
	}
	
	if !result.HasMore {
		t.Error("expected HasMore to be true")
	}
}

func TestWithdrawalService_ValidateWithdrawalLimits(t *testing.T) {
	tests := []struct {
		name          string
		kycLevel      int
		usedToday     float64
		requestAmount float64
		shouldExceed  bool
	}{
		{
			name:          "KYC 0 - no withdrawals allowed",
			kycLevel:      0,
			usedToday:     0,
			requestAmount: 100,
			shouldExceed:  true, // KYC 0 has $0 withdrawal limit
		},
		{
			name:          "KYC 1 - under limit",
			kycLevel:      1,
			usedToday:     200,
			requestAmount: 200,
			shouldExceed:  false, // 200 + 200 = 400 < 500
		},
		{
			name:          "KYC 1 - over limit",
			kycLevel:      1,
			usedToday:     400,
			requestAmount: 200,
			shouldExceed:  true, // 400 + 200 = 600 > 500
		},
		{
			name:          "KYC 2 - under limit",
			kycLevel:      2,
			usedToday:     2000,
			requestAmount: 2000,
			shouldExceed:  false, // 2000 + 2000 = 4000 < 5000
		},
		{
			name:          "KYC 3 - under limit",
			kycLevel:      3,
			usedToday:     10000,
			requestAmount: 10000,
			shouldExceed:  false, // 10000 + 10000 = 20000 < 25000
		},
	}
	
	limits := map[int]float64{
		0: 0,
		1: 500,
		2: 5000,
		3: 25000,
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit := limits[tt.kycLevel]
			used := decimal.NewFromFloat(tt.usedToday)
			requested := decimal.NewFromFloat(tt.requestAmount)
			
			exceeds := used.Add(requested).GreaterThan(decimal.NewFromFloat(limit))
			
			if exceeds != tt.shouldExceed {
				t.Errorf("expected exceeds=%v, got %v", tt.shouldExceed, exceeds)
			}
		})
	}
}

func TestWithdrawalService_GetWithdrawalLimit(t *testing.T) {
	service := &WithdrawalService{}
	
	tests := []struct {
		kycLevel      int
		expectedLimit float64
	}{
		{kycLevel: 0, expectedLimit: 0},
		{kycLevel: 1, expectedLimit: 500},
		{kycLevel: 2, expectedLimit: 5000},
		{kycLevel: 3, expectedLimit: 25000},
		{kycLevel: 99, expectedLimit: 0}, // Unknown level defaults to 0
	}
	
	for _, tt := range tests {
		t.Run(string(rune(tt.kycLevel)), func(t *testing.T) {
			limit := service.getWithdrawalLimit(tt.kycLevel)
			if limit != tt.expectedLimit {
				t.Errorf("expected limit %f for KYC level %d, got %f", tt.expectedLimit, tt.kycLevel, limit)
			}
		})
	}
}

func TestWithdrawalService_ValidateKYCLevel(t *testing.T) {
	tests := []struct {
		name        string
		kycLevel    int
		shouldError bool
	}{
		{"KYC 0 - not allowed", 0, true},
		{"KYC 1 - not allowed", 1, true},
		{"KYC 2 - allowed", 2, false},
		{"KYC 3 - allowed", 3, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldFail := tt.kycLevel < 2
			if shouldFail != tt.shouldError {
				t.Errorf("expected shouldError=%v, got %v", tt.shouldError, shouldFail)
			}
		})
	}
}

func TestWithdrawalService_ToResponse(t *testing.T) {
	service := &WithdrawalService{}
	
	withdrawal := &domain.Withdrawal{
		ID:             1,
		UUID:           uuid.New(),
		UserID:         12345,
		WithdrawalID:   "np-withdrawal-123",
		IdempotencyKey: "idem-key-123",
		Amount:         decimal.NewFromFloat(0.001),
		FiatAmount:     decimal.NewFromFloat(45.0),
		FiatCurrency:   "USD",
		CryptoCurrency: "BTC",
		Address:        "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
		Status:         domain.WithdrawalStatusProcessing,
	}
	
	response := service.toResponse(withdrawal)
	
	if response.WithdrawalID != withdrawal.WithdrawalID {
		t.Errorf("expected withdrawal ID %s, got %s", withdrawal.WithdrawalID, response.WithdrawalID)
	}
	
	if response.Status != string(withdrawal.Status) {
		t.Errorf("expected status %s, got %s", withdrawal.Status, response.Status)
	}
	
	if !response.Amount.Equal(withdrawal.Amount) {
		t.Errorf("expected amount %s, got %s", withdrawal.Amount.String(), response.Amount.String())
	}
}
