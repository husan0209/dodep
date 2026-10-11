package consumer

import (
	"context"
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	"github.com/opus-casino/bonus/internal/service"
	"github.com/opus-casino/bonus/internal/telemetry"
)

// DepositCompletedEvent mirrors the payment service's event payload.
type DepositCompletedEvent struct {
	UserID     int64           `json:"user_id"`
	PaymentID  string          `json:"payment_id"`
	FiatAmount decimal.Decimal `json:"fiat_amount"`
	Currency   string          `json:"currency"`
	IsFirst    bool            `json:"is_first_deposit"`
	CreatedAt  time.Time       `json:"created_at"`
}

// PaymentConsumer listens to payment events and triggers bonus logic.
type PaymentConsumer struct {
	client   *kgo.Client
	bonusSvc *service.BonusService
	metrics  service.MetricsRecorder
	log      *zap.Logger
}

// nilSafeMetrics returns m, or a no-op recorder when m is nil, so the
// consumer can report events unconditionally.
func nilSafeMetrics(m service.MetricsRecorder) service.MetricsRecorder {
	if m == nil {
		return service.NopMetrics()
	}
	return m
}

// NewPaymentConsumer creates a Redpanda consumer for payment events.
// metrics may be nil (no-op); pass the Prometheus recorder in production so
// consumed-event outcomes are observable.
func NewPaymentConsumer(brokers []string, bonusSvc *service.BonusService, metrics service.MetricsRecorder, log *zap.Logger) (*PaymentConsumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup("bonus-service"),
		kgo.ConsumeTopics("payments.completed"),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()),
	)
	if err != nil {
		return nil, err
	}

	return &PaymentConsumer{
		client:   client,
		bonusSvc: bonusSvc,
		metrics:  nilSafeMetrics(metrics),
		log:      log,
	}, nil
}

// Start begins consuming payment events. Blocks until ctx is cancelled.
func (c *PaymentConsumer) Start(ctx context.Context) error {
	c.log.Info("Bonus: starting payment consumer")
	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			c.client.Close()
			return ctx.Err()
		}

		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				c.log.Error("Bonus: kafka fetch error", zap.Error(e.Err))
			}
		}

		fetches.EachRecord(func(r *kgo.Record) {
			if err := c.processRecord(ctx, r); err != nil {
				c.log.Error("Bonus: failed to process payment event",
					zap.Error(err),
					zap.String("topic", r.Topic),
					zap.Int64("offset", r.Offset))
			}
		})
	}
}

// processRecord handles one payments.completed event.
//
// A CONSUMER span is created per message so the whole award (DB write +
// wallet credit) is visible in Jaeger, correlated by Kafka topic/partition/
// offset. The trace root is the consumer loop, because Kafka carries no
// W3C trace context for the payments service.
func (c *PaymentConsumer) processRecord(ctx context.Context, r *kgo.Record) error {
	ctx, span := telemetry.StartConsumerSpan(ctx, "payments.completed process", r.Topic, r.Partition, r.Offset)
	defer span.End()

	var event DepositCompletedEvent
	if err := json.Unmarshal(r.Value, &event); err != nil {
		c.metrics.PaymentEvent(service.PaymentEventMalformed)
		c.log.Warn("Bonus: cannot parse payment event",
			zap.Error(err),
			zap.String("topic", r.Topic),
			zap.Int32("partition", r.Partition),
			zap.Int64("offset", r.Offset))
		telemetry.EndSpan(span, err)
		return nil // Don't retry malformed messages
	}

	if !event.IsFirst {
		c.metrics.PaymentEvent(service.PaymentEventNotFirstDeposit)
		span.SetAttributes(attribute.Bool("event.is_first_deposit", false))
		telemetry.EndSpan(span, nil)
		return nil // Only award welcome bonus on first deposit
	}

	c.log.Info("Bonus: first deposit event received",
		zap.Int64("user_id", event.UserID),
		zap.String("amount", event.FiatAmount.StringFixed(2)),
		zap.String("topic", r.Topic),
		zap.Int64("offset", r.Offset))

	bonus, err := c.bonusSvc.AwardWelcomeBonus(ctx, event.UserID, event.FiatAmount)
	if err != nil {
		c.metrics.PaymentEvent(service.PaymentEventFailed)
		c.log.Error("Bonus: failed to award welcome bonus",
			zap.Error(err),
			zap.Int64("user_id", event.UserID))
		telemetry.EndSpan(span, err)
		return err
	}

	span.SetAttributes(
		attribute.Bool("event.is_first_deposit", true),
		attribute.String("event.amount", event.FiatAmount.String()),
		attribute.Int64("event.user_id", event.UserID),
	)
	if bonus != nil {
		span.SetAttributes(
			attribute.String("bonus.id", bonus.ID.String()),
			attribute.String("bonus.amount", bonus.BonusAmount.String()),
		)
	}
	telemetry.EndSpan(span, nil)

	c.metrics.PaymentEvent(service.PaymentEventAwarded)
	return nil
}
