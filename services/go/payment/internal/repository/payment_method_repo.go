package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opus-casino/payment/internal/domain"
	"gorm.io/gorm"
)

// PaymentMethodRepository manages saved payout destinations.
type PaymentMethodRepository interface {
	Create(ctx context.Context, method *domain.SavedPaymentMethod) error
	GetByID(ctx context.Context, userID int64, id uuid.UUID) (*domain.SavedPaymentMethod, error)
	ListByUserID(ctx context.Context, userID int64, limit, offset int) ([]domain.SavedPaymentMethod, int64, error)
	Update(ctx context.Context, method *domain.SavedPaymentMethod) error
	Delete(ctx context.Context, userID int64, id uuid.UUID) error
	TouchLastUsed(ctx context.Context, userID int64, id uuid.UUID) error
}

type paymentMethodRepo struct {
	db *gorm.DB
}

// NewPaymentMethodRepository creates the repository.
func NewPaymentMethodRepository(db *gorm.DB) PaymentMethodRepository {
	return &paymentMethodRepo{db: db}
}

func (r *paymentMethodRepo) Create(ctx context.Context, method *domain.SavedPaymentMethod) error {
	if method.ID == uuid.Nil {
		method.ID = uuid.New()
	}
	now := time.Now()
	method.CreatedAt = now
	method.UpdatedAt = now
	if err := r.db.WithContext(ctx).Create(method).Error; err != nil {
		return fmt.Errorf("create payment method: %w", err)
	}
	return nil
}

func (r *paymentMethodRepo) GetByID(ctx context.Context, userID int64, id uuid.UUID) (*domain.SavedPaymentMethod, error) {
	var m domain.SavedPaymentMethod
	if err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ? AND is_active = TRUE", id, userID).
		First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get payment method: %w", err)
	}
	return &m, nil
}

func (r *paymentMethodRepo) ListByUserID(ctx context.Context, userID int64, limit, offset int) ([]domain.SavedPaymentMethod, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	q := r.db.WithContext(ctx).Model(&domain.SavedPaymentMethod{}).
		Where("user_id = ? AND is_active = TRUE", userID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count payment methods: %w", err)
	}
	var items []domain.SavedPaymentMethod
	if err := q.Order("is_default DESC, created_at DESC").
		Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list payment methods: %w", err)
	}
	if items == nil {
		items = []domain.SavedPaymentMethod{}
	}
	return items, total, nil
}

func (r *paymentMethodRepo) Update(ctx context.Context, method *domain.SavedPaymentMethod) error {
	method.UpdatedAt = time.Now()
	res := r.db.WithContext(ctx).
		Model(&domain.SavedPaymentMethod{}).
		Where("id = ? AND user_id = ?", method.ID, method.UserID).
		Updates(map[string]interface{}{
			"nickname":     method.Nickname,
			"is_default":   method.IsDefault,
			"is_active":    method.IsActive,
			"last_used_at": method.LastUsedAt,
			"updated_at":   method.UpdatedAt,
		})
	if res.Error != nil {
		return fmt.Errorf("update payment method: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrorPaymentNotFound(int64(0))
	}
	return nil
}

func (r *paymentMethodRepo) Delete(ctx context.Context, userID int64, id uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&domain.SavedPaymentMethod{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("is_active", false)
	if res.Error != nil {
		return fmt.Errorf("delete payment method: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrorPaymentNotFound(int64(0))
	}
	return nil
}

func (r *paymentMethodRepo) TouchLastUsed(ctx context.Context, userID int64, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&domain.SavedPaymentMethod{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]interface{}{
			"last_used_at": time.Now(),
			"updated_at":   time.Now(),
		}).Error
}
