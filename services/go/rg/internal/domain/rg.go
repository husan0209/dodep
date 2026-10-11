package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Channel is the gambling action being gated.
type Channel string

const (
	ChannelBet        Channel = "bet"
	ChannelGameLaunch Channel = "game_launch"
	ChannelDeposit    Channel = "deposit"
	ChannelWithdrawal Channel = "withdrawal"
)

func (c Channel) IsValid() bool {
	switch c {
	case ChannelBet, ChannelGameLaunch, ChannelDeposit, ChannelWithdrawal:
		return true
	}
	return false
}

// ReasonCode explains an enforcement decision. Withdrawals never carry
// a blocking reason.
type ReasonCode string

const (
	ReasonOK           ReasonCode = "ok"
	ReasonSelfExcluded ReasonCode = "RG_SELF_EXCLUDED"
	ReasonCoolOff      ReasonCode = "RG_COOL_OFF"
	ReasonDepositLimit ReasonCode = "RG_DEPOSIT_LIMIT"
	ReasonLossLimit    ReasonCode = "RG_LOSS_LIMIT"
	ReasonWagerLimit   ReasonCode = "RG_WAGER_LIMIT"
	ReasonSessionLimit ReasonCode = "RG_SESSION_LIMIT"
)

// LimitType identifies a single tunable control.
type LimitType string

const (
	LimitDepositDaily   LimitType = "deposit_daily"
	LimitDepositWeekly  LimitType = "deposit_weekly"
	LimitDepositMonthly LimitType = "deposit_monthly"
	LimitLossDaily      LimitType = "loss_daily"
	LimitLossWeekly     LimitType = "loss_weekly"
	LimitLossMonthly    LimitType = "loss_monthly"
	LimitWagerDaily     LimitType = "wager_daily"
	LimitWagerWeekly    LimitType = "wager_weekly"
	LimitSessionMinutes LimitType = "session_minutes"
	LimitRealityCheck   LimitType = "reality_check_minutes"
)

// IsMoney reports whether the limit is a monetary amount (vs minutes).
func (t LimitType) IsMoney() bool {
	switch t {
	case LimitDepositDaily, LimitDepositWeekly, LimitDepositMonthly,
		LimitLossDaily, LimitLossWeekly, LimitLossMonthly,
		LimitWagerDaily, LimitWagerWeekly:
		return true
	}
	return false
}

func (t LimitType) IsValid() bool {
	switch t {
	case LimitDepositDaily, LimitDepositWeekly, LimitDepositMonthly,
		LimitLossDaily, LimitLossWeekly, LimitLossMonthly,
		LimitWagerDaily, LimitWagerWeekly,
		LimitSessionMinutes, LimitRealityCheck:
		return true
	}
	return false
}

// AllLimitTypes enumerates every tunable control.
func AllLimitTypes() []LimitType {
	return []LimitType{
		LimitDepositDaily, LimitDepositWeekly, LimitDepositMonthly,
		LimitLossDaily, LimitLossWeekly, LimitLossMonthly,
		LimitWagerDaily, LimitWagerWeekly,
		LimitSessionMinutes, LimitRealityCheck,
	}
}

// DefaultRealityCheckMinutes is the platform default reality-check interval
// applied when the player never configured one (0 = unset).
const DefaultRealityCheckMinutes = 60

// ExclusionPeriod is the requested self-exclusion length.
type ExclusionPeriod string

const (
	Exclusion24H       ExclusionPeriod = "24h"
	Exclusion7D        ExclusionPeriod = "7d"
	Exclusion30D       ExclusionPeriod = "30d"
	Exclusion6M        ExclusionPeriod = "6m" // GAMSTOP minimum for multi-operator sync
	Exclusion1Y        ExclusionPeriod = "1y"
	ExclusionPermanent ExclusionPeriod = "permanent"
)

func (p ExclusionPeriod) IsValid() bool {
	switch p {
	case Exclusion24H, Exclusion7D, Exclusion30D, Exclusion6M, Exclusion1Y, ExclusionPermanent:
		return true
	}
	return false
}

// Duration resolves the period to a length. Permanent returns ok=false.
func (p ExclusionPeriod) Duration(now time.Time) (until time.Time, ok bool) {
	switch p {
	case Exclusion24H:
		return now.Add(24 * time.Hour), true
	case Exclusion7D:
		return now.Add(7 * 24 * time.Hour), true
	case Exclusion30D:
		return now.Add(30 * 24 * time.Hour), true
	case Exclusion6M:
		return now.AddDate(0, 6, 0), true
	case Exclusion1Y:
		return now.AddDate(1, 0, 0), true
	default:
		return time.Time{}, false
	}
}

// TimeoutPeriod is the cooling-off length (shorter than exclusions).
type TimeoutPeriod string

const (
	Timeout24H TimeoutPeriod = "24h"
	Timeout48H TimeoutPeriod = "48h"
	Timeout7D  TimeoutPeriod = "7d"
	Timeout30D TimeoutPeriod = "30d"
)

func (p TimeoutPeriod) IsValid() bool {
	switch p {
	case Timeout24H, Timeout48H, Timeout7D, Timeout30D:
		return true
	}
	return false
}

// Duration resolves the time-out length.
func (p TimeoutPeriod) Duration(now time.Time) (time.Time, bool) {
	switch p {
	case Timeout24H:
		return now.Add(24 * time.Hour), true
	case Timeout48H:
		return now.Add(48 * time.Hour), true
	case Timeout7D:
		return now.Add(7 * 24 * time.Hour), true
	case Timeout30D:
		return now.Add(30 * 24 * time.Hour), true
	default:
		return time.Time{}, false
	}
}

// ExclusionType is who imposed the exclusion.
type ExclusionType string

const (
	ExclusionSelf       ExclusionType = "self"
	ExclusionOperator   ExclusionType = "operator"
	ExclusionRegulatory ExclusionType = "regulatory"
)

// ExclusionStatus tracks the exclusion lifecycle.
type ExclusionStatus string

const (
	ExclusionActive  ExclusionStatus = "active"
	ExclusionExpired ExclusionStatus = "expired"
	ExclusionRevoked ExclusionStatus = "revoked"
)

// CanTransitionTo enforces the exclusion state machine. Permanent
// exclusions never transition out via any API.
func (s ExclusionStatus) CanTransitionTo(target ExclusionStatus, permanent bool) bool {
	if permanent {
		return false
	}
	switch s {
	case ExclusionActive:
		return target == ExclusionExpired || target == ExclusionRevoked
	case ExclusionExpired:
		return target == ExclusionRevoked
	default:
		return false
	}
}

// ChangeStatus tracks a pending limit increase.
type ChangeStatus string

const (
	ChangePending   ChangeStatus = "pending"
	ChangeApplied   ChangeStatus = "applied"
	ChangeCancelled ChangeStatus = "cancelled"
)

// RGLimits is the effective protection profile. Zero money values and
// zero minutes mean "unset" (no control).
type RGLimits struct {
	UserID              int64
	DepositDaily        decimal.Decimal
	DepositWeekly       decimal.Decimal
	DepositMonthly      decimal.Decimal
	LossDaily           decimal.Decimal
	LossWeekly          decimal.Decimal
	LossMonthly         decimal.Decimal
	WagerDaily          decimal.Decimal
	WagerWeekly         decimal.Decimal
	SessionMinutes      int
	RealityCheckMinutes int
	UpdatedAt           time.Time
}

// EffectiveRealityCheck returns the configured interval or the default.
func (l *RGLimits) EffectiveRealityCheck() int {
	if l == nil || l.RealityCheckMinutes <= 0 {
		return DefaultRealityCheckMinutes
	}
	return l.RealityCheckMinutes
}

// Get returns the effective value of one limit type.
func (l *RGLimits) Get(t LimitType) decimal.Decimal {
	switch t {
	case LimitDepositDaily:
		return l.DepositDaily
	case LimitDepositWeekly:
		return l.DepositWeekly
	case LimitDepositMonthly:
		return l.DepositMonthly
	case LimitLossDaily:
		return l.LossDaily
	case LimitLossWeekly:
		return l.LossWeekly
	case LimitLossMonthly:
		return l.LossMonthly
	case LimitWagerDaily:
		return l.WagerDaily
	case LimitWagerWeekly:
		return l.WagerWeekly
	case LimitSessionMinutes:
		return decimal.NewFromInt(int64(l.SessionMinutes))
	case LimitRealityCheck:
		return decimal.NewFromInt(int64(l.EffectiveRealityCheck()))
	default:
		return decimal.Zero
	}
}

// Set assigns one limit type. Values are stored as decimal (minutes as integer).
func (l *RGLimits) Set(t LimitType, v decimal.Decimal) {
	switch t {
	case LimitDepositDaily:
		l.DepositDaily = v
	case LimitDepositWeekly:
		l.DepositWeekly = v
	case LimitDepositMonthly:
		l.DepositMonthly = v
	case LimitLossDaily:
		l.LossDaily = v
	case LimitLossWeekly:
		l.LossWeekly = v
	case LimitLossMonthly:
		l.LossMonthly = v
	case LimitWagerDaily:
		l.WagerDaily = v
	case LimitWagerWeekly:
		l.WagerWeekly = v
	case LimitSessionMinutes:
		l.SessionMinutes = int(v.IntPart())
	case LimitRealityCheck:
		l.RealityCheckMinutes = int(v.IntPart())
	}
	l.UpdatedAt = time.Now().UTC()
}

// PendingChange is a limit increase waiting out the cooling period.
type PendingChange struct {
	ID          uuid.UUID
	UserID      int64
	LimitType   LimitType
	OldValue    decimal.Decimal
	NewValue    decimal.Decimal
	Status      ChangeStatus
	RequestedAt time.Time
	EffectiveAt time.Time
}

// IsDue reports whether the cooling period has elapsed.
func (c *PendingChange) IsDue(now time.Time) bool {
	return c.Status == ChangePending && !now.Before(c.EffectiveAt)
}

// Exclusion is one self-/operator-/regulatory exclusion record.
type Exclusion struct {
	ID        uuid.UUID
	UserID    int64
	Type      ExclusionType
	Status    ExclusionStatus
	Until     *time.Time // nil = permanent
	Permanent bool
	CreatedBy string
	CreatedAt time.Time
	RevokedAt *time.Time
	RevokedBy string
}

// IsActiveAt reports whether the exclusion blocks gambling at time now.
// Expired rows stay in the table as an immutable audit trail.
func (e *Exclusion) IsActiveAt(now time.Time) bool {
	if e == nil || e.Status != ExclusionActive {
		return false
	}
	if e.Permanent || e.Until == nil {
		return true
	}
	return now.Before(*e.Until)
}

// Timeout is a short cooling-off record.
type Timeout struct {
	ID        uuid.UUID
	UserID    int64
	Until     time.Time
	CreatedAt time.Time
}

// IsActiveAt reports whether the time-out blocks gambling at time now.
func (t *Timeout) IsActiveAt(now time.Time) bool {
	if t == nil {
		return false
	}
	return now.Before(t.Until)
}

// SpendDay aggregates settled money movement per calendar day (UTC).
// Net gambling loss over a window = SUM(wagers) - SUM(payouts).
type SpendDay struct {
	UserID   int64
	Day      time.Time // truncated to UTC midnight
	Deposits decimal.Decimal
	Wagers   decimal.Decimal
	Payouts  decimal.Decimal
}

// Decision is the enforcement outcome.
type Decision struct {
	Allowed   bool
	Reason    ReasonCode
	Message   string
	Remaining *decimal.Decimal // headroom of the binding limit, if any
}
