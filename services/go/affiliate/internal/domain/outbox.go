package domain

import "time"

// OutboxEvent represents a not-yet-published domain event persisted in DB.
type OutboxEvent struct {
	ID            int64
	AggregateType string
	AggregateID   string
	Topic         string
	EventKey      string
	Payload       map[string]any
	Headers       map[string]any
	CreatedAt     time.Time
	PublishedAt   *time.Time
	RetryCount    int
}
