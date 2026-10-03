package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// SetLimitsRequest carries money limits as decimal strings rather than float64,
// per CONVENTIONS.md (NEVER-6: money is never a float). That choice is correct
// but leaves the values untrusted: the field type is `*string`, and before this
// validator existed the request body went straight to the database.
//
// The consequences were not theoretical:
//
//   - "-5000" was accepted as a deposit limit. A negative ceiling is not a
//     restriction, so the player defeated their own deposit limit and any
//     deposit check that compares against it.
//   - "1e9", "NaN", "abc" and arbitrary 10.000-digit strings were accepted,
//     producing limits the downstream wallet could not interpret.
//   - `self_exclusion: false` was accepted, letting a player clear a
//     self-exclusion from a generic profile endpoint. Self-exclusion is only
//     revocable through the Responsible Gambling service, after a cooling-off
//     period; it must not be clearable here.

// moneyLimitPattern accepts a non-negative decimal amount with at most two
// fraction digits. It deliberately rejects a leading '-', a leading '+', an
// exponent, surrounding whitespace and thousands separators. Go's regexp
// engine is RE2, so this is linear-time and cannot be made to backtrack.
var moneyLimitPattern = regexp.MustCompile(`^[0-9]{1,15}(\.[0-9]{1,2})?$`)

// maxSessionMinutes is 24 hours; a session longer than a day is not a limit.
const maxSessionMinutes = 24 * 60

// ErrLimitsRejected indicates a limits request that must not reach storage.
var ErrLimitsRejected = fmt.Errorf("invalid gambling limits request")

// moneyLimit is a named (field, value) pair so validation errors identify the
// offending field instead of failing generically.
type moneyLimit struct {
	name  string
	value *string
}

func (r *SetLimitsRequest) moneyLimits() []moneyLimit {
	return []moneyLimit{
		{"daily_deposit_limit", r.DailyDepositLimit},
		{"weekly_deposit_limit", r.WeeklyDepositLimit},
		{"monthly_deposit_limit", r.MonthlyDepositLimit},
		{"daily_bet_limit", r.DailyBetLimit},
		{"weekly_bet_limit", r.WeeklyBetLimit},
		{"monthly_bet_limit", r.MonthlyBetLimit},
		{"daily_loss_limit", r.DailyLossLimit},
		{"weekly_loss_limit", r.WeeklyLossLimit},
		{"monthly_loss_limit", r.MonthlyLossLimit},
	}
}

// Validate reports whether the request is safe to persist.
//
// Checks, in order:
//  1. every present money limit is a non-negative decimal with ≤2 decimals;
//  2. daily ≤ weekly ≤ monthly within each of the deposit/bet/loss families,
//     because a daily ceiling above the weekly one is self-contradictory and
//     would let the weaker of the two govern in practice;
//  3. the session limit is within 1..1440 minutes;
//  4. self-exclusion is not being cleared.
func (r *SetLimitsRequest) Validate() error {
	if r == nil {
		return fmt.Errorf("%w: request is empty", ErrLimitsRejected)
	}

	for _, l := range r.moneyLimits() {
		if l.value == nil {
			continue
		}
		if !moneyLimitPattern.MatchString(*l.value) {
			return fmt.Errorf("%w: %s must be a non-negative decimal with at most 2 decimal places, got %q",
				ErrLimitsRejected, l.name, *l.value)
		}
	}

	families := []struct {
		name          string
		daily, weekly *string
		monthly       *string
	}{
		{"deposit", r.DailyDepositLimit, r.WeeklyDepositLimit, r.MonthlyDepositLimit},
		{"bet", r.DailyBetLimit, r.WeeklyBetLimit, r.MonthlyBetLimit},
		{"loss", r.DailyLossLimit, r.WeeklyLossLimit, r.MonthlyLossLimit},
	}
	for _, f := range families {
		if err := checkMonotonic(f.name, f.daily, f.weekly, f.monthly); err != nil {
			return err
		}
	}

	if r.SessionTimeMinutes != nil {
		if *r.SessionTimeMinutes < 1 || *r.SessionTimeMinutes > maxSessionMinutes {
			return fmt.Errorf("%w: session_time_minutes must be between 1 and %d, got %d",
				ErrLimitsRejected, maxSessionMinutes, *r.SessionTimeMinutes)
		}
	}

	// Turning self-exclusion OFF here would let a player bypass the one
	// control that cannot be reversed quickly. Revocation is an RG-service
	// operation gated by a cooling-off window.
	if r.SelfExclusion != nil && !*r.SelfExclusion {
		return fmt.Errorf("%w: self-exclusion cannot be cleared here; it is revocable only through the responsible gambling service",
			ErrLimitsRejected)
	}

	return nil
}

// checkMonotonic enforces daily <= weekly <= monthly. Both values must already
// have passed the format check.
func checkMonotonic(name string, daily, weekly, monthly *string) error {
	if daily != nil && weekly != nil {
		if err := requireNotGreater(name+"_daily", *daily, *weekly); err != nil {
			return err
		}
	}
	if weekly != nil && monthly != nil {
		if err := requireNotGreater(name+"_weekly", *weekly, *monthly); err != nil {
			return err
		}
	}
	return nil
}

// requireNotGreater fails when lower is strictly greater than upper.
//
// Comparison runs on minor units (cents) derived by string surgery rather than
// ParseFloat: money is never represented as a binary float here, and the values
// have already been shape-checked by moneyLimitPattern, so the parse cannot
// fail for the reasons that would make a float comparison unsafe.
func requireNotGreater(lowerName, lower, upper string) error {
	lowMinor, ok := minorUnits(lower)
	if !ok {
		return fmt.Errorf("%w: cannot compare %s", ErrLimitsRejected, lowerName)
	}
	upperMinor, ok := minorUnits(upper)
	if !ok {
		return fmt.Errorf("%w: cannot compare upper bound of %s", ErrLimitsRejected, lowerName)
	}
	if lowMinor > upperMinor {
		return fmt.Errorf("%w: %s (%s) must not exceed the larger period limit (%s)",
			ErrLimitsRejected, lowerName, lower, upper)
	}
	return nil
}

// minorUnits converts a validated decimal string to an integer number of cents.
// "10" -> 1000, "10.5" -> 1050, "10.05" -> 1005.
func minorUnits(s string) (int64, bool) {
	whole, frac := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		whole, frac = s[:i], s[i+1:]
	}
	if whole == "" {
		// Covers "" and ".50": an empty integer part means the input is not a
		// well-formed amount, and reading it as zero would silently turn a
		// malformed limit into a zero limit.
		return 0, false
	}
	// The regex allows at most 15 integer digits, so the value always fits
	// comfortably inside int64.
	var units int64
	for i := 0; i < len(whole); i++ {
		d := whole[i]
		if d < '0' || d > '9' {
			// Callers are expected to have shape-checked the value first, but
			// returning a silent wrong number here would corrupt a limit
			// comparison, so non-digits are refused instead of coerced.
			return 0, false
		}
		units = units*10 + int64(d-'0')
	}
	units *= 100
	switch len(frac) {
	case 0:
	case 1:
		if !isDigit(frac[0]) {
			return 0, false
		}
		units += int64(frac[0]-'0') * 10
	case 2:
		if !isDigit(frac[0]) || !isDigit(frac[1]) {
			return 0, false
		}
		units += int64(frac[0]-'0')*10 + int64(frac[1]-'0')
	default:
		return 0, false
	}
	return units, true
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }
