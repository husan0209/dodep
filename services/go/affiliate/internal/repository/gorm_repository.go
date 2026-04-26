package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/opus-casino/affiliate/internal/domain"
)

type GormAffiliateRepository struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewGormAffiliateRepository(db *gorm.DB, logger *zap.Logger) *GormAffiliateRepository {
	return &GormAffiliateRepository{
		db:     db,
		logger: logger,
	}
}

type enrollmentRequestModel struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID     int64
	Status     string
	Reason     string
	ReviewNotes string
	ReviewedBy string
	ReviewedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (enrollmentRequestModel) TableName() string { return "affiliate_enrollment_requests" }

type commissionPlanModel struct {
	ID                       uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name                     string
	CommissionType           string
	CommissionRate           decimal.Decimal
	HoldPeriodDays           int
	MinPayoutAmount          decimal.Decimal
	ApprovalMode             string
	PayoutSchedule           string
	NegativeCarryoverEnabled bool
	IsDefault                bool
	IsActive                 bool
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

func (commissionPlanModel) TableName() string { return "affiliate_commission_plans" }

type affiliateProfileModel struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID           int64
	Status           string
	AffiliateCode    string
	CommissionPlanID uuid.UUID
	CommissionRate   decimal.Decimal
	HoldPeriodDays   int
	MinPayoutAmount  decimal.Decimal
	Currency         string
	KYCRequired      bool
	ApprovalMode     string
	PayoutSchedule   string
	ApprovedBy       string
	ApprovedAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (affiliateProfileModel) TableName() string { return "affiliate_profiles" }

type affiliateLinkModel struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	AffiliateID  uuid.UUID
	CampaignName string
	LandingPage  string
	ReferralCode string
	ReferralURL  string
	UTMSource    string
	UTMMedium    string
	UTMCampaign  string
	IsActive     bool
	CreatedAt    time.Time
}

func (affiliateLinkModel) TableName() string { return "affiliate_links" }

type affiliateClickModel struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	AffiliateID       uuid.UUID
	LinkID            *uuid.UUID
	ClickID           string
	IPHash            string
	UserAgentHash     string
	DeviceFingerprint string
	CountryCode       string
	LandingPage       string
	CreatedAt         time.Time
}

func (affiliateClickModel) TableName() string { return "affiliate_clicks" }

type affiliateAttributionModel struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	AffiliateID    uuid.UUID
	ReferredUserID int64
	ClickID        string
	AttributedAt   time.Time
	CreatedAt      time.Time
}

func (affiliateAttributionModel) TableName() string { return "affiliate_attributions" }

type affiliateEarningModel struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey"`
	AffiliateID      uuid.UUID
	ReferredUserID   int64
	SourceType       string
	SourceID         string
	PeriodStart      time.Time
	PeriodEnd        time.Time
	GGRAmount        decimal.Decimal
	NGRAmount        decimal.Decimal
	CommissionRate   decimal.Decimal
	CommissionAmount decimal.Decimal
	Status           string
	HoldUntil        time.Time
	IdempotencyKey   string
	CreatedAt        time.Time
}

func (affiliateEarningModel) TableName() string { return "affiliate_earnings" }

type affiliatePayoutMethodModel struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey"`
	AffiliateID      uuid.UUID
	MethodType       string
	DisplayName      string
	DetailsMasked    string
	DetailsEncrypted string
	IsDefault        bool
	IsVerified       bool
	CreatedAt        time.Time
}

func (affiliatePayoutMethodModel) TableName() string { return "affiliate_payout_methods" }

type affiliatePayoutModel struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	AffiliateID       uuid.UUID
	Amount            decimal.Decimal
	Currency          string
	MethodID          uuid.UUID
	IdempotencyKey    string
	Status            string
	RequestedAt       time.Time
	ApprovedBy        string
	ApprovedAt        *time.Time
	ProviderReference string
	RejectionReason   string
	CreatedAt         time.Time
}

func (affiliatePayoutModel) TableName() string { return "affiliate_payouts" }

type affiliateFraudFlagModel struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	AffiliateID    uuid.UUID
	ReferredUserID int64
	FlagType       string
	Severity       string
	Status         string
	Details        map[string]string `gorm:"serializer:json"`
	CreatedAt      time.Time
	ResolvedAt     *time.Time
	ResolvedBy     string
}

func (affiliateFraudFlagModel) TableName() string { return "affiliate_fraud_flags" }

type affiliateLedgerAccountModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	AffiliateID uuid.UUID
	AccountType string
	Currency    string
	Balance     decimal.Decimal
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (affiliateLedgerAccountModel) TableName() string { return "affiliate_ledger_accounts" }

type affiliateOutboxModel struct {
	ID            int64 `gorm:"primaryKey"`
	AggregateType string
	AggregateID   string
	Topic         string
	EventKey      string
	Payload       map[string]any `gorm:"serializer:json"`
	Headers       map[string]any `gorm:"serializer:json"`
	CreatedAt     time.Time
	PublishedAt   *time.Time
	RetryCount    int
}

func (affiliateOutboxModel) TableName() string { return "affiliate_outbox" }

func (r *GormAffiliateRepository) GetEnrollmentByUserID(ctx context.Context, userID int64) (*domain.AffiliateEnrollmentRequest, error) {
	var model enrollmentRequestModel
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get enrollment by user id: %w", err)
	}
	return enrollmentModelToDomain(model), nil
}

func (r *GormAffiliateRepository) CreateEnrollment(ctx context.Context, req *domain.AffiliateEnrollmentRequest) error {
	model := enrollmentDomainToModel(req)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model).Error; err != nil {
			return fmt.Errorf("create enrollment: %w", err)
		}
		return r.appendOutboxTx(tx, "affiliate_enrollment", req.ID.String(), "affiliate.enrollment.requested", fmt.Sprintf("%d", req.UserID), map[string]any{
			"enrollment_request_id": req.ID.String(),
			"user_id":               req.UserID,
			"status":                req.Status,
			"reason":                req.Reason,
		})
	})
}

func (r *GormAffiliateRepository) UpdateEnrollmentStatus(ctx context.Context, userID int64, status domain.EnrollmentStatus, reviewNotes string, reviewedBy string, reviewedAt time.Time) error {
	updates := map[string]any{
		"status":       string(status),
		"review_notes": reviewNotes,
		"reviewed_by":  reviewedBy,
		"reviewed_at":  reviewedAt,
		"updated_at":   time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Model(&enrollmentRequestModel{}).Where("user_id = ?", userID).Updates(updates).Error; err != nil {
		return fmt.Errorf("update enrollment status: %w", err)
	}
	return nil
}

func (r *GormAffiliateRepository) GetProfileByUserID(ctx context.Context, userID int64) (*domain.AffiliateProfile, error) {
	var model affiliateProfileModel
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get profile by user id: %w", err)
	}
	return profileModelToDomain(model), nil
}

func (r *GormAffiliateRepository) GetProfileByID(ctx context.Context, affiliateID uuid.UUID) (*domain.AffiliateProfile, error) {
	var model affiliateProfileModel
	err := r.db.WithContext(ctx).Where("id = ?", affiliateID).First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get profile by id: %w", err)
	}
	return profileModelToDomain(model), nil
}

func (r *GormAffiliateRepository) GetProfileByAffiliateCode(ctx context.Context, affiliateCode string) (*domain.AffiliateProfile, error) {
	var model affiliateProfileModel
	err := r.db.WithContext(ctx).Where("affiliate_code = ?", affiliateCode).First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get profile by affiliate code: %w", err)
	}
	return profileModelToDomain(model), nil
}

func (r *GormAffiliateRepository) CreateProfile(ctx context.Context, profile *domain.AffiliateProfile) error {
	model := profileDomainToModel(profile)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model).Error; err != nil {
			return fmt.Errorf("create profile: %w", err)
		}
		if err := r.createLedgerAccountsTx(tx, profile.ID, profile.Currency); err != nil {
			return err
		}
		return r.appendOutboxTx(tx, "affiliate_profile", profile.ID.String(), "affiliate.enrollment.approved", profile.ID.String(), map[string]any{
			"affiliate_id":   profile.ID.String(),
			"user_id":        profile.UserID,
			"affiliate_code": profile.AffiliateCode,
			"approved_by":    profile.ApprovedBy,
		})
	})
}

func (r *GormAffiliateRepository) UpdateProfileStatus(ctx context.Context, affiliateID uuid.UUID, from domain.AffiliateStatus, to domain.AffiliateStatus) error {
	res := r.db.WithContext(ctx).Model(&affiliateProfileModel{}).
		Where("id = ? AND status = ?", affiliateID, string(from)).
		Updates(map[string]any{"status": string(to), "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return fmt.Errorf("update profile status: %w", res.Error)
	}
	return nil
}

func (r *GormAffiliateRepository) GetCommissionPlanByID(ctx context.Context, planID uuid.UUID) (*domain.AffiliateCommissionPlan, error) {
	var model commissionPlanModel
	err := r.db.WithContext(ctx).Where("id = ?", planID).First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get commission plan by id: %w", err)
	}
	return commissionPlanModelToDomain(model), nil
}

func (r *GormAffiliateRepository) GetDefaultCommissionPlan(ctx context.Context) (*domain.AffiliateCommissionPlan, error) {
	var model commissionPlanModel
	err := r.db.WithContext(ctx).Where("is_default = ? AND is_active = ?", true, true).First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get default commission plan: %w", err)
	}
	return commissionPlanModelToDomain(model), nil
}

func (r *GormAffiliateRepository) CreateLink(ctx context.Context, link *domain.AffiliateLink) error {
	model := affiliateLinkDomainToModel(link)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return fmt.Errorf("create link: %w", err)
	}
	return nil
}

func (r *GormAffiliateRepository) ListLinksByAffiliateID(ctx context.Context, affiliateID uuid.UUID) ([]domain.AffiliateLink, error) {
	var models []affiliateLinkModel
	if err := r.db.WithContext(ctx).Where("affiliate_id = ?", affiliateID).Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list links by affiliate id: %w", err)
	}
	result := make([]domain.AffiliateLink, 0, len(models))
	for _, model := range models {
		result = append(result, *affiliateLinkModelToDomain(model))
	}
	return result, nil
}

func (r *GormAffiliateRepository) CreateClick(ctx context.Context, click *domain.AffiliateClick) error {
	model := affiliateClickDomainToModel(click)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model).Error; err != nil {
			return fmt.Errorf("create click: %w", err)
		}
		return r.appendOutboxTx(tx, "affiliate_click", click.ID.String(), "affiliate.click.tracked", click.AffiliateID.String(), map[string]any{
			"click_id":      click.ClickID,
			"affiliate_id":  click.AffiliateID.String(),
			"country_code":  click.CountryCode,
			"landing_page":  click.LandingPage,
		})
	})
}

func (r *GormAffiliateRepository) GetAttributionByReferredUserID(ctx context.Context, referredUserID int64) (*domain.AffiliateAttribution, error) {
	var model affiliateAttributionModel
	err := r.db.WithContext(ctx).Where("referred_user_id = ?", referredUserID).First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get attribution by referred user id: %w", err)
	}
	return attributionModelToDomain(model), nil
}

func (r *GormAffiliateRepository) CreateAttribution(ctx context.Context, attribution *domain.AffiliateAttribution) error {
	model := attributionDomainToModel(attribution)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model).Error; err != nil {
			return fmt.Errorf("create attribution: %w", err)
		}
		return r.appendOutboxTx(tx, "affiliate_attribution", attribution.ID.String(), "affiliate.attribution.created", attribution.AffiliateID.String(), map[string]any{
			"attribution_id":  attribution.ID.String(),
			"affiliate_id":    attribution.AffiliateID.String(),
			"referred_user_id": attribution.ReferredUserID,
			"click_id":        attribution.ClickID,
		})
	})
}

func (r *GormAffiliateRepository) GetDashboard(ctx context.Context, affiliateID uuid.UUID) (*domain.AffiliateDashboard, error) {
	dashboard := &domain.AffiliateDashboard{
		Currency: "USD",
	}
	type sumRow struct {
		Amount decimal.Decimal
	}
	type countRow struct {
		Count int64
	}
	var row sumRow
	monthStart := time.Now().UTC().AddDate(0, 0, -time.Now().UTC().Day()+1)
	todayStart := time.Now().UTC().Truncate(24 * time.Hour)
	if err := r.db.WithContext(ctx).Model(&affiliateEarningModel{}).Select("COALESCE(SUM(commission_amount),0) AS amount").
		Where("affiliate_id = ? AND created_at >= ?", affiliateID, todayStart).Scan(&row).Error; err != nil {
		return nil, fmt.Errorf("dashboard earnings today: %w", err)
	}
	dashboard.EarningsToday = row.Amount
	if err := r.db.WithContext(ctx).Model(&affiliateEarningModel{}).Select("COALESCE(SUM(commission_amount),0) AS amount").
		Where("affiliate_id = ? AND created_at >= ?", affiliateID, monthStart).Scan(&row).Error; err != nil {
		return nil, fmt.Errorf("dashboard earnings month: %w", err)
	}
	dashboard.EarningsThisMonth = row.Amount
	if err := r.db.WithContext(ctx).Model(&affiliateEarningModel{}).Select("COALESCE(SUM(commission_amount),0) AS amount").
		Where("affiliate_id = ? AND status IN ?", affiliateID, []string{string(domain.EarningStatusAccrued), string(domain.EarningStatusPending)}).Scan(&row).Error; err != nil {
		return nil, fmt.Errorf("dashboard pending amount: %w", err)
	}
	dashboard.PendingAmount = row.Amount
	available, err := r.GetAvailableBalance(ctx, affiliateID)
	if err != nil {
		return nil, err
	}
	dashboard.AvailableAmount = available
	if err := r.db.WithContext(ctx).Model(&affiliatePayoutModel{}).Select("COALESCE(SUM(amount),0) AS amount").
		Where("affiliate_id = ? AND status = ?", affiliateID, string(domain.PayoutStatusPaid)).Scan(&row).Error; err != nil {
		return nil, fmt.Errorf("dashboard paid amount: %w", err)
	}
	dashboard.PaidAmount = row.Amount
	var clicks countRow
	if err := r.db.WithContext(ctx).Model(&affiliateClickModel{}).Select("COUNT(*) AS count").Where("affiliate_id = ?", affiliateID).Scan(&clicks).Error; err != nil {
		return nil, fmt.Errorf("dashboard clicks: %w", err)
	}
	dashboard.Clicks = clicks.Count
	var registrations countRow
	if err := r.db.WithContext(ctx).Model(&affiliateAttributionModel{}).Select("COUNT(*) AS count").Where("affiliate_id = ?", affiliateID).Scan(&registrations).Error; err != nil {
		return nil, fmt.Errorf("dashboard registrations: %w", err)
	}
	dashboard.Registrations = registrations.Count
	dashboard.FTDCount = registrations.Count
	dashboard.ActivePlayers = registrations.Count
	dashboard.GGRAmount = decimal.Zero
	dashboard.NGRAmount = decimal.Zero
	dashboard.CommissionAmount = dashboard.EarningsThisMonth
	dashboard.CommissionReversedAmount = decimal.Zero
	dashboard.CommissionAdjustedAmount = decimal.Zero
	return dashboard, nil
}

func (r *GormAffiliateRepository) ListEarnings(ctx context.Context, affiliateID uuid.UUID, status domain.EarningStatus, limit int) ([]domain.AffiliateEarning, error) {
	query := r.db.WithContext(ctx).Model(&affiliateEarningModel{}).Where("affiliate_id = ?", affiliateID).Order("created_at DESC")
	if status != "" {
		query = query.Where("status = ?", string(status))
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	var models []affiliateEarningModel
	if err := query.Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list earnings: %w", err)
	}
	result := make([]domain.AffiliateEarning, 0, len(models))
	for _, model := range models {
		result = append(result, *earningModelToDomain(model))
	}
	return result, nil
}

func (r *GormAffiliateRepository) GetAvailableBalance(ctx context.Context, affiliateID uuid.UUID) (decimal.Decimal, error) {
	var availableRow struct {
		Amount decimal.Decimal
	}
	err := r.db.WithContext(ctx).Model(&affiliateEarningModel{}).
		Select("COALESCE(SUM(commission_amount),0) AS amount").
		Where("affiliate_id = ? AND status = ?", affiliateID, string(domain.EarningStatusAvailable)).
		Scan(&availableRow).Error
	if err != nil {
		return decimal.Zero, fmt.Errorf("get available balance: %w", err)
	}

	var reservedRow struct {
		Amount decimal.Decimal
	}
	err = r.db.WithContext(ctx).Model(&affiliatePayoutModel{}).
		Select("COALESCE(SUM(amount),0) AS amount").
		Where("affiliate_id = ? AND status IN ?", affiliateID, []string{
			string(domain.PayoutStatusRequested),
			string(domain.PayoutStatusReviewing),
			string(domain.PayoutStatusApproved),
			string(domain.PayoutStatusProcessing),
			string(domain.PayoutStatusPaid),
		}).
		Scan(&reservedRow).Error
	if err != nil {
		return decimal.Zero, fmt.Errorf("get reserved payout balance: %w", err)
	}

	effective := availableRow.Amount.Sub(reservedRow.Amount)
	if effective.IsNegative() {
		return decimal.Zero, nil
	}
	return effective, nil
}

func (r *GormAffiliateRepository) CreateEarning(ctx context.Context, earning *domain.AffiliateEarning) error {
	model := earningDomainToModel(earning)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model).Error; err != nil {
			return fmt.Errorf("create earning: %w", err)
		}
		return r.appendOutboxTx(tx, "affiliate_earning", earning.ID.String(), "affiliate.commission.accrued", earning.AffiliateID.String(), map[string]any{
			"earning_id":         earning.ID.String(),
			"affiliate_id":       earning.AffiliateID.String(),
			"referred_user_id":   earning.ReferredUserID,
			"commission_amount":  earning.CommissionAmount.String(),
			"commission_rate":    earning.CommissionRate.String(),
			"status":             earning.Status,
			"hold_until":         earning.HoldUntil,
			"idempotency_key":    earning.IdempotencyKey,
		})
	})
}

func (r *GormAffiliateRepository) CreatePayoutMethod(ctx context.Context, method *domain.AffiliatePayoutMethod) error {
	model := affiliatePayoutMethodDomainToModel(method)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return fmt.Errorf("create payout method: %w", err)
	}
	return nil
}

func (r *GormAffiliateRepository) ListPayoutMethods(ctx context.Context, affiliateID uuid.UUID) ([]domain.AffiliatePayoutMethod, error) {
	var models []affiliatePayoutMethodModel
	if err := r.db.WithContext(ctx).Where("affiliate_id = ?", affiliateID).Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list payout methods: %w", err)
	}
	result := make([]domain.AffiliatePayoutMethod, 0, len(models))
	for _, model := range models {
		result = append(result, *payoutMethodModelToDomain(model))
	}
	return result, nil
}

func (r *GormAffiliateRepository) CreatePayout(ctx context.Context, payout *domain.AffiliatePayout) error {
	model := payoutDomainToModel(payout)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model).Error; err != nil {
			return fmt.Errorf("create payout: %w", err)
		}
		return r.appendOutboxTx(tx, "affiliate_payout", payout.ID.String(), "affiliate.payout.requested", payout.AffiliateID.String(), map[string]any{
			"payout_id":      payout.ID.String(),
			"affiliate_id":   payout.AffiliateID.String(),
			"amount":         payout.Amount.String(),
			"currency":       payout.Currency,
			"method_id":      payout.MethodID.String(),
			"idempotency_key": payout.IdempotencyKey,
		})
	})
}

func (r *GormAffiliateRepository) GetPayoutByID(ctx context.Context, payoutID uuid.UUID) (*domain.AffiliatePayout, error) {
	var model affiliatePayoutModel
	err := r.db.WithContext(ctx).Where("id = ?", payoutID).First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get payout by id: %w", err)
	}
	return payoutModelToDomain(model), nil
}

func (r *GormAffiliateRepository) ListPayoutsByAffiliateID(ctx context.Context, affiliateID uuid.UUID, status domain.PayoutStatus, limit, offset int) ([]domain.AffiliatePayout, int64, error) {
	query := r.db.WithContext(ctx).Model(&affiliatePayoutModel{}).Where("affiliate_id = ?", affiliateID)
	if status != "" {
		query = query.Where("status = ?", string(status))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count affiliate payouts: %w", err)
	}

	var models []affiliatePayoutModel
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&models).Error; err != nil {
		return nil, 0, fmt.Errorf("list affiliate payouts: %w", err)
	}

	result := make([]domain.AffiliatePayout, 0, len(models))
	for _, model := range models {
		result = append(result, *payoutModelToDomain(model))
	}
	return result, total, nil
}

func (r *GormAffiliateRepository) UpdatePayout(ctx context.Context, payout *domain.AffiliatePayout) error {
	model := payoutDomainToModel(payout)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&affiliatePayoutModel{}).
			Where("id = ?", payout.ID).
			Updates(map[string]any{
				"status":             model.Status,
				"approved_by":        model.ApprovedBy,
				"approved_at":        model.ApprovedAt,
				"provider_reference": model.ProviderReference,
				"rejection_reason":   model.RejectionReason,
			}).Error; err != nil {
			return fmt.Errorf("update payout: %w", err)
		}

		topic := "affiliate.payout.updated"
		if payout.Status == domain.PayoutStatusPaid {
			topic = "affiliate.payout.paid"
		}
		if payout.Status == domain.PayoutStatusRejected {
			topic = "affiliate.payout.rejected"
		}

		return r.appendOutboxTx(tx, "affiliate_payout", payout.ID.String(), topic, payout.AffiliateID.String(), map[string]any{
			"payout_id":           payout.ID.String(),
			"affiliate_id":        payout.AffiliateID.String(),
			"status":              payout.Status,
			"approved_by":         payout.ApprovedBy,
			"provider_reference":  payout.ProviderReference,
			"rejection_reason":    payout.RejectionReason,
		})
	})
}

func (r *GormAffiliateRepository) CreateFraudFlag(ctx context.Context, flag *domain.AffiliateFraudFlag) error {
	model := fraudFlagDomainToModel(flag)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model).Error; err != nil {
			return fmt.Errorf("create fraud flag: %w", err)
		}
		return r.appendOutboxTx(tx, "affiliate_fraud_flag", flag.ID.String(), "affiliate.fraud.flagged", flag.AffiliateID.String(), map[string]any{
			"fraud_flag_id":     flag.ID.String(),
			"affiliate_id":      flag.AffiliateID.String(),
			"referred_user_id":  flag.ReferredUserID,
			"flag_type":         flag.FlagType,
			"severity":          flag.Severity,
			"status":            flag.Status,
		})
	})
}

func (r *GormAffiliateRepository) GetOpenFraudFlags(ctx context.Context, affiliateID uuid.UUID) ([]domain.AffiliateFraudFlag, error) {
	var models []affiliateFraudFlagModel
	if err := r.db.WithContext(ctx).
		Where("affiliate_id = ? AND status IN ?", affiliateID, []string{
			string(domain.FraudFlagStatusOpen),
			string(domain.FraudFlagStatusInReview),
		}).
		Order("created_at DESC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("get open fraud flags: %w", err)
	}

	result := make([]domain.AffiliateFraudFlag, 0, len(models))
	for _, model := range models {
		result = append(result, *fraudFlagModelToDomain(model))
	}
	return result, nil
}

func (r *GormAffiliateRepository) GetPayoutMethodByID(ctx context.Context, methodID uuid.UUID) (*domain.AffiliatePayoutMethod, error) {
	var model affiliatePayoutMethodModel
	err := r.db.WithContext(ctx).Where("id = ?", methodID).First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get payout method by id: %w", err)
	}
	return payoutMethodModelToDomain(model), nil
}

func (r *GormAffiliateRepository) UpdatePayoutMethod(ctx context.Context, method *domain.AffiliatePayoutMethod) error {
	updates := map[string]any{
		"method_type":    string(method.MethodType),
		"display_name":   method.DisplayName,
		"details_masked": method.DetailsMasked,
		"is_default":     method.IsDefault,
		"is_verified":    method.IsVerified,
	}
	if err := r.db.WithContext(ctx).
		Model(&affiliatePayoutMethodModel{}).
		Where("id = ? AND affiliate_id = ?", method.ID, method.AffiliateID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("update payout method: %w", err)
	}
	return nil
}

func (r *GormAffiliateRepository) ReleaseEligibleEarnings(ctx context.Context, now time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Model(&affiliateEarningModel{}).
		Where("status IN ? AND hold_until <= ?", []string{string(domain.EarningStatusAccrued), string(domain.EarningStatusPending)}, now).
		Updates(map[string]any{"status": string(domain.EarningStatusAvailable)})
	if res.Error != nil {
		return 0, fmt.Errorf("release eligible earnings: %w", res.Error)
	}
	return res.RowsAffected, nil
}

func (r *GormAffiliateRepository) ListPendingOutboxEvents(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	if limit <= 0 {
		limit = 100
	}

	var models []affiliateOutboxModel
	if err := r.db.WithContext(ctx).
		Where("published_at IS NULL").
		Order("created_at ASC").
		Limit(limit).
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list pending outbox events: %w", err)
	}

	events := make([]domain.OutboxEvent, 0, len(models))
	for _, model := range models {
		events = append(events, outboxModelToDomain(model))
	}
	return events, nil
}

func (r *GormAffiliateRepository) MarkOutboxEventPublished(ctx context.Context, eventID int64) error {
	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).
		Model(&affiliateOutboxModel{}).
		Where("id = ?", eventID).
		Updates(map[string]any{
			"published_at": now,
		}).Error; err != nil {
		return fmt.Errorf("mark outbox event published: %w", err)
	}
	return nil
}

func (r *GormAffiliateRepository) IncrementOutboxEventRetry(ctx context.Context, eventID int64) error {
	if err := r.db.WithContext(ctx).Exec(
		"UPDATE affiliate_outbox SET retry_count = retry_count + 1 WHERE id = ?",
		eventID,
	).Error; err != nil {
		return fmt.Errorf("increment outbox retry: %w", err)
	}
	return nil
}

func (r *GormAffiliateRepository) createLedgerAccountsTx(tx *gorm.DB, affiliateID uuid.UUID, currency string) error {
	accountTypes := []string{"pending", "available", "paid", "reversed", "adjusted"}
	now := time.Now().UTC()
	for _, accountType := range accountTypes {
		model := affiliateLedgerAccountModel{
			ID:          uuid.New(),
			AffiliateID: affiliateID,
			AccountType: accountType,
			Currency:    currency,
			Balance:     decimal.Zero,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := tx.Create(&model).Error; err != nil {
			return fmt.Errorf("create ledger account %s: %w", accountType, err)
		}
	}
	return nil
}

func (r *GormAffiliateRepository) appendOutboxTx(tx *gorm.DB, aggregateType string, aggregateID string, topic string, eventKey string, payload map[string]any) error {
	event := affiliateOutboxModel{
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		Topic:         topic,
		EventKey:      eventKey,
		Payload:       payload,
		Headers: map[string]any{
			"service": "affiliate-service",
		},
		CreatedAt: time.Now().UTC(),
	}
	if err := tx.Create(&event).Error; err != nil {
		return fmt.Errorf("append outbox event: %w", err)
	}
	return nil
}

func enrollmentModelToDomain(model enrollmentRequestModel) *domain.AffiliateEnrollmentRequest {
	return &domain.AffiliateEnrollmentRequest{
		ID:         model.ID,
		UserID:     model.UserID,
		Status:     domain.EnrollmentStatus(model.Status),
		Reason:     model.Reason,
		ReviewNotes: model.ReviewNotes,
		ReviewedBy: model.ReviewedBy,
		ReviewedAt: model.ReviewedAt,
		CreatedAt:  model.CreatedAt,
		UpdatedAt:  model.UpdatedAt,
	}
}

func enrollmentDomainToModel(req *domain.AffiliateEnrollmentRequest) enrollmentRequestModel {
	return enrollmentRequestModel{
		ID:          req.ID,
		UserID:      req.UserID,
		Status:      string(req.Status),
		Reason:      req.Reason,
		ReviewNotes: req.ReviewNotes,
		ReviewedBy:  req.ReviewedBy,
		ReviewedAt:  req.ReviewedAt,
		CreatedAt:   req.CreatedAt,
		UpdatedAt:   req.UpdatedAt,
	}
}

func commissionPlanModelToDomain(model commissionPlanModel) *domain.AffiliateCommissionPlan {
	return &domain.AffiliateCommissionPlan{
		ID:                       model.ID,
		Name:                     model.Name,
		CommissionType:           model.CommissionType,
		CommissionRate:           model.CommissionRate,
		HoldPeriodDays:           model.HoldPeriodDays,
		MinPayoutAmount:          model.MinPayoutAmount,
		ApprovalMode:             domain.ApprovalMode(model.ApprovalMode),
		PayoutSchedule:           domain.PayoutSchedule(model.PayoutSchedule),
		NegativeCarryoverEnabled: model.NegativeCarryoverEnabled,
		IsDefault:                model.IsDefault,
		IsActive:                 model.IsActive,
		CreatedAt:                model.CreatedAt,
		UpdatedAt:                model.UpdatedAt,
	}
}

func profileModelToDomain(model affiliateProfileModel) *domain.AffiliateProfile {
	return &domain.AffiliateProfile{
		ID:               model.ID,
		UserID:           model.UserID,
		Status:           domain.AffiliateStatus(model.Status),
		AffiliateCode:    model.AffiliateCode,
		CommissionPlanID: model.CommissionPlanID,
		CommissionRate:   model.CommissionRate,
		HoldPeriodDays:   model.HoldPeriodDays,
		MinPayoutAmount:  model.MinPayoutAmount,
		Currency:         model.Currency,
		KYCRequired:      model.KYCRequired,
		ApprovalMode:     domain.ApprovalMode(model.ApprovalMode),
		PayoutSchedule:   domain.PayoutSchedule(model.PayoutSchedule),
		ApprovedBy:       model.ApprovedBy,
		ApprovedAt:       model.ApprovedAt,
		CreatedAt:        model.CreatedAt,
		UpdatedAt:        model.UpdatedAt,
	}
}

func profileDomainToModel(profile *domain.AffiliateProfile) affiliateProfileModel {
	return affiliateProfileModel{
		ID:               profile.ID,
		UserID:           profile.UserID,
		Status:           string(profile.Status),
		AffiliateCode:    profile.AffiliateCode,
		CommissionPlanID: profile.CommissionPlanID,
		CommissionRate:   profile.CommissionRate,
		HoldPeriodDays:   profile.HoldPeriodDays,
		MinPayoutAmount:  profile.MinPayoutAmount,
		Currency:         profile.Currency,
		KYCRequired:      profile.KYCRequired,
		ApprovalMode:     string(profile.ApprovalMode),
		PayoutSchedule:   string(profile.PayoutSchedule),
		ApprovedBy:       profile.ApprovedBy,
		ApprovedAt:       profile.ApprovedAt,
		CreatedAt:        profile.CreatedAt,
		UpdatedAt:        profile.UpdatedAt,
	}
}

func affiliateLinkDomainToModel(link *domain.AffiliateLink) affiliateLinkModel {
	return affiliateLinkModel{
		ID:           link.ID,
		AffiliateID:  link.AffiliateID,
		CampaignName: link.CampaignName,
		LandingPage:  link.LandingPage,
		ReferralCode: link.ReferralCode,
		ReferralURL:  link.ReferralURL,
		UTMSource:    link.UTMSource,
		UTMMedium:    link.UTMMedium,
		UTMCampaign:  link.UTMCampaign,
		IsActive:     link.IsActive,
		CreatedAt:    link.CreatedAt,
	}
}

func affiliateLinkModelToDomain(model affiliateLinkModel) *domain.AffiliateLink {
	return &domain.AffiliateLink{
		ID:           model.ID,
		AffiliateID:  model.AffiliateID,
		CampaignName: model.CampaignName,
		LandingPage:  model.LandingPage,
		ReferralCode: model.ReferralCode,
		ReferralURL:  model.ReferralURL,
		UTMSource:    model.UTMSource,
		UTMMedium:    model.UTMMedium,
		UTMCampaign:  model.UTMCampaign,
		IsActive:     model.IsActive,
		CreatedAt:    model.CreatedAt,
	}
}

func earningDomainToModel(earning *domain.AffiliateEarning) affiliateEarningModel {
	return affiliateEarningModel{
		ID:               earning.ID,
		AffiliateID:      earning.AffiliateID,
		ReferredUserID:   earning.ReferredUserID,
		SourceType:       earning.SourceType,
		SourceID:         earning.SourceID,
		PeriodStart:      earning.PeriodStart,
		PeriodEnd:        earning.PeriodEnd,
		GGRAmount:        earning.GGRAmount,
		NGRAmount:        earning.NGRAmount,
		CommissionRate:   earning.CommissionRate,
		CommissionAmount: earning.CommissionAmount,
		Status:           string(earning.Status),
		HoldUntil:        earning.HoldUntil,
		IdempotencyKey:   earning.IdempotencyKey,
		CreatedAt:        earning.CreatedAt,
	}
}

func affiliateClickDomainToModel(click *domain.AffiliateClick) affiliateClickModel {
	return affiliateClickModel{
		ID:                click.ID,
		AffiliateID:       click.AffiliateID,
		LinkID:            click.LinkID,
		ClickID:           click.ClickID,
		IPHash:            click.IPHash,
		UserAgentHash:     click.UserAgentHash,
		DeviceFingerprint: click.DeviceFingerprint,
		CountryCode:       click.CountryCode,
		LandingPage:       click.LandingPage,
		CreatedAt:         click.CreatedAt,
	}
}

func attributionModelToDomain(model affiliateAttributionModel) *domain.AffiliateAttribution {
	return &domain.AffiliateAttribution{
		ID:             model.ID,
		AffiliateID:    model.AffiliateID,
		ReferredUserID: model.ReferredUserID,
		ClickID:        model.ClickID,
		AttributedAt:   model.AttributedAt,
		CreatedAt:      model.CreatedAt,
	}
}

func attributionDomainToModel(attribution *domain.AffiliateAttribution) affiliateAttributionModel {
	return affiliateAttributionModel{
		ID:             attribution.ID,
		AffiliateID:    attribution.AffiliateID,
		ReferredUserID: attribution.ReferredUserID,
		ClickID:        attribution.ClickID,
		AttributedAt:   attribution.AttributedAt,
		CreatedAt:      attribution.CreatedAt,
	}
}

func earningModelToDomain(model affiliateEarningModel) *domain.AffiliateEarning {
	return &domain.AffiliateEarning{
		ID:               model.ID,
		AffiliateID:      model.AffiliateID,
		ReferredUserID:   model.ReferredUserID,
		SourceType:       model.SourceType,
		SourceID:         model.SourceID,
		PeriodStart:      model.PeriodStart,
		PeriodEnd:        model.PeriodEnd,
		GGRAmount:        model.GGRAmount,
		NGRAmount:        model.NGRAmount,
		CommissionRate:   model.CommissionRate,
		CommissionAmount: model.CommissionAmount,
		Status:           domain.EarningStatus(model.Status),
		HoldUntil:        model.HoldUntil,
		IdempotencyKey:   model.IdempotencyKey,
		CreatedAt:        model.CreatedAt,
	}
}

func affiliatePayoutMethodDomainToModel(method *domain.AffiliatePayoutMethod) affiliatePayoutMethodModel {
	return affiliatePayoutMethodModel{
		ID:               method.ID,
		AffiliateID:      method.AffiliateID,
		MethodType:       string(method.MethodType),
		DisplayName:      method.DisplayName,
		DetailsMasked:    method.DetailsMasked,
		DetailsEncrypted: method.DetailsMasked,
		IsDefault:        method.IsDefault,
		IsVerified:       method.IsVerified,
		CreatedAt:        method.CreatedAt,
	}
}

func payoutMethodModelToDomain(model affiliatePayoutMethodModel) *domain.AffiliatePayoutMethod {
	return &domain.AffiliatePayoutMethod{
		ID:            model.ID,
		AffiliateID:   model.AffiliateID,
		MethodType:    domain.PayoutMethodType(model.MethodType),
		DisplayName:   model.DisplayName,
		DetailsMasked: model.DetailsMasked,
		IsDefault:     model.IsDefault,
		IsVerified:    model.IsVerified,
		CreatedAt:     model.CreatedAt,
	}
}

func payoutDomainToModel(payout *domain.AffiliatePayout) affiliatePayoutModel {
	return affiliatePayoutModel{
		ID:                payout.ID,
		AffiliateID:       payout.AffiliateID,
		Amount:            payout.Amount,
		Currency:          payout.Currency,
		MethodID:          payout.MethodID,
		IdempotencyKey:    payout.IdempotencyKey,
		Status:            string(payout.Status),
		RequestedAt:       payout.RequestedAt,
		ApprovedBy:        payout.ApprovedBy,
		ApprovedAt:        payout.ApprovedAt,
		ProviderReference: payout.ProviderReference,
		RejectionReason:   payout.RejectionReason,
		CreatedAt:         payout.CreatedAt,
	}
}

func payoutModelToDomain(model affiliatePayoutModel) *domain.AffiliatePayout {
	return &domain.AffiliatePayout{
		ID:                model.ID,
		AffiliateID:       model.AffiliateID,
		MethodID:          model.MethodID,
		Amount:            model.Amount,
		Currency:          model.Currency,
		Status:            domain.PayoutStatus(model.Status),
		IdempotencyKey:    model.IdempotencyKey,
		ApprovedBy:        model.ApprovedBy,
		ApprovedAt:        model.ApprovedAt,
		ProviderReference: model.ProviderReference,
		RejectionReason:   model.RejectionReason,
		RequestedAt:       model.RequestedAt,
		CreatedAt:         model.CreatedAt,
	}
}

func fraudFlagDomainToModel(flag *domain.AffiliateFraudFlag) affiliateFraudFlagModel {
	return affiliateFraudFlagModel{
		ID:             flag.ID,
		AffiliateID:    flag.AffiliateID,
		ReferredUserID: flag.ReferredUserID,
		FlagType:       flag.FlagType,
		Severity:       string(flag.Severity),
		Status:         string(flag.Status),
		Details:        flag.Details,
		CreatedAt:      flag.CreatedAt,
		ResolvedAt:     flag.ResolvedAt,
		ResolvedBy:     flag.ResolvedBy,
	}
}

func fraudFlagModelToDomain(model affiliateFraudFlagModel) *domain.AffiliateFraudFlag {
	return &domain.AffiliateFraudFlag{
		ID:             model.ID,
		AffiliateID:    model.AffiliateID,
		ReferredUserID: model.ReferredUserID,
		FlagType:       model.FlagType,
		Severity:       domain.FraudSeverity(model.Severity),
		Status:         domain.FraudFlagStatus(model.Status),
		Details:        model.Details,
		CreatedAt:      model.CreatedAt,
		ResolvedAt:     model.ResolvedAt,
		ResolvedBy:     model.ResolvedBy,
	}
}

func outboxModelToDomain(model affiliateOutboxModel) domain.OutboxEvent {
	return domain.OutboxEvent{
		ID:            model.ID,
		AggregateType: model.AggregateType,
		AggregateID:   model.AggregateID,
		Topic:         model.Topic,
		EventKey:      model.EventKey,
		Payload:       model.Payload,
		Headers:       model.Headers,
		CreatedAt:     model.CreatedAt,
		PublishedAt:   model.PublishedAt,
		RetryCount:    model.RetryCount,
	}
}

// ============ Admin operations ============

type affiliateAdjustmentModel struct {
	ID             uuid.UUID       `gorm:"type:uuid;primaryKey"`
	AffiliateID    uuid.UUID
	AdjustmentType string
	Amount         decimal.Decimal
	Currency       string
	Reason         string
	CreatedBy      string
	CreatedAt      time.Time
}

func (affiliateAdjustmentModel) TableName() string { return "affiliate_adjustments" }

func (r *GormAffiliateRepository) ListProfiles(ctx context.Context, status domain.AffiliateStatus, limit, offset int) ([]domain.AffiliateProfile, int64, error) {
	query := r.db.WithContext(ctx).Model(&affiliateProfileModel{})

	if status != "" {
		query = query.Where("status = ?", string(status))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count profiles: %w", err)
	}

	var models []affiliateProfileModel
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&models).Error; err != nil {
		return nil, 0, fmt.Errorf("list profiles: %w", err)
	}

	result := make([]domain.AffiliateProfile, 0, len(models))
	for _, model := range models {
		result = append(result, *profileModelToDomain(model))
	}
	return result, total, nil
}

func (r *GormAffiliateRepository) UpdateCommissionRate(ctx context.Context, affiliateID uuid.UUID, rate decimal.Decimal) error {
	res := r.db.WithContext(ctx).Model(&affiliateProfileModel{}).
		Where("id = ?", affiliateID).
		Updates(map[string]any{"commission_rate": rate, "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return fmt.Errorf("update commission rate: %w", res.Error)
	}
	return nil
}

func (r *GormAffiliateRepository) CreateAdjustment(ctx context.Context, adj *domain.AffiliateAdjustment) error {
	model := affiliateAdjustmentModel{
		ID:             adj.ID,
		AffiliateID:    adj.AffiliateID,
		AdjustmentType: string(adj.AdjustmentType),
		Amount:         adj.Amount,
		Currency:       adj.Currency,
		Reason:         adj.Reason,
		CreatedBy:      adj.CreatedBy,
		CreatedAt:      adj.CreatedAt,
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model).Error; err != nil {
			return fmt.Errorf("create adjustment: %w", err)
		}
		return r.appendOutboxTx(tx, "affiliate_adjustment", adj.ID.String(), "affiliate.adjustment.created", adj.AffiliateID.String(), map[string]any{
			"adjustment_id":   adj.ID.String(),
			"affiliate_id":   adj.AffiliateID.String(),
			"adjustment_type": adj.AdjustmentType,
			"amount":          adj.Amount.String(),
			"reason":          adj.Reason,
			"created_by":      adj.CreatedBy,
		})
	})
}

func (r *GormAffiliateRepository) ListAllPayouts(ctx context.Context, status domain.PayoutStatus, limit, offset int) ([]domain.AffiliatePayout, int64, error) {
	query := r.db.WithContext(ctx).Model(&affiliatePayoutModel{})

	if status != "" {
		query = query.Where("status = ?", string(status))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count payouts: %w", err)
	}

	var models []affiliatePayoutModel
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&models).Error; err != nil {
		return nil, 0, fmt.Errorf("list payouts: %w", err)
	}

	result := make([]domain.AffiliatePayout, 0, len(models))
	for _, model := range models {
		result = append(result, *payoutModelToDomain(model))
	}
	return result, total, nil
}

func (r *GormAffiliateRepository) ListAllFraudFlags(ctx context.Context, status domain.FraudFlagStatus, limit, offset int) ([]domain.AffiliateFraudFlag, int64, error) {
	query := r.db.WithContext(ctx).Model(&affiliateFraudFlagModel{})

	if status != "" {
		query = query.Where("status = ?", string(status))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count fraud flags: %w", err)
	}

	var models []affiliateFraudFlagModel
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&models).Error; err != nil {
		return nil, 0, fmt.Errorf("list fraud flags: %w", err)
	}

	result := make([]domain.AffiliateFraudFlag, 0, len(models))
	for _, model := range models {
		result = append(result, *fraudFlagModelToDomain(model))
	}
	return result, total, nil
}
