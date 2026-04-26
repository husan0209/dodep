package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/opus-casino/payment/internal/domain"
	"github.com/opus-casino/payment/internal/repository"
	"go.uber.org/zap"
)

const (
	// DefaultIdempotencyTTL is the default TTL for idempotency keys (24 hours)
	DefaultIdempotencyTTL = 24 * 60 * 60 // 24 hours in seconds
)

// IdempotencyService handles idempotency key management
// It provides a two-layer approach: DragonflyDB (fast path) + PostgreSQL UNIQUE constraint (safety net)
type IdempotencyService struct {
	idempotencyRepo repository.IdempotencyRepository
	paymentRepo     repository.PaymentRepository
	withdrawalRepo  repository.WithdrawalRepository
	logger          *zap.Logger
}

// NewIdempotencyService creates a new idempotency service
func NewIdempotencyService(
	idempotencyRepo repository.IdempotencyRepository,
	paymentRepo repository.PaymentRepository,
	withdrawalRepo repository.WithdrawalRepository,
	logger *zap.Logger,
) *IdempotencyService {
	return &IdempotencyService{
		idempotencyRepo: idempotencyRepo,
		paymentRepo:     paymentRepo,
		withdrawalRepo:  withdrawalRepo,
		logger:          logger,
	}
}

// IdempotencyResult represents a stored result of an idempotent operation
type IdempotencyResult struct {
	OperationType string          `json:"operation_type"` // "deposit", "withdrawal", "webhook_deposit", "webhook_withdrawal"
	ReferenceID   string          `json:"reference_id"`   // payment_id or withdrawal_id
	Status        string          `json:"status"`         // operation status
	Data          json.RawMessage `json:"data,omitempty"` // operation-specific data
	CreatedAt     time.Time       `json:"created_at"`
}

// CheckOrSetResult contains the result of CheckOrSet operation
type CheckOrSetResult struct {
	IsNew   bool                // true if key is new (operation should proceed)
	Result  *IdempotencyResult  // existing result if key was found
	Err     error               // error if any
}

// CheckOrSet checks if an idempotency key exists and sets it if not.
// Returns true if the key is new (operation should proceed), false if duplicate.
// This implements the fast path using DragonflyDB with atomic SetNX operation.
func (s *IdempotencyService) CheckOrSet(ctx context.Context, key string, operationType string) *CheckOrSetResult {
	// Try to get existing result from cache first
	existingData, found, err := s.idempotencyRepo.Get(ctx, key)
	if err != nil {
		s.logger.Warn("Failed to check idempotency cache",
			zap.String("key", key),
			zap.Error(err),
		)
		// Continue to try SetNX - don't fail on cache read error
	} else if found {
		// Key exists in cache - return existing result
		var result IdempotencyResult
		if unmarshalErr := json.Unmarshal(existingData, &result); unmarshalErr != nil {
			s.logger.Error("Failed to unmarshal cached idempotency result",
				zap.String("key", key),
				zap.Error(unmarshalErr),
			)
			// Treat as new operation but log the error
			return &CheckOrSetResult{IsNew: true, Err: nil}
		}
		
		s.logger.Info("Idempotency key found in cache",
			zap.String("key", key),
			zap.String("operation_type", result.OperationType),
			zap.String("reference_id", result.ReferenceID),
		)
		
		return &CheckOrSetResult{
			IsNew:  false,
			Result: &result,
		}
	}

	// Key not in cache - try atomic SetNX to reserve the key
	// This handles race conditions: only one request will succeed
	placeholder := IdempotencyResult{
		OperationType: operationType,
		Status:        "processing",
		CreatedAt:     time.Now(),
	}
	
	placeholderData, err := json.Marshal(placeholder)
	if err != nil {
		s.logger.Error("Failed to marshal idempotency placeholder",
			zap.String("key", key),
			zap.Error(err),
		)
		return &CheckOrSetResult{IsNew: false, Err: fmt.Errorf("marshal placeholder: %w", err)}
	}

	set, err := s.idempotencyRepo.SetNX(ctx, key, placeholderData, DefaultIdempotencyTTL)
	if err != nil {
		s.logger.Error("Failed to set idempotency key",
			zap.String("key", key),
			zap.Error(err),
		)
		return &CheckOrSetResult{IsNew: false, Err: fmt.Errorf("setnx idempotency key: %w", err)}
	}

	if !set {
		// Another request set the key concurrently - fetch the existing result
		existingData, found, err := s.idempotencyRepo.Get(ctx, key)
		if err != nil || !found {
			// Rare case: key was set but we can't read it
			// Fall back to database check
			return s.checkDatabaseFallback(ctx, key, operationType)
		}

		var result IdempotencyResult
		if unmarshalErr := json.Unmarshal(existingData, &result); unmarshalErr != nil {
			s.logger.Error("Failed to unmarshal concurrent idempotency result",
				zap.String("key", key),
				zap.Error(unmarshalErr),
			)
			return s.checkDatabaseFallback(ctx, key, operationType)
		}

		return &CheckOrSetResult{
			IsNew:  false,
			Result: &result,
		}
	}

	// Key was set successfully - operation should proceed
	s.logger.Info("Idempotency key reserved",
		zap.String("key", key),
		zap.String("operation_type", operationType),
	)

	return &CheckOrSetResult{IsNew: true}
}

// SetResult stores the result of an operation for later retrieval.
// This should be called after a successful operation completes.
func (s *IdempotencyService) SetResult(ctx context.Context, key string, result *IdempotencyResult) error {
	if result == nil {
		return errors.New("result cannot be nil")
	}

	result.CreatedAt = time.Now()
	
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}

	if err := s.idempotencyRepo.Set(ctx, key, data, DefaultIdempotencyTTL); err != nil {
		s.logger.Error("Failed to store idempotency result",
			zap.String("key", key),
			zap.Error(err),
		)
		return fmt.Errorf("set idempotency result: %w", err)
	}

	s.logger.Info("Idempotency result stored",
		zap.String("key", key),
		zap.String("operation_type", result.OperationType),
		zap.String("reference_id", result.ReferenceID),
	)

	return nil
}

// GetResult retrieves a previously stored result by key.
// Returns nil if the key doesn't exist or has expired.
func (s *IdempotencyService) GetResult(ctx context.Context, key string) (*IdempotencyResult, error) {
	data, found, err := s.idempotencyRepo.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("get idempotency key: %w", err)
	}

	if !found {
		return nil, nil
	}

	var result IdempotencyResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal result: %w", err)
	}

	return &result, nil
}

// Delete removes an idempotency key (used for cleanup or rollback)
func (s *IdempotencyService) Delete(ctx context.Context, key string) error {
	if err := s.idempotencyRepo.Delete(ctx, key); err != nil {
		return fmt.Errorf("delete idempotency key: %w", err)
	}
	return nil
}

// checkDatabaseFallback checks PostgreSQL for existing records when cache is inconsistent.
// This is the safety net for race conditions and cache failures.
func (s *IdempotencyService) checkDatabaseFallback(ctx context.Context, key string, operationType string) *CheckOrSetResult {
	s.logger.Info("Falling back to database for idempotency check",
		zap.String("key", key),
		zap.String("operation_type", operationType),
	)

	// Check payments table
	if payment, err := s.paymentRepo.GetByIDempotencyKey(ctx, key); err == nil && payment != nil {
		result := &IdempotencyResult{
			OperationType: "deposit",
			ReferenceID:   payment.PaymentID,
			Status:        string(payment.Status),
			CreatedAt:     payment.CreatedAt,
		}
		return &CheckOrSetResult{IsNew: false, Result: result}
	}

	// Check withdrawals table
	if withdrawal, err := s.withdrawalRepo.GetByIDempotencyKey(ctx, key); err == nil && withdrawal != nil {
		result := &IdempotencyResult{
			OperationType: "withdrawal",
			ReferenceID:   withdrawal.WithdrawalID,
			Status:        string(withdrawal.Status),
			CreatedAt:     withdrawal.CreatedAt,
		}
		return &CheckOrSetResult{IsNew: false, Result: result}
	}

	// Not found in database - treat as new
	// This is safe because PostgreSQL UNIQUE constraint will catch duplicates
	return &CheckOrSetResult{IsNew: true}
}

// HandleUniqueConstraintViolation handles PostgreSQL unique constraint violations.
// This should be called when a database insert fails due to a unique constraint on idempotency_key.
// It fetches the existing record and returns it as an idempotency result.
func (s *IdempotencyService) HandleUniqueConstraintViolation(ctx context.Context, key string, operationType string) (*IdempotencyResult, error) {
	s.logger.Info("Handling unique constraint violation",
		zap.String("key", key),
		zap.String("operation_type", operationType),
	)

	switch operationType {
	case "deposit", "webhook_deposit":
		payment, err := s.paymentRepo.GetByIDempotencyKey(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("get existing payment: %w", err)
		}
		if payment != nil {
			return &IdempotencyResult{
				OperationType: "deposit",
				ReferenceID:   payment.PaymentID,
				Status:        string(payment.Status),
				CreatedAt:     payment.CreatedAt,
			}, nil
		}

	case "withdrawal", "webhook_withdrawal":
		withdrawal, err := s.withdrawalRepo.GetByIDempotencyKey(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("get existing withdrawal: %w", err)
		}
		if withdrawal != nil {
			return &IdempotencyResult{
				OperationType: "withdrawal",
				ReferenceID:   withdrawal.WithdrawalID,
				Status:        string(withdrawal.Status),
				CreatedAt:     withdrawal.CreatedAt,
			}, nil
		}
	}

	return nil, domain.ErrPaymentNotFound
}

// IsDuplicateError checks if an error is a unique constraint violation
func IsDuplicateError(err error) bool {
	// PostgreSQL unique constraint violation error code
	// This is a simplified check - in production, use pgx or pq to check the error code
	return err != nil && (err.Error() != "" && 
		(containsString(err.Error(), "duplicate key") ||
		 containsString(err.Error(), "unique constraint") ||
		 containsString(err.Error(), "SQLSTATE 23505")))
}

// containsString checks if s contains substr (case-insensitive helper)
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := s[i+j]
			subc := substr[j]
			// Simple lowercase comparison
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if subc >= 'A' && subc <= 'Z' {
				subc += 32
			}
			if sc != subc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
