package service

import (
	"context"
	"testing"

	"github.com/opus-casino/payment/internal/client"
	"github.com/opus-casino/payment/internal/domain"
	"github.com/shopspring/decimal"
)

// reviewGateService builds a WithdrawalService with manual review enabled at a
// $1000 fiat threshold and mocks that record whether a payout was created and
// what was persisted.
func reviewGateService(t *testing.T, repo *MockWithdrawalRepository, wallet *MockWalletClient, np *MockNOWPaymentsClient) *WithdrawalService {
	t.Helper()
	return NewWithdrawalService(
		repo,
		&MockIdempotencyRepository{},
		&MockExchangeRateRepository{
			GetFunc: func(context.Context, string, string) (*decimal.Decimal, error) {
				rate := decimal.NewFromInt(45000) // 1 BTC = 45000 USD
				return &rate, nil
			},
		},
		&MockDailyLimitsRepository{},
		np,
		wallet,
		&MockUserClient{
			GetKYCLevelFunc: func(context.Context, int64) (int, error) { return 2, nil },
		},
		nil,
		noopTracer(),
		"",
		WithReviewPolicy(&ReviewPolicy{Threshold: decimal.NewFromInt(1000)}),
	)
}

// A withdrawal at/above the fiat threshold must NOT create a payout: the row
// is parked as pending_review with the wallet lock, so funds stay locked
// until an operator approves or rejects.
func TestInitiateWithdrawal_AboveThreshold_QueuesForReview(t *testing.T) {
	var created *domain.Withdrawal
	repo := &MockWithdrawalRepository{
		GetByIDempotencyKeyFunc: func(context.Context, string) (*domain.Withdrawal, error) {
			return nil, nil
		},
		CreateFunc: func(_ context.Context, w *domain.Withdrawal) error {
			w.ID = 1
			created = w
			return nil
		},
	}
	payoutCreated := false
	np := &MockNOWPaymentsClient{
		CreatePayoutFunc: func(context.Context, client.CreatePayoutRequest) (*client.CreatePayoutResponse, error) {
			payoutCreated = true
			return &client.CreatePayoutResponse{WithdrawalID: "np-should-not-exist"}, nil
		},
	}
	wallet := &MockWalletClient{
		LockFundsFunc: func(context.Context, client.LockRequest) (*client.LockResult, error) {
			return &client.LockResult{LockID: "lock-review-1"}, nil
		},
		UnlockFundsFunc: func(context.Context, string, string) error {
			t.Error("funds must stay locked for queued withdrawals")
			return nil
		},
	}

	svc := reviewGateService(t, repo, wallet, np)
	resp, err := svc.InitiateWithdrawal(context.Background(), InitiateWithdrawalRequest{
		UserID:         7,
		Amount:         decimal.NewFromInt(1), // 45000 USD — above the threshold
		Currency:       domain.CryptoBTC,
		Address:        "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
		IdempotencyKey: "review-1",
	})
	if err != nil {
		t.Fatalf("InitiateWithdrawal: %v", err)
	}

	if payoutCreated {
		t.Fatal("no payout may be created for a review-queued withdrawal")
	}
	if resp.Status != string(domain.WithdrawalStatusPendingReview) {
		t.Fatalf("response status = %q, want %q", resp.Status, domain.WithdrawalStatusPendingReview)
	}
	if created == nil {
		t.Fatal("withdrawal row was not persisted")
	}
	if created.Status != domain.WithdrawalStatusPendingReview {
		t.Errorf("row status = %q, want %q", created.Status, domain.WithdrawalStatusPendingReview)
	}
	if created.LockID != "lock-review-1" {
		t.Errorf("row lock_id = %q, want the wallet lock id", created.LockID)
	}
	if created.WithdrawalID != "" {
		t.Errorf("row must not carry a payout id yet, got %q", created.WithdrawalID)
	}
}

// Below the threshold the instant-payout flow is unchanged: the payout is
// created and the row goes straight to processing.
func TestInitiateWithdrawal_BelowThreshold_InstantPayout(t *testing.T) {
	var created *domain.Withdrawal
	repo := &MockWithdrawalRepository{
		GetByIDempotencyKeyFunc: func(context.Context, string) (*domain.Withdrawal, error) {
			return nil, nil
		},
		CreateFunc: func(_ context.Context, w *domain.Withdrawal) error {
			w.ID = 1
			created = w
			return nil
		},
	}
	payoutCreated := false
	np := &MockNOWPaymentsClient{
		CreatePayoutFunc: func(_ context.Context, req client.CreatePayoutRequest) (*client.CreatePayoutResponse, error) {
			payoutCreated = true
			return &client.CreatePayoutResponse{WithdrawalID: "np-instant-1", Amount: req.Amount}, nil
		},
	}
	wallet := &MockWalletClient{
		LockFundsFunc: func(context.Context, client.LockRequest) (*client.LockResult, error) {
			return &client.LockResult{LockID: "lock-instant-1"}, nil
		},
	}

	// 0.001 BTC = 45 USD — below the $1000 threshold.
	svc := reviewGateService(t, repo, wallet, np)
	resp, err := svc.InitiateWithdrawal(context.Background(), InitiateWithdrawalRequest{
		UserID:         7,
		Amount:         decimal.NewFromFloat(0.001),
		Currency:       domain.CryptoBTC,
		Address:        "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
		IdempotencyKey: "instant-1",
	})
	if err != nil {
		t.Fatalf("InitiateWithdrawal: %v", err)
	}

	if !payoutCreated {
		t.Fatal("below-threshold withdrawals must keep the instant payout flow")
	}
	if resp.Status != string(domain.WithdrawalStatusProcessing) {
		t.Fatalf("response status = %q, want %q", resp.Status, domain.WithdrawalStatusProcessing)
	}
	if created.Status != domain.WithdrawalStatusProcessing {
		t.Errorf("row status = %q, want %q", created.Status, domain.WithdrawalStatusProcessing)
	}
	if created.WithdrawalID != "np-instant-1" {
		t.Errorf("row payout id = %q, want np-instant-1", created.WithdrawalID)
	}
}
