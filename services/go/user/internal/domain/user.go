package domain

import (
	"time"
)

type UserStatus string

const (
	UserStatusActive        UserStatus = "active"
	UserStatusInactive      UserStatus = "inactive"
	UserStatusSuspended     UserStatus = "suspended"
	UserStatusSelfExcluded  UserStatus = "self_excluded"
	UserStatusDeleted       UserStatus = "deleted"
)

type KYCLevel int

const (
	KYCLevelNone      KYCLevel = 0
	KYCLevelBasic     KYCLevel = 1
	KYCLevelVerified  KYCLevel = 2
	KYCLevelEnhanced  KYCLevel = 3
)

type User struct {
	ID            int64       `json:"id" db:"id"`
	UUID          string      `json:"uuid" db:"uuid"`
	Email         string      `json:"email" db:"email"`
	Username      string      `json:"username" db:"username"`
	FirstName     *string     `json:"first_name,omitempty" db:"first_name"`
	LastName      *string     `json:"last_name,omitempty" db:"last_name"`
	Phone         *string     `json:"phone,omitempty" db:"phone"`
	DateOfBirth   *string     `json:"date_of_birth,omitempty" db:"date_of_birth"`
	CountryCode   string      `json:"country_code" db:"country_code"`
	CurrencyCode  string      `json:"currency_code" db:"currency_code"`
	Status        UserStatus  `json:"status" db:"status"`
	KYCLevel      KYCLevel    `json:"kyc_level" db:"kyc_level"`
	Language      string      `json:"language" db:"language"`
	Timezone      string      `json:"timezone" db:"timezone"`
	Address       *string     `json:"address,omitempty" db:"address"`
	City          *string     `json:"city,omitempty" db:"city"`
	PostalCode    *string     `json:"postal_code,omitempty" db:"postal_code"`
	ReferralCode  *string     `json:"referral_code,omitempty" db:"referral_code"`
	CreatedAt     time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at" db:"updated_at"`
	LastLoginAt   *time.Time  `json:"last_login_at,omitempty" db:"last_login_at"`
	Metadata      string      `json:"metadata,omitempty" db:"metadata"`
}

type UpdateUserRequest struct {
	UserID      int64   `json:"user_id"`
	Username    *string `json:"username,omitempty"`
	FirstName   *string `json:"first_name,omitempty"`
	LastName    *string `json:"last_name,omitempty"`
	Phone       *string `json:"phone,omitempty"`
	DateOfBirth *string `json:"date_of_birth,omitempty"`
	Address     *string `json:"address,omitempty"`
	City        *string `json:"city,omitempty"`
	PostalCode  *string `json:"postal_code,omitempty"`
	Language    *string `json:"language,omitempty"`
	Timezone    *string `json:"timezone,omitempty"`
}

type UserPreferences struct {
	UserID                      int64  `json:"user_id" db:"user_id"`
	Language                    string `json:"language" db:"language"`
	Timezone                    string `json:"timezone" db:"timezone"`
	CurrencyDisplay             string `json:"currency_display" db:"currency_display"`
	MarketingEmails             bool   `json:"marketing_emails" db:"marketing_emails"`
	SMSNotifications            bool   `json:"sms_notifications" db:"sms_notifications"`
	PushNotifications           bool   `json:"push_notifications" db:"push_notifications"`
	RealityCheck                bool   `json:"reality_check" db:"reality_check"`
	RealityCheckIntervalMinutes int    `json:"reality_check_interval_minutes" db:"reality_check_interval_minutes"`
	AutoPlay                    bool   `json:"auto_play" db:"auto_play"`
	SoundPreference             string `json:"sound_preference" db:"sound_preference"`
	UpdatedAt                   time.Time `json:"updated_at" db:"updated_at"`
}

type MoneyLimit struct {
	Amount        string     `json:"amount" db:"amount"`
	IsActive      bool       `json:"is_active" db:"is_active"`
	SetAt         time.Time  `json:"set_at" db:"set_at"`
	CooldownUntil *time.Time `json:"cooldown_until,omitempty" db:"cooldown_until"`
}

type TimeLimit struct {
	Minutes  int       `json:"minutes" db:"minutes"`
	IsActive bool      `json:"is_active" db:"is_active"`
	SetAt    time.Time `json:"set_at" db:"set_at"`
}

type UserLimits struct {
	UserID              int64      `json:"user_id" db:"user_id"`
	DailyDepositLimit   *MoneyLimit `json:"daily_deposit_limit,omitempty"`
	WeeklyDepositLimit  *MoneyLimit `json:"weekly_deposit_limit,omitempty"`
	MonthlyDepositLimit *MoneyLimit `json:"monthly_deposit_limit,omitempty"`
	DailyBetLimit       *MoneyLimit `json:"daily_bet_limit,omitempty"`
	WeeklyBetLimit      *MoneyLimit `json:"weekly_bet_limit,omitempty"`
	MonthlyBetLimit     *MoneyLimit `json:"monthly_bet_limit,omitempty"`
	DailyLossLimit      *MoneyLimit `json:"daily_loss_limit,omitempty"`
	WeeklyLossLimit     *MoneyLimit `json:"weekly_loss_limit,omitempty"`
	MonthlyLossLimit    *MoneyLimit `json:"monthly_loss_limit,omitempty"`
	SessionTimeLimit    *TimeLimit `json:"session_time_limit,omitempty"`
	SelfExclusion       bool       `json:"self_exclusion" db:"self_exclusion"`
	SelfExclusionUntil  *time.Time `json:"self_exclusion_until,omitempty" db:"self_exclusion_until"`
	UpdatedAt           time.Time  `json:"updated_at" db:"updated_at"`
}

type SetLimitsRequest struct {
	UserID              int64   `json:"user_id"`
	DailyDepositLimit   *string `json:"daily_deposit_limit,omitempty"`
	WeeklyDepositLimit  *string `json:"weekly_deposit_limit,omitempty"`
	MonthlyDepositLimit *string `json:"monthly_deposit_limit,omitempty"`
	DailyBetLimit       *string `json:"daily_bet_limit,omitempty"`
	WeeklyBetLimit      *string `json:"weekly_bet_limit,omitempty"`
	MonthlyBetLimit     *string `json:"monthly_bet_limit,omitempty"`
	DailyLossLimit      *string `json:"daily_loss_limit,omitempty"`
	WeeklyLossLimit     *string `json:"weekly_loss_limit,omitempty"`
	MonthlyLossLimit    *string `json:"monthly_loss_limit,omitempty"`
	SessionTimeMinutes  *int    `json:"session_time_minutes,omitempty"`
	SelfExclusion       *bool   `json:"self_exclusion,omitempty"`
	SelfExclusionUntil  *time.Time `json:"self_exclusion_until,omitempty"`
}
