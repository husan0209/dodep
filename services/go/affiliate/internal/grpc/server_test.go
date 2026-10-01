package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opus-casino/proto/gen/go/affiliate/v1"
	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"
	"github.com/opus-casino/affiliate/internal/domain"
	"github.com/opus-casino/affiliate/internal/repository"
	"github.com/opus-casino/affiliate/internal/service"
)

// fakeRepo embeds the repository interface so tests override only what the
// exercised handler path touches.
type fakeRepo struct {
	repository.AffiliateRepository
	profile *domain.AffiliateProfile
	plan    *domain.AffiliateCommissionPlan
}

func (f *fakeRepo) GetProfileByUserID(_ context.Context, _ int64) (*domain.AffiliateProfile, error) {
	return f.profile, nil
}

func (f *fakeRepo) GetProfileByID(_ context.Context, _ uuid.UUID) (*domain.AffiliateProfile, error) {
	return f.profile, nil
}

func (f *fakeRepo) GetProfileByAffiliateCode(_ context.Context, _ string) (*domain.AffiliateProfile, error) {
	return f.profile, nil
}

func (f *fakeRepo) GetCommissionPlanByID(_ context.Context, _ uuid.UUID) (*domain.AffiliateCommissionPlan, error) {
	return f.plan, nil
}

func newHandler(repo *fakeRepo) *AffiliateGRPCHandler {
	svc := service.NewAffiliateService(repo, zap.NewNop())
	return NewAffiliateGRPCHandler(svc, zap.NewNop())
}

func activeProfile() *domain.AffiliateProfile {
	now := time.Now().UTC()
	return &domain.AffiliateProfile{
		ID:               uuid.New(),
		UserID:           1001,
		Status:           domain.AffiliateStatusActive,
		AffiliateCode:    "AFF-TEST-01",
		CommissionPlanID: uuid.New(),
		CommissionRate:   decimal.RequireFromString("0.20"),
		HoldPeriodDays:   14,
		MinPayoutAmount:  decimal.RequireFromString("100"),
		Currency:         "USD",
		KYCRequired:      true,
		ApprovalMode:     domain.ApprovalModeManual,
		PayoutSchedule:   domain.PayoutScheduleMonthly,
		ApprovedBy:       "admin",
		ApprovedAt:       &now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func defaultPlan() *domain.AffiliateCommissionPlan {
	now := time.Now().UTC()
	return &domain.AffiliateCommissionPlan{
		ID:                uuid.New(),
		Name:              "Default RevShare 20",
		CommissionType:    "revshare",
		CommissionRate:    decimal.RequireFromString("0.20"),
		HoldPeriodDays:    14,
		MinPayoutAmount:   decimal.RequireFromString("100"),
		ApprovalMode:      domain.ApprovalModeManual,
		PayoutSchedule:    domain.PayoutScheduleMonthly,
		IsDefault:         true,
		IsActive:          true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func TestMapDomainError(t *testing.T) {
	h := newHandler(&fakeRepo{})
	cases := []struct {
		err  error
		want codes.Code
	}{
		{domain.ErrAffiliateNotFound, codes.NotFound},
		{domain.ErrEnrollmentNotFound, codes.NotFound},
		{domain.ErrEnrollmentAlreadyPending, codes.AlreadyExists},
		{domain.ErrAffiliateAlreadyExists, codes.AlreadyExists},
		{domain.ErrAttributionAlreadyBound, codes.AlreadyExists},
		{domain.ErrSelfReferral, codes.InvalidArgument},
		{domain.ErrInvalidPayoutAmount, codes.InvalidArgument},
		{domain.ErrInvalidCommissionAmount, codes.InvalidArgument},
		{domain.ErrAffiliateKYCRequired, codes.FailedPrecondition},
		{domain.ErrAffiliateFraudBlocked, codes.FailedPrecondition},
		{domain.ErrAffiliateInactive, codes.FailedPrecondition},
		{domain.ErrMinPayoutNotReached, codes.FailedPrecondition},
		{domain.ErrInvalidPayoutStatus, codes.FailedPrecondition},
		{errors.New("boom"), codes.Internal},
	}
	for _, tc := range cases {
		if got := status.Code(h.mapDomainError(tc.err)); got != tc.want {
			t.Errorf("mapDomainError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

func TestHandlers_InvalidUUIDRejected(t *testing.T) {
	h := newHandler(&fakeRepo{})
	ctx := context.Background()

	if _, err := h.GetAffiliateDashboard(ctx, &pb.GetAffiliateDashboardRequest{AffiliateId: "bad"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("dashboard bad uuid: got %v, want InvalidArgument", status.Code(err))
	}
	if _, err := h.CreateAffiliateLink(ctx, &pb.CreateAffiliateLinkRequest{AffiliateId: "bad"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("create link bad uuid: got %v, want InvalidArgument", status.Code(err))
	}
	if _, err := h.ListAffiliateLinks(ctx, &pb.ListAffiliateLinksRequest{AffiliateId: "bad"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("list links bad uuid: got %v, want InvalidArgument", status.Code(err))
	}
	if _, err := h.BindReferredUser(ctx, &pb.BindReferredUserRequest{AffiliateId: "bad"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("bind bad uuid: got %v, want InvalidArgument", status.Code(err))
	}
	if _, err := h.CalculateCommission(ctx, &pb.CalculateCommissionRequest{AffiliateId: "bad"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("commission bad uuid: got %v, want InvalidArgument", status.Code(err))
	}
	if _, err := h.ListAffiliateEarnings(ctx, &pb.ListAffiliateEarningsRequest{AffiliateId: "bad"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("earnings bad uuid: got %v, want InvalidArgument", status.Code(err))
	}
	if _, err := h.RequestAffiliatePayout(ctx, &pb.RequestAffiliatePayoutRequest{AffiliateId: "bad"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("payout bad affiliate uuid: got %v, want InvalidArgument", status.Code(err))
	}
	if _, err := h.ApproveAffiliatePayout(ctx, &pb.ApproveAffiliatePayoutRequest{PayoutId: "bad"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("approve bad payout uuid: got %v, want InvalidArgument", status.Code(err))
	}
	if _, err := h.RejectAffiliatePayout(ctx, &pb.RejectAffiliatePayoutRequest{PayoutId: "bad"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("reject bad payout uuid: got %v, want InvalidArgument", status.Code(err))
	}
	if _, err := h.FlagAffiliateFraud(ctx, &pb.FlagAffiliateFraudRequest{AffiliateId: "bad"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("fraud bad uuid: got %v, want InvalidArgument", status.Code(err))
	}
}

func TestGetAffiliateProfile_UnknownUserNotFound(t *testing.T) {
	h := newHandler(&fakeRepo{}) // profile nil
	_, err := h.GetAffiliateProfile(context.Background(), &pb.GetAffiliateProfileRequest{UserId: 999})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("got %v, want NotFound", status.Code(err))
	}
}

func TestGetAffiliateProfile_Success(t *testing.T) {
	profile := activeProfile()
	h := newHandler(&fakeRepo{profile: profile, plan: defaultPlan()})
	resp, err := h.GetAffiliateProfile(context.Background(), &pb.GetAffiliateProfileRequest{UserId: profile.UserID})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if resp.GetProfile().GetAffiliateCode() != "AFF-TEST-01" {
		t.Fatalf("wrong affiliate code: %s", resp.GetProfile().GetAffiliateCode())
	}
	if resp.GetProfile().GetCommissionRate() != "0.2" {
		t.Fatalf("wrong rate: %s", resp.GetProfile().GetCommissionRate())
	}
	if got := resp.GetProfile().GetMinPayoutAmount(); got.GetAmount() != "100" || got.GetCurrency() != "USD" {
		t.Fatalf("wrong min payout money: %+v", got)
	}
	if resp.GetCommissionPlan().GetName() != "Default RevShare 20" {
		t.Fatalf("wrong plan: %s", resp.GetCommissionPlan().GetName())
	}
}

func TestTrackAffiliateClick_UnknownCodeNotFound(t *testing.T) {
	h := newHandler(&fakeRepo{}) // profile nil → ErrAffiliateNotFound
	_, err := h.TrackAffiliateClick(context.Background(), &pb.TrackAffiliateClickRequest{
		AffiliateCode: "NOPE",
		LandingPage:   "/",
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("got %v, want NotFound", status.Code(err))
	}
}

func TestCalculateCommission_ZeroNGRInvalidArgument(t *testing.T) {
	h := newHandler(&fakeRepo{profile: activeProfile()})
	now := time.Now().UTC()
	_, err := h.CalculateCommission(context.Background(), &pb.CalculateCommissionRequest{
		AffiliateId:    uuid.NewString(),
		ReferredUserId: 5,
		SourceType:     "casino",
		SourceId:       "r1",
		PeriodStart:    timestamppb.New(now.Add(-time.Hour)),
		PeriodEnd:      timestamppb.New(now),
		GgrAmount:      &commonv1.Money{Amount: "10", Currency: "USD"},
		NgrAmount:      &commonv1.Money{Amount: "0", Currency: "USD"},
		IdempotencyKey: uuid.NewString(),
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("got %v, want InvalidArgument", status.Code(err))
	}
}
