package models

import (
	"encoding/json"
	"time"
)

type RiskAlert struct {
	ID              string          `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID          int64           `gorm:"not null;index" json:"user_id"`
	Category        string          `gorm:"type:varchar(50);not null" json:"category"`
	Severity        string          `gorm:"type:varchar(20);not null;default:'medium'" json:"severity"`
	Status          string          `gorm:"type:varchar(20);not null;default:'open'" json:"status"`
	RiskScore       int             `gorm:"not null;default:0" json:"risk_score"`
	Title           string          `gorm:"type:varchar(255);not null" json:"title"`
	Description     string          `gorm:"type:text;not null" json:"description"`
	Evidence        json.RawMessage `gorm:"type:jsonb;not null;default:'{}'" json:"evidence"`
	AssignedTo      *string         `gorm:"type:varchar(36)" json:"assigned_to,omitempty"`
	Resolution      *string         `gorm:"type:text" json:"resolution,omitempty"`
	DismissReason   *string         `gorm:"type:varchar(100)" json:"dismiss_reason,omitempty"`
	ResolvedAt      *time.Time      `json:"resolved_at,omitempty"`
	CreatedAt       time.Time       `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt       time.Time       `gorm:"not null;default:now()" json:"updated_at"`
}

type RiskRule struct {
	ID          string          `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name        string          `gorm:"type:varchar(255);not null" json:"name"`
	Description string          `gorm:"type:text" json:"description,omitempty"`
	RuleType    string          `gorm:"type:varchar(50);not null" json:"rule_type"`
	Action      string          `gorm:"type:varchar(20);not null" json:"action"`
	Priority    int             `gorm:"not null;default:100" json:"priority"`
	Enabled     bool            `gorm:"not null;default:true" json:"enabled"`
	Condition   json.RawMessage `gorm:"type:jsonb;not null;default:'{}'" json:"condition"`
	HitCount    int             `gorm:"not null;default:0" json:"hit_count"`
	CreatedBy   string          `gorm:"type:varchar(36);not null" json:"created_by"`
	CreatedAt   time.Time       `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt   time.Time       `gorm:"not null;default:now()" json:"updated_at"`
}

type RiskAuditLog struct {
	ID           string          `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	AdminID      string          `gorm:"type:varchar(36);not null;index" json:"admin_id"`
	AdminEmail   string          `gorm:"type:varchar(255);not null" json:"admin_email"`
	Action       string          `gorm:"type:varchar(100);not null" json:"action"`
	ResourceType string          `gorm:"type:varchar(50);not null" json:"resource_type"`
	ResourceID   string          `gorm:"type:varchar(36);not null" json:"resource_id"`
	OldValue     json.RawMessage `gorm:"type:jsonb" json:"old_value,omitempty"`
	NewValue     json.RawMessage `gorm:"type:jsonb" json:"new_value,omitempty"`
	IPAddress    string          `gorm:"type:varchar(45);not null" json:"ip_address"`
	CreatedAt    time.Time       `gorm:"not null;default:now()" json:"created_at"`
}

type RiskWatchlistEntry struct {
	ID         string     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ListType   string     `gorm:"type:varchar(20);not null;index" json:"list_type"`
	EntityType string     `gorm:"type:varchar(20);not null" json:"entity_type"`
	EntityID   string     `gorm:"type:varchar(255);not null" json:"entity_id"`
	Reason     string     `gorm:"type:text;not null" json:"reason"`
	AddedBy    string     `gorm:"type:varchar(36);not null" json:"added_by"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `gorm:"not null;default:now()" json:"created_at"`
}

type RiskRuleWhitelist struct {
	ID        string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	RuleID    string    `gorm:"type:uuid;not null;index" json:"rule_id"`
	UserID    int64     `gorm:"not null;index" json:"user_id"`
	Reason    string    `gorm:"type:text;not null" json:"reason"`
	AddedBy   string    `gorm:"type:varchar(36);not null" json:"added_by"`
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
}
