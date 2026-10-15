package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// NotificationRepository handles data persistence for notifications.
//
// SQL tables (no migration exists yet — DDL contract for the table owner):
//
//	CREATE TABLE notifications (
//	    id TEXT PRIMARY KEY,
//	    user_id BIGINT NOT NULL,
//	    channel TEXT NOT NULL DEFAULT '',
//	    type TEXT NOT NULL DEFAULT '',
//	    priority TEXT NOT NULL DEFAULT 'normal',
//	    subject TEXT NOT NULL DEFAULT '',
//	    message TEXT NOT NULL DEFAULT '',
//	    data JSONB NOT NULL DEFAULT '{}',
//	    is_read BOOLEAN NOT NULL DEFAULT FALSE,
//	    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
//	    read_at TIMESTAMPTZ,
//	    sent_at TIMESTAMPTZ,
//	    status TEXT NOT NULL DEFAULT 'pending',
//	    error_message TEXT NOT NULL DEFAULT '',
//	    reference_id TEXT NOT NULL DEFAULT '',
//	    metadata JSONB NOT NULL DEFAULT '{}'
//	);
//	CREATE INDEX idx_notifications_user ON notifications(user_id, created_at DESC);
//	CREATE INDEX idx_notifications_pending ON notifications(status) WHERE status = 'pending';
//
//	CREATE TABLE notification_settings (
//	    user_id BIGINT PRIMARY KEY,
//	    email_enabled BOOLEAN NOT NULL DEFAULT TRUE,
//	    sms_enabled BOOLEAN NOT NULL DEFAULT FALSE,
//	    push_enabled BOOLEAN NOT NULL DEFAULT TRUE,
//	    in_app_enabled BOOLEAN NOT NULL DEFAULT TRUE,
//	    type_preferences JSONB NOT NULL DEFAULT '{}',
//	    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
//	);
//
// All queries are parameterized (NEVER string concatenation, per security rules).
// Money never appears here (no amounts stored); user_id always comes from JWT
// at the handler layer, never from request bodies.
type NotificationRepository struct {
	db    dbConn
	redis *redis.Client
}

// dbConn is the subset of *pgxpool.Pool used by this repository.
// *pgxpool.Pool satisfies it, and tests supply scripted fakes.
type dbConn interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row
}

var errRedisUnavailable = errors.New("redis client is not initialized")
var errDatabaseUnavailable = errors.New("database client is not initialized")

// NewNotificationRepository creates a new notification repository
func NewNotificationRepository(db *pgxpool.Pool, rdb *redis.Client) *NotificationRepository {
	var conn dbConn
	if db != nil {
		conn = db
	}
	return &NotificationRepository{
		db:    conn,
		redis: rdb,
	}
}

// Notification represents a notification
type Notification struct {
	ID           string            `json:"id"`
	UserID       uint64            `json:"user_id"`
	Channel      string            `json:"channel"`
	Type         string            `json:"type"`
	Priority     string            `json:"priority"`
	Subject      string            `json:"subject"`
	Message      string            `json:"message"`
	Data         map[string]string `json:"data"`
	IsRead       bool              `json:"is_read"`
	CreatedAt    time.Time         `json:"created_at"`
	ReadAt       *time.Time        `json:"read_at"`
	SentAt       *time.Time        `json:"sent_at"`
	Status       string            `json:"status"`
	ErrorMessage string            `json:"error_message"`
	ReferenceID  string            `json:"reference_id"`
	Metadata     map[string]string `json:"metadata"`
}

// NotificationSettings represents user's notification settings
type NotificationSettings struct {
	UserID          uint64                        `json:"user_id"`
	EmailEnabled    bool                          `json:"email_enabled"`
	SMSEnabled      bool                          `json:"sms_enabled"`
	PushEnabled     bool                          `json:"push_enabled"`
	InAppEnabled    bool                          `json:"in_app_enabled"`
	TypePreferences map[string]ChannelPreferences `json:"type_preferences"`
	UpdatedAt       time.Time                     `json:"updated_at"`
}

// ChannelPreferences represents channel preferences for a notification type
type ChannelPreferences struct {
	EmailEnabled bool `json:"email_enabled"`
	SMSEnabled   bool `json:"sms_enabled"`
	PushEnabled  bool `json:"push_enabled"`
	InAppEnabled bool `json:"in_app_enabled"`
}

// normalizePage applies default/max page sizes and clamps negative offsets.
func normalizePage(limit, offset int32) (int32, int32) {
	const defaultLimit int32 = 20
	const maxLimit int32 = 100
	l := limit
	switch {
	case l <= 0:
		l = defaultLimit
	case l > maxLimit:
		l = maxLimit
	}
	o := offset
	if o < 0 {
		o = 0
	}
	return l, o
}

// defaultSettings returns channel defaults for a user without stored settings.
func defaultSettings(userID uint64) *NotificationSettings {
	return &NotificationSettings{
		UserID:          userID,
		EmailEnabled:    true,
		SMSEnabled:      false,
		PushEnabled:     true,
		InAppEnabled:    true,
		TypePreferences: make(map[string]ChannelPreferences),
		UpdatedAt:       time.Now(),
	}
}

// mustJSON marshals a string map for JSONB columns; never returns nil bytes.
func mustJSON(m map[string]string) []byte {
	if m == nil {
		return []byte("{}")
	}
	b, err := json.Marshal(m)
	if err != nil {
		return []byte("{}")
	}
	return b
}

// scanNotification scans the full notifications column list in canonical order.
func scanNotification(row pgx.Row) (*Notification, error) {
	var n Notification
	var userID int64
	var dataRaw, metaRaw []byte
	if err := row.Scan(
		&n.ID, &userID, &n.Channel, &n.Type, &n.Priority,
		&n.Subject, &n.Message, &dataRaw,
		&n.IsRead, &n.CreatedAt, &n.ReadAt, &n.SentAt,
		&n.Status, &n.ErrorMessage, &n.ReferenceID, &metaRaw,
	); err != nil {
		return nil, err
	}
	n.UserID = uint64(userID) // #nosec G115 -- user_id is BIGINT, always >= 0
	n.Data = map[string]string{}
	if len(dataRaw) > 0 {
		_ = json.Unmarshal(dataRaw, &n.Data)
	}
	n.Metadata = map[string]string{}
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &n.Metadata)
	}
	return &n, nil
}

const notificationColumns = `id, user_id, channel, type, priority, subject, message, data,
		is_read, created_at, read_at, sent_at, status, error_message, reference_id, metadata`

// CreateNotification creates a new notification
func (r *NotificationRepository) CreateNotification(ctx context.Context, notif *Notification) error {
	if r.db == nil {
		return errDatabaseUnavailable
	}
	if notif == nil || notif.ID == "" {
		return errors.New("notification id is required")
	}
	if notif.Status == "" {
		notif.Status = "pending"
	}
	if notif.Priority == "" {
		notif.Priority = "normal"
	}
	if notif.CreatedAt.IsZero() {
		notif.CreatedAt = time.Now()
	}
	_, err := r.db.Exec(ctx, `INSERT INTO notifications
		(id, user_id, channel, type, priority, subject, message, data,
		 is_read, created_at, sent_at, status, error_message, reference_id, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		notif.ID, int64(notif.UserID), notif.Channel, notif.Type, notif.Priority,
		notif.Subject, notif.Message, mustJSON(notif.Data),
		notif.IsRead, notif.CreatedAt, notif.SentAt,
		notif.Status, notif.ErrorMessage, notif.ReferenceID, mustJSON(notif.Metadata),
	)
	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	return nil
}

// GetNotification returns a notification by ID.
// Returns (nil, nil) when the row does not exist.
func (r *NotificationRepository) GetNotification(ctx context.Context, id string) (*Notification, error) {
	if r.db == nil {
		return nil, errDatabaseUnavailable
	}
	row := r.db.QueryRow(ctx, `SELECT `+notificationColumns+`
		FROM notifications WHERE id = $1`, id)
	n, err := scanNotification(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get notification %q: %w", id, err)
	}
	return n, nil
}

// GetUserNotifications returns user's notifications with pagination
func (r *NotificationRepository) GetUserNotifications(ctx context.Context, userID uint64, typeFilter *string, isRead *bool, dateFrom, dateTo *time.Time, limit, offset int32) ([]Notification, int64, error) {
	if r.db == nil {
		return nil, 0, errDatabaseUnavailable
	}
	l, o := normalizePage(limit, offset)

	where := `WHERE user_id = $1`
	args := []any{int64(userID)}
	if typeFilter != nil {
		args = append(args, *typeFilter)
		where += ` AND type = $` + strconv.Itoa(len(args))
	}
	if isRead != nil {
		args = append(args, *isRead)
		where += ` AND is_read = $` + strconv.Itoa(len(args))
	}
	if dateFrom != nil {
		args = append(args, *dateFrom)
		where += ` AND created_at >= $` + strconv.Itoa(len(args))
	}
	if dateTo != nil {
		args = append(args, *dateTo)
		where += ` AND created_at <= $` + strconv.Itoa(len(args))
	}

	var total int64
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count user notifications: %w", err)
	}

	args = append(args, l, o)
	rows, err := r.db.Query(ctx, `SELECT `+notificationColumns+`
		FROM notifications `+where+`
		ORDER BY created_at DESC LIMIT $`+strconv.Itoa(len(args)-1)+
		` OFFSET $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list user notifications: %w", err)
	}
	defer rows.Close()

	var out []Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan user notification: %w", err)
		}
		out = append(out, *n)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate user notifications: %w", err)
	}
	if out == nil {
		out = []Notification{}
	}
	return out, total, nil
}

// GetUnreadCount returns count of unread notifications for a user
func (r *NotificationRepository) GetUnreadCount(ctx context.Context, userID uint64) (int32, error) {
	if r.db == nil {
		return 0, errDatabaseUnavailable
	}
	var total int64
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = FALSE`,
		int64(userID)).Scan(&total); err != nil {
		return 0, fmt.Errorf("get unread count: %w", err)
	}
	return int32(total), nil // #nosec G115 -- count is bounded by page sizes in practice
}

// MarkAsRead marks a notification as read (idempotent: already-read rows are
// left untouched so a repeated call never fails).
func (r *NotificationRepository) MarkAsRead(ctx context.Context, id string, userID uint64) error {
	if r.db == nil {
		return errDatabaseUnavailable
	}
	_, err := r.db.Exec(ctx, `UPDATE notifications
		SET is_read = TRUE, read_at = NOW()
		WHERE id = $1 AND user_id = $2 AND is_read = FALSE`, id, int64(userID))
	if err != nil {
		return fmt.Errorf("mark notification as read: %w", err)
	}
	return nil
}

// MarkAllAsRead marks all user notifications as read, optionally filtered by
// type. Returns the number of rows actually flipped to read.
func (r *NotificationRepository) MarkAllAsRead(ctx context.Context, userID uint64, typeFilter *string) (int32, error) {
	if r.db == nil {
		return 0, errDatabaseUnavailable
	}
	query := `UPDATE notifications
		SET is_read = TRUE, read_at = NOW()
		WHERE user_id = $1 AND is_read = FALSE`
	args := []any{int64(userID)}
	if typeFilter != nil {
		args = append(args, *typeFilter)
		query += ` AND type = $2`
	}
	tag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("mark all notifications as read: %w", err)
	}
	return int32(tag.RowsAffected()), nil // #nosec G115 -- affected rows fit int32 here
}

// DeleteNotification deletes a notification owned by the user
func (r *NotificationRepository) DeleteNotification(ctx context.Context, id string, userID uint64) error {
	if r.db == nil {
		return errDatabaseUnavailable
	}
	_, err := r.db.Exec(ctx,
		`DELETE FROM notifications WHERE id = $1 AND user_id = $2`, id, int64(userID))
	if err != nil {
		return fmt.Errorf("delete notification: %w", err)
	}
	return nil
}

// UpdateNotificationStatus updates notification delivery status
func (r *NotificationRepository) UpdateNotificationStatus(ctx context.Context, id string, status string, errorMessage string) error {
	if r.db == nil {
		return errDatabaseUnavailable
	}
	_, err := r.db.Exec(ctx, `UPDATE notifications
		SET status = $2, error_message = $3 WHERE id = $1`, id, status, errorMessage)
	if err != nil {
		return fmt.Errorf("update notification status: %w", err)
	}
	return nil
}

// GetNotificationSettings returns user's notification settings.
// Users without a stored row get channel defaults (no error).
func (r *NotificationRepository) GetNotificationSettings(ctx context.Context, userID uint64) (*NotificationSettings, error) {
	if r.db == nil {
		return nil, errDatabaseUnavailable
	}
	var s NotificationSettings
	var storedID int64
	var prefsRaw []byte
	err := r.db.QueryRow(ctx, `SELECT user_id, email_enabled, sms_enabled,
			push_enabled, in_app_enabled, type_preferences, updated_at
		FROM notification_settings WHERE user_id = $1`, int64(userID)).Scan(
		&storedID, &s.EmailEnabled, &s.SMSEnabled,
		&s.PushEnabled, &s.InAppEnabled, &prefsRaw, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return defaultSettings(userID), nil
		}
		return nil, fmt.Errorf("get notification settings: %w", err)
	}
	s.UserID = uint64(storedID) // #nosec G115 -- user_id is BIGINT, always >= 0
	s.TypePreferences = map[string]ChannelPreferences{}
	if len(prefsRaw) > 0 {
		_ = json.Unmarshal(prefsRaw, &s.TypePreferences)
	}
	return &s, nil
}

// UpdateNotificationSettings upserts user's notification settings
func (r *NotificationRepository) UpdateNotificationSettings(ctx context.Context, settings *NotificationSettings) error {
	if r.db == nil {
		return errDatabaseUnavailable
	}
	if settings == nil {
		return errors.New("settings are required")
	}
	prefs := settings.TypePreferences
	if prefs == nil {
		prefs = map[string]ChannelPreferences{}
	}
	prefsRaw, err := json.Marshal(prefs)
	if err != nil {
		return fmt.Errorf("marshal type preferences: %w", err)
	}
	_, err = r.db.Exec(ctx, `INSERT INTO notification_settings
		(user_id, email_enabled, sms_enabled, push_enabled, in_app_enabled,
		 type_preferences, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			email_enabled = EXCLUDED.email_enabled,
			sms_enabled = EXCLUDED.sms_enabled,
			push_enabled = EXCLUDED.push_enabled,
			in_app_enabled = EXCLUDED.in_app_enabled,
			type_preferences = EXCLUDED.type_preferences,
			updated_at = NOW()`,
		int64(settings.UserID), settings.EmailEnabled, settings.SMSEnabled,
		settings.PushEnabled, settings.InAppEnabled, prefsRaw,
	)
	if err != nil {
		return fmt.Errorf("upsert notification settings: %w", err)
	}
	return nil
}

// GetPendingNotifications returns pending notifications to be sent
func (r *NotificationRepository) GetPendingNotifications(ctx context.Context, limit int32) ([]Notification, error) {
	if r.db == nil {
		return nil, errDatabaseUnavailable
	}
	l, _ := normalizePage(limit, 0)
	rows, err := r.db.Query(ctx, `SELECT `+notificationColumns+`
		FROM notifications WHERE status = 'pending'
		ORDER BY created_at ASC LIMIT $1`, l)
	if err != nil {
		return nil, fmt.Errorf("list pending notifications: %w", err)
	}
	defer rows.Close()

	out := []Notification{}
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, fmt.Errorf("scan pending notification: %w", err)
		}
		out = append(out, *n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending notifications: %w", err)
	}
	return out, nil
}

// NotificationCacheKey returns the Redis key for a cached notification.
func NotificationCacheKey(id string) string {
	return "notification:" + id
}

// CacheNotification caches notification in Redis
func (r *NotificationRepository) CacheNotification(ctx context.Context, notif *Notification, ttl time.Duration) error {
	if r.redis == nil {
		return errRedisUnavailable
	}

	payload, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("marshal notification for cache: %w", err)
	}

	return r.redis.Set(ctx, NotificationCacheKey(notif.ID), payload, ttl).Err()
}

// GetCachedNotification retrieves cached notification from Redis.
// Returns (nil, nil) on cache miss.
func (r *NotificationRepository) GetCachedNotification(ctx context.Context, id string) (*Notification, error) {
	if r.redis == nil {
		return nil, errRedisUnavailable
	}

	payload, err := r.redis.Get(ctx, NotificationCacheKey(id)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get cached notification %q: %w", id, err)
	}

	var notif Notification
	if err := json.Unmarshal(payload, &notif); err != nil {
		return nil, fmt.Errorf("unmarshal cached notification %q: %w", id, err)
	}

	return &notif, nil
}

// InvalidateNotificationCache invalidates cached notification
func (r *NotificationRepository) InvalidateNotificationCache(ctx context.Context, id string) error {
	if r.redis == nil {
		return errRedisUnavailable
	}

	return r.redis.Del(ctx, NotificationCacheKey(id)).Err()
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

// DecrementUnreadCount decrements unread count in Redis.
// Called (best-effort) by MarkAsRead — the decrement fix is preserved here:
// a read receipt must lower the counter, never raise or reset it.
func (r *NotificationRepository) DecrementUnreadCount(ctx context.Context, userID uint64) error {
	if r.redis == nil {
		return errRedisUnavailable
	}

	key := r.GetUserUnreadKey(userID)
	return r.redis.Decr(ctx, key).Err()
}

// GetUnreadCountFromCache gets unread count from Redis.
// Returns 0 when the key does not exist yet.
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
		return 0, fmt.Errorf("get unread count from cache: %w", err)
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
