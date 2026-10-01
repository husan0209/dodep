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
)

// BonusConfig holds bonus policy parameters (loaded from env).
type BonusConfig struct {
	WelcomePct        int     // e.g. 100 (= 100%)
	WelcomeMaxUSD     float64 // e.g. 200.0
	WelcomeWagering   int     // e.g. 30x
	WelcomeExpiryDays int     // e.g. 30
}

// DefaultBonusConfig returns safe production defaults.
func DefaultBonusConfig() BonusConfig {
	return BonusConfig{
		WelcomePct:        100,
		WelcomeMaxUSD:     200,
		WelcomeWagering:   30,
		WelcomeExpiryDays: 30,
	}
}

// BonusService handles bonus business logic.
type BonusService struct {
	db   *gorm.DB
	cfg  BonusConfig
	log  *zap.Logger
}

// NewBonusService creates a new bonus service.
func NewBonusService(db *gorm.DB, cfg BonusConfig, log *zap.Logger) *BonusService {
	return &BonusService{db: db, cfg: cfg, log: log}
}

// ─── Welcome Bonus ────────────────────────────────────────────────────────────

// AwardWelcomeBonus awards a welcome bonus on first deposit.
// depositAmountUSD is the fiat-equivalent deposit amount.
// Idempotent: if the user already has an active/completed welcome bonus, no-op.
func (s *BonusService) AwardWelcomeBonus(ctx context.Context, userID int64, depositAmountUSD decimal.Decimal) (*domain.Bonus, error) {
	// Idempotency: check for existing welcome bonus
	var existing domain.Bonus
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND type = ? AND status IN ('pending','active','completed')", userID, domain.BonusTypeWelcome).
		First(&existing).Error
	if err == nil {
		s.log.Info("Welcome bonus already awarded", zap.Int64("user_id", userID))
		return &existing, nil
	}

	// Calculate bonus amount: min(deposit * pct%, maxUSD)
	pct := decimal.NewFromInt(int64(s.cfg.WelcomePct)).Div(decimal.NewFromInt(100))
	bonusAmt := depositAmountUSD.Mul(pct)
	maxAmt := decimal.NewFromFloat(s.cfg.WelcomeMaxUSD)
	if bonusAmt.GreaterThan(maxAmt) {
		bonusAmt = maxAmt
	}

	if bonusAmt.IsZero() || bonusAmt.IsNegative() {
		return nil, fmt.Errorf("bonus: zero or negative bonus amount for deposit %s", depositAmountUSD)
	}

	wageringReq := bonusAmt.Mul(decimal.NewFromInt(int64(s.cfg.WelcomeWagering)))
	expiresAt := time.Now().AddDate(0, 0, s.cfg.WelcomeExpiryDays)

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
	}
	now := time.Now()
	bonus.ActivatedAt = &now

	if err := s.db.WithContext(ctx).Create(bonus).Error; err != nil {
		return nil, fmt.Errorf("bonus: create welcome: %w", err)
	}

	s.log.Info("Welcome bonus awarded",
		zap.Int64("user_id", userID),
		zap.String("amount", bonusAmt.StringFixed(2)),
		zap.String("wagering_req", wageringReq.StringFixed(2)),
		zap.Time("expires_at", expiresAt))

	return bonus, nil
}

// ─── Wagering ─────────────────────────────────────────────────────────────────

// RecordWager records a user's bet against their active bonus wagering requirement.
// Called whenever the user places a casino or sportsbook bet.
func (s *BonusService) RecordWager(ctx context.Context, userID int64, betAmountUSD decimal.Decimal) error {
	var bonus domain.Bonus
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND status = 'active'", userID).
		Order("created_at DESC").
		First(&bonus).Error
	if err != nil {
		return nil // No active bonus — nothing to track
	}

	if bonus.IsExpired() {
		return s.expireBonus(ctx, &bonus)
	}

	newCompleted := bonus.WageringCompleted.Add(betAmountUSD)
	update := map[string]interface{}{"wagering_completed": newCompleted}

	if newCompleted.GreaterThanOrEqual(bonus.WageringRequired) {
		now := time.Now()
		update["status"] = domain.BonusStatusCompleted
		update["completed_at"] = now
		s.log.Info("Bonus wagering complete — converting to real balance",
			zap.Int64("user_id", userID),
			zap.String("bonus_id", bonus.ID.String()),
			zap.String("bonus_amount", bonus.BonusAmount.StringFixed(2)))

		// TODO: Emit event to wallet service to credit real balance
		// wallet_client.CreditWallet(ctx, userID, bonus.BonusAmount, "bonus_conversion", bonus.ID.String())
	}

	return s.db.WithContext(ctx).Model(&bonus).Updates(update).Error
}

// GetActiveBonus returns the current active bonus for a user, if any.
func (s *BonusService) GetActiveBonus(ctx context.Context, userID int64) (*domain.Bonus, error) {
	var bonus domain.Bonus
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND status = 'active'", userID).
		First(&bonus).Error; err != nil {
		return nil, nil // Not found = no active bonus
	}
	if bonus.IsExpired() {
		_ = s.expireBonus(ctx, &bonus)
		return nil, nil
	}
	return &bonus, nil
}

// ListBonuses returns all bonuses for a user (paginated).
func (s *BonusService) ListBonuses(ctx context.Context, userID int64, limit, offset int) ([]*domain.Bonus, int64, error) {
	var bonuses []*domain.Bonus
	var total int64

	query := s.db.WithContext(ctx).Model(&domain.Bonus{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Limit(limit).Offset(offset).Order("created_at DESC").Find(&bonuses).Error; err != nil {
		return nil, 0, err
	}

	return bonuses, total, nil
}

func (s *BonusService) expireBonus(ctx context.Context, bonus *domain.Bonus) error {
	s.log.Info("Expiring bonus", zap.String("bonus_id", bonus.ID.String()))
	return s.db.WithContext(ctx).Model(bonus).Update("status", domain.BonusStatusExpired).Error
}
