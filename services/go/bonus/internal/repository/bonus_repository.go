package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opus-casino/bonus/internal/domain"
)

// BonusRepository abstracts persistence for bonuses.
// Service layer depends on this interface, never on *gorm.DB directly.
type BonusRepository interface {
	FindWelcome(ctx context.Context, userID int64) (*domain.Bonus, error)
	FindActive(ctx context.Context, userID int64) (*domain.Bonus, error)
	FindByID(ctx context.Context, userID int64, bonusID uuid.UUID) (*domain.Bonus, error)
	Create(ctx context.Context, bonus *domain.Bonus) error
	UpdateWagering(ctx context.Context, bonusID uuid.UUID, completed map[string]interface{}) error
	MarkExpired(ctx context.Context, bonusID uuid.UUID) error
	ListByUser(ctx context.Context, userID int64, limit, offset int) ([]*domain.Bonus, int64, error)
	Activate(ctx context.Context, bonusID uuid.UUID, userID int64) (*domain.Bonus, error)
	Cancel(ctx context.Context, bonusID uuid.UUID, userID int64) (*domain.Bonus, error)
}

type gormBonusRepository struct {
	db *gorm.DB
}

// NewBonusRepository creates a GORM-backed repository.
// Returns nil-safe wrapper: methods fail fast on nil DB.
func NewBonusRepository(db *gorm.DB) BonusRepository {
	return &gormBonusRepository{db: db}
}

func (r *gormBonusRepository) failFast() error {
	if r == nil || r.db == nil {
		return errors.New("bonus repository: nil database client")
	}
	return nil
}

func (r *gormBonusRepository) FindWelcome(ctx context.Context, userID int64) (*domain.Bonus, error) {
	if err := r.failFast(); err != nil {
		return nil, err
	}
	var bonus domain.Bonus
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND type = ? AND status IN ('pending','active','completed')", userID, domain.BonusTypeWelcome).
		First(&bonus).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &bonus, nil
}

func (r *gormBonusRepository) FindActive(ctx context.Context, userID int64) (*domain.Bonus, error) {
	if err := r.failFast(); err != nil {
		return nil, err
	}
	var bonus domain.Bonus
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = 'active'", userID).
		Order("created_at DESC").
		First(&bonus).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &bonus, nil
}

func (r *gormBonusRepository) FindByID(ctx context.Context, userID int64, bonusID uuid.UUID) (*domain.Bonus, error) {
	if err := r.failFast(); err != nil {
		return nil, err
	}
	var bonus domain.Bonus
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", bonusID, userID).
		First(&bonus).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &bonus, nil
}

func (r *gormBonusRepository) Create(ctx context.Context, bonus *domain.Bonus) error {
	if err := r.failFast(); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(bonus).Error
}

func (r *gormBonusRepository) UpdateWagering(ctx context.Context, bonusID uuid.UUID, completed map[string]interface{}) error {
	if err := r.failFast(); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&domain.Bonus{}).Where("id = ?", bonusID).Updates(completed).Error
}

func (r *gormBonusRepository) MarkExpired(ctx context.Context, bonusID uuid.UUID) error {
	if err := r.failFast(); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&domain.Bonus{}).
		Where("id = ?", bonusID).
		Updates(map[string]interface{}{"status": domain.BonusStatusExpired, "updated_at": time.Now()}).Error
}

func (r *gormBonusRepository) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]*domain.Bonus, int64, error) {
	if err := r.failFast(); err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	var total int64
	query := r.db.WithContext(ctx).Model(&domain.Bonus{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var bonuses []*domain.Bonus
	if err := query.Limit(limit).Offset(offset).Order("created_at DESC").Find(&bonuses).Error; err != nil {
		return nil, 0, err
	}
	return bonuses, total, nil
}

func (r *gormBonusRepository) Activate(ctx context.Context, bonusID uuid.UUID, userID int64) (*domain.Bonus, error) {
	if err := r.failFast(); err != nil {
		return nil, err
	}
	now := time.Now()
	res := r.db.WithContext(ctx).Model(&domain.Bonus{}).
		Where("id = ? AND user_id = ? AND status = ?", bonusID, userID, domain.BonusStatusPending).
		Updates(map[string]interface{}{"status": domain.BonusStatusActive, "activated_at": now, "updated_at": now})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, domain.ErrBonusNotFound
	}
	return r.FindByID(ctx, userID, bonusID)
}

func (r *gormBonusRepository) Cancel(ctx context.Context, bonusID uuid.UUID, userID int64) (*domain.Bonus, error) {
	if err := r.failFast(); err != nil {
		return nil, err
	}
	now := time.Now()
	res := r.db.WithContext(ctx).Model(&domain.Bonus{}).
		Where("id = ? AND user_id = ? AND status IN ('pending','active')", bonusID, userID).
		Updates(map[string]interface{}{"status": domain.BonusStatusCancelled, "cancelled_at": now, "updated_at": now})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, domain.ErrBonusNotFound
	}
	return r.FindByID(ctx, userID, bonusID)
}
