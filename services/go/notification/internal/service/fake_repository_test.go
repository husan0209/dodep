package service

import (
	"context"
	"time"

	"github.com/opus-casino/notification/internal/repository"
)

// fakeRepository is an in-memory Repository for service tests.
//
// *repository.NotificationRepository is concrete and talks to pgx/redis, so
// passing one built from nil clients made every call fail with "database
// client is not initialized" - the service could not be tested at all without
// a live PostgreSQL and Redis.
type fakeRepository struct {
	created      []*repository.Notification
	settings     *repository.NotificationSettings
	settingsErr  error
	createdErr   error
	unreadCount  int32
	statusUpdate []string
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{created: make([]*repository.Notification, 0)}
}

func (f *fakeRepository) CreateNotification(_ context.Context, notif *repository.Notification) error {
	if f.createdErr != nil {
		return f.createdErr
	}
	f.created = append(f.created, notif)
	return nil
}

func (f *fakeRepository) GetNotification(_ context.Context, id string) (*repository.Notification, error) {
	for _, notif := range f.created {
		if notif.ID == id {
			return notif, nil
		}
	}
	return nil, nil
}

func (f *fakeRepository) GetUserNotifications(
	_ context.Context,
	userID uint64,
	_ *string,
	_ *bool,
	_, _ *time.Time,
	_, _ int32,
) ([]repository.Notification, int64, error) {
	matches := make([]repository.Notification, 0, len(f.created))
	for _, notif := range f.created {
		if notif.UserID == userID {
			matches = append(matches, *notif)
		}
	}
	return matches, int64(len(matches)), nil
}

func (f *fakeRepository) GetUnreadCount(context.Context, uint64) (int32, error) {
	return f.unreadCount, nil
}

func (f *fakeRepository) MarkAsRead(_ context.Context, id string, _ uint64) error {
	for _, notif := range f.created {
		if notif.ID == id {
			notif.IsRead = true
		}
	}
	return nil
}

func (f *fakeRepository) MarkAllAsRead(_ context.Context, userID uint64, _ *string) (int32, error) {
	var updated int32
	for _, notif := range f.created {
		if notif.UserID == userID && !notif.IsRead {
			notif.IsRead = true
			updated++
		}
	}
	return updated, nil
}

func (f *fakeRepository) DeleteNotification(_ context.Context, id string, _ uint64) error {
	for i, notif := range f.created {
		if notif.ID == id {
			f.created = append(f.created[:i], f.created[i+1:]...)
			break
		}
	}
	return nil
}

func (f *fakeRepository) UpdateNotificationStatus(_ context.Context, id, status, errorMessage string) error {
	f.statusUpdate = append(f.statusUpdate, id+"="+status)
	for _, notif := range f.created {
		if notif.ID == id {
			notif.Status = status
			notif.ErrorMessage = errorMessage
		}
	}
	return nil
}

func (f *fakeRepository) GetNotificationSettings(context.Context, uint64) (*repository.NotificationSettings, error) {
	return f.settings, f.settingsErr
}

func (f *fakeRepository) UpdateNotificationSettings(_ context.Context, settings *repository.NotificationSettings) error {
	f.settings = settings
	return nil
}

func (f *fakeRepository) IncrementUnreadCount(context.Context, uint64) error {
	f.unreadCount++
	return nil
}

func (f *fakeRepository) DecrementUnreadCount(context.Context, uint64) error {
	if f.unreadCount > 0 {
		f.unreadCount--
	}
	return nil
}

func (f *fakeRepository) SetUnreadCount(_ context.Context, _ uint64, count int32) error {
	f.unreadCount = count
	return nil
}