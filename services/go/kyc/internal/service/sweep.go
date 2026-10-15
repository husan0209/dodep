package service

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/opus-casino/kyc/internal/domain"
)

// ─── Expiry sweep (cron) ───
//
// The sweep runs on a schedule (Sweeper.Start, wired in main) and pages
// MarkExpiredBefore in bounded batches so a backlog of expired identity
// documents cannot OOM the worker or hold a long transaction.
//
// Cache strategy: the sweep marks DOCUMENTS; per-user record downgrades and
// status-cache invalidation happen lazily at read time (applyExpiryLazy,
// within StatusTTL=60s). SweepUser forces that path for a single user —
// used by the scheduler when paging over affected users, or by support
// tooling for one account.

const (
	// DefaultSweepBatchSize bounds one MarkExpiredBefore batch.
	DefaultSweepBatchSize = 200
	// DefaultSweepMaxBatches bounds one RunOnce tick (200*50 = 10k docs).
	DefaultSweepMaxBatches = 50
)

// Sweeper runs the document-expiry cron around a *KYCService.
type Sweeper struct {
	svc        *KYCService
	log        *zap.Logger
	batchSize  int
	maxBatches int
}

// SweeperOption tunes a Sweeper.
type SweeperOption func(*Sweeper)

// WithSweepBatchSize sets the MarkExpiredBefore batch (1..1000).
func WithSweepBatchSize(n int) SweeperOption {
	return func(s *Sweeper) {
		if n > 0 && n <= 1000 {
			s.batchSize = n
		}
	}
}

// WithSweepMaxBatches caps batches per RunOnce tick (>0).
func WithSweepMaxBatches(n int) SweeperOption {
	return func(s *Sweeper) {
		if n > 0 {
			s.maxBatches = n
		}
	}
}

// NewSweeper creates the cron runner. Nil service is rejected fast.
func NewSweeper(svc *KYCService, log *zap.Logger, opts ...SweeperOption) *Sweeper {
	if svc == nil {
		panic("kyc: sweeper requires a service")
	}
	if log == nil {
		log = zap.NewNop()
	}
	s := &Sweeper{svc: svc, log: log, batchSize: DefaultSweepBatchSize, maxBatches: DefaultSweepMaxBatches}
	for _, o := range opts {
		if o != nil {
			o(s)
		}
	}
	return s
}

// RunOnce marks expired documents in bounded batches until a short batch
// (or the batch cap) and appends a single summary audit entry.
// Record downgrades + cache invalidation follow lazily at read time.
func (s *Sweeper) RunOnce(ctx context.Context) (int64, error) {
	now := s.svc.now()
	var total int64
	batches := 0
	for batches < s.maxBatches {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		n, err := s.svc.repo.MarkExpiredBefore(ctx, now, s.batchSize)
		if err != nil {
			return total, fmt.Errorf("kyc: sweep batch %d: %w", batches, err)
		}
		total += n
		batches++
		if n < int64(s.batchSize) {
			break // short batch: nothing left
		}
	}
	_ = s.svc.repo.AppendAudit(ctx, &domain.AuditEntry{
		UserID: 0, Actor: "system",
		Action: domain.AuditExpireDocument, NewStatus: string(domain.DocExpired),
		Reason: fmt.Sprintf("sweep marked %d documents expired in %d batches", total, batches),
	})
	s.log.Info("kyc: expiry sweep tick done",
		zap.Int64("marked", total), zap.Int("batches", batches))
	return total, nil
}

// SweepUser forces the lazy expiry path for one user: expired load-bearing
// documents downgrade the record NOW and the status cache is invalidated.
// Returns true when an expiry was applied.
func (s *Sweeper) SweepUser(ctx context.Context, userID int64) (bool, error) {
	if userID <= 0 {
		return false, domain.ErrInvalidUserID
	}
	return s.svc.applyExpiryLazy(ctx, userID)
}

// Start runs the sweep every interval until ctx is cancelled.
// A failed tick is logged, never fatal: the next tick retries.
func (s *Sweeper) Start(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		interval = time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if n, err := s.RunOnce(ctx); err != nil {
				s.log.Error("kyc: expiry sweep tick failed", zap.Error(err))
			} else {
				s.log.Debug("kyc: expiry sweep tick ok", zap.Int64("marked", n))
			}
		}
	}
}
