package models

import (
	"time"
)

type SportsEvent struct {
	ID            string     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ExternalID    string     `gorm:"type:varchar(100);not null;uniqueIndex" json:"external_id"`
	Sport         string     `gorm:"type:varchar(50);not null;index" json:"sport"`
	League        string     `gorm:"type:varchar(100);not null" json:"league"`
	HomeTeam      string     `gorm:"type:varchar(255);not null" json:"home_team"`
	AwayTeam      string     `gorm:"type:varchar(255);not null" json:"away_team"`
	StartTime     time.Time  `gorm:"not null" json:"start_time"`
	Status        string     `gorm:"type:varchar(20);not null;default:'upcoming'" json:"status"`
	ScoreHome     *int       `json:"score_home,omitempty"`
	ScoreAway     *int       `json:"score_away,omitempty"`
	IsSuspended   bool       `gorm:"not null;default:false" json:"is_suspended"`
	SuspendReason *string    `gorm:"type:text" json:"suspend_reason,omitempty"`
	CustomMargin  *string    `gorm:"type:numeric(5,2)" json:"custom_margin,omitempty"`
	CreatedAt     time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

type SportsMarket struct {
	ID        string `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	EventID   string `gorm:"type:uuid;not null;index" json:"event_id"`
	Name      string `gorm:"type:varchar(100);not null" json:"name"`
	Type      string `gorm:"type:varchar(50);not null" json:"type"`
	IsActive  bool   `gorm:"not null;default:true" json:"is_active"`
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
}

type OddsOverride struct {
	ID          string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	MarketID    string    `gorm:"type:uuid;not null;index" json:"market_id"`
	Selection   string    `gorm:"type:varchar(100);not null" json:"selection"`
	Odds        string    `gorm:"type:numeric(10,4);not null" json:"odds"`
	Reason      string    `gorm:"type:text;not null" json:"reason"`
	SetBy       string    `gorm:"type:varchar(36);not null" json:"set_by"`
	RevertedAt  *time.Time `json:"reverted_at,omitempty"`
	CreatedAt   time.Time `gorm:"not null;default:now()" json:"created_at"`
}

type MarginSetting struct {
	ID          string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ScopeType   string    `gorm:"type:varchar(20);not null" json:"scope_type"`
	ScopeID     *string   `gorm:"type:varchar(100)" json:"scope_id,omitempty"`
	Sport       *string   `gorm:"type:varchar(50)" json:"sport,omitempty"`
	League      *string   `gorm:"type:varchar(100)" json:"league,omitempty"`
	MarginValue string    `gorm:"type:numeric(5,2);not null" json:"margin_value"`
	UpdatedBy   string    `gorm:"type:varchar(36);not null" json:"updated_by"`
	CreatedAt   time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt   time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

type StakeLimit struct {
	ID          string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ScopeType   string    `gorm:"type:varchar(20);not null" json:"scope_type"`
	ScopeID     *string   `gorm:"type:varchar(100)" json:"scope_id,omitempty"`
	MaxStake    string    `gorm:"type:numeric(18,2);not null" json:"max_stake"`
	MaxWin      string    `gorm:"type:numeric(18,2);not null" json:"max_win"`
	MaxLiability string   `gorm:"type:numeric(18,2);not null" json:"max_liability"`
	UpdatedBy   string    `gorm:"type:varchar(36);not null" json:"updated_by"`
	CreatedAt   time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt   time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

type LiabilitySnapshot struct {
	ID           string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	EventID      string    `gorm:"type:varchar(100);not null;index" json:"event_id"`
	MarketID     string    `gorm:"type:varchar(100);not null" json:"market_id"`
	Selection    string    `gorm:"type:varchar(100);not null" json:"selection"`
	TotalStake   string    `gorm:"type:numeric(18,2);not null" json:"total_stake"`
	TotalBets    int       `gorm:"not null;default:0" json:"total_bets"`
	Liability    string    `gorm:"type:numeric(18,2);not null" json:"liability"`
	Limit        string    `gorm:"type:numeric(18,2);not null" json:"limit"`
	RecordedAt   time.Time `gorm:"not null;default:now()" json:"recorded_at"`
}
