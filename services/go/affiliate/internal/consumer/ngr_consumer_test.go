package consumer

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/opus-casino/affiliate/internal/domain"
	"github.com/opus-casino/affiliate/internal/repository"
	"github.com/opus-casino/affiliate/internal/service"
)

// fakeRepo embeds the repository interface so only the methods used by the
// consumer need overrides; anything unexpected panics loudly.
type fakeRepo struct {
	repository.AffiliateRepository
	profile            *domain.AffiliateProfile
	attribution        *domain.AffiliateAttribution
	createdEarning     *domain.AffiliateEarning
	createdAttribution *domain.AffiliateAttribution
}

func (f *fakeRepo) GetProfileByID(_ context.Context, _ uuid.UUID) (*domain.AffiliateProfile, error) {
	return f.profile, nil
}

func (f *fakeRepo) GetAttributionByReferredUserID(_ context.Context, _ int64) (*domain.AffiliateAttribution, error) {
	return f.attribution, nil
}

func (f *fakeRepo) CreateAttribution(_ context.Context, a *domain.AffiliateAttribution) error {
	f.createdAttribution = a
	return nil
}

func (f *fakeRepo) CreateEarning(_ context.Context, e *domain.AffiliateEarning) error {
	f.createdEarning = e
	return nil
}

func (f *fakeRepo) GetClickByClickID(_ context.Context, _ string) (*domain.AffiliateClick, error) {
	return nil, nil
}

func (f *fakeRepo) CountClicksSince(_ context.Context, _ uuid.UUID, _ time.Time) (int64, error) {
	return 0, nil
}

func (f *fakeRepo) CountAttributionsSince(_ context.Context, _ uuid.UUID, _ time.Time) (int64, error) {
	return 0, nil
}

func (f *fakeRepo) CountReferredUsersByDevice(_ context.Context, _ uuid.UUID, _ string, _ time.Time) (int64, error) {
	return 0, nil
}

func (f *fakeRepo) CountReferredUsersByIP(_ context.Context, _ uuid.UUID, _ string, _ time.Time) (int64, error) {
	return 0, nil
}

func activeProfile() *domain.AffiliateProfile {
	return &domain.AffiliateProfile{
		ID:               uuid.New(),
		UserID:           1001,
		Status:           domain.AffiliateStatusActive,
		AffiliateCode:    "AFF-TEST-01",
		CommissionRate:   decimal.RequireFromString("0.20"),
		HoldPeriodDays:   14,
		MinPayoutAmount:  decimal.RequireFromString("100"),
		Currency:         "USD",
		ApprovalMode:     domain.ApprovalModeManual,
		PayoutSchedule:   domain.PayoutScheduleMonthly,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
}

func newConsumer(repo *fakeRepo, batchSize int) *NGRConsumer {
	cfg := DefaultNGRConsumerConfig()
	cfg.BatchSize = batchSize
	svc := service.NewAffiliateService(repo, zap.NewNop())
	return NewNGRConsumer(svc, zap.NewNop(), cfg)
}

func mustMsg(t *testing.T, topic string, v any) Message {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	return Message{Topic: topic, Value: raw}
}

func ngrEvent(affiliateID string, userID int64) NGREvent {
	now := time.Now().UTC()
	return NGREvent{
		EventID:        uuid.NewString(),
		EventType:      "settled",
		UserID:         userID,
		AffiliateID:    affiliateID,
		GGRAmount:      "300.00",
		NGRAmount:      "200.00",
		SourceType:     "casino",
		SourceID:       "round-1",
		PeriodStart:    now.Add(-time.Hour),
		PeriodEnd:      now,
		Currency:       "USD",
		IdempotencyKey: uuid.NewString(),
	}
}

func TestProcessMessage_UnknownTopicIgnored(t *testing.T) {
	repo := &fakeRepo{}
	c := newConsumer(repo, 10)
	if err := c.ProcessMessage(context.Background(), Message{Topic: "random.topic", Value: []byte("{}")}); err != nil {
		t.Fatalf("expected nil for unknown topic, got %v", err)
	}
	if got := c.GetMetrics().EventsProcessed; got != 0 {
		t.Fatalf("expected no processed events, got %d", got)
	}
}

func TestProcessMessage_MalformedNGRSkipped(t *testing.T) {
	repo := &fakeRepo{}
	c := newConsumer(repo, 10)
	msg := Message{Topic: "casino.rounds.settled", Value: []byte("{not-json")}
	if err := c.ProcessMessage(context.Background(), msg); err != nil {
		t.Fatalf("malformed messages must be skipped without error, got %v", err)
	}
	c.flushBatch(context.Background())
	if repo.createdEarning != nil {
		t.Fatal("no commission must be calculated for malformed events")
	}
}

func TestProcessMessage_NGRWithoutAttributionSkipped(t *testing.T) {
	repo := &fakeRepo{}
	c := newConsumer(repo, 10)
	if err := c.ProcessMessage(context.Background(), mustMsg(t, "betting.bets.settled", ngrEvent("", 777))); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	c.flushBatch(context.Background())
	if repo.createdEarning != nil {
		t.Fatal("no commission must be calculated without affiliate attribution")
	}
}

func TestProcessMessage_NGRCalculatesCommission(t *testing.T) {
	profile := activeProfile()
	repo := &fakeRepo{profile: profile}
	c := newConsumer(repo, 10)

	before := time.Now().UTC()
	if err := c.ProcessMessage(context.Background(), mustMsg(t, "casino.rounds.settled", ngrEvent(profile.ID.String(), 2002))); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	c.flushBatch(context.Background())

	e := repo.createdEarning
	if e == nil {
		t.Fatal("expected commission earning to be created")
	}
	if got, want := e.CommissionAmount.String(), "40"; got != want {
		t.Fatalf("expected commission 200*0.20=%s, got %s", want, got)
	}
	if e.Status != domain.EarningStatusAccrued {
		t.Fatalf("expected status accrued, got %s", e.Status)
	}
	if !e.HoldUntil.After(before.AddDate(0, 0, 13)) || !e.HoldUntil.Before(before.AddDate(0, 0, 15)) {
		t.Fatalf("hold_until must be ~14 days out, got %v", e.HoldUntil)
	}
	if c.GetMetrics().CommissionsCalc != 1 {
		t.Fatalf("expected 1 calculated commission, got %d", c.GetMetrics().CommissionsCalc)
	}
}

func TestProcessMessage_AttributionBindsAndWarmsCache(t *testing.T) {
	profile := activeProfile()
	repo := &fakeRepo{profile: profile}
	c := newConsumer(repo, 10)
	ctx := context.Background()

	attr := AttributionEvent{
		EventID:      uuid.NewString(),
		EventType:    "registration",
		UserID:       3003,
		AffiliateID:  profile.ID.String(),
		ClickID:      "click-1",
		AttributedAt: time.Now().UTC(),
		Timestamp:    time.Now().UTC(),
	}
	if err := c.ProcessMessage(ctx, mustMsg(t, "affiliate.player.activity", attr)); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if repo.createdAttribution == nil || repo.createdAttribution.ReferredUserID != 3003 {
		t.Fatalf("expected attribution for user 3003, got %+v", repo.createdAttribution)
	}

	// Same user now produces NGR without affiliate_id: cache must resolve it.
	if err := c.ProcessMessage(ctx, mustMsg(t, "betting.bets.settled", ngrEvent("", 3003))); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	c.flushBatch(ctx)
	if repo.createdEarning == nil {
		t.Fatal("expected commission via cached attribution")
	}
	if repo.createdEarning.AffiliateID != profile.ID {
		t.Fatalf("expected affiliate %s, got %s", profile.ID, repo.createdEarning.AffiliateID)
	}
}

func TestProcessMessage_SelfReferralDoesNotFailConsumer(t *testing.T) {
	profile := activeProfile()
	repo := &fakeRepo{profile: profile}
	c := newConsumer(repo, 10)

	// Referred user == affiliate owner: BindReferredUser refuses, consumer must not error.
	attr := AttributionEvent{
		EventID:      uuid.NewString(),
		EventType:    "registration",
		UserID:       profile.UserID,
		AffiliateID:  profile.ID.String(),
		AttributedAt: time.Now().UTC(),
		Timestamp:    time.Now().UTC(),
	}
	if err := c.ProcessMessage(context.Background(), mustMsg(t, "affiliate.player.activity", attr)); err != nil {
		t.Fatalf("consumer must swallow bind errors, got %v", err)
	}
	if repo.createdAttribution != nil {
		t.Fatal("self-referral attribution must not be created")
	}
}

func TestCalculateCommission_InvalidInput(t *testing.T) {
	repo := &fakeRepo{profile: activeProfile()}
	c := newConsumer(repo, 10)
	ctx := context.Background()

	if err := c.calculateCommission(ctx, NGREvent{AffiliateID: "not-a-uuid", NGRAmount: "10"}); err == nil {
		t.Fatal("expected error for invalid affiliate_id")
	}
	if err := c.calculateCommission(ctx, NGREvent{AffiliateID: uuid.NewString(), NGRAmount: "nope"}); err == nil {
		t.Fatal("expected error for invalid ngr_amount")
	}
}

func TestBatchAutoFlushOnFull(t *testing.T) {
	profile := activeProfile()
	repo := &fakeRepo{profile: profile}
	c := newConsumer(repo, 1) // flush after every event (async)

	if err := c.ProcessMessage(context.Background(), mustMsg(t, "casino.rounds.settled", ngrEvent(profile.ID.String(), 4004))); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for repo.createdEarning == nil && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if repo.createdEarning == nil {
		t.Fatal("expected async batch flush to calculate commission")
	}
}
