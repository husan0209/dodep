package client

import (
	"context"

	"github.com/shopspring/decimal"
)

// WalletAPI is the minimal wallet client surface used by services.
type WalletAPI interface {
	GetBalance(ctx context.Context, userID int64, currency string) (*Balance, error)
	CreditWallet(ctx context.Context, req CreditRequest) (*CreditResult, error)
	LockFunds(ctx context.Context, req LockRequest) (*LockResult, error)
	UnlockFunds(ctx context.Context, lockID string, idempotencyKey string) error
	FinalizeDebit(ctx context.Context, req FinalizeDebitRequest) (*DebitResult, error)
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

