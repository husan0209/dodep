package client

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	walletpb "github.com/platform/proto/gen/go/platform/wallet/v1"
)

// WalletClient handles gRPC communication with Wallet Service
type WalletClient struct {
	client walletpb.WalletServiceClient
	conn   *grpc.ClientConn
	logger *zap.Logger
}

// WalletClientConfig holds configuration for Wallet client
type WalletClientConfig struct {
	Address        string
	Timeout        time.Duration
	EnableTLS      bool
	MaxRecvMsgSize int
	MaxSendMsgSize int
}

// NewWalletClient creates a new Wallet gRPC client
func NewWalletClient(cfg WalletClientConfig, logger *zap.Logger) (*WalletClient, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(cfg.MaxRecvMsgSize),
			grpc.MaxCallSendMsgSize(cfg.MaxSendMsgSize),
		),
	}

	conn, err := grpc.NewClient(cfg.Address, opts...)
	if err != nil {
		return nil, fmt.Errorf("connect wallet service: %w", err)
	}

	return &WalletClient{
		client: walletpb.NewWalletServiceClient(conn),
		conn:   conn,
		logger: logger,
	}, nil
}

// Close closes the gRPC connection
func (c *WalletClient) Close() error {
	return c.conn.Close()
}

// Balance represents wallet balance info
type Balance struct {
	Available decimal.Decimal
	Locked    decimal.Decimal
	Total     decimal.Decimal
}

// CreditRequest represents a credit request
type CreditRequest struct {
	UserID         int64
	Currency       string
	Amount         decimal.Decimal
	IdempotencyKey string
	ReferenceType  string
	ReferenceID    string
}

// CreditResult represents a credit result
type CreditResult struct {
	TransactionID string
	NewBalance    decimal.Decimal
}

// LockRequest represents a lock funds request
type LockRequest struct {
	UserID         int64
	Currency       string
	Amount         decimal.Decimal
	IdempotencyKey string
}

// LockResult represents a lock result
type LockResult struct {
	LockID     string
	NewBalance decimal.Decimal
}

// FinalizeDebitRequest represents a finalize debit request
type FinalizeDebitRequest struct {
	UserID         int64
	Currency       string
	Amount         decimal.Decimal
	IdempotencyKey string
	ReferenceID    string
}

// DebitResult represents a debit result
type DebitResult struct {
	TransactionID string
}

// GetBalance returns user's available balance
func (c *WalletClient) GetBalance(ctx context.Context, userID int64, currency string) (*Balance, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.client.GetBalance(ctx, &walletpb.GetBalanceRequest{
		UserId:       userID,
		CurrencyCode: currency,
	})
	if err != nil {
		return nil, c.mapError(err, "GetBalance")
	}

	return &Balance{
		Available: parseDecimal(resp.Available),
		Locked:    parseDecimal(resp.Locked),
		Total:     parseDecimal(resp.Total),
	}, nil
}

// CreditWallet credits user's wallet (for deposits)
func (c *WalletClient) CreditWallet(ctx context.Context, req CreditRequest) (*CreditResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.client.Credit(ctx, &walletpb.CreditRequest{
		UserId:         req.UserID,
		CurrencyCode:   req.Currency,
		Amount:         req.Amount.String(),
		IdempotencyKey: req.IdempotencyKey,
		ReferenceType:  req.ReferenceType,
		ReferenceId:    req.ReferenceID,
	})
	if err != nil {
		return nil, c.mapError(err, "CreditWallet")
	}

	return &CreditResult{
		TransactionID: resp.TransactionId,
		NewBalance:    parseDecimal(resp.NewBalance),
	}, nil
}

// LockFunds locks funds for withdrawal
func (c *WalletClient) LockFunds(ctx context.Context, req LockRequest) (*LockResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.client.LockFunds(ctx, &walletpb.LockFundsRequest{
		UserId:         req.UserID,
		CurrencyCode:   req.Currency,
		Amount:         req.Amount.String(),
		IdempotencyKey: req.IdempotencyKey,
		ReferenceType:  "withdrawal",
	})
	if err != nil {
		return nil, c.mapError(err, "LockFunds")
	}

	return &LockResult{
		LockID:     resp.LockId,
		NewBalance: parseDecimal(resp.NewBalance),
	}, nil
}

// UnlockFunds unlocks funds after failed withdrawal
func (c *WalletClient) UnlockFunds(ctx context.Context, lockID string, idempotencyKey string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := c.client.UnlockFunds(ctx, &walletpb.UnlockFundsRequest{
		LockId:         lockID,
		IdempotencyKey: idempotencyKey,
	})
	return c.mapError(err, "UnlockFunds")
}

// FinalizeDebit finalizes withdrawal after successful payout
func (c *WalletClient) FinalizeDebit(ctx context.Context, req FinalizeDebitRequest) (*DebitResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.client.FinalizeDebit(ctx, &walletpb.FinalizeDebitRequest{
		UserId:         req.UserID,
		CurrencyCode:   req.Currency,
		Amount:         req.Amount.String(),
		IdempotencyKey: req.IdempotencyKey,
		ReferenceType:  "withdrawal",
		ReferenceId:    req.ReferenceID,
	})
	if err != nil {
		return nil, c.mapError(err, "FinalizeDebit")
	}

	return &DebitResult{
		TransactionID: resp.TransactionId,
	}, nil
}

// mapError converts gRPC errors to domain errors
func (c *WalletClient) mapError(err error, operation string) error {
	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok {
		return fmt.Errorf("%s: %w", operation, err)
	}

	switch st.Code() {
	case codes.NotFound:
		return fmt.Errorf("%s: not found: %s", operation, st.Message())
	case codes.InvalidArgument:
		return fmt.Errorf("%s: invalid argument: %s", operation, st.Message())
	case codes.FailedPrecondition:
		return fmt.Errorf("%s: failed precondition: %s", operation, st.Message())
	case codes.ResourceExhausted:
		return fmt.Errorf("%s: resource exhausted: %s", operation, st.Message())
	case codes.Unavailable:
		return fmt.Errorf("%s: service unavailable: %s", operation, st.Message())
	case codes.DeadlineExceeded:
		return fmt.Errorf("%s: deadline exceeded: %s", operation, st.Message())
	default:
		return fmt.Errorf("%s: %s", operation, st.Message())
	}
}

// parseDecimal parses a string to decimal
func parseDecimal(s string) decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return d
}
