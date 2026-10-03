package domain

import (
	"errors"
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func TestSetLimitsValidateAccepts(t *testing.T) {
	cases := []struct {
		name string
		req  SetLimitsRequest
	}{
		{"empty request", SetLimitsRequest{}},
		{"nil fields only", SetLimitsRequest{}},
		{"zero limit", SetLimitsRequest{DailyDepositLimit: ptr("0")}},
		{"integer amount", SetLimitsRequest{DailyDepositLimit: ptr("500")}},
		{"two decimals", SetLimitsRequest{DailyDepositLimit: ptr("500.00")}},
		{"one decimal", SetLimitsRequest{DailyDepositLimit: ptr("500.5")}},
		{"large but bounded", SetLimitsRequest{MonthlyDepositLimit: ptr("999999999999999.99")}},
		{"self exclusion on", SetLimitsRequest{SelfExclusion: ptr(true)}},
		{"session minimum", SetLimitsRequest{SessionTimeMinutes: ptr(1)}},
		{"session maximum", SetLimitsRequest{SessionTimeMinutes: ptr(1440)}},
		{
			"monotonic deposit family",
			SetLimitsRequest{
				DailyDepositLimit:   ptr("100.00"),
				WeeklyDepositLimit:  ptr("500.00"),
				MonthlyDepositLimit: ptr("1000.00"),
			},
		},
		{
			"equal bounds are consistent",
			SetLimitsRequest{DailyDepositLimit: ptr("100"), WeeklyDepositLimit: ptr("100.00")},
		},
		{
			"families are independent",
			SetLimitsRequest{
				DailyDepositLimit:  ptr("100.00"),
				WeeklyDepositLimit: ptr("500.00"),
				DailyBetLimit:      ptr("50.00"),
				WeeklyBetLimit:     ptr("500.00"),
			},
		},
		{
			"weekly and monthly only",
			SetLimitsRequest{WeeklyBetLimit: ptr("10"), MonthlyBetLimit: ptr("20")},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.req.Validate(); err != nil {
				t.Fatalf("expected acceptance, got %v", err)
			}
		})
	}
}

// TestSetLimitsValidateRejectsMoneyFormat pins the responsible-gambling
// bypasses that unvalidated *string fields allowed. A negative deposit ceiling
// is not a restriction, so accepting one defeats the control entirely.
func TestSetLimitsValidateRejectsMoneyFormat(t *testing.T) {
	moneyFields := []struct {
		name  string
		apply func(*SetLimitsRequest, *string)
	}{
		{"daily_deposit_limit", func(r *SetLimitsRequest, v *string) { r.DailyDepositLimit = v }},
		{"weekly_deposit_limit", func(r *SetLimitsRequest, v *string) { r.WeeklyDepositLimit = v }},
		{"monthly_deposit_limit", func(r *SetLimitsRequest, v *string) { r.MonthlyDepositLimit = v }},
		{"daily_bet_limit", func(r *SetLimitsRequest, v *string) { r.DailyBetLimit = v }},
		{"weekly_bet_limit", func(r *SetLimitsRequest, v *string) { r.WeeklyBetLimit = v }},
		{"monthly_bet_limit", func(r *SetLimitsRequest, v *string) { r.MonthlyBetLimit = v }},
		{"daily_loss_limit", func(r *SetLimitsRequest, v *string) { r.DailyLossLimit = v }},
		{"weekly_loss_limit", func(r *SetLimitsRequest, v *string) { r.WeeklyLossLimit = v }},
		{"monthly_loss_limit", func(r *SetLimitsRequest, v *string) { r.MonthlyLossLimit = v }},
	}
	badValues := []struct {
		name  string
		value string
	}{
		{"negative", "-5000.00"},
		{"negative zero-ish", "-0.01"},
		{"exponent", "1e9"},
		{"uppercase exponent", "1E9"},
		{"leading plus", "+10.00"},
		{"leading dot", ".50"},
		{"trailing dot", "10."},
		{"three decimals", "10.005"},
		{"comma decimal", "10,00"},
		{"thousands separator", "1,000.00"},
		{"not a number", "abc"},
		{"empty", ""},
		{"whitespace padded", " 10.00 "},
		{"inner space", "10 .00"},
		{"hex", "0x10"},
		{"infinity", "Inf"},
		{"nan", "NaN"},
		{"currency suffix", "100USD"},
		{"symbol prefix", "$100.00"},
		{"underscored", "1_000"},
		{"too many integer digits", "12345678901234567890"},
	}
	for _, f := range moneyFields {
		for _, bv := range badValues {
			t.Run(f.name+"/"+bv.name, func(t *testing.T) {
				var req SetLimitsRequest
				f.apply(&req, ptr(bv.value))
				err := req.Validate()
				if err == nil {
					t.Fatalf("%s=%q must be rejected", f.name, bv.value)
				}
				if !errors.Is(err, ErrLimitsRejected) {
					t.Fatalf("error must wrap ErrLimitsRejected, got %v", err)
				}
				if !strings.Contains(err.Error(), f.name) {
					t.Fatalf("error must name the offending field %q, got %v", f.name, err)
				}
			})
		}
	}
}

func TestSetLimitsValidateRejectsSelfExclusionRemoval(t *testing.T) {
	req := SetLimitsRequest{SelfExclusion: ptr(false)}
	err := req.Validate()
	if err == nil {
		t.Fatal("clearing self-exclusion must be rejected")
	}
	if !errors.Is(err, ErrLimitsRejected) {
		t.Fatalf("must wrap ErrLimitsRejected, got %v", err)
	}
	if !strings.Contains(err.Error(), "responsible gambling") {
		t.Fatalf("error should point at the RG service, got %v", err)
	}
}

func TestSetLimitsValidateSessionBounds(t *testing.T) {
	for _, v := range []int{-1, -30, 0, 1441, 100000} {
		req := SetLimitsRequest{SessionTimeMinutes: ptr(v)}
		if err := req.Validate(); err == nil {
			t.Fatalf("session_time_minutes=%d must be rejected", v)
		}
	}
	for _, v := range []int{1, 15, 60, 180, 1440} {
		req := SetLimitsRequest{SessionTimeMinutes: ptr(v)}
		if err := req.Validate(); err != nil {
			t.Fatalf("session_time_minutes=%d must be accepted: %v", v, err)
		}
	}
}

func TestSetLimitsValidateRejectsNonMonotonic(t *testing.T) {
	cases := []struct {
		name string
		req  SetLimitsRequest
	}{
		{"daily above weekly deposit", SetLimitsRequest{
			DailyDepositLimit: ptr("500"), WeeklyDepositLimit: ptr("100"),
		}},
		{"weekly above monthly deposit", SetLimitsRequest{
			WeeklyDepositLimit: ptr("900"), MonthlyDepositLimit: ptr("100"),
		}},
		{"daily above weekly bet", SetLimitsRequest{
			DailyBetLimit: ptr("5"), WeeklyBetLimit: ptr("1"),
		}},
		{"weekly above monthly bet", SetLimitsRequest{
			WeeklyBetLimit: ptr("9.01"), MonthlyBetLimit: ptr("9.00"),
		}},
		{"daily above weekly loss", SetLimitsRequest{
			DailyLossLimit: ptr("800"), WeeklyLossLimit: ptr("200"),
		}},
		{"weekly above monthly loss", SetLimitsRequest{
			WeeklyLossLimit: ptr("1000"), MonthlyLossLimit: ptr("999"),
		}},
		{"one cent above weekly", SetLimitsRequest{
			DailyDepositLimit: ptr("100.01"), WeeklyDepositLimit: ptr("100"),
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.req.Validate(); err == nil {
				t.Fatal("expected rejection of non-monotonic limits")
			}
		})
	}
}

func TestNilRequestIsRejected(t *testing.T) {
	var req *SetLimitsRequest
	if err := req.Validate(); err == nil {
		t.Fatal("nil request must be rejected, not panic")
	}
}

func TestMinorUnits(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"0", 0},
		{"1", 100},
		{"10", 1000},
		{"10.5", 1050},
		{"10.05", 1005},
		{"10.00", 1000},
		{"0.01", 1},
		{"999999999999999.99", 99999999999999999},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := minorUnits(tc.in)
			if !ok {
				t.Fatalf("minorUnits(%q) failed", tc.in)
			}
			if got != tc.want {
				t.Fatalf("minorUnits(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// TestMinorUnitsIsNotFloat guards the money rule from CONVENTIONS.md
// (NEVER-6) at the point where it could silently be reintroduced.
func TestMinorUnitsIsNotFloat(t *testing.T) {
	// 0.1 + 0.2 != 0.3 in binary floating point; in minor units it is exact.
	a, _ := minorUnits("0.1")
	b, _ := minorUnits("0.2")
	c, _ := minorUnits("0.3")
	if a+b != c {
		t.Fatalf("minor-unit arithmetic must be exact: %d+%d = %d, want %d", a, b, a+b, c)
	}
}

func TestMinorUnitsRejectsMalformed(t *testing.T) {
	// "1." is deliberately absent: it parses as 100 minor units, and the
	// responsibility for rejecting that spelling belongs to the format regex,
	// not to the arithmetic helper. The two layers are tested separately.
	for _, s := range []string{"", ".50", "abc", "1.234", "-1", "1e5", "10.0x", "1 0"} {
		if _, ok := minorUnits(s); ok {
			t.Fatalf("minorUnits(%q) must fail", s)
		}
	}
}

func TestMinorUnitsAcceptsTrailingSeparator(t *testing.T) {
	got, ok := minorUnits("1.")
	if !ok || got != 100 {
		t.Fatalf("minorUnits(\"1.\") = %d, %v; want 100, true", got, ok)
	}
}
