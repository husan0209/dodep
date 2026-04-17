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
