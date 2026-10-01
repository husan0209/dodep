package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/opus-casino/affiliate/internal/domain"
	"github.com/opus-casino/affiliate/internal/fraud"
	"github.com/opus-casino/affiliate/internal/repository"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type AffiliateService struct {
	repo     repository.AffiliateRepository
	logger   *zap.Logger
	fraudCfg fraud.Config
}

func NewAffiliateService(repo repository.AffiliateRepository, logger *zap.Logger) *AffiliateService {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &AffiliateService{
		repo:     repo,
		logger:   logger,
		fraudCfg: fraud.DefaultConfig(),
	}
}

// SetFraudConfig overrides the anti-fraud thresholds (risk-manager tuning
// without code changes).
func (s *AffiliateService) SetFraudConfig(cfg fraud.Config) {
	s.fraudCfg = cfg
}

// fraudStore adapts the repository to the fraud engine's signal interface.
type fraudStore struct {
	repo repository.AffiliateRepository
}

func (f fraudStore) GetClickByClickID(ctx context.Context, clickID string) (*domain.AffiliateClick, error) {
	return f.repo.GetClickByClickID(ctx, clickID)
}

func (f fraudStore) CountClicksSince(ctx context.Context, affiliateID uuid.UUID, since time.Time) (int64, error) {
	return f.repo.CountClicksSince(ctx, affiliateID, since)
}

func (f fraudStore) CountAttributionsSince(ctx context.Context, affiliateID uuid.UUID, since time.Time) (int64, error) {
	return f.repo.CountAttributionsSince(ctx, affiliateID, since)
}

func (f fraudStore) CountReferredUsersByDevice(ctx context.Context, affiliateID uuid.UUID, deviceFP string, since time.Time) (int64, error) {
	return f.repo.CountReferredUsersByDevice(ctx, affiliateID, deviceFP, since)
}

func (f fraudStore) CountReferredUsersByIP(ctx context.Context, affiliateID uuid.UUID, ipHash string, since time.Time) (int64, error) {
	return f.repo.CountReferredUsersByIP(ctx, affiliateID, ipHash, since)
}

// evaluateAttributionFraud runs the automatic anti-fraud engine after a
// successful binding and persists one open flag per finding. It never fails
// the attribution itself: fraud must not break legitimate signups.
func (s *AffiliateService) evaluateAttributionFraud(ctx context.Context, attribution *domain.AffiliateAttribution) {
	verdict, err := fraud.Evaluate(ctx, fraudStore{repo: s.repo}, s.fraudCfg, fraud.Input{
		AffiliateID:    attribution.AffiliateID,
		ReferredUserID: attribution.ReferredUserID,
		ClickID:        attribution.ClickID,
		EvaluatedAt:    time.Now().UTC(),
	})
	if err != nil {
		s.logger.Warn("affiliate fraud evaluation failed",
			zap.String("attribution_id", attribution.ID.String()),
			zap.Error(err),
		)
		return
	}
	if verdict == nil {
		return
	}
	for _, finding := range verdict.Findings {
		_, flagErr := s.FlagAffiliateFraud(ctx, FlagAffiliateFraudInput{
			AffiliateID:    attribution.AffiliateID,
			ReferredUserID: attribution.ReferredUserID,
			FlagType:       finding.RuleID,
			Severity:       finding.Severity,
			Details:        finding.Details,
		})
		if flagErr != nil {
			s.logger.Warn("affiliate fraud flag creation failed",
				zap.String("rule_id", finding.RuleID),
				zap.Error(flagErr),
			)
		}
	}
	if verdict.RecommendSuspension {
		s.logger.Warn("affiliate recommended for suspension",
			zap.String("affiliate_id", attribution.AffiliateID.String()),
			zap.String("max_severity", string(verdict.MaxSeverity)),
		)
	}
}

type EnrollAffiliateInput struct {
	UserID int64
	Reason string
}

type ApproveAffiliateInput struct {
	UserID           int64
	ApprovedBy       string
	CommissionPlanID uuid.UUID
	CommissionRate   decimal.Decimal
	HoldPeriodDays   int
	MinPayoutAmount  decimal.Decimal
	Currency         string
	RequireKYC       bool
}

type CreateAffiliateLinkInput struct {
	AffiliateID   uuid.UUID
	CampaignName  string
	LandingPage   string
	UTMSource     string
	UTMMedium     string
	UTMCampaign   string
}

type TrackAffiliateClickInput struct {
	AffiliateCode      string
	Campaign           string
	LandingPage        string
	IPHash             string
	UserAgentHash      string
	DeviceFingerprint  string
	CountryCode        string
}

type BindReferredUserInput struct {
	AffiliateID    uuid.UUID
	ReferredUserID int64
	ClickID        string
}

type RequestPayoutInput struct {
	AffiliateID    uuid.UUID
	MethodID       uuid.UUID
	Amount         decimal.Decimal
	IdempotencyKey string
	KYCApproved    bool
	HasOpenFraud   bool
}

type CalculateCommissionInput struct {
	AffiliateID    uuid.UUID
	ReferredUserID int64
	SourceType     string
	SourceID       string
	PeriodStart    time.Time
	PeriodEnd      time.Time
	GGRAmount      decimal.Decimal
	NGRAmount      decimal.Decimal
	IdempotencyKey string
}

type ApproveAffiliatePayoutInput struct {
	AffiliateID       uuid.UUID
	PayoutID          uuid.UUID
	ApprovedBy        string
	ProviderReference string
}

type RejectAffiliatePayoutInput struct {
	AffiliateID     uuid.UUID
	PayoutID        uuid.UUID
	RejectedBy      string
	RejectionReason string
}

type FlagAffiliateFraudInput struct {
	AffiliateID    uuid.UUID
	ReferredUserID int64
	FlagType       string
	Severity       domain.FraudSeverity
	Details        map[string]string
}

type CreatePayoutMethodInput struct {
	AffiliateID uuid.UUID
	MethodType  domain.PayoutMethodType
	DisplayName string
	DetailsMasked string
	IsDefault   bool
	IsVerified  bool
}

type UpdatePayoutMethodInput struct {
	AffiliateID   uuid.UUID
	MethodID      uuid.UUID
	MethodType    domain.PayoutMethodType
	DisplayName   string
	DetailsMasked string
	IsDefault     bool
	IsVerified    bool
}

func (s *AffiliateService) EnrollAffiliate(ctx context.Context, in EnrollAffiliateInput) (*domain.AffiliateEnrollmentRequest, error) {
	existing, err := s.repo.GetEnrollmentByUserID(ctx, in.UserID)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.Status == domain.EnrollmentStatusPendingReview {
		return nil, domain.ErrEnrollmentAlreadyPending
	}

	req := &domain.AffiliateEnrollmentRequest{
		ID:        uuid.New(),
		UserID:    in.UserID,
		Status:    domain.EnrollmentStatusPendingReview,
		Reason:    in.Reason,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repo.CreateEnrollment(ctx, req); err != nil {
		return nil, err
	}

	return req, nil
}

func (s *AffiliateService) GetAffiliateProfile(ctx context.Context, userID int64) (*domain.AffiliateProfile, error) {
	profile, err := s.repo.GetProfileByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, domain.ErrAffiliateNotFound
	}
	return profile, nil
}

func (s *AffiliateService) ApproveAffiliate(ctx context.Context, in ApproveAffiliateInput) (*domain.AffiliateProfile, error) {
	req, err := s.repo.GetEnrollmentByUserID(ctx, in.UserID)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, domain.ErrEnrollmentNotFound
	}
	existingProfile, err := s.repo.GetProfileByUserID(ctx, in.UserID)
	if err != nil {
		return nil, err
	}
	if existingProfile != nil {
		return nil, domain.ErrAffiliateAlreadyExists
	}

	plan, err := s.resolveCommissionPlan(ctx, in)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	profile := &domain.AffiliateProfile{
		ID:               uuid.New(),
		UserID:           in.UserID,
		Status:           domain.AffiliateStatusActive,
		AffiliateCode:    domain.GenerateAffiliateCode(),
		CommissionPlanID: plan.ID,
		CommissionRate:   plan.CommissionRate,
		HoldPeriodDays:   plan.HoldPeriodDays,
		MinPayoutAmount:  plan.MinPayoutAmount,
		Currency:         defaultCurrency(in.Currency),
		KYCRequired:      in.RequireKYC,
		ApprovalMode:     plan.ApprovalMode,
		PayoutSchedule:   plan.PayoutSchedule,
		ApprovedBy:       in.ApprovedBy,
		ApprovedAt:       &now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := s.repo.CreateProfile(ctx, profile); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateEnrollmentStatus(ctx, in.UserID, domain.EnrollmentStatusApproved, "", in.ApprovedBy, now); err != nil {
		return nil, err
	}

	return profile, nil
}

func (s *AffiliateService) GetAffiliateDashboard(ctx context.Context, affiliateID uuid.UUID) (*domain.AffiliateDashboard, error) {
	dashboard, err := s.repo.GetDashboard(ctx, affiliateID)
	if err != nil {
		return nil, err
	}
	if dashboard == nil {
		return nil, domain.ErrAffiliateNotFound
	}
	return dashboard, nil
}

func (s *AffiliateService) CreateAffiliateLink(ctx context.Context, in CreateAffiliateLinkInput) (*domain.AffiliateLink, error) {
	profile, err := s.repo.GetProfileByID(ctx, in.AffiliateID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, domain.ErrAffiliateNotFound
	}
	if profile.Status != domain.AffiliateStatusActive {
		return nil, domain.ErrAffiliateInactive
	}

	link := &domain.AffiliateLink{
		ID:           uuid.New(),
		AffiliateID:  in.AffiliateID,
		CampaignName: in.CampaignName,
		LandingPage:  in.LandingPage,
		ReferralCode: profile.AffiliateCode,
		ReferralURL:  buildReferralURL(profile.AffiliateCode, in.CampaignName),
		UTMSource:    in.UTMSource,
		UTMMedium:    in.UTMMedium,
		UTMCampaign:  in.UTMCampaign,
		IsActive:     true,
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.repo.CreateLink(ctx, link); err != nil {
		return nil, err
	}
	return link, nil
}

func (s *AffiliateService) ListAffiliateLinks(ctx context.Context, affiliateID uuid.UUID) ([]domain.AffiliateLink, error) {
	profile, err := s.repo.GetProfileByID(ctx, affiliateID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, domain.ErrAffiliateNotFound
	}
	return s.repo.ListLinksByAffiliateID(ctx, affiliateID)
}

func (s *AffiliateService) TrackAffiliateClick(ctx context.Context, in TrackAffiliateClickInput) (*domain.AffiliateClick, error) {
	profile, err := s.repo.GetProfileByAffiliateCode(ctx, in.AffiliateCode)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, domain.ErrAffiliateNotFound
	}

	click := &domain.AffiliateClick{
		ID:                uuid.New(),
		AffiliateID:       profile.ID,
		ClickID:           uuid.NewString(),
		IPHash:            in.IPHash,
		UserAgentHash:     in.UserAgentHash,
		DeviceFingerprint: in.DeviceFingerprint,
		CountryCode:       in.CountryCode,
		LandingPage:       in.LandingPage,
		CreatedAt:         time.Now().UTC(),
	}
	if err := s.repo.CreateClick(ctx, click); err != nil {
		return nil, err
	}
	return click, nil
}

func (s *AffiliateService) BindReferredUser(ctx context.Context, in BindReferredUserInput) (*domain.AffiliateAttribution, error) {
	profile, err := s.repo.GetProfileByID(ctx, in.AffiliateID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, domain.ErrAffiliateNotFound
	}
	if profile.Status != domain.AffiliateStatusActive {
		return nil, domain.ErrAffiliateInactive
	}
	if profile.UserID == in.ReferredUserID {
		return nil, domain.ErrSelfReferral
	}

	existing, err := s.repo.GetAttributionByReferredUserID(ctx, in.ReferredUserID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, domain.ErrAttributionAlreadyBound
	}

	now := time.Now().UTC()
	attribution := &domain.AffiliateAttribution{
		ID:             uuid.New(),
		AffiliateID:    in.AffiliateID,
		ReferredUserID: in.ReferredUserID,
		ClickID:        in.ClickID,
		AttributedAt:   now,
		CreatedAt:      now,
	}
	if err := s.repo.CreateAttribution(ctx, attribution); err != nil {
		return nil, err
	}

	// Automatic anti-fraud screening (flag-first, never blocks the binding).
	s.evaluateAttributionFraud(ctx, attribution)

	return attribution, nil
}

func (s *AffiliateService) ListAffiliateEarnings(ctx context.Context, affiliateID uuid.UUID, status domain.EarningStatus, limit int) ([]domain.AffiliateEarning, error) {
	profile, err := s.repo.GetProfileByID(ctx, affiliateID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, domain.ErrAffiliateNotFound
	}
	if limit <= 0 {
		limit = 50
	}
	return s.repo.ListEarnings(ctx, affiliateID, status, limit)
}

func (s *AffiliateService) CreatePayoutMethod(ctx context.Context, in CreatePayoutMethodInput) (*domain.AffiliatePayoutMethod, error) {
	profile, err := s.repo.GetProfileByID(ctx, in.AffiliateID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, domain.ErrAffiliateNotFound
	}

	method := &domain.AffiliatePayoutMethod{
		ID:            uuid.New(),
		AffiliateID:   in.AffiliateID,
		MethodType:    in.MethodType,
		DisplayName:   in.DisplayName,
		DetailsMasked: in.DetailsMasked,
		IsDefault:     in.IsDefault,
		IsVerified:    in.IsVerified,
		CreatedAt:     time.Now().UTC(),
	}
	if err := s.repo.CreatePayoutMethod(ctx, method); err != nil {
		return nil, err
	}
	return method, nil
}

func (s *AffiliateService) ListPayoutMethods(ctx context.Context, affiliateID uuid.UUID) ([]domain.AffiliatePayoutMethod, error) {
	profile, err := s.repo.GetProfileByID(ctx, affiliateID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, domain.ErrAffiliateNotFound
	}
	return s.repo.ListPayoutMethods(ctx, affiliateID)
}

func (s *AffiliateService) UpdatePayoutMethod(ctx context.Context, in UpdatePayoutMethodInput) (*domain.AffiliatePayoutMethod, error) {
	profile, err := s.repo.GetProfileByID(ctx, in.AffiliateID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, domain.ErrAffiliateNotFound
	}

	method, err := s.repo.GetPayoutMethodByID(ctx, in.MethodID)
	if err != nil {
		return nil, err
	}
	if method == nil || method.AffiliateID != in.AffiliateID {
		return nil, domain.ErrPayoutMethodNotFound
	}

	method.MethodType = in.MethodType
	method.DisplayName = in.DisplayName
	method.DetailsMasked = in.DetailsMasked
	method.IsDefault = in.IsDefault
	method.IsVerified = in.IsVerified

	if err := s.repo.UpdatePayoutMethod(ctx, method); err != nil {
		return nil, err
	}
	return method, nil
}

func (s *AffiliateService) RequestPayout(ctx context.Context, in RequestPayoutInput) (*domain.AffiliatePayout, error) {
	profile, err := s.repo.GetProfileByID(ctx, in.AffiliateID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, domain.ErrAffiliateNotFound
	}

	available, err := s.repo.GetAvailableBalance(ctx, in.AffiliateID)
	if err != nil {
		return nil, err
	}

	if !in.HasOpenFraud {
		flags, err := s.repo.GetOpenFraudFlags(ctx, in.AffiliateID)
		if err != nil {
			return nil, err
		}
		in.HasOpenFraud = len(flags) > 0
	}

	if err := profile.ValidatePayoutEligibility(domain.PayoutEligibility{
		AvailableAmount: available,
		KYCApproved:     in.KYCApproved,
		HasOpenFraud:    in.HasOpenFraud,
		RequestedAt:     time.Now().UTC(),
	}); err != nil {
		return nil, err
	}
	if in.Amount.LessThanOrEqual(decimal.Zero) || in.Amount.GreaterThan(available) {
		return nil, domain.ErrInvalidPayoutAmount
	}

	payout := &domain.AffiliatePayout{
		ID:          uuid.New(),
		AffiliateID: in.AffiliateID,
		MethodID:    in.MethodID,
		Amount:      in.Amount,
		Currency:    profile.Currency,
		Status:      domain.PayoutStatusRequested,
		IdempotencyKey: in.IdempotencyKey,
		RequestedAt: time.Now().UTC(),
	}
	if err := s.repo.CreatePayout(ctx, payout); err != nil {
		return nil, err
	}

	return payout, nil
}

func (s *AffiliateService) CalculateCommission(ctx context.Context, in CalculateCommissionInput) (*domain.AffiliateEarning, error) {
	profile, err := s.repo.GetProfileByID(ctx, in.AffiliateID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, domain.ErrAffiliateNotFound
	}
	if profile.Status != domain.AffiliateStatusActive {
		return nil, domain.ErrAffiliateInactive
	}
	if in.NGRAmount.LessThanOrEqual(decimal.Zero) {
		return nil, domain.ErrInvalidCommissionAmount
	}

	now := time.Now().UTC()
	commissionAmount := in.NGRAmount.Mul(profile.CommissionRate)
	if commissionAmount.LessThanOrEqual(decimal.Zero) {
		return nil, domain.ErrInvalidCommissionAmount
	}

	earning := &domain.AffiliateEarning{
		ID:               uuid.New(),
		AffiliateID:      in.AffiliateID,
		ReferredUserID:   in.ReferredUserID,
		SourceType:       in.SourceType,
		SourceID:         in.SourceID,
		PeriodStart:      in.PeriodStart,
		PeriodEnd:        in.PeriodEnd,
		GGRAmount:        in.GGRAmount,
		NGRAmount:        in.NGRAmount,
		CommissionRate:   profile.CommissionRate,
		CommissionAmount: commissionAmount,
		Status:           domain.EarningStatusAccrued,
		HoldUntil:        now.AddDate(0, 0, profile.HoldPeriodDays),
		IdempotencyKey:   in.IdempotencyKey,
		CreatedAt:        now,
	}
	if err := s.repo.CreateEarning(ctx, earning); err != nil {
		return nil, err
	}
	return earning, nil
}

func (s *AffiliateService) ApproveAffiliatePayout(ctx context.Context, in ApproveAffiliatePayoutInput) (*domain.AffiliatePayout, error) {
	payout, err := s.repo.GetPayoutByID(ctx, in.PayoutID)
	if err != nil {
		return nil, err
	}
	if payout == nil {
		return nil, domain.ErrPayoutNotFound
	}
	if _, err := s.requireActiveProfile(ctx, payout.AffiliateID); err != nil {
		return nil, err
	}
	if payout.Status != domain.PayoutStatusRequested && payout.Status != domain.PayoutStatusReviewing && payout.Status != domain.PayoutStatusApproved {
		return nil, domain.ErrInvalidPayoutStatus
	}

	now := time.Now().UTC()
	payout.Status = domain.PayoutStatusPaid
	payout.ApprovedBy = in.ApprovedBy
	payout.ApprovedAt = &now
	payout.ProviderReference = in.ProviderReference

	if err := s.repo.UpdatePayout(ctx, payout); err != nil {
		return nil, err
	}
	return payout, nil
}

func (s *AffiliateService) RejectAffiliatePayout(ctx context.Context, in RejectAffiliatePayoutInput) (*domain.AffiliatePayout, error) {
	payout, err := s.repo.GetPayoutByID(ctx, in.PayoutID)
	if err != nil {
		return nil, err
	}
	if payout == nil {
		return nil, domain.ErrPayoutNotFound
	}
	if _, err := s.requireActiveProfile(ctx, payout.AffiliateID); err != nil {
		return nil, err
	}
	if payout.Status != domain.PayoutStatusRequested && payout.Status != domain.PayoutStatusReviewing && payout.Status != domain.PayoutStatusApproved && payout.Status != domain.PayoutStatusProcessing {
		return nil, domain.ErrInvalidPayoutStatus
	}

	payout.Status = domain.PayoutStatusRejected
	payout.RejectionReason = in.RejectionReason

	if err := s.repo.UpdatePayout(ctx, payout); err != nil {
		return nil, err
	}
	return payout, nil
}

func (s *AffiliateService) FlagAffiliateFraud(ctx context.Context, in FlagAffiliateFraudInput) (*domain.AffiliateFraudFlag, error) {
	if _, err := s.requireActiveProfile(ctx, in.AffiliateID); err != nil {
		return nil, err
	}

	flag := &domain.AffiliateFraudFlag{
		ID:             uuid.New(),
		AffiliateID:    in.AffiliateID,
		ReferredUserID: in.ReferredUserID,
		FlagType:       in.FlagType,
		Severity:       in.Severity,
		Status:         domain.FraudFlagStatusOpen,
		Details:        in.Details,
		CreatedAt:      time.Now().UTC(),
	}
	if err := s.repo.CreateFraudFlag(ctx, flag); err != nil {
		return nil, err
	}
	return flag, nil
}

func (s *AffiliateService) ReleaseHeldCommissions(ctx context.Context, now time.Time) (int64, error) {
	return s.repo.ReleaseEligibleEarnings(ctx, now)
}

func (s *AffiliateService) resolveCommissionPlan(ctx context.Context, in ApproveAffiliateInput) (*domain.AffiliateCommissionPlan, error) {
	if in.CommissionPlanID != uuid.Nil {
		plan, err := s.repo.GetCommissionPlanByID(ctx, in.CommissionPlanID)
		if err != nil {
			return nil, err
		}
		if plan != nil {
			return s.overridePlan(plan, in), nil
		}
		// Explicit plan ID is unknown: fall through to the default plan
		// WITHOUT stamping the unknown ID (it would violate
		// affiliate_profiles_commission_plan_id_fkey on profile insert).
		in.CommissionPlanID = uuid.Nil
	}

	plan, err := s.repo.GetDefaultCommissionPlan(ctx)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		// No plan row exists at all (seed missing): fail with a clear
		// domain error instead of crashing on the FK constraint.
		return nil, domain.ErrCommissionPlanNotFound
	}
	return s.overridePlan(plan, in), nil
}

func (s *AffiliateService) overridePlan(plan *domain.AffiliateCommissionPlan, in ApproveAffiliateInput) *domain.AffiliateCommissionPlan {
	cloned := *plan
	if in.CommissionPlanID != uuid.Nil {
		cloned.ID = in.CommissionPlanID
	}
	if !in.CommissionRate.IsZero() {
		cloned.CommissionRate = in.CommissionRate
	}
	if in.HoldPeriodDays > 0 {
		cloned.HoldPeriodDays = in.HoldPeriodDays
	}
	if !in.MinPayoutAmount.IsZero() {
		cloned.MinPayoutAmount = in.MinPayoutAmount
	}
	return &cloned
}

func buildReferralURL(code string, campaign string) string {
	if campaign == "" {
		return "/r/" + code
	}
	return "/r/" + code + "/" + campaign
}

func defaultCurrency(currency string) string {
	if currency == "" {
		return "USD"
	}
	return currency
}

func (s *AffiliateService) requireActiveProfile(ctx context.Context, affiliateID uuid.UUID) (*domain.AffiliateProfile, error) {
	profile, err := s.repo.GetProfileByID(ctx, affiliateID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, domain.ErrAffiliateNotFound
	}
	if profile.Status != domain.AffiliateStatusActive {
		return nil, domain.ErrAffiliateInactive
	}
	return profile, nil
}

// ============ Delegate / public convenience methods ============

func (s *AffiliateService) GetProfileByUserID(ctx context.Context, userID int64) (*domain.AffiliateProfile, error) {
	return s.repo.GetProfileByUserID(ctx, userID)
}

func (s *AffiliateService) GetProfileByID(ctx context.Context, affiliateID uuid.UUID) (*domain.AffiliateProfile, error) {
	return s.repo.GetProfileByID(ctx, affiliateID)
}

func (s *AffiliateService) GetCommissionPlanByID(ctx context.Context, planID uuid.UUID) (*domain.AffiliateCommissionPlan, error) {
	return s.repo.GetCommissionPlanByID(ctx, planID)
}

func (s *AffiliateService) GetDashboard(ctx context.Context, affiliateID uuid.UUID) (*domain.AffiliateDashboard, error) {
	return s.repo.GetDashboard(ctx, affiliateID)
}

// ============ Admin operations ============

type RejectAffiliateInput struct {
	UserID      int64
	RejectedBy  string
	ReviewNotes string
}

func (s *AffiliateService) RejectAffiliate(ctx context.Context, in RejectAffiliateInput) error {
	enrollment, err := s.repo.GetEnrollmentByUserID(ctx, in.UserID)
	if err != nil {
		return err
	}
	if enrollment == nil {
		return domain.ErrEnrollmentNotFound
	}
	if enrollment.Status != domain.EnrollmentStatusPendingReview {
		return domain.ErrInvalidPayoutStatus
	}

	now := time.Now().UTC()
	return s.repo.UpdateEnrollmentStatus(ctx, in.UserID, domain.EnrollmentStatusRejected, in.ReviewNotes, in.RejectedBy, now)
}

func (s *AffiliateService) SuspendAffiliate(ctx context.Context, affiliateID uuid.UUID) error {
	profile, err := s.repo.GetProfileByID(ctx, affiliateID)
	if err != nil {
		return err
	}
	if profile == nil {
		return domain.ErrAffiliateNotFound
	}
	if !profile.Status.CanTransitionTo(domain.AffiliateStatusSuspended) {
		return domain.ErrAffiliateInactive
	}

	return s.repo.UpdateProfileStatus(ctx, affiliateID, profile.Status, domain.AffiliateStatusSuspended)
}

func (s *AffiliateService) UpdateCommissionRate(ctx context.Context, affiliateID uuid.UUID, rate decimal.Decimal) error {
	profile, err := s.repo.GetProfileByID(ctx, affiliateID)
	if err != nil {
		return err
	}
	if profile == nil {
		return domain.ErrAffiliateNotFound
	}

	if rate.IsNegative() || rate.GreaterThan(decimal.NewFromFloat(1.0)) {
		return domain.ErrInvalidCommissionAmount
	}

	return s.repo.UpdateCommissionRate(ctx, affiliateID, rate)
}

// ReverseCommissionInput describes a manual commission reversal.
type ReverseCommissionInput struct {
	AffiliateID uuid.UUID
	EarningID   uuid.UUID
	Reason      string
	ReversedBy  string
}

// ReverseCommission cancels an accrued commission (fraud, chargeback,
// correction) and moves the money to the `reversed` ledger account.
// It is idempotent: reversing an already reversed earning is a no-op.
func (s *AffiliateService) ReverseCommission(
	ctx context.Context,
	in ReverseCommissionInput,
) (*domain.AffiliateEarning, error) {
	if in.Reason == "" {
		return nil, domain.ErrValidationFailed
	}

	earnings, err := s.repo.ListEarnings(ctx, in.AffiliateID, "", 0)
	if err != nil {
		return nil, err
	}
	var target *domain.AffiliateEarning
	for i := range earnings {
		if earnings[i].ID == in.EarningID {
			target = &earnings[i]
			break
		}
	}
	if target == nil {
		return nil, domain.ErrEarningNotFound
	}

	reversed, err := s.repo.ReverseEarning(ctx, in.EarningID, in.Reason, in.ReversedBy)
	if err != nil {
		return nil, err
	}

	s.logger.Info("commission reversed",
		zap.String("affiliate_id", in.AffiliateID.String()),
		zap.String("earning_id", in.EarningID.String()),
		zap.String("amount", reversed.CommissionAmount.String()),
		zap.String("reason", in.Reason),
		zap.String("reversed_by", in.ReversedBy),
	)

	return reversed, nil
}

// ListLedgerBalances returns the affiliate ledger balances (pending,
// available, paid, reversed, adjusted).
func (s *AffiliateService) ListLedgerBalances(
	ctx context.Context,
	affiliateID uuid.UUID,
) ([]repository.LedgerAccountBalance, error) {
	if _, err := s.repo.GetProfileByID(ctx, affiliateID); err != nil {
		return nil, err
	}
	return s.repo.GetLedgerBalances(ctx, affiliateID)
}

// ReconcileAffiliate compares ledger balances with balances derived from
// earnings/payouts/adjustments. Any divergence must block payouts.
func (s *AffiliateService) ReconcileAffiliate(
	ctx context.Context,
	affiliateID uuid.UUID,
) (*repository.LedgerReconciliationReport, error) {
	report, err := s.repo.ReconcileLedger(ctx, affiliateID)
	if err != nil {
		return nil, err
	}
	if report.Balanced {
		s.logger.Info("affiliate ledger reconciled",
			zap.String("affiliate_id", affiliateID.String()),
			zap.String("currency", report.Currency),
		)
		return report, nil
	}

	s.logger.Error("affiliate ledger divergence detected",
		zap.String("affiliate_id", affiliateID.String()),
		zap.String("currency", report.Currency),
		zap.Strings("divergences", report.Divergences),
	)
	return report, nil
}

// ListCommissionPlans returns all commission plans for admin management.
func (s *AffiliateService) ListCommissionPlans(ctx context.Context) ([]domain.AffiliateCommissionPlan, error) {
	return s.repo.ListCommissionPlans(ctx)
}

// ChangeCommissionPlan switches an affiliate to another commission plan,
// snapshotting the plan's rate/hold/min-payout onto the profile.
// Only active plans can be assigned; past accruals keep their old rate.
func (s *AffiliateService) ChangeCommissionPlan(ctx context.Context, affiliateID uuid.UUID, planID uuid.UUID) (*domain.AffiliateProfile, error) {
	profile, err := s.repo.GetProfileByID(ctx, affiliateID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, domain.ErrAffiliateNotFound
	}

	plan, err := s.repo.GetCommissionPlanByID(ctx, planID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, domain.ErrCommissionPlanNotFound
	}
	if !plan.IsActive {
		return nil, domain.ErrInvalidCommissionAmount
	}

	if err := s.repo.UpdateProfilePlan(ctx, affiliateID, plan); err != nil {
		return nil, err
	}

	updated, err := s.repo.GetProfileByID(ctx, affiliateID)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, domain.ErrAffiliateNotFound
	}

	s.logger.Info("commission plan changed",
		zap.String("affiliate_id", affiliateID.String()),
		zap.String("plan_id", plan.ID.String()),
		zap.String("rate", plan.CommissionRate.String()),
	)

	return updated, nil
}

type CreateAdjustmentInput struct {
	AffiliateID    uuid.UUID
	AdjustmentType domain.AdjustmentType
	Amount         decimal.Decimal
	Reason         string
	CreatedBy      string
}

func (s *AffiliateService) CreateAdjustment(ctx context.Context, in CreateAdjustmentInput) (*domain.AffiliateAdjustment, error) {
	profile, err := s.repo.GetProfileByID(ctx, in.AffiliateID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, domain.ErrAffiliateNotFound
	}

	adj := &domain.AffiliateAdjustment{
		ID:             uuid.New(),
		AffiliateID:    in.AffiliateID,
		AdjustmentType: in.AdjustmentType,
		Amount:         in.Amount,
		Currency:       profile.Currency,
		Reason:         in.Reason,
		CreatedBy:      in.CreatedBy,
		CreatedAt:      time.Now().UTC(),
	}

	if err := s.repo.CreateAdjustment(ctx, adj); err != nil {
		return nil, err
	}

	s.logger.Info("adjustment created",
		zap.String("affiliate_id", in.AffiliateID.String()),
		zap.String("type", string(in.AdjustmentType)),
		zap.String("amount", in.Amount.String()),
		zap.String("created_by", in.CreatedBy),
	)

	return adj, nil
}

func (s *AffiliateService) ListAffiliates(ctx context.Context, status domain.AffiliateStatus, limit, offset int) ([]domain.AffiliateProfile, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.repo.ListProfiles(ctx, status, limit, offset)
}

func (s *AffiliateService) ListAllPayouts(ctx context.Context, status domain.PayoutStatus, limit, offset int) ([]domain.AffiliatePayout, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.repo.ListAllPayouts(ctx, status, limit, offset)
}

func (s *AffiliateService) ListPayoutsByAffiliate(ctx context.Context, affiliateID uuid.UUID, status domain.PayoutStatus, limit, offset int) ([]domain.AffiliatePayout, int64, error) {
	if _, err := s.requireActiveProfile(ctx, affiliateID); err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.repo.ListPayoutsByAffiliateID(ctx, affiliateID, status, limit, offset)
}

func (s *AffiliateService) ListFraudFlags(ctx context.Context, status domain.FraudFlagStatus, limit, offset int) ([]domain.AffiliateFraudFlag, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.repo.ListAllFraudFlags(ctx, status, limit, offset)
}
