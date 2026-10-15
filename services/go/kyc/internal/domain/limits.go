package domain

import (
	"strings"
	"time"
)

// ─── Custom (VIP) limit overrides ───
//
// Base tiers come from LimitsForLevel. Negotiated VIP terms are stored per
// user in kyc_limit_overrides (see services/go/kyc/migrations/001_kyc.up.sql)
// and applied on top: every set (non-nil) field wins over the base tier,
// every nil field inherits it. Amounts are decimal WHOLE-UNIT strings —
// NEVER float (CONVENTIONS NEVER-6) — validated by isDecimalUnits.
//
// Writes are admin-only (enforced in service.UpsertLimitOverride: the caller
// role must be admin, mirroring the JWT is_admin claim in middleware.go).

// AuditLimitOverride is appended to kyc_audit_log on every override upsert.
const AuditLimitOverride = "limits.override_upserted"

// LimitOverride is one user's custom terms. Nil amount/flag = inherit base.
type LimitOverride struct {
	UserID            int64
	DailyDeposit      *string
	DailyWithdrawal   *string
	MonthlyDeposit    *string
	MonthlyWithdrawal *string
	MaxBet            *string
	CanTradeCrypto    *bool
	CanUseBonus       *bool
	UpdatedBy         string // admin identity, required
	UpdatedAt         time.Time
}

// Validate checks the override before persistence.
func (o *LimitOverride) Validate() error {
	if o == nil {
		return NewValidationError(FieldError{Field: "override", Message: "override is required"})
	}
	var fields []FieldError
	if o.UserID <= 0 {
		fields = append(fields, FieldError{Field: "user_id", Message: "must be positive"})
	}
	if strings.TrimSpace(o.UpdatedBy) == "" {
		fields = append(fields, FieldError{Field: "updated_by", Message: "admin identity is required"})
	} else if len(o.UpdatedBy) > 128 {
		fields = append(fields, FieldError{Field: "updated_by", Message: "must be at most 128 characters"})
	}
	for _, f := range []struct {
		name  string
		value *string
	}{
		{"daily_deposit", o.DailyDeposit},
		{"daily_withdrawal", o.DailyWithdrawal},
		{"monthly_deposit", o.MonthlyDeposit},
		{"monthly_withdrawal", o.MonthlyWithdrawal},
		{"max_bet", o.MaxBet},
	} {
		if f.value != nil && !isDecimalUnits(*f.value) {
			fields = append(fields, FieldError{Field: f.name, Message: "must be whole units, digits only (no float, no sign)"})
		}
	}
	if len(fields) > 0 {
		return NewValidationError(fields...)
	}
	return nil
}

// isDecimalUnits reports whether s is a non-negative whole-unit amount:
// 1-18 digits, nothing else. Rejects floats ("10.5"), exponents ("1e6"),
// signs ("-5", "+5") and blanks.
func isDecimalUnits(s string) bool {
	if len(s) == 0 || len(s) > 18 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ApplyOverride merges an override over base limits. Nil-safe: a nil
// override returns the base untouched.
func ApplyOverride(base Limits, o *LimitOverride) Limits {
	if o == nil {
		return base
	}
	out := base
	if o.DailyDeposit != nil {
		out.DailyDeposit = *o.DailyDeposit
	}
	if o.DailyWithdrawal != nil {
		out.DailyWithdrawal = *o.DailyWithdrawal
	}
	if o.MonthlyDeposit != nil {
		out.MonthlyDeposit = *o.MonthlyDeposit
	}
	if o.MonthlyWithdrawal != nil {
		out.MonthlyWithdrawal = *o.MonthlyWithdrawal
	}
	if o.MaxBet != nil {
		out.MaxBet = *o.MaxBet
	}
	if o.CanTradeCrypto != nil {
		out.CanTradeCrypto = *o.CanTradeCrypto
	}
	if o.CanUseBonus != nil {
		out.CanUseBonus = *o.CanUseBonus
	}
	return out
}

// EffectiveLimits returns the limits in force for a level with an optional
// custom override applied.
func EffectiveLimits(level KYCLevel, o *LimitOverride) Limits {
	return ApplyOverride(LimitsForLevel(level), o)
}
