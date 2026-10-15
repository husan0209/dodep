package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// Deposit mirrors the shared `payments` table owned by the payment service
// (migration 014). Admin finance reads deposits from here for local
// aggregations (summary, charts, reports); the live list endpoint is served
// from the payment service gRPC API, which is authoritative for PSP state.
//
// FiatAmount is the human-readable credit amount; CryptoCurrency/Address
// describe the deposit route for crypto rails.
type Deposit struct {
	ID             int64   `gorm:"column:id;primary_key" json:"id"`
	UUID           string  `gorm:"column:uuid;type:uuid;default:gen_random_uuid()" json:"uuid"`
	UserID         int64   `gorm:"column:user_id;not null;index" json:"user_id"`
	PaymentID      string  `gorm:"column:payment_id;not null" json:"payment_id"`
	IdempotencyKey string  `gorm:"column:idempotency_key;not null" json:"idempotency_key"`
	RequestedAmount string `gorm:"column:requested_amount;type:numeric(18,8);not null" json:"requested_amount"`
	ActualAmount   *string `gorm:"column:actual_amount;type:numeric(18,8)" json:"actual_amount,omitempty"`
	FiatAmount     string  `gorm:"column:fiat_amount;type:numeric(18,2);not null" json:"fiat_amount"`
	FiatCurrency   string  `gorm:"column:fiat_currency;type:char(3);not null;default:'USD'" json:"fiat_currency"`
	CryptoCurrency string  `gorm:"column:crypto_currency;type:varchar(20);not null" json:"crypto_currency"`
	PayAddress     string  `gorm:"column:pay_address;type:varchar(255);not null" json:"pay_address"`
	PayAmount      *string `gorm:"column:pay_amount;type:numeric(18,8)" json:"pay_amount,omitempty"`
	Status         string  `gorm:"column:status;not null;default:'pending'" json:"status"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null;default:now()" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;not null;default:now()" json:"updated_at"`
	CompletedAt    *time.Time `gorm:"column:completed_at" json:"completed_at,omitempty"`
	ExpiresAt      *time.Time `gorm:"column:expires_at" json:"expires_at,omitempty"`
}

// TableName pins the model to the shared payment deposits table.
func (Deposit) TableName() string { return "payments" }

// DepositTimeline is the per-deposit lifecycle projection used by the
// payments admin timeline view.
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
