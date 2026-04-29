package client

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"
	walletv1 "github.com/opus-casino/proto/gen/go/wallet/v1"

	"github.com/opus-casino/casino/internal/service"
)

// WalletClientConfig holds gRPC connection configuration.
type WalletClientConfig struct {
	Address string
	Timeout time.Duration
}

// walletGRPCClient wraps the gRPC wallet client.
type walletGRPCClient struct {
	client walletv1.WalletCoreServiceClient
	conn   *grpc.ClientConn
	log    *zap.Logger
	cfg    WalletClientConfig
}

// NewWalletClient creates a new gRPC wallet client.
func NewWalletClient(cfg WalletClientConfig, log *zap.Logger) (service.WalletClient, error) {
	conn, err := grpc.Dial(
		cfg.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("casino: dial wallet-core at %s: %w", cfg.Address, err)
	}

	log.Info("Casino: wallet-core client created", zap.String("addr", cfg.Address))
	return &walletGRPCClient{
		client: walletv1.NewWalletCoreServiceClient(conn),
		conn:   conn,
		log:    log,
		cfg:    cfg,
	}, nil
}

// GetBalance fetches the current real-money balance for a user.
func (c *walletGRPCClient) GetBalance(ctx context.Context, userID int64, currency string) (decimal.Decimal, error) {
	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	resp, err := c.client.GetBalance(ctx, &walletv1.GetBalanceRequest{
		UserId: &commonv1.UserId{Value: fmt.Sprintf("%d", userID)},
	})
	if err != nil {
		return decimal.Zero, fmt.Errorf("wallet: get balance: %w", err)
	}
	avail := resp.GetBalance().GetAvailable().GetAmount()
	bal, err := decimal.NewFromString(avail)
	if err != nil {
		return decimal.Zero, fmt.Errorf("wallet: parse balance %q: %w", avail, err)
	}
	return bal, nil
}

// Debit debits the user's wallet (casino bet).
func (c *walletGRPCClient) Debit(ctx context.Context, req service.DebitRequest) (service.DebitResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	resp, err := c.client.Debit(ctx, &walletv1.DebitRequest{
		UserId:         &commonv1.UserId{Value: fmt.Sprintf("%d", req.UserID)},
		Amount:         &commonv1.Money{Amount: req.Amount.StringFixed(8), Currency: req.Currency},
		ReferenceId:    req.ReferenceID,
		ReferenceType:  req.ReferenceType,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return service.DebitResult{}, fmt.Errorf("wallet: debit: %w", err)
	}
	if resp.GetError() != nil {
		return service.DebitResult{}, fmt.Errorf("wallet: debit error: %s", resp.GetError().GetErrorMessage())
	}

	newBal, _ := decimal.NewFromString(resp.GetNewBalance().GetAmount())
	return service.DebitResult{
		NewBalance:    newBal,
		TransactionID: resp.GetTransaction().GetId().GetValue(),
	}, nil
}

// Credit credits the user's wallet (casino win).
func (c *walletGRPCClient) Credit(ctx context.Context, req service.CreditRequest) (service.CreditResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	resp, err := c.client.Credit(ctx, &walletv1.CreditRequest{
		UserId:         &commonv1.UserId{Value: fmt.Sprintf("%d", req.UserID)},
		Amount:         &commonv1.Money{Amount: req.Amount.StringFixed(8), Currency: req.Currency},
		ReferenceId:    req.ReferenceID,
		ReferenceType:  req.ReferenceType,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return service.CreditResult{}, fmt.Errorf("wallet: credit: %w", err)
	}
	if resp.GetError() != nil {
		return service.CreditResult{}, fmt.Errorf("wallet: credit error: %s", resp.GetError().GetErrorMessage())
	}

	newBal, _ := decimal.NewFromString(resp.GetNewBalance().GetAmount())
	return service.CreditResult{
		NewBalance:    newBal,
		TransactionID: resp.GetTransaction().GetId().GetValue(),
	}, nil
}

// Rollback reverses a previous wallet transaction via Unlock.
func (c *walletGRPCClient) Rollback(ctx context.Context, req service.RollbackRequest) (service.RollbackResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	resp, err := c.client.Unlock(ctx, &walletv1.UnlockRequest{
		UserId:      &commonv1.UserId{Value: fmt.Sprintf("%d", req.UserID)},
		ReferenceId: req.RefTransID,
	})
	if err != nil {
		return service.RollbackResult{}, fmt.Errorf("wallet: rollback (unlock): %w", err)
	}
	if resp.GetError() != nil {
		return service.RollbackResult{}, fmt.Errorf("wallet: rollback error: %s", resp.GetError().GetErrorMessage())
	}

	newBal, _ := decimal.NewFromString(resp.GetNewAvailable().GetAmount())
	return service.RollbackResult{
		NewBalance:    newBal,
		TransactionID: req.RefTransID,
	}, nil
}

// Close closes the gRPC connection.
func (c *walletGRPCClient) Close() error {
	return c.conn.Close()
}
