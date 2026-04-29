package models

import (
	"encoding/json"
	"time"
)

type Chargeback struct {
	ID               string          `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PlayerID         int64           `gorm:"not null;index" json:"player_id"`
	PlayerEmail      string          `gorm:"type:varchar(255);not null" json:"player_email"`
	TransactionID    string          `gorm:"type:varchar(100);not null" json:"transaction_id"`
	Amount           string          `gorm:"type:numeric(18,2);not null" json:"amount"`
	Currency         string          `gorm:"type:char(3);not null;default:'USD'" json:"currency"`
	Gateway          string          `gorm:"type:varchar(100);not null" json:"gateway"`
	GatewayCbID      *string         `gorm:"type:varchar(255)" json:"gateway_cb_id,omitempty"`
	ReasonCode       *string         `gorm:"type:varchar(50)" json:"reason_code,omitempty"`
	ReasonText       *string         `gorm:"type:varchar(500)" json:"reason_text,omitempty"`
	Status           string          `gorm:"type:chargeback_status;not null;default:'received'" json:"status"`
	ReceivedAt       time.Time       `gorm:"not null;default:now()" json:"received_at"`
	DeadlineAt       *time.Time      `json:"deadline_at,omitempty"`
	ResolvedAt       *time.Time      `json:"resolved_at,omitempty"`
	AssignedTo       *string         `gorm:"type:varchar(36)" json:"assigned_to,omitempty"`
	AssignedToName   *string         `gorm:"type:varchar(255)" json:"assigned_to_name,omitempty"`
	FightEvidence    json.RawMessage `gorm:"type:jsonb;default:'[]'" json:"fight_evidence,omitempty"`
	Notes            *string          `gorm:"type:text" json:"notes,omitempty"`
	CreatedAt        time.Time        `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt        time.Time        `gorm:"not null;default:now()" json:"updated_at"`
}

type CryptoWallet struct {
	ID                   string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Coin                 string    `gorm:"type:varchar(20);not null" json:"coin"`
	WalletType           string    `gorm:"type:varchar(10);not null;check:wallet_type IN ('hot','cold')" json:"wallet_type"`
	Balance              string    `gorm:"type:numeric(18,8);not null;default:0" json:"balance"`
	Address              string    `gorm:"type:varchar(255);not null" json:"address"`
	DailyWithdrawalAvg   string    `gorm:"type:numeric(18,8);not null;default:0" json:"daily_withdrawal_avg"`
	ThresholdAmount      string    `gorm:"type:numeric(18,8);not null;default:0" json:"threshold_amount"`
	PendingDeposits      int       `gorm:"not null;default:0" json:"pending_deposits"`
	PendingWithdrawals   int       `gorm:"not null;default:0" json:"pending_withdrawals"`
	LastUpdated          time.Time `gorm:"not null;default:now()" json:"last_updated"`
	CreatedAt            time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt            time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

type P2PTransaction struct {
	ID                string     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PlayerID          int64      `gorm:"not null;index" json:"player_id"`
	PlayerEmail       string     `gorm:"type:varchar(255);not null" json:"player_email"`
	Type              string     `gorm:"type:p2p_type;not null" json:"type"`
	Amount            string     `gorm:"type:numeric(18,2);not null" json:"amount"`
	Currency          string     `gorm:"type:char(3);not null;default:'TRY'" json:"currency"`
	Method            string     `gorm:"type:p2p_method;not null" json:"method"`
	Status            string     `gorm:"type:p2p_status;not null;default:'pending'" json:"status"`
	ReceiptURL        *string    `gorm:"type:varchar(512)" json:"receipt_url,omitempty"`
	ConfirmedBy       *string    `gorm:"type:varchar(36)" json:"confirmed_by,omitempty"`
	ConfirmedByName   *string    `gorm:"type:varchar(255)" json:"confirmed_by_name,omitempty"`
	ConfirmedAt       *time.Time `json:"confirmed_at,omitempty"`
	Notes             *string    `gorm:"type:text" json:"notes,omitempty"`
	CreatedAt         time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt         time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

type ReconciliationRecord struct {
	ID                string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ReconDate         time.Time `gorm:"type:date;not null" json:"recon_date"`
	Gateway           string    `gorm:"type:varchar(100);not null" json:"gateway"`
	ExpectedBalance   string    `gorm:"type:numeric(18,2);not null;default:0" json:"expected_balance"`
	ActualBalance     string    `gorm:"type:numeric(18,2);not null;default:0" json:"actual_balance"`
	Difference        string    `gorm:"type:numeric(18,2);not null;default:0" json:"difference"`
	PendingTxCount    int       `gorm:"not null;default:0" json:"pending_tx_count"`
	FailedCallbacks   int       `gorm:"not null;default:0" json:"failed_callbacks"`
	ChargebackAmount  string    `gorm:"type:numeric(18,2);not null;default:0" json:"chargeback_amount"`
	Status            string    `gorm:"type:reconciliation_status;not null;default:'pending'" json:"status"`
	Notes             *string    `gorm:"type:text" json:"notes,omitempty"`
	CreatedAt         time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt         time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

type PaymentMethodConfig struct {
	ID                     string     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	CountryCode            string     `gorm:"type:char(2);not null" json:"country_code"`
	Method                 string     `gorm:"type:varchar(50);not null" json:"method"`
	Gateway                string     `gorm:"type:varchar(100);not null" json:"gateway"`
	EnabledDeposit         bool       `gorm:"not null;default:true" json:"enabled_deposit"`
	EnabledWithdrawal      bool       `gorm:"not null;default:true" json:"enabled_withdrawal"`
	MinDeposit             string     `gorm:"type:numeric(18,2);not null;default:0" json:"min_deposit"`
	MaxDeposit             *string    `gorm:"type:numeric(18,2)" json:"max_deposit,omitempty"`
	MinWithdrawal          string     `gorm:"type:numeric(18,2);not null;default:0" json:"min_withdrawal"`
	MaxWithdrawal          *string    `gorm:"type:numeric(18,2)" json:"max_withdrawal,omitempty"`
	FeePercent             string     `gorm:"type:numeric(5,4);not null;default:0" json:"fee_percent"`
	FeeFixed               string     `gorm:"type:numeric(18,2);not null;default:0" json:"fee_fixed"`
	Priority               int        `gorm:"not null;default:0" json:"priority"`
	TemporaryDisabledUntil *time.Time `json:"temporary_disabled_until,omitempty"`
	CreatedAt              time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt              time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}
