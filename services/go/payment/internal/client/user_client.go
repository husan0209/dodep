package client

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"
	userpb "github.com/opus-casino/proto/gen/go/user/v1"
)

// KYCCacheEntry represents a cached KYC level entry
type KYCCacheEntry struct {
	Level      int
	ExpiresAt  time.Time
}

// UserClient handles gRPC communication with User Service
type UserClient struct {
	client    userpb.UserServiceClient
	conn      *grpc.ClientConn
	logger    *zap.Logger
	tracer    trace.Tracer
	timeout   time.Duration
	kycCache  map[int64]KYCCacheEntry
	cacheTTL  time.Duration
}

// UserClientConfig holds configuration for User client
type UserClientConfig struct {
	Address        string
	Timeout        time.Duration
	EnableTLS      bool
	MaxRecvMsgSize int
	MaxSendMsgSize int
	KYCCacheTTL    time.Duration
}

// NewUserClient creates a new User gRPC client
func NewUserClient(cfg UserClientConfig, logger *zap.Logger, tracer trace.Tracer) (*UserClient, error) {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	cacheTTL := cfg.KYCCacheTTL
	if cacheTTL == 0 {
		cacheTTL = 5 * time.Minute // Default 5 minute cache
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(cfg.MaxRecvMsgSize),
			grpc.MaxCallSendMsgSize(cfg.MaxSendMsgSize),
		),
		grpc.WithChainUnaryInterceptor(
			userLoggingInterceptor(logger),
			userTracingInterceptor(tracer),
		),
	}

	conn, err := grpc.NewClient(cfg.Address, opts...)
	if err != nil {
		return nil, fmt.Errorf("connect user service: %w", err)
	}

	return &UserClient{
		client:   userpb.NewUserServiceClient(conn),
		conn:     conn,
		logger:   logger,
		tracer:   tracer,
		timeout:  timeout,
		kycCache: make(map[int64]KYCCacheEntry),
		cacheTTL: cacheTTL,
	}, nil
}

// userLoggingInterceptor creates a logging interceptor for gRPC calls
func userLoggingInterceptor(logger *zap.Logger) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		duration := time.Since(start)

		if err != nil {
			logger.Error("gRPC call failed",
				zap.String("service", "user"),
				zap.String("method", method),
				zap.Duration("duration", duration),
				zap.Error(err),
			)
		} else {
			logger.Debug("gRPC call completed",
				zap.String("service", "user"),
				zap.String("method", method),
				zap.Duration("duration", duration),
			)
		}

		return err
	}
}

// userTracingInterceptor creates a tracing interceptor for gRPC calls
func userTracingInterceptor(tracer trace.Tracer) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if tracer == nil {
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		ctx, span := tracer.Start(ctx, "user."+method)
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

// GetKYCLevel returns user's KYC level with caching
func (c *UserClient) GetKYCLevel(ctx context.Context, userID int64) (int, error) {
	// Check cache first
	if entry, ok := c.kycCache[userID]; ok {
		if time.Now().Before(entry.ExpiresAt) {
			c.logger.Debug("KYC level cache hit",
				zap.Int64("user_id", userID),
				zap.Int("level", entry.Level),
			)
			return entry.Level, nil
		}
		// Cache expired, remove entry
		delete(c.kycCache, userID)
	}

	// Fetch from service
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.GetUser(ctx, &userpb.GetUserRequest{
		UserId: &commonv1.UserId{Value: strconv.FormatInt(userID, 10)},
	})
	if err != nil {
		return 0, c.mapError(err, "GetKYCLevel")
	}

	if resp.User == nil {
		return 0, fmt.Errorf("GetKYCLevel: user not found in response")
	}

	level := int(resp.User.KycLevel)

	// Cache the result
	c.kycCache[userID] = KYCCacheEntry{
		Level:     level,
		ExpiresAt: time.Now().Add(c.cacheTTL),
	}

	c.logger.Debug("KYC level cached",
		zap.Int64("user_id", userID),
		zap.Int("level", level),
		zap.Duration("ttl", c.cacheTTL),
	)

	return level, nil
}

// InvalidateKYCCache invalidates the cached KYC level for a user
func (c *UserClient) InvalidateKYCCache(userID int64) {
	delete(c.kycCache, userID)
	c.logger.Debug("KYC level cache invalidated",
		zap.Int64("user_id", userID),
	)
}

// GetUserStatus returns user's status
func (c *UserClient) GetUserStatus(ctx context.Context, userID int64) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.GetUser(ctx, &userpb.GetUserRequest{
		UserId: &commonv1.UserId{Value: strconv.FormatInt(userID, 10)},
	})
	if err != nil {
		return "", c.mapError(err, "GetUserStatus")
	}

	if resp.User == nil {
		return "", fmt.Errorf("GetUserStatus: user not found in response")
	}

	return resp.User.Status.String(), nil
}

// GetUserInfo returns user information
func (c *UserClient) GetUserInfo(ctx context.Context, userID int64) (*UserInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.GetUser(ctx, &userpb.GetUserRequest{
		UserId: &commonv1.UserId{Value: strconv.FormatInt(userID, 10)},
	})
	if err != nil {
		return nil, c.mapError(err, "GetUserInfo")
	}

	if resp.User == nil {
		return nil, fmt.Errorf("GetUserInfo: user not found in response")
	}

	id, _ := strconv.ParseInt(resp.User.Id.Value, 10, 64)

	return &UserInfo{
		UserID:    id,
		KYCLevel:  int(resp.User.KycLevel),
		Status:    resp.User.Status.String(),
		Email:     resp.User.Email,
		CreatedAt: resp.User.CreatedAt.AsTime(),
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
