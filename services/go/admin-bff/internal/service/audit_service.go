package service

import (
	"context"
	"encoding/json"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/opus-casino/admin-bff/internal/models"
)

type AuditService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewAuditService(db *gorm.DB, logger *zap.Logger) *AuditService {
	return &AuditService{db: db, logger: logger}
}

// Log writes an audit record asynchronously.
func (s *AuditService) Log(ctx context.Context, adminID *int64, action, resourceType, resourceID string, details map[string]interface{}, ip, ua string) {
	var detailsJSON []byte
	if details != nil {
		var err error
		detailsJSON, err = json.Marshal(details)
		if err != nil {
			s.logger.Error("audit marshal failed", zap.Error(err))
			detailsJSON = nil
		}
	}
	entry := models.AuditLog{
		AdminID:      adminID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   &resourceID,
		Details:      detailsJSON,
		IPAddress:    &ip,
		UserAgent:    &ua,
		CreatedAt:    time.Now(),
	}
	// Fire-and-forget with context detach
	go func(e models.AuditLog) {
		if err := s.db.WithContext(context.Background()).Create(&e).Error; err != nil {
			s.logger.Error("audit log insert failed", zap.Error(err))
		}
	}(entry)
}

// List returns paginated audit logs.
func (s *AuditService) List(ctx context.Context, adminID *int64, resourceType, resourceID string, page, pageSize int) ([]models.AuditLog, int64, error) {
	q := s.db.WithContext(ctx).Model(&models.AuditLog{})
	if adminID != nil {
		q = q.Where("admin_id = ?", *adminID)
	}
	if resourceType != "" {
		q = q.Where("resource_type = ?", resourceType)
	}
	if resourceID != "" {
		q = q.Where("resource_id = ?", resourceID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.AuditLog
	offset := (page - 1) * pageSize
	err := q.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&items).Error
	return items, total, err
}
