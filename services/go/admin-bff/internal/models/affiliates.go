package models

import (
	"time"
)

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

type PostbackConfig struct {
	Event         string            `json:"event"`
	URL           string            `json:"url"`
	Method        string            `json:"method"`
	Variables     map[string]string `json:"variables"`
	RetryCount    int               `json:"retry_count"`
	RetryBackoff  string            `json:"retry_backoff"`
}

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
