package fraud

import (
	"context"
	"strconv"

	"github.com/opus-casino/affiliate/internal/domain"
)

// evalSelfReferral flags attributions where the referred user IS the
// affiliate owner. The service rejects such binds outright; this rule covers
// direct inserts and backfills. Critical: a self-referral ring pays the
// affiliate out of the platform's own margin with zero acquisition value.
func evalSelfReferral(_ context.Context, _ SignalStore, _ Config, in Input) (*Finding, error) {
	if in.AffiliateUserID == 0 || in.ReferredUserID == 0 {
		return nil, nil
	}
	if in.AffiliateUserID != in.ReferredUserID {
		return nil, nil
	}
	return &Finding{
		RuleID:   RuleSelfReferral,
		Severity: domain.FraudSeverityCritical,
		Details: map[string]string{
			"rule_id":          RuleSelfReferral,
			"affiliate_id":     in.AffiliateID.String(),
			"referred_user_id": strconv.FormatInt(in.ReferredUserID, 10),
		},
	}, nil
}

// evalKYCIdentityMatch flags attributions where the referred player shares
// the affiliate owner's verified KYC identity (same document, second
// account). High severity: same-household members may legitimately share a
// surname but never a verified document hash; every hit needs review.
func evalKYCIdentityMatch(_ context.Context, _ SignalStore, _ Config, in Input) (*Finding, error) {
	if in.AffiliateKYCIdentityHash == "" || in.ReferredKYCIdentityHash == "" {
		return nil, nil
	}
	if in.AffiliateKYCIdentityHash != in.ReferredKYCIdentityHash {
		return nil, nil
	}
	return &Finding{
		RuleID:   RuleKYCIdentityMatch,
		Severity: domain.FraudSeverityHigh,
		Details: map[string]string{
			"rule_id":          RuleKYCIdentityMatch,
			"affiliate_id":     in.AffiliateID.String(),
			"referred_user_id": strconv.FormatInt(in.ReferredUserID, 10),
		},
	}, nil
}

// evalPaymentInstrumentMatch flags attributions where the referred player's
// payment instrument equals the affiliate's payout destination. Critical:
// the same card/wallet/account funding play and collecting RevShare is the
// textbook self-dealing loop — commission becomes a rebate on own losses.
// Open flags gate payouts, so this finding blocks withdrawals until review.
func evalPaymentInstrumentMatch(_ context.Context, _ SignalStore, _ Config, in Input) (*Finding, error) {
	if in.AffiliatePayoutInstrumentHash == "" || in.ReferredPaymentInstrumentHash == "" {
		return nil, nil
	}
	if in.AffiliatePayoutInstrumentHash != in.ReferredPaymentInstrumentHash {
		return nil, nil
	}
	return &Finding{
		RuleID:   RulePaymentInstrumentMatch,
		Severity: domain.FraudSeverityCritical,
		Details: map[string]string{
			"rule_id":          RulePaymentInstrumentMatch,
			"affiliate_id":     in.AffiliateID.String(),
			"referred_user_id": strconv.FormatInt(in.ReferredUserID, 10),
		},
	}, nil
}
