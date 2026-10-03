package domain

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestExclusionPeriods(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		period ExclusionPeriod
		min    time.Duration
		max    time.Duration
	}{
		{Exclusion24H, 23 * time.Hour, 25 * time.Hour},
		{Exclusion7D, 6*24*time.Hour + 23*time.Hour, 7*24*time.Hour + time.Hour},
		{Exclusion30D, 29 * 24 * time.Hour, 31 * 24 * time.Hour},
	}
	for _, tc := range cases {
		until, ok := tc.period.Duration(now)
		if !ok {
			t.Fatalf("%s should resolve", tc.period)
		}
		d := until.Sub(now)
		if d < tc.min || d > tc.max {
			t.Errorf("%s duration %v out of range", tc.period, d)
		}
	}
	if _, ok := ExclusionPermanent.Duration(now); ok {
		t.Error("permanent must not resolve to a duration")
	}
	if _, ok := ExclusionPeriod("2y").Duration(now); ok {
		t.Error("unknown period must not resolve")
	}
	for _, p := range []ExclusionPeriod{Exclusion24H, Exclusion7D, Exclusion30D, Exclusion6M, Exclusion1Y, ExclusionPermanent} {
		if !p.IsValid() {
			t.Errorf("%s should be valid", p)
		}
	}
	six, _ := Exclusion6M.Duration(now)
	if six.Sub(now) < 5*30*24*time.Hour {
		t.Error("6m must span roughly six months (GAMSTOP minimum)")
	}
}

func TestTimeoutPeriods(t *testing.T) {
	now := time.Now().UTC()
	for _, p := range []TimeoutPeriod{Timeout24H, Timeout48H, Timeout7D, Timeout30D} {
		if !p.IsValid() {
			t.Errorf("%s should be valid", p)
		}
		until, ok := p.Duration(now)
		if !ok || !until.After(now) {
			t.Errorf("%s should resolve to the future", p)
		}
	}
	if TimeoutPeriod("1y").IsValid() {
		t.Error("1y is not a timeout period")
	}
}

func TestExclusionStateMachine(t *testing.T) {
	if !ExclusionActive.CanTransitionTo(ExclusionExpired, false) {
		t.Error("active -> expired must be allowed")
	}
	if !ExclusionActive.CanTransitionTo(ExclusionRevoked, false) {
		t.Error("active -> revoked must be allowed")
	}
	if !ExclusionExpired.CanTransitionTo(ExclusionRevoked, false) {
		t.Error("expired -> revoked must be allowed")
	}
	if ExclusionActive.CanTransitionTo(ExclusionActive, false) {
		t.Error("self-transition must be blocked")
	}
	if ExclusionRevoked.CanTransitionTo(ExclusionActive, false) {
		t.Error("revoked is terminal")
	}
	for _, s := range []ExclusionStatus{ExclusionActive, ExclusionExpired, ExclusionRevoked} {
		if s.CanTransitionTo(ExclusionExpired, true) {
			t.Errorf("permanent exclusion must never transition (from %s)", s)
		}
	}
}

func TestExclusionIsActiveAt(t *testing.T) {
	now := time.Now().UTC()
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)

	active := &Exclusion{Status: ExclusionActive, Until: &future}
	if !active.IsActiveAt(now) {
		t.Error("future temporary exclusion must be active")
	}
	lapsed := &Exclusion{Status: ExclusionActive, Until: &past}
	if lapsed.IsActiveAt(now) {
		t.Error("lapsed exclusion must be inactive")
	}
	perm := &Exclusion{Status: ExclusionActive, Permanent: true}
	if !perm.IsActiveAt(now) {
		t.Error("permanent exclusion must always be active")
	}
	revoked := &Exclusion{Status: ExclusionRevoked, Permanent: true}
	if revoked.IsActiveAt(now) {
		t.Error("revoked exclusion must be inactive")
	}
	var nilExcl *Exclusion
	if nilExcl.IsActiveAt(now) {
		t.Error("nil exclusion must be inactive")
	}

	to := &Timeout{Until: future}
	if !to.IsActiveAt(now) {
		t.Error("future timeout must be active")
	}
	var nilTo *Timeout
	if nilTo.IsActiveAt(now) {
		t.Error("nil timeout must be inactive")
	}
}

func TestLimitsGetSet(t *testing.T) {
	l := &RGLimits{UserID: 7}
	if got := l.Get(LimitDepositDaily); !got.IsZero() {
		t.Error("unset limit must read zero")
	}
	l.Set(LimitDepositDaily, decimal.NewFromInt(500))
	if got := l.Get(LimitDepositDaily); got.String() != "500" {
		t.Errorf("bad round trip: %s", got)
	}
	l.Set(LimitSessionMinutes, decimal.NewFromInt(90))
	if l.SessionMinutes != 90 {
		t.Errorf("bad minutes: %d", l.SessionMinutes)
	}
	if got := l.EffectiveRealityCheck(); got != DefaultRealityCheckMinutes {
		t.Errorf("default reality check must be %d, got %d", DefaultRealityCheckMinutes, got)
	}
	l.Set(LimitRealityCheck, decimal.NewFromInt(30))
	if got := l.EffectiveRealityCheck(); got != 30 {
		t.Errorf("configured reality check must win, got %d", got)
	}
	var nilLimits *RGLimits
	if got := nilLimits.EffectiveRealityCheck(); got != DefaultRealityCheckMinutes {
		t.Error("nil limits must yield default reality check")
	}
}

func TestLimitTypeKinds(t *testing.T) {
	money := []LimitType{LimitDepositDaily, LimitDepositWeekly, LimitDepositMonthly,
		LimitLossDaily, LimitLossWeekly, LimitLossMonthly, LimitWagerDaily, LimitWagerWeekly}
	for _, m := range money {
		if !m.IsMoney() || !m.IsValid() {
			t.Errorf("%s must be a valid money limit", m)
		}
	}
	for _, m := range []LimitType{LimitSessionMinutes, LimitRealityCheck} {
		if m.IsMoney() || !m.IsValid() {
			t.Errorf("%s must be a valid minutes limit", m)
		}
	}
	if LimitType("nope").IsValid() {
		t.Error("unknown limit type must be invalid")
	}
	if len(AllLimitTypes()) != 10 {
		t.Errorf("expected 10 limit types, got %d", len(AllLimitTypes()))
	}
}

func TestPendingChangeIsDue(t *testing.T) {
	now := time.Now().UTC()
	due := &PendingChange{Status: ChangePending, EffectiveAt: now.Add(-time.Minute)}
	if !due.IsDue(now) {
		t.Error("past effective_at must be due")
	}
	waiting := &PendingChange{Status: ChangePending, EffectiveAt: now.Add(time.Hour)}
	if waiting.IsDue(now) {
		t.Error("future effective_at must not be due")
	}
	applied := &PendingChange{Status: ChangeApplied, EffectiveAt: now.Add(-time.Hour)}
	if applied.IsDue(now) {
		t.Error("applied change must never be due")
	}
}

func TestChannels(t *testing.T) {
	for _, c := range []Channel{ChannelBet, ChannelGameLaunch, ChannelDeposit, ChannelWithdrawal} {
		if !c.IsValid() {
			t.Errorf("%s must be valid", c)
		}
	}
	if Channel("slot").IsValid() {
		t.Error("unknown channel must be invalid")
	}
}
