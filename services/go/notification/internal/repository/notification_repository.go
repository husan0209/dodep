package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// NotificationRepository handles data persistence for notifications
type NotificationRepository struct {
	db    *pgxpool.Pool
	redis *redis.Client
	log   *zap.Logger
}

var errRedisUnavailable = errors.New("redis client is not initialized")
var errDatabaseUnavailable = errors.New("database client is not initialized")

// NewNotificationRepository creates a new notification repository
func NewNotificationRepository(db *pgxpool.Pool, rdb *redis.Client) *NotificationRepository {
	return &NotificationRepository{
		db:    db,
		redis: rdb,
	}
}

// Notification represents a notification
type Notification struct {
	ID           string                 `json:"id"`
	UserID       uint64                 `json:"user_id"`
	Channel      string                 `json:"channel"`
	Type         string                 `json:"type"`
	Priority     string                 `json:"priority"`
	Subject      string                 `json:"subject"`
	Message      string                 `json:"message"`
	Data         map[string]string      `json:"data"`
	IsRead       bool                   `json:"is_read"`
	CreatedAt    time.Time              `json:"created_at"`
	ReadAt       *time.Time             `json:"read_at"`
	SentAt       *time.Time             `json:"sent_at"`
	Status       string                 `json:"status"`
	ErrorMessage string                 `json:"error_message"`
	ReferenceID  string                 `json:"reference_id"`
	Metadata     map[string]string      `json:"metadata"`
}

// NotificationSettings represents user's notification settings
type NotificationSettings struct {
	UserID              uint64                        `json:"user_id"`
	EmailEnabled        bool                          `json:"email_enabled"`
	SMSEnabled          bool                          `json:"sms_enabled"`
	PushEnabled         bool                          `json:"push_enabled"`
	InAppEnabled        bool                          `json:"in_app_enabled"`
	TypePreferences     map[string]ChannelPreferences `json:"type_preferences"`
	UpdatedAt           time.Time                     `json:"updated_at"`
}

// ChannelPreferences represents channel preferences for a notification type
type ChannelPreferences struct {
	EmailEnabled bool `json:"email_enabled"`
	SMSEnabled   bool `json:"sms_enabled"`
	PushEnabled  bool `json:"push_enabled"`
	InAppEnabled bool `json:"in_app_enabled"`
}

// CreateNotification creates a new notification
func (r *NotificationRepository) CreateNotification(ctx context.Context, notif *Notification) error {
	if r.db == nil {
		return errDatabaseUnavailable
	}
	return nil
}

// GetNotification returns a notification by ID
func (r *NotificationRepository) GetNotification(ctx context.Context, id string) (*Notification, error) {
	if r.db == nil {
		return nil, errDatabaseUnavailable
	}
	return nil, nil
}

// GetUserNotifications returns user's notifications with pagination
func (r *NotificationRepository) GetUserNotifications(ctx context.Context, userID uint64, typeFilter *string, isRead *bool, dateFrom, dateTo *time.Time, limit, offset int32) ([]Notification, int64, error) {
	if r.db == nil {
		return nil, 0, errDatabaseUnavailable
	}
	return []Notification{}, 0, nil
}

// GetUnreadCount returns count of unread notifications for a user
func (r *NotificationRepository) GetUnreadCount(ctx context.Context, userID uint64) (int32, error) {
	if r.db == nil {
		return 0, errDatabaseUnavailable
	}
	return 0, nil
}

// MarkAsRead marks a notification as read
func (r *NotificationRepository) MarkAsRead(ctx context.Context, id string, userID uint64) error {
	if r.db == nil {
		return errDatabaseUnavailable
	}
	return nil
}

// MarkAllAsRead marks all user notifications as read
func (r *NotificationRepository) MarkAllAsRead(ctx context.Context, userID uint64, typeFilter *string) (int32, error) {
	if r.db == nil {
		return 0, errDatabaseUnavailable
	}
	return 0, nil
}

// DeleteNotification deletes a notification
func (r *NotificationRepository) DeleteNotification(ctx context.Context, id string, userID uint64) error {
	if r.db == nil {
		return errDatabaseUnavailable
	}
	return nil
}

// UpdateNotificationStatus updates notification status
func (r *NotificationRepository) UpdateNotificationStatus(ctx context.Context, id string, status string, errorMessage string) error {
	if r.db == nil {
		return errDatabaseUnavailable
	}
	return nil
}

// GetNotificationSettings returns user's notification settings
func (r *NotificationRepository) GetNotificationSettings(ctx context.Context, userID uint64) (*NotificationSettings, error) {
	// TODO: Implement database query
	return &NotificationSettings{
		UserID:           userID,
		EmailEnabled:     true,
		SMSEnabled:       false,
		PushEnabled:      true,
		InAppEnabled:     true,
		TypePreferences:  make(map[string]ChannelPreferences),
		UpdatedAt:        time.Now(),
	}, nil
}

// UpdateNotificationSettings updates user's notification settings
func (r *NotificationRepository) UpdateNotificationSettings(ctx context.Context, settings *NotificationSettings) error {
	if r.db == nil {
		return errDatabaseUnavailable
	}
	return nil
}

// GetPendingNotifications returns pending notifications to be sent
func (r *NotificationRepository) GetPendingNotifications(ctx context.Context, limit int32) ([]Notification, error) {
	if r.db == nil {
		return nil, errDatabaseUnavailable
	}
	return []Notification{}, nil
}

// CacheNotification caches notification in Redis
func (r *NotificationRepository) CacheNotification(ctx context.Context, notif *Notification, ttl time.Duration) error {
	key := "notification:" + notif.ID
	if r.redis == nil {
		return errRedisUnavailable
	}

	payload, err := json.Marshal(notif)
	if err != nil {
		return err
	}

	return r.redis.Set(ctx, key, payload, ttl).Err()
}

// GetCachedNotification retrieves cached notification from Redis
func (r *NotificationRepository) GetCachedNotification(ctx context.Context, id string) (*Notification, error) {
	key := "notification:" + id
	if r.redis == nil {
		return nil, errRedisUnavailable
	}

	payload, err := r.redis.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var notif Notification
	if err := json.Unmarshal(payload, &notif); err != nil {
		return nil, err
	}

	return &notif, nil
}

// InvalidateNotificationCache invalidates cached notification
func (r *NotificationRepository) InvalidateNotificationCache(ctx context.Context, id string) error {
	key := "notification:" + id
	if r.redis == nil {
		return errRedisUnavailable
	}

	return r.redis.Del(ctx, key).Err()
}

// GetUserUnreadKey returns Redis key for user's unread count
func (r *NotificationRepository) GetUserUnreadKey(userID uint64) string {
	return "notification:unread:" + strconv.FormatUint(userID, 10)
}

// IncrementUnreadCount increments unread count in Redis
func (r *NotificationRepository) IncrementUnreadCount(ctx context.Context, userID uint64) error {
	if r.redis == nil {
		return errRedisUnavailable
	}

	key := r.GetUserUnreadKey(userID)
	return r.redis.Incr(ctx, key).Err()
}

// DecrementUnreadCount decrements unread count in Redis
func (r *NotificationRepository) DecrementUnreadCount(ctx context.Context, userID uint64) error {
	if r.redis == nil {
		return errRedisUnavailable
	}

	key := r.GetUserUnreadKey(userID)
	return r.redis.Decr(ctx, key).Err()
}

// GetUnreadCountFromCache gets unread count from Redis
func (r *NotificationRepository) GetUnreadCountFromCache(ctx context.Context, userID uint64) (int32, error) {
	if r.redis == nil {
		return 0, errRedisUnavailable
	}

	key := r.GetUserUnreadKey(userID)
	valStr, err := r.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var val int32
	_, scanErr := fmt.Sscanf(valStr, "%d", &val)
	return val, scanErr
}

// SetUnreadCount sets unread count in Redis
func (r *NotificationRepository) SetUnreadCount(ctx context.Context, userID uint64, count int32) error {
	if r.redis == nil {
		return errRedisUnavailable
	}

	key := r.GetUserUnreadKey(userID)
	return r.redis.Set(ctx, key, count, 0).Err()
}
