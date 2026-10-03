package service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"
	paymentv1 "github.com/opus-casino/proto/gen/go/payment/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/opus-casino/admin-bff/internal/client"
)

// FinanceService is the back-office facade over Payment Service.
// Money stays in Payment Service; this layer only shapes responses for the
// admin UI and records who decided what.
type FinanceService struct {
	paymentClient *client.PaymentClient
	auditService  *AuditService
}

func NewFinanceService(pc *client.PaymentClient, as *AuditService) *FinanceService {
	return &FinanceService{paymentClient: pc, auditService: as}
}

// TransactionView is the admin-panel shape of a deposit/withdrawal.
// Flat strings (no nested Money) keep the payload JSON-friendly and let the
// UI format amounts without knowing the proto layout.
type TransactionView struct {
	ID              string `json:"id"`
	UserID          string `json:"user_id"`
	Amount          string `json:"amount"`
	CurrencyCode    string `json:"currency_code"`
	Status          string `json:"status"`
	Reference       string `json:"psp_reference"`
	Destination     string `json:"destination,omitempty"`
	ReviewedBy      string `json:"reviewed_by"`
	ReviewedAt      string `json:"reviewed_at"`
	RejectionReason string `json:"rejection_reason"`
	IdempotencyKey  string `json:"idempotency_key"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
	CompletedAt     string `json:"completed_at"`
	ApprovedAt      string `json:"approved_at"`
	CancelledAt     string `json:"cancelled_at"`
	ReviewReason    string `json:"review_reason"`
	ProviderID      string `json:"provider_id"`
}

func (s *FinanceService) ListDeposits(ctx context.Context, status commonv1.TransactionStatus, pageSize int32, cursor string) ([]interface{}, interface{}, error) {
	deps, page, err := s.paymentClient.ListDeposits(ctx, status, pageSize, cursor)
	if err != nil {
		return nil, nil, err
	}
	result := make([]interface{}, 0, len(deps))
	for _, d := range deps {
		result = append(result, depositView(d))
	}
	return result, page, nil
}

// ListWithdrawals lists the cross-user review queue. userID > 0 narrows it to
// a single player.
func (s *FinanceService) ListWithdrawals(ctx context.Context, userID int64, status commonv1.TransactionStatus, pageSize int32, cursor string) ([]interface{}, interface{}, error) {
	wds, page, err := s.paymentClient.ListWithdrawals(ctx, userID, status, pageSize, cursor)
	if err != nil {
		return nil, nil, err
	}
	result := make([]interface{}, 0, len(wds))
	for _, w := range wds {
		result = append(result, withdrawalView(w))
	}
	return result, page, nil
}

// GetWithdrawal fetches one withdrawal by its Payment Service UUID.
func (s *FinanceService) GetWithdrawal(ctx context.Context, withdrawalUUID string) (*TransactionView, error) {
	w, err := s.paymentClient.GetWithdrawal(ctx, withdrawalUUID)
	if err != nil {
		return nil, err
	}
	view := withdrawalView(w)
	return &view, nil
}

// ApproveWithdrawal executes the manual-review approval in Payment Service:
// it claims the withdrawal, triggers the PSP payout and releases the hold.
func (s *FinanceService) ApproveWithdrawal(ctx context.Context, adminID int64, adminName, withdrawalUUID string) (*TransactionView, error) {
	actor := operatorIdentity(adminID, adminName)
	w, err := s.paymentClient.ApproveWithdrawal(ctx, withdrawalUUID, actor, uuid.NewString())
	if err != nil {
		return nil, err
	}
	s.auditService.Log(ctx, &adminID, "withdrawal.approve", "withdrawal", withdrawalUUID, map[string]interface{}{
		"admin_name": adminName,
		"status":     w.Status.String(),
	}, "", "")
	view := withdrawalView(w)
	return &view, nil
}

// RejectWithdrawal declines the withdrawal and releases the reserved funds.
func (s *FinanceService) RejectWithdrawal(ctx context.Context, adminID int64, adminName, withdrawalUUID, reason string) (*TransactionView, error) {
	if reason == "" {
		return nil, fmt.Errorf("a rejection reason is required")
	}
	actor := operatorIdentity(adminID, adminName)
	w, err := s.paymentClient.RejectWithdrawal(ctx, withdrawalUUID, actor, reason, uuid.NewString())
	if err != nil {
		return nil, err
	}
	s.auditService.Log(ctx, &adminID, "withdrawal.reject", "withdrawal", withdrawalUUID, map[string]interface{}{
		"admin_name": adminName,
		"reason":     reason,
		"status":     w.Status.String(),
	}, "", "")
	view := withdrawalView(w)
	return &view, nil
}

// operatorIdentity renders the actor recorded on the decision. Payment Service
// stores it on the withdrawal for the audit trail.
func operatorIdentity(adminID int64, adminName string) string {
	if adminName != "" && adminName != "0" {
		return fmt.Sprintf("%s(%d)", adminName, adminID)
	}
	return strconv.FormatInt(adminID, 10)
}

func rfc3339(ts *timestamppb.Timestamp) string {
	if ts == nil || !ts.IsValid() {
		return ""
	}
	return ts.AsTime().UTC().Format("2006-01-02T15:04:05Z")
}

func depositView(d *paymentv1.Deposit) TransactionView {
	v := TransactionView{
		ID:             d.Id,
		Amount:         d.Amount.GetAmount(),
		CurrencyCode:   d.Currency,
		Status:         d.Status.String(),
		IdempotencyKey: d.IdempotencyKey,
		CreatedAt:      rfc3339(d.CreatedAt),
		UpdatedAt:      rfc3339(d.UpdatedAt),
		CompletedAt:    rfc3339(d.CompletedAt),
	}
	if d.UserId != nil {
		v.UserID = d.UserId.Value
	}
	if d.NetAmount != nil {
		v.Reference = d.NetAmount.Amount
	}
	v.ProviderID = d.ProviderTransactionId
	if v.ProviderID == "" && d.Metadata != nil {
		v.ProviderID = d.Metadata["payment_id"]
	}
	return v
}

func withdrawalView(w *paymentv1.Withdrawal) TransactionView {
	v := TransactionView{
		ID:              w.Id,
		Amount:          w.Amount.GetAmount(),
		CurrencyCode:    firstNonEmpty(w.Currency, w.Amount.GetCurrency()),
		Status:          w.Status.String(),
		IdempotencyKey:  w.IdempotencyKey,
		CreatedAt:       rfc3339(w.CreatedAt),
		UpdatedAt:       rfc3339(w.UpdatedAt),
		CompletedAt:     rfc3339(w.CompletedAt),
		ApprovedAt:      rfc3339(w.ApprovedAt),
		CancelledAt:     rfc3339(w.CancelledAt),
		ReviewedBy:      w.GetApprovedBy(),
		RejectionReason: w.RejectionReason,
		ReviewReason:    w.RejectionReason,
	}
	if w.UserId != nil {
		v.UserID = w.UserId.Value
	}
	if w.PaymentDetails != nil {
		v.Destination = w.PaymentDetails["address"]
		v.ProviderID = w.PaymentDetails["provider_id"]
	}
	v.Reference = v.ProviderID
	return v
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
