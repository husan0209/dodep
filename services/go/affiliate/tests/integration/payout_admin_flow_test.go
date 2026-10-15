// Package integration payout admin flow tests pin the service contracts the
// admin HTTP routes depend on: approve-payout must enforce the KYC gate, and
// payout requests must refuse unverified methods.
//
// Like the rest of this package, every test is skipped unless
// TEST_DATABASE_URL points at a reachable PostgreSQL.
package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/opus-casino/affiliate/internal/domain"
	"github.com/opus-casino/affiliate/internal/repository"
	"github.com/opus-casino/affiliate/internal/service"
)

func TestApprovePayout_EnforcesKYCGate(t *testing.T) {
	db := newTestDB(t)
	applyMigrations(t, db)

	repo := repository.NewGormAffiliateRepository(db, nil)
	svc := service.NewAffiliateService(repo, nil)
	ctx := context.Background()

	mustExec := func(q string, args ...any) {
		t.Helper()
		if err := db.Exec(q, args...).Error; err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}

	// Re-runnable: drop anything a previous run left behind.
	mustExec(`DELETE FROM affiliate_payouts WHERE affiliate_id IN (SELECT id FROM affiliate_profiles WHERE affiliate_code LIKE 'KYC-%')`)
	mustExec(`DELETE FROM affiliate_payout_methods WHERE affiliate_id IN (SELECT id FROM affiliate_profiles WHERE affiliate_code LIKE 'KYC-%')`)
	mustExec(`DELETE FROM affiliate_profiles WHERE affiliate_code LIKE 'KYC-%'`)

	profileID := uuid.New()
	methodID := uuid.New()
	payoutID := uuid.New()

	mustExec(`INSERT INTO affiliate_profiles (id, user_id, status, affiliate_code, commission_rate, kyc_required)
	          VALUES (?, 920001, 'active', ?, 0.20, TRUE)`, profileID, "KYC-"+uuid.NewString()[:8])
	mustExec(`INSERT INTO affiliate_payout_methods (id, affiliate_id, method_type, details_encrypted, is_verified)
	          VALUES (?, ?, 'bank_transfer', 'enc', TRUE)`, methodID, profileID)
	mustExec(`INSERT INTO affiliate_payouts (id, affiliate_id, amount, currency, method_id, idempotency_key, status)
	          VALUES (?, ?, 150.00000000, 'USD', ?, ?, 'requested')`,
		payoutID, profileID, methodID, "kyc-gate-"+uuid.NewString())

	// Without the finance operator's KYC confirmation the approval must fail
	// closed when the profile mandates KYC.
	_, err := svc.ApproveAffiliatePayout(ctx, service.ApproveAffiliatePayoutInput{
		PayoutID:   payoutID,
		ApprovedBy: "admin-1",
		// KYCApproved: false
	})
	if !errors.Is(err, domain.ErrAffiliateKYCRequired) {
		t.Fatalf("expected ErrAffiliateKYCRequired, got %v", err)
	}

	// With the confirmation the same payout approves.
	payout, err := svc.ApproveAffiliatePayout(ctx, service.ApproveAffiliatePayoutInput{
		PayoutID:    payoutID,
		ApprovedBy:  "admin-1",
		KYCApproved: true,
	})
	if err != nil {
		t.Fatalf("approve with kyc_approved=true failed: %v", err)
	}
	if payout.Status != domain.PayoutStatusApproved {
		t.Fatalf("expected status approved, got %q", payout.Status)
	}
	if payout.ApprovedBy != "admin-1" {
		t.Fatalf("expected approved_by admin-1, got %q", payout.ApprovedBy)
	}
}

func TestRequestPayout_RejectsUnverifiedMethod(t *testing.T) {
	db := newTestDB(t)
	applyMigrations(t, db)

	repo := repository.NewGormAffiliateRepository(db, nil)
	svc := service.NewAffiliateService(repo, nil)
	ctx := context.Background()

	mustExec := func(q string, args ...any) {
		t.Helper()
		if err := db.Exec(q, args...).Error; err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}

	mustExec(`DELETE FROM affiliate_payouts WHERE affiliate_id IN (SELECT id FROM affiliate_profiles WHERE affiliate_code LIKE 'UNV-%')`)
	mustExec(`DELETE FROM affiliate_payout_methods WHERE affiliate_id IN (SELECT id FROM affiliate_profiles WHERE affiliate_code LIKE 'UNV-%')`)
	mustExec(`DELETE FROM affiliate_profiles WHERE affiliate_code LIKE 'UNV-%'`)

	profileID := uuid.New()
	methodID := uuid.New()

	mustExec(`INSERT INTO affiliate_profiles (id, user_id, status, affiliate_code, commission_rate, kyc_required)
	          VALUES (?, 920002, 'active', ?, 0.20, FALSE)`, profileID, "UNV-"+uuid.NewString()[:8])
	mustExec(`INSERT INTO affiliate_payout_methods (id, affiliate_id, method_type, details_encrypted, is_verified)
	          VALUES (?, ?, 'crypto', 'enc', FALSE)`, methodID, profileID)

	// A payout to an unverified method must be refused even though the
	// profile does not mandate KYC. Partner updates must never be able to
	// flip is_verified (see PUT /payout-methods/:id).
	_, err := svc.RequestPayout(ctx, service.RequestPayoutInput{
		AffiliateID:    profileID,
		MethodID:       methodID,
		Amount:         decimal.NewFromInt(50),
		IdempotencyKey: "unverified-" + uuid.NewString(),
	})
	if !errors.Is(err, service.ErrPayoutMethodUnverified) {
		t.Fatalf("expected ErrPayoutMethodUnverified, got %v", err)
	}
}
