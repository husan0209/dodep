package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opus-casino/affiliate/internal/domain"
	"github.com/opus-casino/affiliate/internal/repository"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type mockAffiliateRepository struct {
	getEnrollmentByUserIDFunc      func(context.Context, int64) (*domain.AffiliateEnrollmentRequest, error)
	createEnrollmentFunc           func(context.Context, *domain.AffiliateEnrollmentRequest) error
	updateEnrollmentStatusFunc     func(context.Context, int64, domain.EnrollmentStatus, string, string, time.Time) error
	getProfileByUserIDFunc         func(context.Context, int64) (*domain.AffiliateProfile, error)
	getProfileByIDFunc             func(context.Context, uuid.UUID) (*domain.AffiliateProfile, error)
	createProfileFunc              func(context.Context, *domain.AffiliateProfile) error
	updateProfileStatusFunc        func(context.Context, uuid.UUID, domain.AffiliateStatus, domain.AffiliateStatus) error
	getCommissionPlanByIDFunc      func(context.Context, uuid.UUID) (*domain.AffiliateCommissionPlan, error)
	getDefaultCommissionPlanFunc   func(context.Context) (*domain.AffiliateCommissionPlan, error)
	createLinkFunc                 func(context.Context, *domain.AffiliateLink) error
	listLinksByAffiliateIDFunc     func(context.Context, uuid.UUID) ([]domain.AffiliateLink, error)
	getProfileByAffiliateCodeFunc  func(context.Context, string) (*domain.AffiliateProfile, error)
	createClickFunc                func(context.Context, *domain.AffiliateClick) error
	getAttributionByUserIDFunc     func(context.Context, int64) (*domain.AffiliateAttribution, error)
	createAttributionFunc          func(context.Context, *domain.AffiliateAttribution) error
	getDashboardFunc               func(context.Context, uuid.UUID) (*domain.AffiliateDashboard, error)
	listEarningsFunc               func(context.Context, uuid.UUID, domain.EarningStatus, int) ([]domain.AffiliateEarning, error)
	getAvailableBalanceFunc        func(context.Context, uuid.UUID) (decimal.Decimal, error)
	createPayoutMethodFunc         func(context.Context, *domain.AffiliatePayoutMethod) error
	listPayoutMethodsFunc          func(context.Context, uuid.UUID) ([]domain.AffiliatePayoutMethod, error)
	getPayoutMethodByIDFunc        func(context.Context, uuid.UUID) (*domain.AffiliatePayoutMethod, error)
	updatePayoutMethodFunc         func(context.Context, *domain.AffiliatePayoutMethod) error
	createPayoutFunc               func(context.Context, *domain.AffiliatePayout) error
	getPayoutByIDFunc              func(context.Context, uuid.UUID) (*domain.AffiliatePayout, error)
	updatePayoutFunc               func(context.Context, *domain.AffiliatePayout) error
	createEarningFunc              func(context.Context, *domain.AffiliateEarning) error
	createFraudFlagFunc            func(context.Context, *domain.AffiliateFraudFlag) error
	getOpenFraudFlagsFunc          func(context.Context, uuid.UUID) ([]domain.AffiliateFraudFlag, error)
	releaseEligibleEarningsFunc    func(context.Context, time.Time) (int64, error)
}

func (m *mockAffiliateRepository) GetEnrollmentByUserID(ctx context.Context, userID int64) (*domain.AffiliateEnrollmentRequest, error) {
	if m.getEnrollmentByUserIDFunc != nil {
		return m.getEnrollmentByUserIDFunc(ctx, userID)
	}
	return nil, nil
}

func (m *mockAffiliateRepository) CreateEnrollment(ctx context.Context, req *domain.AffiliateEnrollmentRequest) error {
	if m.createEnrollmentFunc != nil {
		return m.createEnrollmentFunc(ctx, req)
	}
	return nil
}

func (m *mockAffiliateRepository) UpdateEnrollmentStatus(ctx context.Context, userID int64, status domain.EnrollmentStatus, reviewNotes string, reviewedBy string, reviewedAt time.Time) error {
	if m.updateEnrollmentStatusFunc != nil {
		return m.updateEnrollmentStatusFunc(ctx, userID, status, reviewNotes, reviewedBy, reviewedAt)
	}
	return nil
}

func (m *mockAffiliateRepository) GetProfileByUserID(ctx context.Context, userID int64) (*domain.AffiliateProfile, error) {
	if m.getProfileByUserIDFunc != nil {
		return m.getProfileByUserIDFunc(ctx, userID)
	}
	return nil, nil
}

func (m *mockAffiliateRepository) GetProfileByID(ctx context.Context, affiliateID uuid.UUID) (*domain.AffiliateProfile, error) {
	if m.getProfileByIDFunc != nil {
		return m.getProfileByIDFunc(ctx, affiliateID)
	}
	return nil, nil
}

func (m *mockAffiliateRepository) CreateProfile(ctx context.Context, profile *domain.AffiliateProfile) error {
	if m.createProfileFunc != nil {
		return m.createProfileFunc(ctx, profile)
	}
	return nil
}

func (m *mockAffiliateRepository) UpdateProfileStatus(ctx context.Context, affiliateID uuid.UUID, from domain.AffiliateStatus, to domain.AffiliateStatus) error {
	if m.updateProfileStatusFunc != nil {
		return m.updateProfileStatusFunc(ctx, affiliateID, from, to)
	}
	return nil
}

func (m *mockAffiliateRepository) GetCommissionPlanByID(ctx context.Context, planID uuid.UUID) (*domain.AffiliateCommissionPlan, error) {
	if m.getCommissionPlanByIDFunc != nil {
		return m.getCommissionPlanByIDFunc(ctx, planID)
	}
	return nil, nil
}

func (m *mockAffiliateRepository) GetDefaultCommissionPlan(ctx context.Context) (*domain.AffiliateCommissionPlan, error) {
	if m.getDefaultCommissionPlanFunc != nil {
		return m.getDefaultCommissionPlanFunc(ctx)
	}
	return nil, nil
}

func (m *mockAffiliateRepository) CreateLink(ctx context.Context, link *domain.AffiliateLink) error {
	if m.createLinkFunc != nil {
		return m.createLinkFunc(ctx, link)
	}
	return nil
}

func (m *mockAffiliateRepository) ListLinksByAffiliateID(ctx context.Context, affiliateID uuid.UUID) ([]domain.AffiliateLink, error) {
	if m.listLinksByAffiliateIDFunc != nil {
		return m.listLinksByAffiliateIDFunc(ctx, affiliateID)
	}
	return nil, nil
}

func (m *mockAffiliateRepository) GetProfileByAffiliateCode(ctx context.Context, affiliateCode string) (*domain.AffiliateProfile, error) {
	if m.getProfileByAffiliateCodeFunc != nil {
		return m.getProfileByAffiliateCodeFunc(ctx, affiliateCode)
	}
	return nil, nil
}

func (m *mockAffiliateRepository) CreateClick(ctx context.Context, click *domain.AffiliateClick) error {
	if m.createClickFunc != nil {
		return m.createClickFunc(ctx, click)
	}
	return nil
}

func (m *mockAffiliateRepository) GetAttributionByReferredUserID(ctx context.Context, referredUserID int64) (*domain.AffiliateAttribution, error) {
	if m.getAttributionByUserIDFunc != nil {
		return m.getAttributionByUserIDFunc(ctx, referredUserID)
	}
	return nil, nil
}

func (m *mockAffiliateRepository) CreateAttribution(ctx context.Context, attribution *domain.AffiliateAttribution) error {
	if m.createAttributionFunc != nil {
		return m.createAttributionFunc(ctx, attribution)
	}
	return nil
}

func (m *mockAffiliateRepository) GetDashboard(ctx context.Context, affiliateID uuid.UUID) (*domain.AffiliateDashboard, error) {
	if m.getDashboardFunc != nil {
		return m.getDashboardFunc(ctx, affiliateID)
	}
	return nil, nil
}

func (m *mockAffiliateRepository) ListEarnings(ctx context.Context, affiliateID uuid.UUID, status domain.EarningStatus, limit int) ([]domain.AffiliateEarning, error) {
	if m.listEarningsFunc != nil {
		return m.listEarningsFunc(ctx, affiliateID, status, limit)
	}
	return nil, nil
}

func (m *mockAffiliateRepository) GetAvailableBalance(ctx context.Context, affiliateID uuid.UUID) (decimal.Decimal, error) {
	if m.getAvailableBalanceFunc != nil {
		return m.getAvailableBalanceFunc(ctx, affiliateID)
	}
	return decimal.Zero, nil
}

func (m *mockAffiliateRepository) CreatePayoutMethod(ctx context.Context, method *domain.AffiliatePayoutMethod) error {
	if m.createPayoutMethodFunc != nil {
		return m.createPayoutMethodFunc(ctx, method)
	}
	return nil
}

func (m *mockAffiliateRepository) ListPayoutMethods(ctx context.Context, affiliateID uuid.UUID) ([]domain.AffiliatePayoutMethod, error) {
	if m.listPayoutMethodsFunc != nil {
		return m.listPayoutMethodsFunc(ctx, affiliateID)
	}
	return nil, nil
}

func (m *mockAffiliateRepository) GetPayoutMethodByID(ctx context.Context, methodID uuid.UUID) (*domain.AffiliatePayoutMethod, error) {
	if m.getPayoutMethodByIDFunc != nil {
		return m.getPayoutMethodByIDFunc(ctx, methodID)
	}
	return nil, nil
}

func (m *mockAffiliateRepository) UpdatePayoutMethod(ctx context.Context, method *domain.AffiliatePayoutMethod) error {
	if m.updatePayoutMethodFunc != nil {
		return m.updatePayoutMethodFunc(ctx, method)
	}
	return nil
}

func (m *mockAffiliateRepository) CreatePayout(ctx context.Context, payout *domain.AffiliatePayout) error {
	if m.createPayoutFunc != nil {
		return m.createPayoutFunc(ctx, payout)
	}
	return nil
}

func (m *mockAffiliateRepository) GetPayoutByID(ctx context.Context, payoutID uuid.UUID) (*domain.AffiliatePayout, error) {
	if m.getPayoutByIDFunc != nil {
		return m.getPayoutByIDFunc(ctx, payoutID)
	}
	return nil, nil
}

func (m *mockAffiliateRepository) UpdatePayout(ctx context.Context, payout *domain.AffiliatePayout) error {
	if m.updatePayoutFunc != nil {
		return m.updatePayoutFunc(ctx, payout)
	}
	return nil
}

func (m *mockAffiliateRepository) CreateEarning(ctx context.Context, earning *domain.AffiliateEarning) error {
	if m.createEarningFunc != nil {
		return m.createEarningFunc(ctx, earning)
	}
	return nil
}

func (m *mockAffiliateRepository) CreateFraudFlag(ctx context.Context, flag *domain.AffiliateFraudFlag) error {
	if m.createFraudFlagFunc != nil {
		return m.createFraudFlagFunc(ctx, flag)
	}
	return nil
}

func (m *mockAffiliateRepository) GetOpenFraudFlags(ctx context.Context, affiliateID uuid.UUID) ([]domain.AffiliateFraudFlag, error) {
	if m.getOpenFraudFlagsFunc != nil {
		return m.getOpenFraudFlagsFunc(ctx, affiliateID)
	}
	return nil, nil
}

func (m *mockAffiliateRepository) ListProfiles(_ context.Context, _ domain.AffiliateStatus, _, _ int) ([]domain.AffiliateProfile, int64, error) {
	return nil, 0, nil
}

func (m *mockAffiliateRepository) UpdateCommissionRate(_ context.Context, _ uuid.UUID, _ decimal.Decimal) error {
	return nil
}

func (m *mockAffiliateRepository) CreateAdjustment(_ context.Context, _ *domain.AffiliateAdjustment) error {
	return nil
}

func (m *mockAffiliateRepository) ListAllPayouts(_ context.Context, _ domain.PayoutStatus, _, _ int) ([]domain.AffiliatePayout, int64, error) {
	return nil, 0, nil
}

func (m *mockAffiliateRepository) ListPayoutsByAffiliateID(_ context.Context, _ uuid.UUID, _ domain.PayoutStatus, _, _ int) ([]domain.AffiliatePayout, int64, error) {
	return nil, 0, nil
}

func (m *mockAffiliateRepository) ListAllFraudFlags(_ context.Context, _ domain.FraudFlagStatus, _, _ int) ([]domain.AffiliateFraudFlag, int64, error) {
	return nil, 0, nil
}

func (m *mockAffiliateRepository) ReleaseEligibleEarnings(ctx context.Context, now time.Time) (int64, error) {
	if m.releaseEligibleEarningsFunc != nil {
		return m.releaseEligibleEarningsFunc(ctx, now)
	}
	return 0, nil
}

func (m *mockAffiliateRepository) ListPendingOutboxEvents(_ context.Context, _ int) ([]domain.OutboxEvent, error) {
	return nil, nil
}

func (m *mockAffiliateRepository) MarkOutboxEventPublished(_ context.Context, _ int64) error {
	return nil
}

func (m *mockAffiliateRepository) IncrementOutboxEventRetry(_ context.Context, _ int64) error {
	return nil
}

func newTestAffiliateService(repo repository.AffiliateRepository) *AffiliateService {
	return NewAffiliateService(repo, zap.NewNop())
}

func TestAffiliateService_EnrollAffiliate_CreatesPendingReviewRequest(t *testing.T) {
	ctx := context.Background()
	repo := &mockAffiliateRepository{
		createEnrollmentFunc: func(ctx context.Context, req *domain.AffiliateEnrollmentRequest) error {
			req.ID = uuid.New()
			return nil
		},
	}

	svc := newTestAffiliateService(repo)

	result, err := svc.EnrollAffiliate(ctx, EnrollAffiliateInput{
		UserID: 42,
		Reason: "content creator",
	})
	if err != nil {
		t.Fatalf("expected enroll to succeed, got %v", err)
	}
	if result.Status != domain.EnrollmentStatusPendingReview {
		t.Fatalf("expected pending_review, got %s", result.Status)
	}
}

func TestAffiliateService_EnrollAffiliate_RejectsDuplicatePendingRequest(t *testing.T) {
	ctx := context.Background()
	repo := &mockAffiliateRepository{
		getEnrollmentByUserIDFunc: func(ctx context.Context, userID int64) (*domain.AffiliateEnrollmentRequest, error) {
			return &domain.AffiliateEnrollmentRequest{
				ID:     uuid.New(),
				UserID: userID,
				Status: domain.EnrollmentStatusPendingReview,
			}, nil
		},
	}

	svc := newTestAffiliateService(repo)

	_, err := svc.EnrollAffiliate(ctx, EnrollAffiliateInput{UserID: 42, Reason: "duplicate"})
	if !errors.Is(err, domain.ErrEnrollmentAlreadyPending) {
		t.Fatalf("expected ErrEnrollmentAlreadyPending, got %v", err)
	}
}

func TestAffiliateService_ApproveAffiliate_CreatesProfileWithDefaultPlan(t *testing.T) {
	ctx := context.Background()
	requestID := uuid.New()
	repo := &mockAffiliateRepository{
		getEnrollmentByUserIDFunc: func(ctx context.Context, userID int64) (*domain.AffiliateEnrollmentRequest, error) {
			return &domain.AffiliateEnrollmentRequest{
				ID:     requestID,
				UserID: userID,
				Status: domain.EnrollmentStatusPendingReview,
			}, nil
		},
		createProfileFunc: func(ctx context.Context, profile *domain.AffiliateProfile) error {
			return nil
		},
		updateEnrollmentStatusFunc: func(ctx context.Context, userID int64, status domain.EnrollmentStatus, reviewNotes string, reviewedBy string, reviewedAt time.Time) error {
			return nil
		},
		getDefaultCommissionPlanFunc: func(ctx context.Context) (*domain.AffiliateCommissionPlan, error) {
			return &domain.AffiliateCommissionPlan{
				ID:             uuid.New(),
				Name:           "default",
				CommissionType: "revshare",
				CommissionRate: decimal.RequireFromString("0.20"),
				HoldPeriodDays: 14,
				MinPayoutAmount: decimal.RequireFromString("100"),
				ApprovalMode:   domain.ApprovalModeManual,
				PayoutSchedule: domain.PayoutScheduleMonthly,
			}, nil
		},
	}

	svc := newTestAffiliateService(repo)

	profile, err := svc.ApproveAffiliate(ctx, ApproveAffiliateInput{
		UserID:           42,
		ApprovedBy:       "admin-1",
		CommissionRate:   decimal.RequireFromString("0.20"),
		HoldPeriodDays:   14,
		MinPayoutAmount:  decimal.RequireFromString("100"),
		Currency:         "USD",
		RequireKYC:       true,
		CommissionPlanID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("expected approve to succeed, got %v", err)
	}
	if profile.Status != domain.AffiliateStatusActive {
		t.Fatalf("expected active profile, got %s", profile.Status)
	}
	if profile.AffiliateCode == "" {
		t.Fatal("expected generated affiliate code")
	}
}

func TestAffiliateService_ApproveAffiliate_RejectsDuplicateProfileForUser(t *testing.T) {
	ctx := context.Background()
	repo := &mockAffiliateRepository{
		getEnrollmentByUserIDFunc: func(ctx context.Context, userID int64) (*domain.AffiliateEnrollmentRequest, error) {
			return &domain.AffiliateEnrollmentRequest{
				ID:     uuid.New(),
				UserID: userID,
				Status: domain.EnrollmentStatusPendingReview,
			}, nil
		},
		getProfileByUserIDFunc: func(ctx context.Context, userID int64) (*domain.AffiliateProfile, error) {
			return &domain.AffiliateProfile{
				ID:     uuid.New(),
				UserID: userID,
				Status: domain.AffiliateStatusActive,
			}, nil
		},
	}

	svc := newTestAffiliateService(repo)

	_, err := svc.ApproveAffiliate(ctx, ApproveAffiliateInput{
		UserID:           42,
		ApprovedBy:       "admin-1",
		CommissionRate:   decimal.RequireFromString("0.20"),
		HoldPeriodDays:   14,
		MinPayoutAmount:  decimal.RequireFromString("100"),
		Currency:         "USD",
		RequireKYC:       true,
		CommissionPlanID: uuid.New(),
	})
	if !errors.Is(err, domain.ErrAffiliateAlreadyExists) {
		t.Fatalf("expected ErrAffiliateAlreadyExists, got %v", err)
	}
}

func TestAffiliateService_CreateAffiliateLink_CreatesReferralURL(t *testing.T) {
	ctx := context.Background()
	affiliateID := uuid.New()
	repo := &mockAffiliateRepository{
		getProfileByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.AffiliateProfile, error) {
			return &domain.AffiliateProfile{
				ID:            affiliateID,
				Status:        domain.AffiliateStatusActive,
				AffiliateCode: "AFF12345",
			}, nil
		},
		createLinkFunc: func(ctx context.Context, link *domain.AffiliateLink) error {
			return nil
		},
	}

	svc := newTestAffiliateService(repo)
	link, err := svc.CreateAffiliateLink(ctx, CreateAffiliateLinkInput{
		AffiliateID:  affiliateID,
		CampaignName: "summer",
		LandingPage:  "/promo",
		UTMSource:    "telegram",
	})
	if err != nil {
		t.Fatalf("expected create link to succeed, got %v", err)
	}
	if link.ReferralURL != "/r/AFF12345/summer" {
		t.Fatalf("expected campaign referral url, got %s", link.ReferralURL)
	}
}

func TestAffiliateService_TrackAffiliateClick_CreatesClickForAffiliateCode(t *testing.T) {
	ctx := context.Background()
	affiliateID := uuid.New()
	repo := &mockAffiliateRepository{
		getProfileByAffiliateCodeFunc: func(ctx context.Context, affiliateCode string) (*domain.AffiliateProfile, error) {
			return &domain.AffiliateProfile{ID: affiliateID, AffiliateCode: affiliateCode}, nil
		},
		createClickFunc: func(ctx context.Context, click *domain.AffiliateClick) error {
			return nil
		},
	}

	svc := newTestAffiliateService(repo)
	click, err := svc.TrackAffiliateClick(ctx, TrackAffiliateClickInput{
		AffiliateCode:  "AFF12345",
		LandingPage:    "/landing",
		IPHash:         "ip",
		UserAgentHash:  "ua",
		CountryCode:    "DE",
	})
	if err != nil {
		t.Fatalf("expected click tracking to succeed, got %v", err)
	}
	if click.AffiliateID != affiliateID {
		t.Fatalf("expected click to be linked to affiliate %s, got %s", affiliateID, click.AffiliateID)
	}
}

func TestAffiliateService_ListAffiliateEarnings_DefaultsLimit(t *testing.T) {
	ctx := context.Background()
	affiliateID := uuid.New()
	var gotLimit int
	repo := &mockAffiliateRepository{
		getProfileByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.AffiliateProfile, error) {
			return &domain.AffiliateProfile{ID: affiliateID, Status: domain.AffiliateStatusActive}, nil
		},
		listEarningsFunc: func(ctx context.Context, id uuid.UUID, status domain.EarningStatus, limit int) ([]domain.AffiliateEarning, error) {
			gotLimit = limit
			return []domain.AffiliateEarning{}, nil
		},
	}

	svc := newTestAffiliateService(repo)
	if _, err := svc.ListAffiliateEarnings(ctx, affiliateID, domain.EarningStatusAvailable, 0); err != nil {
		t.Fatalf("expected listing earnings to succeed, got %v", err)
	}
	if gotLimit != 50 {
		t.Fatalf("expected default limit 50, got %d", gotLimit)
	}
}

func TestAffiliateService_CreatePayoutMethod_CreatesMethodForAffiliate(t *testing.T) {
	ctx := context.Background()
	affiliateID := uuid.New()
	repo := &mockAffiliateRepository{
		getProfileByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.AffiliateProfile, error) {
			return &domain.AffiliateProfile{ID: affiliateID, Status: domain.AffiliateStatusActive}, nil
		},
		createPayoutMethodFunc: func(ctx context.Context, method *domain.AffiliatePayoutMethod) error {
			return nil
		},
	}

	svc := newTestAffiliateService(repo)
	method, err := svc.CreatePayoutMethod(ctx, CreatePayoutMethodInput{
		AffiliateID:   affiliateID,
		MethodType:    domain.PayoutMethodTypeCrypto,
		DisplayName:   "USDT TRC20",
		DetailsMasked: "TX...1234",
		IsDefault:     true,
		IsVerified:    false,
	})
	if err != nil {
		t.Fatalf("expected create payout method to succeed, got %v", err)
	}
	if method.MethodType != domain.PayoutMethodTypeCrypto {
		t.Fatalf("expected crypto method, got %s", method.MethodType)
	}
}

func TestAffiliateService_BindReferredUser_RejectsSelfReferralAndDuplicateBinding(t *testing.T) {
	ctx := context.Background()
	affiliateID := uuid.New()
	repo := &mockAffiliateRepository{
		getProfileByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.AffiliateProfile, error) {
			return &domain.AffiliateProfile{
				ID:      affiliateID,
				UserID:  42,
				Status:  domain.AffiliateStatusActive,
				Currency: "USD",
			}, nil
		},
	}
	svc := newTestAffiliateService(repo)

	_, err := svc.BindReferredUser(ctx, BindReferredUserInput{
		AffiliateID:    affiliateID,
		ReferredUserID: 42,
		ClickID:        "click-1",
	})
	if !errors.Is(err, domain.ErrSelfReferral) {
		t.Fatalf("expected ErrSelfReferral, got %v", err)
	}

	repo.getAttributionByUserIDFunc = func(ctx context.Context, referredUserID int64) (*domain.AffiliateAttribution, error) {
		return &domain.AffiliateAttribution{
			ID:             uuid.New(),
			AffiliateID:    affiliateID,
			ReferredUserID: referredUserID,
		}, nil
	}

	_, err = svc.BindReferredUser(ctx, BindReferredUserInput{
		AffiliateID:    affiliateID,
		ReferredUserID: 77,
		ClickID:        "click-2",
	})
	if !errors.Is(err, domain.ErrAttributionAlreadyBound) {
		t.Fatalf("expected ErrAttributionAlreadyBound, got %v", err)
	}
}

func TestAffiliateService_RequestPayout_RequiresEligibilityAndCreatesRequestedPayout(t *testing.T) {
	ctx := context.Background()
	affiliateID := uuid.New()
	repo := &mockAffiliateRepository{
		getProfileByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.AffiliateProfile, error) {
			return &domain.AffiliateProfile{
				ID:              affiliateID,
				UserID:          42,
				Status:          domain.AffiliateStatusActive,
				MinPayoutAmount: decimal.RequireFromString("100"),
				Currency:        "USD",
				KYCRequired:     true,
			}, nil
		},
		getAvailableBalanceFunc: func(ctx context.Context, id uuid.UUID) (decimal.Decimal, error) {
			return decimal.RequireFromString("150"), nil
		},
		createPayoutFunc: func(ctx context.Context, payout *domain.AffiliatePayout) error {
			return nil
		},
	}

	svc := newTestAffiliateService(repo)

	payout, err := svc.RequestPayout(ctx, RequestPayoutInput{
		AffiliateID:  affiliateID,
		MethodID:     uuid.New(),
		Amount:       decimal.RequireFromString("120"),
		IdempotencyKey: "payout-req-1",
		KYCApproved:  true,
		HasOpenFraud: false,
	})
	if err != nil {
		t.Fatalf("expected payout to succeed, got %v", err)
	}
	if payout.Status != domain.PayoutStatusRequested {
		t.Fatalf("expected requested payout, got %s", payout.Status)
	}
	if !payout.Amount.Equal(decimal.RequireFromString("120")) {
		t.Fatalf("expected payout amount 120, got %s", payout.Amount)
	}
	if payout.IdempotencyKey != "payout-req-1" {
		t.Fatalf("expected idempotency key to be preserved, got %q", payout.IdempotencyKey)
	}
}

func TestAffiliateService_RequestPayout_RejectsAmountGreaterThanAvailable(t *testing.T) {
	ctx := context.Background()
	affiliateID := uuid.New()
	repo := &mockAffiliateRepository{
		getProfileByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.AffiliateProfile, error) {
			return &domain.AffiliateProfile{
				ID:              affiliateID,
				UserID:          42,
				Status:          domain.AffiliateStatusActive,
				MinPayoutAmount: decimal.RequireFromString("100"),
				Currency:        "USD",
			}, nil
		},
		getAvailableBalanceFunc: func(ctx context.Context, id uuid.UUID) (decimal.Decimal, error) {
			return decimal.RequireFromString("150"), nil
		},
	}

	svc := newTestAffiliateService(repo)

	_, err := svc.RequestPayout(ctx, RequestPayoutInput{
		AffiliateID:    affiliateID,
		MethodID:       uuid.New(),
		Amount:         decimal.RequireFromString("200"),
		IdempotencyKey: "payout-req-2",
		KYCApproved:    true,
		HasOpenFraud:   false,
	})
	if !errors.Is(err, domain.ErrInvalidPayoutAmount) {
		t.Fatalf("expected ErrInvalidPayoutAmount, got %v", err)
	}
}

func TestAffiliateService_ReleaseHeldCommissions_ReturnsReleasedCount(t *testing.T) {
	ctx := context.Background()
	repo := &mockAffiliateRepository{
		releaseEligibleEarningsFunc: func(ctx context.Context, now time.Time) (int64, error) {
			return 3, nil
		},
	}

	svc := newTestAffiliateService(repo)
	count, err := svc.ReleaseHeldCommissions(ctx, time.Now())
	if err != nil {
		t.Fatalf("expected release to succeed, got %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 released earnings, got %d", count)
	}
}

func TestAffiliateService_CalculateCommission_CreatesHeldEarningFromPositiveNGR(t *testing.T) {
	ctx := context.Background()
	affiliateID := uuid.New()
	referredUserID := int64(77)
	var created *domain.AffiliateEarning

	repo := &mockAffiliateRepository{
		getProfileByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.AffiliateProfile, error) {
			return &domain.AffiliateProfile{
				ID:             affiliateID,
				Status:         domain.AffiliateStatusActive,
				CommissionRate: decimal.RequireFromString("0.25"),
				HoldPeriodDays: 14,
				Currency:       "USD",
			}, nil
		},
		createEarningFunc: func(ctx context.Context, earning *domain.AffiliateEarning) error {
			copied := *earning
			created = &copied
			return nil
		},
	}

	svc := newTestAffiliateService(repo)
	earning, err := svc.CalculateCommission(ctx, CalculateCommissionInput{
		AffiliateID:    affiliateID,
		ReferredUserID: referredUserID,
		SourceType:     "casino_session",
		SourceID:       "session-1",
		PeriodStart:    time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:      time.Date(2026, 4, 1, 23, 59, 59, 0, time.UTC),
		GGRAmount:      decimal.RequireFromString("200"),
		NGRAmount:      decimal.RequireFromString("120"),
		IdempotencyKey: "ngr-1",
	})
	if err != nil {
		t.Fatalf("expected commission calculation to succeed, got %v", err)
	}
	if created == nil {
		t.Fatal("expected earning to be created")
	}
	if created.Status != domain.EarningStatusAccrued {
		t.Fatalf("expected accrued earning, got %s", created.Status)
	}
	if !created.CommissionAmount.Equal(decimal.RequireFromString("30")) {
		t.Fatalf("expected commission amount 30, got %s", created.CommissionAmount)
	}
	if !earning.HoldUntil.Equal(created.CreatedAt.Add(14 * 24 * time.Hour)) {
		t.Fatalf("expected hold period of 14 days, got %s", earning.HoldUntil)
	}
}

func TestAffiliateService_ApproveAffiliatePayout_MarksRequestedPayoutAsPaid(t *testing.T) {
	ctx := context.Background()
	affiliateID := uuid.New()
	payoutID := uuid.New()
	var updated *domain.AffiliatePayout

	repo := &mockAffiliateRepository{
		getProfileByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.AffiliateProfile, error) {
			return &domain.AffiliateProfile{ID: affiliateID, Status: domain.AffiliateStatusActive}, nil
		},
		getPayoutByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.AffiliatePayout, error) {
			return &domain.AffiliatePayout{
				ID:          payoutID,
				AffiliateID: affiliateID,
				Amount:      decimal.RequireFromString("120"),
				Currency:    "USD",
				Status:      domain.PayoutStatusRequested,
			}, nil
		},
		updatePayoutFunc: func(ctx context.Context, payout *domain.AffiliatePayout) error {
			copied := *payout
			updated = &copied
			return nil
		},
	}

	svc := newTestAffiliateService(repo)
	payout, err := svc.ApproveAffiliatePayout(ctx, ApproveAffiliatePayoutInput{
		AffiliateID:        affiliateID,
		PayoutID:           payoutID,
		ApprovedBy:         "finance-admin",
		ProviderReference:  "provider-42",
	})
	if err != nil {
		t.Fatalf("expected payout approval to succeed, got %v", err)
	}
	if updated == nil {
		t.Fatal("expected payout update to be persisted")
	}
	if payout.Status != domain.PayoutStatusPaid {
		t.Fatalf("expected paid status, got %s", payout.Status)
	}
	if payout.ApprovedBy != "finance-admin" {
		t.Fatalf("expected approved_by to be set, got %q", payout.ApprovedBy)
	}
	if payout.ProviderReference != "provider-42" {
		t.Fatalf("expected provider reference to be set, got %q", payout.ProviderReference)
	}
}

func TestAffiliateService_RejectAffiliatePayout_SetsRejectedStatusAndReason(t *testing.T) {
	ctx := context.Background()
	affiliateID := uuid.New()
	payoutID := uuid.New()
	var updated *domain.AffiliatePayout

	repo := &mockAffiliateRepository{
		getProfileByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.AffiliateProfile, error) {
			return &domain.AffiliateProfile{ID: affiliateID, Status: domain.AffiliateStatusActive}, nil
		},
		getPayoutByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.AffiliatePayout, error) {
			return &domain.AffiliatePayout{
				ID:          payoutID,
				AffiliateID: affiliateID,
				Amount:      decimal.RequireFromString("120"),
				Currency:    "USD",
				Status:      domain.PayoutStatusReviewing,
			}, nil
		},
		updatePayoutFunc: func(ctx context.Context, payout *domain.AffiliatePayout) error {
			copied := *payout
			updated = &copied
			return nil
		},
	}

	svc := newTestAffiliateService(repo)
	payout, err := svc.RejectAffiliatePayout(ctx, RejectAffiliatePayoutInput{
		AffiliateID:      affiliateID,
		PayoutID:         payoutID,
		RejectedBy:       "finance-admin",
		RejectionReason:  "kyc mismatch",
	})
	if err != nil {
		t.Fatalf("expected payout rejection to succeed, got %v", err)
	}
	if updated == nil {
		t.Fatal("expected payout update to be persisted")
	}
	if payout.Status != domain.PayoutStatusRejected {
		t.Fatalf("expected rejected status, got %s", payout.Status)
	}
	if payout.RejectionReason != "kyc mismatch" {
		t.Fatalf("expected rejection reason to be set, got %q", payout.RejectionReason)
	}
}

func TestAffiliateService_FlagAffiliateFraud_CreatesOpenCriticalFlag(t *testing.T) {
	ctx := context.Background()
	affiliateID := uuid.New()
	var created *domain.AffiliateFraudFlag

	repo := &mockAffiliateRepository{
		getProfileByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.AffiliateProfile, error) {
			return &domain.AffiliateProfile{ID: affiliateID, Status: domain.AffiliateStatusActive}, nil
		},
		createFraudFlagFunc: func(ctx context.Context, flag *domain.AffiliateFraudFlag) error {
			copied := *flag
			created = &copied
			return nil
		},
	}

	svc := newTestAffiliateService(repo)
	flag, err := svc.FlagAffiliateFraud(ctx, FlagAffiliateFraudInput{
		AffiliateID:    affiliateID,
		ReferredUserID: 91,
		FlagType:       "multi_accounting",
		Severity:       domain.FraudSeverityCritical,
		Details: map[string]string{
			"ip_cluster": "shared-subnet",
		},
	})
	if err != nil {
		t.Fatalf("expected fraud flag creation to succeed, got %v", err)
	}
	if created == nil {
		t.Fatal("expected fraud flag to be created")
	}
	if flag.Status != domain.FraudFlagStatusOpen {
		t.Fatalf("expected open flag status, got %s", flag.Status)
	}
	if flag.Severity != domain.FraudSeverityCritical {
		t.Fatalf("expected critical severity, got %s", flag.Severity)
	}
}
