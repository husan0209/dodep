package client

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"
	bettingv1 "github.com/opus-casino/proto/gen/go/betting/v1"
)

// BettingClient wraps the betting-engine gRPC client.
type BettingClient struct {
	client  bettingv1.BettingEngineServiceClient
	conn    *grpc.ClientConn
	timeout time.Duration
}

// NewBettingClient creates a new gRPC client for the betting engine.
func NewBettingClient(address string, timeout time.Duration) (*BettingClient, error) {
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
		return nil, fmt.Errorf("connect betting engine: %w", err)
	}
	return &BettingClient{
		client:  bettingv1.NewBettingEngineServiceClient(conn),
		conn:    conn,
		timeout: timeout,
	}, nil
}

// Close closes the underlying gRPC connection.
func (c *BettingClient) Close() error {
	return c.conn.Close()
}

func betIDProto(id string) *commonv1.BetId {
	return &commonv1.BetId{Value: id}
}

// GetBet retrieves a single bet by ID.
func (c *BettingClient) GetBet(ctx context.Context, betID string) (*bettingv1.Bet, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	resp, err := c.client.GetBet(ctx, &bettingv1.GetBetRequest{BetId: betIDProto(betID)})
	if err != nil {
		return nil, fmt.Errorf("get bet: %w", err)
	}
	return resp.Bet, nil
}

// CancelBet voids a pending bet.
func (c *BettingClient) CancelBet(ctx context.Context, betID string, userID int64, reason string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	_, err := c.client.CancelBet(ctx, &bettingv1.CancelBetRequest{
		BetId:  betIDProto(betID),
		UserId: userIDProto(userID),
		Reason: reason,
	})
	if err != nil {
		return fmt.Errorf("cancel bet: %w", err)
	}
	return nil
}

// SettleBet settles a bet with the given result.
func (c *BettingClient) SettleBet(ctx context.Context, betID string, result bettingv1.BetResult, actualWin *commonv1.Money, settlementRef string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req := &bettingv1.SettleBetRequest{
		BetId:                betIDProto(betID),
		Result:               result,
		SettlementReference: settlementRef,
	}
	if actualWin != nil {
		req.ActualWin = actualWin
	}
	_, err := c.client.SettleBet(ctx, req)
	if err != nil {
		return fmt.Errorf("settle bet: %w", err)
	}
	return nil
}

// GetOdds returns current odds for an event and optional market.
func (c *BettingClient) GetOdds(ctx context.Context, eventID, marketID string) (*bettingv1.GetOddsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req := &bettingv1.GetOddsRequest{EventId: eventID}
	if marketID != "" {
		req.MarketId = &marketID
	}
	resp, err := c.client.GetOdds(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get odds: %w", err)
	}
	return resp, nil
}

// GetUserBets returns paginated bets for a user.
func (c *BettingClient) GetUserBets(ctx context.Context, userID int64, status commonv1.BetStatus, betType commonv1.BetType, pageSize int32, cursor string) ([]*bettingv1.Bet, *commonv1.PageResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req := &bettingv1.GetUserBetsRequest{
		UserId:     userIDProto(userID),
		Pagination: &commonv1.PageRequest{PageSize: pageSize, Cursor: cursor},
	}
	if status != commonv1.BetStatus_BET_STATUS_UNSPECIFIED {
		req.Status = &status
	}
	if betType != commonv1.BetType_BET_TYPE_UNSPECIFIED {
		req.BetType = &betType
	}
	resp, err := c.client.GetUserBets(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("get user bets: %w", err)
	}
	return resp.Bets, resp.Pagination, nil
}
