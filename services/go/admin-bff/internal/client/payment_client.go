package client

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"
	paymentv1 "github.com/opus-casino/proto/gen/go/payment/v1"
)

type PaymentClient struct {
	client  paymentv1.PaymentServiceClient
	conn    *grpc.ClientConn
	timeout time.Duration
}

func NewPaymentClient(address string, timeout time.Duration) (*PaymentClient, error) {
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
		return nil, fmt.Errorf("connect payment service: %w", err)
	}
	return &PaymentClient{client: paymentv1.NewPaymentServiceClient(conn), conn: conn, timeout: timeout}, nil
}

func (c *PaymentClient) Close() error { return c.conn.Close() }

func (c *PaymentClient) ListDeposits(ctx context.Context, status commonv1.TransactionStatus, pageSize int32, cursor string) ([]*paymentv1.Deposit, *commonv1.PageResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req := &paymentv1.ListDepositsRequest{Pagination: &commonv1.PageRequest{PageSize: pageSize, Cursor: cursor}}
	if status != commonv1.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED {
		req.Status = &status
	}
	resp, err := c.client.ListDeposits(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("list deposits: %w", err)
	}
	return resp.Deposits, resp.Pagination, nil
}

func (c *PaymentClient) ListWithdrawals(ctx context.Context, status commonv1.TransactionStatus, pageSize int32, cursor string) ([]*paymentv1.Withdrawal, *commonv1.PageResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req := &paymentv1.ListWithdrawalsRequest{Pagination: &commonv1.PageRequest{PageSize: pageSize, Cursor: cursor}}
	if status != commonv1.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED {
		req.Status = &status
	}
	resp, err := c.client.ListWithdrawals(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("list withdrawals: %w", err)
	}
	return resp.Withdrawals, resp.Pagination, nil
}

func (c *PaymentClient) GetDeposit(ctx context.Context, id string) (*paymentv1.Deposit, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	resp, err := c.client.GetDeposit(ctx, &paymentv1.GetDepositRequest{DepositId: id})
	if err != nil {
		return nil, fmt.Errorf("get deposit: %w", err)
	}
	return resp.Deposit, nil
}

func (c *PaymentClient) GetWithdrawal(ctx context.Context, id string) (*paymentv1.Withdrawal, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	resp, err := c.client.GetWithdrawal(ctx, &paymentv1.GetWithdrawalRequest{WithdrawalId: id})
	if err != nil {
		return nil, fmt.Errorf("get withdrawal: %w", err)
	}
	return resp.Withdrawal, nil
}

func (c *PaymentClient) CancelWithdrawal(ctx context.Context, userID int64, id string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	_, err := c.client.CancelWithdrawal(ctx, &paymentv1.CancelWithdrawalRequest{
		UserId:       &commonv1.UserId{Value: strconv.FormatInt(userID, 10)},
		WithdrawalId: id,
	})
	if err != nil {
		return fmt.Errorf("cancel withdrawal: %w", err)
	}
	return nil
}
