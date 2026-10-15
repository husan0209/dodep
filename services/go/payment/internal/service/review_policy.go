package service

import (
	"os"
	"strings"

	"github.com/shopspring/decimal"
)

// ReviewPolicy decides whether a withdrawal request enters the manual-review
// queue instead of paying out immediately. Requests at or above the fiat
// threshold are parked as pending_review with the funds locked in the wallet;
// a finance operator then approves (the dispatcher executes the payout) or
// rejects (the dispatcher returns the funds).
//
// Threshold is expressed in the lock currency (always USD — the wallet lock
// is taken in USD). A non-positive threshold disables manual review entirely
// and restores the instant-payout behavior.
type ReviewPolicy struct {
	Threshold decimal.Decimal
}

// DefaultReviewPolicy returns the disabled policy (instant payouts).
func DefaultReviewPolicy() *ReviewPolicy {
	return &ReviewPolicy{Threshold: decimal.Zero}
}

// ReviewPolicyFromEnv builds the policy from WITHDRAWAL_MANUAL_REVIEW_THRESHOLD_USD.
//
// Unset, empty or unparseable means disabled (fail open to the historical
// behavior — an operator misconfiguration must not silently strand funds in
// the review queue). A negative value is treated the same as zero.
func ReviewPolicyFromEnv() *ReviewPolicy {
	raw := strings.TrimSpace(os.Getenv("WITHDRAWAL_MANUAL_REVIEW_THRESHOLD_USD"))
	if raw == "" {
		return DefaultReviewPolicy()
	}
	threshold, err := decimal.NewFromString(raw)
	if err != nil {
		return DefaultReviewPolicy()
	}
	if threshold.IsNegative() {
		return DefaultReviewPolicy()
	}
	return &ReviewPolicy{Threshold: threshold}
}

// Enabled reports whether manual review is active.
func (p *ReviewPolicy) Enabled() bool {
	return p != nil && p.Threshold.IsPositive()
}

// RequiresManualReview reports whether a fiat amount must be reviewed by an
// operator before the payout is created.
func (p *ReviewPolicy) RequiresManualReview(fiatAmount decimal.Decimal) bool {
	if !p.Enabled() {
		return false
	}
	return fiatAmount.GreaterThanOrEqual(p.Threshold)
}

// ThresholdString renders the threshold for logs (never for comparisons).
func (p *ReviewPolicy) ThresholdString() string {
	if p == nil {
		return "0"
	}
	return p.Threshold.String()
}

// WithdrawalOption configures optional WithdrawalService behavior.
type WithdrawalOption func(*WithdrawalService)

// WithReviewPolicy enables the manual-review threshold policy.
func WithReviewPolicy(policy *ReviewPolicy) WithdrawalOption {
	return func(s *WithdrawalService) {
		if policy != nil {
			s.review = policy
		}
	}
}
