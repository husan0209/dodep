package event

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"
)

// RedpandaPublisherConfig holds Redpanda/Kafka producer settings.
// Zero values are replaced with reference defaults by NewRedpandaPublisher.
type RedpandaPublisherConfig struct {
	// Brokers is the seed broker list (host:port). Required.
	Brokers []string
	// ClientID identifies this producer in broker logs and metrics.
	ClientID string
	// DeliveryTimeout bounds how long a record may be retried client-side
	// before Publish reports an error. The outbox worker accounts the
	// failure and retries on the next poll, so this must be finite.
	DeliveryTimeout time.Duration
}

func (c RedpandaPublisherConfig) withDefaults() RedpandaPublisherConfig {
	if c.ClientID == "" {
		c.ClientID = "affiliate-service-outbox"
	}
	if c.DeliveryTimeout <= 0 {
		c.DeliveryTimeout = 30 * time.Second
	}
	return c
}

// RedpandaPublisher publishes outbox events to Redpanda/Kafka with
// exactly the durability the financial pipeline requires:
//
//   - RequiredAcks(AllISRAcks): no broker-side loss (platform standard);
//   - idempotent production is franz-go's default and stays enabled;
//   - RecordDeliveryTimeout surfaces stuck records as errors so the outbox
//     worker can account retries and keep events unpublished;
//   - Record.Key carries the aggregate (affiliate) ID, so all events of one
//     affiliate land in a single partition in order.
type RedpandaPublisher struct {
	client *kgo.Client
	logger *zap.Logger
}

// NewRedpandaPublisher dials the brokers and fails fast when they are
// unreachable, so misconfiguration surfaces at boot, not on first event.
func NewRedpandaPublisher(
	ctx context.Context,
	cfg RedpandaPublisherConfig,
	logger *zap.Logger,
) (*RedpandaPublisher, error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("redpanda publisher: no brokers configured")
	}
	cfg = cfg.withDefaults()

	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ClientID(cfg.ClientID),
		kgo.SoftwareNameAndVersion("affiliate-service", "v1.0.0"),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.RecordDeliveryTimeout(cfg.DeliveryTimeout),
		kgo.ProducerLinger(10),
		kgo.ProducerBatchMaxBytes(1024*1024),
	)
	if err != nil {
		return nil, fmt.Errorf("redpanda publisher: create client: %w", err)
	}

	if err := client.Ping(ctx); err != nil {
		client.Close()
		return nil, fmt.Errorf("redpanda publisher: ping brokers %v: %w", cfg.Brokers, err)
	}

	logger.Info("redpanda publisher connected",
		zap.Strings("brokers", cfg.Brokers),
		zap.String("client_id", cfg.ClientID),
	)
	return &RedpandaPublisher{client: client, logger: logger}, nil
}

// Close flushes in-flight records and releases the client.
func (p *RedpandaPublisher) Close() {
	p.client.Close()
}

// Publish marshals the outbox payload to JSON and synchronously produces
// one record. A nil payload is rejected: silent empty events would break
// consumer contracts downstream.
func (p *RedpandaPublisher) Publish(
	ctx context.Context,
	topic string,
	key string,
	payload map[string]any,
	headers map[string]any,
) error {
	if topic == "" {
		return fmt.Errorf("redpanda publisher: empty topic")
	}
	if payload == nil {
		return fmt.Errorf("redpanda publisher: nil payload for topic %s", topic)
	}
	record, err := buildRecord(topic, key, payload, headers)
	if err != nil {
		return err
	}
	if err := p.client.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("redpanda publisher: produce to %s: %w", topic, err)
	}
	p.logger.Debug("outbox event published",
		zap.String("topic", topic),
		zap.String("key", key),
	)
	return nil
}

// buildRecord is pure (no broker needed) so record layout is unit-testable.
func buildRecord(
	topic string,
	key string,
	payload map[string]any,
	headers map[string]any,
) (*kgo.Record, error) {
	value, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("redpanda publisher: marshal payload: %w", err)
	}
	record := &kgo.Record{
		Topic:     topic,
		Key:       []byte(key),
		Value:     value,
		Timestamp: time.Now().UTC(),
	}
	for name, v := range headers {
		record.Headers = append(record.Headers, kgo.RecordHeader{
			Key:   name,
			Value: []byte(fmt.Sprint(v)),
		})
	}
	return record, nil
}
