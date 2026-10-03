package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/opus-casino/bonus/internal/domain"
	"github.com/opus-casino/bonus/internal/repository"
)

// BonusConfig holds bonus policy parameters.
// Money values use decimal.Decimal — NEVER float64 (CONVENTIONS NEVER-6).
type BonusConfig struct {
	WelcomePct        int             // e.g. 100 (= 100%)
	WelcomeMaxUSD     decimal.Decimal // e.g. 200.00
	WelcomeWagering   int             // e.g. 30x
	WelcomeExpiryDays int             // e.g. 30
}

// DefaultBonusConfig returns safe production defaults.
func DefaultBonusConfig() BonusConfig {
	return BonusConfig{
		WelcomePct:        100,
		WelcomeMaxUSD:     decimal.NewFromInt(200),
		WelcomeWagering:   30,
		WelcomeExpiryDays: 30,
	}
}

// BonusCreditInput describes a wagering-conversion credit to the real-money wallet.
type BonusCreditInput struct {
	UserID         int64
	Amount         decimal.Decimal
	Currency       string
	ReferenceID    string
	IdempotencyKey string
}

// WalletCrediter credits converted bonus funds to wallet-core.
// Implementations must be idempotent on IdempotencyKey.
type WalletCrediter interface {
	CreditBonusConversion(ctx context.Context, in BonusCreditInput) error
}

// WalletClient is a WalletCrediter with a lifecycle, returned by the concrete
// client constructor so the process can release the connection on shutdown.
type WalletClient interface {
	WalletCrediter
	Close() error
}

// MetricsRecorder receives business/domain events for Prometheus.
// It is intentionally narrow and string-typed so the service layer stays free
// of Prometheus types. Label values must be bounded enums — never user input.
// A nil recorder is valid and behaves as a no-op.
type MetricsRecorder interface {
	BonusAwarded(bonusType, currency, amount string)
	BonusAwardSkipped(reason string)
	WagerRecorded(result string)
	WageringCompleted(bonusType, currency, ratio string)
	ConversionCredit(result string)
	PaymentEvent(result string)
}

// Bounded label values for MetricsRecorder. Keeping them as constants makes the
// cardinality contract explicit and greppable.
const (
	// BonusAwardSkipped reasons.
	SkipAlreadyAwarded = "already_awarded"
	SkipInvalidAmount  = "invalid_amount"

	// WagerRecorded results.
	WagerRecordedResult      = "recorded"
	WagerNoActiveBonus       = "no_active_bonus"
	WagerExpired             = "expired"
	WagerCreditFailed        = "credit_failed"
	WagerInvalidAmount       = "invalid_amount"
	WagerRepositoryFailure   = "repository_failure"
	WagerCompletedConversion = "completed"

	// ConversionCredit results.
	CreditSucceeded      = "credited"
	CreditSkippedNoWalle = "skipped_no_wallet"
	CreditFailed         = "failed"

	// PaymentEvent results.
	PaymentEventAwarded         = "awarded"
	PaymentEventNotFirstDeposit = "skipped_not_first"
	PaymentEventMalformed       = "malformed"
	PaymentEventFailed          = "failed"
)

// BonusService handles bonus business logic.
// It depends on the repository interface, never on *gorm.DB directly,
// except for the legacy db handle kept for backward compatibility.
type BonusService struct {
	db      *gorm.DB
	repo    repository.BonusRepository
	wallet  WalletCrediter
	metrics MetricsRecorder
	cfg     BonusConfig
	log     *zap.Logger
}

// NewBonusService creates a new bonus service (legacy constructor).
func NewBonusService(db *gorm.DB, cfg BonusConfig, log *zap.Logger) *BonusService {
	if log == nil {
		log = zap.NewNop()
	}
	return &BonusService{
		db:      db,
		repo:    repository.NewBonusRepository(db),
		metrics: noopMetrics{},
		cfg:     cfg,
		log:     log,
	}
}

// NewBonusServiceWithRepository creates a service with an explicit repository.
// Preferred for tests and new code.
func NewBonusServiceWithRepository(repo repository.BonusRepository, cfg BonusConfig, log *zap.Logger) *BonusService {
	if log == nil {
		log = zap.NewNop()
	}
	return &BonusService{repo: repo, metrics: noopMetrics{}, cfg: cfg, log: log}
}

// SetWalletCrediter attaches the wallet-core credit client.
// A nil crediter means dev mode: completion is recorded without crediting
// (loudly logged). Production must always set a real client.
func (s *BonusService) SetWalletCrediter(w WalletCrediter) {
	s.wallet = w
}

// noopMetrics is the default recorder: it keeps the service free of
// telemetry wiring in tests and in any bootstrap that does not need metrics.
type noopMetrics struct{}

func (noopMetrics) BonusAwarded(string, string, string)      {}
func (noopMetrics) BonusAwardSkipped(string)                 {}
func (noopMetrics) WagerRecorded(string)                     {}
func (noopMetrics) WageringCompleted(string, string, string) {}
func (noopMetrics) ConversionCredit(string)                  {}
func (noopMetrics) PaymentEvent(string)                      {}

var _ MetricsRecorder = noopMetrics{}

// NopMetrics returns a recorder that discards every event.
// Useful for the consumer and for tests that assert behaviour, not metrics.
func NopMetrics() MetricsRecorder { return noopMetrics{} }

// SetMetrics attaches a Prometheus recorder. Nil or a nil interface is
// replaced with a no-op recorder, so callers never need nil checks.
func (s *BonusService) SetMetrics(m MetricsRecorder) {
	if m == nil {
		m = noopMetrics{}
	}
	s.metrics = m
}

// completionRatio reports wagered/required as a string for the ratio histogram.
// Returns "" when the requirement is zero (ratio is undefined).
func completionRatio(required, wagered decimal.Decimal) string {
	if required.IsZero() {
		return ""
	}
	ratio := wagered.Div(required)
	// Round to 4 decimals to keep the value stable across scrapes.
	return ratio.Round(4).String()
}

// ─── Welcome Bonus ────────────────────────────────────────────────────────────

// AwardWelcomeBonus awards a welcome bonus on first deposit.
// depositAmountUSD is the fiat-equivalent deposit amount.
// Idempotent: if the user already has a welcome bonus, returns it (no-op).
func (s *BonusService) AwardWelcomeBonus(ctx context.Context, userID int64, depositAmountUSD decimal.Decimal) (*domain.Bonus, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("bonus: invalid user_id")
	}
	if depositAmountUSD.IsNegative() || depositAmountUSD.IsZero() {
		s.metrics.BonusAwardSkipped(SkipInvalidAmount)
		return nil, domain.ErrInvalidBonusAmount
	}

	existing, err := s.repo.FindWelcome(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("bonus: check existing welcome: %w", err)
	}
	if existing != nil {
		s.metrics.BonusAwardSkipped(SkipAlreadyAwarded)
		s.log.Info("Welcome bonus already awarded", zap.Int64("user_id", userID))
		return existing, nil
	}

	// Calculate bonus amount: min(deposit * pct%, maxUSD) — all decimal math.
	pct := decimal.NewFromInt(int64(s.cfg.WelcomePct)).Div(decimal.NewFromInt(100))
	bonusAmt := depositAmountUSD.Mul(pct)
	if bonusAmt.GreaterThan(s.cfg.WelcomeMaxUSD) {
		bonusAmt = s.cfg.WelcomeMaxUSD
	}

	if bonusAmt.IsZero() || bonusAmt.IsNegative() {
		s.metrics.BonusAwardSkipped(SkipInvalidAmount)
		return nil, domain.ErrInvalidBonusAmount
	}

	wageringReq := bonusAmt.Mul(decimal.NewFromInt(int64(s.cfg.WelcomeWagering)))
	expiresAt := time.Now().AddDate(0, 0, s.cfg.WelcomeExpiryDays)

	now := time.Now()
	bonus := &domain.Bonus{
		ID:                 uuid.New(),
		UserID:             userID,
		Type:               domain.BonusTypeWelcome,
		Status:             domain.BonusStatusActive,
		BonusAmount:        bonusAmt,
		RealAmount:         depositAmountUSD,
		Currency:           "USD",
		WageringRequired:   wageringReq,
		WageringMultiplier: s.cfg.WelcomeWagering,
		ExpiresAt:          expiresAt,
		ActivatedAt:        &now,
	}

	if err := s.repo.Create(ctx, bonus); err != nil {
		return nil, fmt.Errorf("bonus: create welcome: %w", err)
	}

	s.metrics.BonusAwarded(string(domain.BonusTypeWelcome), bonus.Currency, bonusAmt.String())
	s.log.Info("Welcome bonus awarded",
		zap.Int64("user_id", userID),
		zap.String("amount", bonusAmt.StringFixed(2)),
		zap.String("wagering_req", wageringReq.StringFixed(2)),
		zap.Time("expires_at", expiresAt))

	return bonus, nil
}

// ─── Wagering ─────────────────────────────────────────────────────────────────

// RecordWager records a user's bet against their active bonus wagering requirement.
func (s *BonusService) RecordWager(ctx context.Context, userID int64, betAmountUSD decimal.Decimal) error {
	if betAmountUSD.IsNegative() || betAmountUSD.IsZero() {
		s.metrics.WagerRecorded(WagerInvalidAmount)
		return nil // ignore non-positive wagers
	}
	bonus, err := s.repo.FindActive(ctx, userID)
	if err != nil {
		s.metrics.WagerRecorded(WagerRepositoryFailure)
		return err
	}
	if bonus == nil {
		s.metrics.WagerRecorded(WagerNoActiveBonus)
		return nil // No active bonus — nothing to track
	}

	if bonus.IsExpired() {
		s.metrics.WagerRecorded(WagerExpired)
		return s.expireBonus(ctx, bonus)
	}

	newCompleted := bonus.WageringCompleted.Add(betAmountUSD)
	update := map[string]interface{}{"wagering_completed": newCompleted.String()}

	if newCompleted.GreaterThanOrEqual(bonus.WageringRequired) {
		now := time.Now()
		update["status"] = domain.BonusStatusCompleted
		update["completed_at"] = now
		s.log.Info("Bonus wagering complete — converting to real balance",
			zap.Int64("user_id", userID),
			zap.String("bonus_id", bonus.ID.String()),
			zap.String("bonus_amount", bonus.BonusAmount.StringFixed(2)))

		// Credit BEFORE marking completed: wallet-core dedupes on the
		// idempotency key, so a retry after a DB failure is safe, while
		// marking completed first could lose money on credit failure.
		if err := s.creditConversion(ctx, bonus); err != nil {
			s.metrics.WagerRecorded(WagerCreditFailed)
			return err
		}
		s.metrics.WageringCompleted(string(bonus.Type), bonus.Currency,
			completionRatio(bonus.WageringRequired, newCompleted))
		s.metrics.WagerRecorded(WagerCompletedConversion)
		return s.repo.UpdateWagering(ctx, bonus.ID, update)
	}

	if err := s.repo.UpdateWagering(ctx, bonus.ID, update); err != nil {
		s.metrics.WagerRecorded(WagerRepositoryFailure)
		return err
	}
	s.metrics.WagerRecorded(WagerRecordedResult)
	return nil
}

// creditConversion credits the bonus amount to the user's real-money wallet.
// The idempotency key is stable per bonus, so retries never double-credit.
func (s *BonusService) creditConversion(ctx context.Context, bonus *domain.Bonus) error {
	if s.wallet == nil {
		s.metrics.ConversionCredit(CreditSkippedNoWalle)
		s.log.Error("Bonus: wagering completed but wallet client is not configured — credit SKIPPED",
			zap.Int64("user_id", bonus.UserID),
			zap.String("bonus_id", bonus.ID.String()),
			zap.String("amount", bonus.BonusAmount.StringFixed(2)))
		return nil
	}
	err := s.wallet.CreditBonusConversion(ctx, BonusCreditInput{
		UserID:         bonus.UserID,
		Amount:         bonus.BonusAmount,
		Currency:       bonus.Currency,
		ReferenceID:    bonus.ID.String(),
		IdempotencyKey: "bonus-wagering-" + bonus.ID.String(),
	})
	if err != nil {
		s.metrics.ConversionCredit(CreditFailed)
		return err
	}
	s.metrics.ConversionCredit(CreditSucceeded)
	return nil
}

// GetActiveBonus returns the current active bonus for a user, if any.
func (s *BonusService) GetActiveBonus(ctx context.Context, userID int64) (*domain.Bonus, error) {
	bonus, err := s.repo.FindActive(ctx, userID)
	if err != nil {
		return nil, err
	}
	if bonus == nil {
		return nil, nil
	}
	if bonus.IsExpired() {
		_ = s.expireBonus(ctx, bonus)
		return nil, nil
	}
	return bonus, nil
}

// GetBonus returns a specific bonus owned by the user.
func (s *BonusService) GetBonus(ctx context.Context, userID int64, bonusID uuid.UUID) (*domain.Bonus, error) {
	bonus, err := s.repo.FindByID(ctx, userID, bonusID)
	if err != nil {
		return nil, err
	}
	if bonus == nil {
		return nil, domain.ErrBonusNotFound
	}
	return bonus, nil
}

// ActivateBonus moves a pending bonus to active.
func (s *BonusService) ActivateBonus(ctx context.Context, userID int64, bonusID uuid.UUID) (*domain.Bonus, error) {
	bonus, err := s.repo.FindByID(ctx, userID, bonusID)
	if err != nil {
		return nil, err
	}
	if bonus == nil {
		return nil, domain.ErrBonusNotFound
	}
	if bonus.Status != domain.BonusStatusPending {
		return nil, domain.ErrBonusNotActive
	}
	activated, err := s.repo.Activate(ctx, bonusID, userID)
	if err != nil {
		return nil, err
	}
	s.log.Info("Bonus activated", zap.String("bonus_id", bonusID.String()), zap.Int64("user_id", userID))
	return activated, nil
}

// CancelBonus cancels a pending/active bonus owned by the user.
func (s *BonusService) CancelBonus(ctx context.Context, userID int64, bonusID uuid.UUID) (*domain.Bonus, error) {
	bonus, err := s.repo.FindByID(ctx, userID, bonusID)
	if err != nil {
		return nil, err
	}
	if bonus == nil {
		return nil, domain.ErrBonusNotFound
	}
	if bonus.Status != domain.BonusStatusPending && bonus.Status != domain.BonusStatusActive {
		return nil, domain.ErrBonusNotActive
	}
	cancelled, err := s.repo.Cancel(ctx, bonusID, userID)
	if err != nil {
		return nil, err
	}
	s.log.Info("Bonus cancelled", zap.String("bonus_id", bonusID.String()), zap.Int64("user_id", userID))
	return cancelled, nil
}

// ListBonuses returns all bonuses for a user (paginated).
func (s *BonusService) ListBonuses(ctx context.Context, userID int64, limit, offset int) ([]*domain.Bonus, int64, error) {
	return s.repo.ListByUser(ctx, userID, limit, offset)
}

func (s *BonusService) expireBonus(ctx context.Context, bonus *domain.Bonus) error {
	s.log.Info("Expiring bonus", zap.String("bonus_id", bonus.ID.String()))
	return s.repo.MarkExpired(ctx, bonus.ID)
}
