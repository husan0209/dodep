package models

import (
	"time"
)

// Admin-side affiliate record (013_admin_bff.sql). Renamed to admin_affiliates
// by libs/migrations/postgresql/020_affiliate_admin_bff_split.sql so that the
// affiliate_profiles namespace stays owned by affiliate-service alone.
// This table carries admin-only deal terms (deal_type, cpa_amount,
// postback_configs) that are not part of the RevShare-from-NGR model.
type Affiliate struct {
	ID                  string            `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID              string            `gorm:"type:varchar(36);not null;index" json:"user_id"`
	Status              string            `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	DealType            string            `gorm:"type:varchar(20);not null;default:'revenue_share'" json:"deal_type"`
	RevenueSharePct     float64           `gorm:"not null;default:0" json:"revenue_share_pct"`
	CPAAmount           string            `gorm:"type:numeric(18,2);not null;default:0" json:"cpa_amount"`
	HoldPeriodDays      int               `gorm:"not null;default:0" json:"hold_period_days"`
	MinPayoutAmount     string            `gorm:"type:numeric(18,2);not null;default:0" json:"min_payout_amount"`
	Currency            string            `gorm:"type:varchar(3);not null;default:'USD'" json:"currency"`
	SubAffiliateEnabled bool              `gorm:"not null;default:false" json:"sub_affiliate_enabled"`
	SubAffiliatePct     float64           `gorm:"not null;default:0" json:"sub_affiliate_pct"`
	PostbackConfigs     []PostbackConfig  `gorm:"type:jsonb;not null;default:'[]'" json:"postback_configs"`
	CreatedAt           time.Time         `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt           time.Time         `gorm:"not null;default:now()" json:"updated_at"`
}

func (Affiliate) TableName() string { return "admin_affiliates" }

type PostbackConfig struct {
	Event         string            `json:"event"`
	URL           string            `json:"url"`
	Method        string            `json:"method"`
	Variables     map[string]string `json:"variables"`
	RetryCount    int               `json:"retry_count"`
	RetryBackoff  string            `json:"retry_backoff"`
}

// Admin-side payout record (013_admin_bff.sql), renamed to
// admin_affiliate_payouts by 020_affiliate_admin_bff_split.sql.
//
// NOT the affiliate-service payout: that table (UUID affiliate_id, method_id,
// idempotency_key, NUMERIC(18,8)) is pinned separately in affiliate_stats.go as
// AffiliatePayoutAmount. The two namespaces must never be mixed.
type AffiliatePayout struct {
	ID               string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	AffiliateID      string    `gorm:"type:varchar(36);not null;index" json:"affiliate_id"`
	PeriodStart      time.Time `gorm:"not null" json:"period_start"`
	PeriodEnd        time.Time `gorm:"not null" json:"period_end"`
	Amount           string    `gorm:"type:numeric(18,2);not null" json:"amount"`
	Currency         string    `gorm:"type:varchar(3);not null;default:'USD'" json:"currency"`
	Status           string    `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	ProviderReference *string  `gorm:"type:varchar(255)" json:"provider_reference,omitempty"`
	RejectionReason   *string  `gorm:"type:text" json:"rejection_reason,omitempty"`
	CreatedAt        time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt        time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

func (AffiliatePayout) TableName() string { return "admin_affiliate_payouts" }

// Admin-side fraud flag (013_admin_bff.sql), renamed to admin_fraud_flags by
// 020_affiliate_admin_bff_split.sql. Distinct from affiliate-service's
// affiliate_fraud_flags, which is written by the automatic anti-fraud engine.
type FraudFlag struct {
	ID          string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	AffiliateID string    `gorm:"type:varchar(36);not null;index" json:"affiliate_id"`
	PlayerID    string    `gorm:"type:varchar(36);not null" json:"player_id"`
	FlagType    string    `gorm:"type:varchar(50);not null" json:"flag_type"`
	Reason      string    `gorm:"type:text;not null" json:"reason"`
	Status      string    `gorm:"type:varchar(20);not null;default:'open'" json:"status"`
	ResolvedBy  *string   `gorm:"type:varchar(36)" json:"resolved_by,omitempty"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	CreatedAt   time.Time `gorm:"not null;default:now()" json:"created_at"`
}

func (FraudFlag) TableName() string { return "admin_fraud_flags" }

// Admin-side postback log (013_admin_bff.sql), renamed to
// admin_postback_logs by 020_affiliate_admin_bff_split.sql.

type PostbackLog struct {
	ID          string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	AffiliateID string    `gorm:"type:varchar(36);not null;index" json:"affiliate_id"`
	Event       string    `gorm:"type:varchar(50);not null" json:"event"`
	PlayerID    string    `gorm:"type:varchar(36)" json:"player_id"`
	URL         string    `gorm:"type:text;not null" json:"url"`
	HTTPStatus  int       `gorm:"not null;default:0" json:"http_status"`
	Response    string    `gorm:"type:text" json:"response"`
	AttemptNo   int       `gorm:"not null;default:1" json:"attempt_no"`
	Success     bool      `gorm:"not null;default:false" json:"success"`
	SentAt      time.Time `gorm:"not null;default:now()" json:"sent_at"`
	CreatedAt   time.Time `gorm:"not null;default:now()" json:"created_at"`
}

func (PostbackLog) TableName() string { return "admin_postback_logs" }
