package service

import (
	"context"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/opus-casino/notification/internal/repository"
)

// TestProcessEvent_SupportedEventAliases checks that every event type the
// platform publishes is routed to a real handler, including the plural/singular
// spellings that different producers emit.
//
// Routing is asserted through the handler's own input validation: every handler
// rejects an event with no user_id before touching the repository, so this test
// needs neither a database nor a network. An earlier version of this test
// asserted that ProcessEvent returned no error for a service built on a nil
// *pgxpool.Pool, which could never pass: the handlers do persist the
// notification, so they correctly report the missing database. That assertion
// also could not tell a routed event apart from an unrecognised one, since both
// branches were expected to return nil.
func TestProcessEvent_SupportedEventAliases(t *testing.T) {
	repo := repository.NewNotificationRepository(nil, nil)
	svc := NewNotificationService(repo, zap.NewNop())

	aliases := []string{
		"bet.settled",
		"bets.settled",
		"payment.deposit_confirmed",
		"payments.deposit_confirmed",
		"payment.withdrawal_processed",
		"payments.withdrawal_processed",
		"kyc.status_changed",
		"users.kyc_verified",
		"bonus.activated",
	}

	for _, alias := range aliases {
		t.Run(alias, func(t *testing.T) {
			err := svc.ProcessEvent(context.Background(), alias, map[string]string{})
			if err == nil {
				t.Fatalf("event type %q was not routed to a handler", alias)
			}
			if !strings.Contains(err.Error(), "user_id") {
				t.Fatalf("event type %q did not reach its handler: got %v", alias, err)
			}
		})
	}
}

func TestProcessEvent_UnknownEventTypeIsNoOp(t *testing.T) {
	repo := repository.NewNotificationRepository(nil, nil)
	svc := NewNotificationService(repo, zap.NewNop())

	if err := svc.ProcessEvent(context.Background(), "some.topic.nobody.handles", nil); err != nil {
		t.Fatalf("unknown event type must be ignored, got: %v", err)
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
