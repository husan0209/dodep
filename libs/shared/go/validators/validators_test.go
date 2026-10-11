package validators

import "testing"

func TestIsValidEmail(t *testing.T) {
	valid := []string{"user@example.com", "a.b+tag@sub.domain.co", "x@y.io"}
	for _, v := range valid {
		if !IsValidEmail(v) {
			t.Errorf("expected valid email: %s", v)
		}
	}
	invalid := []string{"", "plain", "@nodomain", "a@b", "a b@c.com", "a@@b.com"}
	for _, v := range invalid {
		if IsValidEmail(v) {
			t.Errorf("expected invalid email: %q", v)
		}
	}
}

func TestIsValidUUID(t *testing.T) {
	if !IsValidUUID("550e8400-e29b-41d4-a716-446655440000") {
		t.Error("expected valid v4 uuid")
	}
	// NOTE: implementation uses uuid.Parse, so it accepts any UUID version,
	// not just v4 despite the doc comment. Test pins actual behavior; if v4-only
	// enforcement is wanted, IsValidUUID must switch to uuidRegex.
	if !IsValidUUID("550e8400-e29b-11d4-a716-446655440000") {
		t.Error("expected v1 uuid to be accepted by current implementation")
	}
	for _, v := range []string{"", "not-a-uuid"} {
		if IsValidUUID(v) {
			t.Errorf("expected invalid uuid: %q", v)
		}
	}
}

func TestCountryAndCurrencyCodes(t *testing.T) {
	for _, c := range []string{"US", "UA", "DE"} {
		if !IsValidCountryCode(c) {
			t.Errorf("expected valid country: %s", c)
		}
	}
	for _, c := range []string{"", "U", "USA", "us", "U1", "U "} {
		if IsValidCountryCode(c) {
			t.Errorf("expected invalid country: %q", c)
		}
	}
	for _, c := range []string{"USD", "EUR", "BTC"} {
		if !IsValidCurrencyCode(c) {
			t.Errorf("expected valid currency: %s", c)
		}
	}
	for _, c := range []string{"", "US", "USDD", "usd", "U1D"} {
		if IsValidCurrencyCode(c) {
			t.Errorf("expected invalid currency: %q", c)
		}
	}
}

func TestIsValidMoneyAmount(t *testing.T) {
	for _, v := range []string{"0", "10", "100.00", "0.99", "999999999.99"} {
		if !IsValidMoneyAmount(v) {
			t.Errorf("expected valid amount: %s", v)
		}
	}
	for _, v := range []string{"", "-5", "10.123", "abc", "10,00", " 10", "10 "} {
		if IsValidMoneyAmount(v) {
			t.Errorf("expected invalid amount: %q", v)
		}
	}
}

func TestIsValidPassword(t *testing.T) {
	if !IsValidPassword("Str0ng!pass") {
		t.Error("expected strong password to pass")
	}
	weak := map[string]string{
		"short1!":    "too short",
		"alllower1!": "no upper",
		"ALLUPPER1!": "no lower",
		"AllUpper!!": "no digit",
		"AllUpper12": "no special",
		"":           "empty",
	}
	for pw, reason := range weak {
		if IsValidPassword(pw) {
			t.Errorf("expected weak password (%s): %q", reason, pw)
		}
	}
}

func TestIsValidPhone(t *testing.T) {
	if !IsValidPhone("+380501234567") {
		t.Error("expected valid E.164 phone")
	}
	for _, v := range []string{"", "0501234567", "+0", "+380 50 123", "abc"} {
		if IsValidPhone(v) {
			t.Errorf("expected invalid phone: %q", v)
		}
	}
}

func TestIsValidOdds(t *testing.T) {
	for _, v := range []string{"1.01", "2.50", "1000", "1.5"} {
		if !IsValidOdds(v) {
			t.Errorf("expected valid odds: %s", v)
		}
	}
	for _, v := range []string{"", "1.00", "0.5", "1000.01", "abc", "-2"} {
		if IsValidOdds(v) {
			t.Errorf("expected invalid odds: %q", v)
		}
	}
}

func TestIsValidPercentage(t *testing.T) {
	for _, v := range []float64{0, 50.5, 100} {
		if !IsValidPercentage(v) {
			t.Errorf("expected valid percentage: %v", v)
		}
	}
	for _, v := range []float64{-0.1, 100.1} {
		if IsValidPercentage(v) {
			t.Errorf("expected invalid percentage: %v", v)
		}
	}
}

func TestIsValidIP(t *testing.T) {
	for _, v := range []string{"127.0.0.1", "192.168.1.1", "::1", "2001:db8::1"} {
		if !IsValidIP(v) {
			t.Errorf("expected valid IP: %s", v)
		}
	}
	for _, v := range []string{"", "999.1.1.1", "abc"} {
		if IsValidIP(v) {
			t.Errorf("expected invalid IP: %q", v)
		}
	}
}

func TestIsValidDate(t *testing.T) {
	if !IsValidDate("2026-09-30") {
		t.Error("expected valid date")
	}
	for _, v := range []string{"", "2026-13-01", "2026-02-30", "30-09-2026", "2026-9-3", "1899-01-01"} {
		if IsValidDate(v) {
			t.Errorf("expected invalid date: %q", v)
		}
	}
}

func TestIsValidUsername(t *testing.T) {
	for _, v := range []string{"player1", "A_bc123", "abcdefghijklmnopqrst"} {
		if !IsValidUsername(v) {
			t.Errorf("expected valid username: %s", v)
		}
	}
	for _, v := range []string{"", "ab", "1abc", "a", "abcdefghijklmnopqrstu", "a-b"} {
		if IsValidUsername(v) {
			t.Errorf("expected invalid username: %q", v)
		}
	}
}
