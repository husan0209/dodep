package consumer

import (
	"context"
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"

	"github.com/opus-casino/bonus/internal/service"
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
	client     *kgo.Client
	bonusSvc   *service.BonusService
	log        *zap.Logger
}

// NewPaymentConsumer creates a Redpanda consumer for payment events.
func NewPaymentConsumer(brokers []string, bonusSvc *service.BonusService, log *zap.Logger) (*PaymentConsumer, error) {
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

func (c *PaymentConsumer) processRecord(ctx context.Context, r *kgo.Record) error {
	var event DepositCompletedEvent
	if err := json.Unmarshal(r.Value, &event); err != nil {
		c.log.Warn("Bonus: cannot parse payment event", zap.Error(err))
		return nil // Don't retry malformed messages
	}

	if !event.IsFirst {
		return nil // Only award welcome bonus on first deposit
	}

	c.log.Info("Bonus: first deposit event received",
		zap.Int64("user_id", event.UserID),
		zap.String("amount", event.FiatAmount.StringFixed(2)))

	if _, err := c.bonusSvc.AwardWelcomeBonus(ctx, event.UserID, event.FiatAmount); err != nil {
		c.log.Error("Bonus: failed to award welcome bonus",
			zap.Error(err),
			zap.Int64("user_id", event.UserID))
		return err
	}

	return nil
}
