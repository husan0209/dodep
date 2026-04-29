package models

import (
	"encoding/json"
	"time"
)

type KycDocument struct {
	ID              string          `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PlayerID        int64           `gorm:"not null;index" json:"player_id"`
	Type            string          `gorm:"type:kyc_document_type;not null" json:"type"`
	Status          string          `gorm:"type:kyc_document_status;not null;default:'pending'" json:"status"`
	FileURL         string          `gorm:"type:varchar(512);not null" json:"file_url"`
	UploadedAt      time.Time       `gorm:"not null;default:now()" json:"uploaded_at"`
	ReviewedBy      *string         `gorm:"type:varchar(36)" json:"reviewed_by,omitempty"`
	ReviewedByName  *string         `gorm:"type:varchar(255)" json:"reviewed_by_name,omitempty"`
	ReviewedAt      *time.Time      `json:"reviewed_at,omitempty"`
	RejectionReason *string         `gorm:"type:varchar(500)" json:"rejection_reason,omitempty"`
	Notes           *string         `gorm:"type:text" json:"notes,omitempty"`
	ExpiresAt       *time.Time      `json:"expires_at,omitempty"`
	OcrData         json.RawMessage `gorm:"type:jsonb" json:"ocr_data,omitempty"`
	CreatedAt       time.Time       `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt       time.Time       `gorm:"not null;default:now()" json:"updated_at"`
}

type KycReview struct {
	ID               string     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	DocumentID       string     `gorm:"type:uuid;not null;index" json:"document_id"`
	PlayerID         int64      `gorm:"not null;index" json:"player_id"`
	PlayerEmail      string     `gorm:"type:varchar(255);not null" json:"player_email"`
	PlayerUsername   string     `gorm:"type:varchar(100);not null" json:"player_username"`
	PlayerGroup      string     `gorm:"type:varchar(50);not null;default:'standard'" json:"player_group"`
	DocumentType     string     `gorm:"type:kyc_document_type;not null" json:"document_type"`
	Priority         string     `gorm:"type:kyc_priority;not null;default:'low'" json:"priority"`
	Status           string     `gorm:"type:kyc_review_status;not null;default:'pending'" json:"status"`
	AssignedTo       *string    `gorm:"type:varchar(36)" json:"assigned_to,omitempty"`
	AssignedToName   *string    `gorm:"type:varchar(255)" json:"assigned_to_name,omitempty"`
	WaitTimeMinutes  int        `gorm:"not null;default:0" json:"wait_time_minutes"`
	ReviewedBy       *string    `gorm:"type:varchar(36)" json:"reviewed_by,omitempty"`
	ReviewedAt       *time.Time `json:"reviewed_at,omitempty"`
	DecisionReason   *string    `gorm:"type:varchar(500)" json:"decision_reason,omitempty"`
	CreatedAt        time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

type ScreeningResult struct {
	ID           string     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PlayerID     int64      `gorm:"not null;index" json:"player_id"`
	PlayerEmail  string     `gorm:"type:varchar(255);not null" json:"player_email"`
	Status       string     `gorm:"type:screening_status;not null;default:'clear'" json:"status"`
	MatchedLists []string   `gorm:"type:text[]" json:"matched_lists"`
	MatchScore   int        `gorm:"not null;default:0" json:"match_score"`
	ScreenedAt   time.Time  `gorm:"not null;default:now()" json:"screened_at"`
	NextScreenAt *time.Time `json:"next_screen_at,omitempty"`
	ScreenedBy   string     `gorm:"type:varchar(36);not null;default:'system'" json:"screened_by"`
	ReviewedBy   *string    `gorm:"type:varchar(36)" json:"reviewed_by,omitempty"`
	ReviewedAt   *time.Time `json:"reviewed_at,omitempty"`
	ReviewNotes  *string    `gorm:"type:text" json:"review_notes,omitempty"`
	CreatedAt    time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

type SofRequest struct {
	ID               string     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PlayerID         int64      `gorm:"not null;index" json:"player_id"`
	PlayerEmail      string     `gorm:"type:varchar(255);not null" json:"player_email"`
	TriggerType      string     `gorm:"type:varchar(50);not null;default:'manual'" json:"trigger_type"`
	ThresholdAmount  *string    `gorm:"type:numeric(18,2)" json:"threshold_amount,omitempty"`
	PeriodDays       *int       `json:"period_days,omitempty"`
	Status           string     `gorm:"type:sof_status;not null;default:'open'" json:"status"`
	DeadlineAt       time.Time  `gorm:"not null" json:"deadline_at"`
	ReviewedBy       *string    `gorm:"type:varchar(36)" json:"reviewed_by,omitempty"`
	ReviewedAt       *time.Time `json:"reviewed_at,omitempty"`
	Notes            *string    `gorm:"type:text" json:"notes,omitempty"`
	CreatedAt        time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

type SofDocument struct {
	ID         string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	RequestID  string    `gorm:"type:uuid;not null;index" json:"request_id"`
	Type       string    `gorm:"type:sof_document_type;not null" json:"type"`
	FileURL    string    `gorm:"type:varchar(512);not null" json:"file_url"`
	UploadedAt time.Time `gorm:"not null;default:now()" json:"uploaded_at"`
}

type RgAlert struct {
	ID              string          `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PlayerID        int64           `gorm:"not null;index" json:"player_id"`
	PlayerEmail     string          `gorm:"type:varchar(255);not null" json:"player_email"`
	AlertType       string          `gorm:"type:rg_alert_type;not null" json:"alert_type"`
	Severity        string          `gorm:"type:rg_alert_severity;not null;default:'medium'" json:"severity"`
	Details         json.RawMessage `gorm:"type:jsonb;not null;default:'{}'" json:"details"`
	AcknowledgedBy  *string         `gorm:"type:varchar(36)" json:"acknowledged_by,omitempty"`
	AcknowledgedAt  *time.Time      `json:"acknowledged_at,omitempty"`
	CreatedAt       time.Time       `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt       time.Time       `gorm:"not null;default:now()" json:"updated_at"`
}

type RgLimit struct {
	ID                           string     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PlayerID                     int64      `gorm:"not null;uniqueIndex" json:"player_id"`
	DepositLimitDaily            *string    `gorm:"type:numeric(18,2)" json:"deposit_limit_daily,omitempty"`
	DepositLimitWeekly           *string    `gorm:"type:numeric(18,2)" json:"deposit_limit_weekly,omitempty"`
	DepositLimitMonthly          *string    `gorm:"type:numeric(18,2)" json:"deposit_limit_monthly,omitempty"`
	LossLimit                    *string    `gorm:"type:numeric(18,2)" json:"loss_limit,omitempty"`
	WagerLimitDaily              *string    `gorm:"type:numeric(18,2)" json:"wager_limit_daily,omitempty"`
	SessionTimeLimitMinutes      *int       `json:"session_time_limit_minutes,omitempty"`
	RealityCheckFrequencyMinutes *int       `json:"reality_check_frequency_minutes,omitempty"`
	SelfExclusionUntil         *time.Time `json:"self_exclusion_until,omitempty"`
	CoolOffUntil               *time.Time `json:"cool_off_until,omitempty"`
	CreatedAt                  time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt                  time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

type KycTeamMetric struct {
	ID                    string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OfficerID             string    `gorm:"type:varchar(36);not null" json:"officer_id"`
	OfficerName           string    `gorm:"type:varchar(255);not null" json:"officer_name"`
	MetricDate            time.Time `gorm:"type:date;not null" json:"metric_date"`
	ReviewedCount         int       `gorm:"not null;default:0" json:"reviewed_count"`
	AvgReviewTimeMinutes  int       `gorm:"not null;default:0" json:"avg_review_time_minutes"`
	ApproveCount          int       `gorm:"not null;default:0" json:"approve_count"`
	RejectCount           int       `gorm:"not null;default:0" json:"reject_count"`
	SlaBreachCount        int       `gorm:"not null;default:0" json:"sla_breach_count"`
}
