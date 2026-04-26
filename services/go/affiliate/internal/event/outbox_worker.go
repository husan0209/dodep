package event

import (
	"context"
	"time"

	"github.com/opus-casino/affiliate/internal/repository"
	"go.uber.org/zap"
)

type OutboxWorker struct {
	repo      repository.AffiliateRepository
	publisher Publisher
	logger    *zap.Logger
	interval  time.Duration
	batchSize int
}

func NewOutboxWorker(
	repo repository.AffiliateRepository,
	publisher Publisher,
	logger *zap.Logger,
	interval time.Duration,
	batchSize int,
) *OutboxWorker {
	if logger == nil {
		logger = zap.NewNop()
	}
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 100
	}

	return &OutboxWorker{
		repo:      repo,
		publisher: publisher,
		logger:    logger,
		interval:  interval,
		batchSize: batchSize,
	}
}

func (w *OutboxWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.logger.Info("outbox worker started", zap.Duration("interval", w.interval), zap.Int("batch_size", w.batchSize))
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("outbox worker stopped", zap.Error(ctx.Err()))
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *OutboxWorker) processBatch(ctx context.Context) {
	events, err := w.repo.ListPendingOutboxEvents(ctx, w.batchSize)
	if err != nil {
		w.logger.Error("failed to list pending outbox events", zap.Error(err))
		return
	}
	if len(events) == 0 {
		return
	}

	for _, evt := range events {
		if err := w.publisher.Publish(ctx, evt.Topic, evt.EventKey, evt.Payload, evt.Headers); err != nil {
			w.logger.Error(
				"failed to publish outbox event",
				zap.Int64("event_id", evt.ID),
				zap.String("topic", evt.Topic),
				zap.Error(err),
			)
			if retryErr := w.repo.IncrementOutboxEventRetry(ctx, evt.ID); retryErr != nil {
				w.logger.Error("failed to increment outbox retry", zap.Int64("event_id", evt.ID), zap.Error(retryErr))
			}
			continue
		}

		if err := w.repo.MarkOutboxEventPublished(ctx, evt.ID); err != nil {
			w.logger.Error("failed to mark outbox event published", zap.Int64("event_id", evt.ID), zap.Error(err))
			continue
		}
	}
}
