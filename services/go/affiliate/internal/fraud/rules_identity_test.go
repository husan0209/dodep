package fraud

import (
	"context"
	"testing"

	"github.com/opus-casino/affiliate/internal/domain"
)

func identityInput() Input {
	in := cleanInput()
	in.AffiliateUserID = 42
	in.ReferredUserID = 77
	return in
}

func TestSelfReferralRule(t *testing.T) {
	in := identityInput()
	in.ReferredUserID = in.AffiliateUserID
	v, err := Evaluate(context.Background(), &stubStore{}, DefaultConfig(), in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v == nil || v.MaxSeverity != domain.FraudSeverityCritical {
		t.Fatalf("self-referral must be critical, got %+v", v)
	}
	found := false
	for _, f := range v.Findings {
		if f.RuleID == RuleSelfReferral {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected %q finding, got %+v", RuleSelfReferral, v.Findings)
	}

	distinct := identityInput()
	if v, _ := Evaluate(context.Background(), &stubStore{}, DefaultConfig(), distinct); v != nil {
		t.Fatalf("distinct users must not trigger self-referral, got %+v", v)
	}
}

func TestKYCIdentityMatchRule(t *testing.T) {
	in := identityInput()
	in.AffiliateKYCIdentityHash = "hmac-doc-1"
	in.ReferredKYCIdentityHash = "hmac-doc-1"
	v, err := Evaluate(context.Background(), &stubStore{}, DefaultConfig(), in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v == nil || v.MaxSeverity != domain.FraudSeverityHigh {
		t.Fatalf("shared KYC identity must be high, got %+v", v)
	}

	different := identityInput()
	different.AffiliateKYCIdentityHash = "hmac-doc-1"
	different.ReferredKYCIdentityHash = "hmac-doc-2"
	if v, _ := Evaluate(context.Background(), &stubStore{}, DefaultConfig(), different); v != nil {
		t.Fatalf("different KYC identities must be clean, got %+v", v)
	}

	// Missing signal on either side skips the rule (not a finding).
	half := identityInput()
	half.ReferredKYCIdentityHash = "hmac-doc-1"
	if v, _ := Evaluate(context.Background(), &stubStore{}, DefaultConfig(), half); v != nil {
		t.Fatalf("half-missing KYC signal must skip, got %+v", v)
	}
}

func TestPaymentInstrumentMatchRule(t *testing.T) {
	in := identityInput()
	in.AffiliatePayoutInstrumentHash = "hmac-card-9"
	in.ReferredPaymentInstrumentHash = "hmac-card-9"
	v, err := Evaluate(context.Background(), &stubStore{}, DefaultConfig(), in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v == nil || v.MaxSeverity != domain.FraudSeverityCritical {
		t.Fatalf("shared payment instrument must be critical, got %+v", v)
	}
	if !v.RecommendSuspension {
		t.Fatal("critical payment match must recommend suspension review")
	}

	different := identityInput()
	different.AffiliatePayoutInstrumentHash = "hmac-card-9"
	different.ReferredPaymentInstrumentHash = "hmac-card-10"
	if v, _ := Evaluate(context.Background(), &stubStore{}, DefaultConfig(), different); v != nil {
		t.Fatalf("different instruments must be clean, got %+v", v)
	}
}
