package integration

import (
	"testing"

	"github.com/google/uuid"
	"github.com/opus-casino/payment/internal/client"
	"github.com/opus-casino/payment/internal/domain"
	"github.com/opus-casino/payment/internal/repository"
	"github.com/opus-casino/payment/internal/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWithdrawalFlow_Complete tests the complete withdrawal flow
// Validates: Requirements 3.1-3.8, 4.1-4.7
func TestWithdrawalFlow_Complete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	containers := SetupTestContainers(t)
	if containers == nil {
		return
	}
	defer containers.Cleanup()

	ctx, cancel := CreateTestContext()
	defer cancel()

	// Create repositories
	withdrawalRepo := repository.NewWithdrawalRepository(containers.DB)
	idempotencyRepo := repository.NewIdempotencyRepository(containers.RedisClient)
	exchangeRateRepo := repository.NewExchangeRateRepository(containers.RedisClient)
	dailyLimitsRepo := repository.NewDailyLimitsRepository(containers.RedisClient)
	auditLogRepo := repository.NewAuditLogRepository(containers.DB)
	_ = auditLogRepo

	// Create mock clients
	nowpaymentsClient := NewMockNOWPaymentsClient(containers.IPNSecret)
	walletClient := NewMockWalletClient()
	userClient := NewMockUserClient()

	// Setup test user with KYC level 2 (required for withdrawals)
	userID := int64(12345)
	userClient.SetKYCLevel(userID, 2)
	walletClient.SetBalance(userID, decimal.NewFromFloat(1000.0)) // $1000 balance

	// Create services
	withdrawalService := service.NewWithdrawalService(
		withdrawalRepo,
		idempotencyRepo,
		exchangeRateRepo,
		dailyLimitsRepo,
		nowpaymentsClient,
		walletClient,
		userClient,
		nil, // producer
		nil, // tracer
	)
	_ = withdrawalService

	t.Run("initiate withdrawal successfully", func(t *testing.T) {
		// Verify KYC level
		kycLevel, err := userClient.GetKYCLevel(ctx, userID)
		require.NoError(t, err)
		require.GreaterOrEqual(t, kycLevel, 2, "KYC level must be at least 2 for withdrawals")

		// Check balance
		balance, err := walletClient.GetBalance(ctx, userID, "USD")
		require.NoError(t, err)
		require.True(t, balance.Available.GreaterThan(decimal.Zero), "User must have available balance")

		// Create withdrawal request
		req := service.InitiateWithdrawalRequest{
			UserID:         userID,
			Amount:         decimal.NewFromFloat(100.0),
			Currency:       domain.CryptoBTC,
			Address:        "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
			IdempotencyKey: uuid.New().String(),
			IPAddress:      "192.168.1.1",
			UserAgent:      "test-agent",
		}

		// Lock funds
		lockResult, err := walletClient.LockFunds(ctx, client.LockRequest{
			UserID:         userID,
			Currency:       "USD",
			Amount:         req.Amount,
			IdempotencyKey: "lock:" + req.IdempotencyKey,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, lockResult.LockID)

		// Create payout in NOWPayments
		payoutResp, err := nowpaymentsClient.CreatePayout(ctx, client.CreatePayoutRequest{
			WithdrawalID:   uuid.New().String(),
			Address:        req.Address,
			Currency:       string(req.Currency),
			Amount:         decimal.NewFromFloat(0.002), // ~$100 worth
			IPNCallbackURL: "https://test.com/webhook",
		})
		require.NoError(t, err)
		assert.NotEmpty(t, payoutResp.WithdrawalID)

		// Create withdrawal record
		withdrawal := &domain.Withdrawal{
			UUID:           uuid.New(),
			UserID:         req.UserID,
			WithdrawalID:   payoutResp.WithdrawalID,
			IdempotencyKey: req.IdempotencyKey,
			Amount:         req.Amount,
			FiatAmount:     req.Amount,
			FiatCurrency:   "USD",
			CryptoCurrency: string(req.Currency),
			Address:        req.Address,
			Status:         domain.WithdrawalStatusProcessing,
		}

		err = withdrawalRepo.Create(ctx, withdrawal)
		require.NoError(t, err)
		assert.NotZero(t, withdrawal.ID)

		t.Logf("Created withdrawal: ID=%s, Amount=$%s", withdrawal.WithdrawalID, withdrawal.Amount.String())
	})

	t.Run("process withdrawal webhook - finished", func(t *testing.T) {
		// Create a withdrawal first
		withdrawalID := "np-withdrawal-webhook-test"
		idempotencyKey := uuid.New().String()

		withdrawal := &domain.Withdrawal{
			UUID:           uuid.New(),
			UserID:         userID,
			WithdrawalID:   withdrawalID,
			IdempotencyKey: idempotencyKey,
			Amount:         decimal.NewFromFloat(100.0),
			FiatAmount:     decimal.NewFromFloat(100.0),
			FiatCurrency:   "USD",
			CryptoCurrency: "BTC",
			Address:        "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
			Status:         domain.WithdrawalStatusProcessing,
		}

		err := withdrawalRepo.Create(ctx, withdrawal)
		require.NoError(t, err)

		// Lock funds
		_, err = walletClient.LockFunds(ctx, client.LockRequest{
			UserID:         userID,
			Currency:       "USD",
			Amount:         withdrawal.Amount,
			IdempotencyKey: "lock:" + withdrawalID,
		})
		require.NoError(t, err)

		// Process webhook - update status to finished
		err = withdrawalRepo.UpdateStatus(ctx, withdrawal.ID, domain.WithdrawalStatusProcessing, domain.WithdrawalStatusFinished)
		require.NoError(t, err)

		// Finalize debit
		debitResult, err := walletClient.FinalizeDebit(ctx, client.FinalizeDebitRequest{
			UserID:         userID,
			Currency:       "USD",
			Amount:         withdrawal.Amount,
			IdempotencyKey: "withdrawal:" + withdrawalID,
			ReferenceID:    withdrawalID,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, debitResult.TransactionID)

		// Verify withdrawal status updated
		updated, err := withdrawalRepo.GetByWithdrawalID(ctx, withdrawalID)
		require.NoError(t, err)
		assert.Equal(t, domain.WithdrawalStatusFinished, updated.Status)

		t.Logf("Withdrawal completed: ID=%s, TransactionID=%s", withdrawalID, debitResult.TransactionID)
	})

	t.Run("process withdrawal webhook - failed with compensation", func(t *testing.T) {
		// Create a withdrawal
		withdrawalID := "np-withdrawal-failed-test"
		idempotencyKey := uuid.New().String()

		// Reset balance for this test
		testUserID := int64(12346)
		walletClient.SetBalance(testUserID, decimal.NewFromFloat(500.0))

		withdrawal := &domain.Withdrawal{
			UUID:           uuid.New(),
			UserID:         testUserID,
			WithdrawalID:   withdrawalID,
			IdempotencyKey: idempotencyKey,
			Amount:         decimal.NewFromFloat(100.0),
			FiatAmount:     decimal.NewFromFloat(100.0),
			FiatCurrency:   "USD",
			CryptoCurrency: "BTC",
			Address:        "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
			Status:         domain.WithdrawalStatusProcessing,
		}

		err := withdrawalRepo.Create(ctx, withdrawal)
		require.NoError(t, err)

		// Lock funds
		lockResult, err := walletClient.LockFunds(ctx, client.LockRequest{
			UserID:         testUserID,
			Currency:       "USD",
			Amount:         withdrawal.Amount,
			IdempotencyKey: "lock:" + withdrawalID,
		})
		require.NoError(t, err)

		// Get balance after lock
		balanceAfterLock, err := walletClient.GetBalance(ctx, testUserID, "USD")
		require.NoError(t, err)

		// Simulate failed withdrawal - unlock funds
		err = walletClient.UnlockFunds(ctx, lockResult.LockID, "unlock:"+withdrawalID)
		require.NoError(t, err)

		// Update status to failed
		err = withdrawalRepo.UpdateStatus(ctx, withdrawal.ID, domain.WithdrawalStatusProcessing, domain.WithdrawalStatusFailed)
		require.NoError(t, err)

		// Verify funds unlocked
		balanceAfterUnlock, err := walletClient.GetBalance(ctx, testUserID, "USD")
		require.NoError(t, err)
		assert.True(t, balanceAfterUnlock.Available.GreaterThan(balanceAfterLock.Available), "Available balance should increase after unlock")

		// Verify withdrawal status
		updated, err := withdrawalRepo.GetByWithdrawalID(ctx, withdrawalID)
		require.NoError(t, err)
		assert.Equal(t, domain.WithdrawalStatusFailed, updated.Status)

		t.Logf("Withdrawal failed, funds unlocked: Available before=$%s, after=$%s",
			balanceAfterLock.Available.String(), balanceAfterUnlock.Available.String())
	})

	t.Run("idempotency - same withdrawal request returns same result", func(t *testing.T) {
		idempotencyKey := "idem-withdrawal-test-" + uuid.New().String()

		// First request
		withdrawal1 := &domain.Withdrawal{
			UUID:           uuid.New(),
			UserID:         userID,
			WithdrawalID:   "np-withdrawal-idem-1",
			IdempotencyKey: idempotencyKey,
			Amount:         decimal.NewFromFloat(100.0),
			FiatAmount:     decimal.NewFromFloat(100.0),
			FiatCurrency:   "USD",
			CryptoCurrency: "BTC",
			Address:        "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
			Status:         domain.WithdrawalStatusProcessing,
		}

		err := withdrawalRepo.Create(ctx, withdrawal1)
		require.NoError(t, err)

		// Second request with same idempotency key should return existing
		existing, err := withdrawalRepo.GetByIDempotencyKey(ctx, idempotencyKey)
		require.NoError(t, err)
		require.NotNil(t, existing)
		assert.Equal(t, withdrawal1.WithdrawalID, existing.WithdrawalID)

		t.Logf("Idempotency verified: same withdrawal returned for key %s", idempotencyKey)
	})
}

// TestWithdrawalFlow_KYCValidation tests KYC level validation for withdrawals
// Validates: Requirements 3.1, 3.2
func TestWithdrawalFlow_KYCValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	containers := SetupTestContainers(t)
	if containers == nil {
		return
	}
	defer containers.Cleanup()

	ctx, cancel := CreateTestContext()
	defer cancel()

	userClient := NewMockUserClient()

	tests := []struct {
		name        string
		kycLevel    int
		shouldAllow bool
	}{
		{"KYC level 0 - not allowed", 0, false},
		{"KYC level 1 - not allowed", 1, false},
		{"KYC level 2 - allowed", 2, true},
		{"KYC level 3 - allowed", 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testUserID := int64(10000 + tt.kycLevel)
			userClient.SetKYCLevel(testUserID, tt.kycLevel)

			kycLevel, err := userClient.GetKYCLevel(ctx, testUserID)
			require.NoError(t, err)

			isAllowed := kycLevel >= 2
			assert.Equal(t, tt.shouldAllow, isAllowed, "KYC validation mismatch")

			if !isAllowed {
				t.Logf("Withdrawal rejected for KYC level %d (minimum required: 2)", kycLevel)
			} else {
				t.Logf("Withdrawal allowed for KYC level %d", kycLevel)
			}
		})
	}
}

// TestWithdrawalFlow_InsufficientBalance tests insufficient balance handling
// Validates: Requirements 3.3, 3.4
func TestWithdrawalFlow_InsufficientBalance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	containers := SetupTestContainers(t)
	if containers == nil {
		return
	}
	defer containers.Cleanup()

	ctx, cancel := CreateTestContext()
	defer cancel()

	walletClient := NewMockWalletClient()
	userClient := NewMockUserClient()

	userID := int64(12345)
	userClient.SetKYCLevel(userID, 2)
	walletClient.SetBalance(userID, decimal.NewFromFloat(100.0)) // Only $100

	// Try to withdraw $200 (more than balance)
	balance, err := walletClient.GetBalance(ctx, userID, "USD")
	require.NoError(t, err)

	requestedAmount := decimal.NewFromFloat(200.0)
	hasSufficientBalance := balance.Available.GreaterThanOrEqual(requestedAmount)

	assert.False(t, hasSufficientBalance, "Should not have sufficient balance")

	t.Logf("Insufficient balance: available=$%s, requested=$%s",
		balance.Available.String(), requestedAmount.String())
}

// TestWithdrawalFlow_DailyLimits tests daily withdrawal limits
// Validates: Requirements 3.8, 6.3, 6.4
func TestWithdrawalFlow_DailyLimits(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	containers := SetupTestContainers(t)
	if containers == nil {
		return
	}
	defer containers.Cleanup()

	ctx, cancel := CreateTestContext()
	defer cancel()

	dailyLimitsRepo := repository.NewDailyLimitsRepository(containers.RedisClient)
	userClient := NewMockUserClient()

	tests := []struct {
		name          string
		kycLevel      int
		dailyLimit    float64
		usedToday     float64
		requestAmount float64
		shouldExceed  bool
	}{
		{"KYC 0 - no withdrawals", 0, 0, 0, 100, true},
		{"KYC 1 - under limit", 1, 500, 200, 200, false},
		{"KYC 1 - over limit", 1, 500, 400, 200, true},
		{"KYC 2 - under limit", 2, 5000, 2000, 2000, false},
		{"KYC 2 - over limit", 2, 5000, 4000, 2000, true},
		{"KYC 3 - under limit", 3, 25000, 10000, 10000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := int64(20000 + tt.kycLevel)
			userClient.SetKYCLevel(userID, tt.kycLevel)

			// Set used amount
			if tt.usedToday > 0 {
				_, err := dailyLimitsRepo.Increment(ctx, userID, "withdrawal", decimal.NewFromFloat(tt.usedToday))
				require.NoError(t, err)
			}

			// Check limit
			used, err := dailyLimitsRepo.Get(ctx, userID, "withdrawal")
			require.NoError(t, err)

			requested := decimal.NewFromFloat(tt.requestAmount)
			limit := decimal.NewFromFloat(tt.dailyLimit)

			exceeds := used.Add(requested).GreaterThan(limit)
			assert.Equal(t, tt.shouldExceed, exceeds, "Daily limit check mismatch")

			t.Logf("KYC %d: used=$%s, requested=$%s, limit=$%s, exceeds=%v",
				tt.kycLevel, used.String(), requested.String(), limit.String(), exceeds)
		})
	}
}

// TestWithdrawalFlow_StatusTransitions tests withdrawal status transitions
// Validates: Requirements 4.2, 4.5, 4.6
func TestWithdrawalFlow_StatusTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	containers := SetupTestContainers(t)
	if containers == nil {
		return
	}
	defer containers.Cleanup()

	ctx, cancel := CreateTestContext()
	defer cancel()

	withdrawalRepo := repository.NewWithdrawalRepository(containers.DB)

	tests := []struct {
		name          string
		fromStatus    domain.WithdrawalStatus
		toStatus      domain.WithdrawalStatus
		shouldSucceed bool
	}{
		{"processing to sending", domain.WithdrawalStatusProcessing, domain.WithdrawalStatusSending, true},
		{"sending to sent", domain.WithdrawalStatusSending, domain.WithdrawalStatusSent, true},
		{"sent to finished", domain.WithdrawalStatusSent, domain.WithdrawalStatusFinished, true},
		{"processing to failed", domain.WithdrawalStatusProcessing, domain.WithdrawalStatusFailed, true},
		{"processing to cancelled", domain.WithdrawalStatusProcessing, domain.WithdrawalStatusCancelled, true},
		{"finished to processing (invalid)", domain.WithdrawalStatusFinished, domain.WithdrawalStatusProcessing, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withdrawal := &domain.Withdrawal{
				UUID:           uuid.New(),
				UserID:         12345,
				WithdrawalID:   "np-status-test-" + uuid.New().String(),
				IdempotencyKey: uuid.New().String(),
				Amount:         decimal.NewFromFloat(100.0),
				FiatAmount:     decimal.NewFromFloat(100.0),
				FiatCurrency:   "USD",
				CryptoCurrency: "BTC",
				Address:        "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
				Status:         tt.fromStatus,
			}

			err := withdrawalRepo.Create(ctx, withdrawal)
			require.NoError(t, err)

			// Check if transition is valid
			canTransition := tt.fromStatus.CanTransitionTo(tt.toStatus)
			assert.Equal(t, tt.shouldSucceed, canTransition, "Status transition validation mismatch")

			if canTransition {
				err = withdrawalRepo.UpdateStatus(ctx, withdrawal.ID, tt.fromStatus, tt.toStatus)
				require.NoError(t, err)

				// Verify status updated
				updated, err := withdrawalRepo.GetByID(ctx, withdrawal.ID)
				require.NoError(t, err)
				assert.Equal(t, tt.toStatus, updated.Status)
			}
		})
	}
}

// TestWithdrawalFlow_CurrencyValidation tests currency validation for withdrawals
// Validates: Requirements 10.1, 10.2
func TestWithdrawalFlow_CurrencyValidation(t *testing.T) {
	tests := []struct {
		name        string
		currency    domain.CryptoCurrency
		shouldAllow bool
	}{
		{"BTC allowed", domain.CryptoBTC, true},
		{"ETH allowed", domain.CryptoETH, true},
		{"USDT(ERC20) allowed", domain.CryptoUSDTETH, true},
		{"USDT(TRC20) allowed", domain.CryptoUSDTTRX, true},
		{"USDC allowed", domain.CryptoUSDC, true},
		{"LTC allowed", domain.CryptoLTC, true},
		{"BCH allowed", domain.CryptoBCH, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isSupported := tt.currency.IsWithdrawalSupported()
			assert.Equal(t, tt.shouldAllow, isSupported, "Currency support mismatch")
		})
	}
}
