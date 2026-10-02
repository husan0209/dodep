package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/opus-casino/affiliate/internal/domain"
	"github.com/opus-casino/affiliate/internal/fraud"
)

// These tests cover the wiring only: that BindReferredUser invokes the
// anti-fraud engine, that findings become reviewable flags, and that engine
// failures never break a legitimate signup. Rule thresholds themselves are
// covered in internal/fraud/fraud_test.go.

func activeProfileForFraud(userID int64) *domain.AffiliateProfile {
	return &domain.AffiliateProfile{
		ID:       uuid.New(),
		UserID:   userID,
		Status:   domain.AffiliateStatusActive,
		Currency: "USD",
	}
}

func TestBindReferredUser_RaisesFraudFlags(t *testing.T) {
	ctx := context.Background()
	affiliateID := uuid.New()

	var flags []*domain.AffiliateFraudFlag
	repo := &mockAffiliateRepository{
		getProfileByIDFunc: func(context.Context, uuid.UUID) (*domain.AffiliateProfile, error) {
			return activeProfileForFraud(42), nil
		},
		getClickByClickIDFunc: func(_ context.Context, clickID string) (*domain.AffiliateClick, error) {
			return &domain.AffiliateClick{
				ClickID:           clickID,
				DeviceFingerprint: "fp-shared",
				IPHash:            "ip-1",
			}, nil
		},
		// 5 distinct users on one device = high severity (default thresholds).
		countReferredUsersByDeviceFunc: func(context.Context, uuid.UUID, string, time.Time) (int64, error) {
			return 5, nil
		},
		createFraudFlagFunc: func(_ context.Context, flag *domain.AffiliateFraudFlag) error {
			flags = append(flags, flag)
			return nil
		},
	}

	attr, err := NewAffiliateService(repo, nil).BindReferredUser(ctx, BindReferredUserInput{
		AffiliateID:    affiliateID,
		ReferredUserID: 77,
		ClickID:        "click-9",
	})
	if err != nil {
		t.Fatalf("binding must succeed despite fraud findings, got %v", err)
	}
	if attr == nil {
		t.Fatal("expected an attribution to be returned")
	}

	if len(flags) != 1 {
		t.Fatalf("expected exactly 1 flag, got %d", len(flags))
	}
	if flags[0].FlagType != "device_sharing" {
		t.Fatalf("expected device_sharing flag, got %q", flags[0].FlagType)
	}
	if flags[0].Severity != domain.FraudSeverityHigh {
		t.Fatalf("expected high severity, got %q", flags[0].Severity)
	}
	if flags[0].Status != domain.FraudFlagStatusOpen {
		t.Fatalf("expected open flag (payout gating), got %q", flags[0].Status)
	}
	if flags[0].ReferredUserID != 77 {
		t.Fatalf("expected referred_user_id 77, got %d", flags[0].ReferredUserID)
	}
	if flags[0].Details["device_fingerprint"] != "fp-shared" {
		t.Fatalf("expected device fingerprint in details, got %+v", flags[0].Details)
	}
}

func TestBindReferredUser_CleanTrafficRaisesNoFlags(t *testing.T) {
	ctx := context.Background()
	var flags []*domain.AffiliateFraudFlag
	repo := &mockAffiliateRepository{
		getProfileByIDFunc: func(context.Context, uuid.UUID) (*domain.AffiliateProfile, error) {
			return activeProfileForFraud(42), nil
		},
		getClickByClickIDFunc: func(_ context.Context, clickID string) (*domain.AffiliateClick, error) {
			return &domain.AffiliateClick{
				ClickID:           clickID,
				DeviceFingerprint: "fp-unique",
				IPHash:            "ip-unique",
			}, nil
		},
		createFraudFlagFunc: func(_ context.Context, flag *domain.AffiliateFraudFlag) error {
			flags = append(flags, flag)
			return nil
		},
	}

	if _, err := NewAffiliateService(repo, nil).BindReferredUser(ctx, BindReferredUserInput{
		AffiliateID:    uuid.New(),
		ReferredUserID: 88,
		ClickID:        "click-1",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(flags) != 0 {
		t.Fatalf("clean traffic must not raise flags, got %d", len(flags))
	}
}

func TestBindReferredUser_FraudEngineFailureDoesNotBlockBinding(t *testing.T) {
	ctx := context.Background()
	repo := &mockAffiliateRepository{
		getProfileByIDFunc: func(context.Context, uuid.UUID) (*domain.AffiliateProfile, error) {
			return activeProfileForFraud(42), nil
		},
		getClickByClickIDFunc: func(context.Context, string) (*domain.AffiliateClick, error) {
			return nil, errors.New("db unavailable")
		},
	}

	attr, err := NewAffiliateService(repo, nil).BindReferredUser(ctx, BindReferredUserInput{
		AffiliateID:    uuid.New(),
		ReferredUserID: 99,
		ClickID:        "click-2",
	})
	if err != nil {
		t.Fatalf("engine failure must not break attribution, got %v", err)
	}
	if attr == nil {
		t.Fatal("attribution must still be persisted")
	}
}

func TestScreenAttribution_CriticalFindingIsNotAutoSuspended(t *testing.T) {
	ctx := context.Background()
	affiliateID := uuid.New()
	var flags []*domain.AffiliateFraudFlag
	repo := &mockAffiliateRepository{
		getProfileByIDFunc: func(context.Context, uuid.UUID) (*domain.AffiliateProfile, error) {
			return activeProfileForFraud(42), nil
		},
		getClickByClickIDFunc: func(_ context.Context, clickID string) (*domain.AffiliateClick, error) {
			return &domain.AffiliateClick{
				ClickID:           clickID,
				DeviceFingerprint: "fp-farm",
				IPHash:            "ip-farm",
			}, nil
		},
		// 12 users on one device = critical (default threshold is 10).
		countReferredUsersByDeviceFunc: func(context.Context, uuid.UUID, string, time.Time) (int64, error) {
			return 12, nil
		},
		createFraudFlagFunc: func(_ context.Context, flag *domain.AffiliateFraudFlag) error {
			flags = append(flags, flag)
			return nil
		},
	}

	svc := NewAffiliateService(repo, nil)
	if _, err := svc.BindReferredUser(ctx, BindReferredUserInput{
		AffiliateID:    affiliateID,
		ReferredUserID: 101,
		ClickID:        "click-3",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(flags) == 0 {
		t.Fatal("expected at least one flag")
	}
	// A critical signal must raise a flag for a human, never silently mutate
	// the affiliate status.
	if flags[0].Severity != domain.FraudSeverityCritical {
		t.Fatalf("expected critical severity, got %q", flags[0].Severity)
	}
	if flags[0].Status != domain.FraudFlagStatusOpen {
		t.Fatalf("expected flag to stay open for review, got %q", flags[0].Status)
	}
}

func TestSetFraudConfig_OverridesThresholds(t *testing.T) {
	ctx := context.Background()
	var flags []*domain.AffiliateFraudFlag
	repo := &mockAffiliateRepository{
		getProfileByIDFunc: func(context.Context, uuid.UUID) (*domain.AffiliateProfile, error) {
			return activeProfileForFraud(42), nil
		},
		getClickByClickIDFunc: func(_ context.Context, clickID string) (*domain.AffiliateClick, error) {
			return &domain.AffiliateClick{
				ClickID:           clickID,
				DeviceFingerprint: "fp-shared",
				IPHash:            "ip-1",
			}, nil
		},
		countReferredUsersByDeviceFunc: func(context.Context, uuid.UUID, string, time.Time) (int64, error) {
			return 3, nil // would be medium at default thresholds
		},
		createFraudFlagFunc: func(_ context.Context, flag *domain.AffiliateFraudFlag) error {
			flags = append(flags, flag)
			return nil
		},
	}

	// Raised threshold silences the same signal.
	cfg := fraud.DefaultConfig()
	cfg.DeviceSharingMedium = 50
	svc := NewAffiliateService(repo, nil)
	svc.SetFraudConfig(cfg)

	if _, err := svc.BindReferredUser(ctx, BindReferredUserInput{
		AffiliateID:    uuid.New(),
		ReferredUserID: 202,
		ClickID:        "click-4",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(flags) != 0 {
		t.Fatalf("raised threshold must silence the finding, got %d flags", len(flags))
	}
}