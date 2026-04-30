package repository

import (
	"context"
	"testing"
	"time"
)

func TestGetUserUnreadKey(t *testing.T) {
	repo := NewNotificationRepository(nil, nil)

	got := repo.GetUserUnreadKey(12345)
	expected := "notification:unread:12345"

	if got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}

func TestCacheMethods_ReturnErrorWhenRedisIsNil(t *testing.T) {
	repo := NewNotificationRepository(nil, nil)
	ctx := context.Background()

	notif := &Notification{
		ID:        "n-1",
		UserID:    1,
		Channel:   "in_app",
		Type:      "bet_settled",
		Subject:   "subject",
		Message:   "message",
		CreatedAt: time.Now(),
	}

	if err := repo.CacheNotification(ctx, notif, time.Minute); err == nil {
		t.Fatalf("expected error when redis is nil")
	}

	if _, err := repo.GetCachedNotification(ctx, "n-1"); err == nil {
		t.Fatalf("expected error when redis is nil")
	}

	if err := repo.InvalidateNotificationCache(ctx, "n-1"); err == nil {
		t.Fatalf("expected error when redis is nil")
	}
}

func TestUnreadCounterMethods_ReturnErrorWhenRedisIsNil(t *testing.T) {
	repo := NewNotificationRepository(nil, nil)
	ctx := context.Background()

	if err := repo.IncrementUnreadCount(ctx, 1); err == nil {
		t.Fatalf("expected error when redis is nil")
	}

	if err := repo.DecrementUnreadCount(ctx, 1); err == nil {
		t.Fatalf("expected error when redis is nil")
	}

	if _, err := repo.GetUnreadCountFromCache(ctx, 1); err == nil {
		t.Fatalf("expected error when redis is nil")
	}

	if err := repo.SetUnreadCount(ctx, 1, 10); err == nil {
		t.Fatalf("expected error when redis is nil")
	}
}

func TestDatabaseMethods_ReturnErrorWhenDatabaseIsNil(t *testing.T) {
	repo := NewNotificationRepository(nil, nil)
	ctx := context.Background()

	notif := &Notification{
		ID:        "n-1",
		UserID:    1,
		Channel:   "in_app",
		Type:      "bet_settled",
		Subject:   "subject",
		Message:   "message",
		CreatedAt: time.Now(),
	}

	if err := repo.CreateNotification(ctx, notif); err == nil {
		t.Fatalf("expected error when database is nil")
	}

	if _, err := repo.GetNotification(ctx, "n-1"); err == nil {
		t.Fatalf("expected error when database is nil")
	}

	if _, _, err := repo.GetUserNotifications(ctx, 1, nil, nil, nil, nil, 10, 0); err == nil {
		t.Fatalf("expected error when database is nil")
	}

	if _, err := repo.GetUnreadCount(ctx, 1); err == nil {
		t.Fatalf("expected error when database is nil")
	}

	if err := repo.MarkAsRead(ctx, "n-1", 1); err == nil {
		t.Fatalf("expected error when database is nil")
	}

	if _, err := repo.MarkAllAsRead(ctx, 1, nil); err == nil {
		t.Fatalf("expected error when database is nil")
	}

	if err := repo.DeleteNotification(ctx, "n-1", 1); err == nil {
		t.Fatalf("expected error when database is nil")
	}

	if err := repo.UpdateNotificationStatus(ctx, "n-1", "sent", ""); err == nil {
		t.Fatalf("expected error when database is nil")
	}

	settings := &NotificationSettings{
		UserID:      1,
		UpdatedAt:   time.Now(),
		TypePreferences: map[string]ChannelPreferences{},
	}
	if err := repo.UpdateNotificationSettings(ctx, settings); err == nil {
		t.Fatalf("expected error when database is nil")
	}

	if _, err := repo.GetPendingNotifications(ctx, 10); err == nil {
		t.Fatalf("expected error when database is nil")
	}
}
