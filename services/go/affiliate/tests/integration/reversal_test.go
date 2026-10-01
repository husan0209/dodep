package integration

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opus-casino/affiliate/internal/domain"
	"github.com/opus-casino/affiliate/internal/repository"
	"github.com/opus-casino/affiliate/internal/service"
	"github.com/shopspring/decimal"
)

// TestReverseAccruedCommission cancels money that is still in hold:
// pending -> reversed, and the earning is no longer withdrawable.
func TestReverseAccruedCommission(t *testing.T) {
	svc, ctx := setupService(t)

	const affiliateUserID int64 = 8008
	mustEnrollAndApprove(t, ctx, svc, affiliateUserID)
	profile, err := svc.GetProfileByUserID(ctx, affiliateUserID)
	if err != nil {
		t.Fatalf("get profile failed: %v", err)
	}

	now := time.Now().UTC()
	earning, err := svc.CalculateCommission(ctx, service.CalculateCommissionInput{
		AffiliateID:    profile.ID,
		ReferredUserID: 8100,
		SourceType:     "casino",
		SourceID:       uuid.NewString(),
		PeriodStart:    now.Add(-24 * time.Hour),
		PeriodEnd:      now,
		GGRAmount:      decimal.RequireFromString("1000"),
		NGRAmount:      decimal.RequireFromString("1000"),
		IdempotencyKey: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("calculate commission failed: %v", err)
	}
	commission := decimal.RequireFromString("200")

	balances, _ := svc.ListLedgerBalances(ctx, profile.ID)
	if got := ledgerBalance(balances, repository.LedgerAccountPending); !got.Equal(commission) {
		t.Fatalf("expected pending=%s, got %s", commission, got)
	}

	reversed, err := svc.ReverseCommission(ctx, service.ReverseCommissionInput{
		AffiliateID: profile.ID,
		EarningID:   earning.ID,
		Reason:      "chargeback on referred player",
		ReversedBy:  "risk@example.com",
	})
	if err != nil {
		t.Fatalf("reverse commission failed: %v", err)
	}
	if reversed.Status != domain.EarningStatusReversed {
		t.Fatalf("expected reversed status, got %s", reversed.Status)
	}

	balances, err = svc.ListLedgerBalances(ctx, profile.ID)
	if err != nil {
		t.Fatalf("list ledger balances failed: %v", err)
	}
	if got := ledgerBalance(balances, repository.LedgerAccountPending); !got.IsZero() {
		t.Fatalf("expected pending=0 after reversal, got %s", got)
	}
	if got := ledgerBalance(balances, repository.LedgerAccountReversed); !got.Equal(commission) {
		t.Fatalf("expected reversed=%s, got %s", commission, got)
	}

	// A reversed earning must never become available, even after hold expiry.
	if _, err := svc.ReleaseHeldCommissions(ctx, now.Add(30*24*time.Hour)); err != nil {
		t.Fatalf("release held commissions failed: %v", err)
	}
	balances, _ = svc.ListLedgerBalances(ctx, profile.ID)
	if got := ledgerBalance(balances, repository.LedgerAccountAvailable); !got.IsZero() {
		t.Fatalf("reversed commission must not become available, got %s", got)
	}

	// Ledger must still reconcile with the business tables.
	report, err := svc.ReconcileAffiliate(ctx, profile.ID)
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if !report.Balanced {
		t.Fatalf("expected balanced ledger, divergences: %v", report.Divergences)
	}

	// Reversal is idempotent.
	if _, err := svc.ReverseCommission(ctx, service.ReverseCommissionInput{
		AffiliateID: profile.ID,
		EarningID:   earning.ID,
		Reason:      "duplicate call",
		ReversedBy:  "risk@example.com",
	}); err != nil {
		t.Fatalf("repeat reversal failed: %v", err)
	}
	balances, _ = svc.ListLedgerBalances(ctx, profile.ID)
	if got := ledgerBalance(balances, repository.LedgerAccountReversed); !got.Equal(commission) {
		t.Fatalf("repeat reversal double-posted: reversed=%s", got)
	}
}

// TestReverseAvailableCommissionAfterHoldRelease verifies the reversal
// debits `available` (not `pending`) once the hold has been released.
func TestReverseAvailableCommissionAfterHoldRelease(t *testing.T) {
	svc, ctx := setupService(t)

	const affiliateUserID int64 = 8208
	mustEnrollAndApprove(t, ctx, svc, affiliateUserID)
	profile, err := svc.GetProfileByUserID(ctx, affiliateUserID)
	if err != nil {
		t.Fatalf("get profile failed: %v", err)
	}

	now := time.Now().UTC()
	earning, err := svc.CalculateCommission(ctx, service.CalculateCommissionInput{
		AffiliateID:    profile.ID,
		ReferredUserID: 8300,
		SourceType:     "sports",
		SourceID:       uuid.NewString(),
		PeriodStart:    now.Add(-24 * time.Hour),
		PeriodEnd:      now,
		GGRAmount:      decimal.RequireFromString("2000"),
		NGRAmount:      decimal.RequireFromString("1000"),
		IdempotencyKey: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("calculate commission failed: %v", err)
	}
	commission := decimal.RequireFromString("200")

	if _, err := svc.ReleaseHeldCommissions(ctx, now.Add(15*24*time.Hour)); err != nil {
		t.Fatalf("release held commissions failed: %v", err)
	}
	balances, _ := svc.ListLedgerBalances(ctx, profile.ID)
	if got := ledgerBalance(balances, repository.LedgerAccountAvailable); !got.Equal(commission) {
		t.Fatalf("expected available=%s after release, got %s", commission, got)
	}

	if _, err := svc.ReverseCommission(ctx, service.ReverseCommissionInput{
		AffiliateID: profile.ID,
		EarningID:   earning.ID,
		Reason:      "bonus abuse confirmed",
		ReversedBy:  "risk@example.com",
	}); err != nil {
		t.Fatalf("reverse commission failed: %v", err)
	}

	balances, _ = svc.ListLedgerBalances(ctx, profile.ID)
	if got := ledgerBalance(balances, repository.LedgerAccountAvailable); !got.IsZero() {
		t.Fatalf("expected available=0 after reversal, got %s", got)
	}
	if got := ledgerBalance(balances, repository.LedgerAccountReversed); !got.Equal(commission) {
		t.Fatalf("expected reversed=%s, got %s", commission, got)
	}

	report, err := svc.ReconcileAffiliate(ctx, profile.ID)
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if !report.Balanced {
		t.Fatalf("expected balanced ledger, divergences: %v", report.Divergences)
	}
}

// TestReverseRequiresReasonAndExistingEarning covers the guard rails.
func TestReverseRequiresReasonAndExistingEarning(t *testing.T) {
	svc, ctx := setupService(t)

	const affiliateUserID int64 = 8408
	mustEnrollAndApprove(t, ctx, svc, affiliateUserID)
	profile, err := svc.GetProfileByUserID(ctx, affiliateUserID)
	if err != nil {
		t.Fatalf("get profile failed: %v", err)
	}

	// Reason is mandatory for auditability.
	_, err = svc.ReverseCommission(ctx, service.ReverseCommissionInput{
		AffiliateID: profile.ID,
		EarningID:   uuid.New(),
		ReversedBy:  "risk@example.com",
	})
	if err != domain.ErrValidationFailed {
		t.Fatalf("expected ErrValidationFailed for empty reason, got %v", err)
	}

	// Unknown earning.
	_, err = svc.ReverseCommission(ctx, service.ReverseCommissionInput{
		AffiliateID: profile.ID,
		EarningID:   uuid.New(),
		Reason:      "manual",
		ReversedBy:  "risk@example.com",
	})
	if err != domain.ErrEarningNotFound {
		t.Fatalf("expected ErrEarningNotFound, got %v", err)
	}
}