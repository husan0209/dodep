package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/opus-casino/affiliate/internal/domain"
	"github.com/shopspring/decimal"
)

type AffiliateRepository interface {
	GetEnrollmentByUserID(ctx context.Context, userID int64) (*domain.AffiliateEnrollmentRequest, error)
	CreateEnrollment(ctx context.Context, req *domain.AffiliateEnrollmentRequest) error
	UpdateEnrollmentStatus(ctx context.Context, userID int64, status domain.EnrollmentStatus, reviewNotes string, reviewedBy string, reviewedAt time.Time) error

	GetProfileByUserID(ctx context.Context, userID int64) (*domain.AffiliateProfile, error)
	GetProfileByID(ctx context.Context, affiliateID uuid.UUID) (*domain.AffiliateProfile, error)
	CreateProfile(ctx context.Context, profile *domain.AffiliateProfile) error
	UpdateProfileStatus(ctx context.Context, affiliateID uuid.UUID, from domain.AffiliateStatus, to domain.AffiliateStatus) error
	GetCommissionPlanByID(ctx context.Context, planID uuid.UUID) (*domain.AffiliateCommissionPlan, error)
	GetDefaultCommissionPlan(ctx context.Context) (*domain.AffiliateCommissionPlan, error)

	CreateLink(ctx context.Context, link *domain.AffiliateLink) error
	ListLinksByAffiliateID(ctx context.Context, affiliateID uuid.UUID) ([]domain.AffiliateLink, error)
	GetProfileByAffiliateCode(ctx context.Context, affiliateCode string) (*domain.AffiliateProfile, error)
	CreateClick(ctx context.Context, click *domain.AffiliateClick) error

	GetAttributionByReferredUserID(ctx context.Context, referredUserID int64) (*domain.AffiliateAttribution, error)
	CreateAttribution(ctx context.Context, attribution *domain.AffiliateAttribution) error

	GetDashboard(ctx context.Context, affiliateID uuid.UUID) (*domain.AffiliateDashboard, error)
	ListEarnings(ctx context.Context, affiliateID uuid.UUID, status domain.EarningStatus, limit int) ([]domain.AffiliateEarning, error)
	GetAvailableBalance(ctx context.Context, affiliateID uuid.UUID) (decimal.Decimal, error)
	CreateEarning(ctx context.Context, earning *domain.AffiliateEarning) error
	CreatePayoutMethod(ctx context.Context, method *domain.AffiliatePayoutMethod) error
	ListPayoutMethods(ctx context.Context, affiliateID uuid.UUID) ([]domain.AffiliatePayoutMethod, error)
	GetPayoutMethodByID(ctx context.Context, methodID uuid.UUID) (*domain.AffiliatePayoutMethod, error)
	UpdatePayoutMethod(ctx context.Context, method *domain.AffiliatePayoutMethod) error
	CreatePayout(ctx context.Context, payout *domain.AffiliatePayout) error
	GetPayoutByID(ctx context.Context, payoutID uuid.UUID) (*domain.AffiliatePayout, error)
	ListPayoutsByAffiliateID(ctx context.Context, affiliateID uuid.UUID, status domain.PayoutStatus, limit, offset int) ([]domain.AffiliatePayout, int64, error)
	UpdatePayout(ctx context.Context, payout *domain.AffiliatePayout) error
	CreateFraudFlag(ctx context.Context, flag *domain.AffiliateFraudFlag) error
	GetOpenFraudFlags(ctx context.Context, affiliateID uuid.UUID) ([]domain.AffiliateFraudFlag, error)

	// Admin list/query operations
	ListProfiles(ctx context.Context, status domain.AffiliateStatus, limit, offset int) ([]domain.AffiliateProfile, int64, error)
	UpdateCommissionRate(ctx context.Context, affiliateID uuid.UUID, rate decimal.Decimal) error
	CreateAdjustment(ctx context.Context, adj *domain.AffiliateAdjustment) error
	ListAllPayouts(ctx context.Context, status domain.PayoutStatus, limit, offset int) ([]domain.AffiliatePayout, int64, error)
	ListAllFraudFlags(ctx context.Context, status domain.FraudFlagStatus, limit, offset int) ([]domain.AffiliateFraudFlag, int64, error)

	ReleaseEligibleEarnings(ctx context.Context, now time.Time) (int64, error)

	ListPendingOutboxEvents(ctx context.Context, limit int) ([]domain.OutboxEvent, error)
	MarkOutboxEventPublished(ctx context.Context, eventID int64) error
	IncrementOutboxEventRetry(ctx context.Context, eventID int64) error
}
