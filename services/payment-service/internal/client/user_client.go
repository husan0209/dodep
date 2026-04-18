package client

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	userpb "github.com/platform/proto/gen/go/platform/user/v1"
)

// UserClient handles gRPC communication with User Service
type UserClient struct {
	client userpb.UserServiceClient
	conn   *grpc.ClientConn
	logger *zap.Logger
}

// UserClientConfig holds configuration for User client
type UserClientConfig struct {
	Address        string
	Timeout        time.Duration
	EnableTLS      bool
	MaxRecvMsgSize int
	MaxSendMsgSize int
}

// NewUserClient creates a new User gRPC client
func NewUserClient(cfg UserClientConfig, logger *zap.Logger) (*UserClient, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(cfg.MaxRecvMsgSize),
			grpc.MaxCallSendMsgSize(cfg.MaxSendMsgSize),
		),
	}

	conn, err := grpc.NewClient(cfg.Address, opts...)
	if err != nil {
		return nil, fmt.Errorf("connect user service: %w", err)
	}

	return &UserClient{
		client: userpb.NewUserServiceClient(conn),
		conn:   conn,
		logger: logger,
	}, nil
}

// Close closes the gRPC connection
func (c *UserClient) Close() error {
	return c.conn.Close()
}

// UserInfo represents user information
type UserInfo struct {
	UserID    int64
	KYCLevel  int
	Status    string
	Email     string
	CreatedAt time.Time
}

// GetKYCLevel returns user's KYC level
func (c *UserClient) GetKYCLevel(ctx context.Context, userID int64) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.client.GetKYCLevel(ctx, &userpb.GetKYCLevelRequest{
		UserId: userID,
	})
	if err != nil {
		return 0, c.mapError(err, "GetKYCLevel")
	}

	return int(resp.Level), nil
}

// GetUserStatus returns user's status
func (c *UserClient) GetUserStatus(ctx context.Context, userID int64) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.client.GetUserStatus(ctx, &userpb.GetUserStatusRequest{
		UserId: userID,
	})
	if err != nil {
		return "", c.mapError(err, "GetUserStatus")
	}

	return resp.Status, nil
}

// GetUserInfo returns user information
func (c *UserClient) GetUserInfo(ctx context.Context, userID int64) (*UserInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.client.GetUser(ctx, &userpb.GetUserRequest{
		UserId: userID,
	})
	if err != nil {
		return nil, c.mapError(err, "GetUserInfo")
	}

	return &UserInfo{
		UserID:    resp.UserId,
		KYCLevel:  int(resp.KycLevel),
		Status:    resp.Status,
		Email:     resp.Email,
		CreatedAt: resp.CreatedAt.AsTime(),
	}, nil
}

// mapError converts gRPC errors to domain errors
func (c *UserClient) mapError(err error, operation string) error {
	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok {
		return fmt.Errorf("%s: %w", operation, err)
	}

	switch st.Code() {
	case codes.NotFound:
		return fmt.Errorf("%s: user not found: %s", operation, st.Message())
	case codes.InvalidArgument:
		return fmt.Errorf("%s: invalid argument: %s", operation, st.Message())
	case codes.FailedPrecondition:
		return fmt.Errorf("%s: failed precondition: %s", operation, st.Message())
	case codes.Unavailable:
		return fmt.Errorf("%s: service unavailable: %s", operation, st.Message())
	case codes.DeadlineExceeded:
		return fmt.Errorf("%s: deadline exceeded: %s", operation, st.Message())
	default:
		return fmt.Errorf("%s: %s", operation, st.Message())
	}
}
