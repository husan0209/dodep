package main

import (
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// grpcRecoveryInterceptor converts panics into Internal errors (fail secure).
func grpcRecoveryInterceptor(log *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("KYC: panic recovered in gRPC handler",
					zap.Any("panic", r), zap.String("method", info.FullMethod))
				err = status.Error(codes.Internal, "internal error")
			}
		}()
		return handler(ctx, req)
	}
}

// grpcLoggingInterceptor records method, duration and outcome (no PII).
func grpcLoggingInterceptor(log *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		fields := []zap.Field{
			zap.String("method", info.FullMethod),
			zap.Duration("duration", time.Since(start)),
		}
		if err != nil {
			st, _ := status.FromError(err)
			log.Warn("KYC: gRPC request failed",
				append(fields, zap.String("code", st.Code().String()))...)
		} else {
			log.Info("KYC: gRPC request completed", fields...)
		}
		return resp, err
	}
}
