package service

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/opus-casino/notification/internal/repository"
)

func TestProcessEvent_SupportedEventAliases(t *testing.T) {
	repo := repository.NewNotificationRepository(nil, nil)
	svc := NewNotificationService(repo, zap.NewNop())

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
			// What this test is actually about is routing: every alias must reach
			// a real handler. An unrecognised event type takes the `default`
			// branch and returns nil, so a nil here means the alias is not wired
			// up -- which is the bug this guards.
			//
			// Asserting `err == nil` instead (as this test used to) can only pass
			// with a live database, because a routed handler persists the
			// notification. The repository here is built with a nil pool on
			// purpose, so a routed handler surfaces as an error and an unrouted
			// one as nil. See TestProcessEvent_UnknownTypeIsIgnored for the
			// contrast that makes this assertion meaningful.
			if err == nil {
				t.Fatalf("event %q was not routed to a handler: ProcessEvent returned nil, which is what an unrecognised event type returns", tc.eventType)
			}
		})
	}
}

// The other half of TestProcessEvent_SupportedEventAliases: without this, "the
// alias produced an error" could just mean "ProcessEvent always errors".
func TestProcessEvent_UnknownTypeIsIgnored(t *testing.T) {
	repo := repository.NewNotificationRepository(nil, nil)
	svc := NewNotificationService(repo, zap.NewNop())

	if err := svc.ProcessEvent(context.Background(), "something.unmapped", map[string]string{}); err != nil {
		t.Fatalf("unrecognised event type should be ignored, got: %v", err)
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
