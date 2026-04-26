package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type AffiliateStatus string

const (
	AffiliateStatusPendingReview AffiliateStatus = "pending_review"
	AffiliateStatusActive        AffiliateStatus = "active"
	AffiliateStatusSuspended     AffiliateStatus = "suspended"
	AffiliateStatusRejected      AffiliateStatus = "rejected"
	AffiliateStatusClosed        AffiliateStatus = "closed"
)

func (s AffiliateStatus) CanTransitionTo(target AffiliateStatus) bool {
	transitions := map[AffiliateStatus][]AffiliateStatus{
		AffiliateStatusPendingReview: {AffiliateStatusActive, AffiliateStatusRejected},
		AffiliateStatusActive:        {AffiliateStatusSuspended, AffiliateStatusClosed},
		AffiliateStatusSuspended:     {AffiliateStatusActive, AffiliateStatusClosed},
	}

	for _, allowed := range transitions[s] {
		if allowed == target {
			return true
		}
	}
	return false
}

type EnrollmentStatus string

const (
	EnrollmentStatusPendingReview EnrollmentStatus = "pending_review"
	EnrollmentStatusApproved      EnrollmentStatus = "approved"
	EnrollmentStatusRejected      EnrollmentStatus = "rejected"
)

type EarningStatus string

const (
	EarningStatusAccrued   EarningStatus = "accrued"
	EarningStatusPending   EarningStatus = "pending"
	EarningStatusAvailable EarningStatus = "available"
	EarningStatusPaid      EarningStatus = "paid"
	EarningStatusReversed  EarningStatus = "reversed"
)

type PayoutStatus string

const (
	PayoutStatusRequested  PayoutStatus = "requested"
	PayoutStatusReviewing  PayoutStatus = "reviewing"
	PayoutStatusApproved   PayoutStatus = "approved"
	PayoutStatusProcessing PayoutStatus = "processing"
	PayoutStatusPaid       PayoutStatus = "paid"
	PayoutStatusRejected   PayoutStatus = "rejected"
	PayoutStatusFailed     PayoutStatus = "failed"
)

type ApprovalMode string

const (
	ApprovalModeManual    ApprovalMode = "manual"
	ApprovalModeAutomatic ApprovalMode = "automatic"
)

type PayoutSchedule string

const (
	PayoutScheduleManual  PayoutSchedule = "manual"
	PayoutScheduleWeekly  PayoutSchedule = "weekly"
	PayoutScheduleMonthly PayoutSchedule = "monthly"
)

type PayoutMethodType string

const (
	PayoutMethodTypeBankTransfer PayoutMethodType = "bank_transfer"
	PayoutMethodTypeCrypto       PayoutMethodType = "crypto"
	PayoutMethodTypeEWallet      PayoutMethodType = "ewallet"
)

type FraudSeverity string

const (
	FraudSeverityLow      FraudSeverity = "low"
	FraudSeverityMedium   FraudSeverity = "medium"
	FraudSeverityHigh     FraudSeverity = "high"
	FraudSeverityCritical FraudSeverity = "critical"
)

type FraudFlagStatus string

const (
	FraudFlagStatusOpen      FraudFlagStatus = "open"
	FraudFlagStatusInReview  FraudFlagStatus = "in_review"
	FraudFlagStatusResolved  FraudFlagStatus = "resolved"
	FraudFlagStatusDismissed FraudFlagStatus = "dismissed"
)

type AffiliateEnrollmentRequest struct {
	ID         uuid.UUID
	UserID     int64
	Status     EnrollmentStatus
	Reason     string
	ReviewNotes string
	ReviewedBy string
	ReviewedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type AffiliateProfile struct {
	ID               uuid.UUID
	UserID           int64
	Status           AffiliateStatus
	AffiliateCode    string
	CommissionPlanID uuid.UUID
	CommissionRate   decimal.Decimal
	HoldPeriodDays   int
	MinPayoutAmount  decimal.Decimal
	Currency         string
	KYCRequired      bool
	ApprovalMode     ApprovalMode
	PayoutSchedule   PayoutSchedule
	ApprovedBy       string
	ApprovedAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type AffiliateCommissionPlan struct {
	ID                       uuid.UUID
	Name                     string
	CommissionType           string
	CommissionRate           decimal.Decimal
	HoldPeriodDays           int
	MinPayoutAmount          decimal.Decimal
	ApprovalMode             ApprovalMode
	PayoutSchedule           PayoutSchedule
	NegativeCarryoverEnabled bool
	IsDefault                bool
	IsActive                 bool
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

type AffiliateLink struct {
	ID           uuid.UUID
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

type AffiliateClick struct {
	ID                uuid.UUID
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

type AffiliateAttribution struct {
	ID             uuid.UUID
	AffiliateID    uuid.UUID
	ReferredUserID int64
	ClickID        string
	AttributedAt   time.Time
	CreatedAt      time.Time
}

type AffiliateDashboard struct {
	EarningsToday            decimal.Decimal
	EarningsThisMonth        decimal.Decimal
	PendingAmount            decimal.Decimal
	AvailableAmount          decimal.Decimal
	PaidAmount               decimal.Decimal
	NextPayoutDate           *time.Time
	Clicks                   int64
	Registrations            int64
	FTDCount                 int64
	ActivePlayers            int64
	GGRAmount                decimal.Decimal
	NGRAmount                decimal.Decimal
	CommissionAmount         decimal.Decimal
	CommissionReversedAmount decimal.Decimal
	CommissionAdjustedAmount decimal.Decimal
	Currency                 string
}

type AffiliateEarning struct {
	ID               uuid.UUID
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
	Status           EarningStatus
	HoldUntil        time.Time
	IdempotencyKey   string
	CreatedAt        time.Time
}

type AffiliatePayoutMethod struct {
	ID            uuid.UUID
	AffiliateID   uuid.UUID
	MethodType    PayoutMethodType
	DisplayName   string
	DetailsMasked string
	IsDefault     bool
	IsVerified    bool
	CreatedAt     time.Time
}

type AffiliatePayout struct {
	ID             uuid.UUID
	AffiliateID    uuid.UUID
	MethodID       uuid.UUID
	Amount         decimal.Decimal
	Currency       string
	Status         PayoutStatus
	IdempotencyKey string
	ApprovedBy     string
	ApprovedAt     *time.Time
	ProviderReference string
	RejectionReason   string
	RequestedAt       time.Time
	CreatedAt         time.Time
}

type AffiliateFraudFlag struct {
	ID             uuid.UUID
	AffiliateID    uuid.UUID
	ReferredUserID int64
	FlagType       string
	Severity       FraudSeverity
	Status         FraudFlagStatus
	Details        map[string]string
	CreatedAt      time.Time
	ResolvedAt     *time.Time
	ResolvedBy     string
}

type AdjustmentType string

const (
	AdjustmentTypeCredit AdjustmentType = "credit"
	AdjustmentTypeDebit  AdjustmentType = "debit"
)

type AffiliateAdjustment struct {
	ID             uuid.UUID
	AffiliateID    uuid.UUID
	AdjustmentType AdjustmentType
	Amount         decimal.Decimal
	Currency       string
	Reason         string
	CreatedBy      string
	CreatedAt      time.Time
}

type PayoutEligibility struct {
	AvailableAmount decimal.Decimal
	KYCApproved     bool
	HasOpenFraud    bool
	RequestedAt     time.Time
}

func (p AffiliateProfile) ValidatePayoutEligibility(in PayoutEligibility) error {
	if p.Status != AffiliateStatusActive {
		return ErrAffiliateInactive
	}
	if in.AvailableAmount.LessThan(p.MinPayoutAmount) {
		return ErrMinPayoutNotReached
	}
	if p.KYCRequired && !in.KYCApproved {
		return ErrAffiliateKYCRequired
	}
	if in.HasOpenFraud {
		return ErrAffiliateFraudBlocked
	}
	return nil
}

func CalculateCommission(ngr decimal.Decimal, rate decimal.Decimal) decimal.Decimal {
	if ngr.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero
	}
	commission := ngr.Mul(rate)
	if commission.IsNegative() {
		return decimal.Zero
	}
	return commission
}

func GenerateAffiliateCode() string {
	code := strings.ReplaceAll(uuid.New().String()[:8], "-", "")
	return strings.ToUpper(code)
}
