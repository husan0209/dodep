package grpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/opus-casino/payment/internal/domain"
	"github.com/opus-casino/payment/internal/repository"
	"github.com/opus-casino/payment/internal/service"
	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"
	paymentv1 "github.com/opus-casino/proto/gen/go/payment/v1"
)

// newWithdrawalServiceForQueue wires a WithdrawalService for read-only queue
// tests: only the repository is exercised, so the money-moving collaborators
// are deliberately absent.
func newWithdrawalServiceForQueue(repo repository.WithdrawalRepository) *service.WithdrawalService {
	return service.NewWithdrawalService(
		repo,
		nil, nil, nil, // idempotency, exchange rate, daily limits
		nil, nil, nil, // NOWPayments, wallet, user
		nil, // producer
		nil, // tracer
		"",  // IPN callback URL
		service.DefaultReviewConfig(),
	)
}

// queueRepo is a stateful in-memory WithdrawalRepository for the review queue.
// It mirrors the keyset ordering the SQL implementation uses: oldest first,
// then by id.
type queueRepo struct {
	rows []*domain.Withdrawal
	seq  int64
}

func newQueueRepo(n int) *queueRepo {
	r := &queueRepo{}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		r.seq++
		rows := &domain.Withdrawal{
			ID:             r.seq,
			UUID:           uuid.New(),
			UserID:         int64(100 + i),
			Amount:         decimal.NewFromInt(int64(i + 1)),
			FiatAmount:     decimal.NewFromInt(int64(1000 * (i + 1))),
			FiatCurrency:   "USD",
			CryptoCurrency: "BTC",
			Address:        "bc1qexampleaddress",
			Status:         domain.WithdrawalStatusPendingReview,
			LockID:         "lock-" + uuid.NewString(),
			IdempotencyKey: uuid.NewString(),
			CreatedAt:      base.Add(time.Duration(i) * time.Minute),
		}
		r.rows = append(r.rows, rows)
	}
	return r
}

func (r *queueRepo) Create(context.Context, *domain.Withdrawal) error { return nil }

func (r *queueRepo) get(id int64) (*domain.Withdrawal, error) {
	for _, w := range r.rows {
		if w.ID == id {
			return w, nil
		}
	}
	return nil, domain.ErrorWithdrawalNotFound(id)
}

func (r *queueRepo) GetByID(_ context.Context, id int64) (*domain.Withdrawal, error) {
	return r.get(id)
}
func (r *queueRepo) GetByWithdrawalID(context.Context, string) (*domain.Withdrawal, error) {
	return nil, domain.NewDetailedError(domain.ErrWithdrawalNotFound, domain.ErrCodeWithdrawalNotFound)
}
func (r *queueRepo) GetByIDempotencyKey(context.Context, string) (*domain.Withdrawal, error) {
	return nil, nil
}
func (r *queueRepo) GetByUUID(_ context.Context, u string) (*domain.Withdrawal, error) {
	for _, w := range r.rows {
		if w.UUID.String() == u {
			return w, nil
		}
	}
	return nil, domain.NewDetailedError(domain.ErrWithdrawalNotFound, domain.ErrCodeWithdrawalNotFound)
}
func (r *queueRepo) UpdateStatus(_ context.Context, id int64, from, to domain.WithdrawalStatus) error {
	w, err := r.get(id)
	if err != nil {
		return err
	}
	if w.Status != from {
		return domain.ErrorInvalidStatusTransition(string(from), string(to))
	}
	w.Status = to
	return nil
}
func (r *queueRepo) RecordDecision(_ context.Context, id int64, by, reason string) error {
	w, err := r.get(id)
	if err != nil {
		return err
	}
	w.DecidedBy = by
	w.DecisionReason = reason
	now := time.Now()
	w.DecidedAt = &now
	return nil
}
func (r *queueRepo) SetProviderWithdrawalID(_ context.Context, id int64, providerID string) error {
	w, err := r.get(id)
	if err != nil {
		return err
	}
	w.WithdrawalID = providerID
	return nil
}
func (r *queueRepo) ListByUserID(_ context.Context, userID int64, f repository.ListFilter) (*repository.ListResult[domain.Withdrawal], error) {
	items := []domain.Withdrawal{}
	for _, w := range r.rows {
		if w.UserID == userID && (f.Status == "" || string(w.Status) == f.Status) {
			items = append(items, *w)
		}
	}
	return &repository.ListResult[domain.Withdrawal]{Items: items}, nil
}

// ListAll mirrors the SQL ordering: oldest first, then by id, with a keyset
// cursor so pages never overlap or skip while decisions happen concurrently.
func (r *queueRepo) ListAll(_ context.Context, f repository.ListFilter) (*repository.ListResult[domain.Withdrawal], error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}

	ordered := make([]*domain.Withdrawal, len(r.rows))
	copy(ordered, r.rows)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].CreatedAt.Equal(ordered[j].CreatedAt) {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].CreatedAt.Before(ordered[j].CreatedAt)
	})

	if f.Status != "" {
		filtered := ordered[:0:0]
		for _, w := range ordered {
			if string(w.Status) == f.Status {
				filtered = append(filtered, w)
			}
		}
		ordered = filtered
	}

	if f.Cursor != "" {
		data, err := base64.URLEncoding.DecodeString(f.Cursor)
		if err != nil {
			return nil, fmt.Errorf("decode cursor: %w", err)
		}
		var cur struct {
			ID        int64     `json:"id"`
			CreatedAt time.Time `json:"created_at"`
		}
		if err := json.Unmarshal(data, &cur); err != nil {
			return nil, fmt.Errorf("unmarshal cursor: %w", err)
		}
		after := ordered[:0:0]
		for _, w := range ordered {
			if w.CreatedAt.After(cur.CreatedAt) ||
				(w.CreatedAt.Equal(cur.CreatedAt) && w.ID > cur.ID) {
				after = append(after, w)
			}
		}
		ordered = after
	}

	if len(ordered) <= limit {
		items := make([]domain.Withdrawal, 0, len(ordered))
		for _, w := range ordered {
			items = append(items, *w)
		}
		return &repository.ListResult[domain.Withdrawal]{Items: items}, nil
	}

	items := make([]domain.Withdrawal, 0, limit)
	for _, w := range ordered[:limit] {
		items = append(items, *w)
	}
	last := ordered[limit-1]
	cursor, _ := json.Marshal(struct {
		ID        int64     `json:"id"`
		CreatedAt time.Time `json:"created_at"`
	}{ID: last.ID, CreatedAt: last.CreatedAt})

	return &repository.ListResult[domain.Withdrawal]{
		Items:      items,
		NextCursor: base64.URLEncoding.EncodeToString(cursor),
		HasMore:    true,
	}, nil
}

func (r *queueRepo) CountByUserIDStatus(context.Context, int64, []domain.WithdrawalStatus) (int64, error) {
	return int64(len(r.rows)), nil
}

// The review queue must be readable across users with a keyset cursor so the
// admin UI pages through the oldest requests first.
func TestListWithdrawals_QueueAcrossUsersWithCursor(t *testing.T) {
	ctx := context.Background()
	repo := newQueueRepo(5)
	svc := newWithdrawalServiceForQueue(repo)
	srv := NewPaymentGRPCServer(nil, svc, nil, zap.NewNop())

	// First page: oldest two.
	resp, err := srv.ListWithdrawals(ctx, &paymentv1.ListWithdrawalsRequest{
		Pagination: &commonv1.PageRequest{PageSize: 2},
	})
	if err != nil {
		t.Fatalf("unexpected gRPC error: %v", err)
	}
	if len(resp.Withdrawals) != 2 {
		t.Fatalf("expected 2 withdrawals, got %d", len(resp.Withdrawals))
	}
	if resp.Pagination == nil || !resp.Pagination.HasMore || resp.Pagination.NextCursor == "" {
		t.Fatalf("expected has_more with a next cursor, got %+v", resp.Pagination)
	}
	if resp.Withdrawals[0].Id != repo.rows[0].UUID.String() {
		t.Error("queue must start at the oldest request")
	}

	// Second page continues where the first stopped.
	next, err := srv.ListWithdrawals(ctx, &paymentv1.ListWithdrawalsRequest{
		Pagination: &commonv1.PageRequest{PageSize: 2, Cursor: resp.Pagination.NextCursor},
	})
	if err != nil {
		t.Fatalf("unexpected gRPC error on second page: %v", err)
	}
	if len(next.Withdrawals) != 2 {
		t.Fatalf("expected 2 withdrawals on page 2, got %d", len(next.Withdrawals))
	}
	if next.Withdrawals[0].Id == resp.Withdrawals[0].Id {
		t.Error("page 2 must not repeat page 1")
	}

	// Filtering by PENDING selects exactly the review-queue states.
	pending := commonv1.TransactionStatus_TRANSACTION_STATUS_PENDING
	filtered, err := srv.ListWithdrawals(ctx, &paymentv1.ListWithdrawalsRequest{
		Pagination: &commonv1.PageRequest{PageSize: 10},
		Status:     &pending,
	})
	if err != nil {
		t.Fatalf("unexpected gRPC error with filter: %v", err)
	}
	if len(filtered.Withdrawals) != 5 {
		t.Fatalf("pending filter must return the whole queue, got %d", len(filtered.Withdrawals))
	}
}

// A player-scoped call must return only that player's withdrawals.
func TestListWithdrawals_ScopedToSingleUser(t *testing.T) {
	ctx := context.Background()
	repo := newQueueRepo(3)
	svc := newWithdrawalServiceForQueue(repo)
	srv := NewPaymentGRPCServer(nil, svc, nil, zap.NewNop())

	userID := repo.rows[1].UserID
	resp, err := srv.ListWithdrawals(ctx, &paymentv1.ListWithdrawalsRequest{
		UserId:     &commonv1.UserId{Value: "101"},
		Pagination: &commonv1.PageRequest{PageSize: 10},
	})
	if err != nil {
		t.Fatalf("unexpected gRPC error: %v", err)
	}
	if len(resp.Withdrawals) != 1 {
		t.Fatalf("expected exactly the player's withdrawal, got %d", len(resp.Withdrawals))
	}
	if resp.Withdrawals[0].UserId.GetValue() != "101" {
		t.Errorf("expected user 101, got %s", resp.Withdrawals[0].UserId.GetValue())
	}
	_ = userID
}

// The admin UI must not receive the crypto destination or the wallet lock id.
func TestListWithdrawals_OmitsSensitiveFields(t *testing.T) {
	ctx := context.Background()
	repo := newQueueRepo(1)
	svc := newWithdrawalServiceForQueue(repo)
	srv := NewPaymentGRPCServer(nil, svc, nil, zap.NewNop())

	resp, err := srv.ListWithdrawals(ctx, &paymentv1.ListWithdrawalsRequest{
		Pagination: &commonv1.PageRequest{PageSize: 10},
	})
	if err != nil {
		t.Fatalf("unexpected gRPC error: %v", err)
	}
	w := resp.Withdrawals[0]
	// The UI does not need it and it is a client-supplied secret used for
	// request replay protection.
	if w.IdempotencyKey != "" {
		t.Error("idempotency key must not be exposed to the review queue")
	}
	if w.PaymentDetails["lock_id"] != "" {
		t.Error("wallet lock handle must not be exposed to the review queue")
	}
	if w.GetApprovedBy() != "" {
		t.Error("an undecided withdrawal must not report a reviewer")
	}
	if w.Status != commonv1.TransactionStatus_TRANSACTION_STATUS_PENDING {
		t.Errorf("pending_review must map to PENDING, got %s", w.Status)
	}
	if w.CreatedAt == nil || !w.CreatedAt.AsTime().Equal(repo.rows[0].CreatedAt) {
		t.Error("created_at must be preserved for queue ordering")
	}
}

// Rejections and approvals must reach the admin UI with the operator recorded.
func TestListWithdrawals_ReflectsDecision(t *testing.T) {
	ctx := context.Background()
	repo := newQueueRepo(1)
	svc := newWithdrawalServiceForQueue(repo)
	srv := NewPaymentGRPCServer(nil, svc, nil, zap.NewNop())

	row := repo.rows[0]
	if err := repo.UpdateStatus(ctx, row.ID, domain.WithdrawalStatusPendingReview, domain.WithdrawalStatusRejected); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if err := repo.RecordDecision(ctx, row.ID, "admin-7", "sanctions screen hit"); err != nil {
		t.Fatalf("record: %v", err)
	}

	resp, err := srv.ListWithdrawals(ctx, &paymentv1.ListWithdrawalsRequest{
		Pagination: &commonv1.PageRequest{PageSize: 10},
	})
	if err != nil {
		t.Fatalf("unexpected gRPC error: %v", err)
	}
	w := resp.Withdrawals[0]
	if w.Status != commonv1.TransactionStatus_TRANSACTION_STATUS_CANCELLED {
		t.Errorf("rejected must map to CANCELLED, got %s", w.Status)
	}
	if w.RejectionReason != "sanctions screen hit" {
		t.Errorf("reason lost: %q", w.RejectionReason)
	}
	if w.GetApprovedBy() != "admin-7" {
		t.Errorf("operator lost: %q", w.GetApprovedBy())
	}
}
