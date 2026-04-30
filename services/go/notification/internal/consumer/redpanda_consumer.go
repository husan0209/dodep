package consumer

import (
	"context"
	"encoding/json"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/opus-casino/notification/internal/service"
)

// RedpandaConsumer consumes events from Redpanda/Kafka
type RedpandaConsumer struct {
	brokers       []string
	service       *service.NotificationService
	log           *zap.Logger
	topics        []string
	groupID       string
}

// Event represents a platform event
type Event struct {
	Type      string            `json:"type"`
	UserID    uint64            `json:"user_id"`
	Data      map[string]string `json:"data"`
	Timestamp time.Time         `json:"timestamp"`
}

// NewRedpandaConsumer creates a new Redpanda consumer
func NewRedpandaConsumer(brokers []string, service *service.NotificationService, log *zap.Logger) *RedpandaConsumer {
	return &RedpandaConsumer{
		brokers: brokers,
		service: service,
		log:     log,
		topics: []string{
			"bets.settled",
			"payments.deposit_confirmed",
			"payments.withdrawal_processed",
			"users.kyc_verified",
			"bonus.activated",
			"bonus.expiring",
		},
		groupID: "notification-service",
	}
}

// Start starts consuming events from Redpanda
func (c *RedpandaConsumer) Start(ctx context.Context) error {
	for _, topic := range c.topics {
		go func(topic string) {
			c.consumeTopic(ctx, topic)
		}(topic)
	}

	<-ctx.Done()
	return nil
}

// consumeTopic consumes events from a specific topic
func (c *RedpandaConsumer) consumeTopic(ctx context.Context, topic string) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  c.brokers,
		Topic:    topic,
		GroupID:  c.groupID,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	})
	defer reader.Close()

	c.log.Info("Starting consumer for topic", zap.String("topic", topic))

	for {
		select {
		case <-ctx.Done():
			c.log.Info("Stopping consumer for topic", zap.String("topic", topic))
			return
		default:
			msg, err := reader.FetchMessage(ctx)
			if err != nil {
				c.log.Error("Failed to fetch message", zap.Error(err))
				continue
			}

			if err := c.processMessage(ctx, msg); err != nil {
				c.log.Error("Failed to process message", zap.Error(err))
				// Don't commit offset on error to retry
				continue
			}

			if err := reader.CommitMessages(ctx, msg); err != nil {
				c.log.Error("Failed to commit offset", zap.Error(err))
			}
		}
	}
}

// processMessage processes a single message
func (c *RedpandaConsumer) processMessage(ctx context.Context, msg kafka.Message) error {
	var event Event
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		c.log.Error("Failed to unmarshal event", zap.Error(err))
		return err
	}

	c.log.Debug("Processing event",
		zap.String("type", event.Type),
		zap.Uint64("user_id", event.UserID))

	// Process event based on type
	if err := c.service.ProcessEvent(ctx, event.Type, event.Data); err != nil {
		c.log.Error("Failed to process event",
			zap.String("type", event.Type),
			zap.Uint64("user_id", event.UserID),
			zap.Error(err))
		return err
	}

	return nil
}

// Stop stops the consumer
func (c *RedpandaConsumer) Stop(ctx context.Context) error {
	c.log.Info("Stopping all consumers")
	return nil
}
