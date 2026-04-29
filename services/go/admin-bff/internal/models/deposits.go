package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

type Deposit struct {
	ID            string     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PlayerID      int64      `gorm:"not null;index" json:"player_id"`
	PlayerEmail   string     `gorm:"type:varchar(255);not null" json:"player_email"`
	Amount        string     `gorm:"type:numeric(18,2);not null" json:"amount"`
	Currency      string     `gorm:"type:char(3);not null;default:'USD'" json:"currency"`
	Method        string     `gorm:"type:varchar(50);not null" json:"method"`
	Gateway       string     `gorm:"type:varchar(100);not null" json:"gateway"`
	GatewayTxID   *string    `gorm:"type:varchar(255)" json:"gateway_tx_id,omitempty"`
	Status        string     `gorm:"type:deposit_status;not null;default:'pending'" json:"status"`
	RiskScore     int        `gorm:"not null;default:0" json:"risk_score"`
	CreatedAt     time.Time  `gorm:"not null;default:now()" json:"created_at"`
	ProcessedAt   *time.Time `json:"processed_at,omitempty"`
	CreditedAt    *time.Time `json:"credited_at,omitempty"`
	FailedAt      *time.Time `json:"failed_at,omitempty"`
	FailureReason *string    `gorm:"type:varchar(500)" json:"failure_reason,omitempty"`
	Notes         *string    `gorm:"type:text" json:"notes,omitempty"`
	UpdatedAt     time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

type DepositTimeline struct {
	Created    time.Time  `json:"created"`
	Sent       *time.Time `json:"sent,omitempty"`
	Callback   *time.Time `json:"callback,omitempty"`
	Credited   *time.Time `json:"credited,omitempty"`
	GatewayRaw *string    `json:"gateway_raw,omitempty"`
}

func (dt DepositTimeline) Value() (driver.Value, error) {
	return json.Marshal(dt)
}

func (dt *DepositTimeline) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(b, dt)
}
