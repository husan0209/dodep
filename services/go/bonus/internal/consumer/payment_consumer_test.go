package consumer

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"

	"github.com/opus-casino/bonus/internal/service"
	"github.com/opus-casino/bonus/internal/testutil"
)

// eventRecorder captures PaymentEvent outcomes.
type eventRecorder struct {
	results []string
}

func (r *eventRecorder) BonusAwarded(string, string, string)      {}
func (r *eventRecorder) BonusAwardSkipped(string)                 {}
func (r *eventRecorder) WagerRecorded(string)                     {}
func (r *eventRecorder) WageringCompleted(string, string, string) {}
func (r *eventRecorder) ConversionCredit(string)                  {}
func (r *eventRecorder) PaymentEvent(result string)               { r.results = append(r.results, result) }

func testConsumer() (*PaymentConsumer, *testutil.FakeBonusRepository) {
	return testConsumerWithMetrics(service.NopMetrics())
}

func testConsumerWithMetrics(m service.MetricsRecorder) (*PaymentConsumer, *testutil.FakeBonusRepository) {
	repo := testutil.NewFakeBonusRepository()
	svc := service.NewBonusServiceWithRepository(repo, service.BonusConfig{
		WelcomePct:        100,
		WelcomeMaxUSD:     decimal.NewFromInt(200),
		WelcomeWagering:   30,
		WelcomeExpiryDays: 30,
	}, zap.NewNop())
	return &PaymentConsumer{bonusSvc: svc, metrics: m, log: zap.NewNop()}, repo
}

func recordOf(t *testing.T, v interface{}) *kgo.Record {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return &kgo.Record{Value: raw}
}

func TestProcessRecord_MalformedJSON_NoRetry(t *testing.T) {
	c, repo := testConsumer()
	err := c.processRecord(context.Background(), &kgo.Record{Value: []byte("{broken")})
	assert.NoError(t, err, "malformed messages must not trigger redelivery")
	assert.Empty(t, repo.Bonuses)
}

func TestProcessRecord_NonFirstDeposit_Ignored(t *testing.T) {
	c, repo := testConsumer()
	err := c.processRecord(context.Background(), recordOf(t, DepositCompletedEvent{
		UserID: 501, FiatAmount: decimal.NewFromInt(100), IsFirst: false,
	}))
	assert.NoError(t, err)
	assert.Empty(t, repo.Bonuses, "only first deposits award welcome bonus")
}

func TestProcessRecord_FirstDeposit_AwardsBonus(t *testing.T) {
	c, repo := testConsumer()
	err := c.processRecord(context.Background(), recordOf(t, DepositCompletedEvent{
		UserID: 502, FiatAmount: decimal.NewFromInt(80), IsFirst: true,
	}))
	require.NoError(t, err)
	require.Len(t, repo.Bonuses, 1)
	for _, b := range repo.Bonuses {
		assert.Equal(t, int64(502), b.UserID)
		assert.Equal(t, "80", b.BonusAmount.String())
	}
}

func TestProcessRecord_InvalidAmount_ReturnsError(t *testing.T) {
	c, repo := testConsumer()
	err := c.processRecord(context.Background(), recordOf(t, DepositCompletedEvent{
		UserID: 503, FiatAmount: decimal.Zero, IsFirst: true,
	}))
	require.Error(t, err, "service rejection must surface for redelivery")
	assert.Empty(t, repo.Bonuses)
}

func TestProcessRecord_MetricsOutcomes(t *testing.T) {
	rec := &eventRecorder{}
	c, _ := testConsumerWithMetrics(rec)
	ctx := context.Background()

	// malformed
	require.NoError(t, c.processRecord(ctx, &kgo.Record{Value: []byte("{broken")}))
	// not first deposit
	require.NoError(t, c.processRecord(ctx, recordOf(t, DepositCompletedEvent{
		UserID: 601, FiatAmount: decimal.NewFromInt(10), IsFirst: false,
	})))
	// awarded
	require.NoError(t, c.processRecord(ctx, recordOf(t, DepositCompletedEvent{
		UserID: 602, FiatAmount: decimal.NewFromInt(20), IsFirst: true,
	})))
	// failed (zero amount is rejected by the service)
	require.Error(t, c.processRecord(ctx, recordOf(t, DepositCompletedEvent{
		UserID: 603, FiatAmount: decimal.Zero, IsFirst: true,
	})))

	assert.Equal(t, []string{
		service.PaymentEventMalformed,
		service.PaymentEventNotFirstDeposit,
		service.PaymentEventAwarded,
		service.PaymentEventFailed,
	}, rec.results)
}
