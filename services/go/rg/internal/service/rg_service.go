package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/opus-casino/rg/internal/domain"
	"github.com/opus-casino/rg/internal/repository"
)

// EventPublisher emits domain events. Production wires a Redpanda publisher
// draining rg_service_outbox; tests inject NoopPublisher. Per-check denials
// are NOT published as events (bet-rate volume) — they are logged; only
// state changes produce events.
type EventPublisher interface {
	Publish(ctx context.Context, topic, key string, payload map[string]interface{}) error
}

// NoopPublisher drops events (tests / local dev).
type NoopPublisher struct{}

func (NoopPublisher) Publish(_ context.Context, _, _ string, _ map[string]interface{}) error {
	return nil
}

// LogPublisher logs events via zap (local dev default).
type LogPublisher struct{ log *zap.Logger }

// NewLogPublisher builds a LogPublisher.
func NewLogPublisher(log *zap.Logger) *LogPublisher { return &LogPublisher{log: log} }

func (p *LogPublisher) Publish(_ context.Context, topic, key string, payload map[string]interface{}) error {
	p.log.Info("rg: event published",
		zap.String("topic", topic), zap.String("key", key), zap.Any("payload", payload))
	return nil
}

// Topics emitted by this service.
const (
	TopicSelfExcluded     = "users.self_excluded"
	TopicExclusionStarted = "rg.exclusion.started"
	TopicExclusionRevoked = "rg.exclusion.revoked"
	TopicExclusionExpired = "rg.exclusion.expired"
	TopicTimeoutStarted   = "rg.timeout.started"
	TopicLimitSet         = "rg.limit.set"
	TopicLimitIncreaseDue = "rg.limit.increase_applied"
	TopicPendingCancelled = "rg.limit.increase_cancelled"
)

// RGService implements responsible-gambling controls.
type RGService struct {
	repo      repository.Repository
	publisher EventPublisher
	log       *zap.Logger
	cooling   time.Duration
	revokeGap time.Duration
}

// NewRGService builds the service. Panics on nil dependencies (fail fast).
func NewRGService(repo repository.Repository, publisher EventPublisher, log *zap.Logger, coolingHours, revokeCoolingHours int) *RGService {
	if repo == nil {
		panic("rg: repository is required")
	}
	if publisher == nil {
		publisher = NoopPublisher{}
	}
	if log == nil {
		log, _ = zap.NewProduction()
	}
	if coolingHours < 24 {
		coolingHours = 24
	}
	if coolingHours > 72 {
		coolingHours = 72
	}
	if revokeCoolingHours < 24 {
		revokeCoolingHours = 24
	}
	return &RGService{
		repo: repo, publisher: publisher, log: log,
		cooling:   time.Duration(coolingHours) * time.Hour,
		revokeGap: time.Duration(revokeCoolingHours) * time.Hour,
	}
}

// ============ Enforcement ============

// CheckInput is the enforcement request. Amount is required for bet and
// deposit channels and must be in the user's account currency.
type CheckInput struct {
	UserID   int64
	Channel  domain.Channel
	Amount   decimal.Decimal
	Currency string
}

// CheckPlayAllowed is the RULE-4 gate. Order is deliberate:
// exclusion → time-out → channel limits. Withdrawals are always allowed.
func (s *RGService) CheckPlayAllowed(ctx context.Context, in CheckInput) (domain.Decision, error) {
	if !in.Channel.IsValid() {
		return domain.Decision{}, domain.ErrInvalidChannel
	}
	now := time.Now().UTC()

	if in.Channel == domain.ChannelWithdrawal {
		return domain.Decision{Allowed: true, Reason: domain.ReasonOK}, nil
	}

	excl, err := s.repo.GetActiveExclusion(ctx, in.UserID)
	if err != nil {
		return domain.Decision{}, err
	}
	if excl.IsActiveAt(now) {
		s.log.Info("rg: play denied (excluded)", zap.Int64("user_id", in.UserID))
		return domain.Decision{Allowed: false, Reason: domain.ReasonSelfExcluded, Message: "gambling is blocked for this account"}, nil
	}

	to, err := s.repo.GetActiveTimeout(ctx, in.UserID, now)
	if err != nil {
		return domain.Decision{}, err
	}
	if to.IsActiveAt(now) {
		return domain.Decision{Allowed: false, Reason: domain.ReasonCoolOff, Message: "cooling-off period is active"}, nil
	}

	switch in.Channel {
	case domain.ChannelDeposit:
		return s.checkDeposit(ctx, in.UserID, in.Amount, in.Currency, now)
	case domain.ChannelBet:
		return s.checkBet(ctx, in.UserID, in.Amount, in.Currency, now)
	case domain.ChannelGameLaunch:
		return domain.Decision{Allowed: true, Reason: domain.ReasonOK}, nil
	default:
		return domain.Decision{}, domain.ErrInvalidChannel
	}
}

func (s *RGService) checkDeposit(ctx context.Context, userID int64, amount decimal.Decimal, currency string, now time.Time) (domain.Decision, error) {
	if amount.IsNegative() {
		return domain.Decision{}, domain.NewValidationError(domain.FieldError{Field: "amount", Message: "must be >= 0"})
	}
	limits, err := s.repo.GetLimits(ctx, userID)
	if err != nil {
		return domain.Decision{}, err
	}
	windows := []struct {
		limit decimal.Decimal
		dur   time.Duration
	}{
		{limits.DepositDaily, 24 * time.Hour},
		{limits.DepositWeekly, 7 * 24 * time.Hour},
		{limits.DepositMonthly, 30 * 24 * time.Hour},
	}
	for _, w := range windows {
		if w.limit.IsZero() {
			continue
		}
		spent, err := s.sumDeposits(ctx, userID, currency, now.Add(-w.dur))
		if err != nil {
			return domain.Decision{}, err
		}
		if spent.Add(amount).GreaterThan(w.limit) {
			rem := w.limit.Sub(spent)
			if rem.IsNegative() {
				rem = decimal.Zero
			}
			return domain.Decision{
				Allowed: false, Reason: domain.ReasonDepositLimit,
				Message:   "deposit limit would be exceeded",
				Remaining: &rem,
			}, nil
		}
	}
	return domain.Decision{Allowed: true, Reason: domain.ReasonOK}, nil
}

func (s *RGService) checkBet(ctx context.Context, userID int64, stake decimal.Decimal, currency string, now time.Time) (domain.Decision, error) {
	if stake.IsNegative() {
		return domain.Decision{}, domain.NewValidationError(domain.FieldError{Field: "amount", Message: "must be >= 0"})
	}
	limits, err := s.repo.GetLimits(ctx, userID)
	if err != nil {
		return domain.Decision{}, err
	}
	// Loss limits: net loss = wagers - payouts over the window.
	for _, w := range []struct {
		limit decimal.Decimal
		dur   time.Duration
	}{
		{limits.LossDaily, 24 * time.Hour},
		{limits.LossWeekly, 7 * 24 * time.Hour},
		{limits.LossMonthly, 30 * 24 * time.Hour},
	} {
		if w.limit.IsZero() {
			continue
		}
		_, wagers, payouts, err := s.repo.SumSpendSince(ctx, userID, currency, now.Add(-w.dur))
		if err != nil {
			return domain.Decision{}, err
		}
		loss := mustDecimal(wagers).Sub(mustDecimal(payouts))
		if loss.GreaterThanOrEqual(w.limit) {
			return domain.Decision{Allowed: false, Reason: domain.ReasonLossLimit, Message: "loss limit reached"}, nil
		}
	}
	// Wager limits: stake counts toward the window.
	for _, w := range []struct {
		limit decimal.Decimal
		dur   time.Duration
	}{
		{limits.WagerDaily, 24 * time.Hour},
		{limits.WagerWeekly, 7 * 24 * time.Hour},
	} {
		if w.limit.IsZero() {
			continue
		}
		_, wagered, _, err := s.repo.SumSpendSince(ctx, userID, currency, now.Add(-w.dur))
		if err != nil {
			return domain.Decision{}, err
		}
		if mustDecimal(wagered).Add(stake).GreaterThan(w.limit) {
			return domain.Decision{Allowed: false, Reason: domain.ReasonWagerLimit, Message: "wager limit would be exceeded"}, nil
		}
	}
	return domain.Decision{Allowed: true, Reason: domain.ReasonOK}, nil
}

func (s *RGService) sumDeposits(ctx context.Context, userID int64, currency string, since time.Time) (decimal.Decimal, error) {
	dep, _, _, err := s.repo.SumSpendSince(ctx, userID, currency, since)
	if err != nil {
		return decimal.Zero, err
	}
	return mustDecimal(dep), nil
}

func mustDecimal(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero
	}
	return d
}

// ============ Limits ============

// SetLimitsInput carries the requested controls. Nil money pointers mean
// "no change"; nil minutes mean "no change". Zero money value clears a limit.
type SetLimitsInput struct {
	UserID              int64
	DepositDaily        *decimal.Decimal
	DepositWeekly       *decimal.Decimal
	DepositMonthly      *decimal.Decimal
	LossDaily           *decimal.Decimal
	LossWeekly          *decimal.Decimal
	LossMonthly         *decimal.Decimal
	WagerDaily          *decimal.Decimal
	WagerWeekly         *decimal.Decimal
	SessionMinutes      *int
	RealityCheckMinutes *int
}

// SetLimitsResult carries effective limits plus pending increases.
type SetLimitsResult struct {
	Limits  *domain.RGLimits
	Pending []*domain.PendingChange
}

// SetLimits applies decreases immediately and stages increases behind the
// cooling period. It runs in a single transaction with outbox staging.
func (s *RGService) SetLimits(ctx context.Context, in SetLimitsInput) (*SetLimitsResult, error) {
	now := time.Now().UTC()
	money := map[domain.LimitType]*decimal.Decimal{
		domain.LimitDepositDaily: in.DepositDaily, domain.LimitDepositWeekly: in.DepositWeekly,
		domain.LimitDepositMonthly: in.DepositMonthly, domain.LimitLossDaily: in.LossDaily,
		domain.LimitLossWeekly: in.LossWeekly, domain.LimitLossMonthly: in.LossMonthly,
		domain.LimitWagerDaily: in.WagerDaily, domain.LimitWagerWeekly: in.WagerWeekly,
	}
	for t, v := range money {
		if v != nil && v.IsNegative() {
			return nil, domain.NewValidationError(domain.FieldError{Field: string(t), Message: "must be >= 0"})
		}
	}
	if in.SessionMinutes != nil && (*in.SessionMinutes < 0 || *in.SessionMinutes > 1440) {
		return nil, domain.NewValidationError(domain.FieldError{Field: "session_minutes", Message: "must be 0..1440"})
	}
	if in.RealityCheckMinutes != nil {
		v := *in.RealityCheckMinutes
		if v != 0 && (v < 15 || v > 240) {
			return nil, domain.NewValidationError(domain.FieldError{Field: "reality_check_minutes", Message: "must be 0 (unset) or 15..240"})
		}
	}

	var result *SetLimitsResult
	err := s.repo.Transact(ctx, func(tx repository.Repository) error {
		limits, err := tx.GetLimits(ctx, in.UserID)
		if err != nil {
			return err
		}
		applied := false
		for t, v := range money {
			if v == nil {
				continue
			}
			current := limits.Get(t)
			switch {
			case current.IsZero():
				// First time a control is set: adding protection takes
				// effect immediately (no existing control is relaxed).
				limits.Set(t, *v)
				applied = true
			case v.LessThan(current):
				// Decrease (or clear to zero): immediate.
				limits.Set(t, *v)
				applied = true
				// Supersede any pending increase for the same control.
				if err := tx.DeletePendingChanges(ctx, in.UserID, t); err != nil {
					return err
				}
			case v.GreaterThan(current):
				// Increase: cooling period.
				if err := tx.DeletePendingChanges(ctx, in.UserID, t); err != nil {
					return err
				}
				ch := &domain.PendingChange{
					ID: uuid.New(), UserID: in.UserID, LimitType: t,
					OldValue: current, NewValue: *v, Status: domain.ChangePending,
					RequestedAt: now, EffectiveAt: now.Add(s.cooling),
				}
				if err := tx.CreatePendingChange(ctx, ch); err != nil {
					return err
				}
			}
		}
		if in.SessionMinutes != nil || in.RealityCheckMinutes != nil {
			cur := limits.SessionMinutes
			rcur := limits.RealityCheckMinutes
			nv := cur
			if in.SessionMinutes != nil {
				nv = *in.SessionMinutes
			}
			rv := rcur
			if in.RealityCheckMinutes != nil {
				rv = *in.RealityCheckMinutes
			}
			// Minute controls follow the same rule independently:
			// unset/decrease apply immediately, increases cool down.
			if in.SessionMinutes != nil {
				switch {
				case cur == 0 || nv < cur:
					limits.SessionMinutes = nv
					applied = true
				case nv > cur:
					if err := tx.DeletePendingChanges(ctx, in.UserID, domain.LimitSessionMinutes); err != nil {
						return err
					}
					if err := tx.CreatePendingChange(ctx, &domain.PendingChange{
						ID: uuid.New(), UserID: in.UserID, LimitType: domain.LimitSessionMinutes,
						OldValue: decimal.NewFromInt(int64(cur)), NewValue: decimal.NewFromInt(int64(nv)),
						Status: domain.ChangePending, RequestedAt: now, EffectiveAt: now.Add(s.cooling),
					}); err != nil {
						return err
					}
				}
			}
			if in.RealityCheckMinutes != nil {
				switch {
				case rcur == 0 || rv < rcur:
					limits.RealityCheckMinutes = rv
					applied = true
				case rv > rcur:
					if err := tx.DeletePendingChanges(ctx, in.UserID, domain.LimitRealityCheck); err != nil {
						return err
					}
					if err := tx.CreatePendingChange(ctx, &domain.PendingChange{
						ID: uuid.New(), UserID: in.UserID, LimitType: domain.LimitRealityCheck,
						OldValue: decimal.NewFromInt(int64(rcur)), NewValue: decimal.NewFromInt(int64(rv)),
						Status: domain.ChangePending, RequestedAt: now, EffectiveAt: now.Add(s.cooling),
					}); err != nil {
						return err
					}
				}
			}
		}
		if applied {
			limits.UpdatedAt = now
			if err := tx.UpsertLimits(ctx, limits); err != nil {
				return err
			}
		}
		pending, err := tx.ListPendingChanges(ctx, in.UserID)
		if err != nil {
			return err
		}
		if err := tx.AppendOutbox(ctx, TopicLimitSet, fmt.Sprintf("%d", in.UserID), map[string]interface{}{
			"user_id": in.UserID, "applied_immediately": applied,
		}); err != nil {
			return err
		}
		result = &SetLimitsResult{Limits: limits, Pending: pending}
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = s.publisher.Publish(ctx, TopicLimitSet, fmt.Sprintf("%d", in.UserID), map[string]interface{}{
		"user_id": in.UserID,
	})
	return result, nil
}

// GetLimits returns effective limits plus pending increases.
func (s *RGService) GetLimits(ctx context.Context, userID int64) (*SetLimitsResult, error) {
	limits, err := s.repo.GetLimits(ctx, userID)
	if err != nil {
		return nil, err
	}
	pending, err := s.repo.ListPendingChanges(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &SetLimitsResult{Limits: limits, Pending: pending}, nil
}

// ApplyDueIncreases applies cooled-down increases. Called by the scheduler
// (admin-triggered job). Returns the number of applied changes.
func (s *RGService) ApplyDueIncreases(ctx context.Context, now time.Time, limit int) (int, error) {
	due, err := s.repo.ListDueChanges(ctx, now, limit)
	if err != nil {
		return 0, err
	}
	applied := 0
	for _, ch := range due {
		err := s.repo.Transact(ctx, func(tx repository.Repository) error {
			limits, err := tx.GetLimits(ctx, ch.UserID)
			if err != nil {
				return err
			}
			// Re-check: a newer decrease may have overtaken the pending increase.
			current := limits.Get(ch.LimitType)
			if !ch.NewValue.GreaterThan(current) {
				return tx.UpdateChangeStatus(ctx, ch.ID, domain.ChangePending, domain.ChangeCancelled)
			}
			limits.Set(ch.LimitType, ch.NewValue)
			limits.UpdatedAt = now
			if err := tx.UpsertLimits(ctx, limits); err != nil {
				return err
			}
			if err := tx.UpdateChangeStatus(ctx, ch.ID, domain.ChangePending, domain.ChangeApplied); err != nil {
				return err
			}
			return tx.AppendOutbox(ctx, TopicLimitIncreaseDue, fmt.Sprintf("%d", ch.UserID), map[string]interface{}{
				"user_id": ch.UserID, "limit_type": string(ch.LimitType), "new_value": ch.NewValue.String(),
			})
		})
		if err != nil {
			s.log.Error("rg: apply increase failed", zap.String("change_id", ch.ID.String()), zap.Error(err))
			continue
		}
		applied++
		_ = s.publisher.Publish(ctx, TopicLimitIncreaseDue, fmt.Sprintf("%d", ch.UserID), map[string]interface{}{
			"user_id": ch.UserID, "limit_type": string(ch.LimitType),
		})
	}
	return applied, nil
}

// ============ Self-exclusion ============

// StartSelfExclusionInput carries the exclusion request.
type StartSelfExclusionInput struct {
	UserID int64
	Period domain.ExclusionPeriod
	Type   domain.ExclusionType
	By     string // "self" for player requests, admin id for operator/regulatory
	Reason string // privacy-sensitive: stored, never logged verbatim
}

// StartSelfExclusion activates an exclusion. Fails if one is already active.
func (s *RGService) StartSelfExclusion(ctx context.Context, in StartSelfExclusionInput) (*domain.Exclusion, error) {
	if !in.Period.IsValid() {
		return nil, domain.NewValidationError(domain.FieldError{Field: "period", Message: "unsupported exclusion period"})
	}
	if in.Type != domain.ExclusionSelf && in.Type != domain.ExclusionOperator && in.Type != domain.ExclusionRegulatory {
		return nil, domain.NewValidationError(domain.FieldError{Field: "type", Message: "unsupported exclusion type"})
	}
	now := time.Now().UTC()
	excl := &domain.Exclusion{
		ID: uuid.New(), UserID: in.UserID, Type: in.Type,
		Status: domain.ExclusionActive, CreatedBy: in.By, CreatedAt: now,
	}
	if in.Period == domain.ExclusionPermanent {
		excl.Permanent = true
	} else {
		until, ok := in.Period.Duration(now)
		if !ok {
			return nil, domain.ErrInvalidPeriod
		}
		excl.Until = &until
	}

	err := s.repo.Transact(ctx, func(tx repository.Repository) error {
		existing, err := tx.GetActiveExclusion(ctx, in.UserID)
		if err != nil {
			return err
		}
		if existing.IsActiveAt(now) {
			return domain.ErrExclusionExists
		}
		if err := tx.CreateExclusion(ctx, excl); err != nil {
			return err
		}
		return tx.AppendOutbox(ctx, TopicExclusionStarted, fmt.Sprintf("%d", in.UserID), map[string]interface{}{
			"exclusion_id": excl.ID.String(), "user_id": in.UserID,
			"type": string(in.Type), "period": string(in.Period), "permanent": excl.Permanent,
		})
	})
	if err != nil {
		return nil, err
	}

	s.log.Info("rg: exclusion started",
		zap.Int64("user_id", in.UserID),
		zap.String("exclusion_id", excl.ID.String()),
		zap.String("type", string(in.Type)),
		zap.Bool("permanent", excl.Permanent))
	_ = s.publisher.Publish(ctx, TopicSelfExcluded, fmt.Sprintf("%d", in.UserID), map[string]interface{}{
		"user_id": in.UserID, "period": string(in.Period), "permanent": excl.Permanent,
	})
	_ = s.publisher.Publish(ctx, TopicExclusionStarted, fmt.Sprintf("%d", in.UserID), map[string]interface{}{
		"exclusion_id": excl.ID.String(), "user_id": in.UserID,
	})
	return excl, nil
}

// RevokeSelfExclusion lifts an EXPIRED temporary exclusion after the
// revocation cooling period and only with explicit confirmation.
// Permanent exclusions are never revocable via any API.
func (s *RGService) RevokeSelfExclusion(ctx context.Context, userID int64, confirm bool, revokedBy string) error {
	if !confirm {
		return domain.ErrConfirmRequired
	}
	now := time.Now().UTC()
	err := s.repo.Transact(ctx, func(tx repository.Repository) error {
		excl, err := tx.GetActiveExclusion(ctx, userID)
		if err != nil {
			return err
		}
		if excl == nil {
			// Maybe already expired: find the latest record.
			history, err := tx.ListExclusions(ctx, userID, 1)
			if err != nil {
				return err
			}
			if len(history) == 0 {
				return domain.ErrExclusionNotFound
			}
			excl = history[0]
		}
		if excl.Permanent {
			return domain.ErrPermanentExclusion
		}
		if excl.IsActiveAt(now) {
			return domain.ErrRevokeTooEarly
		}
		var expiredAt time.Time
		if excl.Until != nil {
			expiredAt = *excl.Until
		} else {
			expiredAt = excl.CreatedAt
		}
		if now.Before(expiredAt.Add(s.revokeGap)) {
			return domain.WithDetails(domain.ErrRevokeCooling, map[string]interface{}{
				"eligible_at": expiredAt.Add(s.revokeGap).UTC().Format(time.RFC3339),
			})
		}
		from := excl.Status
		if from != domain.ExclusionActive && from != domain.ExclusionExpired {
			return domain.ErrConflict
		}
		if err := tx.UpdateExclusionStatus(ctx, excl.ID, from, domain.ExclusionRevoked, map[string]interface{}{
			"revoked_at": now, "revoked_by": revokedBy,
		}); err != nil {
			return err
		}
		return tx.AppendOutbox(ctx, TopicExclusionRevoked, fmt.Sprintf("%d", userID), map[string]interface{}{
			"exclusion_id": excl.ID.String(), "user_id": userID, "revoked_by": revokedBy,
		})
	})
	if err != nil {
		return err
	}
	_ = s.publisher.Publish(ctx, TopicExclusionRevoked, fmt.Sprintf("%d", userID), map[string]interface{}{
		"user_id": userID, "revoked_by": revokedBy,
	})
	return nil
}

// ============ Time-out ============

// StartTimeout activates a short cooling-off period.
func (s *RGService) StartTimeout(ctx context.Context, userID int64, period domain.TimeoutPeriod) (*domain.Timeout, error) {
	if !period.IsValid() {
		return nil, domain.NewValidationError(domain.FieldError{Field: "period", Message: "unsupported timeout period"})
	}
	now := time.Now().UTC()
	until, ok := period.Duration(now)
	if !ok {
		return nil, domain.ErrInvalidPeriod
	}
	to := &domain.Timeout{ID: uuid.New(), UserID: userID, Until: until, CreatedAt: now}
	err := s.repo.Transact(ctx, func(tx repository.Repository) error {
		existing, err := tx.GetActiveTimeout(ctx, userID, now)
		if err != nil {
			return err
		}
		if existing.IsActiveAt(now) {
			return domain.ErrTimeoutExists
		}
		if err := tx.CreateTimeout(ctx, to); err != nil {
			return err
		}
		return tx.AppendOutbox(ctx, TopicTimeoutStarted, fmt.Sprintf("%d", userID), map[string]interface{}{
			"timeout_id": to.ID.String(), "user_id": userID, "until": until.UTC().Format(time.RFC3339),
		})
	})
	if err != nil {
		return nil, err
	}
	_ = s.publisher.Publish(ctx, TopicTimeoutStarted, fmt.Sprintf("%d", userID), map[string]interface{}{
		"timeout_id": to.ID.String(), "user_id": userID,
	})
	return to, nil
}

// ============ Spend & status ============

// SpendKind classifies money movement for limit accounting.
type SpendKind string

const (
	SpendDeposit SpendKind = "deposit"
	SpendWager   SpendKind = "wager"
	SpendPayout  SpendKind = "payout"
)

// RecordSpendInput reports settled money movement in account currency.
type RecordSpendInput struct {
	UserID   int64
	Kind     SpendKind
	Amount   decimal.Decimal
	Currency string
}

// RecordSpend accumulates one movement into today's aggregate.
func (s *RGService) RecordSpend(ctx context.Context, in RecordSpendInput) error {
	if in.Amount.IsNegative() {
		return domain.NewValidationError(domain.FieldError{Field: "amount", Message: "must be >= 0"})
	}
	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	if len(currency) != 3 {
		return domain.NewValidationError(domain.FieldError{Field: "currency", Message: "must be ISO 4217"})
	}
	dep, wag, pay := "0", "0", "0"
	switch in.Kind {
	case SpendDeposit:
		dep = in.Amount.String()
	case SpendWager:
		wag = in.Amount.String()
	case SpendPayout:
		pay = in.Amount.String()
	default:
		return domain.NewValidationError(domain.FieldError{Field: "kind", Message: "must be deposit|wager|payout"})
	}
	return s.repo.RecordSpend(ctx, in.UserID, time.Now().UTC(), currency, dep, wag, pay)
}

// Status aggregates the protection posture for dashboards and support.
type Status struct {
	GamblingAllowed bool
	BlockedReason   domain.ReasonCode
	Limits          *domain.RGLimits
	Pending         []*domain.PendingChange
	Exclusion       *domain.Exclusion
	Timeout         *domain.Timeout
}

// GetStatus computes the aggregate posture (no money movement).
func (s *RGService) GetStatus(ctx context.Context, userID int64) (*Status, error) {
	now := time.Now().UTC()
	limits, err := s.repo.GetLimits(ctx, userID)
	if err != nil {
		return nil, err
	}
	pending, err := s.repo.ListPendingChanges(ctx, userID)
	if err != nil {
		return nil, err
	}
	excl, err := s.repo.GetActiveExclusion(ctx, userID)
	if err != nil {
		return nil, err
	}
	st := &Status{GamblingAllowed: true, BlockedReason: domain.ReasonOK, Limits: limits, Pending: pending}
	if excl.IsActiveAt(now) {
		st.GamblingAllowed = false
		st.BlockedReason = domain.ReasonSelfExcluded
		st.Exclusion = excl
		return st, nil
	}
	if excl != nil && excl.Status == domain.ExclusionActive {
		st.Exclusion = excl // expired-but-unrevoked record, kept for audit visibility
	}
	to, err := s.repo.GetActiveTimeout(ctx, userID, now)
	if err != nil {
		return nil, err
	}
	if to.IsActiveAt(now) {
		st.GamblingAllowed = false
		st.BlockedReason = domain.ReasonCoolOff
		st.Timeout = to
	}
	return st, nil
}
