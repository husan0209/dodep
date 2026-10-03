package client

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"
	walletpb "github.com/opus-casino/proto/gen/go/wallet/v1"
)

// WalletClient handles gRPC communication with Wallet Service
type WalletClient struct {
	client  walletpb.WalletCoreServiceClient
	conn    *grpc.ClientConn
	logger  *zap.Logger
	tracer  trace.Tracer
	timeout time.Duration
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
func NewWalletClient(cfg WalletClientConfig, logger *zap.Logger, tracer trace.Tracer) (*WalletClient, error) {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(cfg.MaxRecvMsgSize),
			grpc.MaxCallSendMsgSize(cfg.MaxSendMsgSize),
		),
		grpc.WithChainUnaryInterceptor(
			loggingInterceptor(logger),
			tracingInterceptor(tracer),
		),
	}

	conn, err := grpc.NewClient(cfg.Address, opts...)
	if err != nil {
		return nil, fmt.Errorf("connect wallet service: %w", err)
	}

	return &WalletClient{
		client:  walletpb.NewWalletCoreServiceClient(conn),
		conn:    conn,
		logger:  logger,
		tracer:  tracer,
		timeout: timeout,
	}, nil
}

// loggingInterceptor creates a logging interceptor for gRPC calls
func loggingInterceptor(logger *zap.Logger) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		duration := time.Since(start)

		if err != nil {
			logger.Error("gRPC call failed",
				zap.String("service", "wallet"),
				zap.String("method", method),
				zap.Duration("duration", duration),
				zap.Error(err),
			)
		} else {
			logger.Debug("gRPC call completed",
				zap.String("service", "wallet"),
				zap.String("method", method),
				zap.Duration("duration", duration),
			)
		}

		return err
	}
}

// tracingInterceptor creates a tracing interceptor for gRPC calls
func tracingInterceptor(tracer trace.Tracer) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if tracer == nil {
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		ctx, span := tracer.Start(ctx, "wallet."+method)
		defer span.End()

		// Inject trace context into metadata
		traceID := span.SpanContext().TraceID().String()
		spanID := span.SpanContext().SpanID().String()
		md := metadata.New(map[string]string{
			"x-trace-id": traceID,
			"x-span-id":  spanID,
		})
		ctx = metadata.NewOutgoingContext(ctx, md)

		return invoker(ctx, method, req, reply, cc, opts...)
	}
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

// LockRequest reserves funds for a business operation.
// Reference must be stable across retries (e.g. "withdrawal:<uuid>"): the
// reservation is idempotent per (user, reference_type, reference).
type LockRequest struct {
	UserID    int64
	Currency  string
	Amount    decimal.Decimal
	Reference string
	// ReferenceType selects the ledger counter-account, e.g. "withdrawal".
	ReferenceType string
}

// LockResult represents a lock result
type LockResult struct {
	// LockID is the wallet-core handle of the reservation (audit trail).
	LockID     string
	NewBalance decimal.Decimal
}

// UnlockRequest releases a reservation back to the available balance.
type UnlockRequest struct {
	UserID    int64
	Reference string
	// ReferenceType must match the value used at LockFunds time.
	ReferenceType string
}

// SettleRequest books a reservation as a completed debit after the reserved
// money left the platform (crypto payout confirmed by the provider).
type SettleRequest struct {
	UserID    int64
	Reference string
	// ReferenceType must match the value used at LockFunds time.
	ReferenceType string
	// IdempotencyKey scopes the settlement transaction (UUID).
	IdempotencyKey string
	Description    string
}

// DebitResult represents a debit result
type DebitResult struct {
	TransactionID string
}

// GetBalance returns user's available balance
func (c *WalletClient) GetBalance(ctx context.Context, userID int64, currency string) (*Balance, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.GetBalance(ctx, &walletpb.GetBalanceRequest{
		UserId:     &commonv1.UserId{Value: strconv.FormatInt(userID, 10)},
		WalletType: commonv1.WalletType_WALLET_TYPE_MAIN,
	})
	if err != nil {
		return nil, c.mapError(err, "GetBalance")
	}

	if resp.Balance == nil {
		return nil, fmt.Errorf("GetBalance: balance not found in response")
	}

	var available, locked, total string
	if resp.Balance.Available != nil {
		available = resp.Balance.Available.Amount
	}
	if resp.Balance.Locked != nil {
		locked = resp.Balance.Locked.Amount
	}
	if resp.Balance.Total != nil {
		total = resp.Balance.Total.Amount
	}

	return &Balance{
		Available: parseDecimal(available),
		Locked:    parseDecimal(locked),
		Total:     parseDecimal(total),
	}, nil
}

// CreditWallet credits user's wallet (for deposits)
func (c *WalletClient) CreditWallet(ctx context.Context, req CreditRequest) (*CreditResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.Credit(ctx, &walletpb.CreditRequest{
		UserId:         &commonv1.UserId{Value: strconv.FormatInt(req.UserID, 10)},
		WalletType:     commonv1.WalletType_WALLET_TYPE_MAIN,
		Amount:         &commonv1.Money{Amount: req.Amount.String(), Currency: req.Currency},
		ReferenceId:    req.ReferenceID,
		ReferenceType:  req.ReferenceType,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, c.mapError(err, "CreditWallet")
	}

	var newBalance string
	if resp.NewBalance != nil {
		newBalance = resp.NewBalance.Amount
	}

	var txId string
	if resp.Transaction != nil && resp.Transaction.Id != nil {
		txId = resp.Transaction.Id.Value
	}

	return &CreditResult{
		TransactionID: txId,
		NewBalance:    parseDecimal(newBalance),
	}, nil
}

// LockFunds reserves funds (available -> locked) for a business reference.
// Retries with the same reference return the existing reservation.
func (c *WalletClient) LockFunds(ctx context.Context, req LockRequest) (*LockResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	reference := strings.TrimSpace(req.Reference)
	if reference == "" {
		return nil, fmt.Errorf("LockFunds: reference is required")
	}
	referenceType := strings.TrimSpace(req.ReferenceType)
	if referenceType == "" {
		return nil, fmt.Errorf("LockFunds: reference type is required")
	}

	resp, err := c.client.Lock(ctx, &walletpb.LockRequest{
		UserId:         &commonv1.UserId{Value: strconv.FormatInt(req.UserID, 10)},
		WalletType:     commonv1.WalletType_WALLET_TYPE_MAIN,
		Amount:         &commonv1.Money{Amount: req.Amount.String(), Currency: req.Currency},
		ReferenceId:    reference,
		ReferenceType:  referenceType,
		IdempotencyKey: reference,
	})
	if err != nil {
		return nil, c.mapError(err, "LockFunds")
	}

	var newBalance string
	if resp.NewAvailable != nil {
		newBalance = resp.NewAvailable.Amount
	}

	return &LockResult{
		LockID:     resp.LockId,
		NewBalance: parseDecimal(newBalance),
	}, nil
}

// UnlockFunds releases a reservation. Idempotent: replaying a compensation
// for an already released reservation succeeds without moving money again.
func (c *WalletClient) UnlockFunds(ctx context.Context, req UnlockRequest) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	reference := strings.TrimSpace(req.Reference)
	referenceType := strings.TrimSpace(req.ReferenceType)
	if reference == "" || referenceType == "" {
		return fmt.Errorf("UnlockFunds: reference and reference type are required")
	}

	_, err := c.client.Unlock(ctx, &walletpb.UnlockRequest{
		UserId:        &commonv1.UserId{Value: strconv.FormatInt(req.UserID, 10)},
		ReferenceId:   reference,
		ReferenceType: referenceType,
	})
	return c.mapError(err, "UnlockFunds")
}

// SettleFunds books a reservation as a completed debit. Use it when the
// reserved money actually left the platform (e.g. the crypto payout was
// confirmed): the locked amount is written off instead of being unlocked and
// debited a second time.
func (c *WalletClient) SettleFunds(ctx context.Context, req SettleRequest) (*DebitResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	reference := strings.TrimSpace(req.Reference)
	referenceType := strings.TrimSpace(req.ReferenceType)
	if reference == "" || referenceType == "" {
		return nil, fmt.Errorf("SettleFunds: reference and reference type are required")
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return nil, fmt.Errorf("SettleFunds: idempotency key is required")
	}

	resp, err := c.client.ConsumeLock(ctx, &walletpb.ConsumeLockRequest{
		UserId:         &commonv1.UserId{Value: strconv.FormatInt(req.UserID, 10)},
		ReferenceId:    reference,
		ReferenceType:  referenceType,
		IdempotencyKey: req.IdempotencyKey,
		Description:    req.Description,
	})
	if err != nil {
		return nil, c.mapError(err, "SettleFunds")
	}

	var txId string
	if resp.Transaction != nil && resp.Transaction.Id != nil {
		txId = resp.Transaction.Id.Value
	}

	return &DebitResult{TransactionID: txId}, nil
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
