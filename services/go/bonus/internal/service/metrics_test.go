package service

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opus-casino/bonus/internal/domain"
	"github.com/opus-casino/bonus/internal/repository"
)

// recordingMetrics captures every recorder call for assertions.
// It also guards against unbounded label values: any label carrying a UUID or
// a long free-form string would be a cardinality bug.
type recordingMetrics struct {
	awarded        []string // "type|currency|amount"
	awardSkipped   []string
	wagerResults   []string
	wageringDone   []string // "type|currency|ratio"
	conversionCres []string
	paymentEvents  []string
}

func (r *recordingMetrics) BonusAwarded(bonusType, currency, amount string) {
	r.awarded = append(r.awarded, bonusType+"|"+currency+"|"+amount)
}

func (r *recordingMetrics) BonusAwardSkipped(reason string) {
	r.awardSkipped = append(r.awardSkipped, reason)
}

func (r *recordingMetrics) WagerRecorded(result string) {
	r.wagerResults = append(r.wagerResults, result)
}

func (r *recordingMetrics) WageringCompleted(bonusType, currency, ratio string) {
	r.wageringDone = append(r.wageringDone, bonusType+"|"+currency+"|"+ratio)
}

func (r *recordingMetrics) ConversionCredit(result string) {
	r.conversionCres = append(r.conversionCres, result)
}

func (r *recordingMetrics) PaymentEvent(result string) {
	r.paymentEvents = append(r.paymentEvents, result)
}

func testServiceWithMetrics(repo repository.BonusRepository, m *recordingMetrics) *BonusService {
	svc := testService(repo)
	svc.SetMetrics(m)
	return svc
}

func TestMetrics_AwardedEmitsTypeCurrencyAmount(t *testing.T) {
	rec := &recordingMetrics{}
	svc := testServiceWithMetrics(newStubRepo(), rec)

	_, err := svc.AwardWelcomeBonus(context.Background(), 1, decimal.NewFromInt(100))
	require.NoError(t, err)
	require.Equal(t, []string{"welcome|USD|100"}, rec.awarded)
}

func TestMetrics_AwardSkippedOnIdempotentHit(t *testing.T) {
	rec := &recordingMetrics{}
	svc := testServiceWithMetrics(newStubRepo(), rec)
	ctx := context.Background()

	_, err := svc.AwardWelcomeBonus(ctx, 2, decimal.NewFromInt(50))
	require.NoError(t, err)
	_, err = svc.AwardWelcomeBonus(ctx, 2, decimal.NewFromInt(50))
	require.NoError(t, err)

	assert.Equal(t, []string{"welcome|USD|50"}, rec.awarded, "only the first award is counted")
	assert.Equal(t, []string{SkipAlreadyAwarded}, rec.awardSkipped)
}

func TestMetrics_AwardSkippedOnInvalidAmount(t *testing.T) {
	rec := &recordingMetrics{}
	svc := testServiceWithMetrics(newStubRepo(), rec)

	_, err := svc.AwardWelcomeBonus(context.Background(), 3, decimal.Zero)
	assert.ErrorIs(t, err, domain.ErrInvalidBonusAmount)
	assert.Equal(t, []string{SkipInvalidAmount}, rec.awardSkipped)
	assert.Empty(t, rec.awarded)
}

func TestMetrics_WagerPartialNoCompletionSignal(t *testing.T) {
	rec := &recordingMetrics{}
	svc := testServiceWithMetrics(newStubRepo(), rec)
	ctx := context.Background()

	_, err := svc.AwardWelcomeBonus(ctx, 4, decimal.NewFromInt(10))
	require.NoError(t, err)
	require.NoError(t, svc.RecordWager(ctx, 4, decimal.NewFromInt(100)))

	assert.Equal(t, []string{WagerRecordedResult}, rec.wagerResults)
	assert.Empty(t, rec.wageringDone, "partial wagering must not report completion")
	assert.Empty(t, rec.conversionCres)
}

func TestMetrics_WagerCompletionEmitsRatioAndCredit(t *testing.T) {
	rec := &recordingMetrics{}
	repo := newStubRepo()
	svc := testServiceWithMetrics(repo, rec)
	crediter := &fakeCrediter{}
	svc.SetWalletCrediter(crediter)
	ctx := context.Background()

	_, err := svc.AwardWelcomeBonus(ctx, 5, decimal.NewFromInt(10))
	require.NoError(t, err)
	require.NoError(t, svc.RecordWager(ctx, 5, decimal.NewFromInt(300)))

	assert.Equal(t, []string{"welcome|USD|1"}, rec.wageringDone, "ratio 300/300 = 1")
	assert.Equal(t, []string{CreditSucceeded}, rec.conversionCres)
	assert.Equal(t, []string{WagerCompletedConversion}, rec.wagerResults)
}

func TestMetrics_WagerCompletionRatioAboveOneIsPreserved(t *testing.T) {
	rec := &recordingMetrics{}
	repo := newStubRepo()
	svc := testServiceWithMetrics(repo, rec)
	svc.SetWalletCrediter(&fakeCrediter{})
	ctx := context.Background()

	// 10 deposit → 10 bonus → 300 required; wager 450 → ratio 1.5
	_, err := svc.AwardWelcomeBonus(ctx, 6, decimal.NewFromInt(10))
	require.NoError(t, err)
	require.NoError(t, svc.RecordWager(ctx, 6, decimal.NewFromInt(450)))

	assert.Equal(t, []string{"welcome|USD|1.5"}, rec.wageringDone)
}

func TestMetrics_CreditFailureKeepsBonusActive(t *testing.T) {
	rec := &recordingMetrics{}
	repo := newStubRepo()
	svc := testServiceWithMetrics(repo, rec)
	svc.SetWalletCrediter(&fakeCrediter{err: assert.AnError})
	ctx := context.Background()

	_, err := svc.AwardWelcomeBonus(ctx, 7, decimal.NewFromInt(10))
	require.NoError(t, err)
	err = svc.RecordWager(ctx, 7, decimal.NewFromInt(300))
	require.Error(t, err)

	assert.Equal(t, []string{CreditFailed}, rec.conversionCres)
	assert.Equal(t, []string{WagerCreditFailed}, rec.wagerResults)
	assert.Empty(t, rec.wageringDone, "completion is only reported after a successful credit")

	active, err := svc.GetActiveBonus(ctx, 7)
	require.NoError(t, err)
	assert.NotNil(t, active, "bonus stays active so the redelivery can retry the credit")
}

func TestMetrics_NoActiveBonusWager(t *testing.T) {
	rec := &recordingMetrics{}
	svc := testServiceWithMetrics(newStubRepo(), rec)

	require.NoError(t, svc.RecordWager(context.Background(), 999, decimal.NewFromInt(10)))
	assert.Equal(t, []string{WagerNoActiveBonus}, rec.wagerResults)
}

func TestMetrics_InvalidWagerAmount(t *testing.T) {
	rec := &recordingMetrics{}
	svc := testServiceWithMetrics(newStubRepo(), rec)

	require.NoError(t, svc.RecordWager(context.Background(), 1, decimal.Zero))
	require.NoError(t, svc.RecordWager(context.Background(), 1, decimal.NewFromInt(-5)))
	assert.Equal(t, []string{WagerInvalidAmount, WagerInvalidAmount}, rec.wagerResults)
}

func TestMetrics_ConversionSkippedWithoutWallet(t *testing.T) {
	rec := &recordingMetrics{}
	repo := newStubRepo()
	svc := testServiceWithMetrics(repo, rec) // no wallet attached
	ctx := context.Background()

	_, err := svc.AwardWelcomeBonus(ctx, 8, decimal.NewFromInt(10))
	require.NoError(t, err)
	require.NoError(t, svc.RecordWager(ctx, 8, decimal.NewFromInt(300)))

	assert.Equal(t, []string{CreditSkippedNoWalle}, rec.conversionCres)
	assert.Equal(t, []string{WagerCompletedConversion}, rec.wagerResults)
}

func TestSetMetrics_NilFallsBackToNoop(t *testing.T) {
	svc := testService(newStubRepo())
	assert.NotPanics(t, func() {
		svc.SetMetrics(nil)
		_, _ = svc.AwardWelcomeBonus(context.Background(), 1, decimal.NewFromInt(10))
		require.NoError(t, svc.RecordWager(context.Background(), 1, decimal.NewFromInt(10)))
	})
}

func TestCompletionRatio(t *testing.T) {
	assert.Equal(t, "1", completionRatio(decimal.NewFromInt(300), decimal.NewFromInt(300)))
	assert.Equal(t, "0.5", completionRatio(decimal.NewFromInt(300), decimal.NewFromInt(150)))
	assert.Equal(t, "1.3333", completionRatio(decimal.NewFromInt(3), decimal.NewFromInt(4)))
	assert.Equal(t, "", completionRatio(decimal.Zero, decimal.NewFromInt(10)), "undefined ratio stays empty")
}
