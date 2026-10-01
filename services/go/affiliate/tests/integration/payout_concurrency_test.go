package integration

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opus-casino/affiliate/internal/domain"
	"github.com/opus-casino/affiliate/internal/service"
	"github.com/shopspring/decimal"
)

// TestConcurrentPayoutsCannotOverdraw is the money-safety test: many
// simultaneous payout requests must never reserve more than the affiliate
// actually has available.
func TestConcurrentPayoutsCannotOverdraw(t *testing.T) {
	svc, ctx := setupService(t)

	const affiliateUserID int64 = 7007
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

	// Make exactly 300 available: hold=0 so the release covers the accrual.
	if err := execTestSQL(t,
		`UPDATE affiliate_profiles SET hold_period_days = 0 WHERE id = ?`, profile.ID); err != nil {
		t.Fatalf("set hold period failed: %v", err)
	}
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		if _, err := svc.CalculateCommission(ctx, service.CalculateCommissionInput{
			AffiliateID:    profile.ID,
			ReferredUserID: int64(7100 + i),
			SourceType:     "sports",
			SourceID:       uuid.NewString(),
			PeriodStart:    now.Add(-24 * time.Hour),
			PeriodEnd:      now,
			GGRAmount:      decimal.RequireFromString("500"),
			NGRAmount:      decimal.RequireFromString("500"),
			IdempotencyKey: uuid.NewString(),
		}); err != nil {
			t.Fatalf("accrue commission failed: %v", err)
		}
	}
	if _, err := svc.ReleaseHeldCommissions(ctx, now.Add(time.Minute)); err != nil {
		t.Fatalf("release held commissions failed: %v", err)
	}

	available, err := svc.GetProfileByID(ctx, profile.ID)
	if err != nil || available == nil {
		t.Fatalf("get profile by id failed: %v", err)
	}

	// 8 concurrent requests of 100 each against 300 available.
	const (
		requests   = 8
		perRequest = "100"
	)
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		granted = map[string]decimal.Decimal{}
		rejected int
	)

	start := make(chan struct{})
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // maximize contention
			payout, err := svc.RequestPayout(ctx, service.RequestPayoutInput{
				AffiliateID:    profile.ID,
				MethodID:       method.ID,
				Amount:         decimal.RequireFromString(perRequest),
				IdempotencyKey: uuid.NewString(),
			})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				rejected++
				if !errors.Is(err, domain.ErrInvalidPayoutAmount) &&
					!errors.Is(err, domain.ErrMinPayoutNotReached) {
					t.Errorf("unexpected payout error: %v", err)
				}
				return
			}
			granted[payout.ID.String()] = payout.Amount
		}()
	}
	close(start)
	wg.Wait()

	reserved := decimal.Zero
	for _, amount := range granted {
		reserved = reserved.Add(amount)
	}
	if reserved.GreaterThan(decimal.RequireFromString("300")) {
		t.Fatalf("OVERDRAW: reserved %s exceeds available 300", reserved)
	}
	if len(granted)+rejected != requests {
		t.Fatalf("unexpected accounting: granted=%d rejected=%d requests=%d",
			len(granted), rejected, requests)
	}
	if len(granted) > 3 {
		t.Fatalf("expected at most 3 payouts of 100 against 300 available, got %d", len(granted))
	}
	if rejected == 0 {
		t.Fatal("expected some requests to be rejected once funds run out")
	}
}

// TestPayoutRequestIsIdempotent proves a retried request reserves once.
func TestPayoutRequestIsIdempotent(t *testing.T) {
	svc, ctx := setupService(t)

	const affiliateUserID int64 = 7207
	mustEnrollAndApprove(t, ctx, svc, affiliateUserID)
	profile, err := svc.GetProfileByUserID(ctx, affiliateUserID)
	if err != nil {
		t.Fatalf("get profile failed: %v", err)
	}

	method, err := svc.CreatePayoutMethod(ctx, service.CreatePayoutMethodInput{
		AffiliateID:   profile.ID,
		MethodType:    domain.PayoutMethodTypeBankTransfer,
		DisplayName:   "Bank transfer",
		DetailsMasked: "****1234",
		IsDefault:     true,
		IsVerified:    true,
	})
	if err != nil {
		t.Fatalf("create payout method failed: %v", err)
	}

	if err := execTestSQL(t,
		`UPDATE affiliate_profiles SET hold_period_days = 0 WHERE id = ?`, profile.ID); err != nil {
		t.Fatalf("set hold period failed: %v", err)
	}
	now := time.Now().UTC()
	if _, err := svc.CalculateCommission(ctx, service.CalculateCommissionInput{
		AffiliateID:    profile.ID,
		ReferredUserID: 7300,
		SourceType:     "casino",
		SourceID:       uuid.NewString(),
		PeriodStart:    now.Add(-24 * time.Hour),
		PeriodEnd:      now,
		GGRAmount:      decimal.RequireFromString("1000"),
		NGRAmount:      decimal.RequireFromString("1000"),
		IdempotencyKey: uuid.NewString(),
	}); err != nil {
		t.Fatalf("accrue commission failed: %v", err)
	}
	if _, err := svc.ReleaseHeldCommissions(ctx, now.Add(time.Minute)); err != nil {
		t.Fatalf("release held commissions failed: %v", err)
	}

	key := uuid.NewString()
	input := service.RequestPayoutInput{
		AffiliateID:    profile.ID,
		MethodID:       method.ID,
		Amount:         decimal.RequireFromString("100"),
		IdempotencyKey: key,
	}
	first, err := svc.RequestPayout(ctx, input)
	if err != nil {
		t.Fatalf("first payout request failed: %v", err)
	}
	second, err := svc.RequestPayout(ctx, input)
	if err != nil {
		t.Fatalf("retried payout request failed: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("retried request must return the same payout: %s vs %s", first.ID, second.ID)
	}

	payouts, _, err := svc.ListPayoutsByAffiliate(ctx, profile.ID, "", 50, 0)
	if err != nil {
		t.Fatalf("list payouts failed: %v", err)
	}
	if len(payouts) != 1 {
		t.Fatalf("expected exactly 1 payout row, got %d", len(payouts))
	}
}
