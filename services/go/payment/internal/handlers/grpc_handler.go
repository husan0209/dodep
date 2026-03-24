package handlers

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opus-casino/proto/gen/go/payment/v1"
	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"

	"github.com/opus-casino/payment/internal/domain"
	"github.com/opus-casino/payment/internal/service"
)

type PaymentGRPCHandler struct {
	pb.UnimplementedPaymentServiceServer
	service *service.PaymentService
	log     *zap.Logger
}

func NewPaymentGRPCHandler(svc *service.PaymentService, log *zap.Logger) *PaymentGRPCHandler {
	return &PaymentGRPCHandler{service: svc, log: log}
}

func (h *PaymentGRPCHandler) GetPaymentMethods(ctx context.Context, req *pb.GetPaymentMethodsRequest) (*pb.GetPaymentMethodsResponse, error) {
	methods, err := h.service.GetPaymentMethods(ctx, req.UserId.Value)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get payment methods")
	}

	var pbMethods []*pb.PaymentMethodInfo
	for _, m := range methods {
		pbMethods = append(pbMethods, &pb.PaymentMethodInfo{
			Id:                  m.ID,
			Type:                pb.PaymentMethodType(pb.PaymentMethodType_value[string(m.Type)]),
			Provider:            m.Provider,
			DisplayName:         m.DisplayName,
			SupportedCurrencies: m.SupportedCurrencies,
		})
	}
	return &pb.GetPaymentMethodsResponse{Methods: pbMethods}, nil
}

func (h *PaymentGRPCHandler) CreateDeposit(ctx context.Context, req *pb.CreateDepositRequest) (*pb.CreateDepositResponse, error) {
	deposit, err := h.service.CreateDeposit(ctx, &domain.CreateDepositRequest{
		UserID:          req.UserId.Value,
		Amount:          req.Amount.Value,
		Currency:        req.Amount.Currency,
		PaymentMethodID: req.PaymentMethodId,
		PaymentProvider: req.PaymentProvider,
		IdempotencyKey:  req.IdempotencyKey,
	})
	if err != nil {
		return &pb.CreateDepositResponse{
			Error: &commonv1.ErrorDetails{Code: "DEPOSIT_FAILED", Message: err.Error()},
		}, nil
	}
	return &pb.CreateDepositResponse{Deposit: toProtoDeposit(deposit)}, nil
}

func (h *PaymentGRPCHandler) GetDeposit(ctx context.Context, req *pb.GetDepositRequest) (*pb.GetDepositResponse, error) {
	deposit, err := h.service.GetDeposit(ctx, req.UserId.Value, req.DepositId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "deposit not found")
	}
	return &pb.GetDepositResponse{Deposit: toProtoDeposit(deposit)}, nil
}

func (h *PaymentGRPCHandler) ListDeposits(ctx context.Context, req *pb.ListDepositsRequest) (*pb.ListDepositsResponse, error) {
	limit, offset := 20, 0
	if req.Pagination != nil {
		limit = int(req.Pagination.Limit)
		offset = int(req.Pagination.Offset)
	}
	deposits, total, err := h.service.ListDeposits(ctx, req.UserId.Value, limit, offset)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list deposits")
	}
	var pbDeposits []*pb.Deposit
	for _, d := range deposits {
		pbDeposits = append(pbDeposits, toProtoDeposit(d))
	}
	return &pb.ListDepositsResponse{
		Deposits:   pbDeposits,
		Pagination: &commonv1.PageResponse{Total: int64(total), Limit: int32(limit), Offset: int32(offset)},
	}, nil
}

func (h *PaymentGRPCHandler) RequestWithdrawal(ctx context.Context, req *pb.RequestWithdrawalRequest) (*pb.RequestWithdrawalResponse, error) {
	withdrawal, err := h.service.RequestWithdrawal(ctx, &domain.RequestWithdrawalRequest{
		UserID:          req.UserId.Value,
		Amount:          req.Amount.Value,
		Currency:        req.Amount.Currency,
		PaymentMethodID: req.PaymentMethodId,
		PaymentProvider: req.PaymentProvider,
		IdempotencyKey:  req.IdempotencyKey,
	})
	if err != nil {
		return &pb.RequestWithdrawalResponse{
			Error: &commonv1.ErrorDetails{Code: "WITHDRAWAL_FAILED", Message: err.Error()},
		}, nil
	}
	return &pb.RequestWithdrawalResponse{Withdrawal: toProtoWithdrawal(withdrawal)}, nil
}

func (h *PaymentGRPCHandler) GetWithdrawal(ctx context.Context, req *pb.GetWithdrawalRequest) (*pb.GetWithdrawalResponse, error) {
	w, err := h.service.GetWithdrawal(ctx, req.UserId.Value, req.WithdrawalId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "withdrawal not found")
	}
	return &pb.GetWithdrawalResponse{Withdrawal: toProtoWithdrawal(w)}, nil
}

func (h *PaymentGRPCHandler) ListWithdrawals(ctx context.Context, req *pb.ListWithdrawalsRequest) (*pb.ListWithdrawalsResponse, error) {
	limit, offset := 20, 0
	if req.Pagination != nil {
		limit = int(req.Pagination.Limit)
		offset = int(req.Pagination.Offset)
	}
	withdrawals, total, err := h.service.ListWithdrawals(ctx, req.UserId.Value, limit, offset)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list withdrawals")
	}
	var pbWithdrawals []*pb.Withdrawal
	for _, w := range withdrawals {
		pbWithdrawals = append(pbWithdrawals, toProtoWithdrawal(w))
	}
	return &pb.ListWithdrawalsResponse{
		Withdrawals: pbWithdrawals,
		Pagination:  &commonv1.PageResponse{Total: int64(total), Limit: int32(limit), Offset: int32(offset)},
	}, nil
}

func (h *PaymentGRPCHandler) CancelWithdrawal(ctx context.Context, req *pb.CancelWithdrawalRequest) (*pb.CancelWithdrawalResponse, error) {
	if err := h.service.CancelWithdrawal(ctx, req.UserId.Value, req.WithdrawalId); err != nil {
		return &pb.CancelWithdrawalResponse{
			Success: false,
			Error:   &commonv1.ErrorDetails{Code: "CANCEL_FAILED", Message: err.Error()},
		}, nil
	}
	return &pb.CancelWithdrawalResponse{Success: true}, nil
}

func (h *PaymentGRPCHandler) GetPaymentMethod(ctx context.Context, req *pb.GetPaymentMethodRequest) (*pb.GetPaymentMethodResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (h *PaymentGRPCHandler) SavePaymentMethod(ctx context.Context, req *pb.SavePaymentMethodRequest) (*pb.SavePaymentMethodResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (h *PaymentGRPCHandler) DeletePaymentMethod(ctx context.Context, req *pb.DeletePaymentMethodRequest) (*pb.DeletePaymentMethodResponse, error) {
	if err := h.service.HandleWebhook(ctx, &domain.WebhookEvent{}); err != nil {
		return &pb.DeletePaymentMethodResponse{Success: false}, nil
	}
	return &pb.DeletePaymentMethodResponse{Success: true}, nil
}

func toProtoDeposit(d *domain.Deposit) *pb.Deposit {
	pbD := &pb.Deposit{
		Id:            d.ID,
		UserId:        &commonv1.UserId{Value: d.UserID},
		Amount:        &commonv1.Money{Value: d.Amount, Currency: d.Currency},
		Fee:           &commonv1.Money{Value: d.Fee, Currency: d.Currency},
		NetAmount:     &commonv1.Money{Value: d.NetAmount, Currency: d.Currency},
		Currency:      d.Currency,
		Status:        commonv1.TransactionStatus(commonv1.TransactionStatus_value[string(d.Status)]),
		IdempotencyKey: d.IdempotencyKey,
		CreatedAt:     timestamppb.New(d.CreatedAt),
		UpdatedAt:     timestamppb.New(d.UpdatedAt),
	}
	if d.CompletedAt != nil {
		pbD.CompletedAt = timestamppb.New(*d.CompletedAt)
	}
	return pbD
}

func toProtoWithdrawal(w *domain.Withdrawal) *pb.Withdrawal {
	pbW := &pb.Withdrawal{
		Id:             w.ID,
		UserId:         &commonv1.UserId{Value: w.UserID},
		Amount:         &commonv1.Money{Value: w.Amount, Currency: w.Currency},
		Fee:            &commonv1.Money{Value: w.Fee, Currency: w.Currency},
		NetAmount:      &commonv1.Money{Value: w.NetAmount, Currency: w.Currency},
		Currency:       w.Currency,
		Status:         commonv1.TransactionStatus(commonv1.TransactionStatus_value[string(w.Status)]),
		IdempotencyKey: w.IdempotencyKey,
		CreatedAt:      timestamppb.New(w.CreatedAt),
		UpdatedAt:      timestamppb.New(w.UpdatedAt),
	}
	if w.CompletedAt != nil {
		pbW.CompletedAt = timestamppb.New(*w.CompletedAt)
	}
	if w.ApprovedAt != nil {
		pbW.ApprovedAt = timestamppb.New(*w.ApprovedAt)
	}
	return pbW
}
