package handlers

import (
	"testing"
)

func strptr(s string) *string { return &s }

func TestBuildSetLimitsRequest_MapsSupportedFields(t *testing.T) {
	req, notApplied, err := buildSetLimitsRequest("EUR",
		strptr("100.00"), strptr("500"), strptr("50.5"), strptr("25"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(notApplied) != 0 {
		t.Fatalf("expected empty notApplied, got %v", notApplied)
	}
	if req.GetDailyDepositLimit().GetAmount().GetAmount() != "100" ||
		req.GetDailyDepositLimit().GetAmount().GetCurrency() != "EUR" {
		t.Errorf("bad daily deposit: %+v", req.GetDailyDepositLimit())
	}
	if req.GetWeeklyDepositLimit().GetAmount().GetAmount() != "500" {
		t.Errorf("bad weekly deposit: %+v", req.GetWeeklyDepositLimit())
	}
	if req.GetDailyBetLimit().GetAmount().GetAmount() != "50.5" {
		t.Errorf("bad daily bet: %+v", req.GetDailyBetLimit())
	}
	if req.GetDailyLossLimit().GetAmount().GetAmount() != "25" {
		t.Errorf("bad daily loss: %+v", req.GetDailyLossLimit())
	}
	// Optional-only mapping: unset fields stay nil (leave unchanged downstream).
	req2, _, err := buildSetLimitsRequest("USD", nil, nil, strptr("10"), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req2.GetDailyDepositLimit() != nil || req2.GetDailyLossLimit() != nil {
		t.Errorf("unset fields must stay nil: %+v", req2)
	}
	if req2.GetDailyBetLimit() == nil {
		t.Errorf("daily bet must be set: %+v", req2)
	}
}

func TestBuildSetLimitsRequest_RejectsBadAmounts(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{"not a number", "abc"},
		{"negative", "-5"},
		{"empty", ""},
		{"float artifact", "0.1+0.2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := buildSetLimitsRequest("USD", &tc.value, nil, nil, nil); err == nil {
				t.Errorf("expected error for %q", tc.value)
			}
		})
	}
}

func TestMoneyLimit_NilStaysNil(t *testing.T) {
	got, err := moneyLimit(nil, "USD")
	if err != nil || got != nil {
		t.Errorf("expected (nil, nil), got (%v, %v)", got, err)
	}
}
