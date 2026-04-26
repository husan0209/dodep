package event

import (
	"context"

	"go.uber.org/zap"
)

// Publisher is a transport abstraction for outbox delivery.
type Publisher interface {
	Publish(ctx context.Context, topic string, key string, payload map[string]any, headers map[string]any) error
}

// LogPublisher is a safe default publisher for environments where broker wiring
// is not configured yet. It preserves delivery flow and observability.
type LogPublisher struct {
	logger *zap.Logger
}

func NewLogPublisher(logger *zap.Logger) *LogPublisher {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &LogPublisher{logger: logger}
}

func (p *LogPublisher) Publish(ctx context.Context, topic string, key string, payload map[string]any, headers map[string]any) error {
	p.logger.Info(
		"outbox event published",
		zap.String("topic", topic),
		zap.String("key", key),
		zap.Any("payload", payload),
		zap.Any("headers", headers),
	)
	return nil
}
