package event

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"
)

// Producer handles event publishing to Redpanda
type Producer struct {
	client *kgo.Client
	topic  string
	logger *zap.Logger
}

// ProducerConfig holds configuration for the producer
type ProducerConfig struct {
	Brokers []string
	Topic   string
}

// NewProducer creates a new event producer
func NewProducer(cfg ProducerConfig, logger *zap.Logger) (*Producer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.ProducerLinger(10),
		kgo.ProducerBatchMaxBytes(1024*1024),
	)
	if err != nil {
		return nil, fmt.Errorf("create kafka client: %w", err)
	}

	return &Producer{
		client: client,
		topic:  cfg.Topic,
		logger: logger,
	}, nil
}

// Close closes the producer
func (p *Producer) Close() error {
	p.client.Close()
	return nil
}

// Publish publishes an event
func (p *Producer) Publish(ctx context.Context, key string, event interface{}) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	record := &kgo.Record{
		Topic: p.topic,
		Key:   []byte(key),
		Value: data,
	}

	result := p.client.ProduceSync(ctx, record)
	if err := result.FirstErr(); err != nil {
		return fmt.Errorf("produce event: %w", err)
	}

	p.logger.Debug("Event published",
		zap.String("topic", p.topic),
		zap.String("key", key),
	)

	return nil
}

// PublishAsync publishes an event asynchronously
func (p *Producer) PublishAsync(ctx context.Context, key string, event interface{}, callback func(error)) {
	data, err := json.Marshal(event)
	if err != nil {
		if callback != nil {
			callback(fmt.Errorf("marshal event: %w", err))
		}
		return
	}

	record := &kgo.Record{
		Topic: p.topic,
		Key:   []byte(key),
		Value: data,
	}

	p.client.Produce(ctx, record, func(r *kgo.Record, err error) {
		if callback != nil {
			callback(err)
		}
		if err != nil {
			p.logger.Error("Failed to publish event",
				zap.Error(err),
				zap.String("topic", p.topic),
				zap.String("key", key),
			)
		}
	})
}
