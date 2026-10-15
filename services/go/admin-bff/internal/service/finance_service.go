package service

import (
	"context"

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

// The manual-review decision flow (approve/reject/hold) lives in the
// withdrawals HTTP handlers: it writes the decision columns of the shared
// `withdrawals` table (migration 022), which the payment service reads.
// Driving it through a payment gRPC RPC would require proto changes; the
// table-level flow keeps a single source of truth without a contract bump.
