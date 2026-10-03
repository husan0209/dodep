package service

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/opus-casino/notification/internal/repository"
)

func TestEventAliases_FoldOntoCanonicalType(t *testing.T) {
	testCases := []struct {
		name      string
		eventType string
		expected  string
	}{
		{name: "legacy singular bet", eventType: "bet.settled", expected: eventBetSettled},
		{name: "topic bet", eventType: "bets.settled", expected: eventBetSettled},
		{name: "legacy singular deposit", eventType: "payment.deposit_confirmed", expected: eventDepositConfirmed},
		{name: "topic deposit", eventType: "payments.deposit_confirmed", expected: eventDepositConfirmed},
		{name: "legacy singular withdrawal", eventType: "payment.withdrawal_processed", expected: eventWithdrawalProcessed},
		{name: "topic withdrawal", eventType: "payments.withdrawal_processed", expected: eventWithdrawalProcessed},
		{name: "bonus", eventType: "bonus.activated", expected: eventBonusActivated},
		{name: "legacy kyc", eventType: "kyc.status_changed", expected: eventKYCStatusChanged},
		{name: "topic kyc", eventType: "users.kyc_verified", expected: eventKYCStatusChanged},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			canonical, handled := eventAliases[tc.eventType]
			if !handled {
				t.Fatalf("event type %q is not registered as a supported alias", tc.eventType)
			}
			if canonical != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, canonical)
			}
		})
	}
}

// Every canonical event type must be reachable from ProcessEvent: the switch in
// ProcessEvent dispatches on the canonical value, so an alias without a matching
// case would silently drop notifications.
func TestEventAliases_AllCanonicalTypesAreDispatched(t *testing.T) {
	dispatched := map[string]bool{
		eventBetSettled:          true,
		eventDepositConfirmed:    true,
		eventWithdrawalProcessed: true,
		eventBonusActivated:      true,
		eventKYCStatusChanged:    true,
	}

	for alias, canonical := range eventAliases {
		if !dispatched[canonical] {
			t.Errorf("alias %q resolves to %q, which ProcessEvent does not dispatch", alias, canonical)
		}
	}
}

func TestProcessEvent_UnknownTypeIsIgnored(t *testing.T) {
	// No repository is wired up: an event we do not own must be dropped before
	// any persistence is attempted, so this must succeed without a database.
	svc := NewNotificationService(repository.NewNotificationRepository(nil, nil), zap.NewNop())

	if err := svc.ProcessEvent(context.Background(), "wallet.something_new", map[string]string{"user_id": "42"}); err != nil {
		t.Fatalf("expected no error for an unhandled event, got: %v", err)
	}
}

func TestProcessEvent_RejectsPayloadWithoutUserID(t *testing.T) {
	svc := NewNotificationService(repository.NewNotificationRepository(nil, nil), zap.NewNop())

	err := svc.ProcessEvent(context.Background(), "bets.settled", map[string]string{"bet_id": "bet-1"})
	if err == nil {
		t.Fatal("expected an error when user_id is missing from the payload")
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
