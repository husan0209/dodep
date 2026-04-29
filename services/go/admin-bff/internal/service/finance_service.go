package service

import (
	"context"
	"fmt"

	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"
	"github.com/opus-casino/admin-bff/internal/client"
)

type FinanceService struct {
	paymentClient *client.PaymentClient
	auditService  *AuditService
}

func NewFinanceService(pc *client.PaymentClient, as *AuditService) *FinanceService {
	return &FinanceService{paymentClient: pc, auditService: as}
}

func (s *FinanceService) ListDeposits(ctx context.Context, status commonv1.TransactionStatus, pageSize int32, cursor string) ([]interface{}, interface{}, error) {
	deps, page, err := s.paymentClient.ListDeposits(ctx, status, pageSize, cursor)
	if err != nil {
		return nil, nil, err
	}
	result := make([]interface{}, len(deps))
	for i, d := range deps {
		result[i] = d
	}
	return result, page, nil
}

func (s *FinanceService) ListWithdrawals(ctx context.Context, status commonv1.TransactionStatus, pageSize int32, cursor string) ([]interface{}, interface{}, error) {
	wds, page, err := s.paymentClient.ListWithdrawals(ctx, status, pageSize, cursor)
	if err != nil {
		return nil, nil, err
	}
	result := make([]interface{}, len(wds))
	for i, w := range wds {
		result[i] = w
	}
	return result, page, nil
}

func (s *FinanceService) ApproveWithdrawal(ctx context.Context, adminID int64, withdrawalID string) error {
	// Payment proto currently lacks ApproveWithdrawal; fallback to Cancel or extend proto.
	// For now this delegates to a future gRPC method or local orchestration.
	// TODO: extend payment/v1/payment.proto and regenerate when protoc is available.
	_ = adminID
	_ = withdrawalID
	return fmt.Errorf("not implemented: payment proto lacks ApproveWithdrawal RPC")
}

func (s *FinanceService) RejectWithdrawal(ctx context.Context, adminID int64, withdrawalID, reason string) error {
	_ = adminID
	_ = withdrawalID
	_ = reason
	return fmt.Errorf("not implemented: payment proto lacks RejectWithdrawal RPC")
}
