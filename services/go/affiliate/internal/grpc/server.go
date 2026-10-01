package grpc

import (
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opus-casino/proto/gen/go/affiliate/v1"
	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"
	"github.com/opus-casino/affiliate/internal/domain"
	"github.com/opus-casino/affiliate/internal/service"
)

type AffiliateGRPCHandler struct {
	pb.UnimplementedAffiliateServiceServer
	svc    *service.AffiliateService
	logger *zap.Logger
}

func NewAffiliateGRPCHandler(svc *service.AffiliateService, logger *zap.Logger) *AffiliateGRPCHandler {
	return &AffiliateGRPCHandler{
		svc:    svc,
		logger: logger,
	}
}

func (h *AffiliateGRPCHandler) EnrollAffiliate(ctx context.Context, req *pb.EnrollAffiliateRequest) (*pb.EnrollAffiliateResponse, error) {
	result, err := h.svc.EnrollAffiliate(ctx, service.EnrollAffiliateInput{
		UserID: req.GetUserId(),
		Reason: req.GetReason(),
	})
	if err != nil {
		return nil, h.mapDomainError(err)
	}

	return &pb.EnrollAffiliateResponse{
		Request: h.enrollmentToProto(result),
	}, nil
}

func (h *AffiliateGRPCHandler) GetAffiliateProfile(ctx context.Context, req *pb.GetAffiliateProfileRequest) (*pb.GetAffiliateProfileResponse, error) {
	profile, err := h.svc.GetProfileByUserID(ctx, req.GetUserId())
	if err != nil {
		return nil, h.mapDomainError(err)
	}
	if profile == nil {
		return nil, status.Error(codes.NotFound, "affiliate profile not found")
	}

	plan, _ := h.svc.GetCommissionPlanByID(ctx, profile.CommissionPlanID)

	return &pb.GetAffiliateProfileResponse{
		Profile:        h.profileToProto(profile),
		CommissionPlan: h.planToProto(plan),
	}, nil
}

func (h *AffiliateGRPCHandler) GetAffiliateDashboard(ctx context.Context, req *pb.GetAffiliateDashboardRequest) (*pb.GetAffiliateDashboardResponse, error) {
	affiliateID, err := uuid.Parse(req.GetAffiliateId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid affiliate_id")
	}

	dashboard, err := h.svc.GetDashboard(ctx, affiliateID)
	if err != nil {
		return nil, h.mapDomainError(err)
	}

	return &pb.GetAffiliateDashboardResponse{
		Dashboard: h.dashboardToProto(dashboard),
	}, nil
}

func (h *AffiliateGRPCHandler) CreateAffiliateLink(ctx context.Context, req *pb.CreateAffiliateLinkRequest) (*pb.CreateAffiliateLinkResponse, error) {
	affiliateID, err := uuid.Parse(req.GetAffiliateId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid affiliate_id")
	}

	link, err := h.svc.CreateAffiliateLink(ctx, service.CreateAffiliateLinkInput{
		AffiliateID:  affiliateID,
		CampaignName: req.GetCampaignName(),
		LandingPage:  req.GetLandingPage(),
		UTMSource:    req.GetUtmSource(),
		UTMMedium:    req.GetUtmMedium(),
		UTMCampaign:  req.GetUtmCampaign(),
	})
	if err != nil {
		return nil, h.mapDomainError(err)
	}

	return &pb.CreateAffiliateLinkResponse{
		Link: h.linkToProto(link),
	}, nil
}

func (h *AffiliateGRPCHandler) ListAffiliateLinks(ctx context.Context, req *pb.ListAffiliateLinksRequest) (*pb.ListAffiliateLinksResponse, error) {
	affiliateID, err := uuid.Parse(req.GetAffiliateId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid affiliate_id")
	}

	links, err := h.svc.ListAffiliateLinks(ctx, affiliateID)
	if err != nil {
		return nil, h.mapDomainError(err)
	}

	protoLinks := make([]*pb.AffiliateLink, len(links))
	for i, l := range links {
		protoLinks[i] = h.linkToProto(&l)
	}

	return &pb.ListAffiliateLinksResponse{
		Links: protoLinks,
	}, nil
}

func (h *AffiliateGRPCHandler) TrackAffiliateClick(ctx context.Context, req *pb.TrackAffiliateClickRequest) (*pb.TrackAffiliateClickResponse, error) {
	click, err := h.svc.TrackAffiliateClick(ctx, service.TrackAffiliateClickInput{
		AffiliateCode:      req.GetAffiliateCode(),
		Campaign:           req.GetCampaign(),
		LandingPage:        req.GetLandingPage(),
		IPHash:             req.GetIpHash(),
		UserAgentHash:      req.GetUserAgentHash(),
		DeviceFingerprint:  req.GetDeviceFingerprint(),
		CountryCode:        req.GetCountryCode(),
	})
	if err != nil {
		return nil, h.mapDomainError(err)
	}

	return &pb.TrackAffiliateClickResponse{
		ClickId:      click.ClickID,
		AffiliateId:  click.AffiliateID.String(),
		RedirectUrl:  click.LandingPage,
	}, nil
}

func (h *AffiliateGRPCHandler) BindReferredUser(ctx context.Context, req *pb.BindReferredUserRequest) (*pb.BindReferredUserResponse, error) {
	affiliateID, err := uuid.Parse(req.GetAffiliateId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid affiliate_id")
	}

	attribution, err := h.svc.BindReferredUser(ctx, service.BindReferredUserInput{
		AffiliateID:    affiliateID,
		ReferredUserID: req.GetReferredUserId(),
		ClickID:        req.GetClickId(),
	})
	if err != nil {
		return nil, h.mapDomainError(err)
	}

	return &pb.BindReferredUserResponse{
		AttributionId: attribution.ID.String(),
	}, nil
}

func (h *AffiliateGRPCHandler) CalculateCommission(ctx context.Context, req *pb.CalculateCommissionRequest) (*pb.CalculateCommissionResponse, error) {
	affiliateID, err := uuid.Parse(req.GetAffiliateId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid affiliate_id")
	}

	ggr, _ := decimal.NewFromString(req.GetGgrAmount().GetAmount())
	ngr, _ := decimal.NewFromString(req.GetNgrAmount().GetAmount())

	earning, err := h.svc.CalculateCommission(ctx, service.CalculateCommissionInput{
		AffiliateID:    affiliateID,
		ReferredUserID: req.GetReferredUserId(),
		SourceType:     req.GetSourceType(),
		SourceID:       req.GetSourceId(),
		PeriodStart:    req.GetPeriodStart().AsTime(),
		PeriodEnd:      req.GetPeriodEnd().AsTime(),
		GGRAmount:      ggr,
		NGRAmount:      ngr,
		IdempotencyKey: req.GetIdempotencyKey(),
	})
	if err != nil {
		return nil, h.mapDomainError(err)
	}

	return &pb.CalculateCommissionResponse{
		Earning: h.earningToProto(earning),
	}, nil
}

func (h *AffiliateGRPCHandler) ListAffiliateEarnings(ctx context.Context, req *pb.ListAffiliateEarningsRequest) (*pb.ListAffiliateEarningsResponse, error) {
	affiliateID, err := uuid.Parse(req.GetAffiliateId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid affiliate_id")
	}

	statusFilter := earningStatusFromProto(req.GetStatus())
	limit := int(req.GetPagination().GetPageSize())
	if limit <= 0 {
		limit = 50
	}

	earnings, err := h.svc.ListAffiliateEarnings(ctx, affiliateID, statusFilter, limit)
	if err != nil {
		return nil, h.mapDomainError(err)
	}

	protoEarnings := make([]*pb.AffiliateEarning, len(earnings))
	for i, e := range earnings {
		protoEarnings[i] = h.earningToProto(&e)
	}

	return &pb.ListAffiliateEarningsResponse{
		Items: protoEarnings,
	}, nil
}

func (h *AffiliateGRPCHandler) RequestAffiliatePayout(ctx context.Context, req *pb.RequestAffiliatePayoutRequest) (*pb.RequestAffiliatePayoutResponse, error) {
	affiliateID, err := uuid.Parse(req.GetAffiliateId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid affiliate_id")
	}
	methodID, err := uuid.Parse(req.GetMethodId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid method_id")
	}
	amount, err := decimal.NewFromString(req.GetAmount().GetAmount())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid amount")
	}

	payout, err := h.svc.RequestPayout(ctx, service.RequestPayoutInput{
		AffiliateID:    affiliateID,
		MethodID:       methodID,
		Amount:         amount,
		IdempotencyKey: req.GetIdempotencyKey(),
		KYCApproved:    false, // Should be checked by caller
		HasOpenFraud:   false, // Should be checked by caller
	})
	if err != nil {
		return nil, h.mapDomainError(err)
	}

	return &pb.RequestAffiliatePayoutResponse{
		Payout: h.payoutToProto(payout),
	}, nil
}

func (h *AffiliateGRPCHandler) ApproveAffiliatePayout(ctx context.Context, req *pb.ApproveAffiliatePayoutRequest) (*pb.ApproveAffiliatePayoutResponse, error) {
	payoutID, err := uuid.Parse(req.GetPayoutId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid payout_id")
	}

	payout, err := h.svc.ApproveAffiliatePayout(ctx, service.ApproveAffiliatePayoutInput{
		PayoutID:          payoutID,
		ApprovedBy:        req.GetApprovedBy(),
		ProviderReference: req.GetProviderReference(),
	})
	if err != nil {
		return nil, h.mapDomainError(err)
	}

	return &pb.ApproveAffiliatePayoutResponse{
		Payout: h.payoutToProto(payout),
	}, nil
}

func (h *AffiliateGRPCHandler) RejectAffiliatePayout(ctx context.Context, req *pb.RejectAffiliatePayoutRequest) (*pb.RejectAffiliatePayoutResponse, error) {
	payoutID, err := uuid.Parse(req.GetPayoutId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid payout_id")
	}

	payout, err := h.svc.RejectAffiliatePayout(ctx, service.RejectAffiliatePayoutInput{
		PayoutID:        payoutID,
		RejectedBy:      req.GetRejectedBy(),
		RejectionReason: req.GetReason(),
	})
	if err != nil {
		return nil, h.mapDomainError(err)
	}

	return &pb.RejectAffiliatePayoutResponse{
		Payout: h.payoutToProto(payout),
	}, nil
}

func (h *AffiliateGRPCHandler) FlagAffiliateFraud(ctx context.Context, req *pb.FlagAffiliateFraudRequest) (*pb.FlagAffiliateFraudResponse, error) {
	affiliateID, err := uuid.Parse(req.GetAffiliateId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid affiliate_id")
	}

	flag, err := h.svc.FlagAffiliateFraud(ctx, service.FlagAffiliateFraudInput{
		AffiliateID:    affiliateID,
		ReferredUserID: req.GetReferredUserId(),
		FlagType:       req.GetFlagType(),
		Severity:       fraudSeverityFromProto(req.GetSeverity()),
		Details:        req.GetDetails(),
	})
	if err != nil {
		return nil, h.mapDomainError(err)
	}

	return &pb.FlagAffiliateFraudResponse{
		Flag: h.fraudFlagToProto(flag),
	}, nil
}

// ============ Conversion helpers ============

// Domain enums use short snake_case values ("paid"), while the protobuf
// enums use prefixed names ("PAYOUT_STATUS_PAID"), so direct map lookups
// via pb.*_value always miss and silently yield UNSPECIFIED.
// These mappers translate explicitly in both directions.

func enrollmentStatusToProto(s domain.EnrollmentStatus) pb.EnrollmentStatus {
	switch s {
	case domain.EnrollmentStatusPendingReview:
		return pb.EnrollmentStatus_ENROLLMENT_STATUS_PENDING_REVIEW
	case domain.EnrollmentStatusApproved:
		return pb.EnrollmentStatus_ENROLLMENT_STATUS_APPROVED
	case domain.EnrollmentStatusRejected:
		return pb.EnrollmentStatus_ENROLLMENT_STATUS_REJECTED
	default:
		return pb.EnrollmentStatus_ENROLLMENT_STATUS_UNSPECIFIED
	}
}

func affiliateStatusToProto(s domain.AffiliateStatus) pb.AffiliateStatus {
	switch s {
	case domain.AffiliateStatusPendingReview:
		return pb.AffiliateStatus_AFFILIATE_STATUS_PENDING_REVIEW
	case domain.AffiliateStatusActive:
		return pb.AffiliateStatus_AFFILIATE_STATUS_ACTIVE
	case domain.AffiliateStatusSuspended:
		return pb.AffiliateStatus_AFFILIATE_STATUS_SUSPENDED
	case domain.AffiliateStatusRejected:
		return pb.AffiliateStatus_AFFILIATE_STATUS_REJECTED
	case domain.AffiliateStatusClosed:
		return pb.AffiliateStatus_AFFILIATE_STATUS_CLOSED
	default:
		return pb.AffiliateStatus_AFFILIATE_STATUS_UNSPECIFIED
	}
}

func earningStatusToProto(s domain.EarningStatus) pb.EarningStatus {
	switch s {
	case domain.EarningStatusAccrued:
		return pb.EarningStatus_EARNING_STATUS_ACCRUED
	case domain.EarningStatusPending:
		return pb.EarningStatus_EARNING_STATUS_PENDING
	case domain.EarningStatusAvailable:
		return pb.EarningStatus_EARNING_STATUS_AVAILABLE
	case domain.EarningStatusPaid:
		return pb.EarningStatus_EARNING_STATUS_PAID
	case domain.EarningStatusReversed:
		return pb.EarningStatus_EARNING_STATUS_REVERSED
	default:
		return pb.EarningStatus_EARNING_STATUS_UNSPECIFIED
	}
}

func earningStatusFromProto(s pb.EarningStatus) domain.EarningStatus {
	switch s {
	case pb.EarningStatus_EARNING_STATUS_ACCRUED:
		return domain.EarningStatusAccrued
	case pb.EarningStatus_EARNING_STATUS_PENDING:
		return domain.EarningStatusPending
	case pb.EarningStatus_EARNING_STATUS_AVAILABLE:
		return domain.EarningStatusAvailable
	case pb.EarningStatus_EARNING_STATUS_PAID:
		return domain.EarningStatusPaid
	case pb.EarningStatus_EARNING_STATUS_REVERSED:
		return domain.EarningStatusReversed
	default:
		return ""
	}
}

func payoutStatusToProto(s domain.PayoutStatus) pb.PayoutStatus {
	switch s {
	case domain.PayoutStatusRequested:
		return pb.PayoutStatus_PAYOUT_STATUS_REQUESTED
	case domain.PayoutStatusReviewing:
		return pb.PayoutStatus_PAYOUT_STATUS_REVIEWING
	case domain.PayoutStatusApproved:
		return pb.PayoutStatus_PAYOUT_STATUS_APPROVED
	case domain.PayoutStatusProcessing:
		return pb.PayoutStatus_PAYOUT_STATUS_PROCESSING
	case domain.PayoutStatusPaid:
		return pb.PayoutStatus_PAYOUT_STATUS_PAID
	case domain.PayoutStatusRejected:
		return pb.PayoutStatus_PAYOUT_STATUS_REJECTED
	case domain.PayoutStatusFailed:
		return pb.PayoutStatus_PAYOUT_STATUS_FAILED
	default:
		return pb.PayoutStatus_PAYOUT_STATUS_UNSPECIFIED
	}
}

func commissionTypeToProto(s string) pb.CommissionType {
	if s == "revshare" {
		return pb.CommissionType_COMMISSION_TYPE_REVSHARE
	}
	return pb.CommissionType_COMMISSION_TYPE_UNSPECIFIED
}

func approvalModeToProto(s domain.ApprovalMode) pb.ApprovalMode {
	switch s {
	case domain.ApprovalModeManual:
		return pb.ApprovalMode_APPROVAL_MODE_MANUAL
	case domain.ApprovalModeAutomatic:
		return pb.ApprovalMode_APPROVAL_MODE_AUTOMATIC
	default:
		return pb.ApprovalMode_APPROVAL_MODE_UNSPECIFIED
	}
}

func payoutScheduleToProto(s domain.PayoutSchedule) pb.PayoutSchedule {
	switch s {
	case domain.PayoutScheduleManual:
		return pb.PayoutSchedule_PAYOUT_SCHEDULE_MANUAL
	case domain.PayoutScheduleWeekly:
		return pb.PayoutSchedule_PAYOUT_SCHEDULE_WEEKLY
	case domain.PayoutScheduleMonthly:
		return pb.PayoutSchedule_PAYOUT_SCHEDULE_MONTHLY
	default:
		return pb.PayoutSchedule_PAYOUT_SCHEDULE_UNSPECIFIED
	}
}

func fraudSeverityToProto(s domain.FraudSeverity) pb.FraudSeverity {
	switch s {
	case domain.FraudSeverityLow:
		return pb.FraudSeverity_FRAUD_SEVERITY_LOW
	case domain.FraudSeverityMedium:
		return pb.FraudSeverity_FRAUD_SEVERITY_MEDIUM
	case domain.FraudSeverityHigh:
		return pb.FraudSeverity_FRAUD_SEVERITY_HIGH
	case domain.FraudSeverityCritical:
		return pb.FraudSeverity_FRAUD_SEVERITY_CRITICAL
	default:
		return pb.FraudSeverity_FRAUD_SEVERITY_UNSPECIFIED
	}
}

func fraudSeverityFromProto(s pb.FraudSeverity) domain.FraudSeverity {
	switch s {
	case pb.FraudSeverity_FRAUD_SEVERITY_LOW:
		return domain.FraudSeverityLow
	case pb.FraudSeverity_FRAUD_SEVERITY_MEDIUM:
		return domain.FraudSeverityMedium
	case pb.FraudSeverity_FRAUD_SEVERITY_HIGH:
		return domain.FraudSeverityHigh
	case pb.FraudSeverity_FRAUD_SEVERITY_CRITICAL:
		return domain.FraudSeverityCritical
	default:
		return ""
	}
}

func fraudFlagStatusToProto(s domain.FraudFlagStatus) pb.FraudFlagStatus {
	switch s {
	case domain.FraudFlagStatusOpen:
		return pb.FraudFlagStatus_FRAUD_FLAG_STATUS_OPEN
	case domain.FraudFlagStatusInReview:
		return pb.FraudFlagStatus_FRAUD_FLAG_STATUS_IN_REVIEW
	case domain.FraudFlagStatusResolved:
		return pb.FraudFlagStatus_FRAUD_FLAG_STATUS_RESOLVED
	case domain.FraudFlagStatusDismissed:
		return pb.FraudFlagStatus_FRAUD_FLAG_STATUS_DISMISSED
	default:
		return pb.FraudFlagStatus_FRAUD_FLAG_STATUS_UNSPECIFIED
	}
}

func (h *AffiliateGRPCHandler) enrollmentToProto(e *domain.AffiliateEnrollmentRequest) *pb.AffiliateEnrollmentRequestRecord {
	var reviewedAt, createdAt, updatedAt *timestamppb.Timestamp
	if e.ReviewedAt != nil {
		reviewedAt = timestamppb.New(*e.ReviewedAt)
	}
	createdAt = timestamppb.New(e.CreatedAt)
	updatedAt = timestamppb.New(e.UpdatedAt)

	return &pb.AffiliateEnrollmentRequestRecord{
		Id:          e.ID.String(),
		UserId:      e.UserID,
		Status:      enrollmentStatusToProto(e.Status),
		Reason:      e.Reason,
		ReviewNotes: e.ReviewNotes,
		ReviewedBy:  e.ReviewedBy,
		ReviewedAt:  reviewedAt,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
}

func (h *AffiliateGRPCHandler) profileToProto(p *domain.AffiliateProfile) *pb.AffiliateProfile {
	var approvedAt, createdAt, updatedAt *timestamppb.Timestamp
	if p.ApprovedAt != nil {
		approvedAt = timestamppb.New(*p.ApprovedAt)
	}
	createdAt = timestamppb.New(p.CreatedAt)
	updatedAt = timestamppb.New(p.UpdatedAt)

	return &pb.AffiliateProfile{
		Id:               p.ID.String(),
		UserId:           p.UserID,
		Status:           affiliateStatusToProto(p.Status),
		AffiliateCode:    p.AffiliateCode,
		CommissionPlanId: p.CommissionPlanID.String(),
		CommissionRate:   p.CommissionRate.String(),
		HoldPeriodDays:   int32(p.HoldPeriodDays),
		MinPayoutAmount:  &commonv1.Money{Amount: p.MinPayoutAmount.String(), Currency: p.Currency},
		Currency:         p.Currency,
		KycRequired:      p.KYCRequired,
		ApprovedBy:       p.ApprovedBy,
		ApprovedAt:       approvedAt,
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
		ApprovalMode:     approvalModeToProto(p.ApprovalMode),
		PayoutSchedule:   payoutScheduleToProto(p.PayoutSchedule),
	}
}

func (h *AffiliateGRPCHandler) planToProto(p *domain.AffiliateCommissionPlan) *pb.AffiliateCommissionPlan {
	if p == nil {
		return nil
	}
	return &pb.AffiliateCommissionPlan{
		Id:                     p.ID.String(),
		Name:                   p.Name,
		CommissionType:         commissionTypeToProto(p.CommissionType),
		CommissionRate:         p.CommissionRate.String(),
		HoldPeriodDays:         int32(p.HoldPeriodDays),
		MinPayoutAmount:        &commonv1.Money{Amount: p.MinPayoutAmount.String()},
		ApprovalMode:           approvalModeToProto(p.ApprovalMode),
		PayoutSchedule:         payoutScheduleToProto(p.PayoutSchedule),
		NegativeCarryoverEnabled: p.NegativeCarryoverEnabled,
		IsDefault:              p.IsDefault,
		IsActive:               p.IsActive,
		CreatedAt:              timestamppb.New(p.CreatedAt),
		UpdatedAt:              timestamppb.New(p.UpdatedAt),
	}
}

func (h *AffiliateGRPCHandler) linkToProto(l *domain.AffiliateLink) *pb.AffiliateLink {
	return &pb.AffiliateLink{
		Id:           l.ID.String(),
		AffiliateId:  l.AffiliateID.String(),
		CampaignName: l.CampaignName,
		LandingPage:  l.LandingPage,
		ReferralCode: l.ReferralCode,
		ReferralUrl:  l.ReferralURL,
		UtmSource:    l.UTMSource,
		UtmMedium:    l.UTMMedium,
		UtmCampaign:  l.UTMCampaign,
		IsActive:     l.IsActive,
		CreatedAt:    timestamppb.New(l.CreatedAt),
	}
}

func (h *AffiliateGRPCHandler) dashboardToProto(d *domain.AffiliateDashboard) *pb.AffiliateDashboard {
	var nextPayoutDate *timestamppb.Timestamp
	if d.NextPayoutDate != nil {
		nextPayoutDate = timestamppb.New(*d.NextPayoutDate)
	}

	return &pb.AffiliateDashboard{
		EarningsToday:            &commonv1.Money{Amount: d.EarningsToday.String(), Currency: d.Currency},
		EarningsThisMonth:        &commonv1.Money{Amount: d.EarningsThisMonth.String(), Currency: d.Currency},
		PendingAmount:            &commonv1.Money{Amount: d.PendingAmount.String(), Currency: d.Currency},
		AvailableAmount:          &commonv1.Money{Amount: d.AvailableAmount.String(), Currency: d.Currency},
		PaidAmount:               &commonv1.Money{Amount: d.PaidAmount.String(), Currency: d.Currency},
		NextPayoutDate:           nextPayoutDate,
		Clicks:                   d.Clicks,
		Registrations:            d.Registrations,
		FtdCount:                 d.FTDCount,
		ActivePlayers:            d.ActivePlayers,
		GgrAmount:                &commonv1.Money{Amount: d.GGRAmount.String(), Currency: d.Currency},
		NgrAmount:                &commonv1.Money{Amount: d.NGRAmount.String(), Currency: d.Currency},
		CommissionAmount:         &commonv1.Money{Amount: d.CommissionAmount.String(), Currency: d.Currency},
		CommissionReversedAmount: &commonv1.Money{Amount: d.CommissionReversedAmount.String(), Currency: d.Currency},
		CommissionAdjustedAmount: &commonv1.Money{Amount: d.CommissionAdjustedAmount.String(), Currency: d.Currency},
	}
}

func (h *AffiliateGRPCHandler) earningToProto(e *domain.AffiliateEarning) *pb.AffiliateEarning {
	return &pb.AffiliateEarning{
		Id:              e.ID.String(),
		AffiliateId:     e.AffiliateID.String(),
		ReferredUserId:  e.ReferredUserID,
		SourceType:      e.SourceType,
		SourceId:        e.SourceID,
		PeriodStart:     timestamppb.New(e.PeriodStart),
		PeriodEnd:       timestamppb.New(e.PeriodEnd),
		GgrAmount:       &commonv1.Money{Amount: e.GGRAmount.String()},
		NgrAmount:       &commonv1.Money{Amount: e.NGRAmount.String()},
		CommissionRate:  e.CommissionRate.String(),
		CommissionAmount: &commonv1.Money{Amount: e.CommissionAmount.String()},
		Status:          earningStatusToProto(e.Status),
		HoldUntil:       timestamppb.New(e.HoldUntil),
		CreatedAt:       timestamppb.New(e.CreatedAt),
		IdempotencyKey:  e.IdempotencyKey,
	}
}

func (h *AffiliateGRPCHandler) payoutToProto(p *domain.AffiliatePayout) *pb.AffiliatePayout {
	var approvedAt *timestamppb.Timestamp
	if p.ApprovedAt != nil {
		approvedAt = timestamppb.New(*p.ApprovedAt)
	}

	return &pb.AffiliatePayout{
		Id:               p.ID.String(),
		AffiliateId:      p.AffiliateID.String(),
		Amount:           &commonv1.Money{Amount: p.Amount.String(), Currency: p.Currency},
		Currency:         p.Currency,
		MethodId:         p.MethodID.String(),
		Status:           payoutStatusToProto(p.Status),
		RequestedAt:      timestamppb.New(p.RequestedAt),
		ApprovedBy:       p.ApprovedBy,
		ApprovedAt:       approvedAt,
		ProviderReference: p.ProviderReference,
		CreatedAt:        timestamppb.New(p.CreatedAt),
		RejectionReason:  p.RejectionReason,
		IdempotencyKey:   p.IdempotencyKey,
	}
}

func (h *AffiliateGRPCHandler) fraudFlagToProto(f *domain.AffiliateFraudFlag) *pb.AffiliateFraudFlag {
	var resolvedAt *timestamppb.Timestamp
	if f.ResolvedAt != nil {
		resolvedAt = timestamppb.New(*f.ResolvedAt)
	}

	return &pb.AffiliateFraudFlag{
		Id:             f.ID.String(),
		AffiliateId:    f.AffiliateID.String(),
		ReferredUserId: f.ReferredUserID,
		FlagType:       f.FlagType,
		Severity:       fraudSeverityToProto(f.Severity),
		Status:         fraudFlagStatusToProto(f.Status),
		Details:        f.Details,
		CreatedAt:      timestamppb.New(f.CreatedAt),
		ResolvedAt:     resolvedAt,
		ResolvedBy:     f.ResolvedBy,
	}
}

func (h *AffiliateGRPCHandler) mapDomainError(err error) error {
	switch err {
	case domain.ErrAffiliateNotFound, domain.ErrEnrollmentNotFound,
		domain.ErrCommissionPlanNotFound, domain.ErrEarningNotFound:
		return status.Error(codes.NotFound, err.Error())
	case domain.ErrEnrollmentAlreadyPending, domain.ErrAffiliateAlreadyExists, domain.ErrAttributionAlreadyBound:
		return status.Error(codes.AlreadyExists, err.Error())
	case domain.ErrSelfReferral, domain.ErrInvalidPayoutAmount, domain.ErrInvalidCommissionAmount,
		domain.ErrEarningNotReversible, domain.ErrValidationFailed:
		return status.Error(codes.InvalidArgument, err.Error())
	case domain.ErrAffiliateKYCRequired, domain.ErrAffiliateFraudBlocked, domain.ErrAffiliateInactive:
		return status.Error(codes.FailedPrecondition, err.Error())
	case domain.ErrMinPayoutNotReached:
		return status.Error(codes.FailedPrecondition, err.Error())
	case domain.ErrInvalidPayoutStatus:
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
