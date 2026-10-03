package service

import (
	"context"
	"time"

	"github.com/opus-casino/notification/internal/repository"
)

// stubRepository is an in-memory NotificationRepository used by the service
// tests. It keeps the assertions about business behaviour (event fan-out,
// aliasing, unread counters) independent of a live Postgres/Redis.
//
// Only the methods the tests exercise return meaningful values; the rest are
// stubs so the fake satisfies the interface.
type stubRepository struct {
	created []*repository.Notification
}

var _ NotificationRepository = (*stubRepository)(nil)

func (s *stubRepository) CreateNotification(_ context.Context, notif *repository.Notification) error {
	s.created = append(s.created, notif)
	return nil
}

func (s *stubRepository) UpdateNotificationStatus(_ context.Context, _, _, _ string) error {
	return nil
}

func (s *stubRepository) MarkAsRead(_ context.Context, _ string, _ uint64) error { return nil }

func (s *stubRepository) MarkAllAsRead(_ context.Context, _ uint64, _ *string) (int32, error) {
	return 0, nil
}

func (s *stubRepository) DeleteNotification(_ context.Context, _ string, _ uint64) error {
	return nil
}

func (s *stubRepository) GetNotification(_ context.Context, _ string) (*repository.Notification, error) {
	return nil, nil
}

func (s *stubRepository) GetUserNotifications(_ context.Context, _ uint64, _ *string, _ *bool, _, _ *time.Time, _, _ int32) ([]repository.Notification, int64, error) {
	return nil, 0, nil
}

func (s *stubRepository) GetUnreadCount(_ context.Context, _ uint64) (int32, error) { return 0, nil }

func (s *stubRepository) IncrementUnreadCount(_ context.Context, _ uint64) error { return nil }

func (s *stubRepository) DecrementUnreadCount(_ context.Context, _ uint64) error { return nil }

func (s *stubRepository) SetUnreadCount(_ context.Context, _ uint64, _ int32) error { return nil }

func (s *stubRepository) GetNotificationSettings(_ context.Context, _ uint64) (*repository.NotificationSettings, error) {
	return nil, nil
}

func (s *stubRepository) UpdateNotificationSettings(_ context.Context, _ *repository.NotificationSettings) error {
	return nil
}
