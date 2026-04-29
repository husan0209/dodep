package models

import (
	"encoding/json"
	"time"
)

type RegulatoryReport struct {
	ID            string          `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Jurisdiction  string          `gorm:"type:varchar(20);not null" json:"jurisdiction"`
	ReportType    string          `gorm:"type:varchar(50);not null" json:"report_type"`
	PeriodStart   time.Time       `gorm:"type:date;not null" json:"period_start"`
	PeriodEnd     time.Time       `gorm:"type:date;not null" json:"period_end"`
	Status        string          `gorm:"type:varchar(20);not null;default:'draft'" json:"status"`
	GeneratedAt   *time.Time      `json:"generated_at,omitempty"`
	SubmittedAt   *time.Time      `json:"submitted_at,omitempty"`
	SubmittedBy   *string         `gorm:"type:varchar(36)" json:"submitted_by,omitempty"`
	RegulatorRef  *string         `gorm:"type:varchar(100)" json:"regulator_ref,omitempty"`
	FileURL       *string         `gorm:"type:varchar(512)" json:"file_url,omitempty"`
	DataSnapshot  json.RawMessage `gorm:"type:jsonb" json:"data_snapshot,omitempty"`
	Notes         *string         `gorm:"type:text" json:"notes,omitempty"`
	CreatedAt     time.Time       `gorm:"not null;default:now()" json:"created_at"`
}

func (RegulatoryReport) TableName() string { return "regulatory_reports" }

type SARReport struct {
	ID              string          `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Jurisdiction    string          `gorm:"type:varchar(20);not null" json:"jurisdiction"`
	PlayerID        int64           `gorm:"not null;index" json:"player_id"`
	TriggerType     string          `gorm:"type:varchar(20);not null" json:"trigger_type"`
	TriggerAlertID  *string         `gorm:"type:varchar(36)" json:"trigger_alert_id,omitempty"`
	Status          string          `gorm:"type:varchar(20);not null;default:'draft'" json:"status"`
	AmountInvolved  *string         `gorm:"type:numeric(18,2)" json:"amount_involved,omitempty"`
	Currency        *string         `gorm:"type:char(3)" json:"currency,omitempty"`
	Description     string          `gorm:"type:text;not null" json:"description"`
	SupportingData  json.RawMessage `gorm:"type:jsonb" json:"supporting_data,omitempty"`
	AssignedTo      *string         `gorm:"type:varchar(36)" json:"assigned_to,omitempty"`
	InternalNotes   *string         `gorm:"type:text" json:"internal_notes,omitempty"`
	SubmittedAt     *time.Time      `json:"submitted_at,omitempty"`
	SubmittedBy     *string         `gorm:"type:varchar(36)" json:"submitted_by,omitempty"`
	RegulatorRef    *string         `gorm:"type:varchar(100)" json:"regulator_ref,omitempty"`
	TippingOffLock  bool            `gorm:"not null;default:true" json:"tipping_off_lock"`
	CreatedAt       time.Time       `gorm:"not null;default:now()" json:"created_at"`
}

func (SARReport) TableName() string { return "sar_reports" }

type PlayerComplaint struct {
	ID          string     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PlayerID    int64      `gorm:"not null;index" json:"player_id"`
	TicketID    *string    `gorm:"type:varchar(36)" json:"ticket_id,omitempty"`
	Category    string     `gorm:"type:varchar(30);not null" json:"category"`
	Description string     `gorm:"type:text;not null" json:"description"`
	Status      string     `gorm:"type:varchar(20);not null;default:'open'" json:"status"`
	ADRRef      *string    `gorm:"type:varchar(100)" json:"adr_ref,omitempty"`
	Resolution  *string    `gorm:"type:text" json:"resolution,omitempty"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	AssignedTo  *string    `gorm:"type:varchar(36)" json:"assigned_to,omitempty"`
	CreatedAt   time.Time  `gorm:"not null;default:now()" json:"created_at"`
}

func (PlayerComplaint) TableName() string { return "player_complaints" }

type JurisdictionGGR struct {
	Period       time.Time `gorm:"type:date;not null;primaryKey" json:"period"`
	Jurisdiction string    `gorm:"type:varchar(20);not null;primaryKey" json:"jurisdiction"`
	Currency     string    `gorm:"type:char(3);not null;primaryKey" json:"currency"`
	CasinoGGR    string    `gorm:"type:numeric(18,2);not null;default:0" json:"casino_ggr"`
	SportsGGR    string    `gorm:"type:numeric(18,2);not null;default:0" json:"sports_ggr"`
	LiveGGR      string    `gorm:"type:numeric(18,2);not null;default:0" json:"live_ggr"`
	TaxRate      *string   `gorm:"type:numeric(5,4)" json:"tax_rate,omitempty"`
	TaxAmount    *string   `gorm:"type:numeric(18,2)" json:"tax_amount,omitempty"`
}

func (JurisdictionGGR) TableName() string { return "jurisdiction_ggr" }

type TaxConfig struct {
	ID           string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Jurisdiction string    `gorm:"type:varchar(20);not null" json:"jurisdiction"`
	TaxType      string    `gorm:"type:varchar(20);not null" json:"tax_type"`
	TaxBase      string    `gorm:"type:varchar(20);not null" json:"tax_base"`
	Rate         string    `gorm:"type:numeric(5,4);not null" json:"rate"`
	Currency     string    `gorm:"type:char(3);not null" json:"currency"`
	EffectiveFrom time.Time `gorm:"type:date;not null" json:"effective_from"`
	EffectiveTo  *time.Time `gorm:"type:date" json:"effective_to,omitempty"`
	CreatedAt    time.Time `gorm:"not null;default:now()" json:"created_at"`
}

func (TaxConfig) TableName() string { return "tax_configs" }
