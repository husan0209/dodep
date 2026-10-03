package types

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
)

func TestNewMoney(t *testing.T) {
	m, err := NewMoney("100.00", "USD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Amount.String() != "100" || m.Currency != "USD" {
		t.Fatalf("wrong money: %+v", m)
	}
	if _, err := NewMoney("abc", "USD"); err == nil {
		t.Error("expected error for non-numeric amount")
	}
	if _, err := NewMoney("-5.00", "USD"); err != ErrNegativeAmount {
		t.Errorf("expected ErrNegativeAmount, got %v", err)
	}
	if got := MustNewMoney("10.50", "EUR"); got.String() != "10.5 EUR" {
		t.Errorf("wrong string: %s", got.String())
	}
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected MustNewMoney to panic on bad input")
		}
	}()
	MustNewMoney("nope", "USD")
}

func TestMoneyArithmetic(t *testing.T) {
	a := MustNewMoney("100.00", "USD")
	b := MustNewMoney("25.50", "USD")

	sum, err := a.Add(b)
	if err != nil || sum.Amount.String() != "125.5" {
		t.Fatalf("bad add: %+v %v", sum, err)
	}
	diff, err := a.Subtract(b)
	if err != nil || diff.Amount.String() != "74.5" {
		t.Fatalf("bad subtract: %+v %v", diff, err)
	}
	scaled := a.Multiply(decimal.NewFromFloat(1.5))
	if scaled.Amount.String() != "150" {
		t.Fatalf("bad multiply: %+v", scaled)
	}

	eur := MustNewMoney("10.00", "EUR")
	if _, err := a.Add(eur); err != ErrCurrencyMismatch {
		t.Errorf("expected currency mismatch on add, got %v", err)
	}
	if _, err := a.Subtract(eur); err != ErrCurrencyMismatch {
		t.Errorf("expected currency mismatch on subtract, got %v", err)
	}
}

func TestMoneyPredicates(t *testing.T) {
	zero := MustNewMoney("0", "USD")
	if !zero.IsZero() || zero.IsPositive() {
		t.Error("zero predicates wrong")
	}
	pos := MustNewMoney("0.01", "USD")
	if pos.IsZero() || !pos.IsPositive() {
		t.Error("positive predicates wrong")
	}
}

func TestIDsRoundTrip(t *testing.T) {
	uid := NewUserId()
	if uid.String() == "" {
		t.Fatal("empty user id string")
	}
	parsed, err := ParseUserId(uid.String())
	if err != nil || parsed != uid {
		t.Fatalf("user id round trip failed: %v", err)
	}
	if _, err := ParseUserId("nope"); err == nil {
		t.Error("expected parse error")
	}

	for _, tc := range []struct {
		name  string
		str   func() string
		parse func(string) error
	}{
		{"bet", func() string { return NewBetId().String() }, func(s string) error { _, e := ParseBetId(s); return e }},
		{"tx", func() string { return NewTransactionId().String() }, func(s string) error { _, e := ParseTransactionId(s); return e }},
		{"game", func() string { return NewGameId().String() }, func(s string) error { _, e := ParseGameId(s); return e }},
		{"session", func() string { return NewSessionId().String() }, func(s string) error { _, e := ParseSessionId(s); return e }},
	} {
		if err := tc.parse(tc.str()); err != nil {
			t.Errorf("%s id round trip failed: %v", tc.name, err)
		}
		if err := tc.parse("bad"); err == nil {
			t.Errorf("%s id should reject bad input", tc.name)
		}
	}
}

func TestUserIdJSON(t *testing.T) {
	uid := NewUserId()
	raw, err := json.Marshal(uid)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var back UserId
	if err := json.Unmarshal(raw, &back); err != nil || back != uid {
		t.Fatalf("json round trip failed: %v", err)
	}
	var bad UserId
	if err := json.Unmarshal([]byte(`"not-a-uuid"`), &bad); err == nil {
		t.Error("expected error for bad uuid json")
	}
	if err := json.Unmarshal([]byte(`123`), &bad); err == nil {
		t.Error("expected error for non-string json")
	}
}

func TestApiResponseWrappers(t *testing.T) {
	ok := Success("hello")
	if ok.Data == nil || *ok.Data != "hello" || ok.Error != nil {
		t.Fatalf("bad success wrapper: %+v", ok)
	}
	failed := Error[string](ErrorDetails{ErrorCode: "NOT_FOUND", ErrorMessage: "missing"})
	if failed.Data != nil || failed.Error == nil || failed.Error.ErrorCode != "NOT_FOUND" {
		t.Fatalf("bad error wrapper: %+v", failed)
	}
	if got := DefaultPaginationParams(); got.PageSize != 20 {
		t.Fatalf("bad default pagination: %+v", got)
	}
}
