package integration

import (
	"testing"

	"github.com/google/uuid"
	"github.com/opus-casino/affiliate/internal/domain"
	"github.com/shopspring/decimal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// insertPlan seeds a commission plan row directly: the repository exposes
// no create-plan API (plans are managed via migrations/seeds in production).
func insertPlan(
	t *testing.T,
	id uuid.UUID,
	name, rate string,
	holdDays int,
	active bool,
) {
	t.Helper()
	db, err := gorm.Open(postgres.Open(testDSN(testDBName)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Skipf("PostgreSQL not available, skipping integration test: %v", err)
	}
	sqlDB, _ := db.DB()
	defer func() {
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	}()
	if err := db.Exec(`
		INSERT INTO affiliate_commission_plans
		    (id, name, commission_type, commission_rate, hold_period_days,
		     min_payout_amount, approval_mode, payout_schedule,
		     negative_carryover_enabled, is_default, is_active)
		VALUES (?, ?, 'revshare', ?::numeric, ?, 100, 'manual', 'monthly', FALSE, FALSE, ?)
		ON CONFLICT (id) DO UPDATE SET
		    commission_rate = EXCLUDED.commission_rate,
		    hold_period_days = EXCLUDED.hold_period_days,
		    is_active = EXCLUDED.is_active
	`, id, name, rate, holdDays, active).Error; err != nil {
		t.Fatalf("seed plan failed: %v", err)
	}
}

func TestChangeCommissionPlan(t *testing.T) {
	svc, ctx := setupService(t)

	const userID int64 = 4004
	mustEnrollAndApprove(t, ctx, svc, userID)

	profile, err := svc.GetProfileByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("get profile failed: %v", err)
	}
	if !profile.CommissionRate.Equal(decimal.RequireFromString("0.20")) {
		t.Fatalf("expected default rate 0.20, got %s", profile.CommissionRate)
	}

	vipPlanID := uuid.New()
	insertPlan(t, vipPlanID, "VIP RevShare 50", "0.50", 30, true)

	updated, err := svc.ChangeCommissionPlan(ctx, profile.ID, vipPlanID)
	if err != nil {
		t.Fatalf("change plan failed: %v", err)
	}
	if updated.CommissionPlanID != vipPlanID {
		t.Fatalf("expected plan %s, got %s", vipPlanID, updated.CommissionPlanID)
	}
	if !updated.CommissionRate.Equal(decimal.RequireFromString("0.50")) {
		t.Fatalf("expected rate 0.50, got %s", updated.CommissionRate)
	}
	if updated.HoldPeriodDays != 30 {
		t.Fatalf("expected hold 30 days, got %d", updated.HoldPeriodDays)
	}

	// Unknown plan -> clear domain error, not an FK crash.
	if _, err := svc.ChangeCommissionPlan(ctx, profile.ID, uuid.New()); err != domain.ErrCommissionPlanNotFound {
		t.Fatalf("expected ErrCommissionPlanNotFound, got %v", err)
	}

	// Inactive plan cannot be assigned.
	deadPlanID := uuid.New()
	insertPlan(t, deadPlanID, "Retired", "0.10", 7, false)
	if _, err := svc.ChangeCommissionPlan(ctx, profile.ID, deadPlanID); err != domain.ErrInvalidCommissionAmount {
		t.Fatalf("expected ErrInvalidCommissionAmount for inactive plan, got %v", err)
	}

	// Unknown affiliate -> NotFound.
	if _, err := svc.ChangeCommissionPlan(ctx, uuid.New(), vipPlanID); err != domain.ErrAffiliateNotFound {
		t.Fatalf("expected ErrAffiliateNotFound, got %v", err)
	}

	plans, err := svc.ListCommissionPlans(ctx)
	if err != nil {
		t.Fatalf("list plans failed: %v", err)
	}
	if len(plans) < 3 { // seeded default + 2 inserted above
		t.Fatalf("expected at least 3 plans, got %d", len(plans))
	}
}
