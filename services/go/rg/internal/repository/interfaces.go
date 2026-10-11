package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/opus-casino/rg/internal/domain"
)

// LimitsRepository persists effective limits and pending increases.
type LimitsRepository interface {
	GetLimits(ctx context.Context, userID int64) (*domain.RGLimits, error)
	UpsertLimits(ctx context.Context, limits *domain.RGLimits) error
	CreatePendingChange(ctx context.Context, ch *domain.PendingChange) error
	ListPendingChanges(ctx context.Context, userID int64) ([]*domain.PendingChange, error)
	ListDueChanges(ctx context.Context, now time.Time, limit int) ([]*domain.PendingChange, error)
	UpdateChangeStatus(ctx context.Context, id uuid.UUID, from domain.ChangeStatus, to domain.ChangeStatus) error
	DeletePendingChanges(ctx context.Context, userID int64, limitType domain.LimitType) error
}

// ExclusionRepository persists self-/operator-/regulatory exclusions.
type ExclusionRepository interface {
	CreateExclusion(ctx context.Context, e *domain.Exclusion) error
	GetActiveExclusion(ctx context.Context, userID int64) (*domain.Exclusion, error)
	ListExclusions(ctx context.Context, userID int64, limit int) ([]*domain.Exclusion, error)
	UpdateExclusionStatus(ctx context.Context, id uuid.UUID, from domain.ExclusionStatus, to domain.ExclusionStatus, patch map[string]interface{}) error
}

// TimeoutRepository persists cooling-off time-outs.
type TimeoutRepository interface {
	CreateTimeout(ctx context.Context, t *domain.Timeout) error
	GetActiveTimeout(ctx context.Context, userID int64, now time.Time) (*domain.Timeout, error)
}

// SpendRepository persists daily money-movement aggregates used for
// rolling-window limit accounting. Aggregates are keyed by (user, day,
// currency); callers MUST report amounts in the user's account currency
// (payment/wallet services convert before reporting).
type SpendRepository interface {
	RecordSpend(ctx context.Context, userID int64, day time.Time, currency, deposits, wagers, payouts string) error
	SumSpendSince(ctx context.Context, userID int64, currency string, since time.Time) (deposits, wagers, payouts string, err error)
}

// OutboxRepository stages domain events for publishing.
type OutboxRepository interface {
	AppendOutbox(ctx context.Context, topic, key string, payload map[string]interface{}) error
	ListPendingOutbox(ctx context.Context, limit int) ([]OutboxEvent, error)
	MarkOutboxPublished(ctx context.Context, id uuid.UUID) error
}

// OutboxEvent is a staged domain event.
type OutboxEvent struct {
	ID        uuid.UUID
	Topic     string
	EventKey  string
	Payload   map[string]interface{}
	CreatedAt time.Time
}

// Repository aggregates all RG persistence interfaces.
type Repository interface {
	LimitsRepository
	ExclusionRepository
	TimeoutRepository
	SpendRepository
	OutboxRepository

	// Transact runs fn inside a single database transaction, so state
	// changes and outbox events commit atomically (transactional outbox).
	Transact(ctx context.Context, fn func(Repository) error) error
}
