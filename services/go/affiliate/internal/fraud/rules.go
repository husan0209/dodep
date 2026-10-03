package fraud

import (
	"context"

	"github.com/opus-casino/affiliate/internal/domain"
)

// evalClickVelocity flags bot/click-farm traffic: thousands of clicks per
// hour from one affiliate with (typically) no conversions.
func evalClickVelocity(ctx context.Context, store SignalStore, cfg Config, in Input) (*Finding, error) {
	n, err := store.CountClicksSince(ctx, in.AffiliateID, in.EvaluatedAt.Add(-cfg.ClickVelocityWindow))
	if err != nil {
		return nil, err
	}
	details := map[string]string{
		"rule_id":      RuleClickVelocity,
		"clicks":       itoa(n),
		"window":       cfg.ClickVelocityWindow.String(),
		"affiliate_id": in.AffiliateID.String(),
	}
	switch {
	case n >= cfg.ClicksPerHourHigh:
		return &Finding{RuleID: RuleClickVelocity, Severity: domain.FraudSeverityHigh, Details: details}, nil
	case n >= cfg.ClicksPerHourWarn:
		return &Finding{RuleID: RuleClickVelocity, Severity: domain.FraudSeverityMedium, Details: details}, nil
	default:
		return nil, nil
	}
}

// evalRegistrationVelocity flags abnormal signup bursts per affiliate
// (farmed accounts, incentivized traffic dumps).
func evalRegistrationVelocity(ctx context.Context, store SignalStore, cfg Config, in Input) (*Finding, error) {
	n, err := store.CountAttributionsSince(ctx, in.AffiliateID, in.EvaluatedAt.Add(-cfg.RegistrationVelocityWindow))
	if err != nil {
		return nil, err
	}
	// Include the attribution being evaluated: counters may not see it yet
	// depending on read-your-write guarantees.
	n++
	details := map[string]string{
		"rule_id":       RuleRegistrationVelocity,
		"registrations": itoa(n),
		"window":        cfg.RegistrationVelocityWindow.String(),
		"affiliate_id":  in.AffiliateID.String(),
	}
	switch {
	case n >= cfg.RegistrationsPerDayHigh:
		return &Finding{RuleID: RuleRegistrationVelocity, Severity: domain.FraudSeverityHigh, Details: details}, nil
	case n >= cfg.RegistrationsPerDayWarn:
		return &Finding{RuleID: RuleRegistrationVelocity, Severity: domain.FraudSeverityMedium, Details: details}, nil
	default:
		return nil, nil
	}
}

// evalDeviceSharing flags multi-accounting: several referred users arriving
// from the same device fingerprint. Empty fingerprints are skipped
// (missing signal, not a signal).
func evalDeviceSharing(ctx context.Context, store SignalStore, cfg Config, in Input) (*Finding, error) {
	if in.DeviceFP == "" {
		return nil, nil
	}
	n, err := store.CountReferredUsersByDevice(ctx, in.AffiliateID, in.DeviceFP, in.EvaluatedAt.Add(-cfg.DeviceWindow))
	if err != nil {
		return nil, err
	}
	details := map[string]string{
		"rule_id":            RuleDeviceSharing,
		"device_fingerprint": in.DeviceFP,
		"distinct_users":     itoa(n),
		"window":             cfg.DeviceWindow.String(),
		"affiliate_id":       in.AffiliateID.String(),
	}
	switch {
	case n >= cfg.DeviceSharingCritical:
		return &Finding{RuleID: RuleDeviceSharing, Severity: domain.FraudSeverityCritical, Details: details}, nil
	case n >= cfg.DeviceSharingHigh:
		return &Finding{RuleID: RuleDeviceSharing, Severity: domain.FraudSeverityHigh, Details: details}, nil
	case n >= cfg.DeviceSharingMedium:
		return &Finding{RuleID: RuleDeviceSharing, Severity: domain.FraudSeverityMedium, Details: details}, nil
	default:
		return nil, nil
	}
}

// evalIPCluster flags IP clustering: several referred users behind one IP
// hash (shared VPN exit, click farm, dormitory NAT). Thresholds are higher
// than device sharing because legitimate IP sharing is common.
func evalIPCluster(ctx context.Context, store SignalStore, cfg Config, in Input) (*Finding, error) {
	if in.IPHash == "" {
		return nil, nil
	}
	n, err := store.CountReferredUsersByIP(ctx, in.AffiliateID, in.IPHash, in.EvaluatedAt.Add(-cfg.IPWindow))
	if err != nil {
		return nil, err
	}
	details := map[string]string{
		"rule_id":        RuleIPCluster,
		"ip_hash":        in.IPHash,
		"distinct_users": itoa(n),
		"window":         cfg.IPWindow.String(),
		"affiliate_id":   in.AffiliateID.String(),
	}
	switch {
	case n >= cfg.IPClusterCritical:
		return &Finding{RuleID: RuleIPCluster, Severity: domain.FraudSeverityCritical, Details: details}, nil
	case n >= cfg.IPClusterHigh:
		return &Finding{RuleID: RuleIPCluster, Severity: domain.FraudSeverityHigh, Details: details}, nil
	case n >= cfg.IPClusterMedium:
		return &Finding{RuleID: RuleIPCluster, Severity: domain.FraudSeverityMedium, Details: details}, nil
	default:
		return nil, nil
	}
}

// evalClicklessAttribution flags attributions with no click record
// (direct/injected attribution, cookie-stuffing analog). Low severity:
// offline promo codes and direct deals are legitimate, review only.
func evalClicklessAttribution(_ context.Context, _ SignalStore, _ Config, in Input) (*Finding, error) {
	if in.ClickID != "" {
		return nil, nil
	}
	return &Finding{
		RuleID:   RuleClicklessAttribution,
		Severity: domain.FraudSeverityLow,
		Details: map[string]string{
			"rule_id":          RuleClicklessAttribution,
			"affiliate_id":     in.AffiliateID.String(),
			"referred_user_id": itoa(in.ReferredUserID),
		},
	}, nil
}
