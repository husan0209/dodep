package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestCalculateCommission_UsesNGRAndClampsNegativeCarryoverForMVP(t *testing.T) {
	tests := []struct {
		name     string
		ngr      string
		rate     string
		expected string
	}{
		{name: "positive ngr", ngr: "100.00", rate: "0.50", expected: "50"},
		{name: "zero ngr", ngr: "0", rate: "0.50", expected: "0"},
		{name: "negative ngr", ngr: "-10.00", rate: "0.50", expected: "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateCommission(decimal.RequireFromString(tt.ngr), decimal.RequireFromString(tt.rate))
			if !got.Equal(decimal.RequireFromString(tt.expected)) {
				t.Fatalf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestAffiliateStatus_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name     string
		from     AffiliateStatus
		to       AffiliateStatus
		expected bool
	}{
		{name: "pending to active", from: AffiliateStatusPendingReview, to: AffiliateStatusActive, expected: true},
		{name: "pending to rejected", from: AffiliateStatusPendingReview, to: AffiliateStatusRejected, expected: true},
		{name: "active to suspended", from: AffiliateStatusActive, to: AffiliateStatusSuspended, expected: true},
		{name: "suspended to active", from: AffiliateStatusSuspended, to: AffiliateStatusActive, expected: true},
		{name: "active to closed", from: AffiliateStatusActive, to: AffiliateStatusClosed, expected: true},
		{name: "rejected to active", from: AffiliateStatusRejected, to: AffiliateStatusActive, expected: false},
		{name: "closed to active", from: AffiliateStatusClosed, to: AffiliateStatusActive, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.from.CanTransitionTo(tt.to); got != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestAffiliateProfile_CanRequestPayout(t *testing.T) {
	profile := AffiliateProfile{
		ID:              uuid.New(),
		UserID:          42,
		Status:          AffiliateStatusActive,
		MinPayoutAmount: decimal.RequireFromString("100"),
		Currency:        "USD",
		KYCRequired:     true,
	}

	eligible := PayoutEligibility{
		AvailableAmount: decimal.RequireFromString("150"),
		KYCApproved:     true,
		HasOpenFraud:    false,
		RequestedAt:     time.Now(),
	}

	if err := profile.ValidatePayoutEligibility(eligible); err != nil {
		t.Fatalf("expected eligible payout, got error: %v", err)
	}

	ineligible := []struct {
		name string
		in   PayoutEligibility
		err  error
	}{
		{
			name: "below min payout",
			in: PayoutEligibility{
				AvailableAmount: decimal.RequireFromString("10"),
				KYCApproved:     true,
				RequestedAt:     time.Now(),
			},
			err: ErrMinPayoutNotReached,
		},
		{
			name: "kyc required",
			in: PayoutEligibility{
				AvailableAmount: decimal.RequireFromString("150"),
				KYCApproved:     false,
				RequestedAt:     time.Now(),
			},
			err: ErrAffiliateKYCRequired,
		},
		{
			name: "fraud blocked",
			in: PayoutEligibility{
				AvailableAmount: decimal.RequireFromString("150"),
				KYCApproved:     true,
				HasOpenFraud:    true,
				RequestedAt:     time.Now(),
			},
			err: ErrAffiliateFraudBlocked,
		},
	}

	for _, tt := range ineligible {
		t.Run(tt.name, func(t *testing.T) {
			if err := profile.ValidatePayoutEligibility(tt.in); err == nil || err != tt.err {
				t.Fatalf("expected %v, got %v", tt.err, err)
			}
		})
	}
}
