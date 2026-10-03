package service

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"

	"github.com/opus-casino/notification/internal/repository"
)

// TestProcessEvent_SupportedEventAliases asserts that every accepted event-type
// spelling is actually routed to a handler.
//
// The repository is built without a database on purpose, so a *routed* event ends in
// repository.ErrDatabaseUnavailable while an *unrouted* one returns nil from the
// `default` branch of ProcessEvent. Asserting "no error" here (as this test used to)
// could only ever pass if every alias silently fell through to `default`, i.e. the
// exact bug the test exists to catch.
func TestProcessEvent_SupportedEventAliases(t *testing.T) {
	repo := repository.NewNotificationRepository(nil, nil)
	svc := NewNotificationService(repo, zap.NewNop())

	testCases := []struct {
		name      string
		eventType string
		data      map[string]string
	}{
		{
			name:      "bet.settled",
			eventType: "bet.settled",
			data: map[string]string{
				"user_id": "42",
				"bet_id":  "bet-1",
				"result":  "won",
			},
		},
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
			name:      "payment.deposit_confirmed",
			eventType: "payment.deposit_confirmed",
			data: map[string]string{
				"user_id":  "42",
				"amount":   "100.00",
				"currency": "USD",
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
			name:      "payment.withdrawal_processed",
			eventType: "payment.withdrawal_processed",
			data: map[string]string{
				"user_id":  "42",
				"amount":   "50.00",
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
			name:      "kyc.status_changed",
			eventType: "kyc.status_changed",
			data: map[string]string{
				"user_id": "42",
				"status":  "verified",
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
			if !errors.Is(err, repository.ErrDatabaseUnavailable) {
				t.Fatalf("event %q was not routed to a handler: got %v, want %v",
					tc.eventType, err, repository.ErrDatabaseUnavailable)
			}
		})
	}
}

// TestProcessEvent_UnknownEventTypeIsIgnored pins the other half of the routing
// contract: an event nobody handles must be dropped silently, not error.
func TestProcessEvent_UnknownEventTypeIsIgnored(t *testing.T) {
	repo := repository.NewNotificationRepository(nil, nil)
	svc := NewNotificationService(repo, zap.NewNop())

	if err := svc.ProcessEvent(context.Background(), "totally.unknown.event", map[string]string{
		"user_id": "42",
	}); err != nil {
		t.Fatalf("unknown event type must be ignored, got: %v", err)
	}
}

// TestProcessEvent_MissingUserIDIsRejected proves the handlers really validate their
// payload instead of forwarding an empty user_id to the database.
func TestProcessEvent_MissingUserIDIsRejected(t *testing.T) {
	repo := repository.NewNotificationRepository(nil, nil)
	svc := NewNotificationService(repo, zap.NewNop())

	err := svc.ProcessEvent(context.Background(), "bets.settled", map[string]string{
		"bet_id": "bet-1",
		"result": "won",
	})
	if err == nil {
		t.Fatal("expected an error when user_id is absent")
	}
	if errors.Is(err, repository.ErrDatabaseUnavailable) {
		t.Fatalf("event without user_id must be rejected before persistence, got: %v", err)
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
