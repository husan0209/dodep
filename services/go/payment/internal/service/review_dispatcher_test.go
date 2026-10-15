package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/opus-casino/payment/internal/client"
	"github.com/opus-casino/payment/internal/domain"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
)

func newTestReviewDispatcher(
	repo *MockWithdrawalRepository,
	wallet *MockWalletClient,
	np *MockNOWPaymentsClient,
) *ReviewDispatcher {
	return NewReviewDispatcher(repo, wallet, np, zerolog.Nop(), 20, "https://payments.example/ipn")
}

func TestReviewPolicy_FromEnv(t *testing.T) {
	t.Run("unset disables review", func(t *testing.T) {
		t.Setenv("WITHDRAWAL_MANUAL_REVIEW_THRESHOLD_USD", "")
		p := ReviewPolicyFromEnv()
		if p.Enabled() {
			t.Fatal("expected manual review disabled")
		}
		if p.RequiresManualReview(decimal.NewFromInt(10_000)) {
			t.Fatal("disabled policy must never require review")
		}
	})

	t.Run("invalid value disables review", func(t *testing.T) {
		t.Setenv("WITHDRAWAL_MANUAL_REVIEW_THRESHOLD_USD", "not-a-number")
		if ReviewPolicyFromEnv().Enabled() {
			t.Fatal("unparseable threshold must fail open to instant payouts")
		}
	})

	t.Run("negative value disables review", func(t *testing.T) {
		t.Setenv("WITHDRAWAL_MANUAL_REVIEW_THRESHOLD_USD", "-50")
		if ReviewPolicyFromEnv().Enabled() {
			t.Fatal("negative threshold must disable review")
		}
	})

	t.Run("amounts at or above the threshold require review", func(t *testing.T) {
		t.Setenv("WITHDRAWAL_MANUAL_REVIEW_THRESHOLD_USD", "1000")
		p := ReviewPolicyFromEnv()
		if !p.Enabled() {
			t.Fatal("expected manual review enabled")
		}
		cases := []struct {
			amount string
			want   bool
		}{
			{"999.99", false},
			{"1000", true},
			{"1000.01", true},
		}
		for _, tc := range cases {
			got := p.RequiresManualReview(decimal.RequireFromString(tc.amount))
			if got != tc.want {
				t.Fatalf("amount %s: got %v, want %v", tc.amount, got, tc.want)
			}
		}
	})
}

func TestReviewDispatcher_RunOnce_Approved(t *testing.T) {
	decidedAt := time.Now().UTC()
	row := &domain.Withdrawal{
		ID:             7,
		UserID:         42,
		Amount:         decimal.NewFromFloat(0.5),
		FiatAmount:     decimal.NewFromInt(1500),
		CryptoCurrency: string(domain.CryptoUSDTTRX),
		Address:        "TXYZ",
		IdempotencyKey: "wd-7",
		LockID:         "lock-7",
		DecidedBy:      "admin-3",
		DecidedAt:      &decidedAt,
		Status:         domain.WithdrawalStatusApproved,
	}

	var gotPayoutReq client.CreatePayoutRequest
	var gotPayoutIDFor int64
	var gotPayoutID string
	var gotFrom, gotTo domain.WithdrawalStatus

	repo := &MockWithdrawalRepository{
		ClaimDecidedWithdrawalsFunc: func(_ context.Context, limit int) ([]*domain.Withdrawal, error) {
			if limit != 20 {
				t.Fatalf("expected default batch size 20, got %d", limit)
			}
			return []*domain.Withdrawal{row}, nil
		},
		UpdatePayoutIDFunc: func(_ context.Context, id int64, payoutID string) error {
			gotPayoutIDFor, gotPayoutID = id, payoutID
			return nil
		},
		UpdateStatusFunc: func(_ context.Context, id int64, from, to domain.WithdrawalStatus) error {
			gotFrom, gotTo = from, to
			return nil
		},
	}
	wallet := &MockWalletClient{
		UnlockFundsFunc: func(context.Context, string, string) error {
			t.Fatal("approved rows must not unlock funds")
			return nil
		},
	}
	np := &MockNOWPaymentsClient{
		CreatePayoutFunc: func(_ context.Context, req client.CreatePayoutRequest) (*client.CreatePayoutResponse, error) {
			gotPayoutReq = req
			return &client.CreatePayoutResponse{WithdrawalID: "np-payout-9"}, nil
		},
	}

	d := newTestReviewDispatcher(repo, wallet, np)
	if err := d.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if gotPayoutReq.Address != "TXYZ" || gotPayoutReq.Currency != "USDTTRC20" || !gotPayoutReq.Amount.Equal(decimal.NewFromFloat(0.5)) {
		t.Fatalf("payout request did not mirror the withdrawal: %+v", gotPayoutReq)
	}
	if gotPayoutIDFor != 7 || gotPayoutID != "np-payout-9" {
		t.Fatalf("payout id not persisted: id=%d payout=%q", gotPayoutIDFor, gotPayoutID)
	}
	if gotFrom != domain.WithdrawalStatusApproved || gotTo != domain.WithdrawalStatusSending {
		t.Fatalf("expected approved→sending, got %s→%s", gotFrom, gotTo)
	}
}

func TestReviewDispatcher_RunOnce_Rejected(t *testing.T) {
	decidedAt := time.Now().UTC()
	row := &domain.Withdrawal{
		ID:             8,
		UserID:         42,
		Amount:         decimal.NewFromInt(1),
		CryptoCurrency: "USDT_TRC20",
		IdempotencyKey: "wd-8",
		LockID:         "lock-8",
		DecidedBy:      "admin-4",
		DecidedAt:      &decidedAt,
		DecisionReason: "velocity pattern",
		Status:         domain.WithdrawalStatusRejected,
	}

	var unlockedLock, unlockedKey string
	var gotFrom, gotTo domain.WithdrawalStatus

	repo := &MockWithdrawalRepository{
		ClaimDecidedWithdrawalsFunc: func(_ context.Context, _ int) ([]*domain.Withdrawal, error) {
			return []*domain.Withdrawal{row}, nil
		},
		UpdateStatusFunc: func(_ context.Context, id int64, from, to domain.WithdrawalStatus) error {
			gotFrom, gotTo = from, to
			return nil
		},
	}
	np := &MockNOWPaymentsClient{
		CreatePayoutFunc: func(context.Context, client.CreatePayoutRequest) (*client.CreatePayoutResponse, error) {
			t.Fatal("rejected rows must not create payouts")
			return nil, nil
		},
	}
	wallet := &MockWalletClient{
		UnlockFundsFunc: func(_ context.Context, lockID, idempotencyKey string) error {
			unlockedLock, unlockedKey = lockID, idempotencyKey
			return nil
		},
	}

	d := newTestReviewDispatcher(repo, wallet, np)
	if err := d.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if unlockedLock != "lock-8" || unlockedKey != "wd-8_unlock" {
		t.Fatalf("funds not returned with the original lock: lock=%q key=%q", unlockedLock, unlockedKey)
	}
	if gotFrom != domain.WithdrawalStatusRejected || gotTo != domain.WithdrawalStatusCancelled {
		t.Fatalf("expected rejected→cancelled, got %s→%s", gotFrom, gotTo)
	}
}

func TestReviewDispatcher_ApprovedPayoutFailureKeepsClaim(t *testing.T) {
	row := &domain.Withdrawal{
		ID:             9,
		Amount:         decimal.NewFromInt(2),
		CryptoCurrency: "BTC",
		Address:        "bc1xyz",
		IdempotencyKey: "wd-9",
		LockID:         "lock-9",
		DecidedBy:      "admin-5",
		Status:         domain.WithdrawalStatusApproved,
	}

	repo := &MockWithdrawalRepository{
		ClaimDecidedWithdrawalsFunc: func(_ context.Context, _ int) ([]*domain.Withdrawal, error) {
			return []*domain.Withdrawal{row}, nil
		},
		UpdateStatusFunc: func(context.Context, int64, domain.WithdrawalStatus, domain.WithdrawalStatus) error {
			t.Fatal("no status change may happen when payout creation fails")
			return nil
		},
	}
	np := &MockNOWPaymentsClient{
		CreatePayoutFunc: func(context.Context, client.CreatePayoutRequest) (*client.CreatePayoutResponse, error) {
			return nil, errors.New("provider unavailable")
		},
	}
	wallet := &MockWalletClient{}

	d := newTestReviewDispatcher(repo, wallet, np)
	if err := d.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce must not surface processing failures: %v", err)
	}
	// The claim marker stays set: the row is NOT retried automatically.
	// Operators see the error log and act manually.
}

func TestReviewDispatcher_RejectedUnlockFailureKeepsClaim(t *testing.T) {
	row := &domain.Withdrawal{
		ID:             10,
		IdempotencyKey: "wd-10",
		LockID:         "lock-10",
		DecidedBy:      "admin-6",
		Status:         domain.WithdrawalStatusRejected,
	}

	repo := &MockWithdrawalRepository{
		ClaimDecidedWithdrawalsFunc: func(_ context.Context, _ int) ([]*domain.Withdrawal, error) {
			return []*domain.Withdrawal{row}, nil
		},
		UpdateStatusFunc: func(context.Context, int64, domain.WithdrawalStatus, domain.WithdrawalStatus) error {
			t.Fatal("no status change may happen when the fund return fails")
			return nil
		},
	}
	wallet := &MockWalletClient{
		UnlockFundsFunc: func(context.Context, string, string) error {
			return errors.New("wallet unreachable")
		},
	}
	np := &MockNOWPaymentsClient{}

	d := newTestReviewDispatcher(repo, wallet, np)
	if err := d.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce must not surface processing failures: %v", err)
	}
}

func TestReviewDispatcher_SkipsUnexpectedStatus(t *testing.T) {
	row := &domain.Withdrawal{ID: 11, Status: domain.WithdrawalStatusPendingReview}

	repo := &MockWithdrawalRepository{
		ClaimDecidedWithdrawalsFunc: func(_ context.Context, _ int) ([]*domain.Withdrawal, error) {
			return []*domain.Withdrawal{row}, nil
		},
	}
	d := newTestReviewDispatcher(repo, &MockWalletClient{}, &MockNOWPaymentsClient{})
	if err := d.RunOnce(context.Background()); err != nil {
		t.Fatalf("unexpected statuses must be skipped, not fail the pass: %v", err)
	}
}
