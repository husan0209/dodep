package repository

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opus-casino/payment/internal/domain"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// paymentRepo implements PaymentRepository using GORM
type paymentRepo struct {
	db *gorm.DB
}

// NewPaymentRepository creates a new payment repository
func NewPaymentRepository(db *gorm.DB) PaymentRepository {
	return &paymentRepo{db: db}
}

// Create creates a new payment record
func (r *paymentRepo) Create(ctx context.Context, payment *domain.Payment) error {
	if payment.UUID == uuid.Nil {
		payment.UUID = uuid.New()
	}
	payment.CreatedAt = time.Now()
	payment.UpdatedAt = time.Now()

	result := r.db.WithContext(ctx).Create(payment)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return domain.NewDetailedError(domain.ErrPaymentAlreadyProcessed, domain.ErrCodePaymentAlreadyProcessed)
		}
		return fmt.Errorf("create payment: %w", result.Error)
	}
	return nil
}

// GetByID retrieves a payment by internal ID
func (r *paymentRepo) GetByID(ctx context.Context, id int64) (*domain.Payment, error) {
	var payment domain.Payment
	result := r.db.WithContext(ctx).First(&payment, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, domain.ErrorPaymentNotFound(id)
		}
		return nil, fmt.Errorf("get payment by id: %w", result.Error)
	}
	return &payment, nil
}

// GetByPaymentID retrieves a payment by NOWPayments payment_id
func (r *paymentRepo) GetByPaymentID(ctx context.Context, paymentID string) (*domain.Payment, error) {
	var payment domain.Payment
	result := r.db.WithContext(ctx).Where("payment_id = ?", paymentID).First(&payment)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, domain.ErrorPaymentNotFoundByPaymentID(paymentID)
		}
		return nil, fmt.Errorf("get payment by payment_id: %w", result.Error)
	}
	return &payment, nil
}

// GetByIDempotencyKey retrieves a payment by idempotency key
func (r *paymentRepo) GetByIDempotencyKey(ctx context.Context, key string) (*domain.Payment, error) {
	var payment domain.Payment
	result := r.db.WithContext(ctx).Where("idempotency_key = ?", key).First(&payment)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil // Not found is not an error for idempotency check
		}
		return nil, fmt.Errorf("get payment by idempotency_key: %w", result.Error)
	}
	return &payment, nil
}

// GetByUUID retrieves a payment by UUID
func (r *paymentRepo) GetByUUID(ctx context.Context, uuidStr string) (*domain.Payment, error) {
	parsedUUID, err := uuid.Parse(uuidStr)
	if err != nil {
		return nil, fmt.Errorf("parse uuid: %w", err)
	}

	var payment domain.Payment
	result := r.db.WithContext(ctx).Where("uuid = ?", parsedUUID).First(&payment)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, domain.NewDetailedError(domain.ErrPaymentNotFound, domain.ErrCodePaymentNotFound)
		}
		return nil, fmt.Errorf("get payment by uuid: %w", result.Error)
	}
	return &payment, nil
}

// UpdateStatus updates payment status with optimistic locking
func (r *paymentRepo) UpdateStatus(ctx context.Context, id int64, fromStatus, toStatus domain.PaymentStatus) error {
	result := r.db.WithContext(ctx).
		Model(&domain.Payment{}).
		Where("id = ? AND status = ?", id, fromStatus).
		Update("status", toStatus)

	if result.Error != nil {
		return fmt.Errorf("update payment status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return domain.ErrorInvalidStatusTransition(string(fromStatus), string(toStatus))
	}

	// Update updated_at
	r.db.WithContext(ctx).Model(&domain.Payment{}).
		Where("id = ?", id).
		Update("updated_at", time.Now())

	return nil
}

// UpdateActualAmount updates the actual received amount
func (r *paymentRepo) UpdateActualAmount(ctx context.Context, id int64, actualAmount decimal.Decimal) error {
	result := r.db.WithContext(ctx).
		Model(&domain.Payment{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"actual_amount": actualAmount,
			"updated_at":    time.Now(),
		})

	if result.Error != nil {
		return fmt.Errorf("update actual amount: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return domain.ErrorPaymentNotFound(id)
	}

	return nil
}

// ListByUserID lists payments for a user with pagination
func (r *paymentRepo) ListByUserID(ctx context.Context, userID int64, filter ListFilter) (*ListResult[domain.Payment], error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	query := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC")

	// Apply status filter
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}

	// Apply cursor pagination
	if filter.Cursor != "" {
		cursor, err := decodePaymentCursor(filter.Cursor)
		if err == nil {
			query = query.Where("created_at < ? OR (created_at = ? AND id < ?)", cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
		}
	}

	var payments []domain.Payment
	result := query.Limit(filter.Limit + 1).Find(&payments)
	if result.Error != nil {
		return nil, fmt.Errorf("list payments: %w", result.Error)
	}

	hasMore := len(payments) > filter.Limit
	if hasMore {
		payments = payments[:filter.Limit]
	}

	var nextCursor string
	if hasMore && len(payments) > 0 {
		nextCursor = encodePaymentCursor(payments[len(payments)-1])
	}

	return &ListResult[domain.Payment]{
		Items:      payments,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// CountByUserIDStatus counts payments by user and status
func (r *paymentRepo) CountByUserIDStatus(ctx context.Context, userID int64, statuses []domain.PaymentStatus) (int64, error) {
	var count int64
	result := r.db.WithContext(ctx).
		Model(&domain.Payment{}).
		Where("user_id = ? AND status IN ?", userID, statuses).
		Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("count payments: %w", result.Error)
	}
	return count, nil
}

// Cursor types for pagination
type paymentCursor struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func decodePaymentCursor(encoded string) (*paymentCursor, error) {
	data, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	var cursor paymentCursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return nil, err
	}

	return &cursor, nil
}

func encodePaymentCursor(payment domain.Payment) string {
	cursor := paymentCursor{
		ID:        payment.ID,
		CreatedAt: payment.CreatedAt,
	}

	data, _ := json.Marshal(cursor)
	return base64.URLEncoding.EncodeToString(data)
}
