package handlers

import (
	"testing"
)

func TestParseCommissionRate(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "zero", raw: "0", want: "0"},
		{name: "twenty percent", raw: "0.20", want: "0.2"},
		{name: "full", raw: "1", want: "1"},
		{name: "full decimal", raw: "1.00", want: "1"},
		{name: "whitespace trimmed", raw: "  0.35  ", want: "0.35"},
		{name: "empty rejected", raw: "", wantErr: true},
		{name: "blank rejected", raw: "   ", wantErr: true},
		{name: "negative rejected", raw: "-0.01", wantErr: true},
		{name: "above one rejected", raw: "1.0001", wantErr: true},
		{name: "percent scale rejected", raw: "20", wantErr: true},
		{name: "non numeric rejected", raw: "abc", wantErr: true},
		{name: "float artifact rejected", raw: "0.30000000000000004", wantErr: false, want: "0.30000000000000004"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCommissionRate(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got %q", tc.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.raw, err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSanitizeAffiliateUpdates(t *testing.T) {
	t.Run("allowlist drops protected columns", func(t *testing.T) {
		got, err := sanitizeAffiliateUpdates(map[string]interface{}{
			"id":                "evil",
			"status":            "active",
			"created_at":        "2020-01-01",
			"sub_affiliate_pct": "0.5",
			"currency":          "usd",
			"revenue_share_pct": "0.20",
			"hold_period_days":  float64(14),
			"min_payout_amount": "100.00",
			"deal_type":         "revenue_share",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, banned := range []string{"id", "status", "created_at", "sub_affiliate_pct"} {
			if _, ok := got[banned]; ok {
				t.Fatalf("protected field %q must be dropped", banned)
			}
		}
		if got["currency"] != "USD" {
			t.Fatalf("currency must be uppercased, got %v", got["currency"])
		}
		if got["revenue_share_pct"] != "0.2" {
			t.Fatalf("rate must be canonicalized, got %v", got["revenue_share_pct"])
		}
		if got["hold_period_days"] != 14 {
			t.Fatalf("hold days must be int 14, got %v", got["hold_period_days"])
		}
	})

	t.Run("invalid rate rejected", func(t *testing.T) {
		if _, err := sanitizeAffiliateUpdates(map[string]interface{}{"revenue_share_pct": "2.5"}); err == nil {
			t.Fatal("expected error for out-of-range rate")
		}
	})

	t.Run("fractional days rejected", func(t *testing.T) {
		if _, err := sanitizeAffiliateUpdates(map[string]interface{}{"hold_period_days": float64(1.5)}); err == nil {
			t.Fatal("expected error for fractional hold days")
		}
	})

	t.Run("bad currency rejected", func(t *testing.T) {
		if _, err := sanitizeAffiliateUpdates(map[string]interface{}{"currency": "USDD"}); err == nil {
			t.Fatal("expected error for non-3-letter currency")
		}
	})
}
