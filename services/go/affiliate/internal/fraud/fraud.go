// Package fraud implements the automatic affiliate anti-fraud engine.
//
// Industry background (iGaming affiliate fraud, SEON/LexisNexis/Fraudlogix
// reports 2024-2026): the dominant vectors are self-referral rings,
// multi-accounting via shared devices (device-hash matching), IP clustering
// (VPN/datacenter/click farms), abnormal click velocity with no conversions,
// and clickless/direct attributions (cookie-stuffing analog). Bonus abuse
// alone accounts for the majority of iGaming fraud cases.
//
// Design principles applied here:
//   - flag-first, block-later: rules create reviewable fraud flags; only the
//     pre-existing self-referral check hard-blocks. Payouts are already gated
//     on open flags, so new findings automatically tighten payout control.
//   - fraud evaluation must never break attribution: engine errors are logged
//     and swallowed by the caller.
//   - thresholds are conservative by default and centralized in Config so a
//     risk manager can tune them without code changes.
package fraud

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/opus-casino/affiliate/internal/domain"
)

// Rule IDs double as fraud flag types stored in affiliate_fraud_flags.
const (
	RuleClickVelocity        = "click_velocity"
	RuleRegistrationVelocity = "registration_velocity"
	RuleDeviceSharing        = "device_sharing"
	RuleIPCluster            = "ip_cluster"
	RuleClicklessAttribution = "clickless_attribution"
)

// Config holds tunable thresholds and observation windows.
// Zero values are replaced by DefaultConfig.
type Config struct {
	// ClickVelocityWindow is the sliding window for click counting.
	ClickVelocityWindow time.Duration
	// ClicksPerHourWarn raises a medium finding, ClicksPerHourHigh a high one.
	ClicksPerHourWarn int64
	ClicksPerHourHigh int64
	// RegistrationVelocityWindow is the sliding window for attribution counting.
	RegistrationVelocityWindow time.Duration
	// RegistrationsPerDayWarn raises medium, RegistrationsPerDayHigh raises high.
	RegistrationsPerDayWarn int64
	RegistrationsPerDayHigh int64
	// DeviceWindow is the lookback for shared-device counting.
	DeviceWindow time.Duration
	// DeviceSharingMedium/High/Critical are distinct-user thresholds.
	DeviceSharingMedium   int64
	DeviceSharingHigh     int64
	DeviceSharingCritical int64
	// IPWindow is the lookback for shared-IP counting.
	IPWindow time.Duration
	// IPClusterMedium/High/Critical are distinct-user thresholds.
	IPClusterMedium   int64
	IPClusterHigh     int64
	IPClusterCritical int64
}

// DefaultConfig returns conservative production starting values.
// False positives are expected (shared household devices, corporate NAT);
// review fraud-flag queues before tightening.
func DefaultConfig() Config {
	return Config{
		ClickVelocityWindow:        time.Hour,
		ClicksPerHourWarn:          200,
		ClicksPerHourHigh:          1000,
		RegistrationVelocityWindow: 24 * time.Hour,
		RegistrationsPerDayWarn:    20,
		RegistrationsPerDayHigh:    50,
		DeviceWindow:               30 * 24 * time.Hour,
		DeviceSharingMedium:        2,
		DeviceSharingHigh:          5,
		DeviceSharingCritical:      10,
		IPWindow:                   30 * 24 * time.Hour,
		IPClusterMedium:            3,
		IPClusterHigh:              10,
		IPClusterCritical:          25,
	}
}

// Input is a single attribution event to evaluate.
type Input struct {
	AffiliateID    uuid.UUID
	ReferredUserID int64
	ClickID        string
	DeviceFP       string
	IPHash         string
	EvaluatedAt    time.Time
}

// Finding is one triggered rule.
type Finding struct {
	RuleID   string
	Severity domain.FraudSeverity
	Details  map[string]string
}

// Verdict aggregates all findings of one evaluation.
type Verdict struct {
	Findings            []Finding
	MaxSeverity         domain.FraudSeverity
	RecommendSuspension bool
}

// SignalStore provides the historical counters rules need.
// Implementations must return (0, nil) on empty data, never fabricated counts.
type SignalStore interface {
	GetClickByClickID(ctx context.Context, clickID string) (*domain.AffiliateClick, error)
	CountClicksSince(ctx context.Context, affiliateID uuid.UUID, since time.Time) (int64, error)
	CountAttributionsSince(ctx context.Context, affiliateID uuid.UUID, since time.Time) (int64, error)
	CountReferredUsersByDevice(ctx context.Context, affiliateID uuid.UUID, deviceFP string, since time.Time) (int64, error)
	CountReferredUsersByIP(ctx context.Context, affiliateID uuid.UUID, ipHash string, since time.Time) (int64, error)
}

// Evaluate runs all rules against one attribution and returns the verdict.
// A nil verdict with nil error means "clean".
func Evaluate(ctx context.Context, store SignalStore, cfg Config, in Input) (*Verdict, error) {
	if in.EvaluatedAt.IsZero() {
		in.EvaluatedAt = time.Now().UTC()
	}

	// Resolve device/IP from the click when the caller did not provide them.
	if (in.DeviceFP == "" || in.IPHash == "") && in.ClickID != "" {
		click, err := store.GetClickByClickID(ctx, in.ClickID)
		if err != nil {
			return nil, err
		}
		if click != nil {
			if in.DeviceFP == "" {
				in.DeviceFP = click.DeviceFingerprint
			}
			if in.IPHash == "" {
				in.IPHash = click.IPHash
			}
		}
	}

	var findings []Finding
	rules := []func(context.Context, SignalStore, Config, Input) (*Finding, error){
		evalClickVelocity,
		evalRegistrationVelocity,
		evalDeviceSharing,
		evalIPCluster,
		evalClicklessAttribution,
	}
	for _, rule := range rules {
		f, err := rule(ctx, store, cfg, in)
		if err != nil {
			return nil, err
		}
		if f != nil {
			findings = append(findings, *f)
		}
	}

	if len(findings) == 0 {
		return nil, nil
	}
	v := &Verdict{Findings: findings}
	for _, f := range findings {
		if severityRank(f.Severity) > severityRank(v.MaxSeverity) {
			v.MaxSeverity = f.Severity
		}
		if f.Severity == domain.FraudSeverityCritical {
			v.RecommendSuspension = true
		}
	}
	return v, nil
}

func severityRank(s domain.FraudSeverity) int {
	switch s {
	case domain.FraudSeverityCritical:
		return 4
	case domain.FraudSeverityHigh:
		return 3
	case domain.FraudSeverityMedium:
		return 2
	case domain.FraudSeverityLow:
		return 1
	default:
		return 0
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
