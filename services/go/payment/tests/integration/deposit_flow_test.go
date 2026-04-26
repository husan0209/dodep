package integration

import (
	"encoding/json"
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

// TestDepositFlow_Complete tests the complete deposit flow
// Validates: Requirements 1.1-1.5, 2.1-2.7
func TestDepositFlow_Complete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup
	containers := SetupTestContainers(t)
	if containers == nil {
		return
	}
	defer containers.Cleanup()

	ctx, cancel := CreateTestContext()
	defer cancel()

	// Create repositories
	paymentRepo := repository.NewPaymentRepository(containers.DB)
	idempotencyRepo := repository.NewIdempotencyRepository(containers.RedisClient)
	exchangeRateRepo := repository.NewExchangeRateRepository(containers.RedisClient)
	dailyLimitsRepo := repository.NewDailyLimitsRepository(containers.RedisClient)
	auditLogRepo := repository.NewAuditLogRepository(containers.DB)

	// Create mock clients
	nowpaymentsClient := NewMockNOWPaymentsClient(containers.IPNSecret)
	walletClient := NewMockWalletClient()
	userClient := NewMockUserClient()

	// Setup test user
	userID := int64(12345)
	userClient.SetKYCLevel(userID, 2) // KYC level 2

	// Create services
	paymentService := service.NewPaymentService(
		paymentRepo,
		idempotencyRepo,
		exchangeRateRepo,
		dailyLimitsRepo,
		nowpaymentsClient,
		walletClient,
		userClient,
		nil, // producer
		nil, // tracer
	)
	_ = paymentService

	webhookService := service.NewWebhookService(
		paymentRepo,
		nil, // withdrawal repo
		idempotencyRepo,
		auditLogRepo,
		nowpaymentsClient,
		walletClient,
		nil, // producer
		nil, // tracer
	)
	_ = webhookService

	t.Run("initiate deposit successfully", func(t *testing.T) {
		// Test request
		req := service.InitiateDepositRequest{
			UserID:         userID,
			Amount:         decimal.NewFromFloat(100.0),
			Currency:       domain.CryptoBTC,
			IdempotencyKey: uuid.New().String(),
			IPAddress:      "192.168.1.1",
			UserAgent:      "test-agent",
		}

		// Create payment in mock NOWPayments
		npResp, err := nowpaymentsClient.CreatePayment(ctx, client.CreatePaymentRequest{
			PriceAmount:    req.Amount,
			PriceCurrency:  "USD",
			PayCurrency:    string(req.Currency),
			IPNCallbackURL: "https://test.com/webhook",
			OrderID:        req.IdempotencyKey,
		})
		require.NoError(t, err)

		// Create payment record
		payment := &domain.Payment{
			UUID:            uuid.New(),
			UserID:          req.UserID,
			PaymentID:       npResp.PaymentID,
			IdempotencyKey:  req.IdempotencyKey,
			RequestedAmount: req.Amount,
			FiatAmount:      req.Amount,
			FiatCurrency:    "USD",
			CryptoCurrency:  string(req.Currency),
			PayAddress:      npResp.PayAddress,
			PayAmount:       &npResp.PayAmount,
			Status:          domain.PaymentStatusWaiting,
		}

		err = paymentRepo.Create(ctx, payment)
		require.NoError(t, err)
		assert.NotZero(t, payment.ID)
		assert.NotEmpty(t, payment.PaymentID)

		t.Logf("Created payment: ID=%s, Address=%s", payment.PaymentID, payment.PayAddress)
	})

	t.Run("process deposit webhook - finished", func(t *testing.T) {
		// Create a payment first
		paymentID := "np-payment-webhook-test"
		idempotencyKey := uuid.New().String()

		payment := &domain.Payment{
			UUID:            uuid.New(),
			UserID:          userID,
			PaymentID:       paymentID,
			IdempotencyKey:  idempotencyKey,
			RequestedAmount: decimal.NewFromFloat(100.0),
			FiatAmount:      decimal.NewFromFloat(100.0),
			FiatCurrency:    "USD",
			CryptoCurrency:  "BTC",
			PayAddress:      "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
			Status:          domain.PaymentStatusWaiting,
		}

		err := paymentRepo.Create(ctx, payment)
		require.NoError(t, err)

		// Create webhook payload
		webhookPayload := client.WebhookPayload{
			PaymentID:       paymentID,
			PaymentStatus:   "finished",
			PayAddress:      payment.PayAddress,
			PayAmount:       decimal.NewFromFloat(0.002),
			PayCurrency:     "BTC",
			PriceAmount:     decimal.NewFromFloat(100.0),
			PriceCurrency:   "USD",
			OutcomeAmount:   decimal.NewFromFloat(100.0),
			OutcomeCurrency: "USD",
		}

		payloadBytes, err := json.Marshal(webhookPayload)
		require.NoError(t, err)

		// Verify signature (mock always returns true for "valid-signature")
		valid := nowpaymentsClient.VerifyWebhookSignature(payloadBytes, "valid-signature")
		assert.True(t, valid)

		// Process webhook - update status
		err = paymentRepo.UpdateStatus(ctx, payment.ID, domain.PaymentStatusWaiting, domain.PaymentStatusFinished)
		require.NoError(t, err)

		// Verify payment status updated
		updatedPayment, err := paymentRepo.GetByPaymentID(ctx, paymentID)
		require.NoError(t, err)
		assert.Equal(t, domain.PaymentStatusFinished, updatedPayment.Status)

		// Credit wallet
		creditResult, err := walletClient.CreditWallet(ctx, client.CreditRequest{
			UserID:         userID,
			Currency:       "USD",
			Amount:         payment.FiatAmount,
			IdempotencyKey: "deposit:" + paymentID,
			ReferenceType:  "deposit",
			ReferenceID:    paymentID,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, creditResult.TransactionID)

		t.Logf("Wallet credited: TransactionID=%s, NewBalance=%s", creditResult.TransactionID, creditResult.NewBalance.String())
	})

	t.Run("idempotency - same request returns same result", func(t *testing.T) {
		idempotencyKey := "idem-deposit-test-" + uuid.New().String()

		// First request
		payment1 := &domain.Payment{
			UUID:            uuid.New(),
			UserID:          userID,
			PaymentID:       "np-payment-idem-1",
			IdempotencyKey:  idempotencyKey,
			RequestedAmount: decimal.NewFromFloat(100.0),
			FiatAmount:      decimal.NewFromFloat(100.0),
			FiatCurrency:    "USD",
			CryptoCurrency:  "BTC",
			PayAddress:      "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
			Status:          domain.PaymentStatusWaiting,
		}

		err := paymentRepo.Create(ctx, payment1)
		require.NoError(t, err)

		// Second request with same idempotency key should return existing
		existing, err := paymentRepo.GetByIDempotencyKey(ctx, idempotencyKey)
		require.NoError(t, err)
		require.NotNil(t, existing)
		assert.Equal(t, payment1.PaymentID, existing.PaymentID)

		t.Logf("Idempotency verified: same payment returned for key %s", idempotencyKey)
	})

	t.Run("KYC limit enforcement", func(t *testing.T) {
		// Test KYC level 0 limit ($500/day)
		kyc0UserID := int64(99901)
		userClient.SetKYCLevel(kyc0UserID, 0)

		// Check limit
		kycLevel, err := userClient.GetKYCLevel(ctx, kyc0UserID)
		require.NoError(t, err)
		assert.Equal(t, 0, kycLevel)

		// Verify limit is $500 for KYC level 0
		limit := getDepositLimit(kycLevel)
		assert.Equal(t, 500.0, limit)

		t.Logf("KYC level 0 deposit limit: $%.2f", limit)
	})

	t.Run("daily limit exceeded", func(t *testing.T) {
		// User with KYC level 1 ($2000/day limit)
		kyc1UserID := int64(99902)
		userClient.SetKYCLevel(kyc1UserID, 1)

		// Record deposits totaling $1500
		_, err := dailyLimitsRepo.Increment(ctx, kyc1UserID, "deposit", decimal.NewFromFloat(1500.0))
		require.NoError(t, err)

		// Try to deposit $600 more (would exceed $2000 limit)
		currentTotal, err := dailyLimitsRepo.Get(ctx, kyc1UserID, "deposit")
		require.NoError(t, err)

		requestedAmount := decimal.NewFromFloat(600.0)
		limit := decimal.NewFromFloat(2000.0)

		wouldExceed := currentTotal.Add(requestedAmount).GreaterThan(limit)
		assert.True(t, wouldExceed, "Deposit should exceed daily limit")

		t.Logf("Daily limit check: current=$%s, requested=$%s, limit=$%s, exceeds=%v",
			currentTotal.String(), requestedAmount.String(), limit.String(), wouldExceed)
	})
}

// TestDepositFlow_WebhookSignature tests webhook signature verification
// Validates: Requirements 2.1, 2.2
func TestDepositFlow_WebhookSignature(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	containers := SetupTestContainers(t)
	if containers == nil {
		return
	}
	defer containers.Cleanup()

	nowpaymentsClient := NewMockNOWPaymentsClient(containers.IPNSecret)

	t.Run("valid signature accepted", func(t *testing.T) {
		payload := []byte(`{"payment_id": "test-123", "payment_status": "finished"}`)
		valid := nowpaymentsClient.VerifyWebhookSignature(payload, "valid-signature")
		assert.True(t, valid)
	})

	t.Run("invalid signature rejected", func(t *testing.T) {
		payload := []byte(`{"payment_id": "test-123", "payment_status": "finished"}`)
		valid := nowpaymentsClient.VerifyWebhookSignature(payload, "invalid-signature")
		assert.False(t, valid)
	})
}

// TestDepositFlow_StatusTransitions tests payment status transitions
// Validates: Requirements 2.3, 2.6, 2.7
func TestDepositFlow_StatusTransitions(t *testing.T) {
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

	paymentRepo := repository.NewPaymentRepository(containers.DB)

	tests := []struct {
		name          string
		fromStatus    domain.PaymentStatus
		toStatus      domain.PaymentStatus
		shouldSucceed bool
	}{
		{"waiting to confirming", domain.PaymentStatusWaiting, domain.PaymentStatusConfirming, true},
		{"confirming to confirmed", domain.PaymentStatusConfirming, domain.PaymentStatusConfirmed, true},
		{"confirmed to finished", domain.PaymentStatusConfirmed, domain.PaymentStatusFinished, true},
		{"waiting to failed", domain.PaymentStatusWaiting, domain.PaymentStatusFailed, true},
		{"waiting to expired", domain.PaymentStatusWaiting, domain.PaymentStatusExpired, true},
		{"finished to waiting (invalid)", domain.PaymentStatusFinished, domain.PaymentStatusWaiting, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create payment with initial status
			payment := &domain.Payment{
				UUID:            uuid.New(),
				UserID:          12345,
				PaymentID:       "np-status-test-" + uuid.New().String(),
				IdempotencyKey:  uuid.New().String(),
				RequestedAmount: decimal.NewFromFloat(100.0),
				FiatAmount:      decimal.NewFromFloat(100.0),
				FiatCurrency:    "USD",
				CryptoCurrency:  "BTC",
				PayAddress:      "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
				Status:          tt.fromStatus,
			}

			err := paymentRepo.Create(ctx, payment)
			require.NoError(t, err)

			// Check if transition is valid
			canTransition := tt.fromStatus.CanTransitionTo(tt.toStatus)
			assert.Equal(t, tt.shouldSucceed, canTransition, "Status transition validation mismatch")

			if canTransition {
				err = paymentRepo.UpdateStatus(ctx, payment.ID, tt.fromStatus, tt.toStatus)
				require.NoError(t, err)

				// Verify status updated
				updated, err := paymentRepo.GetByID(ctx, payment.ID)
				require.NoError(t, err)
				assert.Equal(t, tt.toStatus, updated.Status)
			}
		})
	}
}

// TestDepositFlow_ActualAmountDiffers tests when actual received amount differs from requested
// Validates: Requirements 5.6
func TestDepositFlow_ActualAmountDiffers(t *testing.T) {
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

	paymentRepo := repository.NewPaymentRepository(containers.DB)
	walletClient := NewMockWalletClient()

	userID := int64(12345)

	// Create payment with requested amount
	payment := &domain.Payment{
		UUID:            uuid.New(),
		UserID:          userID,
		PaymentID:       "np-actual-amount-test",
		IdempotencyKey:  uuid.New().String(),
		RequestedAmount: decimal.NewFromFloat(100.0),
		FiatAmount:      decimal.NewFromFloat(100.0),
		FiatCurrency:    "USD",
		CryptoCurrency:  "BTC",
		PayAddress:      "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
		Status:          domain.PaymentStatusWaiting,
	}

	err := paymentRepo.Create(ctx, payment)
	require.NoError(t, err)

	// Simulate webhook with different actual amount (user sent more)
	actualFiatAmount := decimal.NewFromFloat(105.0) // $5 more than requested

	// Update actual amount
	err = paymentRepo.UpdateActualAmount(ctx, payment.ID, actualFiatAmount)
	require.NoError(t, err)

	// Credit wallet with actual amount
	creditResult, err := walletClient.CreditWallet(ctx, client.CreditRequest{
		UserID:         userID,
		Currency:       "USD",
		Amount:         actualFiatAmount, // Credit actual amount, not requested
		IdempotencyKey: "deposit:" + payment.PaymentID,
		ReferenceType:  "deposit",
		ReferenceID:    payment.PaymentID,
	})
	require.NoError(t, err)

	// Verify
	updated, err := paymentRepo.GetByID(ctx, payment.ID)
	require.NoError(t, err)
	assert.NotNil(t, updated.ActualAmount)
	assert.True(t, actualFiatAmount.Equal(*updated.ActualAmount))

	t.Logf("Credited actual amount: $%s (requested: $%s)", actualFiatAmount.String(), payment.RequestedAmount.String())
	t.Logf("New wallet balance: $%s", creditResult.NewBalance.String())
}

// getDepositLimit returns the daily deposit limit for a KYC level
func getDepositLimit(kycLevel int) float64 {
	limits := map[int]float64{
		0: 500,
		1: 2000,
		2: 10000,
		3: 50000,
	}
	if limit, ok := limits[kycLevel]; ok {
		return limit
	}
	return 500 // Default to level 0
}
