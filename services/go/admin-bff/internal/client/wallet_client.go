package client

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"
	walletv1 "github.com/opus-casino/proto/gen/go/wallet/v1"
)

// WalletClient wraps the wallet-core gRPC client. The admin panel uses it for
// balance adjustments (manual credit/debit), which move real money and thus
// require TOTP confirmation at the handler layer and a full audit trail.
type WalletClient struct {
	client  walletv1.WalletCoreServiceClient
	conn    *grpc.ClientConn
	timeout time.Duration
}

// NewWalletClient creates a new gRPC client for the wallet core.
func NewWalletClient(address string, timeout time.Duration) (*WalletClient, error) {
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	conn, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultCallOptions(
			grpc.WaitForReady(true),
			grpc.MaxCallRecvMsgSize(4*1024*1024),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("connect wallet core: %w", err)
	}
	return &WalletClient{
		client:  walletv1.NewWalletCoreServiceClient(conn),
		conn:    conn,
		timeout: timeout,
	}, nil
}

// Close closes the underlying gRPC connection.
func (c *WalletClient) Close() error {
	return c.conn.Close()
}

// Credit adds funds to a user's main wallet. referenceType is typically
// "adjustment"; the idempotency key must be unique per operator action so a
// retried call cannot double-credit.
func (c *WalletClient) Credit(ctx context.Context, userID int64, amount, currency, description, idempotencyKey string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.Credit(ctx, &walletv1.CreditRequest{
		UserId:         userIDProto(userID),
		WalletType:     commonv1.WalletType_WALLET_TYPE_MAIN,
		Amount:         &commonv1.Money{Amount: amount, Currency: currency},
		ReferenceType:  "adjustment",
		IdempotencyKey: idempotencyKey,
		Description:    description,
	})
	if err != nil {
		return "", fmt.Errorf("wallet credit: %w", err)
	}
	if resp.Error != nil {
		return "", fmt.Errorf("wallet credit: %s", resp.Error.ErrorMessage)
	}
	if resp.NewBalance == nil {
		return "", nil
	}
	return resp.NewBalance.Amount, nil
}

// Debit removes funds from a user's main wallet. The wallet service rejects
// the debit when the balance is insufficient.
func (c *WalletClient) Debit(ctx context.Context, userID int64, amount, currency, description, idempotencyKey string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.Debit(ctx, &walletv1.DebitRequest{
		UserId:         userIDProto(userID),
		WalletType:     commonv1.WalletType_WALLET_TYPE_MAIN,
		Amount:         &commonv1.Money{Amount: amount, Currency: currency},
		ReferenceType:  "adjustment",
		IdempotencyKey: idempotencyKey,
		Description:    description,
	})
	if err != nil {
		return "", fmt.Errorf("wallet debit: %w", err)
	}
	if resp.Error != nil {
		return "", fmt.Errorf("wallet debit: %s", resp.Error.ErrorMessage)
	}
	if resp.NewBalance == nil {
		return "", nil
	}
	return resp.NewBalance.Amount, nil
}

// GetBalance returns the main-wallet available balance for a user.
func (c *WalletClient) GetBalance(ctx context.Context, userID int64, currency string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.GetBalance(ctx, &walletv1.GetBalanceRequest{
		UserId:     userIDProto(userID),
		WalletType: commonv1.WalletType_WALLET_TYPE_MAIN,
	})
	if err != nil {
		return "", fmt.Errorf("wallet get balance: %w", err)
	}
	if resp.Balance == nil || resp.Balance.Available == nil {
		return "", nil
	}
	return resp.Balance.Available.Amount, nil
}
