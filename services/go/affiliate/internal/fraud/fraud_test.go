package fraud

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/opus-casino/affiliate/internal/domain"
)

var errBoom = errors.New("boom")

type stubStore struct {
	click        *domain.AffiliateClick
	clicks       int64
	attributions int64
	byDevice     int64
	byIP         int64
	err          error
}

func (s *stubStore) GetClickByClickID(_ context.Context, _ string) (*domain.AffiliateClick, error) {
	return s.click, s.err
}

func (s *stubStore) CountClicksSince(_ context.Context, _ uuid.UUID, _ time.Time) (int64, error) {
	return s.clicks, s.err
}

func (s *stubStore) CountAttributionsSince(_ context.Context, _ uuid.UUID, _ time.Time) (int64, error) {
	return s.attributions, s.err
}

func (s *stubStore) CountReferredUsersByDevice(_ context.Context, _ uuid.UUID, _ string, _ time.Time) (int64, error) {
	return s.byDevice, s.err
}

func (s *stubStore) CountReferredUsersByIP(_ context.Context, _ uuid.UUID, _ string, _ time.Time) (int64, error) {
	return s.byIP, s.err
}

func cleanInput() Input {
	return Input{
		AffiliateID:    uuid.New(),
		ReferredUserID: 42,
		ClickID:        "click-1",
		DeviceFP:       "fp-1",
		IPHash:         "ip-1",
		EvaluatedAt:    time.Now().UTC(),
	}
}

func TestEvaluate_CleanTrafficNoVerdict(t *testing.T) {
	v, err := Evaluate(context.Background(), &stubStore{}, DefaultConfig(), cleanInput())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if v != nil {
		t.Fatalf("expected nil verdict for clean traffic, got %+v", v)
	}
}

func TestEvaluate_StoreErrorPropagates(t *testing.T) {
	_, err := Evaluate(context.Background(), &stubStore{err: errBoom}, DefaultConfig(), cleanInput())
	if err == nil {
		t.Fatal("expected store error to propagate")
	}
}

func TestDeviceSharingThresholds(t *testing.T) {
	cases := []struct {
		users int64
		want  domain.FraudSeverity
		none  bool
	}{
		{1, "", true},
		{2, domain.FraudSeverityMedium, false},
		{5, domain.FraudSeverityHigh, false},
		{10, domain.FraudSeverityCritical, false},
	}
	for _, tc := range cases {
		v, err := Evaluate(context.Background(), &stubStore{byDevice: tc.users}, DefaultConfig(), cleanInput())
		if err != nil {
			t.Fatalf("users=%d: %v", tc.users, err)
		}
		if tc.none {
			if v != nil {
				t.Fatalf("users=%d: expected no verdict, got %+v", tc.users, v)
			}
			continue
		}
		if v == nil || v.MaxSeverity != tc.want {
			t.Fatalf("users=%d: expected max severity %s, got %+v", tc.users, tc.want, v)
		}
		if tc.want == domain.FraudSeverityCritical && !v.RecommendSuspension {
			t.Fatalf("users=%d: critical must recommend suspension", tc.users)
		}
	}
}

func TestIPClusterThresholds(t *testing.T) {
	cases := []struct {
		users int64
		want  domain.FraudSeverity
		none  bool
	}{
		{2, "", true},
		{3, domain.FraudSeverityMedium, false},
		{10, domain.FraudSeverityHigh, false},
		{25, domain.FraudSeverityCritical, false},
	}
	for _, tc := range cases {
		v, err := Evaluate(context.Background(), &stubStore{byIP: tc.users}, DefaultConfig(), cleanInput())
		if err != nil {
			t.Fatalf("users=%d: %v", tc.users, err)
		}
		if tc.none && v != nil {
			t.Fatalf("users=%d: expected no verdict, got %+v", tc.users, v)
		}
		if !tc.none && (v == nil || v.MaxSeverity != tc.want) {
			t.Fatalf("users=%d: expected %s, got %+v", tc.users, tc.want, v)
		}
	}
}

func TestVelocityThresholds(t *testing.T) {
	cfg := DefaultConfig()

	v, _ := Evaluate(context.Background(), &stubStore{clicks: 199}, cfg, cleanInput())
	if v != nil {
		t.Fatalf("199 clicks/h must be clean, got %+v", v)
	}
	v, _ = Evaluate(context.Background(), &stubStore{clicks: 200}, cfg, cleanInput())
	if v == nil || v.MaxSeverity != domain.FraudSeverityMedium {
		t.Fatalf("200 clicks/h must be medium, got %+v", v)
	}
	v, _ = Evaluate(context.Background(), &stubStore{clicks: 1000}, cfg, cleanInput())
	if v == nil || v.MaxSeverity != domain.FraudSeverityHigh {
		t.Fatalf("1000 clicks/h must be high, got %+v", v)
	}

	v, _ = Evaluate(context.Background(), &stubStore{attributions: 18}, cfg, cleanInput())
	if v != nil {
		t.Fatalf("19 registrations/day must be clean, got %+v", v)
	}
	v, _ = Evaluate(context.Background(), &stubStore{attributions: 49}, cfg, cleanInput())
	if v == nil || v.MaxSeverity != domain.FraudSeverityHigh {
		t.Fatalf("50 registrations/day must be high, got %+v", v)
	}
}

func TestClicklessAttributionIsLow(t *testing.T) {
	in := cleanInput()
	in.ClickID = ""
	v, err := Evaluate(context.Background(), &stubStore{}, DefaultConfig(), in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v == nil || v.MaxSeverity != domain.FraudSeverityLow {
		t.Fatalf("clickless attribution must be low severity, got %+v", v)
	}
	if v.RecommendSuspension {
		t.Fatal("low severity must not recommend suspension")
	}
}

func TestClickResolvesMissingSignals(t *testing.T) {
	store := &stubStore{
		click: &domain.AffiliateClick{
			ClickID:           "click-9",
			DeviceFingerprint: "fp-shared",
			IPHash:            "ip-shared",
		},
		byDevice: 5, // resolved fp triggers high
	}
	in := cleanInput()
	in.ClickID = "click-9"
	in.DeviceFP = ""
	in.IPHash = ""
	v, err := Evaluate(context.Background(), store, DefaultConfig(), in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v == nil || v.MaxSeverity != domain.FraudSeverityHigh {
		t.Fatalf("expected high via resolved device, got %+v", v)
	}
}

func TestEmptySignalsSkipped(t *testing.T) {
	in := cleanInput()
	in.DeviceFP = ""
	in.IPHash = ""
	v, err := Evaluate(context.Background(), &stubStore{}, DefaultConfig(), in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != nil {
		t.Fatalf("empty signals must be skipped, got %+v", v)
	}
}

func TestVerdictAggregatesMaxSeverity(t *testing.T) {
	store := &stubStore{clicks: 5000, byDevice: 2, byIP: 30}
	v, err := Evaluate(context.Background(), store, DefaultConfig(), cleanInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v == nil {
		t.Fatal("expected verdict")
	}
	if len(v.Findings) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(v.Findings))
	}
	if v.MaxSeverity != domain.FraudSeverityCritical {
		t.Fatalf("expected critical max, got %s", v.MaxSeverity)
	}
	if !v.RecommendSuspension {
		t.Fatal("expected suspension recommendation")
	}
}

func TestTunableConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DeviceSharingMedium = 100
	v, err := Evaluate(context.Background(), &stubStore{byDevice: 3}, cfg, cleanInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != nil {
		t.Fatalf("raised threshold must silence finding, got %+v", v)
	}
}
