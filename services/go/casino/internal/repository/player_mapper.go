package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// playerMappingRecord maps opaque provider player_id → internal user_id.
type playerMappingRecord struct {
	ID           int64  `gorm:"primaryKey;autoIncrement"`
	ProviderName string `gorm:"not null;index"`
	PlayerID     string `gorm:"not null;index"`
	UserID       int64  `gorm:"not null;index"`
}

func (playerMappingRecord) TableName() string { return "casino_player_mappings" }

// PlayerMapper implements service.PlayerMapper using PostgreSQL.
type PlayerMapper struct {
	db *gorm.DB
}

// NewPlayerMapper creates a new player mapper backed by PostgreSQL.
func NewPlayerMapper(db *gorm.DB) *PlayerMapper {
	return &PlayerMapper{db: db}
}

// GetInternalUserID resolves a provider-side player_id to our internal user_id.
func (m *PlayerMapper) GetInternalUserID(ctx context.Context, providerName, playerID string) (int64, error) {
	var rec playerMappingRecord
	err := m.db.WithContext(ctx).
		Where("provider_name = ? AND player_id = ?", providerName, playerID).
		First(&rec).Error
	if err != nil {
		return 0, fmt.Errorf("player mapper: resolve %s/%s: %w", providerName, playerID, err)
	}
	return rec.UserID, nil
}

// GetPlayerID resolves our user_id to a provider-side opaque player_id.
// Creates a new mapping if none exists.
func (m *PlayerMapper) GetPlayerID(ctx context.Context, providerName string, userID int64) (string, error) {
	var rec playerMappingRecord
	err := m.db.WithContext(ctx).
		Where("provider_name = ? AND user_id = ?", providerName, userID).
		First(&rec).Error
	if err == nil {
		return rec.PlayerID, nil
	}

	// Auto-create a deterministic player_id on first access
	playerID := fmt.Sprintf("p%d_%s", userID, providerName[:3])
	newRec := playerMappingRecord{
		ProviderName: providerName,
		PlayerID:     playerID,
		UserID:       userID,
	}
	if err := m.db.WithContext(ctx).Create(&newRec).Error; err != nil {
		return "", fmt.Errorf("player mapper: create %s/%d: %w", providerName, userID, err)
	}

	return playerID, nil
}
