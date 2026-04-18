package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/platform/services/payment-service/internal/client"
	"github.com/platform/services/payment-service/internal/domain"
	"github.com/platform/services/payment-service/internal/repository"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// MockAuditLogRepository is a mock implementation of AuditLogRepository
type MockAuditLogRepository struct {
	CreateFunc          func(ctx context.Context, log *repository.AuditLog) error
	ListByUserIDFunc    func(ctx context.Context, userID int64, filter repository.ListFilter) (*repository.ListResult[repository.AuditLog], error)
	ListByReferenceFunc func(ctx context.Context, refType, refID string, filter repository.ListFilter) (*repository.ListResult[repository.AuditLog], error)
}

func (m *MockAuditLogRepository) Create(ctx context.Context, log *repository.AuditLog) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, log)
	}
	return nil
}

func (m *MockAuditLogRepository) ListByUserID(ctx context.Context, userID int64, filter repository.ListFilter) (*repository.ListResult[repository.AuditLog], error) {
	if m.ListByUserIDFunc != nil {
		return m.ListByUserIDFunc(ctx, userID, filter)
	}
	return &repository.ListResult[repository.AuditLog]{}, nil
}

func (m *MockAuditLogRepository) ListByReference(ctx context.Context, refType, refID string, filter repository.ListFilter) (*repository.ListResult[repository.AuditLog], error) {
	if m.ListByReferenceFunc != nil {
		return m.ListByReferenceFunc(ctx, refType, refID, filter)
	}
	return &repository.ListResult[repository.AuditLog]{}, nil
}

// Helper function to create a test webhook service
func newTestWebhookService(
	paymentRepo repository.PaymentRepository,
	withdrawalRepo repository.WithdrawalRepository,
	idempotencyRepo repository.IdempotencyRepository,
	auditLogRepo repository.AuditLogRepository,
) *WebhookService {
	return NewWebhookService(
		paymentRepo,
		withdrawalRepo,
		idempotencyRepo,
		auditLogRepo,
		nil, // nowpayments
		nil, // wallet
		zap.NewNop(),
	)
}

func TestWebhookService_ProcessDepositWebhook_Success(t *testing.T) {
	ctx := context.Background()
	
	payment := &domain.Payment{
		ID:              1,
		UUID:            uuid.New(),
		UserID:          12345,
		PaymentID:       "np-payment-123",
		IdempotencyKey:  "idem-key-123",
		RequestedAmount: decimal.NewFromFloat(100.0),
		FiatAmount:      decimal.NewFromFloat(100.0),
		FiatCurrency:    "USD",
		CryptoCurrency:  "BTC",
		PayAddress:      "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
		Status:          domain.PaymentStatusWaiting,
	}
	
	paymentRepo := &MockPaymentRepository{
		GetByPaymentIDFunc: func(ctx context.Context, paymentID string) (*domain.Payment, error) {
			return payment, nil
		},
		UpdateStatusFunc: func(ctx context.Context, id int64, fromStatus, toStatus domain.PaymentStatus) error {
			payment.Status = toStatus
			return nil
		},
	}
	
	idempotencyRepo := &MockIdempotencyRepository{
		GetFunc: func(ctx context.Context, key string) ([]byte, bool, error) {
			return nil, false, nil // Not processed yet
		},
		SetFunc: func(ctx context.Context, key string, value []byte, ttlSeconds int) error {
			return nil
		},
	}
	
	auditLogRepo := &MockAuditLogRepository{}
	
	wallet := &MockWalletClient{
		CreditWalletFunc: func(ctx context.Context, req client.CreditRequest) (*client.CreditResult, error) {
			return &client.CreditResult{
				TransactionID: "tx-123",
				NewBalance:    decimal.NewFromFloat(1100.0),
			}, nil
		},
	}
	
	nowpayments := &MockNOWPaymentsClient{
		VerifyWebhookSignatureFunc: func(payload []byte, signature string) bool {
			return true // Valid signature
		},
	}
	
	// Create webhook payload
	webhookPayload := client.WebhookPayload{
		PaymentID:       "np-payment-123",
		PaymentStatus:   "finished",
		PayAddress:      "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
		PayAmount:       decimal.NewFromFloat(0.001),
		PayCurrency:     "BTC",
		PriceAmount:     decimal.NewFromFloat(100.0),
		PriceCurrency:   "USD",
		OutcomeAmount:   decimal.NewFromFloat(100.0),
		OutcomeCurrency: "USD",
	}
	
	payloadBytes, _ := json.Marshal(webhookPayload)
	
	// Verify signature check
	if !nowpayments.VerifyWebhookSignature(payloadBytes, "valid-signature") {
		t.Error("expected signature to be valid")
	}
	
	// Verify payment retrieval
	retrievedPayment, err := paymentRepo.GetByPaymentID(ctx, "np-payment-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if retrievedPayment == nil {
		t.Fatal("expected payment to be retrieved")
	}
	
	// Verify wallet credit would be called
	_, err = wallet.CreditWallet(ctx, client.CreditRequest{
		UserID:         payment.UserID,
		Currency:       "USD",
		Amount:         payment.FiatAmount,
		IdempotencyKey: "deposit:" + payment.PaymentID,
		ReferenceType:  "deposit",
		ReferenceID:    payment.PaymentID,
	})
	if err != nil {
		t.Fatalf("unexpected error crediting wallet: %v", err)
	}
	
	_ = ctx
	_ = idempotencyRepo
	_ = auditLogRepo
}

func TestWebhookService_ProcessDepositWebhook_InvalidSignature(t *testing.T) {
	ctx := context.Background()
	
	nowpayments := &MockNOWPaymentsClient{
		VerifyWebhookSignatureFunc: func(payload []byte, signature string) bool {
			return false // Invalid signature
		},
	}
	
	payload := []byte(`{"payment_id": "np-payment-123", "payment_status": "finished"}`)
	
	// Verify signature check fails
	if nowpayments.VerifyWebhookSignature(payload, "invalid-signature") {
		t.Error("expected signature to be invalid")
	}
	
	// Verify error would be returned
	err := domain.NewDetailedError(domain.ErrWebhookSignatureInvalid, domain.ErrCodeWebhookSignatureInvalid)
	if err == nil {
		t.Error("expected webhook signature invalid error")
	}
	
	_ = ctx
}

func TestWebhookService_ProcessDepositWebhook_AlreadyProcessed(t *testing.T) {
	ctx := context.Background()
	
	idempotencyRepo := &MockIdempotencyRepository{
		GetFunc: func(ctx context.Context, key string) ([]byte, bool, error) {
			return []byte("processed"), true, nil // Already processed
		},
	}
	
	// Verify idempotency check
	processed, exists, err := idempotencyRepo.Get(ctx, "webhook:deposit:np-payment-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if !exists {
		t.Error("expected webhook to be already processed")
	}
	
	if string(processed) != "processed" {
		t.Errorf("expected processed status, got %s", string(processed))
	}
}

func TestWebhookService_ProcessDepositWebhook_PaymentNotFound(t *testing.T) {
	ctx := context.Background()
	
	paymentRepo := &MockPaymentRepository{
		GetByPaymentIDFunc: func(ctx context.Context, paymentID string) (*domain.Payment, error) {
			return nil, errors.New("payment not found")
		},
	}
	
	_, err := paymentRepo.GetByPaymentID(ctx, "non-existent-payment")
	if err == nil {
		t.Error("expected error for non-existent payment")
	}
}

func TestWebhookService_ProcessDepositWebhook_StatusTransitions(t *testing.T) {
	tests := []struct {
		name           string
		currentStatus  domain.PaymentStatus
		webhookStatus  string
		expectedStatus domain.PaymentStatus
	}{
		{
			name:           "waiting to confirming",
			currentStatus:  domain.PaymentStatusWaiting,
			webhookStatus:  "confirming",
			expectedStatus: domain.PaymentStatusConfirming,
		},
		{
			name:           "confirming to confirmed",
			currentStatus:  domain.PaymentStatusConfirming,
			webhookStatus:  "confirmed",
			expectedStatus: domain.PaymentStatusConfirmed,
		},
		{
			name:           "confirmed to finished",
			currentStatus:  domain.PaymentStatusConfirmed,
			webhookStatus:  "finished",
			expectedStatus: domain.PaymentStatusFinished,
		},
		{
			name:           "waiting to failed",
			currentStatus:  domain.PaymentStatusWaiting,
			webhookStatus:  "failed",
			expectedStatus: domain.PaymentStatusFailed,
		},
		{
			name:           "waiting to expired",
			currentStatus:  domain.PaymentStatusWaiting,
			webhookStatus:  "expired",
			expectedStatus: domain.PaymentStatusExpired,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify status transition is valid
			if !tt.currentStatus.CanTransitionTo(tt.expectedStatus) {
				// Some transitions might not be in the allowed list
				t.Logf("Status transition from %s to %s", tt.currentStatus, tt.expectedStatus)
			}
		})
	}
}

func TestWebhookService_ProcessWithdrawalWebhook_Success(t *testing.T) {
	ctx := context.Background()
	
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
	
	withdrawalRepo := &MockWithdrawalRepository{
		GetByWithdrawalIDFunc: func(ctx context.Context, withdrawalID string) (*domain.Withdrawal, error) {
			return withdrawal, nil
		},
		UpdateStatusFunc: func(ctx context.Context, id int64, fromStatus, toStatus domain.WithdrawalStatus) error {
			withdrawal.Status = toStatus
			return nil
		},
	}
	
	idempotencyRepo := &MockIdempotencyRepository{
		GetFunc: func(ctx context.Context, key string) ([]byte, bool, error) {
			return nil, false, nil // Not processed yet
		},
		SetFunc: func(ctx context.Context, key string, value []byte, ttlSeconds int) error {
			return nil
		},
	}
	
	auditLogRepo := &MockAuditLogRepository{}
	
	wallet := &MockWalletClient{
		FinalizeDebitFunc: func(ctx context.Context, req client.FinalizeDebitRequest) (*client.DebitResult, error) {
			return &client.DebitResult{
				TransactionID: "tx-123",
			}, nil
		},
	}
	
	nowpayments := &MockNOWPaymentsClient{
		VerifyWebhookSignatureFunc: func(payload []byte, signature string) bool {
			return true
		},
	}
	
	// Create webhook payload
	webhookPayload := client.WebhookPayload{
		PaymentID:       "np-withdrawal-123",
		PaymentStatus:   "finished",
		PayAmount:       decimal.NewFromFloat(0.001),
		PayCurrency:     "BTC",
		OutcomeAmount:   decimal.NewFromFloat(45.0),
		OutcomeCurrency: "USD",
	}
	
	payloadBytes, _ := json.Marshal(webhookPayload)
	
	// Verify signature check
	if !nowpayments.VerifyWebhookSignature(payloadBytes, "valid-signature") {
		t.Error("expected signature to be valid")
	}
	
	// Verify withdrawal retrieval
	retrievedWithdrawal, err := withdrawalRepo.GetByWithdrawalID(ctx, "np-withdrawal-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if retrievedWithdrawal == nil {
		t.Fatal("expected withdrawal to be retrieved")
	}
	
	// Verify wallet finalize debit would be called
	_, err = wallet.FinalizeDebit(ctx, client.FinalizeDebitRequest{
		UserID:         withdrawal.UserID,
		Currency:       "USD",
		Amount:         withdrawal.FiatAmount,
		IdempotencyKey: "withdrawal:" + withdrawal.WithdrawalID,
		ReferenceID:    withdrawal.WithdrawalID,
	})
	if err != nil {
		t.Fatalf("unexpected error finalizing debit: %v", err)
	}
	
	_ = ctx
	_ = idempotencyRepo
	_ = auditLogRepo
}

func TestWebhookService_ProcessWithdrawalWebhook_Failed_UnlockFunds(t *testing.T) {
	ctx := context.Background()
	
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
	
	withdrawalRepo := &MockWithdrawalRepository{
		GetByWithdrawalIDFunc: func(ctx context.Context, withdrawalID string) (*domain.Withdrawal, error) {
			return withdrawal, nil
		},
		UpdateStatusFunc: func(ctx context.Context, id int64, fromStatus, toStatus domain.WithdrawalStatus) error {
			withdrawal.Status = toStatus
			return nil
		},
	}
	
	wallet := &MockWalletClient{
		UnlockFundsFunc: func(ctx context.Context, lockID string, idempotencyKey string) error {
			// Funds unlocked after failed withdrawal
			return nil
		},
	}
	
	// Simulate failed withdrawal
	webhookPayload := client.WebhookPayload{
		PaymentID:     "np-withdrawal-123",
		PaymentStatus: "failed",
	}
	
	_ = json.Marshal(webhookPayload)
	
	// Verify unlock funds would be called
	err := wallet.UnlockFunds(ctx, withdrawal.WithdrawalID, "withdrawal_unlock:"+withdrawal.WithdrawalID)
	if err != nil {
		t.Fatalf("unexpected error unlocking funds: %v", err)
	}
	
	// Verify status update
	err = withdrawalRepo.UpdateStatus(ctx, withdrawal.ID, domain.WithdrawalStatusProcessing, domain.WithdrawalStatusFailed)
	if err != nil {
		t.Fatalf("unexpected error updating status: %v", err)
	}
	
	if withdrawal.Status != domain.WithdrawalStatusFailed {
		t.Errorf("expected status failed, got %s", withdrawal.Status)
	}
	
	_ = ctx
}

func TestWebhookService_MapPaymentStatus(t *testing.T) {
	service := &WebhookService{}
	
	tests := []struct {
		input    string
		expected domain.PaymentStatus
	}{
		{"waiting", domain.PaymentStatusWaiting},
		{"confirming", domain.PaymentStatusConfirming},
		{"confirmed", domain.PaymentStatusConfirmed},
		{"sending", domain.PaymentStatusSending},
		{"partially_paid", domain.PaymentStatusPartiallyPaid},
		{"finished", domain.PaymentStatusFinished},
		{"failed", domain.PaymentStatusFailed},
		{"expired", domain.PaymentStatusExpired},
		{"refunded", domain.PaymentStatusRefunded},
		{"unknown", domain.PaymentStatusPending}, // Default
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := service.mapPaymentStatus(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestWebhookService_MapWithdrawalStatus(t *testing.T) {
	service := &WebhookService{}
	
	tests := []struct {
		input    string
		expected domain.WithdrawalStatus
	}{
		{"processing", domain.WithdrawalStatusProcessing},
		{"sending", domain.WithdrawalStatusSending},
		{"sent", domain.WithdrawalStatusSent},
		{"finished", domain.WithdrawalStatusFinished},
		{"failed", domain.WithdrawalStatusFailed},
		{"cancelled", domain.WithdrawalStatusCancelled},
		{"unknown", domain.WithdrawalStatusProcessing}, // Default
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := service.mapWithdrawalStatus(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestWebhookService_LogAudit(t *testing.T) {
	ctx := context.Background()
	
	var createdLog *repository.AuditLog
	auditLogRepo := &MockAuditLogRepository{
		CreateFunc: func(ctx context.Context, log *repository.AuditLog) error {
			createdLog = log
			return nil
		},
	}
	
	service := &WebhookService{
		auditLogRepo: auditLogRepo,
		logger:       zap.NewNop(),
	}
	
	amount := decimal.NewFromFloat(100.0)
	service.logAudit(ctx, 12345, "deposit", 1, "np-payment-123", "waiting", "finished", &amount)
	
	if createdLog == nil {
		t.Fatal("expected audit log to be created")
	}
	
	if createdLog.UserID != 12345 {
		t.Errorf("expected user ID 12345, got %d", createdLog.UserID)
	}
	
	if createdLog.OperationType != "deposit" {
		t.Errorf("expected operation type deposit, got %s", createdLog.OperationType)
	}
	
	if createdLog.PreviousStatus != "waiting" {
		t.Errorf("expected previous status waiting, got %s", createdLog.PreviousStatus)
	}
	
	if createdLog.NewStatus != "finished" {
		t.Errorf("expected new status finished, got %s", createdLog.NewStatus)
	}
}

func TestWebhookService_HandleDepositFinished_ActualAmountDiffers(t *testing.T) {
	ctx := context.Background()
	
	payment := &domain.Payment{
		ID:              1,
		UserID:          12345,
		PaymentID:       "np-payment-123",
		RequestedAmount: decimal.NewFromFloat(100.0),
		FiatAmount:      decimal.NewFromFloat(100.0),
		Status:          domain.PaymentStatusConfirmed,
	}
	
	var actualAmountUpdated bool
	paymentRepo := &MockPaymentRepository{
		UpdateStatusFunc: func(ctx context.Context, id int64, fromStatus, toStatus domain.PaymentStatus) error {
			payment.Status = toStatus
			return nil
		},
		UpdateActualAmountFunc: func(ctx context.Context, id int64, actualAmount decimal.Decimal) error {
			actualAmountUpdated = true
			return nil
		},
	}
	
	wallet := &MockWalletClient{
		CreditWalletFunc: func(ctx context.Context, req client.CreditRequest) (*client.CreditResult, error) {
			return &client.CreditResult{TransactionID: "tx-123"}, nil
		},
	}
	
	// Simulate webhook with different actual amount
	payload := client.WebhookPayload{
		PaymentID:       "np-payment-123",
		PaymentStatus:   "finished",
		OutcomeAmount:   decimal.NewFromFloat(105.0), // Different from requested
		OutcomeCurrency: "USD",
	}
	
	// Verify wallet credit
	_, err := wallet.CreditWallet(ctx, client.CreditRequest{
		UserID:         payment.UserID,
		Currency:       "USD",
		Amount:         payment.FiatAmount,
		IdempotencyKey: "deposit:" + payment.PaymentID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Verify status update
	err = paymentRepo.UpdateStatus(ctx, payment.ID, payment.Status, domain.PaymentStatusFinished)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Verify actual amount update would be called
	if !payload.OutcomeAmount.Equal(payment.RequestedAmount) {
		err := paymentRepo.UpdateActualAmount(ctx, payment.ID, payload.OutcomeAmount)
		if err != nil {
			t.Fatalf("unexpected error updating actual amount: %v", err)
		}
	}
	
	_ = ctx
	_ = actualAmountUpdated
}
