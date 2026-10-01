package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/opus-casino/bonus/internal/domain"
	"github.com/opus-casino/bonus/internal/repository"
	"github.com/opus-casino/bonus/internal/testutil"
)

// newStubRepo is kept as a local alias so existing tests read unchanged.
func newStubRepo() *testutil.FakeBonusRepository {
	return testutil.NewFakeBonusRepository()
}

func testService(repo repository.BonusRepository) *BonusService {
	return NewBonusServiceWithRepository(repo, BonusConfig{
		WelcomePct:        100,
		WelcomeMaxUSD:     decimal.NewFromInt(200),
		WelcomeWagering:   30,
		WelcomeExpiryDays: 30,
	}, zap.NewNop())
}

func TestAwardWelcomeBonus_Success(t *testing.T) {
	svc := testService(newStubRepo())
	bonus, err := svc.AwardWelcomeBonus(context.Background(), 1, decimal.NewFromInt(100))
	require.NoError(t, err)
	assert.Equal(t, "100", bonus.BonusAmount.String())
	assert.Equal(t, "3000", bonus.WageringRequired.String()) // 100 * 30x
	assert.Equal(t, domain.BonusStatusActive, bonus.Status)
}

func TestAwardWelcomeBonus_CappedAtMax(t *testing.T) {
	svc := testService(newStubRepo())
	bonus, err := svc.AwardWelcomeBonus(context.Background(), 1, decimal.NewFromInt(1000))
	require.NoError(t, err)
	assert.Equal(t, "200", bonus.BonusAmount.String()) // capped at max
}

func TestAwardWelcomeBonus_Idempotent(t *testing.T) {
	repo := newStubRepo()
	svc := testService(repo)
	first, err := svc.AwardWelcomeBonus(context.Background(), 7, decimal.NewFromInt(50))
	require.NoError(t, err)
	second, err := svc.AwardWelcomeBonus(context.Background(), 7, decimal.NewFromInt(50))
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID)
	assert.Len(t, repo.Bonuses, 1)
}

func TestAwardWelcomeBonus_InvalidAmount(t *testing.T) {
	svc := testService(newStubRepo())
	_, err := svc.AwardWelcomeBonus(context.Background(), 1, decimal.Zero)
	assert.ErrorIs(t, err, domain.ErrInvalidBonusAmount)
	_, err = svc.AwardWelcomeBonus(context.Background(), 0, decimal.NewFromInt(10))
	assert.Error(t, err)
}

func TestRecordWager_CompletesBonus(t *testing.T) {
	repo := newStubRepo()
	svc := testService(repo)
	bonus, err := svc.AwardWelcomeBonus(context.Background(), 3, decimal.NewFromInt(10))
	require.NoError(t, err)
	require.Equal(t, "300", bonus.WageringRequired.String())

	require.NoError(t, svc.RecordWager(context.Background(), 3, decimal.NewFromInt(300)))
	updated, err := svc.GetBonus(context.Background(), 3, bonus.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.BonusStatusCompleted, updated.Status)
	assert.True(t, updated.IsWageringComplete())
}

// fakeCrediter records conversion credits without calling wallet-core.
type fakeCrediter struct {
	calls []BonusCreditInput
	err   error
}

func (f *fakeCrediter) CreditBonusConversion(_ context.Context, in BonusCreditInput) error {
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, in)
	return nil
}

func TestRecordWager_CreditsWalletOnCompletion(t *testing.T) {
	repo := newStubRepo()
	svc := testService(repo)
	crediter := &fakeCrediter{}
	svc.SetWalletCrediter(crediter)

	bonus, err := svc.AwardWelcomeBonus(context.Background(), 21, decimal.NewFromInt(10))
	require.NoError(t, err)
	require.NoError(t, svc.RecordWager(context.Background(), 21, decimal.NewFromInt(300)))

	require.Len(t, crediter.calls, 1)
	call := crediter.calls[0]
	assert.Equal(t, int64(21), call.UserID)
	assert.Equal(t, bonus.BonusAmount.String(), call.Amount.String())
	assert.Equal(t, bonus.ID.String(), call.ReferenceID)
	assert.Equal(t, "bonus-wagering-"+bonus.ID.String(), call.IdempotencyKey)

	updated, err := svc.GetBonus(context.Background(), 21, bonus.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.BonusStatusCompleted, updated.Status)
}

func TestRecordWager_CreditFailure_KeepsBonusActive(t *testing.T) {
	repo := newStubRepo()
	svc := testService(repo)
	svc.SetWalletCrediter(&fakeCrediter{err: assert.AnError})

	bonus, err := svc.AwardWelcomeBonus(context.Background(), 22, decimal.NewFromInt(10))
	require.NoError(t, err)
	err = svc.RecordWager(context.Background(), 22, decimal.NewFromInt(300))
	require.Error(t, err, "credit failure must surface for consumer redelivery")

	stillActive, err := svc.GetActiveBonus(context.Background(), 22)
	require.NoError(t, err)
	require.NotNil(t, stillActive, "bonus must stay active so retry can credit again")
	assert.Equal(t, bonus.ID, stillActive.ID)
}

func TestRecordWager_PartialWager_NoCredit(t *testing.T) {
	repo := newStubRepo()
	svc := testService(repo)
	crediter := &fakeCrediter{}
	svc.SetWalletCrediter(crediter)

	_, err := svc.AwardWelcomeBonus(context.Background(), 23, decimal.NewFromInt(10))
	require.NoError(t, err)
	require.NoError(t, svc.RecordWager(context.Background(), 23, decimal.NewFromInt(100)))
	assert.Empty(t, crediter.calls, "partial wagering must not trigger conversion credit")
}

func TestRecordWager_NoActiveBonus_NoError(t *testing.T) {
	svc := testService(newStubRepo())
	assert.NoError(t, svc.RecordWager(context.Background(), 999, decimal.NewFromInt(10)))
}

func TestActivateBonus_PendingToActive(t *testing.T) {
	repo := newStubRepo()
	svc := testService(repo)
	pending := &domain.Bonus{ID: uuid.New(), UserID: 5, Type: domain.BonusTypeReload, Status: domain.BonusStatusPending}
	repo.Bonuses[pending.ID] = pending
	activated, err := svc.ActivateBonus(context.Background(), 5, pending.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.BonusStatusActive, activated.Status)
}

func TestActivateBonus_NotFound(t *testing.T) {
	svc := testService(newStubRepo())
	_, err := svc.ActivateBonus(context.Background(), 5, uuid.New())
	assert.ErrorIs(t, err, domain.ErrBonusNotFound)
}

func TestCancelBonus(t *testing.T) {
	repo := newStubRepo()
	svc := testService(repo)
	bonus, err := svc.AwardWelcomeBonus(context.Background(), 9, decimal.NewFromInt(20))
	require.NoError(t, err)
	cancelled, err := svc.CancelBonus(context.Background(), 9, bonus.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.BonusStatusCancelled, cancelled.Status)
}

func TestGetBonus_WrongOwner_NotFound(t *testing.T) {
	repo := newStubRepo()
	svc := testService(repo)
	bonus, err := svc.AwardWelcomeBonus(context.Background(), 11, decimal.NewFromInt(20))
	require.NoError(t, err)
	_, err = svc.GetBonus(context.Background(), 12, bonus.ID)
	assert.ErrorIs(t, err, domain.ErrBonusNotFound)
}

func TestBonusDomain_WageringHelpers(t *testing.T) {
	b := &domain.Bonus{
		WageringRequired:  decimal.NewFromInt(100),
		WageringCompleted: decimal.NewFromInt(30),
	}
	assert.False(t, b.IsWageringComplete())
	assert.Equal(t, "70", b.RemainingWagering().String())
	b.WageringCompleted = decimal.NewFromInt(150)
	assert.True(t, b.IsWageringComplete())
	assert.Equal(t, "0", b.RemainingWagering().String()) // never negative
}
