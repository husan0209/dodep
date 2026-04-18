package repository

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/platform/services/payment-service/internal/domain"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// withdrawalRepo implements WithdrawalRepository using GORM
type withdrawalRepo struct {
	db *gorm.DB
}

// NewWithdrawalRepository creates a new withdrawal repository
func NewWithdrawalRepository(db *gorm.DB) WithdrawalRepository {
	return &withdrawalRepo{db: db}
}

// Create creates a new withdrawal record
func (r *withdrawalRepo) Create(ctx context.Context, withdrawal *domain.Withdrawal) error {
	if withdrawal.UUID == uuid.Nil {
		withdrawal.UUID = uuid.New()
	}
	withdrawal.CreatedAt = time.Now()
	withdrawal.UpdatedAt = time.Now()

	result := r.db.WithContext(ctx).Create(withdrawal)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return domain.NewDetailedError(domain.ErrWithdrawalAlreadyProcessed, domain.ErrCodeWithdrawalAlreadyProcessed)
		}
		return fmt.Errorf("create withdrawal: %w", result.Error)
	}
	return nil
}

// GetByID retrieves a withdrawal by internal ID
func (r *withdrawalRepo) GetByID(ctx context.Context, id int64) (*domain.Withdrawal, error) {
	var withdrawal domain.Withdrawal
	result := r.db.WithContext(ctx).First(&withdrawal, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, domain.ErrorWithdrawalNotFound(id)
		}
		return nil, fmt.Errorf("get withdrawal by id: %w", result.Error)
	}
	return &withdrawal, nil
}

// GetByWithdrawalID retrieves a withdrawal by NOWPayments withdrawal_id
func (r *withdrawalRepo) GetByWithdrawalID(ctx context.Context, withdrawalID string) (*domain.Withdrawal, error) {
	var withdrawal domain.Withdrawal
	result := r.db.WithContext(ctx).Where("withdrawal_id = ?", withdrawalID).First(&withdrawal)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, domain.NewDetailedError(domain.ErrWithdrawalNotFound, domain.ErrCodeWithdrawalNotFound)
		}
		return nil, fmt.Errorf("get withdrawal by withdrawal_id: %w", result.Error)
	}
	return &withdrawal, nil
}

// GetByIDempotencyKey retrieves a withdrawal by idempotency key
func (r *withdrawalRepo) GetByIDempotencyKey(ctx context.Context, key string) (*domain.Withdrawal, error) {
	var withdrawal domain.Withdrawal
	result := r.db.WithContext(ctx).Where("idempotency_key = ?", key).First(&withdrawal)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil // Not found is not an error for idempotency check
		}
		return nil, fmt.Errorf("get withdrawal by idempotency_key: %w", result.Error)
	}
	return &withdrawal, nil
}

// GetByUUID retrieves a withdrawal by UUID
func (r *withdrawalRepo) GetByUUID(ctx context.Context, uuidStr string) (*domain.Withdrawal, error) {
	parsedUUID, err := uuid.Parse(uuidStr)
	if err != nil {
		return nil, fmt.Errorf("parse uuid: %w", err)
	}

	var withdrawal domain.Withdrawal
	result := r.db.WithContext(ctx).Where("uuid = ?", parsedUUID).First(&withdrawal)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, domain.NewDetailedError(domain.ErrWithdrawalNotFound, domain.ErrCodeWithdrawalNotFound)
		}
		return nil, fmt.Errorf("get withdrawal by uuid: %w", result.Error)
	}
	return &withdrawal, nil
}

// UpdateStatus updates withdrawal status with optimistic locking
func (r *withdrawalRepo) UpdateStatus(ctx context.Context, id int64, fromStatus, toStatus domain.WithdrawalStatus) error {
	result := r.db.WithContext(ctx).
		Model(&domain.Withdrawal{}).
		Where("id = ? AND status = ?", id, fromStatus).
		Update("status", toStatus)

	if result.Error != nil {
		return fmt.Errorf("update withdrawal status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return domain.ErrorInvalidStatusTransition(string(fromStatus), string(toStatus))
	}

	// Update updated_at
	r.db.WithContext(ctx).Model(&domain.Withdrawal{}).
		Where("id = ?", id).
		Update("updated_at", time.Now())

	return nil
}

// ListByUserID lists withdrawals for a user with pagination
func (r *withdrawalRepo) ListByUserID(ctx context.Context, userID int64, filter ListFilter) (*ListResult[domain.Withdrawal], error) {
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
		cursor, err := decodeWithdrawalCursor(filter.Cursor)
		if err == nil {
			query = query.Where("created_at < ? OR (created_at = ? AND id < ?)", cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
		}
	}

	var withdrawals []domain.Withdrawal
	result := query.Limit(filter.Limit + 1).Find(&withdrawals)
	if result.Error != nil {
		return nil, fmt.Errorf("list withdrawals: %w", result.Error)
	}

	hasMore := len(withdrawals) > filter.Limit
	if hasMore {
		withdrawals = withdrawals[:filter.Limit]
	}

	var nextCursor string
	if hasMore && len(withdrawals) > 0 {
		nextCursor = encodeWithdrawalCursor(withdrawals[len(withdrawals)-1])
	}

	return &ListResult[domain.Withdrawal]{
		Items:      withdrawals,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// CountByUserIDStatus counts withdrawals by user and status
func (r *withdrawalRepo) CountByUserIDStatus(ctx context.Context, userID int64, statuses []domain.WithdrawalStatus) (int64, error) {
	var count int64
	result := r.db.WithContext(ctx).
		Model(&domain.Withdrawal{}).
		Where("user_id = ? AND status IN ?", userID, statuses).
		Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("count withdrawals: %w", result.Error)
	}
	return count, nil
}

// Cursor types for pagination
type withdrawalCursor struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func decodeWithdrawalCursor(encoded string) (*withdrawalCursor, error) {
	data, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	var cursor withdrawalCursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return nil, err
	}

	return &cursor, nil
}

func encodeWithdrawalCursor(withdrawal domain.Withdrawal) string {
	cursor := withdrawalCursor{
		ID:        withdrawal.ID,
		CreatedAt: withdrawal.CreatedAt,
	}

	data, _ := json.Marshal(cursor)
	return base64.URLEncoding.EncodeToString(data)
}
