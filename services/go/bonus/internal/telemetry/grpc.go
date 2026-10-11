package telemetry

import (
	"context"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryMetricsInterceptor records duration and outcome of every gRPC call.
// The `code` label is the status code name, which is a bounded set.
func UnaryMetricsInterceptor(m *Metrics) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		m.ObserveGRPC(info.FullMethod, codeName(err), time.Since(start))
		return resp, err
	}
}

// UnaryRecoveryInterceptor converts panics into codes.Internal so a single bad
// request can never take the gRPC server down.
func UnaryRecoveryInterceptor(log *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				if log != nil {
					log.Error("panic recovered in gRPC handler",
						zap.String("method", info.FullMethod),
						zap.Any("panic", r),
					)
				}
				// Generic message: never leak internals to callers.
				err = status.Error(codes.Internal, "internal error")
			}
		}()
		return handler(ctx, req)
	}
}

// UnaryLoggingInterceptor emits one structured log line per gRPC call
// (observability standard: JSON, no fmt.Println).
func UnaryLoggingInterceptor(log *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)
		fields := []zap.Field{
			zap.String("method", shortMethod(info.FullMethod)),
			zap.String("code", codeName(err)),
			zap.Int64("duration_ms", duration.Milliseconds()),
		}
		if log == nil {
			return resp, err
		}
		if err != nil {
			log.Warn("grpc request failed", fields...)
		} else {
			log.Info("grpc request completed", fields...)
		}
		return resp, err
	}
}

// UnaryTracingInterceptor creates a SERVER span per gRPC call and propagates
// the context, so a trace can span wallet-core → bonus-service → wallet-core.
func UnaryTracingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Keep the W3C trace context the caller sent; only start a root span
		// when nothing was propagated (never restart a trace mid-flow).
		ctx, span := StartServerSpan(ctx, shortMethod(info.FullMethod),
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.service", info.FullMethod),
		)
		defer span.End()

		resp, err := handler(ctx, req)
		span.SetAttributes(attribute.String("rpc.grpc.status_code", codeName(err)))
		EndSpan(span, err)
		return resp, err
	}
}

// ChainUnary composes interceptors in order: the first one is the outermost.
func ChainUnary(interceptors ...grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		chain := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			current, next := interceptors[i], chain
			chain = func(ctx context.Context, req interface{}) (interface{}, error) {
				return current(ctx, req, info, next)
			}
		}
		return chain(ctx, req)
	}
}

func codeName(err error) string {
	if err == nil {
		return codes.OK.String()
	}
	if st, ok := status.FromError(err); ok {
		return st.Code().String()
	}
	return codes.Unknown.String()
}

// shortMethod trims the service prefix: "bonus.v1.BonusService/GetBonus" →
// "GetBonus", keeping the label readable and still bounded.
func shortMethod(full string) string {
	if i := strings.LastIndex(full, "/"); i >= 0 && i+1 < len(full) {
		return full[i+1:]
	}
	return full
}

// ShortMethod is the exported form of shortMethod (used by tests).
func ShortMethod(full string) string { return shortMethod(full) }

// CodeName exposes the bounded status-code label (used by tests).
func CodeName(err error) string { return codeName(err) }
