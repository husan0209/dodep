package models

import (
	"time"
)

type SystemSetting struct {
	Key       string    `gorm:"primaryKey;type:varchar(100)" json:"key"`
	Value     string    `gorm:"type:text;not null" json:"value"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

func (SystemSetting) TableName() string { return "system_settings" }

type IPWhitelistEntry struct {
	ID        string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	IPAddress string    `gorm:"type:inet;not null" json:"ip_address"`
	Label     *string   `gorm:"type:varchar(255)" json:"label,omitempty"`
	IsGlobal  bool      `gorm:"not null;default:false" json:"is_global"`
	AdminID   *int64    `gorm:"index" json:"admin_id,omitempty"`
	CreatedBy string    `gorm:"type:varchar(36);not null" json:"created_by"`
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
}

func (IPWhitelistEntry) TableName() string { return "ip_whitelist" }

// CommunicationSuppression - A7.1 Addendum: suppress channels per player.
type CommunicationSuppression struct {
	ID        string     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PlayerID  string     `gorm:"type:varchar(36);not null;index" json:"player_id"`
	Reason    string     `gorm:"type:varchar(100);not null" json:"reason"` // unsubscribed|hard_bounce|spam_complaint|self_excluded|gdpr_erasure
	Channel   *string    `gorm:"type:varchar(20)" json:"channel,omitempty"` // nil = all channels; 'email'|'sms'|'push'
	AddedBy   string     `gorm:"type:varchar(100);not null" json:"added_by"` // 'player'|'auto'|admin_email
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `gorm:"not null;default:now()" json:"created_at"`
}

func (CommunicationSuppression) TableName() string { return "communication_suppressions" }

// FrequencyCap - A7.3 Addendum: per-channel sending limits config.
type FrequencyCap struct {
	ID            string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	EmailPerDay   int       `gorm:"not null;default:1" json:"email_per_day"`
	EmailPerWeek  int       `gorm:"not null;default:3" json:"email_per_week"`
	SMSPerDay     int       `gorm:"not null;default:0" json:"sms_per_day"`
	SMSPerWeek    int       `gorm:"not null;default:2" json:"sms_per_week"`
	PushPerDay    int       `gorm:"not null;default:3" json:"push_per_day"`
	PushPerHour   int       `gorm:"not null;default:1" json:"push_per_hour"`
	UpdatedAt     time.Time `gorm:"not null;default:now()" json:"updated_at"`
	UpdatedBy     string    `gorm:"type:varchar(36)" json:"updated_by"`
}

func (FrequencyCap) TableName() string { return "frequency_caps" }

// TOTPChallenge - pending TOTP verification for 2-step login flow.
type TOTPChallenge struct {
	ID        string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	AdminID   int64     `gorm:"not null;index" json:"admin_id"`
	Token     string    `gorm:"type:varchar(255);not null;uniqueIndex" json:"token"` // short-lived challenge token (bcrypt hash)
	ExpiresAt time.Time `gorm:"not null" json:"expires_at"`
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
}

func (TOTPChallenge) TableName() string { return "totp_challenges" }
