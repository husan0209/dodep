package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opus-casino/payment/internal/client"
	"github.com/opus-casino/payment/internal/domain"
	"github.com/opus-casino/payment/internal/repository"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestContainers holds the test container resources
type TestContainers struct {
	DB            *gorm.DB
	RedisClient   *redis.Client
	Cleanup       func()
	IPNSecret     string
	APIKey        string
}

// SetupTestContainers initializes PostgreSQL and Redis containers for testing
func SetupTestContainers(t *testing.T) *TestContainers {
	t.Helper()
	ctx := context.Background()

	// For now, we use local connections (in CI, these would be testcontainers)
	// This allows tests to run with docker-compose or local services

	dbDSN := "host=localhost user=postgres password=postgres dbname=payment_test port=5432 sslmode=disable"
	redisAddr := "localhost:6379"

	// Try to connect to PostgreSQL
	db, err := gorm.Open(postgres.Open(dbDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Skipf("PostgreSQL not available, skipping integration test: %v", err)
		return nil
	}

	// Run migrations
	if err := runMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Connect to Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "",
		DB:       1, // Use separate DB for tests
	})

	// Test Redis connection
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available, skipping integration test: %v", err)
		return nil
	}

	cleanup := func() {
		// Clean up test data
		db.Exec("TRUNCATE payments, withdrawals, payment_audit_logs CASCADE")
		redisClient.FlushDB(ctx)
	}

	return &TestContainers{
		DB:          db,
		RedisClient: redisClient,
		Cleanup:     cleanup,
		IPNSecret:   "test-ipn-secret-key",
		APIKey:      "test-api-key",
	}
}

// runMigrations runs database migrations
func runMigrations(db *gorm.DB) error {
	// Create enum types
	if err := db.Exec(`
		DO $$ BEGIN
			CREATE TYPE payment_status AS ENUM (
				'pending', 'waiting', 'confirming', 'confirmed', 
				'sending', 'partially_paid', 'finished', 'failed', 
				'expired', 'refunded'
			);
		EXCEPTION
			WHEN duplicate_object THEN null;
		END $$;
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		DO $$ BEGIN
			CREATE TYPE withdrawal_status AS ENUM (
				'processing', 'sending', 'sent', 'finished', 'failed', 'cancelled'
			);
		EXCEPTION
			WHEN duplicate_object THEN null;
		END $$;
	`).Error; err != nil {
		return err
	}

	// Auto migrate
	if err := db.AutoMigrate(&domain.Payment{}, &domain.Withdrawal{}, &repository.AuditLog{}); err != nil {
		return err
	}

	return nil
}

// MockNOWPaymentsClient is a mock implementation for integration tests
type MockNOWPaymentsClient struct {
	payments    map[string]*client.CreatePaymentResponse
	payouts     map[string]*client.CreatePayoutResponse
	ipnSecret   string
	paymentID   int
	payoutID    int
}

// NewMockNOWPaymentsClient creates a new mock client
func NewMockNOWPaymentsClient(ipnSecret string) *MockNOWPaymentsClient {
	return &MockNOWPaymentsClient{
		payments:  make(map[string]*client.CreatePaymentResponse),
		payouts:   make(map[string]*client.CreatePayoutResponse),
		ipnSecret: ipnSecret,
	}
}

// CreatePayment creates a mock payment
func (m *MockNOWPaymentsClient) CreatePayment(ctx context.Context, req client.CreatePaymentRequest) (*client.CreatePaymentResponse, error) {
	m.paymentID++
	paymentID := fmt.Sprintf("np-payment-%d", m.paymentID)
	
	resp := &client.CreatePaymentResponse{
		PaymentID:     paymentID,
		PaymentStatus: "waiting",
		PayAddress:    "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
		PayAmount:     decimal.NewFromFloat(0.001),
		PayCurrency:   req.PayCurrency,
		PriceAmount:   req.PriceAmount,
		PriceCurrency: req.PriceCurrency,
		CreatedAt:     time.Now(),
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	}
	
	m.payments[paymentID] = resp
	return resp, nil
}

// CreatePayout creates a mock payout
func (m *MockNOWPaymentsClient) CreatePayout(ctx context.Context, req client.CreatePayoutRequest) (*client.CreatePayoutResponse, error) {
	m.payoutID++
	withdrawalID := fmt.Sprintf("np-withdrawal-%d", m.payoutID)
	
	resp := &client.CreatePayoutResponse{
		WithdrawalID: withdrawalID,
		Status:       "processing",
		Amount:       req.Amount,
		Currency:     req.Currency,
		Address:      req.Address,
	}
	
	m.payouts[withdrawalID] = resp
	return resp, nil
}

// GetEstimatedPrice returns a mock exchange rate
func (m *MockNOWPaymentsClient) GetEstimatedPrice(ctx context.Context, amount decimal.Decimal, fromCurrency, toCurrency string) (*client.EstimatedPriceResponse, error) {
	// Mock BTC/USD rate
	rate := decimal.NewFromFloat(45000.0)
	return &client.EstimatedPriceResponse{
		EstimatedAmount: rate,
		CurrencyFrom:    fromCurrency,
		CurrencyTo:      toCurrency,
	}, nil
}

// GetCurrencies returns mock supported currencies
func (m *MockNOWPaymentsClient) GetCurrencies(ctx context.Context) (*client.CurrenciesResponse, error) {
	return &client.CurrenciesResponse{
		Currencies: []string{"BTC", "ETH", "USDT", "USDC", "LTC", "BCH"},
	}, nil
}

// VerifyWebhookSignature verifies webhook signature
func (m *MockNOWPaymentsClient) VerifyWebhookSignature(payload []byte, signature string) bool {
	// For testing, accept "valid-signature" as valid
	return signature == "valid-signature" || signature == ""
}

// MockWalletClient is a mock implementation for integration tests
type MockWalletClient struct {
	balances map[int64]decimal.Decimal
	locks    map[string]decimal.Decimal
}

// NewMockWalletClient creates a new mock wallet client
func NewMockWalletClient() *MockWalletClient {
	return &MockWalletClient{
		balances: make(map[int64]decimal.Decimal),
		locks:    make(map[string]decimal.Decimal),
	}
}

// SetBalance sets a user's balance for testing
func (m *MockWalletClient) SetBalance(userID int64, balance decimal.Decimal) {
	m.balances[userID] = balance
}

// GetBalance returns user's balance
func (m *MockWalletClient) GetBalance(ctx context.Context, userID int64, currency string) (*client.Balance, error) {
	balance, ok := m.balances[userID]
	if !ok {
		balance = decimal.Zero
	}
	
	locked := decimal.Zero
	if lockAmount, ok := m.locks[fmt.Sprintf("user:%d", userID)]; ok {
		locked = lockAmount
	}
	
	return &client.Balance{
		Available: balance.Sub(locked),
		Locked:    locked,
		Total:     balance,
	}, nil
}

// CreditWallet credits user's wallet
func (m *MockWalletClient) CreditWallet(ctx context.Context, req client.CreditRequest) (*client.CreditResult, error) {
	balance := m.balances[req.UserID]
	newBalance := balance.Add(req.Amount)
	m.balances[req.UserID] = newBalance
	
	return &client.CreditResult{
		TransactionID: uuid.New().String(),
		NewBalance:    newBalance,
	}, nil
}

// LockFunds locks funds for withdrawal
func (m *MockWalletClient) LockFunds(ctx context.Context, req client.LockRequest) (*client.LockResult, error) {
	balance := m.balances[req.UserID]
	if balance.LessThan(req.Amount) {
		return nil, fmt.Errorf("insufficient balance")
	}
	
	m.locks[fmt.Sprintf("user:%d", req.UserID)] = req.Amount
	m.locks[req.IdempotencyKey] = req.Amount
	
	return &client.LockResult{
		LockID:     req.IdempotencyKey,
		NewBalance: balance.Sub(req.Amount),
	}, nil
}

// UnlockFunds unlocks funds after failed withdrawal
func (m *MockWalletClient) UnlockFunds(ctx context.Context, lockID string, idempotencyKey string) error {
	delete(m.locks, lockID)
	delete(m.locks, idempotencyKey)
	return nil
}

// FinalizeDebit finalizes withdrawal
func (m *MockWalletClient) FinalizeDebit(ctx context.Context, req client.FinalizeDebitRequest) (*client.DebitResult, error) {
	// Remove locked amount and deduct from balance
	lockKey := fmt.Sprintf("user:%d", req.UserID)
	if locked, ok := m.locks[lockKey]; ok {
		balance := m.balances[req.UserID]
		m.balances[req.UserID] = balance.Sub(locked)
		delete(m.locks, lockKey)
	}
	
	return &client.DebitResult{
		TransactionID: uuid.New().String(),
	}, nil
}

func (m *MockWalletClient) Close() error { return nil }

// MockUserClient is a mock implementation for integration tests
type MockUserClient struct {
	kycLevels map[int64]int
}

// NewMockUserClient creates a new mock user client
func NewMockUserClient() *MockUserClient {
	return &MockUserClient{
		kycLevels: make(map[int64]int),
	}
}

// SetKYCLevel sets a user's KYC level for testing
func (m *MockUserClient) SetKYCLevel(userID int64, level int) {
	m.kycLevels[userID] = level
}

// GetKYCLevel returns user's KYC level
func (m *MockUserClient) GetKYCLevel(ctx context.Context, userID int64) (int, error) {
	level, ok := m.kycLevels[userID]
	if !ok {
		return 0, nil // Default to level 0
	}
	return level, nil
}

// GetUserStatus returns user's status
func (m *MockUserClient) GetUserStatus(ctx context.Context, userID int64) (string, error) {
	return "active", nil
}

// GetUserInfo returns user info
func (m *MockUserClient) GetUserInfo(ctx context.Context, userID int64) (*client.UserInfo, error) {
	level, _ := m.kycLevels[userID]
	return &client.UserInfo{
		UserID:   userID,
		KYCLevel: level,
		Status:   "active",
	}, nil
}

func (m *MockUserClient) Close() error { return nil }

// TestServiceFactory creates services for testing
type TestServiceFactory struct {
	containers *TestContainers
	logger     *zap.Logger
}

// NewTestServiceFactory creates a new test service factory
func NewTestServiceFactory(containers *TestContainers) *TestServiceFactory {
	return &TestServiceFactory{
		containers: containers,
		logger:     zap.NewNop(),
	}
}

// CreateTestContext creates a context with timeout for tests
func CreateTestContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}
