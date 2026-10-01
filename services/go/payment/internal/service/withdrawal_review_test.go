package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/opus-casino/payment/internal/client"
	"github.com/opus-casino/payment/internal/domain"
	"github.com/opus-casino/payment/internal/repository"
	"github.com/shopspring/decimal"
)

// fakeWithdrawalRepo is a stateful in-memory WithdrawalRepository that
// enforces the same optimistic guard as the SQL implementation
// (UpdateStatus touches a row only when the current status matches).
type fakeWithdrawalRepo struct {
	mu     sync.Mutex
	rows   map[int64]*domain.Withdrawal
	byUUID map[string]int64
	seq    int64
}

func newFakeWithdrawalRepo() *fakeWithdrawalRepo {
	return &fakeWithdrawalRepo{rows: map[int64]*domain.Withdrawal{}, byUUID: map[string]int64{}}
}

func (f *fakeWithdrawalRepo) Create(ctx context.Context, w *domain.Withdrawal) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	w.ID = f.seq
	cp := *w
	f.rows[w.ID] = &cp
	f.byUUID[w.UUID.String()] = w.ID
	return nil
}

func (f *fakeWithdrawalRepo) get(id int64) (*domain.Withdrawal, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w, ok := f.rows[id]
	if !ok {
		return nil, domain.ErrorWithdrawalNotFound(id)
	}
	cp := *w
	return &cp, nil
}

func (f *fakeWithdrawalRepo) GetByID(ctx context.Context, id int64) (*domain.Withdrawal, error) {
	return f.get(id)
}

func (f *fakeWithdrawalRepo) GetByWithdrawalID(ctx context.Context, withdrawalID string) (*domain.Withdrawal, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, w := range f.rows {
		if w.WithdrawalID == withdrawalID {
			cp := *w
			return &cp, nil
		}
	}
	return nil, domain.NewDetailedError(domain.ErrWithdrawalNotFound, domain.ErrCodeWithdrawalNotFound)
}

func (f *fakeWithdrawalRepo) GetByIDempotencyKey(ctx context.Context, key string) (*domain.Withdrawal, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, w := range f.rows {
		if w.IdempotencyKey == key {
			cp := *w
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *fakeWithdrawalRepo) GetByUUID(ctx context.Context, uuidStr string) (*domain.Withdrawal, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.byUUID[uuidStr]
	if !ok {
		return nil, domain.NewDetailedError(domain.ErrWithdrawalNotFound, domain.ErrCodeWithdrawalNotFound)
	}
	cp := *f.rows[id]
	return &cp, nil
}

func (f *fakeWithdrawalRepo) UpdateStatus(ctx context.Context, id int64, fromStatus, toStatus domain.WithdrawalStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	w, ok := f.rows[id]
	if !ok {
		return domain.ErrorWithdrawalNotFound(id)
	}
	if w.Status != fromStatus {
		return domain.ErrorInvalidStatusTransition(string(fromStatus), string(toStatus))
	}
	w.Status = toStatus
	return nil
}

func (f *fakeWithdrawalRepo) RecordDecision(ctx context.Context, id int64, decidedBy, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	w, ok := f.rows[id]
	if !ok {
		return domain.ErrorWithdrawalNotFound(id)
	}
	w.DecidedBy = decidedBy
	w.DecisionReason = reason
	return nil
}

func (f *fakeWithdrawalRepo) SetProviderWithdrawalID(ctx context.Context, id int64, providerID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	w, ok := f.rows[id]
	if !ok {
		return domain.ErrorWithdrawalNotFound(id)
	}
	w.WithdrawalID = providerID
	return nil
}

func (f *fakeWithdrawalRepo) ListByUserID(ctx context.Context, userID int64, filter repository.ListFilter) (*repository.ListResult[domain.Withdrawal], error) {
	return &repository.ListResult[domain.Withdrawal]{}, nil
}

func (f *fakeWithdrawalRepo) ListAll(ctx context.Context, filter repository.ListFilter) (*repository.ListResult[domain.Withdrawal], error) {
	return &repository.ListResult[domain.Withdrawal]{}, nil
}

func (f *fakeWithdrawalRepo) CountByUserIDStatus(ctx context.Context, userID int64, statuses []domain.WithdrawalStatus) (int64, error) {
	return 0, nil
}

// reviewTestDeps bundles a service wired with fakes.
type reviewTestDeps struct {
	svc        *WithdrawalService
	repo       *fakeWithdrawalRepo
	payouts    *atomic.Int64
	unlocks    *atomic.Int64
	lastUnlock atomic.Value // string idempotency key
}

func newReviewService(thresholdUSD string) *reviewTestDeps {
	repo := newFakeWithdrawalRepo()
	var payouts, unlocks atomic.Int64
	d := &reviewTestDeps{repo: repo, payouts: &payouts, unlocks: &unlocks}

	rate := decimal.NewFromInt(50000) // 1 BTC = $50k
	exchange := &MockExchangeRateRepository{
		GetFunc: func(ctx context.Context, fromCurrency, toCurrency string) (*decimal.Decimal, error) {
			return &rate, nil
		},
	}
	daily := &MockDailyLimitsRepository{
		GetFunc: func(ctx context.Context, userID int64, operationType string) (decimal.Decimal, error) {
			return decimal.Zero, nil
		},
	}
	user := &MockUserClient{
		GetKYCLevelFunc: func(ctx context.Context, userID int64) (int, error) { return 2, nil },
	}
	wallet := &MockWalletClient{
		LockFundsFunc: func(ctx context.Context, req client.LockRequest) (*client.LockResult, error) {
			return &client.LockResult{LockID: req.Reference, NewBalance: decimal.Zero}, nil
		},
		UnlockFundsFunc: func(ctx context.Context, req client.UnlockRequest) error {
			unlocks.Add(1)
			d.lastUnlock.Store(req.Reference)
			return nil
		},
	}
	nowp := &MockNOWPaymentsClient{
		CreatePayoutFunc: func(ctx context.Context, req client.CreatePayoutRequest) (*client.CreatePayoutResponse, error) {
			payouts.Add(1)
			return &client.CreatePayoutResponse{
				WithdrawalID: "np-" + req.WithdrawalID,
				Status:       "processing",
				Amount:       req.Amount,
				Currency:     req.Currency,
				Address:      req.Address,
			}, nil
		},
	}

	limit, err := decimal.NewFromString(thresholdUSD)
	if err != nil {
		panic(err)
	}
	d.svc = NewWithdrawalService(
		repo,
		&MockIdempotencyRepository{},
		exchange,
		daily,
		nowp,
		wallet,
		user,
		nil, // producer (nil-safe: events skipped)
		nil, // tracer
		"",
		ReviewConfig{Enabled: true, AutoApproveLimitUSD: limit},
	)
	return d
}

func initiateReviewWithdrawal(t *testing.T, d *reviewTestDeps, amountBTC string, key string) *InitiateWithdrawalResponse {
	t.Helper()
	amount, err := decimal.NewFromString(amountBTC)
	if err != nil {
		t.Fatal(err)
	}
	res, err := d.svc.InitiateWithdrawal(context.Background(), InitiateWithdrawalRequest{
		UserID:         42,
		Amount:         amount,
		Currency:       domain.CryptoBTC,
		Address:        "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
		IdempotencyKey: key,
		IPAddress:      "127.0.0.1",
		UserAgent:      "test",
	})
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}
	return res
}

func TestReviewFlow_BelowThresholdStaysInstant(t *testing.T) {
	d := newReviewService("1000")
	// 0.001 BTC * $50k = $50 < $1000 -> instant PSP flow.
	res := initiateReviewWithdrawal(t, d, "0.001", "key-instant-1")
	if res.Status != string(domain.WithdrawalStatusProcessing) {
		t.Fatalf("expected processing, got %s", res.Status)
	}
	if d.payouts.Load() != 1 {
		t.Fatalf("expected 1 PSP payout, got %d", d.payouts.Load())
	}
}

func TestReviewFlow_AboveThresholdQueuesReview(t *testing.T) {
	d := newReviewService("1000")
	// 0.1 BTC * $50k = $5000 >= $1000 -> manual review, no PSP call.
	res := initiateReviewWithdrawal(t, d, "0.1", "key-review-1")
	if res.Status != string(domain.WithdrawalStatusPendingReview) {
		t.Fatalf("expected pending_review, got %s", res.Status)
	}
	if d.payouts.Load() != 0 {
		t.Fatalf("PSP must not be called before approval, got %d calls", d.payouts.Load())
	}
	w, err := d.svc.GetWithdrawal(context.Background(), res.WithdrawalUUID)
	if err != nil {
		t.Fatal(err)
	}
	if w.LockID == "" {
		t.Fatal("expected wallet lock id stored on the review withdrawal")
	}
	if w.WithdrawalID != "" {
		t.Fatal("review withdrawal must not have a provider payout id yet")
	}
}

func TestReviewFlow_ApproveExecutesPayout(t *testing.T) {
	d := newReviewService("1000")
	res := initiateReviewWithdrawal(t, d, "0.1", "key-review-2")

	approved, err := d.svc.ApproveWithdrawal(context.Background(), DecideWithdrawalRequest{
		WithdrawalUUID: res.WithdrawalUUID,
		AdminID:        "admin-7",
		IdempotencyKey: "decide-1",
	})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if approved.Status != domain.WithdrawalStatusProcessing {
		t.Fatalf("expected processing, got %s", approved.Status)
	}
	if approved.WithdrawalID == "" {
		t.Fatal("expected provider payout id attached")
	}
	if approved.DecidedBy != "admin-7" {
		t.Fatalf("expected decided_by admin-7, got %q", approved.DecidedBy)
	}
	if d.payouts.Load() != 1 {
		t.Fatalf("expected exactly 1 PSP payout, got %d", d.payouts.Load())
	}
}

func TestReviewFlow_ApproveIsIdempotent(t *testing.T) {
	d := newReviewService("1000")
	res := initiateReviewWithdrawal(t, d, "0.1", "key-review-3")

	first, err := d.svc.ApproveWithdrawal(context.Background(), DecideWithdrawalRequest{
		WithdrawalUUID: res.WithdrawalUUID, AdminID: "admin-7", IdempotencyKey: "decide-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := d.svc.ApproveWithdrawal(context.Background(), DecideWithdrawalRequest{
		WithdrawalUUID: res.WithdrawalUUID, AdminID: "admin-7", IdempotencyKey: "decide-2",
	})
	if err != nil {
		t.Fatalf("repeat approve must succeed, got %v", err)
	}
	if second.Status != first.Status || second.WithdrawalID != first.WithdrawalID {
		t.Fatal("repeat approve must return the same state")
	}
	if d.payouts.Load() != 1 {
		t.Fatalf("PSP must be called once, got %d", d.payouts.Load())
	}
}

func TestReviewFlow_ConcurrentApproveExecutesOnce(t *testing.T) {
	d := newReviewService("1000")
	res := initiateReviewWithdrawal(t, d, "0.1", "key-review-4")

	const racers = 8
	var wg sync.WaitGroup
	errs := make([]error, racers)
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = d.svc.ApproveWithdrawal(context.Background(), DecideWithdrawalRequest{
				WithdrawalUUID: res.WithdrawalUUID, AdminID: "admin-7", IdempotencyKey: "decide-race",
			})
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("racer %d failed: %v", i, err)
		}
	}
	if d.payouts.Load() != 1 {
		t.Fatalf("concurrent approves must execute exactly 1 payout, got %d", d.payouts.Load())
	}
}

func TestReviewFlow_ApproveNonPendingFails(t *testing.T) {
	d := newReviewService("1000")
	res := initiateReviewWithdrawal(t, d, "0.001", "key-instant-2") // processing already
	_, err := d.svc.ApproveWithdrawal(context.Background(), DecideWithdrawalRequest{
		WithdrawalUUID: res.WithdrawalUUID, AdminID: "admin-7", IdempotencyKey: "decide-3",
	})
	if err == nil {
		t.Fatal("approving a processing withdrawal must fail")
	}
}

func TestReviewFlow_RejectReleasesFunds(t *testing.T) {
	d := newReviewService("1000")
	res := initiateReviewWithdrawal(t, d, "0.1", "key-review-5")

	rejected, err := d.svc.RejectWithdrawal(context.Background(), DecideWithdrawalRequest{
		WithdrawalUUID: res.WithdrawalUUID, AdminID: "admin-7", Reason: "sanctions screen hit", IdempotencyKey: "decide-4",
	})
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if rejected.Status != domain.WithdrawalStatusRejected {
		t.Fatalf("expected rejected, got %s", rejected.Status)
	}
	if rejected.DecisionReason != "sanctions screen hit" {
		t.Fatalf("reason not stored: %q", rejected.DecisionReason)
	}
	if d.unlocks.Load() != 1 {
		t.Fatalf("expected 1 unlock, got %d", d.unlocks.Load())
	}
	if d.payouts.Load() != 0 {
		t.Fatalf("rejected withdrawal must never reach PSP, got %d payouts", d.payouts.Load())
	}

	// Repeat is idempotent and must not unlock twice.
	if _, err := d.svc.RejectWithdrawal(context.Background(), DecideWithdrawalRequest{
		WithdrawalUUID: res.WithdrawalUUID, AdminID: "admin-7", Reason: "sanctions screen hit", IdempotencyKey: "decide-4",
	}); err != nil {
		t.Fatalf("repeat reject must succeed, got %v", err)
	}
	if d.unlocks.Load() != 1 {
		t.Fatalf("repeat reject must not unlock twice, got %d", d.unlocks.Load())
	}
}

func TestReviewFlow_RejectRequiresReason(t *testing.T) {
	d := newReviewService("1000")
	res := initiateReviewWithdrawal(t, d, "0.1", "key-review-6")
	_, err := d.svc.RejectWithdrawal(context.Background(), DecideWithdrawalRequest{
		WithdrawalUUID: res.WithdrawalUUID, AdminID: "admin-7",
	})
	if err == nil {
		t.Fatal("reject without reason must fail")
	}
}

func TestReviewFlow_RejectAfterApproveFails(t *testing.T) {
	d := newReviewService("1000")
	res := initiateReviewWithdrawal(t, d, "0.1", "key-review-7")
	if _, err := d.svc.ApproveWithdrawal(context.Background(), DecideWithdrawalRequest{
		WithdrawalUUID: res.WithdrawalUUID, AdminID: "admin-7", IdempotencyKey: "decide-5",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.svc.RejectWithdrawal(context.Background(), DecideWithdrawalRequest{
		WithdrawalUUID: res.WithdrawalUUID, AdminID: "admin-7", Reason: "too late", IdempotencyKey: "decide-6",
	}); err == nil {
		t.Fatal("rejecting an approved withdrawal must fail")
	}
}

func TestReviewFlow_CancelByOwner(t *testing.T) {
	d := newReviewService("1000")
	res := initiateReviewWithdrawal(t, d, "0.1", "key-review-8")

	cancelled, err := d.svc.CancelWithdrawal(context.Background(), 42, res.WithdrawalUUID)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if cancelled.Status != domain.WithdrawalStatusCancelled {
		t.Fatalf("expected cancelled, got %s", cancelled.Status)
	}
	if d.unlocks.Load() != 1 {
		t.Fatalf("expected 1 unlock, got %d", d.unlocks.Load())
	}
}

func TestReviewFlow_CancelByStrangerFails(t *testing.T) {
	d := newReviewService("1000")
	res := initiateReviewWithdrawal(t, d, "0.1", "key-review-9")
	if _, err := d.svc.CancelWithdrawal(context.Background(), 777, res.WithdrawalUUID); err == nil {
		t.Fatal("cancel by another user must fail")
	}
}

func TestReviewFlow_CancelProcessingFails(t *testing.T) {
	d := newReviewService("1000")
	res := initiateReviewWithdrawal(t, d, "0.001", "key-instant-3") // already processing
	if _, err := d.svc.CancelWithdrawal(context.Background(), 42, res.WithdrawalUUID); err == nil {
		t.Fatal("cancelling a processing withdrawal must fail")
	}
}

func TestWithdrawalTransitions_ReviewMatrix(t *testing.T) {
	cases := []struct {
		from    domain.WithdrawalStatus
		to      domain.WithdrawalStatus
		allowed bool
	}{
		{domain.WithdrawalStatusPendingReview, domain.WithdrawalStatusApproved, true},
		{domain.WithdrawalStatusPendingReview, domain.WithdrawalStatusRejected, true},
		{domain.WithdrawalStatusPendingReview, domain.WithdrawalStatusCancelled, true},
		{domain.WithdrawalStatusPendingReview, domain.WithdrawalStatusProcessing, false},
		{domain.WithdrawalStatusApproved, domain.WithdrawalStatusProcessing, true},
		{domain.WithdrawalStatusApproved, domain.WithdrawalStatusRejected, false},
		{domain.WithdrawalStatusRejected, domain.WithdrawalStatusApproved, false},
		{domain.WithdrawalStatusProcessing, domain.WithdrawalStatusSending, true},
		{domain.WithdrawalStatusFinished, domain.WithdrawalStatusCancelled, false},
	}
	for _, tc := range cases {
		if got := tc.from.CanTransitionTo(tc.to); got != tc.allowed {
			t.Errorf("%s -> %s: got %v, want %v", tc.from, tc.to, got, tc.allowed)
		}
	}
	if !domain.WithdrawalStatusRejected.IsFinal() {
		t.Error("rejected must be final")
	}
	if domain.WithdrawalStatusApproved.IsFinal() {
		t.Error("approved must not be final")
	}
}

func TestReviewFlow_ApproveMissingWithdrawal(t *testing.T) {
	d := newReviewService("1000")
	_, err := d.svc.ApproveWithdrawal(context.Background(), DecideWithdrawalRequest{
		WithdrawalUUID: uuid.New().String(), AdminID: "admin-7", IdempotencyKey: "decide-7",
	})
	if err == nil {
		t.Fatal("approving unknown withdrawal must fail")
	}
	if !errors.Is(err, domain.ErrWithdrawalNotFound) && !isNotFound(err) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func isNotFound(err error) bool {
	var detailed *domain.DetailedError
	if errors.As(err, &detailed) {
		return detailed.Code == domain.ErrCodeWithdrawalNotFound
	}
	return false
}
