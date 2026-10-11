package constants

import (
	"strings"
	"testing"
)

// TestConstants pins the shared constant surface. These values are consumed
// by every Go service; an accidental change must fail loudly here.
func TestCurrencies(t *testing.T) {
	required := []string{"USD", "EUR", "GBP", "BTC", "ETH", "USDT"}
	for _, c := range required {
		v, ok := Currencies[c]
		if !ok || v != c {
			t.Errorf("currency %s missing or mismatched: %q", c, v)
		}
	}
}

func TestRestrictedCountries(t *testing.T) {
	if len(RestrictedCountries) == 0 {
		t.Fatal("restricted list must not be empty")
	}
	seen := map[string]bool{}
	for _, c := range RestrictedCountries {
		if len(c) != 2 || strings.ToUpper(c) != c {
			t.Errorf("country must be ISO alpha-2 uppercase: %q", c)
		}
		if seen[c] {
			t.Errorf("duplicate country: %s", c)
		}
		seen[c] = true
	}
	for _, must := range []string{"US", "CN", "RU"} {
		if !seen[must] {
			t.Errorf("expected %s in restricted list", must)
		}
	}
}

func TestRateLimitsSane(t *testing.T) {
	if RateLimits.LoginAttempts == 0 || RateLimits.LoginAttempts > 20 {
		t.Errorf("suspicious login attempts: %d", RateLimits.LoginAttempts)
	}
	if RateLimits.APIRequestsPerMinute == 0 || RateLimits.BetsPerSecond == 0 {
		t.Error("rate limits must be positive")
	}
	if RateLimits.TOTPWindowSeconds != 30 {
		t.Errorf("TOTP window must be 30s (RFC 6238), got %d", RateLimits.TOTPWindowSeconds)
	}
}

func TestBetAndPaymentLimitsOrder(t *testing.T) {
	if BetLimits.MinStake == "" || BetLimits.MaxStake == "" {
		t.Fatal("stake bounds must be set")
	}
	if BetLimits.MinStake >= BetLimits.MaxStake {
		t.Errorf("min stake %s must be < max stake %s", BetLimits.MinStake, BetLimits.MaxStake)
	}
	if BetLimits.MinOdds >= BetLimits.MaxOdds {
		t.Errorf("min odds %s must be < max odds %s", BetLimits.MinOdds, BetLimits.MaxOdds)
	}
	if PaymentLimits.MinDeposit == "" || PaymentLimits.MinWithdrawal == "" {
		t.Fatal("payment bounds must be set")
	}
	if PaymentLimits.MinDeposit >= PaymentLimits.MaxDepositDaily {
		t.Error("min deposit must be < max daily deposit")
	}
}

func TestSessionTTLs(t *testing.T) {
	if Session.AccessTokenTTLSeconds != 900 {
		t.Errorf("access TTL must be 15min (900s), got %d", Session.AccessTokenTTLSeconds)
	}
	if Session.RefreshTokenTTLSeconds != 604800 {
		t.Errorf("refresh TTL must be 7d, got %d", Session.RefreshTokenTTLSeconds)
	}
	if Session.MaxSessionsPerUser <= 0 {
		t.Error("max sessions must be positive")
	}
}

func TestErrorCodeNamespaces(t *testing.T) {
	prefixed := map[string]string{
		ErrorCodes.AuthInvalidCredentials: "AUTH_",
		ErrorCodes.WalletNotFound:         "WALLET_",
		ErrorCodes.BetNotFound:            "BET_",
		ErrorCodes.InternalError:          "SYS_",
	}
	for code, prefix := range prefixed {
		if !strings.HasPrefix(code, prefix) {
			t.Errorf("code %q must start with %q", code, prefix)
		}
	}
	seen := map[string]string{}
	all := []string{
		ErrorCodes.AuthInvalidCredentials, ErrorCodes.AuthTokenExpired,
		ErrorCodes.AuthTokenInvalid, ErrorCodes.Auth2FARequired,
		ErrorCodes.Auth2FAInvalid, ErrorCodes.AuthAccountLocked,
		ErrorCodes.WalletNotFound, ErrorCodes.InsufficientBalance,
		ErrorCodes.InsufficientAvailableBalance, ErrorCodes.BetNotFound,
		ErrorCodes.BetInvalid, ErrorCodes.BetAlreadySettled,
		ErrorCodes.BetLimitExceeded, ErrorCodes.BetOddsChanged,
		ErrorCodes.InternalError, ErrorCodes.ServiceUnavailable,
		ErrorCodes.RateLimitExceeded,
	}
	for _, c := range all {
		if c == "" {
			t.Fatal("error code must not be empty")
		}
		if prev, dup := seen[c]; dup {
			t.Errorf("duplicate error code %q (also %q)", c, prev)
		}
		seen[c] = c
	}
}
