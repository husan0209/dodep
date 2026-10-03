package grpc

import (
	"context"
	"strconv"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/opus-casino/payment/internal/domain"
	"github.com/opus-casino/payment/internal/service"
	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"
	paymentv1 "github.com/opus-casino/proto/gen/go/payment/v1"
)

// PaymentGRPCServer implements payment.v1.PaymentService.
// It is a thin translation layer: user identity comes from request fields
// (set by trusted internal callers such as admin-bff), money travels as
// decimal strings, and every decision delegates to the service layer.
type PaymentGRPCServer struct {
	paymentv1.UnimplementedPaymentServiceServer
	payments    *service.PaymentService
	withdrawals *service.WithdrawalService
	methods     *service.PaymentMethodService
	log         *zap.Logger
}

// NewPaymentGRPCServer creates the server.
func NewPaymentGRPCServer(
	payments *service.PaymentService,
	withdrawals *service.WithdrawalService,
	methods *service.PaymentMethodService,
	log *zap.Logger,
) *PaymentGRPCServer {
	return &PaymentGRPCServer{payments: payments, withdrawals: withdrawals, methods: methods, log: log}
}

func parseUserID(v *commonv1.UserId) (int64, error) {
	if v == nil || v.Value == "" {
		return 0, status.Error(codes.InvalidArgument, "user_id required")
	}
	id, err := strconv.ParseInt(v.Value, 10, 64)
	if err != nil || id <= 0 {
		return 0, status.Error(codes.InvalidArgument, "invalid user_id")
	}
	return id, nil
}

// parseOptionalUserID returns 0 when the caller omits user scope
// (admin/ops fan-out); user-facing callers must pass their id.
func parseOptionalUserID(v *commonv1.UserId) (int64, error) {
	if v == nil || v.Value == "" {
		return 0, nil
	}
	return parseUserID(v)
}

func parseMoney(m *commonv1.Money) (decimal.Decimal, error) {
	if m == nil || m.Amount == "" {
		return decimal.Zero, status.Error(codes.InvalidArgument, "amount required")
	}
	amount, err := decimal.NewFromString(m.Amount)
	if err != nil || amount.IsNegative() {
		return decimal.Zero, status.Error(codes.InvalidArgument, "invalid amount")
	}
	return amount, nil
}

func moneyOf(amount decimal.Decimal, currency string) *commonv1.Money {
	return &commonv1.Money{Amount: amount.String(), Currency: currency}
}

// ============ Deposits ============

func (s *PaymentGRPCServer) CreateDeposit(ctx context.Context, req *paymentv1.CreateDepositRequest) (*paymentv1.CreateDepositResponse, error) {
	userID, err := parseUserID(req.UserId)
	if err != nil {
		return nil, err
	}
	amount, err := parseMoney(req.Amount)
	if err != nil {
		return nil, err
	}
	currency := domain.CryptoCurrency(req.Amount.Currency)
	if !currency.IsDepositSupported() {
		return &paymentv1.CreateDepositResponse{Error: errDetails(domain.ErrorCurrencyNotSupported(req.Amount.Currency))}, nil
	}
	res, err := s.payments.InitiateDeposit(ctx, service.InitiateDepositRequest{
		UserID:         userID,
		Amount:         amount,
		Currency:       currency,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return &paymentv1.CreateDepositResponse{Error: errDetails(err)}, nil
	}
	payment, err := s.payments.GetPayment(ctx, res.PaymentUUID)
	if err != nil {
		return &paymentv1.CreateDepositResponse{Error: errDetails(err)}, nil
	}
	return &paymentv1.CreateDepositResponse{Deposit: toProtoDeposit(payment)}, nil
}

func (s *PaymentGRPCServer) GetDeposit(ctx context.Context, req *paymentv1.GetDepositRequest) (*paymentv1.GetDepositResponse, error) {
	if _, err := parseUserID(req.UserId); err != nil {
		return nil, err
	}
	if req.DepositId == "" {
		return nil, status.Error(codes.InvalidArgument, "deposit_id required")
	}
	payment, err := s.payments.GetPayment(ctx, req.DepositId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "deposit not found")
	}
	return &paymentv1.GetDepositResponse{Deposit: toProtoDeposit(payment)}, nil
}

func (s *PaymentGRPCServer) ListDeposits(ctx context.Context, req *paymentv1.ListDepositsRequest) (*paymentv1.ListDepositsResponse, error) {
	userID, err := parseOptionalUserID(req.UserId)
	if err != nil {
		return nil, err
	}
	limit, cursor, statusFilter := fromPagination(req.Pagination)
	if req.Status != nil {
		statusFilter = fromDepositStatus(*req.Status)
	}
	_ = req.DateRange

	var items []domain.Payment
	var nextCursor string
	var hasMore bool
	if userID == 0 {
		result, err := s.payments.ListAllPayments(ctx, limit, cursor, statusFilter)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		items, nextCursor, hasMore = result.Items, result.NextCursor, result.HasMore
	} else {
		result, err := s.payments.ListPayments(ctx, service.ListPaymentsRequest{
			UserID: userID, Limit: limit, Cursor: cursor, Status: statusFilter,
		})
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		items, nextCursor, hasMore = result.Items, result.NextCursor, result.HasMore
	}
	resp := &paymentv1.ListDepositsResponse{
		Pagination: &commonv1.PageResponse{NextCursor: nextCursor, HasMore: hasMore},
	}
	for i := range items {
		resp.Deposits = append(resp.Deposits, toProtoDeposit(&items[i]))
	}
	return resp, nil
}

// ============ Withdrawals ============

func (s *PaymentGRPCServer) RequestWithdrawal(ctx context.Context, req *paymentv1.RequestWithdrawalRequest) (*paymentv1.RequestWithdrawalResponse, error) {
	userID, err := parseUserID(req.UserId)
	if err != nil {
		return nil, err
	}
	amount, err := parseMoney(req.Amount)
	if err != nil {
		return nil, err
	}
	res, err := s.withdrawals.InitiateWithdrawal(ctx, service.InitiateWithdrawalRequest{
		UserID:         userID,
		Amount:         amount,
		Currency:       domain.CryptoCurrency(req.PaymentProvider),
		Address:        withdrawalAddress(req),
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return &paymentv1.RequestWithdrawalResponse{Error: errDetails(err)}, nil
	}
	w, err := s.withdrawals.GetWithdrawal(ctx, res.WithdrawalUUID)
	if err != nil {
		return &paymentv1.RequestWithdrawalResponse{Error: errDetails(err)}, nil
	}
	return &paymentv1.RequestWithdrawalResponse{Withdrawal: toProtoWithdrawal(w)}, nil
}

func (s *PaymentGRPCServer) GetWithdrawal(ctx context.Context, req *paymentv1.GetWithdrawalRequest) (*paymentv1.GetWithdrawalResponse, error) {
	if _, err := parseUserID(req.UserId); err != nil {
		return nil, err
	}
	if req.WithdrawalId == "" {
		return nil, status.Error(codes.InvalidArgument, "withdrawal_id required")
	}
	w, err := s.withdrawals.GetWithdrawal(ctx, req.WithdrawalId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "withdrawal not found")
	}
	return &paymentv1.GetWithdrawalResponse{Withdrawal: toProtoWithdrawal(w)}, nil
}

func (s *PaymentGRPCServer) ListWithdrawals(ctx context.Context, req *paymentv1.ListWithdrawalsRequest) (*paymentv1.ListWithdrawalsResponse, error) {
	userID, err := parseOptionalUserID(req.UserId)
	if err != nil {
		return nil, err
	}
	limit, cursor, statusFilter := fromPagination(req.Pagination)
	if req.Status != nil {
		statusFilter = fromWithdrawalStatus(*req.Status)
	}
	_ = req.DateRange

	var items []domain.Withdrawal
	var nextCursor string
	var hasMore bool
	if userID == 0 {
		// Admin/ops review queue across users.
		result, err := s.withdrawals.ListAllWithdrawals(ctx, limit, statusFilter, cursor)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		items, nextCursor, hasMore = result.Items, result.NextCursor, result.HasMore
	} else {
		result, err := s.withdrawals.ListWithdrawals(ctx, service.ListPaymentsRequest{
			UserID: userID, Limit: limit, Cursor: cursor, Status: statusFilter,
		})
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		items, nextCursor, hasMore = result.Items, result.NextCursor, result.HasMore
	}
	resp := &paymentv1.ListWithdrawalsResponse{
		Pagination: &commonv1.PageResponse{NextCursor: nextCursor, HasMore: hasMore},
	}
	for i := range items {
		resp.Withdrawals = append(resp.Withdrawals, toProtoWithdrawal(&items[i]))
	}
	return resp, nil
}

func (s *PaymentGRPCServer) CancelWithdrawal(ctx context.Context, req *paymentv1.CancelWithdrawalRequest) (*paymentv1.CancelWithdrawalResponse, error) {
	userID, err := parseUserID(req.UserId)
	if err != nil {
		return nil, err
	}
	if req.WithdrawalId == "" {
		return nil, status.Error(codes.InvalidArgument, "withdrawal_id required")
	}
	if _, err := s.withdrawals.CancelWithdrawal(ctx, userID, req.WithdrawalId); err != nil {
		return &paymentv1.CancelWithdrawalResponse{Success: false, Error: errDetails(err)}, nil
	}
	return &paymentv1.CancelWithdrawalResponse{Success: true}, nil
}

func (s *PaymentGRPCServer) ApproveWithdrawal(ctx context.Context, req *paymentv1.ApproveWithdrawalRequest) (*paymentv1.ApproveWithdrawalResponse, error) {
	if req.WithdrawalId == "" {
		return nil, status.Error(codes.InvalidArgument, "withdrawal_id required")
	}
	if req.ApprovedBy == "" {
		return nil, status.Error(codes.InvalidArgument, "approved_by required")
	}
	w, err := s.withdrawals.ApproveWithdrawal(ctx, service.DecideWithdrawalRequest{
		WithdrawalUUID: req.WithdrawalId,
		AdminID:        req.ApprovedBy,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return &paymentv1.ApproveWithdrawalResponse{Error: errDetails(err)}, nil
	}
	return &paymentv1.ApproveWithdrawalResponse{Withdrawal: toProtoWithdrawal(w)}, nil
}

func (s *PaymentGRPCServer) RejectWithdrawal(ctx context.Context, req *paymentv1.RejectWithdrawalRequest) (*paymentv1.RejectWithdrawalResponse, error) {
	if req.WithdrawalId == "" {
		return nil, status.Error(codes.InvalidArgument, "withdrawal_id required")
	}
	if req.RejectedBy == "" {
		return nil, status.Error(codes.InvalidArgument, "rejected_by required")
	}
	if req.Reason == "" {
		return nil, status.Error(codes.InvalidArgument, "reason required")
	}
	w, err := s.withdrawals.RejectWithdrawal(ctx, service.DecideWithdrawalRequest{
		WithdrawalUUID: req.WithdrawalId,
		AdminID:        req.RejectedBy,
		Reason:         req.Reason,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return &paymentv1.RejectWithdrawalResponse{Error: errDetails(err)}, nil
	}
	return &paymentv1.RejectWithdrawalResponse{Withdrawal: toProtoWithdrawal(w)}, nil
}

// ============ Payment methods ============

func (s *PaymentGRPCServer) GetPaymentMethods(ctx context.Context, req *paymentv1.GetPaymentMethodsRequest) (*paymentv1.GetPaymentMethodsResponse, error) {
	if _, err := parseUserID(req.UserId); err != nil {
		return nil, err
	}
	// Static catalog derived from the supported currency list (same source
	// as the REST /methods endpoint). Saved user destinations are managed
	// via Save/Get/DeletePaymentMethod.
	resp := &paymentv1.GetPaymentMethodsResponse{}
	for _, c := range domain.SupportedWithdrawalCurrencies() {
		resp.Methods = append(resp.Methods, &paymentv1.PaymentMethodInfo{
			Id:                        "crypto:" + string(c),
			Type:                      paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_CRYPTO,
			Provider:                  "nowpayments",
			DisplayName:               string(c) + " (" + domain.CryptoCurrency(c).Network() + ")",
			SupportedCurrencies:       []string{string(c)},
			SupportedTransactionTypes: []string{"deposit", "withdrawal"},
		})
	}
	return resp, nil
}

func (s *PaymentGRPCServer) GetPaymentMethod(ctx context.Context, req *paymentv1.GetPaymentMethodRequest) (*paymentv1.GetPaymentMethodResponse, error) {
	userID, err := parseUserID(req.UserId)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.PaymentMethodId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid payment_method_id")
	}
	m, err := s.methods.GetMethod(ctx, userID, id)
	if err != nil {
		return &paymentv1.GetPaymentMethodResponse{Error: errDetails(err)}, nil
	}
	return &paymentv1.GetPaymentMethodResponse{Method: toProtoMethod(m)}, nil
}

func (s *PaymentGRPCServer) SavePaymentMethod(ctx context.Context, req *paymentv1.SavePaymentMethodRequest) (*paymentv1.SavePaymentMethodResponse, error) {
	userID, err := parseUserID(req.UserId)
	if err != nil {
		return nil, err
	}
	m, err := s.methods.SaveMethod(ctx, service.SaveMethodInput{
		UserID:    userID,
		Type:      fromProtoMethodType(req.Type),
		Provider:  req.Provider,
		Nickname:  req.Nickname,
		Details:   req.Details,
		IsDefault: req.IsDefault,
	})
	if err != nil {
		return &paymentv1.SavePaymentMethodResponse{Error: errDetails(err)}, nil
	}
	return &paymentv1.SavePaymentMethodResponse{Method: toProtoMethod(m)}, nil
}

func (s *PaymentGRPCServer) DeletePaymentMethod(ctx context.Context, req *paymentv1.DeletePaymentMethodRequest) (*paymentv1.DeletePaymentMethodResponse, error) {
	userID, err := parseUserID(req.UserId)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.PaymentMethodId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid payment_method_id")
	}
	if err := s.methods.DeleteMethod(ctx, userID, id); err != nil {
		return &paymentv1.DeletePaymentMethodResponse{Success: false, Error: errDetails(err)}, nil
	}
	return &paymentv1.DeletePaymentMethodResponse{Success: true}, nil
}

// ============ Mapping ============

func errDetails(err error) *commonv1.ErrorDetails {
	return &commonv1.ErrorDetails{
		ErrorCode:    commonv1.ErrorCode(domain.GetErrorCode(err)),
		ErrorMessage: err.Error(),
	}
}

func fromPagination(p *commonv1.PageRequest) (limit int, cursor, status string) {
	limit = 20
	if p != nil {
		if p.PageSize > 0 {
			limit = int(p.PageSize)
		}
		cursor = p.Cursor
	}
	return limit, cursor, status
}

func withdrawalAddress(req *paymentv1.RequestWithdrawalRequest) string {
	if addr, ok := req.PaymentDetails["address"]; ok && addr != "" {
		return addr
	}
	return ""
}

func toProtoDeposit(p *domain.Payment) *paymentv1.Deposit {
	d := &paymentv1.Deposit{
		Id:        p.UUID.String(),
		UserId:    &commonv1.UserId{Value: strconv.FormatInt(p.UserID, 10)},
		Amount:    moneyOf(p.RequestedAmount, p.FiatCurrency),
		Currency:  p.FiatCurrency,
		Status:    toProtoDepositStatus(p.Status),
		CreatedAt: timestamppb.New(p.CreatedAt),
		UpdatedAt: timestamppb.New(p.UpdatedAt),
	}
	if p.PayAmount != nil {
		d.NetAmount = moneyOf(*p.PayAmount, p.CryptoCurrency)
	}
	if p.CompletedAt != nil {
		d.CompletedAt = timestamppb.New(*p.CompletedAt)
	}
	return d
}

func toProtoWithdrawal(w *domain.Withdrawal) *paymentv1.Withdrawal {
	pw := &paymentv1.Withdrawal{
		Id:        w.UUID.String(),
		UserId:    &commonv1.UserId{Value: strconv.FormatInt(w.UserID, 10)},
		Amount:    moneyOf(w.Amount, w.CryptoCurrency),
		NetAmount: moneyOf(w.Amount, w.CryptoCurrency),
		Currency:  w.CryptoCurrency,
		Status:    toProtoWithdrawalStatus(w.Status),
		// IdempotencyKey is a client-supplied replay secret: it stays internal
		// and is never published to admin-facing consumers.
		CreatedAt: timestamppb.New(w.CreatedAt),
		UpdatedAt: timestamppb.New(w.UpdatedAt),
	}
	if w.DecidedAt != nil {
		ts := timestamppb.New(*w.DecidedAt)
		switch w.Status {
		case domain.WithdrawalStatusRejected:
			pw.CancelledAt = ts
		default:
			pw.ApprovedAt = ts
		}
		if w.DecidedBy != "" {
			by := w.DecidedBy
			pw.ApprovedBy = &by
		}
	}
	if w.DecisionReason != "" {
		pw.RejectionReason = w.DecisionReason
	}
	if w.CompletedAt != nil && !w.CompletedAt.IsZero() {
		pw.CompletedAt = timestamppb.New(*w.CompletedAt)
	}
	// Payout destination and provider reference only: the wallet lock handle
	// stays inside the service.
	pw.PaymentDetails = map[string]string{
		"address":     w.Address,
		"provider_id": w.WithdrawalID,
	}
	return pw
}

func toProtoMethod(m *domain.SavedPaymentMethod) *paymentv1.PaymentMethod {
	pm := &paymentv1.PaymentMethod{
		Id:           m.ID.String(),
		UserId:       &commonv1.UserId{Value: strconv.FormatInt(m.UserID, 10)},
		Type:         toProtoMethodType(m.Type),
		Provider:     m.Provider,
		Nickname:     m.Nickname,
		DisplayValue: m.DisplayValue,
		IsDefault:    m.IsDefault,
		IsActive:     m.IsActive,
		CreatedAt:    timestamppb.New(m.CreatedAt),
	}
	if m.LastUsedAt != nil {
		pm.LastUsedAt = timestamppb.New(*m.LastUsedAt)
	}
	return pm
}

func toProtoDepositStatus(s domain.PaymentStatus) commonv1.TransactionStatus {
	switch s {
	case domain.PaymentStatusPending, domain.PaymentStatusWaiting,
		domain.PaymentStatusConfirming, domain.PaymentStatusConfirmed,
		domain.PaymentStatusSending, domain.PaymentStatusPartiallyPaid:
		return commonv1.TransactionStatus_TRANSACTION_STATUS_PENDING
	case domain.PaymentStatusFinished:
		return commonv1.TransactionStatus_TRANSACTION_STATUS_COMPLETED
	case domain.PaymentStatusFailed, domain.PaymentStatusExpired:
		return commonv1.TransactionStatus_TRANSACTION_STATUS_FAILED
	default:
		return commonv1.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED
	}
}

func toProtoWithdrawalStatus(s domain.WithdrawalStatus) commonv1.TransactionStatus {
	switch s {
	case domain.WithdrawalStatusPendingReview, domain.WithdrawalStatusApproved:
		return commonv1.TransactionStatus_TRANSACTION_STATUS_PENDING
	case domain.WithdrawalStatusProcessing, domain.WithdrawalStatusSending, domain.WithdrawalStatusSent:
		return commonv1.TransactionStatus_TRANSACTION_STATUS_PROCESSING
	case domain.WithdrawalStatusFinished:
		return commonv1.TransactionStatus_TRANSACTION_STATUS_COMPLETED
	case domain.WithdrawalStatusFailed:
		return commonv1.TransactionStatus_TRANSACTION_STATUS_FAILED
	case domain.WithdrawalStatusRejected, domain.WithdrawalStatusCancelled:
		return commonv1.TransactionStatus_TRANSACTION_STATUS_CANCELLED
	default:
		return commonv1.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED
	}
}

// fromDepositStatus converts a proto status filter to the deposits.status
// value (the deposits table keeps its own 'pending' state).
func fromDepositStatus(s commonv1.TransactionStatus) string {
	switch s {
	case commonv1.TransactionStatus_TRANSACTION_STATUS_PENDING:
		return "pending"
	case commonv1.TransactionStatus_TRANSACTION_STATUS_PROCESSING:
		return "sending"
	case commonv1.TransactionStatus_TRANSACTION_STATUS_COMPLETED:
		return "finished"
	case commonv1.TransactionStatus_TRANSACTION_STATUS_FAILED:
		return "failed"
	case commonv1.TransactionStatus_TRANSACTION_STATUS_CANCELLED:
		return "cancelled"
	default:
		return ""
	}
}

// fromWithdrawalStatus converts a proto status filter to a
// withdrawals.status value. The review queue only acts on pending_review, so
// PENDING maps to that state; PROCESSING covers the states between approval and
// the provider confirming the payout.
func fromWithdrawalStatus(s commonv1.TransactionStatus) string {
	switch s {
	case commonv1.TransactionStatus_TRANSACTION_STATUS_PENDING:
		return "pending_review"
	case commonv1.TransactionStatus_TRANSACTION_STATUS_PROCESSING:
		return "processing"
	case commonv1.TransactionStatus_TRANSACTION_STATUS_COMPLETED:
		return "finished"
	case commonv1.TransactionStatus_TRANSACTION_STATUS_FAILED:
		return "failed"
	case commonv1.TransactionStatus_TRANSACTION_STATUS_CANCELLED:
		return "cancelled"
	default:
		return ""
	}
}

func toProtoMethodType(t domain.PaymentMethodType) paymentv1.PaymentMethodType {
	switch t {
	case domain.PaymentMethodTypeCreditCard:
		return paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_CREDIT_CARD
	case domain.PaymentMethodTypeDebitCard:
		return paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_DEBIT_CARD
	case domain.PaymentMethodTypeBankTransfer:
		return paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_BANK_TRANSFER
	case domain.PaymentMethodTypeEWallet:
		return paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_E_WALLET
	case domain.PaymentMethodTypeCrypto:
		return paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_CRYPTO
	case domain.PaymentMethodTypePrepaid:
		return paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_PREPAID
	case domain.PaymentMethodTypeMobile:
		return paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_MOBILE
	case domain.PaymentMethodTypePIX:
		return paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_PIX
	case domain.PaymentMethodTypeUPI:
		return paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_UPI
	default:
		return paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_UNSPECIFIED
	}
}

func fromProtoMethodType(t paymentv1.PaymentMethodType) domain.PaymentMethodType {
	switch t {
	case paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_CREDIT_CARD:
		return domain.PaymentMethodTypeCreditCard
	case paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_DEBIT_CARD:
		return domain.PaymentMethodTypeDebitCard
	case paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_BANK_TRANSFER:
		return domain.PaymentMethodTypeBankTransfer
	case paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_E_WALLET:
		return domain.PaymentMethodTypeEWallet
	case paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_CRYPTO:
		return domain.PaymentMethodTypeCrypto
	case paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_PREPAID:
		return domain.PaymentMethodTypePrepaid
	case paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_MOBILE:
		return domain.PaymentMethodTypeMobile
	case paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_PIX:
		return domain.PaymentMethodTypePIX
	case paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_UPI:
		return domain.PaymentMethodTypeUPI
	default:
		return domain.PaymentMethodTypeUnspecified
	}
}
