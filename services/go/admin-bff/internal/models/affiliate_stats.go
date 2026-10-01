package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// The admin panel aggregates read-only projections of the affiliate-service
// schema (libs/migrations/postgresql/021_affiliates.sql). The admin BFF does
// not own these tables: it only reads them to render affiliate drill-downs.
// Money columns are NUMERIC(18,8) and are surfaced as strings/decimal —
// NEVER float (CONVENTIONS NEVER-6).

// AffiliateProfile mirrors affiliate_profiles (affiliate-service, read-only).
// Scoped by user_id because the admin-side `affiliates` table uses a separate
// UUID space (013_admin_bff.sql) that must not be mixed with this one.
type AffiliateProfile struct {
	ID               string    `gorm:"type:uuid;primary_key"`
	UserID           int64     `gorm:"column:user_id;not null"`
	Status           string    `gorm:"column:status"`
	AffiliateCode    string    `gorm:"column:affiliate_code"`
	CommissionRate   string    `gorm:"column:commission_rate;type:numeric(10,4)"`
	HoldPeriodDays   int       `gorm:"column:hold_period_days"`
	MinPayoutAmount  string    `gorm:"column:min_payout_amount;type:numeric(18,8)"`
	Currency         string    `gorm:"column:currency"`
	ApprovedAt       *time.Time `gorm:"column:approved_at"`
}

// TableName pins the affiliate-service table.
func (AffiliateProfile) TableName() string { return "affiliate_profiles" }

// AffiliateAttribution mirrors affiliate_attributions (read-only).
type AffiliateAttribution struct {
	ID             string     `gorm:"type:uuid;primary_key"`
	AffiliateID    string     `gorm:"type:uuid;column:affiliate_id"`
	ReferredUserID int64      `gorm:"column:referred_user_id"`
	ClickID        *string    `gorm:"column:click_id"`
	AttributionModel string   `gorm:"column:attribution_model"`
	AttributedAt   time.Time  `gorm:"column:attributed_at"`
	FTDAt          *time.Time `gorm:"column:ftd_at"`
	IsFTDQualified bool       `gorm:"column:is_ftd_qualified"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
}

// TableName pins the affiliate-service table.
func (AffiliateAttribution) TableName() string { return "affiliate_attributions" }

// AffiliateDailyAggregate mirrors affiliate_daily_aggregates (read-only).
// Used for the funnel (clicks → registrations → FTD → active) and revenue.
type AffiliateDailyAggregate struct {
	ID                 string          `gorm:"type:uuid;primary_key"`
	AffiliateID        string          `gorm:"type:uuid;column:affiliate_id"`
	ReportDate         time.Time       `gorm:"column:report_date"`
	Clicks             int64           `gorm:"column:clicks"`
	Registrations      int64           `gorm:"column:registrations"`
	FTDCount           int64           `gorm:"column:ftd_count"`
	ActivePlayers      int64           `gorm:"column:active_players"`
	GGRAmount          decimal.Decimal `gorm:"column:ggr_amount;type:numeric(18,8)"`
	NGRAmount          decimal.Decimal `gorm:"column:ngr_amount;type:numeric(18,8)"`
	CommissionAccrued  decimal.Decimal `gorm:"column:commission_accrued;type:numeric(18,8)"`
	CommissionReversed decimal.Decimal `gorm:"column:commission_reversed;type:numeric(18,8)"`
}

// TableName pins the affiliate-service table.
func (AffiliateDailyAggregate) TableName() string { return "affiliate_daily_aggregates" }

// AffiliateEarning mirrors affiliate_earnings (read-only).
// Status is the affiliate_earning_status enum value.
type AffiliateEarning struct {
	ID              string          `gorm:"type:uuid;primary_key"`
	AffiliateID     string          `gorm:"type:uuid;column:affiliate_id"`
	ReferredUserID  int64           `gorm:"column:referred_user_id"`
	SourceType      string          `gorm:"column:source_type"`
	GGRAmount       decimal.Decimal `gorm:"column:ggr_amount;type:numeric(18,8)"`
	NGRAmount       decimal.Decimal `gorm:"column:ngr_amount;type:numeric(18,8)"`
	CommissionAmount decimal.Decimal `gorm:"column:commission_amount;type:numeric(18,8)"`
	Status          string          `gorm:"column:status"`
	HoldUntil       time.Time       `gorm:"column:hold_until"`
	CreatedAt       time.Time       `gorm:"column:created_at"`
}

// TableName pins the affiliate-service table.
func (AffiliateEarning) TableName() string { return "affiliate_earnings" }

// AffiliateFraudFlagAmount mirrors affiliate_fraud_flags (affiliate-service,
// read-only). Deliberately NOT models.FraudFlag: that one is pinned to the
// admin-side admin_fraud_flags table, which lives in a different affiliate id
// namespace. Mixing them would silently return zero open flags.
type AffiliateFraudFlagAmount struct {
	ID          string    `gorm:"type:uuid;primary_key"`
	AffiliateID string    `gorm:"type:uuid;column:affiliate_id"`
	ReferredUserID *int64 `gorm:"column:referred_user_id"`
	FlagType    string    `gorm:"column:flag_type"`
	Status      string    `gorm:"column:status"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

// TableName pins the affiliate-service table.
func (AffiliateFraudFlagAmount) TableName() string { return "affiliate_fraud_flags" }

// AffiliatePayoutAmount mirrors affiliate_payouts (affiliate-service, read-only).
type AffiliatePayoutAmount struct {
	ID          string          `gorm:"type:uuid;primary_key"`
	AffiliateID string          `gorm:"type:uuid;column:affiliate_id"`
	Amount      decimal.Decimal `gorm:"column:amount;type:numeric(18,8)"`
	Status      string          `gorm:"column:status"`
	CreatedAt   time.Time       `gorm:"column:created_at"`
}

// TableName pins the affiliate-service table.
func (AffiliatePayoutAmount) TableName() string { return "affiliate_payouts" }