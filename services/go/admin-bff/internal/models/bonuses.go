package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

type GameWeights struct {
	Slots      int `json:"slots"`
	LiveCasino int `json:"live_casino"`
	TableGames int `json:"table_games"`
	Sports     int `json:"sports"`
}

func (gw GameWeights) Value() (driver.Value, error) {
	return json.Marshal(gw)
}

func (gw *GameWeights) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(b, gw)
}

type StringSliceJSON []string

func (s StringSliceJSON) Value() (driver.Value, error) {
	return json.Marshal(s)
}

func (s *StringSliceJSON) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(b, s)
}

type Bonus struct {
	ID                  string     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name                string     `gorm:"type:varchar(255);not null" json:"name"`
	Description         *string    `gorm:"type:text" json:"description,omitempty"`
	Status              string     `gorm:"type:bonus_engine_status;not null;default:'draft'" json:"status"`
	BonusType           string     `gorm:"type:bonus_type;not null" json:"bonus_type"`
	ValidFrom           *time.Time `json:"valid_from,omitempty"`
	ValidTo             *time.Time `json:"valid_to,omitempty"`
	MaxUsesGlobal       int        `gorm:"not null;default:0" json:"max_uses_global"`
	MaxUsesPerPlayer    int        `gorm:"not null;default:1" json:"max_uses_per_player"`
	MatchPercentage     *string    `gorm:"type:numeric(5,2)" json:"match_percentage,omitempty"`
	MaxBonusAmount      *string    `gorm:"type:numeric(18,2)" json:"max_bonus_amount,omitempty"`
	MinDeposit          *string    `gorm:"type:numeric(18,2)" json:"min_deposit,omitempty"`
	FreeSpinsCount      *int       `json:"free_spins_count,omitempty"`
	FreeSpinsGame       *string    `gorm:"type:varchar(255)" json:"free_spins_game,omitempty"`
	SpinValue           *string    `gorm:"type:numeric(18,2)" json:"spin_value,omitempty"`
	CashbackPercentage  *string    `gorm:"type:numeric(5,2)" json:"cashback_percentage,omitempty"`
	CashbackCalculation *string    `gorm:"type:varchar(20)" json:"cashback_calculation,omitempty"`
	CashbackPeriod      *string    `gorm:"type:varchar(20)" json:"cashback_period,omitempty"`
	FreebetAmount       *string    `gorm:"type:numeric(18,2)" json:"freebet_amount,omitempty"`
	FreebetMinOdds      *string    `gorm:"type:numeric(5,2)" json:"freebet_min_odds,omitempty"`
	FreebetAllowed      *string    `gorm:"type:varchar(20)" json:"freebet_allowed,omitempty"`
	ReturnStakeOnWin    *bool      `json:"return_stake_on_win,omitempty"`
	WageringMultiplier  string     `gorm:"type:numeric(5,2);not null;default:1.0" json:"wagering_multiplier"`
	WageringTarget      string     `gorm:"type:bonus_wagering_target;not null;default:'deposit_and_bonus'" json:"wagering_target"`
	WageringTimeframeDays int      `gorm:"not null;default:7" json:"wagering_timeframe_days"`
	MaxBetWhileActive   *string    `gorm:"type:numeric(18,2)" json:"max_bet_while_active,omitempty"`
	MaxWinFromBonus     *string    `gorm:"type:numeric(18,2)" json:"max_win_from_bonus,omitempty"`
	GameWeights         GameWeights `gorm:"type:jsonb;default:'{\"slots\":100,\"live_casino\":0,\"table_games\":0,\"sports\":0}'::jsonb" json:"game_weights"`
	ExcludedGames       StringSliceJSON `gorm:"type:jsonb;default:'[]'::jsonb" json:"excluded_games,omitempty"`
	Sticky              bool       `gorm:"not null;default:false" json:"sticky"`
	EligibleCountries   StringSliceJSON `gorm:"type:jsonb;default:'[]'::jsonb" json:"eligible_countries,omitempty"`
	ExcludedTags        StringSliceJSON `gorm:"type:jsonb;default:'[]'::jsonb" json:"excluded_tags,omitempty"`
	PlayerGroups        StringSliceJSON `gorm:"type:jsonb;default:'[\"all\"]'::jsonb" json:"player_groups,omitempty"`
	PromoCode           *string    `gorm:"type:varchar(100)" json:"promo_code,omitempty"`
	AutoAssignTrigger   string     `gorm:"type:bonus_trigger;not null;default:'manual'" json:"auto_assign_trigger"`
	CanCombine          bool       `gorm:"not null;default:false" json:"can_combine"`
	TotalIssued         int        `gorm:"not null;default:0" json:"total_issued"`
	TotalCost           string     `gorm:"type:numeric(18,2);not null;default:0" json:"total_cost"`
	ConversionRatePct   *string    `gorm:"type:numeric(5,2)" json:"conversion_rate_pct,omitempty"`
	CreatedBy           string     `gorm:"type:varchar(36);not null" json:"created_by"`
	CreatedByName       string     `gorm:"type:varchar(255);not null" json:"created_by_name"`
	CreatedAt           time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt           time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

type PlayerBonus struct {
	ID                string     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PlayerID          int64      `gorm:"not null;index" json:"player_id"`
	PlayerEmail       string     `gorm:"type:varchar(255);not null" json:"player_email"`
	BonusID           string     `gorm:"type:uuid;not null" json:"bonus_id"`
	BonusName         string     `gorm:"type:varchar(255);not null" json:"bonus_name"`
	IssuedAt          time.Time  `gorm:"not null;default:now()" json:"issued_at"`
	BonusAmount       string     `gorm:"type:numeric(18,2);not null;default:0" json:"bonus_amount"`
	TargetWagerAmount string     `gorm:"type:numeric(18,2);not null;default:0" json:"target_wager_amount"`
	WageredAmount     string     `gorm:"type:numeric(18,2);not null;default:0" json:"wagered_amount"`
	Status            string     `gorm:"type:player_bonus_status;not null;default:'active'" json:"status"`
	ProgressPct       string     `gorm:"type:numeric(5,2);not null;default:0" json:"progress_pct"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	VoidedAt          *time.Time `json:"voided_at,omitempty"`
	VoidedBy          *string    `gorm:"type:varchar(36)" json:"voided_by,omitempty"`
	VoidReason        *string    `gorm:"type:text" json:"void_reason,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	MaxBetViolation   bool       `gorm:"not null;default:false" json:"max_bet_violation"`
	CreatedAt         time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt         time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

type BonusActivation struct {
	ID           string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	BonusID      string    `gorm:"type:uuid;not null" json:"bonus_id"`
	PlayerID     int64     `gorm:"not null" json:"player_id"`
	PlayerEmail  string    `gorm:"type:varchar(255);not null" json:"player_email"`
	TriggeredBy  string    `gorm:"type:varchar(50);not null;default:'manual'" json:"triggered_by"`
	BonusAmount  string    `gorm:"type:numeric(18,2);not null;default:0" json:"bonus_amount"`
	CreatedAt    time.Time `gorm:"not null;default:now()" json:"created_at"`
}

type WageringMonitor struct {
	ID              string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PlayerID        int64     `gorm:"not null;index" json:"player_id"`
	PlayerEmail     string    `gorm:"type:varchar(255);not null" json:"player_email"`
	BonusID         string    `gorm:"type:uuid;not null" json:"bonus_id"`
	BonusName       string    `gorm:"type:varchar(255);not null" json:"bonus_name"`
	WageredAmount   string    `gorm:"type:numeric(18,2);not null;default:0" json:"wagered_amount"`
	TargetAmount    string    `gorm:"type:numeric(18,2);not null;default:0" json:"target_amount"`
	ProgressPct     string    `gorm:"type:numeric(5,2);not null;default:0" json:"progress_pct"`
	HoursRemaining  int       `gorm:"not null;default:0" json:"hours_remaining"`
	AbnormallyFast  bool      `gorm:"not null;default:false" json:"abnormally_fast"`
	NearCompletion  bool      `gorm:"not null;default:false" json:"near_completion"`
	ExpiresSoon     bool      `gorm:"not null;default:false" json:"expires_soon"`
	UpdatedAt       time.Time `gorm:"not null;default:now()" json:"updated_at"`
}
