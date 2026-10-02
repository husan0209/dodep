// Package integration contains database-backed tests. Every test here is
// skipped unless TEST_DATABASE_URL points at a reachable PostgreSQL, so `go
// test ./...` stays green on machines and CI runners without a database.
//
//	createdb opus_casino_test
//	TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/opus_casino_test?sslmode=disable' \
//	  go test ./tests/integration/...
package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opus-casino/affiliate/internal/repository"
)

// testDSN returns the DSN from the environment, skipping the test when unset.
func testDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping database-backed test")
	}
	return dsn
}

// applyMigrations runs the affiliate schema migration (021) against the test
// database. The file is idempotent, so re-running is safe.
func applyMigrations(t *testing.T, db *gorm.DB) {
	t.Helper()
	// tests/integration -> affiliate -> go -> services -> repo root
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "..", "libs", "migrations", "postgresql", "021_affiliates.sql"))
	if err != nil {
		t.Fatalf("resolve migration path: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration %s: %v", path, err)
	}
	for _, stmt := range splitSQLStatements(string(raw)) {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("apply migration statement failed: %v\nstatement: %.120s", err, stmt)
		}
	}
}

// splitSQLStatements splits a script on top-level semicolons.
//
// A naive strings.Split(";") corrupts this migration: it contains
//
//	DO $$ BEGIN ... END $$;
//
// whose body is full of semicolons. Track dollar-quoted blocks and single-quoted
// literals so only real statement terminators split.
func splitSQLStatements(script string) []string {
	const dollarTag = "$$"
	var out []string
	var cur strings.Builder
	inDollar := false
	inSingle := false

	for i := 0; i < len(script); i++ {
		c := script[i]

		if !inSingle && !inDollar && c == '\'' {
			inSingle = true
			cur.WriteByte(c)
			continue
		}
		if inSingle {
			if c == '\'' {
				// '' is an escaped quote inside a literal.
				if i+1 < len(script) && script[i+1] == '\'' {
					cur.WriteString("''")
					i++
					continue
				}
				inSingle = false
			}
			cur.WriteByte(c)
			continue
		}

		if !inDollar && strings.HasPrefix(script[i:], dollarTag) {
			inDollar = true
			cur.WriteString(dollarTag)
			i += len(dollarTag) - 1
			continue
		}
		if inDollar {
			if strings.HasPrefix(script[i:], dollarTag) {
				inDollar = false
				cur.WriteString(dollarTag)
				i += len(dollarTag) - 1
				continue
			}
			cur.WriteByte(c)
			continue
		}

		if c == ';' {
			if s := strings.TrimSpace(cur.String()); s != "" {
				out = append(out, s)
			}
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	if s := strings.TrimSpace(cur.String()); s != "" {
		out = append(out, s)
	}
	return out
}

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.Open(testDSN(t)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("obtain sql.DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("ping test database: %v", err)
	}
	return db
}

func TestFraudSignalQueries(t *testing.T) {
	db := newTestDB(t)
	applyMigrations(t, db)

	repo := repository.NewGormAffiliateRepository(db, nil)
	ctx := context.Background()
	now := time.Now().UTC()

	affiliateA := uuid.New()
	affiliateB := uuid.New()

	mustExec := func(q string, args ...any) {
		t.Helper()
		if err := db.Exec(q, args...).Error; err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}

	// Make the test re-runnable: drop anything a previous run left behind.
	// Children first, because of the FK from clicks/attributions to profiles.
	mustExec(`DELETE FROM affiliate_attributions WHERE click_id LIKE 'SIG-%'`)
	mustExec(`DELETE FROM affiliate_clicks WHERE click_id LIKE 'SIG-%'`)
	mustExec(`DELETE FROM affiliate_profiles WHERE affiliate_code LIKE 'SIG-%'`)

	profileA := uuid.New()
	mustExec(`INSERT INTO affiliate_profiles (id, user_id, status, affiliate_code, commission_rate)
	          VALUES (?, 900001, 'active', ?, 0.20)`, profileA, "SIG-"+affiliateA.String()[:8])
	profileB := uuid.New()
	mustExec(`INSERT INTO affiliate_profiles (id, user_id, status, affiliate_code, commission_rate)
	          VALUES (?, 900002, 'active', ?, 0.20)`, profileB, "SIG-"+affiliateB.String()[:8])

	seedClick := func(id uuid.UUID, affiliateID uuid.UUID, clickID, deviceFP, ipHash string) {
		mustExec(`INSERT INTO affiliate_clicks (id, affiliate_id, click_id, ip_hash, user_agent_hash,
		              device_fingerprint, country_code, landing_page, created_at)
		          VALUES (?, ?, ?, ?, 'uah', ?, 'US', '/', ?)`,
			id, affiliateID, clickID, ipHash, deviceFP, now.Add(-time.Hour))
	}
	seedAttribution := func(id uuid.UUID, affiliateID uuid.UUID, userID int64, clickID string) {
		mustExec(`INSERT INTO affiliate_attributions (id, affiliate_id, referred_user_id, click_id, created_at)
		          VALUES (?, ?, ?, ?, ?)`, id, affiliateID, userID, clickID, now.Add(-time.Hour))
	}

	click1 := uuid.New()
	click2 := uuid.New()
	click3 := uuid.New()
	seedClick(click1, profileA, "SIG-C1-"+affiliateA.String()[:8], "device-shared", "ip-shared")
	seedClick(click2, profileA, "SIG-C2-"+affiliateA.String()[:8], "device-shared", "ip-shared")
	seedClick(click3, profileB, "SIG-C3-"+affiliateB.String()[:8], "device-solo", "ip-solo")
	seedAttribution(uuid.New(), profileA, 910001, "SIG-C1-"+affiliateA.String()[:8])
	seedAttribution(uuid.New(), profileA, 910002, "SIG-C2-"+affiliateA.String()[:8])
	seedAttribution(uuid.New(), profileB, 910003, "SIG-C3-"+affiliateB.String()[:8])

	t.Run("GetClickByClickID resolves device and IP signals", func(t *testing.T) {
		click, err := repo.GetClickByClickID(ctx, "SIG-C1-"+affiliateA.String()[:8])
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if click == nil {
			t.Fatal("expected the click to be found")
		}
		if click.DeviceFingerprint != "device-shared" {
			t.Fatalf("expected device-shared, got %q", click.DeviceFingerprint)
		}
		if click.IPHash != "ip-shared" {
			t.Fatalf("expected ip-shared, got %q", click.IPHash)
		}
	})

	t.Run("GetClickByClickID returns nil for unknown click", func(t *testing.T) {
		click, err := repo.GetClickByClickID(ctx, "does-not-exist")
		if err != nil {
			t.Fatalf("unknown click must not be an error, got %v", err)
		}
		if click != nil {
			t.Fatalf("expected nil click, got %+v", click)
		}
	})

	t.Run("CountReferredUsersByDevice counts distinct users", func(t *testing.T) {
		n, err := repo.CountReferredUsersByDevice(ctx, profileA, "device-shared", now.Add(-24*time.Hour))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 2 {
			t.Fatalf("expected 2 distinct users on the shared device, got %d", n)
		}
	})

	t.Run("CountReferredUsersByIP counts distinct users", func(t *testing.T) {
		n, err := repo.CountReferredUsersByIP(ctx, profileA, "ip-shared", now.Add(-24*time.Hour))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 2 {
			t.Fatalf("expected 2 distinct users behind the shared IP, got %d", n)
		}
	})

	t.Run("signal counters are scoped per affiliate", func(t *testing.T) {
		n, err := repo.CountReferredUsersByDevice(ctx, profileB, "device-shared", now.Add(-24*time.Hour))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 0 {
			t.Fatalf("affiliate B must not see affiliate A traffic, got %d", n)
		}
	})

	t.Run("windows exclude older rows", func(t *testing.T) {
		n, err := repo.CountReferredUsersByDevice(ctx, profileA, "device-shared", now.Add(time.Hour))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 0 {
			t.Fatalf("future window must exclude seeded rows, got %d", n)
		}
	})

	t.Run("CountClicksSince and CountAttributionsSince", func(t *testing.T) {
		clicks, err := repo.CountClicksSince(ctx, profileA, now.Add(-24*time.Hour))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if clicks < 2 {
			t.Fatalf("expected at least 2 clicks for affiliate A, got %d", clicks)
		}
		attrs, err := repo.CountAttributionsSince(ctx, profileA, now.Add(-24*time.Hour))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if attrs < 2 {
			t.Fatalf("expected at least 2 attributions for affiliate A, got %d", attrs)
		}
	})
}