package consumer

import (
	"context"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"
)

// FranzConsumer wraps franz-go client for Redpanda/Kafka consumption
type FranzConsumer struct {
	client *kgo.Client
	logger *zap.Logger
}

// FranzConsumerConfig holds configuration for the consumer
type FranzConsumerConfig struct {
	Brokers []string
	GroupID string
	Topics  []string
}

// NewFranzConsumer creates a new franz-go based consumer
func NewFranzConsumer(ctx context.Context, cfg FranzConsumerConfig, logger *zap.Logger) (*FranzConsumer, error) {
	opts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ConsumerGroup(cfg.GroupID),
		kgo.ConsumeTopics(cfg.Topics...),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()),
		kgo.SessionTimeout(30 * time.Second),
		kgo.HeartbeatInterval(3 * time.Second),
		kgo.FetchMaxWait(5 * time.Second),
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, err
	}

	// Verify connection
	if err := client.Ping(ctx); err != nil {
		client.Close()
		return nil, err
	}

	logger.Info("franz consumer connected",
		zap.Strings("brokers", cfg.Brokers),
		zap.String("group_id", cfg.GroupID),
		zap.Strings("topics", cfg.Topics),
	)

	return &FranzConsumer{
		client: client,
		logger: logger,
	}, nil
}

// Consume starts consuming messages and calls the handler for each
func (c *FranzConsumer) Consume(ctx context.Context, handler func(Message) error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		fetches := c.client.PollFetches(ctx)
		if fetches.IsClientClosed() {
			return nil
		}

		if errs := fetches.Errors(); len(errs) > 0 {
			for _, err := range errs {
				c.logger.Error("fetch error",
					zap.String("topic", err.Topic),
					zap.Int32("partition", err.Partition),
					zap.Error(err.Err),
				)
			}
		}

		iter := fetches.RecordIter()
		for !iter.Done() {
			record := iter.Next()

			msg := Message{
				Topic:     record.Topic,
				Key:       record.Key,
				Value:     record.Value,
				Partition: record.Partition,
				Offset:    record.Offset,
			}

			if err := handler(msg); err != nil {
				c.logger.Error("message handler error",
					zap.String("topic", record.Topic),
					zap.Int64("offset", record.Offset),
					zap.Error(err),
				)
				// Continue processing - don't fail the whole consumer
			}
		}
	}
}

// Close closes the consumer
func (c *FranzConsumer) Close() {
	c.client.Close()
	c.logger.Info("franz consumer closed")
}
