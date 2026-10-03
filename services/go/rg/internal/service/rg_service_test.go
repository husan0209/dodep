package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/opus-casino/rg/internal/domain"
	"github.com/opus-casino/rg/internal/repository"
)

// fakeRepo is an in-memory Repository (no DB required).
type fakeRepo struct {
	mu       sync.Mutex
	limits   map[int64]*domain.RGLimits
	pending  map[uuid.UUID]*domain.PendingChange
	excl     map[int64][]*domain.Exclusion
	timeouts map[int64][]*domain.Timeout
	spend    map[int64]map[string]*daySpend
	outbox   []string
}

type daySpend struct {
	dep, wag, pay decimal.Decimal
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		limits:   make(map[int64]*domain.RGLimits),
		pending:  make(map[uuid.UUID]*domain.PendingChange),
		excl:     make(map[int64][]*domain.Exclusion),
		timeouts: make(map[int64][]*domain.Timeout),
		spend:    make(map[int64]map[string]*daySpend),
	}
}

func (f *fakeRepo) GetLimits(_ context.Context, userID int64) (*domain.RGLimits, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if l, ok := f.limits[userID]; ok {
		cp := *l
		return &cp, nil
	}
	return &domain.RGLimits{UserID: userID}, nil
}

func (f *fakeRepo) UpsertLimits(_ context.Context, limits *domain.RGLimits) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *limits
	f.limits[limits.UserID] = &cp
	return nil
}

func (f *fakeRepo) CreatePendingChange(_ context.Context, ch *domain.PendingChange) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *ch
	f.pending[ch.ID] = &cp
	return nil
}

func (f *fakeRepo) ListPendingChanges(_ context.Context, userID int64) ([]*domain.PendingChange, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.PendingChange
	for _, ch := range f.pending {
		if ch.UserID == userID && ch.Status == domain.ChangePending {
			cp := *ch
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (f *fakeRepo) ListDueChanges(_ context.Context, now time.Time, limit int) ([]*domain.PendingChange, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.PendingChange
	for _, ch := range f.pending {
		if ch.Status == domain.ChangePending && !now.Before(ch.EffectiveAt) {
			cp := *ch
			out = append(out, &cp)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (f *fakeRepo) UpdateChangeStatus(_ context.Context, id uuid.UUID, from domain.ChangeStatus, to domain.ChangeStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	ch, ok := f.pending[id]
	if !ok || ch.Status != from {
		return domain.ErrConflict
	}
	ch.Status = to
	return nil
}

func (f *fakeRepo) DeletePendingChanges(_ context.Context, userID int64, limitType domain.LimitType) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, ch := range f.pending {
		if ch.UserID == userID && ch.LimitType == limitType && ch.Status == domain.ChangePending {
			delete(f.pending, id)
		}
	}
	return nil
}

func (f *fakeRepo) CreateExclusion(_ context.Context, e *domain.Exclusion) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *e
	f.excl[e.UserID] = append(f.excl[e.UserID], &cp)
	return nil
}

func (f *fakeRepo) GetActiveExclusion(_ context.Context, userID int64) (*domain.Exclusion, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	list := f.excl[userID]
	for i := len(list) - 1; i >= 0; i-- {
		if list[i].Status == domain.ExclusionActive {
			cp := *list[i]
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *fakeRepo) ListExclusions(_ context.Context, userID int64, _ int) ([]*domain.Exclusion, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.Exclusion
	for _, e := range f.excl[userID] {
		cp := *e
		out = append(out, &cp)
	}
	return out, nil
}

func (f *fakeRepo) UpdateExclusionStatus(_ context.Context, id uuid.UUID, from domain.ExclusionStatus, to domain.ExclusionStatus, patch map[string]interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, list := range f.excl {
		for _, e := range list {
			if e.ID == id {
				if e.Status != from || !e.Status.CanTransitionTo(to, e.Permanent) {
					return domain.ErrConflict
				}
				e.Status = to
				if v, ok := patch["revoked_by"].(string); ok {
					e.RevokedBy = v
				}
				if v, ok := patch["revoked_at"].(time.Time); ok {
					e.RevokedAt = &v
				}
				return nil
			}
		}
	}
	return domain.ErrConflict
}

func (f *fakeRepo) CreateTimeout(_ context.Context, t *domain.Timeout) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *t
	f.timeouts[t.UserID] = append(f.timeouts[t.UserID], &cp)
	return nil
}

func (f *fakeRepo) GetActiveTimeout(_ context.Context, userID int64, now time.Time) (*domain.Timeout, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, t := range f.timeouts[userID] {
		if t.IsActiveAt(now) {
			cp := *t
			return &cp, nil
		}
	}
	return nil, nil
}

func dayKey(t time.Time) string { return t.UTC().Truncate(24 * time.Hour).Format("2006-01-02") }

func (f *fakeRepo) RecordSpend(_ context.Context, userID int64, day time.Time, currency, deposits, wagers, payouts string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.spend[userID] == nil {
		f.spend[userID] = make(map[string]*daySpend)
	}
	k := dayKey(day) + "|" + currency
	ds := f.spend[userID][k]
	if ds == nil {
		ds = &daySpend{}
		f.spend[userID][k] = ds
	}
	ds.dep = ds.dep.Add(mustDec(deposits))
	ds.wag = ds.wag.Add(mustDec(wagers))
	ds.pay = ds.pay.Add(mustDec(payouts))
	return nil
}

func (f *fakeRepo) SumSpendSince(_ context.Context, userID int64, currency string, since time.Time) (string, string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var dep, wag, pay decimal.Decimal
	sinceKey := dayKey(since)
	for k, ds := range f.spend[userID] {
		parts := splitKey(k)
		if parts[1] != currency || parts[0] < sinceKey {
			continue
		}
		dep = dep.Add(ds.dep)
		wag = wag.Add(ds.wag)
		pay = pay.Add(ds.pay)
	}
	return dep.String(), wag.String(), pay.String(), nil
}

func splitKey(k string) []string {
	for i := len(k) - 1; i >= 0; i-- {
		if k[i] == '|' {
			return []string{k[:i], k[i+1:]}
		}
	}
	return []string{k, ""}
}

func mustDec(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero
	}
	return d
}

func (f *fakeRepo) AppendOutbox(_ context.Context, topic, _ string, _ map[string]interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.outbox = append(f.outbox, topic)
	return nil
}

func (f *fakeRepo) ListPendingOutbox(_ context.Context, _ int) ([]repository.OutboxEvent, error) {
	return nil, nil
}

func (f *fakeRepo) MarkOutboxPublished(_ context.Context, _ uuid.UUID) error { return nil }

func (f *fakeRepo) Transact(_ context.Context, fn func(repository.Repository) error) error {
	return fn(f)
}

var _ repository.Repository = (*fakeRepo)(nil)

func newTestService() (*RGService, *fakeRepo) {
	log, _ := zap.NewDevelopment()
	repo := newFakeRepo()
	return NewRGService(repo, NoopPublisher{}, log, 24, 24), repo
}

func dec(s string) decimal.Decimal { return mustDec(s) }

// ============ Enforcement ============

func TestWithdrawalAlwaysAllowed(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	d, err := svc.CheckPlayAllowed(ctx, CheckInput{UserID: 1, Channel: domain.ChannelWithdrawal})
	if err != nil || !d.Allowed {
		t.Fatalf("withdrawal must always be allowed: %+v %v", d, err)
	}
}

func TestExcludedBlocksEverything(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	if _, err := svc.StartSelfExclusion(ctx, StartSelfExclusionInput{
		UserID: 2, Period: domain.Exclusion30D, Type: domain.ExclusionSelf, By: "self",
	}); err != nil {
		t.Fatalf("exclude failed: %v", err)
	}
	for _, ch := range []domain.Channel{domain.ChannelBet, domain.ChannelGameLaunch, domain.ChannelDeposit} {
		d, err := svc.CheckPlayAllowed(ctx, CheckInput{UserID: 2, Channel: ch, Amount: dec("10"), Currency: "USD"})
		if err != nil {
			t.Fatalf("check failed: %v", err)
		}
		if d.Allowed || d.Reason != domain.ReasonSelfExcluded {
			t.Errorf("channel %s must be blocked with self-excluded, got %+v", ch, d)
		}
	}
	// Withdrawal still allowed.
	d, _ := svc.CheckPlayAllowed(ctx, CheckInput{UserID: 2, Channel: domain.ChannelWithdrawal})
	if !d.Allowed {
		t.Error("withdrawal must stay allowed for excluded users")
	}
}

func TestTimeoutBlocksBetNotWithdrawal(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	if _, err := svc.StartTimeout(ctx, 3, domain.Timeout24H); err != nil {
		t.Fatalf("timeout failed: %v", err)
	}
	d, _ := svc.CheckPlayAllowed(ctx, CheckInput{UserID: 3, Channel: domain.ChannelBet, Amount: dec("5"), Currency: "USD"})
	if d.Allowed || d.Reason != domain.ReasonCoolOff {
		t.Errorf("bet must be blocked by timeout: %+v", d)
	}
	d, _ = svc.CheckPlayAllowed(ctx, CheckInput{UserID: 3, Channel: domain.ChannelWithdrawal})
	if !d.Allowed {
		t.Error("withdrawal must stay allowed during timeout")
	}
}

func TestDepositLimitEnforced(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	d100 := dec("100")
	if _, err := svc.SetLimits(ctx, SetLimitsInput{UserID: 4, DepositDaily: &d100}); err != nil {
		t.Fatalf("set limits failed: %v", err)
	}
	if err := svc.RecordSpend(ctx, RecordSpendInput{UserID: 4, Kind: SpendDeposit, Amount: dec("90"), Currency: "USD"}); err != nil {
		t.Fatalf("record spend failed: %v", err)
	}
	d, _ := svc.CheckPlayAllowed(ctx, CheckInput{UserID: 4, Channel: domain.ChannelDeposit, Amount: dec("5"), Currency: "USD"})
	if !d.Allowed {
		t.Errorf("90+5 within 100 must pass: %+v", d)
	}
	d, _ = svc.CheckPlayAllowed(ctx, CheckInput{UserID: 4, Channel: domain.ChannelDeposit, Amount: dec("20"), Currency: "USD"})
	if d.Allowed || d.Reason != domain.ReasonDepositLimit {
		t.Fatalf("90+20 over 100 must fail: %+v", d)
	}
	if d.Remaining == nil || d.Remaining.String() != "10" {
		t.Errorf("remaining must be 10, got %+v", d.Remaining)
	}
}

func TestWagerAndLossLimits(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	// Wager scenario on its own user (no loss limit set).
	w50 := dec("50")
	if _, err := svc.SetLimits(ctx, SetLimitsInput{UserID: 5, WagerDaily: &w50}); err != nil {
		t.Fatalf("set limits failed: %v", err)
	}
	if err := svc.RecordSpend(ctx, RecordSpendInput{UserID: 5, Kind: SpendWager, Amount: dec("45"), Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	d, _ := svc.CheckPlayAllowed(ctx, CheckInput{UserID: 5, Channel: domain.ChannelBet, Amount: dec("10"), Currency: "USD"})
	if d.Allowed || d.Reason != domain.ReasonWagerLimit {
		t.Errorf("wager limit must trigger: %+v", d)
	}
	d, _ = svc.CheckPlayAllowed(ctx, CheckInput{UserID: 5, Channel: domain.ChannelBet, Amount: dec("5"), Currency: "USD"})
	if !d.Allowed {
		t.Errorf("45+5 within 50 must pass: %+v", d)
	}
	// Loss scenario on its own user (no wager limit set).
	if err := svc.RecordSpend(ctx, RecordSpendInput{UserID: 6, Kind: SpendWager, Amount: dec("100"), Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecordSpend(ctx, RecordSpendInput{UserID: 6, Kind: SpendPayout, Amount: dec("65"), Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	l40 := dec("40")
	if _, err := svc.SetLimits(ctx, SetLimitsInput{UserID: 6, LossDaily: &l40}); err != nil {
		t.Fatal(err)
	}
	// Net loss 35 < 40 passes.
	d, _ = svc.CheckPlayAllowed(ctx, CheckInput{UserID: 6, Channel: domain.ChannelBet, Amount: dec("0"), Currency: "USD"})
	if !d.Allowed {
		t.Errorf("loss 35 < 40 must pass: %+v", d)
	}
	// Push loss to 45 >= 40: blocked.
	if err := svc.RecordSpend(ctx, RecordSpendInput{UserID: 6, Kind: SpendWager, Amount: dec("10"), Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	d, _ = svc.CheckPlayAllowed(ctx, CheckInput{UserID: 6, Channel: domain.ChannelBet, Amount: dec("0"), Currency: "USD"})
	if d.Allowed || d.Reason != domain.ReasonLossLimit {
		t.Errorf("loss limit must trigger: %+v", d)
	}
}

func TestInvalidChannelAndAmount(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	if _, err := svc.CheckPlayAllowed(ctx, CheckInput{UserID: 1, Channel: "slot"}); !errors.Is(err, domain.ErrInvalidChannel) {
		t.Errorf("expected invalid channel, got %v", err)
	}
	if _, err := svc.CheckPlayAllowed(ctx, CheckInput{UserID: 1, Channel: domain.ChannelBet, Amount: dec("-5")}); err == nil {
		t.Error("expected negative amount rejection")
	}
}

// ============ Limits ============

func TestDecreaseImmediateIncreasePending(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	d500, d100, d1000 := dec("500"), dec("100"), dec("1000")
	if _, err := svc.SetLimits(ctx, SetLimitsInput{UserID: 7, DepositDaily: &d500}); err != nil {
		t.Fatal(err)
	}
	res, err := svc.SetLimits(ctx, SetLimitsInput{UserID: 7, DepositDaily: &d100})
	if err != nil {
		t.Fatal(err)
	}
	if res.Limits.DepositDaily.String() != "100" || len(res.Pending) != 0 {
		t.Fatalf("decrease must apply immediately: %+v", res.Limits)
	}
	res, err = svc.SetLimits(ctx, SetLimitsInput{UserID: 7, DepositDaily: &d1000})
	if err != nil {
		t.Fatal(err)
	}
	if res.Limits.DepositDaily.String() != "100" {
		t.Fatalf("increase must not apply yet: %s", res.Limits.DepositDaily)
	}
	if len(res.Pending) != 1 || res.Pending[0].NewValue.String() != "1000" {
		t.Fatalf("expected one pending increase: %+v", res.Pending)
	}
	if until := time.Until(res.Pending[0].EffectiveAt); until < 23*time.Hour || until > 25*time.Hour {
		t.Errorf("cooling must be ~24h, got %v", until)
	}
	_ = repo
}

func TestApplyDueIncreases(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	d100, d900 := dec("100"), dec("900")
	if _, err := svc.SetLimits(ctx, SetLimitsInput{UserID: 8, DepositDaily: &d100}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetLimits(ctx, SetLimitsInput{UserID: 8, DepositDaily: &d900}); err != nil {
		t.Fatal(err)
	}
	n, err := svc.ApplyDueIncreases(ctx, time.Now().UTC(), 100)
	if err != nil || n != 0 {
		t.Fatalf("nothing due yet: %d %v", n, err)
	}
	n, err = svc.ApplyDueIncreases(ctx, time.Now().UTC().Add(25*time.Hour), 100)
	if err != nil || n != 1 {
		t.Fatalf("one change must apply: %d %v", n, err)
	}
	lim, _ := repo.GetLimits(ctx, 8)
	if lim.DepositDaily.String() != "900" {
		t.Fatalf("increase must be effective: %s", lim.DepositDaily)
	}
	// Overtaken increase is replaced, never stacked: 700 pending, then 60
	// (above the effective 50, so it also cools) replaces it.
	d50 := dec("50")
	if _, err := svc.SetLimits(ctx, SetLimitsInput{UserID: 8, DepositDaily: &d50}); err != nil {
		t.Fatal(err)
	}
	d700 := dec("700")
	if _, err := svc.SetLimits(ctx, SetLimitsInput{UserID: 8, DepositDaily: &d700}); err != nil {
		t.Fatal(err)
	}
	d60 := dec("60")
	res, err := svc.SetLimits(ctx, SetLimitsInput{UserID: 8, DepositDaily: &d60})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Pending) != 1 || res.Pending[0].NewValue.String() != "60" {
		t.Fatalf("60 must replace pending 700 (still cooling): %+v", res.Pending)
	}
	if res.Limits.DepositDaily.String() != "50" {
		t.Fatalf("effective must stay 50 during cooling: %s", res.Limits.DepositDaily)
	}
	n, err = svc.ApplyDueIncreases(ctx, time.Now().UTC().Add(50*time.Hour), 100)
	if err != nil || n != 1 {
		t.Fatalf("replacement change must apply: %d %v", n, err)
	}
	lim, _ = repo.GetLimits(ctx, 8)
	if lim.DepositDaily.String() != "60" {
		t.Fatalf("effective must become 60: %s", lim.DepositDaily)
	}
}

func TestSetLimitsValidation(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	neg := dec("-1")
	if _, err := svc.SetLimits(ctx, SetLimitsInput{UserID: 1, DepositDaily: &neg}); err == nil {
		t.Error("negative limit must be rejected")
	}
	badMin := -5
	if _, err := svc.SetLimits(ctx, SetLimitsInput{UserID: 1, SessionMinutes: &badMin}); err == nil {
		t.Error("negative minutes must be rejected")
	}
	badRC := 5
	if _, err := svc.SetLimits(ctx, SetLimitsInput{UserID: 1, RealityCheckMinutes: &badRC}); err == nil {
		t.Error("reality check below 15 must be rejected")
	}
}

// ============ Exclusion ============

func TestSelfExclusionLifecycle(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	for _, p := range []domain.ExclusionPeriod{domain.Exclusion24H, domain.Exclusion7D, domain.Exclusion30D, domain.Exclusion6M, domain.Exclusion1Y} {
		excl, err := svc.StartSelfExclusion(ctx, StartSelfExclusionInput{UserID: 10, Period: p, Type: domain.ExclusionSelf, By: "self"})
		if err != nil {
			t.Fatalf("period %s failed: %v", p, err)
		}
		if excl.Permanent || excl.Until == nil {
			t.Fatalf("period %s must be temporary with until", p)
		}
		// Second exclusion while active must fail; reset by expiring manually.
		if _, err := svc.StartSelfExclusion(ctx, StartSelfExclusionInput{UserID: 10, Period: p, Type: domain.ExclusionSelf}); !errors.Is(err, domain.ErrExclusionExists) {
			t.Fatalf("duplicate must fail: %v", err)
		}
		break // one iteration is enough; duplicates covered
	}
	if _, err := svc.StartSelfExclusion(ctx, StartSelfExclusionInput{UserID: 11, Period: "2y", Type: domain.ExclusionSelf}); err == nil {
		t.Error("invalid period must be rejected")
	}
	perm, err := svc.StartSelfExclusion(ctx, StartSelfExclusionInput{UserID: 12, Period: domain.ExclusionPermanent, Type: domain.ExclusionSelf, By: "self"})
	if err != nil || !perm.Permanent {
		t.Fatalf("permanent failed: %+v %v", perm, err)
	}
	if err := svc.RevokeSelfExclusion(ctx, 12, true, "self"); !errors.Is(err, domain.ErrPermanentExclusion) {
		t.Errorf("permanent must never revoke: %v", err)
	}
}

func TestRevokeRules(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	if err := svc.RevokeSelfExclusion(ctx, 99, true, "self"); !errors.Is(err, domain.ErrExclusionNotFound) {
		t.Errorf("missing exclusion must 404: %v", err)
	}
	excl, err := svc.StartSelfExclusion(ctx, StartSelfExclusionInput{UserID: 20, Period: domain.Exclusion24H, Type: domain.ExclusionSelf, By: "self"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RevokeSelfExclusion(ctx, 20, false, "self"); !errors.Is(err, domain.ErrConfirmRequired) {
		t.Errorf("confirm required: %v", err)
	}
	if err := svc.RevokeSelfExclusion(ctx, 20, true, "self"); !errors.Is(err, domain.ErrRevokeTooEarly) {
		t.Errorf("active must not revoke: %v", err)
	}
	// Simulate expiry 1h ago (revoke cooling of 24h not elapsed).
	past := time.Now().UTC().Add(-1 * time.Hour)
	repo.mu.Lock()
	for _, list := range repo.excl {
		for _, e := range list {
			if e.ID == excl.ID {
				e.Until = &past
			}
		}
	}
	repo.mu.Unlock()
	if err := svc.RevokeSelfExclusion(ctx, 20, true, "self"); err == nil {
		t.Error("revoke cooling must still block (only 1h since expiry)")
	} else if !errors.Is(err, domain.ErrRevokeCooling) {
		t.Errorf("expected cooling error, got %v", err)
	}
	// Expire long ago: revocation succeeds.
	older := time.Now().UTC().Add(-72 * time.Hour)
	repo.mu.Lock()
	for _, list := range repo.excl {
		for _, e := range list {
			if e.ID == excl.ID {
				e.Until = &older
			}
		}
	}
	repo.mu.Unlock()
	if err := svc.RevokeSelfExclusion(ctx, 20, true, "self"); err != nil {
		t.Errorf("revoke must succeed after cooling: %v", err)
	}
}

func TestTimeoutDuplicate(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	if _, err := svc.StartTimeout(ctx, 30, domain.Timeout7D); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartTimeout(ctx, 30, domain.Timeout24H); !errors.Is(err, domain.ErrTimeoutExists) {
		t.Errorf("duplicate timeout must fail: %v", err)
	}
	if _, err := svc.StartTimeout(ctx, 31, "1y"); err == nil {
		t.Error("invalid timeout period must fail")
	}
}

func TestRecordSpendValidation(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	if err := svc.RecordSpend(ctx, RecordSpendInput{UserID: 1, Kind: "lottery", Amount: dec("1"), Currency: "USD"}); err == nil {
		t.Error("bad kind must fail")
	}
	if err := svc.RecordSpend(ctx, RecordSpendInput{UserID: 1, Kind: SpendDeposit, Amount: dec("-1"), Currency: "USD"}); err == nil {
		t.Error("negative amount must fail")
	}
	if err := svc.RecordSpend(ctx, RecordSpendInput{UserID: 1, Kind: SpendDeposit, Amount: dec("1"), Currency: "US"}); err == nil {
		t.Error("bad currency must fail")
	}
}

func TestGetStatus(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	st, err := svc.GetStatus(ctx, 40)
	if err != nil || !st.GamblingAllowed || st.BlockedReason != domain.ReasonOK {
		t.Fatalf("clean user must be allowed: %+v %v", st, err)
	}
	if st.Limits.EffectiveRealityCheck() != domain.DefaultRealityCheckMinutes {
		t.Error("default reality check must apply")
	}
	if _, err := svc.StartSelfExclusion(ctx, StartSelfExclusionInput{UserID: 40, Period: domain.Exclusion7D, Type: domain.ExclusionSelf, By: "self"}); err != nil {
		t.Fatal(err)
	}
	st, err = svc.GetStatus(ctx, 40)
	if err != nil || st.GamblingAllowed || st.Exclusion == nil {
		t.Fatalf("excluded user must be blocked: %+v %v", st, err)
	}
	// Time-out blocks with cool-off reason on a non-excluded user.
	if _, err := svc.StartTimeout(ctx, 41, domain.Timeout7D); err != nil {
		t.Fatal(err)
	}
	st, err = svc.GetStatus(ctx, 41)
	if err != nil || st.GamblingAllowed || st.BlockedReason != domain.ReasonCoolOff || st.Timeout == nil {
		t.Fatalf("timed-out user must be blocked: %+v %v", st, err)
	}
}

func TestNewRGServiceRequiresRepo(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil repo")
		}
	}()
	log, _ := zap.NewDevelopment()
	NewRGService(nil, NoopPublisher{}, log, 24, 24)
}
