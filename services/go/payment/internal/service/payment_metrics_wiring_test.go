package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/opus-casino/payment/internal/client"
	"github.com/opus-casino/payment/internal/domain"
	"github.com/opus-casino/payment/internal/observability"
	"github.com/opus-casino/payment/internal/repository"
	dto "github.com/prometheus/client_model/go"
	"github.com/shopspring/decimal"
)

// The service metrics are process-global (promauto on the default registry), so
// every assertion below is a delta around the call under test.

// depositCount reads payment_deposits_total for one label set.
func depositCount(t *testing.T, status, currency, kycLevel string) float64 {
	t.Helper()
	return vecValue(t, observability.DepositCounter.WithLabelValues(status, currency, kycLevel))
}

func withdrawalCount(t *testing.T, status, currency, kycLevel string) float64 {
	t.Helper()
	return vecValue(t, observability.WithdrawalCounter.WithLabelValues(status, currency, kycLevel))
}

func errorCount(t *testing.T, errorType, operation string) float64 {
	t.Helper()
	return vecValue(t, observability.ErrorCounter.WithLabelValues(errorType, operation))
}

// vecValue reads a single labelled child of a metric vector via its protobuf
// form, so the test does not need a second registry.
func vecValue(t *testing.T, child interface{ Write(*dto.Metric) error }) float64 {
	t.Helper()
	var m dto.Metric
	if err := child.Write(&m); err != nil {
		t.Fatalf("read metric: %v", err)
	}
	return m.GetCounter().GetValue()
}

// depositFixture builds a PaymentService whose collaborators succeed, so the
// only thing under test is what the happy path records.
func depositFixture(t *testing.T, kycLevel int) *PaymentService {
	t.Helper()
	return NewPaymentService(
		&MockPaymentRepository{
			GetByIDempotencyKeyFunc: func(context.Context, string) (*domain.Payment, error) {
				return nil, nil
			},
			CreateFunc: func(_ context.Context, p *domain.Payment) error {
				p.ID = 1
				return nil
			},
		},
		&MockIdempotencyRepository{},
		&MockExchangeRateRepository{
			GetFunc: func(context.Context, string, string) (*decimal.Decimal, error) {
				rate := decimal.NewFromInt(45000)
				return &rate, nil
			},
		},
		&MockDailyLimitsRepository{},
		&MockNOWPaymentsClient{
			CreatePaymentFunc: func(context.Context, client.CreatePaymentRequest) (*client.CreatePaymentResponse, error) {
				return &client.CreatePaymentResponse{
					PaymentID:     "np-1",
					PaymentStatus: "waiting",
					PayAmount:     decimal.NewFromInt(1),
					ExpiresAt:     time.Now().Add(24 * time.Hour),
				}, nil
			},
		},
		nil,
		&MockUserClient{
			GetKYCLevelFunc: func(context.Context, int64) (int, error) { return kycLevel, nil },
		},
		nil,
		noopTracer(),
		"",
	)
}

func TestInitiateDeposit_RecordsPendingWithKYCLevel(t *testing.T) {
	svc := depositFixture(t, 2)

	before := depositCount(t, observability.DepositStatusPending, string(domain.CryptoBTC), "2")

	resp, err := svc.InitiateDeposit(context.Background(), InitiateDepositRequest{
		UserID:         1,
		Amount:         decimal.NewFromInt(100),
		Currency:       domain.CryptoBTC,
		IdempotencyKey: "k1",
	})
	if err != nil {
		t.Fatalf("InitiateDeposit: %v", err)
	}
	if resp.PaymentID != "np-1" {
		t.Fatalf("unexpected payment id %q", resp.PaymentID)
	}

	if got := depositCount(t, observability.DepositStatusPending, string(domain.CryptoBTC), "2"); got != before+1 {
		t.Errorf("deposits_total{status=pending,kyc_level=2} = %v, want %v", got, before+1)
	}
}

// A retried request must not inflate the volume dashboards, otherwise deposit
// counts diverge from actual money in.
func TestInitiateDeposit_IdempotentReplayRecordsNothing(t *testing.T) {
	existing := &domain.Payment{
		ID:             1,
		PaymentID:      "np-existing",
		CryptoCurrency: string(domain.CryptoBTC),
		Status:         domain.PaymentStatusPending,
	}
	svc := NewPaymentService(
		&MockPaymentRepository{
			GetByIDempotencyKeyFunc: func(context.Context, string) (*domain.Payment, error) {
				return existing, nil
			},
		},
		&MockIdempotencyRepository{},
		&MockExchangeRateRepository{},
		&MockDailyLimitsRepository{},
		&MockNOWPaymentsClient{},
		nil,
		&MockUserClient{},
		nil,
		noopTracer(),
		"",
	)

	before := depositCount(t, observability.DepositStatusPending, string(domain.CryptoBTC), "0")

	if _, err := svc.InitiateDeposit(context.Background(), InitiateDepositRequest{
		UserID:         1,
		Amount:         decimal.NewFromInt(100),
		Currency:       domain.CryptoBTC,
		IdempotencyKey: "k1",
	}); err != nil {
		t.Fatalf("InitiateDeposit: %v", err)
	}

	if got := depositCount(t, observability.DepositStatusPending, string(domain.CryptoBTC), "0"); got != before {
		t.Errorf("idempotent replay moved deposits_total: %v -> %v", before, got)
	}
}

// The daily-limit rejection is a business rule, not a provider fault. It gets
// its own error_type so the limit rules can be tuned separately.
func TestInitiateDeposit_DailyLimitExceededRecordsLimitError(t *testing.T) {
	svc := NewPaymentService(
		&MockPaymentRepository{
			GetByIDempotencyKeyFunc: func(context.Context, string) (*domain.Payment, error) {
				return nil, nil
			},
		},
		&MockIdempotencyRepository{},
		&MockExchangeRateRepository{},
		&MockDailyLimitsRepository{
			GetFunc: func(context.Context, int64, string) (decimal.Decimal, error) {
				// KYC level 0 caps deposits at 500/day.
				return decimal.NewFromInt(450), nil
			},
		},
		&MockNOWPaymentsClient{},
		nil,
		&MockUserClient{
			GetKYCLevelFunc: func(context.Context, int64) (int, error) { return 0, nil },
		},
		nil,
		noopTracer(),
		"",
	)

	before := errorCount(t, "limit_exceeded", observability.OperationDeposit)

	_, err := svc.InitiateDeposit(context.Background(), InitiateDepositRequest{
		UserID:         1,
		Amount:         decimal.NewFromInt(100),
		Currency:       domain.CryptoBTC,
		IdempotencyKey: "k1",
	})
	if err == nil {
		t.Fatal("expected the daily limit to reject the deposit")
	}
	if domain.GetErrorCode(err) != domain.ErrCodeDailyLimitExceeded {
		t.Errorf("error code = %d, want %d", domain.GetErrorCode(err), domain.ErrCodeDailyLimitExceeded)
	}

	if got := errorCount(t, "limit_exceeded", observability.OperationDeposit); got != before+1 {
		t.Errorf("errors_total{error_type=limit_exceeded} = %v, want %v", got, before+1)
	}
}

func TestMetricErrorType_MapsDomainCodes(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{domain.ErrorDailyLimitExceeded(1, 0, 1), "limit_exceeded"},
		{domain.ErrorKYCRequiredLevel(1), "kyc_required"},
		{domain.ErrorInsufficientBalance(0, 1), "insufficient_balance"},
		{domain.ErrorCurrencyNotSupported("NOPE"), "currency_not_supported"},
		{domain.ErrorProviderUnavailable("NOWPayments", errors.New("boom")), "provider_error"},
		{domain.NewDetailedError(domain.ErrWebhookSignatureInvalid, domain.ErrCodeWebhookSignatureInvalid), "webhook_invalid"},
		{errors.New("plain error"), "internal_error"},
		{nil, "internal_error"},
	}
	for _, tc := range cases {
		if got := metricErrorType(tc.err); got != tc.want {
			t.Errorf("metricErrorType(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
}

// An unrecognised error must collapse to a constant. If the raw message ever
// reached the label, one crafted provider response would mint a new series.
func TestMetricErrorType_NeverLeaksErrorText(t *testing.T) {
	err := errors.New("provider said: user=12345 token=deadbeef")

	got := metricErrorType(err)

	if got == err.Error() {
		t.Fatalf("metricErrorType returned the raw error text: %q", got)
	}
	for _, leak := range []string{"12345", "deadbeef", "provider said"} {
		if strings.Contains(got, leak) {
			t.Errorf("label %q leaks %q", got, leak)
		}
	}
}

func TestWalletErrorType_MapsStablePrefixes(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{errors.New("LockFunds: not found: no wallet"), "wallet_not_found"},
		{errors.New("LockFunds: invalid argument: bad amount"), "wallet_rejected_request"},
		{errors.New("LockFunds: failed precondition: wallet locked"), "wallet_locked"},
		{errors.New("LockFunds: resource exhausted: balance too low"), "insufficient_balance"},
		{errors.New("LockFunds: service unavailable: downstream down"), "wallet_unavailable"},
		{errors.New("LockFunds: deadline exceeded"), "wallet_unavailable"},
		{domain.ErrorInsufficientBalance(0, 10), "insufficient_balance"},
		{errors.New("something nobody predicted"), "internal_error"},
		{nil, "internal_error"},
	}
	for _, tc := range cases {
		if got := walletErrorType(tc.err); got != tc.want {
			t.Errorf("walletErrorType(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
}

// withdrawalFixture builds a WithdrawalService whose collaborators succeed.
func withdrawalFixture(t *testing.T, kycLevel int, wallet *MockWalletClient) *WithdrawalService {
	t.Helper()
	return NewWithdrawalService(
		&MockWithdrawalRepository{
			GetByIDempotencyKeyFunc: func(context.Context, string) (*domain.Withdrawal, error) {
				return nil, nil
			},
			CreateFunc: func(_ context.Context, w *domain.Withdrawal) error {
				w.ID = 1
				return nil
			},
		},
		&MockIdempotencyRepository{},
		&MockExchangeRateRepository{
			GetFunc: func(context.Context, string, string) (*decimal.Decimal, error) {
				rate := decimal.NewFromInt(45000)
				return &rate, nil
			},
		},
		&MockDailyLimitsRepository{},
		&MockNOWPaymentsClient{
			CreatePayoutFunc: func(context.Context, client.CreatePayoutRequest) (*client.CreatePayoutResponse, error) {
				return &client.CreatePayoutResponse{WithdrawalID: "np-w-1"}, nil
			},
		},
		wallet,
		&MockUserClient{
			GetKYCLevelFunc: func(context.Context, int64) (int, error) { return kycLevel, nil },
		},
		nil,
		noopTracer(),
		"",
	)
}

func TestInitiateWithdrawal_RecordsProcessingWithKYCLevel(t *testing.T) {
	wallet := &MockWalletClient{
		LockFundsFunc: func(context.Context, client.LockRequest) (*client.LockResult, error) {
			return &client.LockResult{LockID: "lock-1"}, nil
		},
	}
	svc := withdrawalFixture(t, 2, wallet)

	before := withdrawalCount(t, observability.WithdrawalStatusProcessing, string(domain.CryptoBTC), "2")

	if _, err := svc.InitiateWithdrawal(context.Background(), InitiateWithdrawalRequest{
		UserID:         1,
		Amount:         decimal.NewFromInt(1),
		Currency:       domain.CryptoBTC,
		Address:        "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
		IdempotencyKey: "w1",
	}); err != nil {
		t.Fatalf("InitiateWithdrawal: %v", err)
	}

	if got := withdrawalCount(t, observability.WithdrawalStatusProcessing, string(domain.CryptoBTC), "2"); got != before+1 {
		t.Errorf("withdrawals_total{status=processing,kyc_level=2} = %v, want %v", got, before+1)
	}
}

// Withdrawals below KYC level 2 must be rejected, and the rejection must be
// visible without spending money on a provider call.
func TestInitiateWithdrawal_BelowKYCLvl2IsRejectedAndRecorded(t *testing.T) {
	wallet := &MockWalletClient{}
	svc := withdrawalFixture(t, 1, wallet)

	before := errorCount(t, "kyc_required", observability.OperationWithdrawal)

	_, err := svc.InitiateWithdrawal(context.Background(), InitiateWithdrawalRequest{
		UserID:         1,
		Amount:         decimal.NewFromInt(1),
		Currency:       domain.CryptoBTC,
		Address:        "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
		IdempotencyKey: "w1",
	})
	if err == nil {
		t.Fatal("expected KYC level 1 to be rejected")
	}
	if domain.GetErrorCode(err) != domain.ErrCodeKYCRequired {
		t.Errorf("error code = %d, want %d", domain.GetErrorCode(err), domain.ErrCodeKYCRequired)
	}

	if got := errorCount(t, "kyc_required", observability.OperationWithdrawal); got != before+1 {
		t.Errorf("errors_total{error_type=kyc_required} = %v, want %v", got, before+1)
	}
	if wallet.LockFundsFunc != nil {
		t.Error("funds were locked for a withdrawal that should have been rejected")
	}
}

// When the payout fails after the funds are locked, the unlock is the only thing
// standing between the player and a stranded balance. Its failure is the single
// most expensive bug in this flow, so it gets a dedicated label.
func TestInitiateWithdrawal_UnlockFailureAfterPayoutErrorIsRecorded(t *testing.T) {
	wallet := &MockWalletClient{
		LockFundsFunc: func(context.Context, client.LockRequest) (*client.LockResult, error) {
			return &client.LockResult{LockID: "lock-1"}, nil
		},
		UnlockFundsFunc: func(context.Context, string, string) error {
			return errors.New("wallet unreachable")
		},
	}
	svc := NewWithdrawalService(
		&MockWithdrawalRepository{
			GetByIDempotencyKeyFunc: func(context.Context, string) (*domain.Withdrawal, error) {
				return nil, nil
			},
		},
		&MockIdempotencyRepository{},
		&MockExchangeRateRepository{
			GetFunc: func(context.Context, string, string) (*decimal.Decimal, error) {
				rate := decimal.NewFromInt(45000)
				return &rate, nil
			},
		},
		&MockDailyLimitsRepository{},
		&MockNOWPaymentsClient{
			CreatePayoutFunc: func(context.Context, client.CreatePayoutRequest) (*client.CreatePayoutResponse, error) {
				return nil, errors.New("provider 500")
			},
		},
		wallet,
		&MockUserClient{
			GetKYCLevelFunc: func(context.Context, int64) (int, error) { return 2, nil },
		},
		nil,
		noopTracer(),
		"",
	)

	beforeCompensation := errorCount(t, "compensation_failed", observability.OperationWithdrawal)
	beforeProvider := errorCount(t, "provider_error", observability.OperationWithdrawal)

	if _, err := svc.InitiateWithdrawal(context.Background(), InitiateWithdrawalRequest{
		UserID:         1,
		Amount:         decimal.NewFromInt(1),
		Currency:       domain.CryptoBTC,
		Address:        "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
		IdempotencyKey: "w1",
	}); err == nil {
		t.Fatal("expected the provider error to fail the withdrawal")
	}

	if got := errorCount(t, "provider_error", observability.OperationWithdrawal); got != beforeProvider+1 {
		t.Errorf("errors_total{error_type=provider_error} = %v, want %v", got, beforeProvider+1)
	}
	if got := errorCount(t, "compensation_failed", observability.OperationWithdrawal); got != beforeCompensation+1 {
		t.Errorf("errors_total{error_type=compensation_failed} = %v, want %v", got, beforeCompensation+1)
	}
}

// The daily limit is charged after the row is written. If that write fails the
// player can exceed their daily cap, so it must be recorded.
func TestInitiateWithdrawal_LimitTrackingFailureIsRecorded(t *testing.T) {
	wallet := &MockWalletClient{
		LockFundsFunc: func(context.Context, client.LockRequest) (*client.LockResult, error) {
			return &client.LockResult{LockID: "lock-1"}, nil
		},
	}
	svc := NewWithdrawalService(
		&MockWithdrawalRepository{
			GetByIDempotencyKeyFunc: func(context.Context, string) (*domain.Withdrawal, error) {
				return nil, nil
			},
			CreateFunc: func(_ context.Context, w *domain.Withdrawal) error {
				w.ID = 1
				return nil
			},
		},
		&MockIdempotencyRepository{},
		&MockExchangeRateRepository{
			GetFunc: func(context.Context, string, string) (*decimal.Decimal, error) {
				rate := decimal.NewFromInt(45000)
				return &rate, nil
			},
		},
		&MockDailyLimitsRepository{
			IncrementFunc: func(context.Context, int64, string, decimal.Decimal) (decimal.Decimal, error) {
				return decimal.Zero, errors.New("redis down")
			},
		},
		&MockNOWPaymentsClient{
			CreatePayoutFunc: func(context.Context, client.CreatePayoutRequest) (*client.CreatePayoutResponse, error) {
				return &client.CreatePayoutResponse{WithdrawalID: "np-w-1"}, nil
			},
		},
		wallet,
		&MockUserClient{
			GetKYCLevelFunc: func(context.Context, int64) (int, error) { return 2, nil },
		},
		nil,
		noopTracer(),
		"",
	)

	before := errorCount(t, "daily_limit_not_tracked", observability.OperationWithdrawal)

	if _, err := svc.InitiateWithdrawal(context.Background(), InitiateWithdrawalRequest{
		UserID:         1,
		Amount:         decimal.NewFromInt(1),
		Currency:       domain.CryptoBTC,
		Address:        "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
		IdempotencyKey: "w1",
	}); err != nil {
		t.Fatalf("InitiateWithdrawal: %v", err)
	}

	if got := errorCount(t, "daily_limit_not_tracked", observability.OperationWithdrawal); got != before+1 {
		t.Errorf("errors_total{error_type=daily_limit_not_tracked} = %v, want %v", got, before+1)
	}
}

// The deposit is only "completed" once the wallet is credited. Recording the
// transition before the credit would report money that never arrived.
func TestHandleDepositFinished_WalletCreditFailureIsNotCountedAsCompleted(t *testing.T) {
	svc := NewWebhookService(
		&MockPaymentRepository{
			UpdateStatusFunc: func(context.Context, int64, domain.PaymentStatus, domain.PaymentStatus) error {
				return nil
			},
		},
		nil,
		&MockIdempotencyRepository{},
		nil,
		&MockNOWPaymentsClient{},
		&MockWalletClient{
			CreditWalletFunc: func(context.Context, client.CreditRequest) (*client.CreditResult, error) {
				return nil, errors.New("wallet unreachable")
			},
		},
		nil,
		noopTracer(),
	)

	payment := &domain.Payment{
		ID:             1,
		UserID:         1,
		PaymentID:      "np-1",
		CryptoCurrency: string(domain.CryptoBTC),
		FiatAmount:     decimal.NewFromInt(100),
		Status:         domain.PaymentStatusPending,
	}

	beforeCreditFailure := errorCount(t, "deposit_credit_failed", observability.OperationWebhook)
	beforeCompleted := depositCount(t, observability.DepositStatusCompleted, string(domain.CryptoBTC), observability.KYCUnknownLabel)

	if err := svc.handleDepositFinished(context.Background(), payment, client.WebhookPayload{}); err == nil {
		t.Fatal("expected the credit failure to propagate")
	}

	if got := errorCount(t, "deposit_credit_failed", observability.OperationWebhook); got != beforeCreditFailure+1 {
		t.Errorf("errors_total{error_type=deposit_credit_failed} = %v, want %v", got, beforeCreditFailure+1)
	}
	if got := depositCount(t, observability.DepositStatusCompleted, string(domain.CryptoBTC), observability.KYCUnknownLabel); got != beforeCompleted {
		t.Errorf("deposit counted as completed despite a failed credit: %v -> %v", beforeCompleted, got)
	}
}

func TestHandleDepositFinished_RecordsCompletedAfterCredit(t *testing.T) {
	svc := NewWebhookService(
		&MockPaymentRepository{
			UpdateStatusFunc: func(context.Context, int64, domain.PaymentStatus, domain.PaymentStatus) error {
				return nil
			},
		},
		nil,
		&MockIdempotencyRepository{},
		nil,
		&MockNOWPaymentsClient{},
		&MockWalletClient{},
		nil,
		noopTracer(),
	)

	payment := &domain.Payment{
		ID:             1,
		UserID:         1,
		PaymentID:      "np-1",
		CryptoCurrency: string(domain.CryptoBTC),
		FiatAmount:     decimal.NewFromInt(100),
		Status:         domain.PaymentStatusPending,
	}

	before := depositCount(t, observability.DepositStatusCompleted, string(domain.CryptoBTC), observability.KYCUnknownLabel)

	if err := svc.handleDepositFinished(context.Background(), payment, client.WebhookPayload{}); err != nil {
		t.Fatalf("handleDepositFinished: %v", err)
	}

	if got := depositCount(t, observability.DepositStatusCompleted, string(domain.CryptoBTC), observability.KYCUnknownLabel); got != before+1 {
		t.Errorf("deposits_total{status=completed} = %v, want %v", got, before+1)
	}
}

// A failed payout must give the money back. If the unlock fails the balance
// stays locked for a payout that never happened.
func TestHandleWithdrawalFailed_UnlockFailureIsRecorded(t *testing.T) {
	svc := NewWebhookService(
		nil,
		&MockWithdrawalRepository{
			UpdateStatusFunc: func(context.Context, int64, domain.WithdrawalStatus, domain.WithdrawalStatus) error {
				return nil
			},
		},
		&MockIdempotencyRepository{},
		nil,
		&MockNOWPaymentsClient{},
		&MockWalletClient{
			UnlockFundsFunc: func(context.Context, string, string) error {
				return errors.New("wallet unreachable")
			},
		},
		nil,
		noopTracer(),
	)

	withdrawal := &domain.Withdrawal{
		ID:             1,
		UserID:         1,
		WithdrawalID:   "np-w-1",
		CryptoCurrency: string(domain.CryptoBTC),
		FiatAmount:     decimal.NewFromInt(100),
		Status:         domain.WithdrawalStatusProcessing,
	}

	beforeUnlock := errorCount(t, "withdrawal_unlock_failed", observability.OperationWebhook)
	beforeFailed := withdrawalCount(t, observability.WithdrawalStatusFailed, string(domain.CryptoBTC), observability.KYCUnknownLabel)

	if err := svc.handleWithdrawalFailed(context.Background(), withdrawal); err != nil {
		t.Fatalf("handleWithdrawalFailed: %v", err)
	}

	if got := errorCount(t, "withdrawal_unlock_failed", observability.OperationWebhook); got != beforeUnlock+1 {
		t.Errorf("errors_total{error_type=withdrawal_unlock_failed} = %v, want %v", got, beforeUnlock+1)
	}
	if got := withdrawalCount(t, observability.WithdrawalStatusFailed, string(domain.CryptoBTC), observability.KYCUnknownLabel); got != beforeFailed+1 {
		t.Errorf("withdrawals_total{status=failed} = %v, want %v", got, beforeFailed+1)
	}
}

// An expired deposit is not the same event as a failed one: the player may
// still have funds in flight. The two statuses are kept apart on purpose.
func TestHandleDepositFailed_SeparatesExpiredFromFailed(t *testing.T) {
	svc := NewWebhookService(
		&MockPaymentRepository{
			UpdateStatusFunc: func(context.Context, int64, domain.PaymentStatus, domain.PaymentStatus) error {
				return nil
			},
		},
		nil,
		&MockIdempotencyRepository{},
		nil,
		&MockNOWPaymentsClient{},
		&MockWalletClient{},
		nil,
		noopTracer(),
	)

	cur := string(domain.CryptoBTC)
	payment := &domain.Payment{ID: 1, UserID: 1, PaymentID: "np-1", CryptoCurrency: cur, Status: domain.PaymentStatusPending}

	beforeFailed := depositCount(t, observability.DepositStatusFailed, cur, observability.KYCUnknownLabel)
	beforeExpired := depositCount(t, observability.DepositStatusExpired, cur, observability.KYCUnknownLabel)

	if err := svc.handleDepositFailed(context.Background(), payment, domain.PaymentStatusExpired); err != nil {
		t.Fatalf("handleDepositFailed: %v", err)
	}

	if got := depositCount(t, observability.DepositStatusExpired, cur, observability.KYCUnknownLabel); got != beforeExpired+1 {
		t.Errorf("deposits_total{status=expired} = %v, want %v", got, beforeExpired+1)
	}
	if got := depositCount(t, observability.DepositStatusFailed, cur, observability.KYCUnknownLabel); got != beforeFailed {
		t.Errorf("an expired deposit was also counted as failed: %v -> %v", beforeFailed, got)
	}
}

// Provider retries are routine. Counting a replayed webhook twice would make
// completions exceed the deposits that were actually opened.
func TestProcessDepositWebhook_ReplayRecordsNothing(t *testing.T) {
	cur := string(domain.CryptoBTC)
	svc := NewWebhookService(
		&MockPaymentRepository{
			GetByPaymentIDFunc: func(context.Context, string) (*domain.Payment, error) {
				return nil, errors.New("must not be reached")
			},
		},
		nil,
		&MockIdempotencyRepository{
			GetFunc: func(context.Context, string) ([]byte, bool, error) {
				return []byte("processed"), true, nil
			},
		},
		nil,
		&MockNOWPaymentsClient{},
		&MockWalletClient{
			CreditWalletFunc: func(context.Context, client.CreditRequest) (*client.CreditResult, error) {
				return nil, errors.New("must not be reached")
			},
		},
		nil,
		noopTracer(),
	)

	payload := []byte(`{"payment_id":"np-1","payment_status":"finished"}`)

	beforeCompleted := depositCount(t, observability.DepositStatusCompleted, cur, observability.KYCUnknownLabel)

	res, err := svc.ProcessDepositWebhook(context.Background(), ProcessWebhookRequest{
		Payload:   payload,
		Signature: "sig",
	})
	if err != nil {
		t.Fatalf("ProcessDepositWebhook: %v", err)
	}
	if !res.Processed {
		t.Fatal("expected the replay to be reported as processed")
	}

	if got := depositCount(t, observability.DepositStatusCompleted, cur, observability.KYCUnknownLabel); got != beforeCompleted {
		t.Errorf("webhook replay moved deposits_total: %v -> %v", beforeCompleted, got)
	}
}

func TestProcessDepositWebhook_InvalidSignatureIsRecorded(t *testing.T) {
	svc := NewWebhookService(
		&MockPaymentRepository{},
		nil,
		&MockIdempotencyRepository{},
		nil,
		&MockNOWPaymentsClient{
			VerifyWebhookSignatureFunc: func([]byte, string) bool { return false },
		},
		&MockWalletClient{},
		nil,
		noopTracer(),
	)

	before := errorCount(t, "webhook_invalid", observability.OperationWebhook)

	_, err := svc.ProcessDepositWebhook(context.Background(), ProcessWebhookRequest{
		Payload:   []byte(`{"payment_id":"np-1"}`),
		Signature: "forged",
	})
	if err == nil {
		t.Fatal("expected a forged signature to be rejected")
	}
	if domain.GetErrorCode(err) != domain.ErrCodeWebhookSignatureInvalid {
		t.Errorf("error code = %d, want %d", domain.GetErrorCode(err), domain.ErrCodeWebhookSignatureInvalid)
	}

	if got := errorCount(t, "webhook_invalid", observability.OperationWebhook); got != before+1 {
		t.Errorf("errors_total{error_type=webhook_invalid} = %v, want %v", got, before+1)
	}
}

// The signature check is the only thing standing between a forged webhook and
// a credited balance, so a rejected signature must never reach the wallet.
func TestProcessDepositWebhook_InvalidSignatureDoesNotCredit(t *testing.T) {
	credited := false
	svc := NewWebhookService(
		&MockPaymentRepository{},
		nil,
		&MockIdempotencyRepository{},
		nil,
		&MockNOWPaymentsClient{
			VerifyWebhookSignatureFunc: func([]byte, string) bool { return false },
		},
		&MockWalletClient{
			CreditWalletFunc: func(context.Context, client.CreditRequest) (*client.CreditResult, error) {
				credited = true
				return &client.CreditResult{}, nil
			},
		},
		nil,
		noopTracer(),
	)

	if _, err := svc.ProcessDepositWebhook(context.Background(), ProcessWebhookRequest{
		Payload:   []byte(`{"payment_id":"np-1","payment_status":"finished"}`),
		Signature: "forged",
	}); err == nil {
		t.Fatal("expected rejection")
	}

	if credited {
		t.Fatal("wallet was credited from an unsigned webhook")
	}
}

// Compile-time guard: the fixtures above must keep satisfying the repository
// contracts they are standing in for.
var (
	_ repository.PaymentRepository    = (*MockPaymentRepository)(nil)
	_ repository.WithdrawalRepository = (*MockWithdrawalRepository)(nil)
)
