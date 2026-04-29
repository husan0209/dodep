package models

import (
	"encoding/json"
	"time"
)

type CasinoGame struct {
	ID                  string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ExternalID          string    `gorm:"type:varchar(100);not null;uniqueIndex" json:"external_id"`
	Name                string    `gorm:"type:varchar(255);not null" json:"name"`
	DisplayName         *string   `gorm:"type:varchar(255)" json:"display_name,omitempty"`
	ProviderID          string    `gorm:"type:varchar(100);not null;index" json:"provider_id"`
	ProviderName        string    `gorm:"type:varchar(255);not null" json:"provider_name"`
	Category            string    `gorm:"type:varchar(50);not null" json:"category"`
	Tags                []string  `gorm:"type:text[]" json:"tags"`
	Description         string    `gorm:"type:text" json:"description"`
	ImageURL            string    `gorm:"type:varchar(512)" json:"image_url"`
	ThumbnailURL        string    `gorm:"type:varchar(512)" json:"thumbnail_url"`
	SupportedCurrencies []string  `gorm:"type:text[]" json:"supported_currencies"`
	MinBet              string    `gorm:"type:numeric(18,2);not null" json:"min_bet"`
	MaxBet              string    `gorm:"type:numeric(18,2);not null" json:"max_bet"`
	Rtp                 float64   `gorm:"not null;default:96.0" json:"rtp"`
	Volatility          string    `gorm:"type:varchar(20);not null;default:'medium'" json:"volatility"`
	IsActive            bool      `gorm:"not null;default:true" json:"is_active"`
	IsDemoAvailable     bool      `gorm:"not null;default:true" json:"is_demo_available"`
	RestrictedCountries []string  `gorm:"type:text[]" json:"restricted_countries"`
	CountryRestrictions []string  `gorm:"type:text[]" json:"country_restrictions"`
	Badge               *string   `gorm:"type:varchar(50)" json:"badge,omitempty"`
	SortWeight          int32     `gorm:"not null;default:0" json:"sort_weight"`
	PopularityScore     int32     `gorm:"not null;default:0" json:"popularity_score"`
	ReleasedAt          time.Time `gorm:"not null" json:"released_at"`
	CreatedAt           time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt           time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

type CasinoProvider struct {
	ID                     string            `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ExternalID             string            `gorm:"type:varchar(100);not null;uniqueIndex" json:"external_id"`
	Name                   string            `gorm:"type:varchar(255);not null" json:"name"`
	LogoURL                string            `gorm:"type:varchar(512)" json:"logo_url"`
	Description            string            `gorm:"type:text" json:"description"`
	IntegrationType        string            `gorm:"type:varchar(50);not null;default:'direct'" json:"integration_type"`
	IsActive               bool              `gorm:"not null;default:true" json:"is_active"`
	GamesCount             int32             `gorm:"not null;default:0" json:"games_count"`
	SupportedCurrencies    []string          `gorm:"type:text[]" json:"supported_currencies"`
	RestrictedCountries    []string          `gorm:"type:text[]" json:"restricted_countries"`
	RevenueSharePct        float64           `gorm:"not null;default:0" json:"revenue_share_pct"`
	SettlementCurrency     string            `gorm:"type:varchar(3);not null;default:'USD'" json:"settlement_currency"`
	APICredentialsEncrypted *string          `gorm:"type:text" json:"api_credentials_encrypted,omitempty"`
	Metadata               map[string]string `gorm:"type:jsonb;not null;default:'{}'" json:"metadata"`
	CreatedAt              time.Time         `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt              time.Time         `gorm:"not null;default:now()" json:"updated_at"`
}

type RtpConfig struct {
	ID            string     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	GameID        *string    `gorm:"type:varchar(100);index" json:"game_id,omitempty"`
	ProviderID    *string    `gorm:"type:varchar(100);index" json:"provider_id,omitempty"`
	PlayerGroup   string     `gorm:"type:varchar(50);not null;default:'default'" json:"player_group"`
	TargetRtp     float64    `gorm:"not null;default:96.0" json:"target_rtp"`
	ImpactEstimate *float64  `gorm:"type:numeric(5,2)" json:"impact_estimate,omitempty"`
	OverrideBy    *string    `gorm:"type:varchar(36)" json:"override_by,omitempty"`
	OverrideAt    *time.Time `json:"override_at,omitempty"`
	ConfirmedBy   *string    `gorm:"type:varchar(36)" json:"confirmed_by,omitempty"`
	ConfirmedAt   *time.Time `json:"confirmed_at,omitempty"`
	CreatedAt     time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

type JackpotPool struct {
	ID                string         `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name              string         `gorm:"type:varchar(255);not null" json:"name"`
	Type              string         `gorm:"type:varchar(50);not null" json:"type"`
	GameIDs           []string       `gorm:"type:text[]" json:"game_ids"`
	EligibleGames     []string       `gorm:"type:text[]" json:"eligible_games"`
	SeedAmount        string         `gorm:"type:numeric(18,2);not null" json:"seed_amount"`
	CurrentAmount     string         `gorm:"type:numeric(18,2);not null" json:"current_amount"`
	Currency          string         `gorm:"type:varchar(3);not null" json:"currency"`
	ContributionPct   float64        `gorm:"not null;default:0" json:"contribution_pct"`
	SeedValue         string         `gorm:"type:numeric(18,2);not null" json:"seed_value"`
	DailyDropsConfig  map[string]any `gorm:"type:jsonb;not null;default:'{}'" json:"daily_drops_config"`
	IsActive          bool           `gorm:"not null;default:true" json:"is_active"`
	CreatedAt         time.Time      `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt         time.Time      `gorm:"not null;default:now()" json:"updated_at"`
}

type ProviderSettlement struct {
	ID              string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ProviderID      string    `gorm:"type:varchar(100);not null;index" json:"provider_id"`
	PeriodStart     time.Time `gorm:"not null" json:"period_start"`
	PeriodEnd       time.Time `gorm:"not null" json:"period_end"`
	Currency        string    `gorm:"type:varchar(3);not null" json:"currency"`
	GGR             string    `gorm:"type:numeric(18,2);not null" json:"ggr"`
	RevenueShare    string    `gorm:"type:numeric(18,2);not null" json:"revenue_share"`
	Status          string    `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	InvoiceNumber   *string   `gorm:"type:varchar(100)" json:"invoice_number,omitempty"`
	PaidAt          *time.Time `json:"paid_at,omitempty"`
	CreatedAt       time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt       time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

// RtpAuditLog tracks every RTP change with before/after snapshot for compliance.
type RtpAuditLog struct {
	ID           string     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	GameID       *string    `gorm:"type:varchar(100);index" json:"game_id,omitempty"`
	ProviderID   *string    `gorm:"type:varchar(100);index" json:"provider_id,omitempty"`
	PlayerGroup  string     `gorm:"type:varchar(50);not null" json:"player_group"`
	BeforeRtp    float64    `gorm:"not null" json:"before_rtp"`
	AfterRtp     float64    `gorm:"not null" json:"after_rtp"`
	ImpactEstimate *float64 `gorm:"type:numeric(5,2)" json:"impact_estimate,omitempty"`
	ChangedBy    string     `gorm:"type:varchar(36);not null" json:"changed_by"`
	ConfirmedBy  *string    `gorm:"type:varchar(36)" json:"confirmed_by,omitempty"`
	ConfirmedAt  *time.Time `json:"confirmed_at,omitempty"`
	Reason       string     `gorm:"type:text" json:"reason"`
	CreatedAt    time.Time  `gorm:"not null;default:now()" json:"created_at"`
}

// CasinoGameSession tracks a player session in a casino game.
type CasinoGameSession struct {
	ID             string     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID         int64      `gorm:"not null;index" json:"user_id"`
	GameID         string     `gorm:"type:uuid;not null;index" json:"game_id"`
	ProviderID     string     `gorm:"type:varchar(100);not null;index" json:"provider_id"`
	Status         string     `gorm:"type:varchar(20);not null;default:'active'" json:"status"`
	BalanceAtStart string     `gorm:"type:numeric(18,2);not null" json:"balance_at_start"`
	Currency       string     `gorm:"type:varchar(3);not null;default:'USD'" json:"currency"`
	TotalBet       string     `gorm:"type:numeric(18,2);not null;default:0" json:"total_bet"`
	TotalWin       string     `gorm:"type:numeric(18,2);not null;default:0" json:"total_win"`
	NetResult      string     `gorm:"type:numeric(18,2);not null;default:0" json:"net_result"`
	RoundsPlayed   int32      `gorm:"not null;default:0" json:"rounds_played"`
	DeviceType     *string    `gorm:"type:varchar(20)" json:"device_type,omitempty"`
	IPAddress      *string    `gorm:"type:inet" json:"ip_address,omitempty"`
	StartedAt      time.Time  `gorm:"not null" json:"started_at"`
	EndedAt        *time.Time `json:"ended_at,omitempty"`
	LastActivityAt time.Time  `gorm:"not null" json:"last_activity_at"`
	CreatedAt      time.Time  `gorm:"not null;default:now()" json:"created_at"`
}

// CasinoGameRound tracks an individual round / bet within a session.
type CasinoGameRound struct {
	ID         string          `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	SessionID  string          `gorm:"type:uuid;not null;index" json:"session_id"`
	UserID     int64           `gorm:"not null;index" json:"user_id"`
	GameID     string          `gorm:"type:uuid;not null;index" json:"game_id"`
	ProviderID string          `gorm:"type:varchar(100);not null;index" json:"provider_id"`
	RoundID    string          `gorm:"type:varchar(255);not null" json:"round_id"`
	BetAmount  string          `gorm:"type:numeric(18,2);not null" json:"bet_amount"`
	WinAmount  string          `gorm:"type:numeric(18,2);not null" json:"win_amount"`
	NetResult  string          `gorm:"type:numeric(18,2);not null" json:"net_result"`
	Currency   string          `gorm:"type:varchar(3);not null;default:'USD'" json:"currency"`
	Status     string          `gorm:"type:varchar(20);not null;default:'completed'" json:"status"`
	Details    json.RawMessage `gorm:"type:jsonb;not null;default:'{}'" json:"details"`
	StartedAt  time.Time       `gorm:"not null" json:"started_at"`
	EndedAt    *time.Time      `json:"ended_at,omitempty"`
	CreatedAt  time.Time       `gorm:"not null;default:now()" json:"created_at"`
}
