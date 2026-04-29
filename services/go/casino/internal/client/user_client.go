package client

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"
	userv1 "github.com/opus-casino/proto/gen/go/user/v1"
)

// UserClientConfig holds gRPC config for User Service.
type UserClientConfig struct {
	Address string
	Timeout time.Duration
}

// UserServiceClient defines the subset of user service calls needed by casino.
type UserServiceClient interface {
	GetKYCLevel(ctx context.Context, userID int64) (int, error)
	GetCountry(ctx context.Context, userID int64) (string, error)
	IsSelfExcluded(ctx context.Context, userID int64) (bool, error)
}

type userGRPCClient struct {
	client userv1.UserServiceClient
	conn   *grpc.ClientConn
	log    *zap.Logger
	cfg    UserClientConfig
}

// NewUserClient creates a gRPC user client.
func NewUserClient(cfg UserClientConfig, log *zap.Logger) (UserServiceClient, error) {
	conn, err := grpc.Dial(
		cfg.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    10 * time.Second,
			Timeout: 5 * time.Second,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("casino: dial user service at %s: %w", cfg.Address, err)
	}

	log.Info("Casino: user service client created", zap.String("addr", cfg.Address))
	return &userGRPCClient{
		client: userv1.NewUserServiceClient(conn),
		conn:   conn,
		log:    log,
		cfg:    cfg,
	}, nil
}

// getUser is a shared helper that fetches the user profile.
func (c *userGRPCClient) getUser(ctx context.Context, userID int64) (*userv1.User, error) {
	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	resp, err := c.client.GetUser(ctx, &userv1.GetUserRequest{
		UserId: &commonv1.UserId{Value: fmt.Sprintf("%d", userID)},
	})
	if err != nil {
		return nil, fmt.Errorf("user: get user %d: %w", userID, err)
	}
	if resp.GetUser() == nil {
		return nil, fmt.Errorf("user: user %d not found", userID)
	}
	return resp.GetUser(), nil
}

// GetKYCLevel returns the user's current KYC verification level (0–4).
func (c *userGRPCClient) GetKYCLevel(ctx context.Context, userID int64) (int, error) {
	u, err := c.getUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	return int(u.GetKycLevel()), nil
}

// GetCountry returns the ISO 3166-1 alpha-2 country code for the user.
func (c *userGRPCClient) GetCountry(ctx context.Context, userID int64) (string, error) {
	u, err := c.getUser(ctx, userID)
	if err != nil {
		return "", err
	}
	return u.GetCountry(), nil
}

// IsSelfExcluded returns true if the user's status is SELF_EXCLUDED.
func (c *userGRPCClient) IsSelfExcluded(ctx context.Context, userID int64) (bool, error) {
	u, err := c.getUser(ctx, userID)
	if err != nil {
		return false, err
	}
	return u.GetStatus() == userv1.UserStatus_USER_STATUS_SELF_EXCLUDED, nil
}
