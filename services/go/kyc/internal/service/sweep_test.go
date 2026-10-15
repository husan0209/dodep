package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/opus-casino/kyc/internal/cache"
	"github.com/opus-casino/kyc/internal/crypto"
	"github.com/opus-casino/kyc/internal/domain"
	"github.com/opus-casino/kyc/internal/repository"
	"github.com/opus-casino/kyc/internal/screening"
	"github.com/opus-casino/kyc/internal/testutil"
)

// ─── Batch-paging fake (limit-respecting MarkExpiredBefore) ───

type sweepBatchRepo struct {
	mu        sync.Mutex
	remaining int
	calls     int
	batches   []int
	audits    int
	fail      error
}

var _ repository.KYCRepository = (*sweepBatchRepo)(nil)

func (f *sweepBatchRepo) GetRecord(_ context.Context, _ int64) (*domain.KycRecord, error) {
	return nil, nil
}

func (f *sweepBatchRepo) UpsertRecord(_ context.Context, _ *domain.KycRecord) error {
	return nil
}

func (f *sweepBatchRepo) CreateDocument(_ context.Context, _ *domain.Document) error {
	return nil
}

func (f *sweepBatchRepo) GetDocument(_ context.Context, _ uuid.UUID) (*domain.Document, error) {
	return nil, nil
}

func (f *sweepBatchRepo) ListDocuments(_ context.Context, _ int64, _ *domain.DocumentType, _ *domain.DocumentStatus, _ int, _ string) ([]*domain.Document, string, bool, error) {
	return nil, "", false, nil
}

func (f *sweepBatchRepo) UpdateDocument(_ context.Context, _ *domain.Document) error {
	return nil
}

func (f *sweepBatchRepo) DeleteDocument(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (f *sweepBatchRepo) FindByNumberHash(_ context.Context, _ string, _ int64) (*domain.Document, error) {
	return nil, nil
}

func (f *sweepBatchRepo) MarkExpiredBefore(_ context.Context, _ time.Time, limit int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.fail != nil {
		return 0, f.fail
	}
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	n := limit
	if f.remaining < n {
		n = f.remaining
	}
	f.remaining -= n
	f.batches = append(f.batches, n)
	return int64(n), nil
}

func (f *sweepBatchRepo) CreateScreening(_ context.Context, _ *domain.Screening) error {
	return nil
}

func (f *sweepBatchRepo) GetScreening(_ context.Context, _ uuid.UUID) (*domain.Screening, error) {
	return nil, nil
}

func (f *sweepBatchRepo) ListPendingScreenings(_ context.Context, _ int) ([]*domain.Screening, error) {
	return nil, nil
}

func (f *sweepBatchRepo) UpdateScreening(_ context.Context, _ *domain.Screening) error {
	return nil
}

func (f *sweepBatchRepo) LatestScreening(_ context.Context, _ int64) (*domain.Screening, error) {
	return nil, nil
}

func (f *sweepBatchRepo) AppendAudit(_ context.Context, _ *domain.AuditEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.audits++
	return nil
}

func (f *sweepBatchRepo) ListAudit(_ context.Context, _ int64, _ int) ([]*domain.AuditEntry, error) {
	return nil, nil
}

func newSweepServiceWithRepo(t *testing.T, repo repository.KYCRepository) *KYCService {
	t.Helper()
	cr, err := crypto.Ephemeral()
	require.NoError(t, err)
	return NewKYCService(repo, cache.NewMemory(), screening.StubProvider{}, cr,
		Config{WebhookSecret: "test-secret"}, zap.NewNop())
}

func TestSweeper_RunOncePagesBatches(t *testing.T) {
	repo := &sweepBatchRepo{remaining: 450}
	svc := newSweepServiceWithRepo(t, repo)
	sweeper := NewSweeper(svc, zap.NewNop(), WithSweepBatchSize(200))

	total, err := sweeper.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(450), total)
	assert.Equal(t, 3, repo.calls, "200 + 200 + short 50 batch")
	assert.Equal(t, []int{200, 200, 50}, repo.batches)
	assert.Equal(t, 1, repo.audits, "one summary audit per tick, not per batch")
}

func TestSweeper_RunOnceRespectsMaxBatches(t *testing.T) {
	repo := &sweepBatchRepo{remaining: 1000}
	svc := newSweepServiceWithRepo(t, repo)
	sweeper := NewSweeper(svc, zap.NewNop(), WithSweepBatchSize(200), WithSweepMaxBatches(2))

	total, err := sweeper.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(400), total)
	assert.Equal(t, 2, repo.calls)
}

func TestSweeper_RunOncePropagatesError(t *testing.T) {
	repo := &sweepBatchRepo{remaining: 10, fail: errors.New("db down")}
	svc := newSweepServiceWithRepo(t, repo)
	sweeper := NewSweeper(svc, zap.NewNop())

	total, err := sweeper.RunOnce(context.Background())
	require.Error(t, err)
	assert.Equal(t, int64(0), total)
}

func TestSweeper_RunOnceNothingToDo(t *testing.T) {
	repo := &sweepBatchRepo{}
	svc := newSweepServiceWithRepo(t, repo)
	sweeper := NewSweeper(svc, zap.NewNop())

	total, err := sweeper.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Equal(t, 1, repo.calls, "single short batch proves the queue is empty")
}

// ─── SweepUser + cache invalidation ───

func newSweepHarness(t *testing.T) (*harness, *cache.MemoryStatusCache) {
	t.Helper()
	repo := testutil.NewFakeRepo()
	mem := cache.NewMemory()
	cr, err := crypto.Ephemeral()
	require.NoError(t, err)
	svc := NewKYCService(repo, mem, screening.StubProvider{}, cr,
		Config{WebhookSecret: "test-secret"}, zap.NewNop())
	return &harness{svc: svc, repo: repo}, mem
}

func TestSweeper_SweepUserInvalidatesCache(t *testing.T) {
	h, mem := newSweepHarness(t)
	ctx := context.Background()
	p := submitPassport(t, h, 30, "ID3000")
	s := submitSelfie(t, h, 30)
	verifyDoc(t, h, p.ID, "Jane Doe")
	verifyDoc(t, h, s.ID, "Jane Doe")

	st, err := h.svc.GetStatus(ctx, 30)
	require.NoError(t, err)
	require.Equal(t, domain.StatusVerified, st.Record.Status)

	// Simulate passport expiry after verification.
	past := time.Now().Add(-time.Hour)
	h.repo.SetDocExpiry(p.ID, &past)

	// The cached snapshot is now stale (still verified).
	cached, err := mem.GetStatus(ctx, 30)
	require.NoError(t, err)
	require.NotNil(t, cached)
	require.Equal(t, domain.StatusVerified, cached.Status)

	sweeper := NewSweeper(h.svc, zap.NewNop())
	changed, err := sweeper.SweepUser(ctx, 30)
	require.NoError(t, err)
	assert.True(t, changed)

	// Invalidation is real: the entry is gone until the next read refreshes it.
	cached, err = mem.GetStatus(ctx, 30)
	require.NoError(t, err)
	assert.Nil(t, cached, "status cache must be invalidated on downgrade")

	st, err = h.svc.GetStatus(ctx, 30)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusExpired, st.Record.Status)
	assert.Equal(t, domain.LevelNone, st.Record.Level)
}

func TestSweeper_SweepUserInvalidUser(t *testing.T) {
	h, _ := newSweepHarness(t)
	sweeper := NewSweeper(h.svc, zap.NewNop())
	_, err := sweeper.SweepUser(context.Background(), 0)
	assert.ErrorIs(t, err, domain.ErrInvalidUserID)
}

// ─── Review-outcome events ───

type fakePublisher struct {
	mu     sync.Mutex
	topics []string
	events []domain.KYCStatusEvent
}

func (f *fakePublisher) PublishKYCEvent(_ context.Context, topic string, e domain.KYCStatusEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.topics = append(f.topics, topic)
	f.events = append(f.events, e)
	return nil
}

type errPublisher struct{}

func (errPublisher) PublishKYCEvent(_ context.Context, _ string, _ domain.KYCStatusEvent) error {
	return errors.New("redpanda down")
}

func TestVerify_PublishesVerifiedEvent(t *testing.T) {
	h, _ := newSweepHarness(t)
	fp := &fakePublisher{}
	h.svc.SetEventPublisher(fp)

	p := submitPassport(t, h, 40, "ID4000")
	s := submitSelfie(t, h, 40)
	verifyDoc(t, h, p.ID, "Jane Doe")
	verifyDoc(t, h, s.ID, "Jane Doe")

	require.Len(t, fp.topics, 1, "only the promotion that lands on verified publishes")
	assert.Equal(t, domain.TopicKYCVerified, fp.topics[0])
	assert.Equal(t, "kyc.verified", fp.topics[0])
	ev := fp.events[0]
	assert.Equal(t, int64(40), ev.UserID)
	assert.Equal(t, domain.LevelIdentity, ev.Level)
	assert.Equal(t, domain.StatusVerified, ev.Status)
	assert.NotEmpty(t, ev.EventID)
	assert.False(t, ev.OccurredAt.IsZero())
}

func TestVerify_ScreeningHoldPublishesNothing(t *testing.T) {
	h, _ := newSweepHarness(t)
	fp := &fakePublisher{}
	h.svc.SetEventPublisher(fp)

	p := submitPassport(t, h, 41, "ID4100")
	s := submitSelfie(t, h, 41)
	verifyDoc(t, h, p.ID, "Robert PEP Johnson")
	verifyDoc(t, h, s.ID, "Robert PEP Johnson")

	assert.Empty(t, fp.topics, "flagged users are never announced as verified")
}

func TestVerify_PublishFailureDoesNotFailReview(t *testing.T) {
	h, _ := newSweepHarness(t)
	h.svc.SetEventPublisher(errPublisher{})
	p := submitPassport(t, h, 43, "ID4300")
	_, err := h.svc.VerifyDocument(context.Background(), VerifyInput{DocumentID: p.ID, VerifiedBy: "admin-1"})
	require.NoError(t, err, "best-effort events must never fail a review")
}

func TestReject_PublishesRejectedEvent(t *testing.T) {
	h, _ := newSweepHarness(t)
	fp := &fakePublisher{}
	h.svc.SetEventPublisher(fp)

	p := submitPassport(t, h, 42, "ID4200")
	_, err := h.svc.RejectDocument(context.Background(), RejectInput{
		DocumentID: p.ID, RejectedBy: "admin-1", Reason: "blurry image",
	})
	require.NoError(t, err)

	require.Len(t, fp.topics, 1)
	assert.Equal(t, domain.TopicKYCRejected, fp.topics[0])
	assert.Equal(t, "kyc.rejected", fp.topics[0])
	assert.Equal(t, int64(42), fp.events[0].UserID)
	assert.Equal(t, domain.StatusRejected, fp.events[0].Status)
	assert.Equal(t, "blurry image", fp.events[0].Reason)
}

// ─── Custom VIP limits (service layer) ───

func strPtr(s string) *string { return &s }

func TestUpsertLimitOverride_AdminOnly(t *testing.T) {
	h, _ := newSweepHarness(t)
	ctx := context.Background()

	valid := &domain.LimitOverride{UserID: 50, DailyDeposit: strPtr("999999"), UpdatedBy: "admin-1"}

	for _, role := range []string{"", "user", "moderator", "support"} {
		err := h.svc.UpsertLimitOverride(ctx, role, valid)
		assert.ErrorIs(t, err, domain.ErrForbidden, "role %q must be refused", role)
	}

	// Admin role is case-insensitive (mirrors the JWT is_admin claim check).
	require.NoError(t, h.svc.UpsertLimitOverride(ctx, "Admin", valid))

	// Invalid terms are rejected even for admins.
	bad := &domain.LimitOverride{UserID: 50, DailyDeposit: strPtr("10.5"), UpdatedBy: "admin-1"}
	err := h.svc.UpsertLimitOverride(ctx, "admin", bad)
	assert.Error(t, err)
	var ve *domain.ValidationError
	assert.ErrorAs(t, err, &ve)

	// Upsert is audit-logged.
	found := false
	for _, a := range h.repo.Audits {
		if a.Action == domain.AuditLimitOverride && a.UserID == 50 {
			found = true
		}
	}
	assert.True(t, found, "override upsert must append an audit entry")
}

func TestEffectiveLimits_MergesOverride(t *testing.T) {
	h, _ := newSweepHarness(t)
	ctx := context.Background()

	// Unknown user: base LevelNone limits, no override.
	lim, err := h.svc.EffectiveLimits(ctx, 9999)
	require.NoError(t, err)
	assert.Equal(t, domain.LimitsForLevel(domain.LevelNone), lim)

	// Promote user 50 to Identity, then customise one field.
	p := submitPassport(t, h, 50, "ID5000")
	s := submitSelfie(t, h, 50)
	verifyDoc(t, h, p.ID, "Jane Doe")
	verifyDoc(t, h, s.ID, "Jane Doe")

	lim, err = h.svc.EffectiveLimits(ctx, 50)
	require.NoError(t, err)
	assert.Equal(t, "5000", lim.DailyDeposit)

	require.NoError(t, h.svc.UpsertLimitOverride(ctx, "admin", &domain.LimitOverride{
		UserID: 50, DailyDeposit: strPtr("777000"), UpdatedBy: "admin-1",
	}))
	lim, err = h.svc.EffectiveLimits(ctx, 50)
	require.NoError(t, err)
	assert.Equal(t, "777000", lim.DailyDeposit, "override wins per field")
	assert.Equal(t, "2000", lim.DailyWithdrawal, "unset fields inherit the base tier")
	assert.Equal(t, "500", lim.MaxBet)

	_, err = h.svc.EffectiveLimits(ctx, 0)
	assert.ErrorIs(t, err, domain.ErrInvalidUserID)
}
