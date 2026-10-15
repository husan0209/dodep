package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/opus-casino/affiliate/internal/domain"
)

func testAccrualProfile(affiliateID uuid.UUID) *domain.AffiliateProfile {
	return &domain.AffiliateProfile{
		ID:              affiliateID,
		UserID:          42,
		Status:          domain.AffiliateStatusActive,
		CommissionRate:  decimal.RequireFromString("0.25"),
		HoldPeriodDays:  14,
		MinPayoutAmount: decimal.RequireFromString("100"),
		Currency:        "USD",
	}
}

func verifiedMethod(affiliateID uuid.UUID) *domain.AffiliatePayoutMethod {
	return &domain.AffiliatePayoutMethod{
		ID:          uuid.New(),
		AffiliateID: affiliateID,
		MethodType:  domain.PayoutMethodTypeCrypto,
		IsVerified:  true,
	}
}

func TestComputeNGR_Formula(t *testing.T) {
	tests := []struct {
		name       string
		ggr        string
		deductions NGRComponents
		expected   string
	}{
		{
			name: "casino ngr",
			ggr:  "1000",
			deductions: NGRComponents{
				Bonuses:        decimal.RequireFromString("100"),
				PaymentFees:    decimal.RequireFromString("20"),
				Chargebacks:    decimal.RequireFromString("30"),
				FraudWriteoffs: decimal.RequireFromString("10"),
				Taxes:          decimal.RequireFromString("40"),
			},
			expected: "800",
		},
		{
			name:       "no deductions equals ggr",
			ggr:        "500",
			deductions: NGRComponents{},
			expected:   "500",
		},
		{
			name:       "negative ngr stays auditable (clamp happens at commission)",
			ggr:        "100",
			deductions: NGRComponents{Bonuses: decimal.RequireFromString("150")},
			expected:   "-50",
		},
		{
			name:       "manual adjustments deducted",
			ggr:        "300",
			deductions: NGRComponents{ManualAdjustments: decimal.RequireFromString("25")},
			expected:   "275",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ComputeNGR(decimal.RequireFromString(tt.ggr), tt.deductions)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(decimal.RequireFromString(tt.expected)) {
				t.Fatalf("expected NGR %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestComputeNGR_RejectsNegativeComponent(t *testing.T) {
	_, err := ComputeNGR(decimal.RequireFromString("100"), NGRComponents{
		Bonuses: decimal.RequireFromString("-5"),
	})
	if err == nil {
		t.Fatal("expected negative deduction component to be rejected")
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
}

func TestCalculateCommission_NegativeNGR_PersistsZeroRow(t *testing.T) {
	ctx := context.Background()
	affiliateID := uuid.New()
	var created *domain.AffiliateEarning
	repo := &mockAffiliateRepository{
		getProfileByIDFunc: func(context.Context, uuid.UUID) (*domain.AffiliateProfile, error) {
			return testAccrualProfile(affiliateID), nil
		},
		createEarningFunc: func(_ context.Context, e *domain.AffiliateEarning) error {
			copied := *e
			created = &copied
			return nil
		},
	}

	earning, err := newTestAffiliateService(repo).CalculateCommission(ctx, CalculateCommissionInput{
		AffiliateID:    affiliateID,
		ReferredUserID: 77,
		SourceType:     "casino_session",
		SourceID:       "session-neg",
		GGRAmount:      decimal.RequireFromString("100"),
		Deductions:     NGRComponents{Bonuses: decimal.RequireFromString("150")},
		IdempotencyKey: "neg-1",
	})
	if err != nil {
		t.Fatalf("negative NGR must not error (max(0,...)), got %v", err)
	}
	if created == nil {
		t.Fatal("expected zero-amount row to be persisted for audit")
	}
	if !created.CommissionAmount.IsZero() {
		t.Fatalf("expected zero commission, got %s", created.CommissionAmount)
	}
	if !created.NGRAmount.Equal(decimal.RequireFromString("-50")) {
		t.Fatalf("expected auditable NGR -50, got %s", created.NGRAmount)
	}
	if earning.Status != domain.EarningStatusAccrued {
		t.Fatalf("expected accrued status, got %s", earning.Status)
	}
}

func TestCalculateCommission_RequiresIdempotencyKey(t *testing.T) {
	_, err := newTestAffiliateService(&mockAffiliateRepository{}).CalculateCommission(
		context.Background(),
		CalculateCommissionInput{AffiliateID: uuid.New(), GGRAmount: decimal.RequireFromString("10")},
	)
	if !errors.Is(err, ErrIdempotencyKeyRequired) {
		t.Fatalf("expected ErrIdempotencyKeyRequired, got %v", err)
	}
}

func TestCalculateCommission_IdempotentReplayReturnsExisting(t *testing.T) {
	ctx := context.Background()
	affiliateID := uuid.New()
	existing := &domain.AffiliateEarning{ID: uuid.New(), AffiliateID: affiliateID}
	creates := 0
	repo := &mockAffiliateRepository{
		getProfileByIDFunc: func(context.Context, uuid.UUID) (*domain.AffiliateProfile, error) {
			return testAccrualProfile(affiliateID), nil
		},
		getEarningByIdempotencyKeyFunc: func(context.Context, string) (*domain.AffiliateEarning, error) {
			return existing, nil
		},
		createEarningFunc: func(context.Context, *domain.AffiliateEarning) error {
			creates++
			return nil
		},
	}

	got, err := newTestAffiliateService(repo).CalculateCommission(ctx, CalculateCommissionInput{
		AffiliateID:    affiliateID,
		GGRAmount:      decimal.RequireFromString("100"),
		IdempotencyKey: "replay-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != existing.ID {
		t.Fatal("expected the original earning on replay")
	}
	if creates != 0 {
		t.Fatalf("expected no duplicate insert, got %d creates", creates)
	}
}

func TestAccrueDaily_BatchCreatesOneEarningPerItem(t *testing.T) {
	ctx := context.Background()
	affiliateID := uuid.New()
	creates := 0
	repo := &mockAffiliateRepository{
		getProfileByIDFunc: func(context.Context, uuid.UUID) (*domain.AffiliateProfile, error) {
			return testAccrualProfile(affiliateID), nil
		},
		createEarningFunc: func(context.Context, *domain.AffiliateEarning) error {
			creates++
			return nil
		},
	}

	day := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	results, err := newTestAffiliateService(repo).AccrueDaily(ctx, []DailyAccrualItem{
		{
			AffiliateID: affiliateID, ReferredUserID: 1,
			SourceType: "casino_session", SourceID: "s-1",
			PeriodStart: day, PeriodEnd: day,
			GGRAmount:      decimal.RequireFromString("200"),
			Deductions:     NGRComponents{Bonuses: decimal.RequireFromString("50")},
			IdempotencyKey: "day-1",
		},
		{
			AffiliateID: affiliateID, ReferredUserID: 2,
			SourceType: "sports_settled", SourceID: "s-2",
			PeriodStart: day, PeriodEnd: day,
			GGRAmount:      decimal.RequireFromString("100"),
			Deductions:     NGRComponents{},
			IdempotencyKey: "day-2",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 || creates != 2 {
		t.Fatalf("expected 2 earnings, got %d results / %d creates", len(results), creates)
	}
	// NGR = 200-50 = 150 at 25% -> 37.5
	if !results[0].CommissionAmount.Equal(decimal.RequireFromString("37.5")) {
		t.Fatalf("expected 37.5, got %s", results[0].CommissionAmount)
	}
}

func TestCalculateCommission_HoldUntilRespectsProfilePeriod(t *testing.T) {
	ctx := context.Background()
	affiliateID := uuid.New()
	var created *domain.AffiliateEarning
	repo := &mockAffiliateRepository{
		getProfileByIDFunc: func(context.Context, uuid.UUID) (*domain.AffiliateProfile, error) {
			p := testAccrualProfile(affiliateID)
			p.HoldPeriodDays = 30
			return p, nil
		},
		createEarningFunc: func(_ context.Context, e *domain.AffiliateEarning) error {
			copied := *e
			created = &copied
			return nil
		},
	}

	before := time.Now().UTC()
	_, err := newTestAffiliateService(repo).CalculateCommission(ctx, CalculateCommissionInput{
		AffiliateID:    affiliateID,
		GGRAmount:      decimal.RequireFromString("100"),
		IdempotencyKey: "hold-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lower := before.Add(30*24*time.Hour - time.Minute)
	upper := before.Add(30*24*time.Hour + time.Minute)
	if created.HoldUntil.Before(lower) || created.HoldUntil.After(upper) {
		t.Fatalf("expected 30-day hold, got %s", created.HoldUntil)
	}
}

func TestReverseEarning_Flow(t *testing.T) {
	ctx := context.Background()
	earningID := uuid.New()
	affiliateID := uuid.New()

	newRepo := func(status domain.EarningStatus) *mockAffiliateRepository {
		return &mockAffiliateRepository{
			getEarningByIDFunc: func(context.Context, uuid.UUID) (*domain.AffiliateEarning, error) {
				return &domain.AffiliateEarning{
					ID:               earningID,
					AffiliateID:      affiliateID,
					Status:           status,
					CommissionAmount: decimal.RequireFromString("30"),
				}, nil
			},
			reverseEarningFunc: func(_ context.Context, id uuid.UUID, reason string) (*domain.AffiliateEarning, error) {
				if reason == "" {
					t.Fatal("reason must reach the repository for audit")
				}
				return &domain.AffiliateEarning{ID: id, AffiliateID: affiliateID, Status: domain.EarningStatusReversed}, nil
			},
		}
	}

	reversed, err := newTestAffiliateService(newRepo(domain.EarningStatusAvailable)).ReverseEarning(ctx, ReverseEarningInput{
		EarningID: earningID,
		Reason:    "chargeback on referred traffic",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reversed.Status != domain.EarningStatusReversed {
		t.Fatalf("expected reversed, got %s", reversed.Status)
	}

	if _, err := newTestAffiliateService(newRepo(domain.EarningStatusPaid)).ReverseEarning(ctx, ReverseEarningInput{
		EarningID: earningID, Reason: "too late",
	}); !errors.Is(err, ErrEarningNotReversible) {
		t.Fatalf("paid earnings must not reverse, got %v", err)
	}

	if _, err := newTestAffiliateService(newRepo(domain.EarningStatusReversed)).ReverseEarning(ctx, ReverseEarningInput{
		EarningID: earningID, Reason: "double",
	}); !errors.Is(err, ErrEarningNotReversible) {
		t.Fatalf("double reversal must be rejected, got %v", err)
	}

	if _, err := newTestAffiliateService(newRepo(domain.EarningStatusAccrued)).ReverseEarning(ctx, ReverseEarningInput{
		EarningID: earningID, Reason: "   ",
	}); err == nil {
		t.Fatal("empty reversal reason must be rejected for audit")
	}

	missing := &mockAffiliateRepository{}
	if _, err := newTestAffiliateService(missing).ReverseEarning(ctx, ReverseEarningInput{
		EarningID: earningID, Reason: "x",
	}); !errors.Is(err, ErrEarningNotFound) {
		t.Fatalf("expected ErrEarningNotFound, got %v", err)
	}
}

func TestRequestPayout_Gates(t *testing.T) {
	ctx := context.Background()

	t.Run("missing idempotency key", func(t *testing.T) {
		_, err := newTestAffiliateService(&mockAffiliateRepository{}).RequestPayout(
			ctx, RequestPayoutInput{AffiliateID: uuid.New(), Amount: decimal.RequireFromString("120")},
		)
		if !errors.Is(err, ErrIdempotencyKeyRequired) {
			t.Fatalf("expected ErrIdempotencyKeyRequired, got %v", err)
		}
	})

	t.Run("unverified method blocked", func(t *testing.T) {
		affiliateID := uuid.New()
		repo := &mockAffiliateRepository{
			getProfileByIDFunc: func(context.Context, uuid.UUID) (*domain.AffiliateProfile, error) {
				p := testAccrualProfile(affiliateID)
				p.KYCRequired = false
				return p, nil
			},
			getPayoutMethodByIDFunc: func(context.Context, uuid.UUID) (*domain.AffiliatePayoutMethod, error) {
				return &domain.AffiliatePayoutMethod{AffiliateID: affiliateID, IsVerified: false}, nil
			},
		}
		_, err := newTestAffiliateService(repo).RequestPayout(ctx, RequestPayoutInput{
			AffiliateID: affiliateID, MethodID: uuid.New(),
			Amount: decimal.RequireFromString("120"), IdempotencyKey: "k1",
		})
		if !errors.Is(err, ErrPayoutMethodUnverified) {
			t.Fatalf("expected ErrPayoutMethodUnverified, got %v", err)
		}
	})

	t.Run("foreign method blocked", func(t *testing.T) {
		affiliateID := uuid.New()
		repo := &mockAffiliateRepository{
			getProfileByIDFunc: func(context.Context, uuid.UUID) (*domain.AffiliateProfile, error) {
				p := testAccrualProfile(affiliateID)
				p.KYCRequired = false
				return p, nil
			},
			getPayoutMethodByIDFunc: func(context.Context, uuid.UUID) (*domain.AffiliatePayoutMethod, error) {
				return &domain.AffiliatePayoutMethod{AffiliateID: uuid.New(), IsVerified: true}, nil
			},
		}
		_, err := newTestAffiliateService(repo).RequestPayout(ctx, RequestPayoutInput{
			AffiliateID: affiliateID, MethodID: uuid.New(),
			Amount: decimal.RequireFromString("120"), IdempotencyKey: "k2",
		})
		if !errors.Is(err, ErrPayoutMethodUnverified) {
			t.Fatalf("expected ErrPayoutMethodUnverified, got %v", err)
		}
	})

	payoutBase := func(affiliateID uuid.UUID, mutate func(*domain.AffiliateProfile)) *mockAffiliateRepository {
		return &mockAffiliateRepository{
			getProfileByIDFunc: func(context.Context, uuid.UUID) (*domain.AffiliateProfile, error) {
				p := testAccrualProfile(affiliateID)
				if mutate != nil {
					mutate(p)
				}
				return p, nil
			},
			getPayoutMethodByIDFunc: func(_ context.Context, _ uuid.UUID) (*domain.AffiliatePayoutMethod, error) {
				return verifiedMethod(affiliateID), nil
			},
			getAvailableBalanceFunc: func(context.Context, uuid.UUID) (decimal.Decimal, error) {
				return decimal.RequireFromString("150"), nil
			},
			createPayoutFunc: func(context.Context, *domain.AffiliatePayout) error { return nil },
		}
	}

	t.Run("kyc gate", func(t *testing.T) {
		affiliateID := uuid.New()
		_, err := newTestAffiliateService(payoutBase(affiliateID, func(p *domain.AffiliateProfile) {
			p.KYCRequired = true
		})).RequestPayout(ctx, RequestPayoutInput{
			AffiliateID: affiliateID, MethodID: uuid.New(),
			Amount: decimal.RequireFromString("120"), IdempotencyKey: "k3",
			KYCApproved: false,
		})
		if !errors.Is(err, domain.ErrAffiliateKYCRequired) {
			t.Fatalf("expected ErrAffiliateKYCRequired, got %v", err)
		}
	})

	t.Run("open fraud blocks", func(t *testing.T) {
		affiliateID := uuid.New()
		_, err := newTestAffiliateService(payoutBase(affiliateID, func(p *domain.AffiliateProfile) {
			p.KYCRequired = false
		})).RequestPayout(ctx, RequestPayoutInput{
			AffiliateID: affiliateID, MethodID: uuid.New(),
			Amount: decimal.RequireFromString("120"), IdempotencyKey: "k4",
			HasOpenFraud: true,
		})
		if !errors.Is(err, domain.ErrAffiliateFraudBlocked) {
			t.Fatalf("expected ErrAffiliateFraudBlocked, got %v", err)
		}
	})

	t.Run("below min payout", func(t *testing.T) {
		affiliateID := uuid.New()
		repo := payoutBase(affiliateID, func(p *domain.AffiliateProfile) {
			p.KYCRequired = false
			p.MinPayoutAmount = decimal.RequireFromString("100")
		})
		repo.getAvailableBalanceFunc = func(context.Context, uuid.UUID) (decimal.Decimal, error) {
			return decimal.RequireFromString("99.99"), nil
		}
		_, err := newTestAffiliateService(repo).RequestPayout(ctx, RequestPayoutInput{
			AffiliateID: affiliateID, MethodID: uuid.New(),
			Amount: decimal.RequireFromString("99.99"), IdempotencyKey: "k5",
		})
		if !errors.Is(err, domain.ErrMinPayoutNotReached) {
			t.Fatalf("expected ErrMinPayoutNotReached, got %v", err)
		}
	})

	t.Run("idempotent replay returns original", func(t *testing.T) {
		affiliateID := uuid.New()
		original := &domain.AffiliatePayout{ID: uuid.New(), Status: domain.PayoutStatusRequested}
		repo := payoutBase(affiliateID, nil)
		repo.getPayoutByIdempotencyKeyFunc = func(context.Context, string) (*domain.AffiliatePayout, error) {
			return original, nil
		}
		creates := 0
		repo.createPayoutFunc = func(context.Context, *domain.AffiliatePayout) error {
			creates++
			return nil
		}
		got, err := newTestAffiliateService(repo).RequestPayout(ctx, RequestPayoutInput{
			AffiliateID: affiliateID, MethodID: uuid.New(),
			Amount: decimal.RequireFromString("120"), IdempotencyKey: "k6",
			KYCApproved: true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != original.ID || creates != 0 {
			t.Fatal("replay must return the original payout without insert")
		}
	})
}

func TestApproveAffiliatePayout_Gates(t *testing.T) {
	ctx := context.Background()
	affiliateID := uuid.New()
	payoutID := uuid.New()

	approveBase := func(profile *domain.AffiliateProfile, flags []domain.AffiliateFraudFlag) *mockAffiliateRepository {
		return &mockAffiliateRepository{
			getProfileByIDFunc: func(context.Context, uuid.UUID) (*domain.AffiliateProfile, error) {
				return profile, nil
			},
			getPayoutByIDFunc: func(context.Context, uuid.UUID) (*domain.AffiliatePayout, error) {
				return &domain.AffiliatePayout{
					ID: payoutID, AffiliateID: affiliateID,
					Amount: decimal.RequireFromString("120"), Currency: "USD",
					Status: domain.PayoutStatusRequested,
				}, nil
			},
			getOpenFraudFlagsFunc: func(context.Context, uuid.UUID) ([]domain.AffiliateFraudFlag, error) {
				return flags, nil
			},
			updatePayoutFunc: func(context.Context, *domain.AffiliatePayout) error { return nil },
		}
	}

	activeKYC := func() *domain.AffiliateProfile {
		p := testAccrualProfile(affiliateID)
		p.KYCRequired = true
		return p
	}

	if _, err := newTestAffiliateService(approveBase(activeKYC(), nil)).ApproveAffiliatePayout(ctx, ApproveAffiliatePayoutInput{
		AffiliateID: affiliateID, PayoutID: payoutID, ApprovedBy: "fin",
		KYCApproved: false,
	}); !errors.Is(err, domain.ErrAffiliateKYCRequired) {
		t.Fatalf("approval without KYC must fail, got %v", err)
	}

	if _, err := newTestAffiliateService(approveBase(activeKYC(), []domain.AffiliateFraudFlag{
		{ID: uuid.New(), FlagType: "device_sharing", Severity: domain.FraudSeverityHigh},
	})).ApproveAffiliatePayout(ctx, ApproveAffiliatePayoutInput{
		AffiliateID: affiliateID, PayoutID: payoutID, ApprovedBy: "fin",
		KYCApproved: true,
	}); !errors.Is(err, domain.ErrAffiliateFraudBlocked) {
		t.Fatalf("approval with open fraud must fail, got %v", err)
	}
}

func TestMarkPayoutPaid_Flow(t *testing.T) {
	ctx := context.Background()
	affiliateID := uuid.New()
	payoutID := uuid.New()

	paidBase := func(status domain.PayoutStatus) *mockAffiliateRepository {
		return &mockAffiliateRepository{
			getProfileByIDFunc: func(context.Context, uuid.UUID) (*domain.AffiliateProfile, error) {
				return testAccrualProfile(affiliateID), nil
			},
			getPayoutByIDFunc: func(context.Context, uuid.UUID) (*domain.AffiliatePayout, error) {
				return &domain.AffiliatePayout{
					ID: payoutID, AffiliateID: affiliateID,
					Amount: decimal.RequireFromString("120"), Currency: "USD",
					Status: status,
				}, nil
			},
			updatePayoutFunc: func(context.Context, *domain.AffiliatePayout) error { return nil },
		}
	}

	got, err := newTestAffiliateService(paidBase(domain.PayoutStatusApproved)).MarkPayoutPaid(ctx, MarkPayoutPaidInput{
		PayoutID: payoutID, ProviderReference: "psp-9",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != domain.PayoutStatusPaid || got.ProviderReference != "psp-9" {
		t.Fatalf("expected paid with provider ref, got %+v", got)
	}

	if _, err := newTestAffiliateService(paidBase(domain.PayoutStatusRequested)).MarkPayoutPaid(ctx, MarkPayoutPaidInput{
		PayoutID: payoutID,
	}); !errors.Is(err, domain.ErrInvalidPayoutStatus) {
		t.Fatalf("paying a non-approved payout must fail, got %v", err)
	}
}

func TestCreateAdjustment_Validation(t *testing.T) {
	ctx := context.Background()
	affiliateID := uuid.New()
	repo := &mockAffiliateRepository{
		getProfileByIDFunc: func(context.Context, uuid.UUID) (*domain.AffiliateProfile, error) {
			return testAccrualProfile(affiliateID), nil
		},
	}
	svc := newTestAffiliateService(repo)

	valid := CreateAdjustmentInput{
		AffiliateID: affiliateID, AdjustmentType: domain.AdjustmentTypeCredit,
		Amount: decimal.RequireFromString("10"), Reason: "goodwill",
		CreatedBy: "admin-1",
	}

	bad := []struct {
		name   string
		mutate func(*CreateAdjustmentInput)
	}{
		{"unknown type", func(i *CreateAdjustmentInput) { i.AdjustmentType = "bonus" }},
		{"zero amount", func(i *CreateAdjustmentInput) { i.Amount = decimal.Zero }},
		{"negative amount", func(i *CreateAdjustmentInput) { i.Amount = decimal.RequireFromString("-1") }},
		{"empty reason", func(i *CreateAdjustmentInput) { i.Reason = "  " }},
		{"empty author", func(i *CreateAdjustmentInput) { i.CreatedBy = "" }},
	}
	for _, tt := range bad {
		t.Run(tt.name, func(t *testing.T) {
			in := valid
			tt.mutate(&in)
			if _, err := svc.CreateAdjustment(ctx, in); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
