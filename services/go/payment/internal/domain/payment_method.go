package domain

import (
	"time"

	"github.com/google/uuid"
)

// PaymentMethodType mirrors payment.v1.PaymentMethodType (subset used here).
type PaymentMethodType string

const (
	PaymentMethodTypeUnspecified  PaymentMethodType = "unspecified"
	PaymentMethodTypeCreditCard   PaymentMethodType = "credit_card"
	PaymentMethodTypeDebitCard    PaymentMethodType = "debit_card"
	PaymentMethodTypeBankTransfer PaymentMethodType = "bank_transfer"
	PaymentMethodTypeEWallet      PaymentMethodType = "e_wallet"
	PaymentMethodTypeCrypto       PaymentMethodType = "crypto"
	PaymentMethodTypePrepaid      PaymentMethodType = "prepaid"
	PaymentMethodTypeMobile       PaymentMethodType = "mobile"
	PaymentMethodTypePIX          PaymentMethodType = "pix"
	PaymentMethodTypeUPI          PaymentMethodType = "upi"
)

// SavedPaymentMethod is a user-stored payout destination.
// Raw details are NEVER persisted: only the AES-256-GCM envelope
// (DetailsEncrypted) is stored; reads return a masked DisplayValue.
type SavedPaymentMethod struct {
	ID               uuid.UUID
	UserID           int64
	Type             PaymentMethodType
	Provider         string
	Nickname         string
	DisplayValue     string
	DetailsEncrypted string
	IsDefault        bool
	IsActive         bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
	LastUsedAt       *time.Time
}

// NewSavedPaymentMethod creates a method with defaults.
func NewSavedPaymentMethod() *SavedPaymentMethod {
	return &SavedPaymentMethod{
		ID:        uuid.New(),
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// TableName maps the model to the migration table.
func (SavedPaymentMethod) TableName() string { return "payment_methods" }
