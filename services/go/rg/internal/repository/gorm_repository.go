package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/opus-casino/rg/internal/domain"
)

// Service-owned tables (rg_service_*). Rationale: the legacy
// rg_player_limits table keys players by UUID with a hard FK that can
// never reference users(BIGSERIAL id); this service keys by BIGINT user_id
// with no cross-service FK (microservice boundary).

type limitModel struct {
	UserID              int64           `gorm:"primaryKey"`
	DepositDaily        decimal.Decimal `gorm:"type:numeric(18,8);not null;default:0"`
	DepositWeekly       decimal.Decimal `gorm:"type:numeric(18,8);not null;default:0"`
	DepositMonthly      decimal.Decimal `gorm:"type:numeric(18,8);not null;default:0"`
	LossDaily           decimal.Decimal `gorm:"type:numeric(18,8);not null;default:0"`
	LossWeekly          decimal.Decimal `gorm:"type:numeric(18,8);not null;default:0"`
	LossMonthly         decimal.Decimal `gorm:"type:numeric(18,8);not null;default:0"`
	WagerDaily          decimal.Decimal `gorm:"type:numeric(18,8);not null;default:0"`
	WagerWeekly         decimal.Decimal `gorm:"type:numeric(18,8);not null;default:0"`
	SessionMinutes      int             `gorm:"not null;default:0"`
	RealityCheckMinutes int             `gorm:"not null;default:0"`
	UpdatedAt           time.Time
}

// TableName overrides default pluralization.
func (limitModel) TableName() string { return "rg_service_limits" }

type pendingChangeModel struct {
	ID          uuid.UUID       `gorm:"type:uuid;primaryKey"`
	UserID      int64           `gorm:"not null;index"`
	LimitType   string          `gorm:"type:varchar(32);not null"`
	OldValue    decimal.Decimal `gorm:"type:numeric(18,8);not null"`
	NewValue    decimal.Decimal `gorm:"type:numeric(18,8);not null"`
	Status      string          `gorm:"type:varchar(16);not null;default:'pending';index"`
	RequestedAt time.Time       `gorm:"not null"`
	EffectiveAt time.Time       `gorm:"not null;index"`
}

// TableName overrides default pluralization.
func (pendingChangeModel) TableName() string { return "rg_service_pending_changes" }

type exclusionModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    int64     `gorm:"not null;index"`
	Type      string    `gorm:"type:varchar(16);not null"`
	Status    string    `gorm:"type:varchar(16);not null;default:'active';index"`
	Until     *time.Time
	Permanent bool      `gorm:"not null;default:false"`
	CreatedBy string    `gorm:"type:varchar(128);not null;default:''"`
	CreatedAt time.Time `gorm:"not null"`
	RevokedAt *time.Time
	RevokedBy string `gorm:"type:varchar(128);not null;default:''"`
}

// TableName overrides default pluralization.
func (exclusionModel) TableName() string { return "rg_service_exclusions" }

type timeoutModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    int64     `gorm:"not null;index"`
	Until     time.Time `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null"`
}

// TableName overrides default pluralization.
func (timeoutModel) TableName() string { return "rg_service_timeouts" }

type spendDayModel struct {
	UserID   int64           `gorm:"primaryKey"`
	Day      time.Time       `gorm:"primaryKey;type:date"`
	Currency string          `gorm:"primaryKey;type:char(3)"`
	Deposits decimal.Decimal `gorm:"type:numeric(18,8);not null;default:0"`
	Wagers   decimal.Decimal `gorm:"type:numeric(18,8);not null;default:0"`
	Payouts  decimal.Decimal `gorm:"type:numeric(18,8);not null;default:0"`
}

// TableName overrides default pluralization.
func (spendDayModel) TableName() string { return "rg_service_spend_daily" }

type outboxModel struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey"`
	Topic       string         `gorm:"type:varchar(128);not null"`
	EventKey    string         `gorm:"type:varchar(128);not null"`
	Payload     map[string]any `gorm:"serializer:json"`
	CreatedAt   time.Time      `gorm:"not null"`
	PublishedAt *time.Time
}

// TableName overrides default pluralization.
func (outboxModel) TableName() string { return "rg_service_outbox" }

// GormRepository is the GORM-backed Repository implementation.
type GormRepository struct {
	db *gorm.DB
}

// NewGormRepository builds a repository. Panics on nil db (fail fast).
func NewGormRepository(db *gorm.DB) *GormRepository {
	if db == nil {
		panic("rg: gorm.DB is required")
	}
	return &GormRepository{db: db}
}

// AutoMigrate creates service-owned tables (CREATE TABLE / ADD COLUMN only).
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&limitModel{}, &pendingChangeModel{}, &exclusionModel{},
		&timeoutModel{}, &spendDayModel{}, &outboxModel{},
	)
}

func (r *GormRepository) GetLimits(ctx context.Context, userID int64) (*domain.RGLimits, error) {
	var m limitModel
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&m).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &domain.RGLimits{UserID: userID}, nil
		}
		return nil, fmt.Errorf("rg: get limits: %w", err)
	}
	return &domain.RGLimits{
		UserID: m.UserID, DepositDaily: m.DepositDaily, DepositWeekly: m.DepositWeekly,
		DepositMonthly: m.DepositMonthly, LossDaily: m.LossDaily, LossWeekly: m.LossWeekly,
		LossMonthly: m.LossMonthly, WagerDaily: m.WagerDaily, WagerWeekly: m.WagerWeekly,
		SessionMinutes: m.SessionMinutes, RealityCheckMinutes: m.RealityCheckMinutes,
		UpdatedAt: m.UpdatedAt,
	}, nil
}

func (r *GormRepository) UpsertLimits(ctx context.Context, limits *domain.RGLimits) error {
	m := limitModel{
		UserID: limits.UserID, DepositDaily: limits.DepositDaily, DepositWeekly: limits.DepositWeekly,
		DepositMonthly: limits.DepositMonthly, LossDaily: limits.LossDaily, LossWeekly: limits.LossWeekly,
		LossMonthly: limits.LossMonthly, WagerDaily: limits.WagerDaily, WagerWeekly: limits.WagerWeekly,
		SessionMinutes: limits.SessionMinutes, RealityCheckMinutes: limits.RealityCheckMinutes,
		UpdatedAt: time.Now().UTC(),
	}
	// Explicit upsert: Save() would emit UPDATE-then-Create, which races when
	// two requests set limits for the same first time.
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"deposit_daily", "deposit_weekly", "deposit_monthly",
			"loss_daily", "loss_weekly", "loss_monthly",
			"wager_daily", "wager_weekly",
			"session_minutes", "reality_check_minutes", "updated_at",
		}),
	}).Create(&m).Error
	if err != nil {
		return fmt.Errorf("rg: upsert limits: %w", err)
	}
	return nil
}

func (r *GormRepository) CreatePendingChange(ctx context.Context, ch *domain.PendingChange) error {
	m := pendingChangeModel{
		ID: ch.ID, UserID: ch.UserID, LimitType: string(ch.LimitType),
		OldValue: ch.OldValue, NewValue: ch.NewValue, Status: string(ch.Status),
		RequestedAt: ch.RequestedAt, EffectiveAt: ch.EffectiveAt,
	}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		return fmt.Errorf("rg: create pending change: %w", err)
	}
	return nil
}

func (r *GormRepository) ListPendingChanges(ctx context.Context, userID int64) ([]*domain.PendingChange, error) {
	var models []pendingChangeModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, string(domain.ChangePending)).
		Order("effective_at ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("rg: list pending changes: %w", err)
	}
	return toPendingDomain(models), nil
}

func (r *GormRepository) ListDueChanges(ctx context.Context, now time.Time, limit int) ([]*domain.PendingChange, error) {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	var models []pendingChangeModel
	if err := r.db.WithContext(ctx).
		Where("status = ? AND effective_at <= ?", string(domain.ChangePending), now).
		Order("effective_at ASC").Limit(limit).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("rg: list due changes: %w", err)
	}
	return toPendingDomain(models), nil
}

func (r *GormRepository) UpdateChangeStatus(ctx context.Context, id uuid.UUID, from domain.ChangeStatus, to domain.ChangeStatus) error {
	res := r.db.WithContext(ctx).Model(&pendingChangeModel{}).
		Where("id = ? AND status = ?", id, string(from)).
		Update("status", string(to))
	if res.Error != nil {
		return fmt.Errorf("rg: update change status: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrConflict
	}
	return nil
}

func (r *GormRepository) DeletePendingChanges(ctx context.Context, userID int64, limitType domain.LimitType) error {
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND limit_type = ? AND status = ?", userID, string(limitType), string(domain.ChangePending)).
		Delete(&pendingChangeModel{}).Error; err != nil {
		return fmt.Errorf("rg: delete pending changes: %w", err)
	}
	return nil
}

func (r *GormRepository) CreateExclusion(ctx context.Context, e *domain.Exclusion) error {
	m := exclusionModel{
		ID: e.ID, UserID: e.UserID, Type: string(e.Type), Status: string(e.Status),
		Until: e.Until, Permanent: e.Permanent, CreatedBy: e.CreatedBy, CreatedAt: e.CreatedAt,
		RevokedAt: e.RevokedAt, RevokedBy: e.RevokedBy,
	}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		return fmt.Errorf("rg: create exclusion: %w", err)
	}
	return nil
}

func (r *GormRepository) GetActiveExclusion(ctx context.Context, userID int64) (*domain.Exclusion, error) {
	var m exclusionModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, string(domain.ExclusionActive)).
		Order("created_at DESC").First(&m).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("rg: get active exclusion: %w", err)
	}
	return toExclusionDomain(&m), nil
}

func (r *GormRepository) ListExclusions(ctx context.Context, userID int64, limit int) ([]*domain.Exclusion, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var models []exclusionModel
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).
		Order("created_at DESC").Limit(limit).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("rg: list exclusions: %w", err)
	}
	out := make([]*domain.Exclusion, 0, len(models))
	for i := range models {
		out = append(out, toExclusionDomain(&models[i]))
	}
	return out, nil
}

func (r *GormRepository) UpdateExclusionStatus(ctx context.Context, id uuid.UUID, from domain.ExclusionStatus, to domain.ExclusionStatus, patch map[string]interface{}) error {
	updates := map[string]interface{}{"status": string(to)}
	for k, v := range patch {
		updates[k] = v
	}
	res := r.db.WithContext(ctx).Model(&exclusionModel{}).
		Where("id = ? AND status = ?", id, string(from)).
		Updates(updates)
	if res.Error != nil {
		return fmt.Errorf("rg: update exclusion status: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrConflict
	}
	return nil
}

func (r *GormRepository) CreateTimeout(ctx context.Context, t *domain.Timeout) error {
	m := timeoutModel{ID: t.ID, UserID: t.UserID, Until: t.Until, CreatedAt: t.CreatedAt}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		return fmt.Errorf("rg: create timeout: %w", err)
	}
	return nil
}

func (r *GormRepository) GetActiveTimeout(ctx context.Context, userID int64, now time.Time) (*domain.Timeout, error) {
	var m timeoutModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND until > ?", userID, now).
		Order("until DESC").First(&m).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("rg: get active timeout: %w", err)
	}
	return &domain.Timeout{ID: m.ID, UserID: m.UserID, Until: m.Until, CreatedAt: m.CreatedAt}, nil
}

// Transact runs fn inside a single database transaction, so state changes and
// outbox events commit atomically (transactional outbox pattern).
func (r *GormRepository) Transact(ctx context.Context, fn func(Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&GormRepository{db: tx})
	})
}

func (r *GormRepository) RecordSpend(ctx context.Context, userID int64, day time.Time, currency, deposits, wagers, payouts string) error {
	dep, err := decimal.NewFromString(deposits)
	if err != nil {
		return fmt.Errorf("rg: bad deposits amount: %w", err)
	}
	wag, err := decimal.NewFromString(wagers)
	if err != nil {
		return fmt.Errorf("rg: bad wagers amount: %w", err)
	}
	pay, err := decimal.NewFromString(payouts)
	if err != nil {
		return fmt.Errorf("rg: bad payouts amount: %w", err)
	}
	if dep.IsNegative() || wag.IsNegative() || pay.IsNegative() {
		return domain.NewValidationError(domain.FieldError{Field: "amount", Message: "spend amounts must be >= 0"})
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if len(currency) != 3 {
		return domain.NewValidationError(domain.FieldError{Field: "currency", Message: "must be ISO 4217"})
	}
	day = day.UTC().Truncate(24 * time.Hour)
	err = r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "day"}, {Name: "currency"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"deposits": gorm.Expr("rg_service_spend_daily.deposits + ?", dep),
			"wagers":   gorm.Expr("rg_service_spend_daily.wagers + ?", wag),
			"payouts":  gorm.Expr("rg_service_spend_daily.payouts + ?", pay),
		}),
	}).Create(&spendDayModel{UserID: userID, Day: day, Currency: currency, Deposits: dep, Wagers: wag, Payouts: pay}).Error
	if err != nil {
		return fmt.Errorf("rg: record spend: %w", err)
	}
	return nil
}

func (r *GormRepository) SumSpendSince(ctx context.Context, userID int64, currency string, since time.Time) (string, string, string, error) {
	var row struct {
		Deposits decimal.Decimal
		Wagers   decimal.Decimal
		Payouts  decimal.Decimal
	}
	if err := r.db.WithContext(ctx).Model(&spendDayModel{}).
		Select("COALESCE(SUM(deposits),0) AS deposits, COALESCE(SUM(wagers),0) AS wagers, COALESCE(SUM(payouts),0) AS payouts").
		Where("user_id = ? AND currency = ? AND day >= ?", userID, currency, since.UTC().Truncate(24*time.Hour)).
		Scan(&row).Error; err != nil {
		return "", "", "", fmt.Errorf("rg: sum spend: %w", err)
	}
	return row.Deposits.String(), row.Wagers.String(), row.Payouts.String(), nil
}

func (r *GormRepository) AppendOutbox(ctx context.Context, topic, key string, payload map[string]interface{}) error {
	m := outboxModel{
		ID: uuid.New(), Topic: topic, EventKey: key, Payload: payload, CreatedAt: time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		return fmt.Errorf("rg: append outbox: %w", err)
	}
	return nil
}

func (r *GormRepository) ListPendingOutbox(ctx context.Context, limit int) ([]OutboxEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var models []outboxModel
	if err := r.db.WithContext(ctx).Where("published_at IS NULL").
		Order("created_at ASC").Limit(limit).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("rg: list outbox: %w", err)
	}
	out := make([]OutboxEvent, 0, len(models))
	for _, m := range models {
		out = append(out, OutboxEvent{ID: m.ID, Topic: m.Topic, EventKey: m.EventKey, Payload: m.Payload, CreatedAt: m.CreatedAt})
	}
	return out, nil
}

func (r *GormRepository) MarkOutboxPublished(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).Model(&outboxModel{}).
		Where("id = ?", id).Update("published_at", now).Error; err != nil {
		return fmt.Errorf("rg: mark outbox published: %w", err)
	}
	return nil
}

func toPendingDomain(models []pendingChangeModel) []*domain.PendingChange {
	out := make([]*domain.PendingChange, 0, len(models))
	for _, m := range models {
		out = append(out, &domain.PendingChange{
			ID: m.ID, UserID: m.UserID, LimitType: domain.LimitType(m.LimitType),
			OldValue: m.OldValue, NewValue: m.NewValue, Status: domain.ChangeStatus(m.Status),
			RequestedAt: m.RequestedAt, EffectiveAt: m.EffectiveAt,
		})
	}
	return out
}

func toExclusionDomain(m *exclusionModel) *domain.Exclusion {
	return &domain.Exclusion{
		ID: m.ID, UserID: m.UserID, Type: domain.ExclusionType(m.Type),
		Status: domain.ExclusionStatus(m.Status), Until: m.Until, Permanent: m.Permanent,
		CreatedBy: m.CreatedBy, CreatedAt: m.CreatedAt, RevokedAt: m.RevokedAt, RevokedBy: m.RevokedBy,
	}
}
