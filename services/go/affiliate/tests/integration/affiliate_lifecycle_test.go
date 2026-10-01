package integration

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opus-casino/affiliate/internal/domain"
	"github.com/opus-casino/affiliate/internal/service"
	"github.com/shopspring/decimal"
)

// TestAffiliateLifecycle covers the MVP money path on a real database:
// enroll -> approve -> link -> click -> bind -> commission -> hold release
// -> payout request -> payout approve.
func TestAffiliateLifecycle(t *testing.T) {
	svc, ctx := setupService(t)

	const affiliateUserID int64 = 1001
	const referredUserID int64 = 2002

	mustEnrollAndApprove(t, ctx, svc, affiliateUserID)

	profile, err := svc.GetProfileByUserID(ctx, affiliateUserID)
	if err != nil {
		t.Fatalf("get profile failed: %v", err)
	}
	if profile.Status != domain.AffiliateStatusActive {
		t.Fatalf("expected active profile, got %s", profile.Status)
	}
	if profile.AffiliateCode == "" {
		t.Fatal("expected generated affiliate code")
	}

	link, err := svc.CreateAffiliateLink(ctx, service.CreateAffiliateLinkInput{
		AffiliateID:  profile.ID,
		CampaignName: "main",
		LandingPage:  "/",
		UTMSource:    "test",
	})
	if err != nil {
		t.Fatalf("create link failed: %v", err)
	}

	click, err := svc.TrackAffiliateClick(ctx, service.TrackAffiliateClickInput{
		AffiliateCode: profile.AffiliateCode,
		Campaign:      "main",
		LandingPage:   "/",
		IPHash:        "ip",
		CountryCode:   "US",
	})
	if err != nil {
		t.Fatalf("track click failed: %v", err)
	}

	if _, err := svc.BindReferredUser(ctx, service.BindReferredUserInput{
		AffiliateID:    profile.ID,
		ReferredUserID: referredUserID,
		ClickID:        click.ClickID,
	}); err != nil {
		t.Fatalf("bind referred user failed: %v", err)
	}
	_ = link

	// One referred player belongs to exactly one affiliate.
	if _, err := svc.BindReferredUser(ctx, service.BindReferredUserInput{
		AffiliateID:    profile.ID,
		ReferredUserID: referredUserID,
		ClickID:        click.ClickID,
	}); err != domain.ErrAttributionAlreadyBound {
		t.Fatalf("expected ErrAttributionAlreadyBound, got %v", err)
	}

	// Self-referral is forbidden.
	if _, err := svc.BindReferredUser(ctx, service.BindReferredUserInput{
		AffiliateID:    profile.ID,
		ReferredUserID: affiliateUserID,
	}); err != domain.ErrSelfReferral {
		t.Fatalf("expected ErrSelfReferral, got %v", err)
	}

	now := time.Now().UTC()
	earning, err := svc.CalculateCommission(ctx, service.CalculateCommissionInput{
		AffiliateID:    profile.ID,
		ReferredUserID: referredUserID,
		SourceType:     "sports",
		SourceID:       "bet-1",
		PeriodStart:    now.Add(-24 * time.Hour),
		PeriodEnd:      now,
		GGRAmount:      decimal.RequireFromString("2000"),
		NGRAmount:      decimal.RequireFromString("1000"),
		IdempotencyKey: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("calculate commission failed: %v", err)
	}
	// revshare from NGR: 1000 * 0.20 = 200.
	if !earning.CommissionAmount.Equal(decimal.RequireFromString("200")) {
		t.Fatalf("expected commission 200, got %s", earning.CommissionAmount)
	}
	if earning.Status != domain.EarningStatusAccrued {
		t.Fatalf("expected accrued earning, got %s", earning.Status)
	}

	// Fresh accruals are still in hold.
	released, err := svc.ReleaseHeldCommissions(ctx, now)
	if err != nil {
		t.Fatalf("hold release failed: %v", err)
	}
	if released != 0 {
		t.Fatalf("expected 0 released before hold expiry, got %d", released)
	}

	// After the 14-day hold the earning becomes available.
	released, err = svc.ReleaseHeldCommissions(ctx, now.Add(15*24*time.Hour))
	if err != nil {
		t.Fatalf("hold release after hold failed: %v", err)
	}
	if released != 1 {
		t.Fatalf("expected 1 released earning, got %d", released)
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

	payout, err := svc.RequestPayout(ctx, service.RequestPayoutInput{
		AffiliateID:    profile.ID,
		MethodID:       method.ID,
		Amount:         decimal.RequireFromString("150"),
		IdempotencyKey: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("request payout failed: %v", err)
	}
	if payout.Status != domain.PayoutStatusRequested {
		t.Fatalf("expected requested payout, got %s", payout.Status)
	}

	// Over-withdrawal beyond the available balance is rejected.
	if _, err := svc.RequestPayout(ctx, service.RequestPayoutInput{
		AffiliateID:    profile.ID,
		MethodID:       method.ID,
		Amount:         decimal.RequireFromString("1000000"),
		IdempotencyKey: uuid.NewString(),
	}); err == nil {
		t.Fatal("expected over-withdrawal to fail")
	}

	paid, err := svc.ApproveAffiliatePayout(ctx, service.ApproveAffiliatePayoutInput{
		AffiliateID:       profile.ID,
		PayoutID:          payout.ID,
		ApprovedBy:        "admin@example.com",
		ProviderReference: "psp-test-1",
	})
	if err != nil {
		t.Fatalf("approve payout failed: %v", err)
	}
	if paid.Status != domain.PayoutStatusPaid {
		t.Fatalf("expected paid payout, got %s", paid.Status)
	}
}

// TestEnrollmentDuplicate verifies the pending-review unique guard.
func TestEnrollmentDuplicate(t *testing.T) {
	svc, ctx := setupService(t)

	const userID int64 = 3003
	if _, err := svc.EnrollAffiliate(ctx, service.EnrollAffiliateInput{
		UserID: userID,
	}); err != nil {
		t.Fatalf("first enroll failed: %v", err)
	}
	if _, err := svc.EnrollAffiliate(ctx, service.EnrollAffiliateInput{
		UserID: userID,
	}); err != domain.ErrEnrollmentAlreadyPending {
		t.Fatalf("expected ErrEnrollmentAlreadyPending, got %v", err)
	}
}
