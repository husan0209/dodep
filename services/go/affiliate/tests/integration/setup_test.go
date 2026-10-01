package integration

import (
	"context"
	"os"
	"testing"

	"github.com/opus-casino/affiliate/internal/repository"
	"github.com/opus-casino/affiliate/internal/service"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// testDBName is created on demand so parallel agents' databases don't clash.
const testDBName = "affiliate_test"

func testDSN(dbname string) string {
	if dsn := os.Getenv("AFFILIATE_TEST_DATABASE_URL"); dsn != "" {
		return dsn
	}
	return "postgres://postgres:postgres@localhost:5432/" + dbname + "?sslmode=disable"
}

func ensureTestDB(t *testing.T) {
	t.Helper()
	admin, err := gorm.Open(postgres.Open(testDSN("postgres")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Skipf("PostgreSQL not available, skipping integration test: %v", err)
	}
	sqlDB, err := admin.DB()
	if err != nil {
		t.Skipf("PostgreSQL not available, skipping integration test: %v", err)
	}
	defer sqlDB.Close()

	var exists bool
	if err := admin.Raw(
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = ?)", testDBName,
	).Scan(&exists).Error; err != nil {
		t.Skipf("PostgreSQL not available, skipping integration test: %v", err)
	}
	if !exists {
		// CREATE DATABASE cannot run in a transaction; gorm Exec is fine.
		if err := admin.Exec("CREATE DATABASE " + testDBName).Error; err != nil {
			t.Skipf("cannot create test database, skipping: %v", err)
		}
	}
}

func applyMigrations(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, name := range []string{
		"../../migrations/001_create_affiliate_tables.up.sql",
		"../../migrations/002_affiliate_schema_fixes.up.sql",
	} {
		sql, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("cannot read migration %s: %v", name, err)
		}
		if err := db.Exec(string(sql)).Error; err != nil {
			t.Fatalf("cannot apply migration %s: %v", name, err)
		}
	}
}

func truncateAll(t *testing.T, db *gorm.DB) {
	t.Helper()
	tables := []string{
		"affiliate_outbox",
		"affiliate_ledger_entries",
		"affiliate_ledger_accounts",
		"affiliate_fraud_flags",
		"affiliate_payouts",
		"affiliate_payout_methods",
		"affiliate_adjustments",
		"affiliate_earnings",
		"affiliate_daily_aggregates",
		"affiliate_attributions",
		"affiliate_clicks",
		"affiliate_links",
		"affiliate_profiles",
		"affiliate_enrollment_requests",
		// NOTE: affiliate_commission_plans is seed/reference data
		// (Default RevShare 20) and must survive cleanup.
	}
	for _, tbl := range tables {
		// Missing tables (older migration set) must not fail the cleanup.
		db.Exec("TRUNCATE " + tbl + " CASCADE")
	}
}

// setupService returns a service bound to a migrated test database.
// Tests are skipped (not failed) when Postgres is unavailable so that
// `go test ./...` stays green on machines without a local database.
func setupService(t *testing.T) (*service.AffiliateService, context.Context) {
	t.Helper()
	ensureTestDB(t)

	db, err := gorm.Open(postgres.Open(testDSN(testDBName)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Skipf("PostgreSQL not available, skipping integration test: %v", err)
	}

	applyMigrations(t, db)
	truncateAll(t, db)
	t.Cleanup(func() {
		truncateAll(t, db)
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})

	repo := repository.NewGormAffiliateRepository(db, zap.NewNop())
	svc := service.NewAffiliateService(repo, zap.NewNop())
	return svc, context.Background()
}

// execTestSQL runs a raw statement against the test database. Used to
// simulate out-of-band changes when testing reconciliation.
func execTestSQL(t *testing.T, query string, args ...any) error {
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
	return db.Exec(query, args...).Error
}

func mustEnrollAndApprove(
	t *testing.T,
	ctx context.Context,
	svc *service.AffiliateService,
	userID int64,
) {
	t.Helper()
	if _, err := svc.EnrollAffiliate(ctx, service.EnrollAffiliateInput{
		UserID: userID,
		Reason: "integration test",
	}); err != nil {
		t.Fatalf("enroll failed: %v", err)
	}
	if _, err := svc.ApproveAffiliate(ctx, service.ApproveAffiliateInput{
		UserID:          userID,
		ApprovedBy:      "admin@example.com",
		CommissionRate:  decimal.RequireFromString("0.20"),
		HoldPeriodDays:  14,
		MinPayoutAmount: decimal.RequireFromString("100"),
		Currency:        "USD",
	}); err != nil {
		t.Fatalf("approve failed: %v", err)
	}
}
