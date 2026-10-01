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

// TestLedgerTracksMoneyMovement verifies every affiliate money movement lands
// in the separate affiliate ledger (never in the player wallet):
//
//	accrual -> pending, hold release -> available, payout paid -> paid,
//	manual adjustment -> adjusted, reconciliation finds no divergence.
func TestLedgerTracksMoneyMovement(t *testing.T) {
	svc, ctx := setupService(t)

	const affiliateUserID int64 = 5005
	const referredUserID int64 = 6006

	mustEnrollAndApprove(t, ctx, svc, affiliateUserID)
	profile, err := svc.GetProfileByUserID(ctx, affiliateUserID)
	if err != nil {
		t.Fatalf("get profile failed: %v", err)
	}

	method, err := svc.CreatePayoutMethod(ctx, service.CreatePayoutMethodInput{
		AffiliateID:   profile.ID,
		MethodType:    domain.PayoutMethodTypeCrypto,
		DisplayName:   "USDT TRC20",
		DetailsMasked: "TX***abcd",
		IsDefault:     true,
		IsVerified:    true,
	})
	if err != nil {
		t.Fatalf("create payout method failed: %v", err)
	}

	now := time.Now().UTC()
	wantCommission := decimal.RequireFromString("200") // NGR 1000 * rate 0.20

	if _, err := svc.CalculateCommission(ctx, service.CalculateCommissionInput{
		AffiliateID:    profile.ID,
		ReferredUserID: referredUserID,
		SourceType:     "sports",
		SourceID:       "bet-1",
		PeriodStart:    now.Add(-24 * time.Hour),
		PeriodEnd:      now,
		GGRAmount:      decimal.RequireFromString("2000"),
		NGRAmount:      decimal.RequireFromString("1000"),
		IdempotencyKey: uuid.NewString(),
	}); err != nil {
		t.Fatalf("calculate commission failed: %v", err)
	}

	// 1. After accrual: pending = commission, everything else zero.
	balances, err := svc.ListLedgerBalances(ctx, profile.ID)
	if err != nil {
		t.Fatalf("list ledger balances failed: %v", err)
	}
	if got := ledgerBalance(balances, repository.LedgerAccountPending); !got.Equal(wantCommission) {
		t.Fatalf("expected pending=%s, got %s", wantCommission, got)
	}
	if got := ledgerBalance(balances, repository.LedgerAccountAvailable); !got.IsZero() {
		t.Fatalf("expected available=0 before hold release, got %s", got)
	}

	// 2. Hold release moves pending -> available.
	released, err := svc.ReleaseHeldCommissions(ctx, now.Add(15*24*time.Hour))
	if err != nil {
		t.Fatalf("release held commissions failed: %v", err)
	}
	if released != 1 {
		t.Fatalf("expected 1 released earning, got %d", released)
	}
	balances, err = svc.ListLedgerBalances(ctx, profile.ID)
	if err != nil {
		t.Fatalf("list ledger balances failed: %v", err)
	}
	if got := ledgerBalance(balances, repository.LedgerAccountPending); !got.IsZero() {
		t.Fatalf("expected pending=0 after release, got %s", got)
	}
	if got := ledgerBalance(balances, repository.LedgerAccountAvailable); !got.Equal(wantCommission) {
		t.Fatalf("expected available=%s after release, got %s", wantCommission, got)
	}

	// 3. Manual adjustment is booked on the `adjusted` account.
	if _, err := svc.CreateAdjustment(ctx, service.CreateAdjustmentInput{
		AffiliateID:    profile.ID,
		AdjustmentType: domain.AdjustmentTypeCredit,
		Amount:         decimal.RequireFromString("50"),
		Reason:         "goodwill",
		CreatedBy:      "admin@example.com",
	}); err != nil {
		t.Fatalf("create adjustment failed: %v", err)
	}
	balances, _ = svc.ListLedgerBalances(ctx, profile.ID)
	if got := ledgerBalance(balances, repository.LedgerAccountAdjusted); !got.Equal(decimal.RequireFromString("50")) {
		t.Fatalf("expected adjusted=50, got %s", got)
	}

	// 4. Payout settled as paid moves available -> paid.
	payout, err := svc.RequestPayout(ctx, service.RequestPayoutInput{
		AffiliateID:    profile.ID,
		MethodID:       method.ID,
		Amount:         decimal.RequireFromString("150"),
		IdempotencyKey: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("request payout failed: %v", err)
	}
	if _, err := svc.ApproveAffiliatePayout(ctx, service.ApproveAffiliatePayoutInput{
		AffiliateID:       profile.ID,
		PayoutID:          payout.ID,
		ApprovedBy:        "admin@example.com",
		ProviderReference: "psp-1",
	}); err != nil {
		t.Fatalf("approve payout failed: %v", err)
	}
	balances, _ = svc.ListLedgerBalances(ctx, profile.ID)
	if got := ledgerBalance(balances, repository.LedgerAccountAvailable); !got.Equal(decimal.RequireFromString("50")) {
		t.Fatalf("expected available=50 after payout, got %s", got)
	}
	if got := ledgerBalance(balances, repository.LedgerAccountPaid); !got.Equal(decimal.RequireFromString("150")) {
		t.Fatalf("expected paid=150, got %s", got)
	}

	// 5. Ledger must reconcile with business tables.
	report, err := svc.ReconcileAffiliate(ctx, profile.ID)
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if !report.Balanced {
		t.Fatalf("expected balanced ledger, divergences: %v", report.Divergences)
	}

	// 6. Replaying the hold release must not double-post.
	if _, err := svc.ReleaseHeldCommissions(ctx, now.Add(30*24*time.Hour)); err != nil {
		t.Fatalf("second release failed: %v", err)
	}
	balances, _ = svc.ListLedgerBalances(ctx, profile.ID)
	if got := ledgerBalance(balances, repository.LedgerAccountAvailable); !got.Equal(decimal.RequireFromString("50")) {
		t.Fatalf("repeat release changed available balance: %s", got)
	}
}

// TestLedgerReconciliationDetectsDivergence corrupts a business row and
// asserts reconciliation reports it instead of silently passing.
func TestLedgerReconciliationDetectsDivergence(t *testing.T) {
	svc, ctx := setupService(t)

	const affiliateUserID int64 = 5105
	mustEnrollAndApprove(t, ctx, svc, affiliateUserID)
	profile, err := svc.GetProfileByUserID(ctx, affiliateUserID)
	if err != nil {
		t.Fatalf("get profile failed: %v", err)
	}

	now := time.Now().UTC()
	if _, err := svc.CalculateCommission(ctx, service.CalculateCommissionInput{
		AffiliateID:    profile.ID,
		ReferredUserID: 6106,
		SourceType:     "casino",
		SourceID:       "round-1",
		PeriodStart:    now.Add(-24 * time.Hour),
		PeriodEnd:      now,
		GGRAmount:      decimal.RequireFromString("1000"),
		NGRAmount:      decimal.RequireFromString("500"),
		IdempotencyKey: uuid.NewString(),
	}); err != nil {
		t.Fatalf("calculate commission failed: %v", err)
	}

	report, err := svc.ReconcileAffiliate(ctx, profile.ID)
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if !report.Balanced {
		t.Fatalf("fresh ledger must reconcile, divergences: %v", report.Divergences)
	}

	// Simulate a manual DB edit the ledger does not know about.
	if err := execTestSQL(t,
		`UPDATE affiliate_earnings SET commission_amount = 999 WHERE affiliate_id = ?`,
		profile.ID); err != nil {
		t.Fatalf("corrupt earning failed: %v", err)
	}

	report, err = svc.ReconcileAffiliate(ctx, profile.ID)
	if err != nil {
		t.Fatalf("reconcile after corruption failed: %v", err)
	}
	if report.Balanced {
		t.Fatal("expected divergence to be detected")
	}
	if len(report.Divergences) == 0 {
		t.Fatal("expected at least one divergence description")
	}
}

// TestEarningAccrualIsIdempotent proves upstream retries cannot double-pay.
func TestEarningAccrualIsIdempotent(t *testing.T) {
	svc, ctx := setupService(t)

	const affiliateUserID int64 = 5205
	mustEnrollAndApprove(t, ctx, svc, affiliateUserID)
	profile, err := svc.GetProfileByUserID(ctx, affiliateUserID)
	if err != nil {
		t.Fatalf("get profile failed: %v", err)
	}

	now := time.Now().UTC()
	key := uuid.NewString()
	input := service.CalculateCommissionInput{
		AffiliateID:    profile.ID,
		ReferredUserID: 6206,
		SourceType:     "sports",
		SourceID:       "bet-42",
		PeriodStart:    now.Add(-24 * time.Hour),
		PeriodEnd:      now,
		GGRAmount:      decimal.RequireFromString("2000"),
		NGRAmount:      decimal.RequireFromString("1000"),
		IdempotencyKey: key,
	}

	first, err := svc.CalculateCommission(ctx, input)
	if err != nil {
		t.Fatalf("first accrual failed: %v", err)
	}
	second, err := svc.CalculateCommission(ctx, input)
	if err != nil {
		t.Fatalf("second accrual failed: %v", err)
	}

	if first.ID != second.ID {
		t.Fatalf("idempotent accrual must return the same earning: %s vs %s", first.ID, second.ID)
	}

	earnings, err := svc.ListAffiliateEarnings(ctx, profile.ID, domain.EarningStatusAccrued, 50)
	if err != nil {
		t.Fatalf("list earnings failed: %v", err)
	}
	if len(earnings) != 1 {
		t.Fatalf("expected exactly 1 earning, got %d", len(earnings))
	}

	balances, err := svc.ListLedgerBalances(ctx, profile.ID)
	if err != nil {
		t.Fatalf("list ledger balances failed: %v", err)
	}
	if got := ledgerBalance(balances, repository.LedgerAccountPending); !got.Equal(decimal.RequireFromString("200")) {
		t.Fatalf("ledger pending must be credited exactly once, got %s", got)
	}
}

func ledgerBalance(
	balances []repository.LedgerAccountBalance,
	accountType string,
) decimal.Decimal {
	for _, b := range balances {
		if b.AccountType == accountType {
			return b.Balance
		}
	}
	return decimal.Zero
}