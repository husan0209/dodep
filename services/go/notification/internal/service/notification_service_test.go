package service

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"

	"github.com/opus-casino/notification/internal/repository"
)

func TestProcessEvent_SupportedEventAliases(t *testing.T) {
	repo := repository.NewNotificationRepository(nil, nil)
	svc := NewNotificationService(repo, zap.NewNop())

	// Every handler validates the payload before it touches the database and
	// reports a missing user_id as errUserIDMissing. That error is therefore the
	// observable that proves an alias was routed to its handler: an event type
	// that is not wired up falls through to the default branch and returns nil.
	//
	// The happy path cannot be asserted here. SendNotification persists through
	// the *pgxpool.Pool held by the repository, and this is a unit test with no
	// database, so any payload carrying a user_id ends in
	// "database client is not initialized". Asserting on that error would only
	// test the repository guard, not the routing.
	testCases := []struct {
		name      string
		eventType string
	}{
		{name: "bets.settled", eventType: "bets.settled"},
		{name: "payments.deposit_confirmed", eventType: "payments.deposit_confirmed"},
		{name: "payments.withdrawal_processed", eventType: "payments.withdrawal_processed"},
		{name: "users.kyc_verified", eventType: "users.kyc_verified"},
		{name: "bonus.activated", eventType: "bonus.activated"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.ProcessEvent(context.Background(), tc.eventType, map[string]string{})

			if !errors.Is(err, errUserIDMissing) {
				t.Fatalf("expected %q to be routed to its handler, got error: %v", tc.eventType, err)
			}
		})
	}
}

func TestProcessEvent_UnknownEventTypeIsIgnored(t *testing.T) {
	repo := repository.NewNotificationRepository(nil, nil)
	svc := NewNotificationService(repo, zap.NewNop())

	if err := svc.ProcessEvent(context.Background(), "totally.unknown", map[string]string{"user_id": "42"}); err != nil {
		t.Fatalf("expected an unknown event type to be ignored, got: %v", err)
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
