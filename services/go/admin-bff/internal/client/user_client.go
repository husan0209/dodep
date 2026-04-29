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
	userv1 "github.com/opus-casino/proto/gen/go/user/v1"
)

// UserClient wraps the user-service gRPC client.
type UserClient struct {
	client  userv1.UserServiceClient
	conn    *grpc.ClientConn
	timeout time.Duration
}

// NewUserClient creates a new gRPC client for the user service.
func NewUserClient(address string, timeout time.Duration) (*UserClient, error) {
	if timeout == 0 {
		timeout = 5 * time.Second
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
		return nil, fmt.Errorf("connect user service: %w", err)
	}

	return &UserClient{
		client:  userv1.NewUserServiceClient(conn),
		conn:    conn,
		timeout: timeout,
	}, nil
}

// Close closes the underlying gRPC connection.
func (c *UserClient) Close() error {
	return c.conn.Close()
}

func userIDProto(id int64) *commonv1.UserId {
	return &commonv1.UserId{Value: strconv.FormatInt(id, 10)}
}

// GetUser retrieves a user by ID.
func (c *UserClient) GetUser(ctx context.Context, userID int64) (*userv1.User, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.GetUser(ctx, &userv1.GetUserRequest{UserId: userIDProto(userID)})
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return resp.User, nil
}

// GetUserByEmail retrieves a user by email.
func (c *UserClient) GetUserByEmail(ctx context.Context, email string) (*userv1.User, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.GetUserByEmail(ctx, &userv1.GetUserByEmailRequest{Email: email})
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return resp.User, nil
}

// UpdateUser updates user profile fields.
func (c *UserClient) UpdateUser(ctx context.Context, userID int64, req *userv1.UpdateUserRequest) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req.UserId = userIDProto(userID)
	_, err := c.client.UpdateUser(ctx, req)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

// DeleteUser soft-deletes a user.
func (c *UserClient) DeleteUser(ctx context.Context, userID int64, reason string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	_, err := c.client.DeleteUser(ctx, &userv1.DeleteUserRequest{
		UserId: userIDProto(userID),
		Reason: reason,
	})
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

// GetActivity retrieves user activity history.
func (c *UserClient) GetActivity(ctx context.Context, userID int64, pageSize int32, cursor string) ([]*userv1.ActivityEntry, *commonv1.PageResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.GetActivity(ctx, &userv1.GetActivityRequest{
		UserId:     userIDProto(userID),
		Pagination: &commonv1.PageRequest{PageSize: pageSize, Cursor: cursor},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("get activity: %w", err)
	}
	return resp.Activities, resp.Pagination, nil
}
