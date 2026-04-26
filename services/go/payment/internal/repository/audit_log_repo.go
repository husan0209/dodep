package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// auditLogRepo implements AuditLogRepository using GORM
type auditLogRepo struct {
	db *gorm.DB
	// In production, this would write to ClickHouse via Redpanda
}

// NewAuditLogRepository creates a new audit log repository
func NewAuditLogRepository(db *gorm.DB) AuditLogRepository {
	return &auditLogRepo{db: db}
}

// Create creates a new audit log entry
func (r *auditLogRepo) Create(ctx context.Context, log *AuditLog) error {
	// Generate ID if not set
	if log.ID == 0 {
		log.ID = time.Now().UnixNano()
	}
	
	// In production, this would:
	// 1. Publish to Redpanda topic payments.audit
	// 2. ClickHouse would consume and store the log
	// For now, we just return success
	return nil
}

// ListByUserID retrieves audit logs for a user
func (r *auditLogRepo) ListByUserID(ctx context.Context, userID int64, filter ListFilter) (*ListResult[AuditLog], error) {
	// Placeholder - in production, this would query ClickHouse
	return &ListResult[AuditLog]{
		Items:      []AuditLog{},
		NextCursor: "",
		HasMore:    false,
	}, nil
}

// ListByReference retrieves audit logs by reference
func (r *auditLogRepo) ListByReference(ctx context.Context, refType, refID string, filter ListFilter) (*ListResult[AuditLog], error) {
	// Placeholder - in production, this would query ClickHouse
	return &ListResult[AuditLog]{
		Items:      []AuditLog{},
		NextCursor: "",
		HasMore:    false,
	}, nil
}

// CreateAuditLog creates an audit log entry with standard fields
func CreateAuditLog(userID int64, operationType, referenceType, referenceID string) *AuditLog {
	return &AuditLog{
		UserID:        userID,
		OperationType: operationType,
		ReferenceType: referenceType,
		ReferenceID:   referenceID,
		CreatedAt:     time.Now(),
	}
}

// WithStatusChange adds status change information to the audit log
func (l *AuditLog) WithStatusChange(prevStatus, newStatus string) *AuditLog {
	l.PreviousStatus = prevStatus
	l.NewStatus = newStatus
	return l
}

// WithTraceInfo adds trace information to the audit log
func (l *AuditLog) WithTraceInfo(traceID, correlationID string) *AuditLog {
	l.TraceID = traceID
	l.CorrelationID = correlationID
	return l
}

// WithError adds error information to the audit log
func (l *AuditLog) WithError(code, message string) *AuditLog {
	l.ErrorCode = code
	l.ErrorMessage = message
	return l
}

// GenerateTraceID generates a new trace ID
func GenerateTraceID() string {
	return uuid.New().String()
}
