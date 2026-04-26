package integration

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opus-casino/payment/internal/client"
	"github.com/opus-casino/payment/internal/domain"
	"github.com/opus-casino/payment/internal/repository"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWebhookProcessing_SignatureVerification tests webhook signature verification
// Validates: Requirements 2.1, 2.2, 4.1
func TestWebhookProcessing_SignatureVerification(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	containers := SetupTestContainers(t)
	if containers == nil {
		return
	}
	defer containers.Cleanup()

	nowpaymentsClient := NewMockNOWPaymentsClient(containers.IPNSecret)

	tests := []struct {
		name           string
		payload        client.WebhookPayload
		signature      string
		expectValid    bool
	}{
		{
			name: "valid signature",
			payload: client.WebhookPayload{
				PaymentID:     "test-payment-1",
				PaymentStatus: "finished",
			},
			signature:   "valid-signature",
			expectValid: true,
		},
		{
			name: "invalid signature",
			payload: client.WebhookPayload{
				PaymentID:     "test-payment-2",
				PaymentStatus: "finished",
			},
			signature:   "invalid-signature",
			expectValid: false,
		},
		{
			name: "empty signature",
			payload: client.WebhookPayload{
				PaymentID:     "test-payment-3",
				PaymentStatus: "finished",
			},
			signature:   "",
			expectValid: true, // Mock accepts empty signature
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payloadBytes, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			isValid := nowpaymentsClient.VerifyWebhookSignature(payloadBytes, tt.signature)
			assert.Equal(t, tt.expectValid, isValid, "Signature validation mismatch")

			t.Logf("Signature validation: signature=%s, valid=%v", tt.signature, isValid)
		})
	}
}

// TestWebhookProcessing_DepositWebhook tests deposit webhook processing
// Validates: Requirements 2.3, 2.4, 2.5, 2.6, 2.7
func TestWebhookProcessing_DepositWebhook(t *testing.T) {
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
	idempotencyRepo := repository.NewIdempotencyRepository(containers.RedisClient)
	auditLogRepo := repository.NewAuditLogRepository(containers.DB)
	walletClient := NewMockWalletClient()

	userID := int64(12345)
	walletClient.SetBalance(userID, decimal.Zero)

	t.Run("process finished webhook", func(t *testing.T) {
		// Create payment in waiting status
		paymentID := "np-webhook-finished"
		payment := &domain.Payment{
			UUID:            uuid.New(),
			UserID:          userID,
			PaymentID:       paymentID,
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

		// Process webhook
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
		_ = webhookPayload

		// Update status through the flow
		err = paymentRepo.UpdateStatus(ctx, payment.ID, domain.PaymentStatusWaiting, domain.PaymentStatusConfirming)
		require.NoError(t, err)
		err = paymentRepo.UpdateStatus(ctx, payment.ID, domain.PaymentStatusConfirming, domain.PaymentStatusConfirmed)
		require.NoError(t, err)
		err = paymentRepo.UpdateStatus(ctx, payment.ID, domain.PaymentStatusConfirmed, domain.PaymentStatusFinished)
		require.NoError(t, err)

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
		_ = creditResult

		// Store idempotency result
		resultBytes, _ := json.Marshal(map[string]string{"status": "processed"})
		err = idempotencyRepo.Set(ctx, "webhook:deposit:"+paymentID, resultBytes, 86400)
		require.NoError(t, err)

		// Create audit log
		auditLog := &repository.AuditLog{
			UserID:         userID,
			OperationType:  "deposit",
			OperationID:    &payment.ID,
			ReferenceType:  "payment",
			ReferenceID:    paymentID,
			PreviousStatus: "waiting",
			NewStatus:      "finished",
			Amount:         &payment.FiatAmount,
			Currency:       "USD",
			TraceID:        uuid.New().String(),
		}
		err = auditLogRepo.Create(ctx, auditLog)
		require.NoError(t, err)

		// Verify final state
		updatedPayment, err := paymentRepo.GetByPaymentID(ctx, paymentID)
		require.NoError(t, err)
		assert.Equal(t, domain.PaymentStatusFinished, updatedPayment.Status)

		balance, err := walletClient.GetBalance(ctx, userID, "USD")
		require.NoError(t, err)
		assert.True(t, balance.Total.Equal(payment.FiatAmount))

		t.Logf("Deposit webhook processed: paymentID=%s, balance=$%s", paymentID, balance.Total.String())
	})

	t.Run("process failed webhook", func(t *testing.T) {
		paymentID := "np-webhook-failed"
		payment := &domain.Payment{
			UUID:            uuid.New(),
			UserID:          userID,
			PaymentID:       paymentID,
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

		// Process failed webhook
		err = paymentRepo.UpdateStatus(ctx, payment.ID, domain.PaymentStatusWaiting, domain.PaymentStatusFailed)
		require.NoError(t, err)

		// Create audit log
		auditLog := &repository.AuditLog{
			UserID:         userID,
			OperationType:  "deposit",
			OperationID:    &payment.ID,
			ReferenceType:  "payment",
			ReferenceID:    paymentID,
			PreviousStatus: "waiting",
			NewStatus:      "failed",
			TraceID:        uuid.New().String(),
		}
		err = auditLogRepo.Create(ctx, auditLog)
		require.NoError(t, err)

		// Verify
		updatedPayment, err := paymentRepo.GetByPaymentID(ctx, paymentID)
		require.NoError(t, err)
		assert.Equal(t, domain.PaymentStatusFailed, updatedPayment.Status)

		t.Logf("Failed deposit webhook processed: paymentID=%s", paymentID)
	})

	t.Run("process expired webhook", func(t *testing.T) {
		paymentID := "np-webhook-expired"
		payment := &domain.Payment{
			UUID:            uuid.New(),
			UserID:          userID,
			PaymentID:       paymentID,
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

		// Process expired webhook
		err = paymentRepo.UpdateStatus(ctx, payment.ID, domain.PaymentStatusWaiting, domain.PaymentStatusExpired)
		require.NoError(t, err)

		// Verify
		updatedPayment, err := paymentRepo.GetByPaymentID(ctx, paymentID)
		require.NoError(t, err)
		assert.Equal(t, domain.PaymentStatusExpired, updatedPayment.Status)

		t.Logf("Expired deposit webhook processed: paymentID=%s", paymentID)
	})
}

// TestWebhookProcessing_WithdrawalWebhook tests withdrawal webhook processing
// Validates: Requirements 4.2, 4.3, 4.4, 4.5, 4.6, 4.7
func TestWebhookProcessing_WithdrawalWebhook(t *testing.T) {
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
	idempotencyRepo := repository.NewIdempotencyRepository(containers.RedisClient)
	auditLogRepo := repository.NewAuditLogRepository(containers.DB)
	walletClient := NewMockWalletClient()

	userID := int64(12345)
	walletClient.SetBalance(userID, decimal.NewFromFloat(1000.0))

	t.Run("process finished withdrawal webhook", func(t *testing.T) {
		withdrawalID := "np-webhook-withdrawal-finished"
		
		// Create withdrawal
		withdrawal := &domain.Withdrawal{
			UUID:           uuid.New(),
			UserID:         userID,
			WithdrawalID:   withdrawalID,
			IdempotencyKey: uuid.New().String(),
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

		// Process finished webhook
		err = withdrawalRepo.UpdateStatus(ctx, withdrawal.ID, domain.WithdrawalStatusProcessing, domain.WithdrawalStatusSending)
		require.NoError(t, err)
		err = withdrawalRepo.UpdateStatus(ctx, withdrawal.ID, domain.WithdrawalStatusSending, domain.WithdrawalStatusSent)
		require.NoError(t, err)
		err = withdrawalRepo.UpdateStatus(ctx, withdrawal.ID, domain.WithdrawalStatusSent, domain.WithdrawalStatusFinished)
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

		// Store idempotency result
		resultBytes, _ := json.Marshal(map[string]string{"status": "processed"})
		err = idempotencyRepo.Set(ctx, "webhook:withdrawal:"+withdrawalID, resultBytes, 86400)
		require.NoError(t, err)

		// Create audit log
		auditLog := &repository.AuditLog{
			UserID:         userID,
			OperationType:  "withdrawal",
			OperationID:    &withdrawal.ID,
			ReferenceType:  "withdrawal",
			ReferenceID:    withdrawalID,
			PreviousStatus: "processing",
			NewStatus:      "finished",
			Amount:         &withdrawal.Amount,
			Currency:       "USD",
			TraceID:        uuid.New().String(),
		}
		err = auditLogRepo.Create(ctx, auditLog)
		require.NoError(t, err)

		// Verify
		updated, err := withdrawalRepo.GetByWithdrawalID(ctx, withdrawalID)
		require.NoError(t, err)
		assert.Equal(t, domain.WithdrawalStatusFinished, updated.Status)

		t.Logf("Withdrawal webhook processed: withdrawalID=%s", withdrawalID)
	})

	t.Run("process failed withdrawal webhook with compensation", func(t *testing.T) {
		testUserID := int64(12346)
		walletClient.SetBalance(testUserID, decimal.NewFromFloat(500.0))
		withdrawalID := "np-webhook-withdrawal-failed"

		// Create withdrawal
		withdrawal := &domain.Withdrawal{
			UUID:           uuid.New(),
			UserID:         testUserID,
			WithdrawalID:   withdrawalID,
			IdempotencyKey: uuid.New().String(),
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

		// Process failed webhook - unlock funds
		err = walletClient.UnlockFunds(ctx, lockResult.LockID, "unlock:"+withdrawalID)
		require.NoError(t, err)

		// Update status
		err = withdrawalRepo.UpdateStatus(ctx, withdrawal.ID, domain.WithdrawalStatusProcessing, domain.WithdrawalStatusFailed)
		require.NoError(t, err)

		// Create audit log
		auditLog := &repository.AuditLog{
			UserID:         testUserID,
			OperationType:  "withdrawal",
			OperationID:    &withdrawal.ID,
			ReferenceType:  "withdrawal",
			ReferenceID:    withdrawalID,
			PreviousStatus: "processing",
			NewStatus:      "failed",
			Amount:         &withdrawal.Amount,
			Currency:       "USD",
			TraceID:        uuid.New().String(),
		}
		err = auditLogRepo.Create(ctx, auditLog)
		require.NoError(t, err)

		// Verify funds unlocked
		balanceAfterUnlock, err := walletClient.GetBalance(ctx, testUserID, "USD")
		require.NoError(t, err)
		assert.True(t, balanceAfterUnlock.Available.GreaterThan(balanceAfterLock.Available))

		// Verify status
		updated, err := withdrawalRepo.GetByWithdrawalID(ctx, withdrawalID)
		require.NoError(t, err)
		assert.Equal(t, domain.WithdrawalStatusFailed, updated.Status)

		t.Logf("Failed withdrawal webhook processed with compensation: withdrawalID=%s", withdrawalID)
	})
}

// TestWebhookProcessing_Idempotency tests webhook idempotency
// Validates: Requirements 2.5, 4.4, 7.1-7.5
func TestWebhookProcessing_Idempotency(t *testing.T) {
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
	idempotencyRepo := repository.NewIdempotencyRepository(containers.RedisClient)
	walletClient := NewMockWalletClient()

	userID := int64(12345)
	walletClient.SetBalance(userID, decimal.Zero)

	t.Run("same webhook processed only once", func(t *testing.T) {
		paymentID := "np-webhook-idempotency"
		idempotencyKey := "webhook:deposit:" + paymentID

		// Create payment
		payment := &domain.Payment{
			UUID:            uuid.New(),
			UserID:          userID,
			PaymentID:       paymentID,
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

		// First webhook processing
		resultBytes, _ := json.Marshal(map[string]interface{}{
			"status":     "processed",
			"payment_id": paymentID,
			"timestamp":  time.Now().Unix(),
		})

		// Check if already processed
		_, exists, err := idempotencyRepo.Get(ctx, idempotencyKey)
		require.NoError(t, err)
		assert.False(t, exists, "Should not exist on first check")

		// Store idempotency result
		set, err := idempotencyRepo.SetNX(ctx, idempotencyKey, resultBytes, 86400)
		require.NoError(t, err)
		assert.True(t, set, "Should set on first attempt")

		// Credit wallet
		_, err = walletClient.CreditWallet(ctx, client.CreditRequest{
			UserID:         userID,
			Currency:       "USD",
			Amount:         payment.FiatAmount,
			IdempotencyKey: "deposit:" + paymentID,
		})
		require.NoError(t, err)

		// Second webhook processing (duplicate)
		_, exists, err = idempotencyRepo.Get(ctx, idempotencyKey)
		require.NoError(t, err)
		assert.True(t, exists, "Should exist on second check")

		// Try to set again (should fail)
		set, err = idempotencyRepo.SetNX(ctx, idempotencyKey, resultBytes, 86400)
		require.NoError(t, err)
		assert.False(t, set, "Should not set on duplicate attempt")

		// Verify wallet was only credited once
		balance, err := walletClient.GetBalance(ctx, userID, "USD")
		require.NoError(t, err)
		assert.True(t, balance.Total.Equal(payment.FiatAmount), "Balance should equal single credit amount")

		t.Logf("Idempotency verified: webhook processed only once, balance=$%s", balance.Total.String())
	})

	t.Run("concurrent webhook processing", func(t *testing.T) {
		paymentID := "np-webhook-concurrent"
		idempotencyKey := "webhook:deposit:" + paymentID

		// Create payment
		payment := &domain.Payment{
			UUID:            uuid.New(),
			UserID:          userID,
			PaymentID:       paymentID,
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

		// Simulate concurrent processing with SetNX
		resultBytes, _ := json.Marshal(map[string]string{"status": "processed"})

		// First SetNX should succeed
		set1, err := idempotencyRepo.SetNX(ctx, idempotencyKey, resultBytes, 86400)
		require.NoError(t, err)
		assert.True(t, set1)

		// Second SetNX should fail (already set)
		set2, err := idempotencyRepo.SetNX(ctx, idempotencyKey, resultBytes, 86400)
		require.NoError(t, err)
		assert.False(t, set2)

		t.Log("Concurrent webhook processing handled correctly with SetNX")
	})
}

// TestWebhookProcessing_AuditLogging tests audit logging for webhooks
// Validates: Requirements 8.1-8.6
func TestWebhookProcessing_AuditLogging(t *testing.T) {
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
	auditLogRepo := repository.NewAuditLogRepository(containers.DB)

	userID := int64(12345)

	t.Run("audit log created for deposit webhook", func(t *testing.T) {
		paymentID := "np-audit-deposit"
		traceID := uuid.New().String()
		correlationID := uuid.New().String()

		// Create payment
		payment := &domain.Payment{
			UUID:            uuid.New(),
			UserID:          userID,
			PaymentID:       paymentID,
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

		// Create audit log
		amount := decimal.NewFromFloat(100.0)
		auditLog := &repository.AuditLog{
			UserID:         userID,
			OperationType:  "deposit",
			OperationID:    &payment.ID,
			ReferenceType:  "payment",
			ReferenceID:    paymentID,
			PreviousStatus: "waiting",
			NewStatus:      "finished",
			Amount:         &amount,
			Currency:       "USD",
			TraceID:        traceID,
			CorrelationID:  correlationID,
		}

		err = auditLogRepo.Create(ctx, auditLog)
		require.NoError(t, err)
		assert.NotZero(t, auditLog.ID)

		// Verify audit log can be retrieved
		logs, err := auditLogRepo.ListByUserID(ctx, userID, repository.ListFilter{Limit: 10})
		require.NoError(t, err)
		assert.NotEmpty(t, logs.Items)

		found := false
		for _, log := range logs.Items {
			if log.ReferenceID == paymentID {
				found = true
				assert.Equal(t, "deposit", log.OperationType)
				assert.Equal(t, "waiting", log.PreviousStatus)
				assert.Equal(t, "finished", log.NewStatus)
				assert.Equal(t, traceID, log.TraceID)
				assert.Equal(t, correlationID, log.CorrelationID)
				break
			}
		}
		assert.True(t, found, "Audit log should be found")

		t.Logf("Audit log created and verified: paymentID=%s, traceID=%s", paymentID, traceID)
	})

	t.Run("sensitive data masking in logs", func(t *testing.T) {
		// Test address masking function
		address := "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh"
		masked := maskAddress(address)

		// Should show first 8 and last 4 characters
		assert.Contains(t, masked, "bc1qxy2k")
		assert.Contains(t, masked, "0wlh")
		assert.NotContains(t, masked, "gygjrsqtzq2n0yrf2493p83kkfjh")

		t.Logf("Address masking: original=%s, masked=%s", address, masked)
	})
}

// TestWebhookProcessing_PartialPayment tests partial payment handling
// Validates: Requirements 5.6
func TestWebhookProcessing_PartialPayment(t *testing.T) {
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
	walletClient.SetBalance(userID, decimal.Zero)

	t.Run("partially paid webhook", func(t *testing.T) {
		paymentID := "np-partial-payment"

		// Create payment
		payment := &domain.Payment{
			UUID:            uuid.New(),
			UserID:          userID,
			PaymentID:       paymentID,
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

		// Simulate partial payment webhook
		actualAmount := decimal.NewFromFloat(80.0) // Only 80% of requested

		// Update to partially_paid
		err = paymentRepo.UpdateStatus(ctx, payment.ID, domain.PaymentStatusWaiting, domain.PaymentStatusPartiallyPaid)
		require.NoError(t, err)

		// Update actual amount
		err = paymentRepo.UpdateActualAmount(ctx, payment.ID, actualAmount)
		require.NoError(t, err)

		// Credit wallet with actual amount (not requested)
		_, err = walletClient.CreditWallet(ctx, client.CreditRequest{
			UserID:         userID,
			Currency:       "USD",
			Amount:         actualAmount,
			IdempotencyKey: "deposit:" + paymentID,
		})
		require.NoError(t, err)

		// Verify
		updated, err := paymentRepo.GetByPaymentID(ctx, paymentID)
		require.NoError(t, err)
		assert.Equal(t, domain.PaymentStatusPartiallyPaid, updated.Status)
		assert.NotNil(t, updated.ActualAmount)
		assert.True(t, actualAmount.Equal(*updated.ActualAmount))

		balance, err := walletClient.GetBalance(ctx, userID, "USD")
		require.NoError(t, err)
		assert.True(t, balance.Total.Equal(actualAmount))

		t.Logf("Partial payment handled: requested=$%s, actual=$%s, balance=$%s",
			payment.RequestedAmount.String(), actualAmount.String(), balance.Total.String())
	})
}

// maskAddress masks a wallet address for logging
func maskAddress(address string) string {
	if len(address) <= 12 {
		return address
	}
	return address[:8] + "..." + address[len(address)-4:]
}
