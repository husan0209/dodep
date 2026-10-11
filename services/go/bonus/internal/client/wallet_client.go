package client

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"
	walletv1 "github.com/opus-casino/proto/gen/go/wallet/v1"

	"github.com/opus-casino/bonus/internal/service"
	"github.com/opus-casino/bonus/internal/telemetry"
)

// WalletClientConfig holds the wallet-core gRPC connection settings.
type WalletClientConfig struct {
	Address string
	Timeout time.Duration
}

type grpcWalletClient struct {
	client walletv1.WalletCoreServiceClient
	conn   *grpc.ClientConn
	log    *zap.Logger
	cfg    WalletClientConfig
}

// NewWalletClient dials wallet-core (lazy connection) for bonus conversion credits.
// The returned client must be Closed on shutdown to release the connection.
func NewWalletClient(cfg WalletClientConfig, log *zap.Logger) (service.WalletClient, error) {
	if cfg.Address == "" {
		return nil, fmt.Errorf("bonus: wallet gRPC address is empty")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	if log == nil {
		log = zap.NewNop()
	}
	conn, err := grpc.NewClient(
		cfg.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()), // mTLS handled by Istio
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("bonus: dial wallet-core at %s: %w", cfg.Address, err)
	}
	log.Info("Bonus: wallet-core client created", zap.String("addr", cfg.Address))
	return &grpcWalletClient{client: walletv1.NewWalletCoreServiceClient(conn), conn: conn, log: log, cfg: cfg}, nil
}

// CreditBonusConversion credits converted bonus funds to the user's MAIN wallet.
// IdempotencyKey must be stable per bonus so wallet-core dedupes retries.
func (c *grpcWalletClient) CreditBonusConversion(ctx context.Context, in service.BonusCreditInput) error {
	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	// Span the outbound call so a bonus conversion can be followed across
	// services. The parent span (HTTP request or consumer event) is preserved.
	ctx, span := telemetry.StartClientSpan(ctx, "wallet.v1.WalletCoreService/Credit",
		attribute.String("peer.service", c.cfg.Address),
		attribute.String("wallet.amount", in.Amount.String()),
		attribute.String("wallet.currency", in.Currency),
		attribute.String("wallet.reference_type", "bonus"),
	)
	defer span.End()

	resp, err := c.client.Credit(ctx, &walletv1.CreditRequest{
		UserId:         &commonv1.UserId{Value: fmt.Sprintf("%d", in.UserID)},
		WalletType:     commonv1.WalletType_WALLET_TYPE_MAIN,
		Amount:         &commonv1.Money{Amount: in.Amount.StringFixed(8), Currency: in.Currency},
		ReferenceId:    in.ReferenceID,
		ReferenceType:  "bonus",
		IdempotencyKey: in.IdempotencyKey,
		Description:    "Bonus wagering conversion",
	})
	if err != nil {
		telemetry.EndSpan(span, err)
		return fmt.Errorf("bonus: wallet credit: %w", err)
	}
	if resp.GetError() != nil {
		err = fmt.Errorf("bonus: wallet credit rejected: %s", resp.GetError().GetErrorMessage())
		telemetry.EndSpan(span, err)
		return err
	}
	telemetry.EndSpan(span, nil)

	newBal, _ := decimal.NewFromString(resp.GetNewBalance().GetAmount())
	c.log.Info("Bonus: wagering conversion credited",
		zap.Int64("user_id", in.UserID),
		zap.String("reference_id", in.ReferenceID),
		zap.String("amount", in.Amount.StringFixed(2)),
		zap.String("new_balance", newBal.StringFixed(2)))
	return nil
}

// Close closes the underlying gRPC connection.
func (c *grpcWalletClient) Close() error {
	return c.conn.Close()
}
