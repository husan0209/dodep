package handlers

import (
	"errors"
	"strings"
	"testing"
)

func TestPseudonymizePlayer(t *testing.T) {
	t.Run("deterministic for the same id", func(t *testing.T) {
		a := pseudonymizePlayer(4242)
		b := pseudonymizePlayer(4242)
		if a != b {
			t.Fatalf("expected stable pseudonym, got %q and %q", a, b)
		}
	})

	t.Run("different for different ids", func(t *testing.T) {
		if pseudonymizePlayer(1) == pseudonymizePlayer(2) {
			t.Fatal("expected distinct pseudonyms")
		}
	})

	t.Run("does not leak the raw id and is prefixed", func(t *testing.T) {
		got := pseudonymizePlayer(987654321)
		if !strings.HasPrefix(got, "plr_") {
			t.Fatalf("expected plr_ prefix, got %q", got)
		}
		if strings.Contains(got, "987654321") {
			t.Fatalf("pseudonym %q must not contain the raw player id", got)
		}
		// 6 hex bytes -> 12 hex chars.
		if len(got) != len("plr_")+12 {
			t.Fatalf("unexpected pseudonym length: %q", got)
		}
	})
}

func TestFraudFlagTransitionTable(t *testing.T) {
	t.Run("open and in_review can be closed", func(t *testing.T) {
		for _, from := range []string{"open", "in_review"} {
			for _, to := range []string{"resolved", "dismissed"} {
				if !containsString(validFraudFlagTransitions[from], to) {
					t.Fatalf("%s -> %s must be allowed", from, to)
				}
			}
		}
	})

	t.Run("terminal states have no outgoing transitions", func(t *testing.T) {
		for _, terminal := range []string{"resolved", "dismissed"} {
			if _, ok := validFraudFlagTransitions[terminal]; ok {
				t.Fatalf("%s must be terminal", terminal)
			}
		}
	})

	t.Run("flags cannot be reopened or flipped", func(t *testing.T) {
		if containsString(validFraudFlagTransitions["open"], "open") {
			t.Fatal("open -> open must not be allowed")
		}
		if containsString(validFraudFlagTransitions["in_review"], "resolved") != true {
			t.Fatal("in_review -> resolved must be allowed")
		}
	})
}

func TestPayoutTransitionTable(t *testing.T) {
	t.Run("pending can be approved or rejected", func(t *testing.T) {
		for _, to := range []string{"approved", "rejected"} {
			if !containsString(validPayoutTransitions["pending"], to) {
				t.Fatalf("pending -> %s must be allowed", to)
			}
		}
	})

	t.Run("approved can only move to paid", func(t *testing.T) {
		allowed := validPayoutTransitions["approved"]
		if !containsString(allowed, "paid") {
			t.Fatal("approved -> paid must be allowed")
		}
		if containsString(allowed, "rejected") {
			t.Fatal("money already approved must not be rejectable")
		}
	})

	t.Run("rejected and paid are terminal", func(t *testing.T) {
		for _, terminal := range []string{"rejected", "paid", "processing"} {
			if _, ok := validPayoutTransitions[terminal]; ok {
				t.Fatalf("%s must have no outgoing transitions", terminal)
			}
		}
	})

	t.Run("cannot skip pending straight to paid", func(t *testing.T) {
		if containsString(validPayoutTransitions["pending"], "paid") {
			t.Fatal("pending -> paid must be blocked (compliance gate)")
		}
	})

	t.Run("no self transitions", func(t *testing.T) {
		for from, allowed := range validPayoutTransitions {
			if containsString(allowed, from) {
				t.Fatalf("%s -> %s must be blocked", from, from)
			}
		}
	})
}

func TestPayoutTransitionErrorsAreDistinguishable(t *testing.T) {
	if errors.Is(errInvalidPayoutTransition, errPayoutAlreadyTransitioned) {
		t.Fatal("illegal transition and race must be distinct errors (409 reasons)")
	}
	if errors.Is(errAffiliatePayoutNotFound, errInvalidPayoutTransition) {
		t.Fatal("not found must be distinct from illegal transition")
	}
}

func TestContainsString(t *testing.T) {
	if !containsString([]string{"a", "b"}, "b") {
		t.Fatal("expected b to be found")
	}
	if containsString(nil, "a") {
		t.Fatal("empty haystack must not match")
	}
	if containsString([]string{"a"}, "A") {
		t.Fatal("match must be case sensitive")
	}
}
