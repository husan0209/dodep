package service

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/opus-casino/notification/internal/repository"
)

// fakeRepository is an in-memory NotificationRepository for unit tests.
type fakeRepository struct{}

func (f *fakeRepository) CreateNotification(ctx context.Context, notif *repository.Notification) error {
	return nil
}

func (f *fakeRepository) GetNotification(ctx context.Context, id string) (*repository.Notification, error) {
	return &repository.Notification{ID: id}, nil
}

func (f *fakeRepository) GetUserNotifications(ctx context.Context, userID uint64, typeFilter *string, isRead *bool, dateFrom, dateTo *time.Time, limit, offset int32) ([]repository.Notification, int64, error) {
	return []repository.Notification{}, 0, nil
}

func (f *fakeRepository) GetUnreadCount(ctx context.Context, userID uint64) (int32, error) {
	return 0, nil
}

func (f *fakeRepository) MarkAsRead(ctx context.Context, id string, userID uint64) error {
	return nil
}

func (f *fakeRepository) MarkAllAsRead(ctx context.Context, userID uint64, typeFilter *string) (int32, error) {
	return 0, nil
}

func (f *fakeRepository) DeleteNotification(ctx context.Context, id string, userID uint64) error {
	return nil
}

func (f *fakeRepository) UpdateNotificationStatus(ctx context.Context, id string, status string, errorMessage string) error {
	return nil
}

func (f *fakeRepository) GetNotificationSettings(ctx context.Context, userID uint64) (*repository.NotificationSettings, error) {
	return &repository.NotificationSettings{
		UserID:       userID,
		EmailEnabled: true,
		SMSEnabled:   true,
		PushEnabled:  true,
		InAppEnabled: true,
		UpdatedAt:    time.Now(),
	}, nil
}

func (f *fakeRepository) UpdateNotificationSettings(ctx context.Context, settings *repository.NotificationSettings) error {
	return nil
}

func (f *fakeRepository) IncrementUnreadCount(ctx context.Context, userID uint64) error {
	return nil
}

func (f *fakeRepository) DecrementUnreadCount(ctx context.Context, userID uint64) error {
	return nil
}

func (f *fakeRepository) SetUnreadCount(ctx context.Context, userID uint64, count int32) error {
	return nil
}

func TestProcessEvent_SupportedEventAliases(t *testing.T) {
	svc := NewNotificationService(&fakeRepository{}, zap.NewNop())

	testCases := []struct {
		name      string
		eventType string
		data      map[string]string
	}{
		{
			name:      "bets.settled",
			eventType: "bets.settled",
			data: map[string]string{
				"user_id": "42",
				"bet_id":  "bet-1",
				"result":  "won",
			},
		},
		{
			name:      "payments.deposit_confirmed",
			eventType: "payments.deposit_confirmed",
			data: map[string]string{
				"user_id":  "42",
				"amount":   "100.00",
				"currency": "USD",
			},
		},
		{
			name:      "payments.withdrawal_processed",
			eventType: "payments.withdrawal_processed",
			data: map[string]string{
				"user_id":  "42",
				"amount":   "50.00",
				"currency": "USD",
			},
		},
		{
			name:      "users.kyc_verified",
			eventType: "users.kyc_verified",
			data: map[string]string{
				"user_id": "42",
				"status":  "verified",
			},
		},
		{
			name:      "bonus.activated",
			eventType: "bonus.activated",
			data: map[string]string{
				"user_id":    "42",
				"bonus_name": "welcome",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.ProcessEvent(context.Background(), tc.eventType, tc.data)
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
		})
	}
}

func TestGetUint64FromData(t *testing.T) {
	testCases := []struct {
		name     string
		data     map[string]string
		key      string
		expected uint64
	}{
		{
			name: "valid",
			data: map[string]string{
				"user_id": "123",
			},
			key:      "user_id",
			expected: 123,
		},
		{
			name: "invalid",
			data: map[string]string{
				"user_id": "abc",
			},
			key:      "user_id",
			expected: 0,
		},
		{
			name:     "missing",
			data:     map[string]string{},
			key:      "user_id",
			expected: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := getUint64FromData(tc.data, tc.key)
			if got != tc.expected {
				t.Fatalf("expected %d, got %d", tc.expected, got)
			}
		})
	}
}
