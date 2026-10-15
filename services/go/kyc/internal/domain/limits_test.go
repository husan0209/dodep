package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strptr(s string) *string { return &s }

func TestLimitOverride_Validate(t *testing.T) {
	t.Parallel()
	valid := &LimitOverride{UserID: 1, DailyDeposit: strptr("100000"), UpdatedBy: "admin-1", UpdatedAt: time.Now()}
	require.NoError(t, valid.Validate())

	cases := []struct {
		name   string
		mutate func(*LimitOverride)
		field  string
	}{
		{"zero user", func(o *LimitOverride) { o.UserID = 0 }, "user_id"},
		{"missing admin", func(o *LimitOverride) { o.UpdatedBy = "  " }, "updated_by"},
		{"float deposit", func(o *LimitOverride) { o.DailyDeposit = strptr("10.5") }, "daily_deposit"},
		{"exponent deposit", func(o *LimitOverride) { o.DailyDeposit = strptr("1e6") }, "daily_deposit"},
		{"negative withdrawal", func(o *LimitOverride) { o.DailyWithdrawal = strptr("-5") }, "daily_withdrawal"},
		{"blank max bet", func(o *LimitOverride) { o.MaxBet = strptr("") }, "max_bet"},
		{"oversized amount", func(o *LimitOverride) { o.MaxBet = strptr(strings.Repeat("9", 19)) }, "max_bet"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o := &LimitOverride{UserID: 1, DailyDeposit: strptr("100000"), UpdatedBy: "admin-1"}
			tc.mutate(o)
			err := o.Validate()
			require.Error(t, err)
			var ve *ValidationError
			require.ErrorAs(t, err, &ve)
			found := false
			for _, f := range ve.Fields {
				if f.Field == tc.field {
					found = true
				}
			}
			assert.True(t, found, "expected field %q in %v", tc.field, ve.Fields)
		})
	}
}

func TestApplyOverride_Merge(t *testing.T) {
	t.Parallel()
	base := LimitsForLevel(LevelEnhanced)

	assert.Equal(t, base, ApplyOverride(base, nil), "nil override must return base untouched")

	custom := "999999"
	no := false
	got := ApplyOverride(base, &LimitOverride{
		DailyDeposit: &custom, CanTradeCrypto: &no,
	})
	assert.Equal(t, "999999", got.DailyDeposit)
	assert.False(t, got.CanTradeCrypto)
	assert.Equal(t, base.DailyWithdrawal, got.DailyWithdrawal, "unset fields inherit base")
	assert.Equal(t, base.MonthlyDeposit, got.MonthlyDeposit)
	assert.Equal(t, base.MaxBet, got.MaxBet)
	assert.Equal(t, base.CanUseBonus, got.CanUseBonus)
}

func TestEffectiveLimits(t *testing.T) {
	t.Parallel()
	assert.Equal(t, LimitsForLevel(LevelVIP), EffectiveLimits(LevelVIP, nil))
	custom := "1"
	assert.Equal(t, "1", EffectiveLimits(LevelNone, &LimitOverride{MaxBet: &custom}).MaxBet)
}
