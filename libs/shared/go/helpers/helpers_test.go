package helpers

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/opus-casino/shared/go/types"
)

func mustMoney(t *testing.T, amount, currency string) types.Money {
	t.Helper()
	m, err := types.NewMoney(amount, currency)
	if err != nil {
		t.Fatalf("bad fixture: %v", err)
	}
	return m
}

func TestFormatMoney(t *testing.T) {
	cases := []struct {
		money types.Money
		want  string
	}{
		{mustMoney(t, "100", "USD"), "$100.00"},
		{mustMoney(t, "10.5", "EUR"), "€10.50"},
		{mustMoney(t, "5", "GBP"), "£5.00"},
		{mustMoney(t, "7", "KZT"), "KZT 7.00"},
	}
	for _, tc := range cases {
		if got := FormatMoney(tc.money, "en"); got != tc.want {
			t.Errorf("expected %q, got %q", tc.want, got)
		}
	}
}

func TestParseAddSubtractMoney(t *testing.T) {
	m, err := ParseMoney("42.50", "USD")
	if err != nil || m.Currency != "USD" {
		t.Fatalf("parse failed: %+v %v", m, err)
	}
	if _, err := ParseMoney("junk", "USD"); err == nil {
		t.Error("expected parse error")
	}

	sum, err := AddMoney(mustMoney(t, "10", "USD"), mustMoney(t, "5", "USD"))
	if err != nil || sum.Amount.String() != "15" {
		t.Fatalf("bad add: %+v %v", sum, err)
	}
	if _, err := AddMoney(mustMoney(t, "10", "USD"), mustMoney(t, "5", "EUR")); err == nil {
		t.Error("expected currency mismatch on add")
	}

	diff, err := SubtractMoney(mustMoney(t, "10", "USD"), mustMoney(t, "4", "USD"))
	if err != nil || diff.Amount.String() != "6" {
		t.Fatalf("bad subtract: %+v %v", diff, err)
	}
	if _, err := SubtractMoney(mustMoney(t, "10", "USD"), mustMoney(t, "4", "EUR")); err == nil {
		t.Error("expected currency mismatch on subtract")
	}
}

func TestMultiplyAndCompareMoney(t *testing.T) {
	scaled := MultiplyMoney(mustMoney(t, "100", "USD"), decimal.NewFromFloat(0.5))
	if scaled.Amount.String() != "50" {
		t.Fatalf("bad multiply: %+v", scaled)
	}
	cmp, err := CompareMoney(mustMoney(t, "10", "USD"), mustMoney(t, "20", "USD"))
	if err != nil || cmp != -1 {
		t.Fatalf("expected -1, got %d %v", cmp, err)
	}
	cmp, err = CompareMoney(mustMoney(t, "20", "USD"), mustMoney(t, "20", "USD"))
	if err != nil || cmp != 0 {
		t.Fatalf("expected 0, got %d %v", cmp, err)
	}
	cmp, err = CompareMoney(mustMoney(t, "30", "USD"), mustMoney(t, "20", "USD"))
	if err != nil || cmp != 1 {
		t.Fatalf("expected 1, got %d %v", cmp, err)
	}
	if _, err := CompareMoney(mustMoney(t, "1", "USD"), mustMoney(t, "1", "EUR")); err == nil {
		t.Error("expected currency mismatch on compare")
	}
}

func TestGenerateUUIDAndClocks(t *testing.T) {
	a, b := GenerateUUID(), GenerateUUID()
	if a == "" || a == b {
		t.Fatal("uuids must be unique non-empty")
	}
	if _, err := uuid.Parse(a); err != nil {
		t.Fatalf("generated value is not a uuid: %s", a)
	}
	before := NowMs()
	if NowMs() < before {
		t.Error("clock went backwards")
	}
	iso := NowISO()
	if _, err := time.Parse(time.RFC3339, iso); err != nil {
		t.Errorf("bad ISO timestamp: %s", iso)
	}
}

func TestDeepClone(t *testing.T) {
	type inner struct {
		Tags []string `json:"tags"`
	}
	type doc struct {
		Name  string `json:"name"`
		Inner inner  `json:"inner"`
	}
	src := doc{Name: "x", Inner: inner{Tags: []string{"a"}}}
	cloned, err := DeepClone(src)
	if err != nil {
		t.Fatalf("clone failed: %v", err)
	}
	cloned.Inner.Tags[0] = "mutated"
	if src.Inner.Tags[0] != "a" {
		t.Fatal("clone shares memory with source")
	}
	if _, err := DeepClone(make(chan int)); err == nil {
		t.Error("expected error cloning unserializable value")
	}
}

func TestCalculatePercentage(t *testing.T) {
	got := CalculatePercentage(decimal.NewFromInt(25), decimal.NewFromInt(200))
	if got == nil || got.String() != "12.5" {
		t.Fatalf("bad percentage: %+v", got)
	}
	if got := CalculatePercentage(decimal.NewFromInt(1), decimal.Zero); got != nil {
		t.Fatalf("expected nil on zero total, got %+v", got)
	}
}

func TestClampAndRound(t *testing.T) {
	d := decimal.NewFromInt
	if got := ClampDecimal(d(5), d(10), d(20)); !got.Equal(d(10)) {
		t.Errorf("clamp low failed: %s", got)
	}
	if got := ClampDecimal(d(25), d(10), d(20)); !got.Equal(d(20)) {
		t.Errorf("clamp high failed: %s", got)
	}
	if got := ClampDecimal(d(15), d(10), d(20)); !got.Equal(d(15)) {
		t.Errorf("clamp mid failed: %s", got)
	}
	half, _ := decimal.NewFromString("2.345")
	if got := RoundDecimal(half, 2); got.String() != "2.35" {
		t.Errorf("round failed: %s", got)
	}
}

func TestRetry(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 5, InitialDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond, Multiplier: 2}

	calls := 0
	got, err := Retry(func() (string, error) {
		calls++
		if calls < 3 {
			return "", errors.New("flaky")
		}
		return "ok", nil
	}, cfg)
	if err != nil || got != "ok" || calls != 3 {
		t.Fatalf("retry should succeed on 3rd try: %q %v calls=%d", got, err, calls)
	}

	calls = 0
	sentinel := errors.New("always")
	_, err = Retry(func() (int, error) {
		calls++
		return 0, sentinel
	}, cfg)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected last error, got %v", err)
	}
	if calls != 6 {
		t.Fatalf("expected 1 + 5 retries, got %d", calls)
	}
}

func TestDebouncerAndThrottler(t *testing.T) {
	db := NewDebouncer(50 * time.Millisecond)
	if !db.ShouldAllow() {
		t.Fatal("first call must be allowed")
	}
	if db.ShouldAllow() {
		t.Fatal("immediate second call must be denied")
	}
	time.Sleep(60 * time.Millisecond)
	if !db.ShouldAllow() {
		t.Fatal("call after delay must be allowed")
	}

	th := NewThrottler(50 * time.Millisecond)
	if !th.ShouldAllow() || th.ShouldAllow() {
		t.Fatal("throttler should allow then deny")
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()
	if cfg.MaxRetries != 3 || cfg.Multiplier != 2.0 {
		t.Fatalf("bad defaults: %+v", cfg)
	}
}
