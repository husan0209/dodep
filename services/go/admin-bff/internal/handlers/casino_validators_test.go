package handlers

import (
	"strings"
	"testing"

	"github.com/opus-casino/admin-bff/internal/models"
)

func TestParsePercent(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "zero", raw: "0", want: "0"},
		{name: "ninety five", raw: "95.50", want: "95.5"},
		{name: "max", raw: "100", want: "100"},
		{name: "whitespace trimmed", raw: "  96.0  ", want: "96"},
		{name: "empty rejected", raw: "", wantErr: true},
		{name: "negative rejected", raw: "-0.01", wantErr: true},
		{name: "above hundred rejected", raw: "100.01", wantErr: true},
		{name: "thousand rejected", raw: "1000", wantErr: true},
		{name: "non numeric rejected", raw: "abc", wantErr: true},
		{name: "percent literal rejected", raw: "95%", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parsePercent(tc.raw)
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

func TestPercentFromAny(t *testing.T) {
	if got, err := percentFromAny(float64(95.5)); err != nil || got != "95.5" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := percentFromAny(float64(-1)); err == nil {
		t.Fatal("expected error for negative JSON number")
	}
	if _, err := percentFromAny(true); err == nil {
		t.Fatal("expected error for non-numeric input")
	}
}

func TestSanitizeCasinoProviderUpdates(t *testing.T) {
	t.Run("enabled alias and allowlist", func(t *testing.T) {
		got, err := sanitizeCasinoProviderUpdates(map[string]interface{}{
			"enabled":                   true,
			"id":                        "evil",
			"external_id":               "tampered",
			"created_at":                "2020-01-01",
			"api_credentials_encrypted": "leak",
			"revenue_share_pct":         "95.50",
			"settlement_currency":       "usd",
			"games_count":               float64(120),
			"name":                      "New name",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got["is_active"] != true {
			t.Fatalf("enabled must map to is_active, got %v", got["is_active"])
		}
		if _, ok := got["enabled"]; ok {
			t.Fatal("raw alias key must not be written to the column")
		}
		for _, banned := range []string{"id", "external_id", "created_at", "api_credentials_encrypted"} {
			if _, ok := got[banned]; ok {
				t.Fatalf("protected field %q must be dropped", banned)
			}
		}
		if got["revenue_share_pct"] != "95.5" {
			t.Fatalf("rate must be canonicalized, got %v", got["revenue_share_pct"])
		}
		if got["settlement_currency"] != "USD" {
			t.Fatalf("currency must be uppercased, got %v", got["settlement_currency"])
		}
		if got["games_count"] != 120 {
			t.Fatalf("games_count must be int 120, got %v", got["games_count"])
		}
	})

	t.Run("rate out of range rejected", func(t *testing.T) {
		if _, err := sanitizeCasinoProviderUpdates(map[string]interface{}{"revenue_share_pct": "150"}); err == nil {
			t.Fatal("expected error for rate above 100")
		}
	})

	t.Run("bad enabled type rejected", func(t *testing.T) {
		if _, err := sanitizeCasinoProviderUpdates(map[string]interface{}{"enabled": "yes"}); err == nil {
			t.Fatal("expected error for non-boolean enabled")
		}
	})

	t.Run("empty body yields empty updates", func(t *testing.T) {
		got, err := sanitizeCasinoProviderUpdates(map[string]interface{}{"status": "active"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("expected no updates, got %v", got)
		}
	})
}

func TestSanitizeCasinoGameUpdates(t *testing.T) {
	t.Run("allowlist drops protected columns", func(t *testing.T) {
		got, err := sanitizeCasinoGameUpdates(map[string]interface{}{
			"id":          "evil",
			"external_id": "tampered",
			"provider_id": "other",
			"category":    "slots",
			"min_bet":     "0.10",
			"max_bet":     "100.00",
			"rtp":         float64(96.5),
			"sort_weight": float64(10),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, banned := range []string{"id", "external_id", "provider_id"} {
			if _, ok := got[banned]; ok {
				t.Fatalf("protected field %q must be dropped", banned)
			}
		}
		if got["min_bet"] != "0.1" || got["max_bet"] != "100" {
			t.Fatalf("money must be canonicalized, got %v/%v", got["min_bet"], got["max_bet"])
		}
		if got["rtp"] != "96.5" {
			t.Fatalf("rtp must be canonicalized, got %v", got["rtp"])
		}
		if got["sort_weight"] != 10 {
			t.Fatalf("sort_weight must be int 10, got %v", got["sort_weight"])
		}
	})

	t.Run("rtp out of range rejected", func(t *testing.T) {
		if _, err := sanitizeCasinoGameUpdates(map[string]interface{}{"rtp": "150"}); err == nil {
			t.Fatal("expected error for rtp above 100")
		}
		if _, err := sanitizeCasinoGameUpdates(map[string]interface{}{"rtp": float64(-1)}); err == nil {
			t.Fatal("expected error for negative rtp")
		}
	})

	t.Run("invalid money rejected", func(t *testing.T) {
		if _, err := sanitizeCasinoGameUpdates(map[string]interface{}{"min_bet": "-5"}); err == nil {
			t.Fatal("expected error for negative min_bet")
		}
		if _, err := sanitizeCasinoGameUpdates(map[string]interface{}{"max_bet": "abc"}); err == nil {
			t.Fatal("expected error for non-numeric max_bet")
		}
	})
}

func TestSanitizeJackpotPool(t *testing.T) {
	t.Run("normalizes and resets server fields", func(t *testing.T) {
		pool := models.JackpotPool{
			ID:              "client-supplied-id",
			Name:            "Mega Drop",
			Type:            "fixed",
			SeedAmount:      "1000.00",
			CurrentAmount:   "",
			SeedValue:       "",
			ContributionPct: "1.50",
			Currency:        "eur",
		}
		if err := sanitizeJackpotPool(&pool); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pool.ID != "" {
			t.Fatalf("client id must be reset, got %q", pool.ID)
		}
		if pool.CurrentAmount != "1000" {
			t.Fatalf("current_amount must default to seed, got %q", pool.CurrentAmount)
		}
		if pool.SeedValue != "0" {
			t.Fatalf("seed_value must default to 0, got %q", pool.SeedValue)
		}
		if pool.ContributionPct != "1.5" {
			t.Fatalf("contribution must be canonicalized, got %q", pool.ContributionPct)
		}
		if pool.Currency != "EUR" {
			t.Fatalf("currency must be uppercased, got %q", pool.Currency)
		}
		if pool.CreatedAt.IsZero() || pool.UpdatedAt.IsZero() {
			t.Fatal("timestamps must be server-controlled")
		}
	})

	t.Run("missing name rejected", func(t *testing.T) {
		pool := models.JackpotPool{Name: "  ", SeedAmount: "10"}
		if err := sanitizeJackpotPool(&pool); err == nil {
			t.Fatal("expected error for missing name")
		}
	})

	t.Run("invalid money rejected", func(t *testing.T) {
		pool := models.JackpotPool{Name: "Pool", SeedAmount: "-1"}
		if err := sanitizeJackpotPool(&pool); err == nil || !strings.Contains(err.Error(), "seed_amount") {
			t.Fatalf("expected seed_amount error, got %v", err)
		}
	})

	t.Run("invalid contribution rejected", func(t *testing.T) {
		pool := models.JackpotPool{Name: "Pool", SeedAmount: "10", ContributionPct: "200"}
		if err := sanitizeJackpotPool(&pool); err == nil {
			t.Fatal("expected error for contribution above 100")
		}
	})

	t.Run("invalid currency rejected", func(t *testing.T) {
		pool := models.JackpotPool{Name: "Pool", SeedAmount: "10", Currency: "USDD"}
		if err := sanitizeJackpotPool(&pool); err == nil {
			t.Fatal("expected error for non-3-letter currency")
		}
	})
}
