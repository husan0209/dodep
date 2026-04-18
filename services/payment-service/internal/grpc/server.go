package grpc

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/platform/services/payment-service/internal/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// Server represents the gRPC server for inter-service communication.
type Server struct {
	server   *grpc.Server
	listener net.Listener
	port     int
	logger   *zap.Logger
}

// ServerConfig holds configuration for the gRPC server.
type ServerConfig struct {
	Port              int
	EnableReflection  bool
	MaxRecvMsgSize    int
	MaxSendMsgSize    int
}

// NewServer creates a new gRPC server with health check and interceptors.
func NewServer(cfg ServerConfig, logger *zap.Logger) (*Server, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return nil, fmt.Errorf("listen on port %d: %w", cfg.Port, err)
	}

	// Create interceptors
	unaryInterceptors := []grpc.UnaryServerInterceptor{
		loggingInterceptor(logger),
		tracingInterceptor(),
	}

	streamInterceptors := []grpc.StreamServerInterceptor{
		streamLoggingInterceptor(logger),
		streamTracingInterceptor(),
	}

	// Create gRPC server with options
	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(unaryInterceptors...),
		grpc.ChainStreamInterceptor(streamInterceptors...),
	}

	if cfg.MaxRecvMsgSize > 0 {
		opts = append(opts, grpc.MaxRecvMsgSize(cfg.MaxRecvMsgSize))
	}
	if cfg.MaxSendMsgSize > 0 {
		opts = append(opts, grpc.MaxSendMsgSize(cfg.MaxSendMsgSize))
	}

	server := grpc.NewServer(opts...)

	// Register health check service
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(server, healthServer)

	// Enable reflection for development
	if cfg.EnableReflection {
		reflection.Register(server)
		logger.Info("gRPC reflection enabled")
	}

	logger.Info("gRPC server initialized",
		zap.Int("port", cfg.Port),
		zap.Bool("health_check", true),
	)

	return &Server{
		server:   server,
		listener: listener,
		port:     cfg.Port,
		logger:   logger,
	}, nil
}

// Start starts the gRPC server.
func (s *Server) Start() error {
	s.logger.Info("Starting gRPC server", zap.Int("port", s.port))
	return s.server.Serve(s.listener)
}

// Shutdown gracefully stops the gRPC server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down gRPC server")

	done := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("gRPC server stopped gracefully")
		return nil
	case <-ctx.Done():
		s.logger.Warn("gRPC server shutdown timeout, forcing stop")
		s.server.Stop()
		return ctx.Err()
	}
}

// GRPCServer returns the underlying gRPC server for service registration.
func (s *Server) GRPCServer() *grpc.Server {
	return s.server
}

// Addr returns the server address.
func (s *Server) Addr() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return ""
}

// loggingInterceptor logs unary RPC calls.
func loggingInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		duration := time.Since(start)
		logger.Info("gRPC unary call",
			zap.String("method", info.FullMethod),
			zap.Duration("duration", duration),
			zap.Error(err),
		)

		return resp, err
	}
}

// tracingInterceptor adds OpenTelemetry tracing context to unary RPC calls.
func tracingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Extract trace context from incoming metadata
		ctx = observability.ExtractTraceContext(ctx)

		// Start a new span for this RPC call
		ctx, span := observability.StartSpan(ctx, info.FullMethod,
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer observability.EndSpan(span)

		// Set span attributes
		observability.SetSpanAttributes(span,
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.method", info.FullMethod),
		)

		// Call the handler
		resp, err := handler(ctx, req)

		// Record error if any
		if err != nil {
			observability.RecordError(span, err)
		} else {
			observability.SetSpanStatus(span, codes.Ok, "")
		}

		return resp, err
	}
}

// streamLoggingInterceptor logs streaming RPC calls.
func streamLoggingInterceptor(logger *zap.Logger) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()

		err := handler(srv, ss)

		duration := time.Since(start)
		logger.Info("gRPC stream call",
			zap.String("method", info.FullMethod),
			zap.Duration("duration", duration),
			zap.Error(err),
		)

		return err
	}
}

// streamTracingInterceptor adds OpenTelemetry tracing context to streaming RPC calls.
func streamTracingInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// Extract trace context from incoming metadata
		ctx := observability.ExtractTraceContext(ss.Context())

		// Start a new span for this RPC call
		ctx, span := observability.StartSpan(ctx, info.FullMethod,
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer observability.EndSpan(span)

		// Set span attributes
		observability.SetSpanAttributes(span,
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.method", info.FullMethod),
		)

		// Wrap the stream with the new context
		wrappedStream := &contextStream{
			ServerStream: ss,
			ctx:          ctx,
		}

		// Call the handler
		err := handler(srv, wrappedStream)

		// Record error if any
		if err != nil {
			observability.RecordError(span, err)
		} else {
			observability.SetSpanStatus(span, codes.Ok, "")
		}

		return err
	}
}

// contextStream wraps a grpc.ServerStream with a custom context.
type contextStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context returns the wrapped context.
func (s *contextStream) Context() context.Context {
	return s.ctx
}
