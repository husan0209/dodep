package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

type WithdrawalChecklist struct {
	KycVerified      bool `json:"kyc_verified"`
	BonusWagered     bool `json:"bonus_wagered"`
	MethodMatch      bool `json:"method_match"`
	NoChargeback     bool `json:"no_chargeback"`
	RiskScoreOK      bool `json:"risk_score_ok"`
	AmountWithinLimits bool `json:"amount_within_limits"`
	NoAMLFlags       bool `json:"no_aml_flags"`
}

func (wc WithdrawalChecklist) Value() (driver.Value, error) {
	return json.Marshal(wc)
}

func (wc *WithdrawalChecklist) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(b, wc)
}

type Withdrawal struct {
	ID               string              `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PlayerID         int64               `gorm:"not null;index" json:"player_id"`
	PlayerEmail      string              `gorm:"type:varchar(255);not null" json:"player_email"`
	Amount           string              `gorm:"type:numeric(18,2);not null" json:"amount"`
	Currency         string              `gorm:"type:char(3);not null;default:'USD'" json:"currency"`
	Method           string              `gorm:"type:varchar(50);not null" json:"method"`
	WalletOrAccount  *string             `gorm:"type:varchar(255)" json:"wallet_or_account,omitempty"`
	Gateway          *string             `gorm:"type:varchar(100)" json:"gateway,omitempty"`
	Status           string              `gorm:"type:withdrawal_status;not null;default:'pending'" json:"status"`
	RiskScore        int                 `gorm:"not null;default:0" json:"risk_score"`
	KycStatus        string              `gorm:"type:varchar(20);not null;default:'pending'" json:"kyc_status"`
	WagerStatus      string              `gorm:"type:varchar(20);not null;default:'pending'" json:"wager_status"`
	AmlStatus        string              `gorm:"type:varchar(20);not null;default:'pending'" json:"aml_status"`
	Checklist        WithdrawalChecklist `gorm:"type:jsonb;default:'{\"kyc_verified\":false,\"bonus_wagered\":false,\"method_match\":false,\"no_chargeback\":false,\"risk_score_ok\":false,\"amount_within_limits\":false,\"no_aml_flags\":false}'::jsonb" json:"checklist"`
	WaitHours        int                 `gorm:"not null;default:0" json:"wait_hours"`
	ApprovedBy       *string             `gorm:"type:varchar(36)" json:"approved_by,omitempty"`
	ApprovedByName   *string             `gorm:"type:varchar(255)" json:"approved_by_name,omitempty"`
	ApprovedAt       *time.Time          `json:"approved_at,omitempty"`
	RejectedBy       *string             `gorm:"type:varchar(36)" json:"rejected_by,omitempty"`
	RejectedByName   *string             `gorm:"type:varchar(255)" json:"rejected_by_name,omitempty"`
	RejectedAt       *time.Time          `json:"rejected_at,omitempty"`
	RejectReason     *string             `gorm:"type:varchar(255)" json:"reject_reason,omitempty"`
	HoldReason       *string             `gorm:"type:text" json:"hold_reason,omitempty"`
	CreatedAt        time.Time           `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt        time.Time           `gorm:"not null;default:now()" json:"updated_at"`
}
