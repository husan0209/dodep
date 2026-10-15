# SKILL #17 — go-grpc-services.skill.md

```markdown
# go-grpc-services.skill.md
# GAMBLING PLATFORM — GO gRPC SERVICE PATTERNS
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: Go Business Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

Go services expose gRPC endpoints for internal service-to-service calls.
Go services also act as gRPC CLIENTS calling Rust services (wallet, betting).

Proto files: shared proto/ directory.
Code generation: buf.build.

# ============================================================
# SECTION 2: gRPC SERVER IMPLEMENTATION
# ============================================================

```go
package grpc

import (
    "context"
    "fmt"
    "net"

    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "google.golang.org/grpc/reflection"

    pb "github.com/platform/proto/gen/go/platform/user/v1"
    "github.com/platform/services/user-service/internal/domain"
    "github.com/platform/services/user-service/internal/service"
)

type UserGRPCServer struct {
    pb.UnimplementedUserServiceServer
    userSvc *service.UserService
}

func NewUserGRPCServer(userSvc *service.UserService) *UserGRPCServer {
    return &UserGRPCServer{userSvc: userSvc}
}

func (s *UserGRPCServer) GetUser(
    ctx context.Context,
    req *pb.GetUserRequest,
) (*pb.GetUserResponse, error) {
    if req.UserId <= 0 {
        return nil, status.Error(codes.InvalidArgument, "user_id must be positive")
    }

    user, err := s.userSvc.GetByID(ctx, req.UserId)
    if err != nil {
        return nil, mapDomainErrorToStatus(err)
    }

    return &pb.GetUserResponse{
        User: userToProto(user),
    }, nil
}

func (s *UserGRPCServer) UpdateKYCLevel(
    ctx context.Context,
    req *pb.UpdateKYCLevelRequest,
) (*pb.UpdateKYCLevelResponse, error) {
    if req.UserId <= 0 {
        return nil, status.Error(codes.InvalidArgument, "user_id must be positive")
    }
    if req.Level < 0 || req.Level > 4 {
        return nil, status.Error(codes.InvalidArgument, "level must be 0-4")
    }

    err := s.userSvc.UpdateKYCLevel(ctx, req.UserId, int(req.Level))
    if err != nil {
        return nil, mapDomainErrorToStatus(err)
    }

    return &pb.UpdateKYCLevelResponse{Success: true}, nil
}

func (s *UserGRPCServer) BlockUser(
    ctx context.Context,
    req *pb.BlockUserRequest,
) (*pb.BlockUserResponse, error) {
    err := s.userSvc.BlockUser(ctx, req.UserId, req.Reason, req.BlockedBy)
    if err != nil {
        return nil, mapDomainErrorToStatus(err)
    }

    return &pb.BlockUserResponse{Success: true}, nil
}
SERVER STARTUP
Go

func StartGRPCServer(ctx context.Context, port int, userSvc *service.UserService) error {
    listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
    if err != nil {
        return fmt.Errorf("listen: %w", err)
    }

    server := grpc.NewServer(
        grpc.UnaryInterceptor(chainUnaryInterceptors(
            recoveryInterceptor(),
            loggingInterceptor(),
            tracingInterceptor(),
            metricsInterceptor(),
        )),
    )

    pb.RegisterUserServiceServer(server, NewUserGRPCServer(userSvc))
    reflection.Register(server) // enable grpcurl in dev

    // Graceful shutdown
    go func() {
        <-ctx.Done()
        server.GracefulStop()
    }()

    log.Info().Int("port", port).Msg("gRPC server listening")
    return server.Serve(listener)
}
============================================================
SECTION 3: ERROR MAPPING
============================================================
Go

func mapDomainErrorToStatus(err error) error {
    switch {
    case errors.Is(err, domain.ErrUserNotFound):
        return status.Error(codes.NotFound, "User not found")
    case errors.Is(err, domain.ErrEmailExists):
        return status.Error(codes.AlreadyExists, "Email already registered")
    case errors.Is(err, domain.ErrForbidden):
        return status.Error(codes.PermissionDenied, err.Error())
    case errors.Is(err, domain.ErrSelfExcluded):
        return status.Error(codes.PermissionDenied, "User is self-excluded")
    case errors.Is(err, domain.ErrConflict):
        return status.Error(codes.Aborted, err.Error())
    case errors.Is(err, domain.ErrRateLimited):
        return status.Error(codes.ResourceExhausted, "Rate limited")
    default:
        var validationErr *domain.ValidationError
        if errors.As(err, &validationErr) {
            return status.Error(codes.InvalidArgument, validationErr.Error())
        }
        log.Error().Err(err).Msg("Internal gRPC error")
        return status.Error(codes.Internal, "Internal error")
    }
}
============================================================
SECTION 4: gRPC CLIENT
============================================================
Go

package client

import (
    "context"
    "fmt"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure" // mTLS handled by Istio
    "google.golang.org/grpc/keepalive"

    walletpb "github.com/platform/proto/gen/go/platform/wallet/v1"
)

type WalletClient struct {
    client walletpb.WalletServiceClient
    conn   *grpc.ClientConn
}

func NewWalletClient(address string) (*WalletClient, error) {
    conn, err := grpc.NewClient(address,
        grpc.WithTransportCredentials(insecure.NewCredentials()), // Istio handles mTLS
        grpc.WithKeepaliveParams(keepalive.ClientParameters{
            Time:                30 * time.Second,
            Timeout:             10 * time.Second,
            PermitWithoutStream: true,
        }),
        grpc.WithDefaultCallOptions(
            grpc.WaitForReady(true),
            grpc.MaxCallRecvMsgSize(4*1024*1024), // 4MB max message
        ),
        grpc.WithUnaryInterceptor(tracingClientInterceptor()),
    )
    if err != nil {
        return nil, fmt.Errorf("connect wallet service: %w", err)
    }

    return &WalletClient{
        client: walletpb.NewWalletServiceClient(conn),
        conn:   conn,
    }, nil
}

func (c *WalletClient) Close() error {
    return c.conn.Close()
}

// ── Typed client methods with error mapping ──

func (c *WalletClient) GetBalance(
    ctx context.Context, userID int64, currency string,
) (*Balance, error) {
    ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
    defer cancel()

    resp, err := c.client.GetBalance(ctx, &walletpb.GetBalanceRequest{
        UserId:       userID,
        CurrencyCode: currency,
    })
    if err != nil {
        return nil, mapWalletError(err)
    }

    return &Balance{
        Available: parseDecimal(resp.Available),
        Locked:    parseDecimal(resp.Locked),
        Total:     parseDecimal(resp.Total),
    }, nil
}

func (c *WalletClient) CreditWallet(
    ctx context.Context, userID int64, amount string,
    currency string, idempotencyKey string,
    refType string, refID int64,
) (*CreditResult, error) {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    resp, err := c.client.Credit(ctx, &walletpb.CreditRequest{
        UserId:         userID,
        CurrencyCode:   currency,
        Amount:         amount,
        IdempotencyKey: idempotencyKey,
        ReferenceType:  refType,
        ReferenceId:    refID,
    })
    if err != nil {
        return nil, mapWalletError(err)
    }

    return &CreditResult{
        TransactionID: resp.TransactionId,
        NewBalance:    parseDecimal(resp.NewBalance),
    }, nil
}

func mapWalletError(err error) error {
    st, ok := status.FromError(err)
    if !ok {
        return fmt.Errorf("wallet service: %w", err)
    }
    switch st.Code() {
    case codes.FailedPrecondition:
        return domain.ErrInsufficientBalance
    case codes.NotFound:
        return domain.ErrUserNotFound
    case codes.Unavailable, codes.DeadlineExceeded:
        return fmt.Errorf("wallet service unavailable: %s", st.Message())
    default:
        return fmt.Errorf("wallet service error: %s", st.Message())
    }
}
============================================================
SECTION 5: INTERCEPTORS
============================================================
Go

// Logging interceptor
func loggingInterceptor() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{},
        info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
    ) (interface{}, error) {
        start := time.Now()
        resp, err := handler(ctx, req)
        duration := time.Since(start)

        logger := log.With().
            Str("method", info.FullMethod).
            Dur("duration", duration).
            Logger()

        if err != nil {
            st, _ := status.FromError(err)
            logger.Warn().Str("code", st.Code().String()).Msg("gRPC request failed")
        } else {
            logger.Info().Msg("gRPC request completed")
        }

        return resp, err
    }
}

// Recovery interceptor (catch panics)
func recoveryInterceptor() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{},
        info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
    ) (resp interface{}, err error) {
        defer func() {
            if r := recover(); r != nil {
                log.Error().Interface("panic", r).Str("method", info.FullMethod).
                    Msg("Panic recovered in gRPC handler")
                err = status.Error(codes.Internal, "Internal error")
            }
        }()
        return handler(ctx, req)
    }
}

// Chain multiple interceptors
func chainUnaryInterceptors(interceptors ...grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{},
        info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
    ) (interface{}, error) {
        chain := handler
        for i := len(interceptors) - 1; i >= 0; i-- {
            next := chain
            interceptor := interceptors[i]
            chain = func(ctx context.Context, req interface{}) (interface{}, error) {
                return interceptor(ctx, req, info, func(ctx context.Context, req interface{}) (interface{}, error) {
                    return next(ctx, req)
                })
            }
        }
        return chain(ctx, req)
    }
}
============================================================
SECTION 6: PROTO ↔ DOMAIN CONVERSION
============================================================
Go

// ALWAYS convert between proto and domain types at the gRPC boundary.
// NEVER use proto types in service or repository layer.

func userToProto(user *domain.User) *pb.User {
    return &pb.User{
        Id:           user.ID,
        Uuid:         user.UUID.String(),
        Email:        user.Email,
        Status:       string(user.Status),
        KycLevel:     int32(user.KYCLevel),
        CountryCode:  user.CountryCode,
        CurrencyCode: user.CurrencyCode,
        CreatedAt:    timestamppb.New(user.CreatedAt),
    }
}

func protoToUser(pb *pb.User) *domain.User {
    return &domain.User{
        ID:           pb.Id,
        UUID:         uuid.MustParse(pb.Uuid),
        Email:        pb.Email,
        Status:       domain.UserStatus(pb.Status),
        KYCLevel:     int(pb.KycLevel),
        CountryCode:  pb.CountryCode,
        CurrencyCode: pb.CurrencyCode,
        CreatedAt:    pb.CreatedAt.AsTime(),
    }
}
============================================================
SECTION 7: ANTI-PATTERNS
============================================================
text

❌ NEVER use proto types in service/repository (convert at boundary)
❌ NEVER skip context.WithTimeout on client calls (unbounded wait)
❌ NEVER create new connection per request (reuse via client struct)
❌ NEVER return codes.Internal with detailed error message
❌ NEVER skip interceptors (logging, tracing, recovery)
❌ NEVER put business logic in gRPC server methods (delegate to service)
❌ NEVER use reflection.Register in production (dev/staging only)
❌ NEVER ignore gRPC status codes in client error handling
❌ NEVER use context.Background() in gRPC handlers (use request ctx)
❌ NEVER skip graceful shutdown (GracefulStop, not Stop)
============================================================
SECTION 8: TESTING
============================================================
Go

func TestGetUser_Success(t *testing.T) {
    // Setup mock service
    mockSvc := &mockUserService{}
    mockSvc.On("GetByID", mock.Anything, int64(1)).Return(&domain.User{
        ID: 1, Email: "test@example.com", Status: domain.UserStatusActive,
    }, nil)

    server := NewUserGRPCServer(mockSvc)

    resp, err := server.GetUser(context.Background(), &pb.GetUserRequest{UserId: 1})

    require.NoError(t, err)
    assert.Equal(t, int64(1), resp.User.Id)
    assert.Equal(t, "test@example.com", resp.User.Email)
}

func TestGetUser_NotFound(t *testing.T) {
    mockSvc := &mockUserService{}
    mockSvc.On("GetByID", mock.Anything, int64(999)).Return(nil, domain.ErrUserNotFound)

    server := NewUserGRPCServer(mockSvc)

    resp, err := server.GetUser(context.Background(), &pb.GetUserRequest{UserId: 999})

    assert.Nil(t, resp)
    st, ok := status.FromError(err)
    require.True(t, ok)
    assert.Equal(t, codes.NotFound, st.Code())
}