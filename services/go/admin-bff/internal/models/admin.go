package models

import (
	"time"

	"github.com/lib/pq"
)

type AdminUser struct {
	ID           int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	Email        string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	PasswordHash string         `gorm:"type:varchar(255);not null" json:"-"`
	FirstName    *string        `gorm:"type:varchar(100)" json:"first_name,omitempty"`
	LastName     *string        `gorm:"type:varchar(100)" json:"last_name,omitempty"`
	Role         string         `gorm:"type:varchar(50);not null;default:'viewer'" json:"role"`
	Status       string         `gorm:"type:varchar(50);not null;default:'active'" json:"status"`
	Permissions  pq.StringArray `gorm:"type:text[];default:'{}'" json:"permissions"`
	TOTPSecret   *string        `gorm:"type:text" json:"-"` // encrypted in Vault; nil = TOTP disabled
	TOTPEnabled  bool           `gorm:"not null;default:false" json:"totp_enabled"`
	IPWhitelist  pq.StringArray `gorm:"type:text[];default:'{}'" json:"ip_whitelist"`
	LastLoginAt  *time.Time     `json:"last_login_at,omitempty"`
	LastLoginIP  *string        `gorm:"type:varchar(45)" json:"last_login_ip,omitempty"`
	CreatedAt    time.Time      `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"not null;default:now()" json:"updated_at"`
}

func (AdminUser) TableName() string { return "admin_users" }

type AdminSession struct {
	ID                string     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	AdminID           int64      `gorm:"not null;index" json:"admin_id"`
	RefreshTokenHash  string     `gorm:"type:varchar(255);not null;index" json:"-"`
	IPAddress         *string    `gorm:"type:inet" json:"ip_address,omitempty"`
	UserAgent         *string    `gorm:"type:text" json:"user_agent,omitempty"`
	DeviceFingerprint *string    `gorm:"type:text" json:"device_fingerprint,omitempty"`
	CountryCode       *string    `gorm:"type:char(2)" json:"country_code,omitempty"`
	CreatedAt         time.Time  `gorm:"not null;default:now()" json:"created_at"`
	ExpiresAt         time.Time  `gorm:"not null" json:"expires_at"`
	RevokedAt         *time.Time `json:"revoked_at,omitempty"`
}

func (AdminSession) TableName() string { return "admin_sessions" }

type AuditLog struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	AdminID      *int64    `gorm:"index" json:"admin_id,omitempty"`
	AdminEmail   *string   `gorm:"type:varchar(255)" json:"admin_email,omitempty"`
	AdminRole    *string   `gorm:"type:varchar(50)" json:"admin_role,omitempty"`
	Action       string    `gorm:"type:varchar(100);not null" json:"action"`
	ResourceType string    `gorm:"type:varchar(100);not null" json:"resource_type"`
	ResourceID   *string   `gorm:"type:varchar(255)" json:"resource_id,omitempty"`
	Before       []byte    `gorm:"type:jsonb" json:"before,omitempty"`
	After        []byte    `gorm:"type:jsonb" json:"after,omitempty"`
	Details      []byte    `gorm:"type:jsonb" json:"details,omitempty"`
	Reason       *string   `gorm:"type:text" json:"reason,omitempty"`
	TraceID      *string   `gorm:"type:varchar(64)" json:"trace_id,omitempty"`
	IPAddress    *string   `gorm:"type:inet" json:"ip_address,omitempty"`
	UserAgent    *string   `gorm:"type:text" json:"user_agent,omitempty"`
	CreatedAt    time.Time `gorm:"not null;default:now()" json:"created_at"`
}

func (AuditLog) TableName() string { return "audit_logs" }
