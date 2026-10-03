package client

import (
	"context"

	"github.com/shopspring/decimal"
)

// WalletAPI is the minimal wallet client surface used by services.
//
// Reservation lifecycle (withdrawal):
//
//	LockFunds   reserve funds (available -> locked), idempotent per reference
//	SettleFunds book the reservation as a debit after the money left the platform
//	UnlockFunds return the reservation when the operation did not happen
//
// SettleFunds is NOT "unlock + debit": the amount already left the available
// balance at lock time, so debiting again would charge the player twice.
type WalletAPI interface {
	GetBalance(ctx context.Context, userID int64, currency string) (*Balance, error)
	CreditWallet(ctx context.Context, req CreditRequest) (*CreditResult, error)
	LockFunds(ctx context.Context, req LockRequest) (*LockResult, error)
	UnlockFunds(ctx context.Context, req UnlockRequest) error
	SettleFunds(ctx context.Context, req SettleRequest) (*DebitResult, error)
	Close() error
}

// UserAPI is the minimal user client surface used by services.
type UserAPI interface {
	GetKYCLevel(ctx context.Context, userID int64) (int, error)
	GetUserStatus(ctx context.Context, userID int64) (string, error)
	GetUserInfo(ctx context.Context, userID int64) (*UserInfo, error)
	Close() error
}

// NOWPaymentsAPI is the minimal NOWPayments client surface used by services.
type NOWPaymentsAPI interface {
	VerifyWebhookSignature(payload []byte, signature string) bool
	// Payment flow
	CreatePayment(ctx context.Context, req CreatePaymentRequest) (*CreatePaymentResponse, error)
	GetCurrencies(ctx context.Context) (*CurrenciesResponse, error)
	GetEstimatedPrice(ctx context.Context, amount decimal.Decimal, fromCurrency, toCurrency string) (*EstimatedPriceResponse, error)
	// Withdrawal flow
	CreatePayout(ctx context.Context, req CreatePayoutRequest) (*CreatePayoutResponse, error)
}
