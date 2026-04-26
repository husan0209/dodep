package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestMigrations_UpAndDown tests all migrations can be applied and rolled back
// Validates: Phase 1.2 - Database Migrations
func TestMigrations_UpAndDown(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Create PostgreSQL container with testcontainers
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("payment_test"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err, "Failed to start PostgreSQL container")

	// Ensure container is terminated at the end
	defer func() {
		_ = pgContainer.Terminate(ctx)
	}()

	// Get connection string
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "Failed to get connection string")

	// Connect to database
	db, err := gorm.Open(gormpostgres.Open(connStr), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "Failed to connect to database")

	sqlDB, err := db.DB()
	require.NoError(t, err, "Failed to get underlying sql.DB")
	defer sqlDB.Close()

	t.Run("run all up migrations", func(t *testing.T) {
		// Read and execute migration 001
		migration001Up, err := os.ReadFile("../../migrations/001_create_payments.up.sql")
		require.NoError(t, err, "Failed to read migration 001 up")

		err = db.Exec(string(migration001Up)).Error
		require.NoError(t, err, "Failed to execute migration 001 up")

		// Read and execute migration 002
		migration002Up, err := os.ReadFile("../../migrations/002_create_withdrawals.up.sql")
		require.NoError(t, err, "Failed to read migration 002 up")

		err = db.Exec(string(migration002Up)).Error
		require.NoError(t, err, "Failed to execute migration 002 up")

		// Read and execute migration 003
		migration003Up, err := os.ReadFile("../../migrations/003_create_audit_logs.up.sql")
		require.NoError(t, err, "Failed to read migration 003 up")

		err = db.Exec(string(migration003Up)).Error
		require.NoError(t, err, "Failed to execute migration 003 up")
	})

	t.Run("verify tables exist", func(t *testing.T) {
		// Check payments table exists
		var paymentsExists bool
		err := sqlDB.QueryRow(`
			SELECT EXISTS (
				SELECT FROM information_schema.tables 
				WHERE table_schema = 'public' 
				AND table_name = 'payments'
			)
		`).Scan(&paymentsExists)
		require.NoError(t, err)
		assert.True(t, paymentsExists, "payments table should exist")

		// Check withdrawals table exists
		var withdrawalsExists bool
		err = sqlDB.QueryRow(`
			SELECT EXISTS (
				SELECT FROM information_schema.tables 
				WHERE table_schema = 'public' 
				AND table_name = 'withdrawals'
			)
		`).Scan(&withdrawalsExists)
		require.NoError(t, err)
		assert.True(t, withdrawalsExists, "withdrawals table should exist")

		// Check payment_audit_logs table exists
		var auditLogsExists bool
		err = sqlDB.QueryRow(`
			SELECT EXISTS (
				SELECT FROM information_schema.tables 
				WHERE table_schema = 'public' 
				AND table_name = 'payment_audit_logs'
			)
		`).Scan(&auditLogsExists)
		require.NoError(t, err)
		assert.True(t, auditLogsExists, "payment_audit_logs table should exist")
	})

	t.Run("verify enums exist with correct values", func(t *testing.T) {
		// Check payment_status enum exists
		var paymentStatusExists bool
		err := sqlDB.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM pg_type 
				WHERE typname = 'payment_status'
			)
		`).Scan(&paymentStatusExists)
		require.NoError(t, err)
		assert.True(t, paymentStatusExists, "payment_status enum should exist")

		// Verify payment_status enum values
		rows, err := sqlDB.Query(`
			SELECT enumlabel 
			FROM pg_enum 
			JOIN pg_type ON pg_enum.enumtypid = pg_type.oid 
			WHERE pg_type.typname = 'payment_status'
			ORDER BY enumsortorder
		`)
		require.NoError(t, err)
		defer rows.Close()

		var paymentStatusValues []string
		for rows.Next() {
			var val string
			require.NoError(t, rows.Scan(&val))
			paymentStatusValues = append(paymentStatusValues, val)
		}

		expectedPaymentStatuses := []string{
			"pending", "waiting", "confirming", "confirmed",
			"sending", "partially_paid", "finished", "failed",
			"expired", "refunded",
		}
		assert.Equal(t, expectedPaymentStatuses, paymentStatusValues, "payment_status enum values mismatch")

		// Check withdrawal_status enum exists
		var withdrawalStatusExists bool
		err = sqlDB.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM pg_type 
				WHERE typname = 'withdrawal_status'
			)
		`).Scan(&withdrawalStatusExists)
		require.NoError(t, err)
		assert.True(t, withdrawalStatusExists, "withdrawal_status enum should exist")

		// Verify withdrawal_status enum values
		rows, err = sqlDB.Query(`
			SELECT enumlabel 
			FROM pg_enum 
			JOIN pg_type ON pg_enum.enumtypid = pg_type.oid 
			WHERE pg_type.typname = 'withdrawal_status'
			ORDER BY enumsortorder
		`)
		require.NoError(t, err)
		defer rows.Close()

		var withdrawalStatusValues []string
		for rows.Next() {
			var val string
			require.NoError(t, rows.Scan(&val))
			withdrawalStatusValues = append(withdrawalStatusValues, val)
		}

		expectedWithdrawalStatuses := []string{
			"processing", "sending", "sent", "finished", "failed", "cancelled",
		}
		assert.Equal(t, expectedWithdrawalStatuses, withdrawalStatusValues, "withdrawal_status enum values mismatch")
	})

	t.Run("verify indexes exist", func(t *testing.T) {
		// Check payments indexes
		paymentsIndexes := []string{
			"idx_payments_user_id_created",
			"idx_payments_payment_id",
			"idx_payments_idempotency_key",
			"idx_payments_status_created",
			"idx_payments_active",
		}

		for _, idxName := range paymentsIndexes {
			var exists bool
			err := sqlDB.QueryRow(`
				SELECT EXISTS (
					SELECT 1 FROM pg_indexes 
					WHERE indexname = $1
				)
			`, idxName).Scan(&exists)
			require.NoError(t, err)
			assert.True(t, exists, fmt.Sprintf("index %s should exist on payments table", idxName))
		}

		// Check withdrawals indexes
		withdrawalsIndexes := []string{
			"idx_withdrawals_user_id_created",
			"idx_withdrawals_withdrawal_id",
			"idx_withdrawals_idempotency_key",
			"idx_withdrawals_status",
		}

		for _, idxName := range withdrawalsIndexes {
			var exists bool
			err := sqlDB.QueryRow(`
				SELECT EXISTS (
					SELECT 1 FROM pg_indexes 
					WHERE indexname = $1
				)
			`, idxName).Scan(&exists)
			require.NoError(t, err)
			assert.True(t, exists, fmt.Sprintf("index %s should exist on withdrawals table", idxName))
		}

		// Check audit_logs indexes
		auditLogsIndexes := []string{
			"idx_audit_logs_user_id",
			"idx_audit_logs_operation_type",
			"idx_audit_logs_reference",
			"idx_audit_logs_trace_id",
			"idx_audit_logs_created_at",
			"idx_audit_logs_errors",
		}

		for _, idxName := range auditLogsIndexes {
			var exists bool
			err := sqlDB.QueryRow(`
				SELECT EXISTS (
					SELECT 1 FROM pg_indexes 
					WHERE indexname = $1
				)
			`, idxName).Scan(&exists)
			require.NoError(t, err)
			assert.True(t, exists, fmt.Sprintf("index %s should exist on payment_audit_logs table", idxName))
		}
	})

	t.Run("verify triggers exist", func(t *testing.T) {
		// Check payments updated_at trigger
		var paymentsTriggerExists bool
		err := sqlDB.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM pg_trigger 
				WHERE tgname = 'trigger_payments_updated_at'
			)
		`).Scan(&paymentsTriggerExists)
		require.NoError(t, err)
		assert.True(t, paymentsTriggerExists, "trigger_payments_updated_at should exist")

		// Check withdrawals updated_at trigger
		var withdrawalsTriggerExists bool
		err = sqlDB.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM pg_trigger 
				WHERE tgname = 'trigger_withdrawals_updated_at'
			)
		`).Scan(&withdrawalsTriggerExists)
		require.NoError(t, err)
		assert.True(t, withdrawalsTriggerExists, "trigger_withdrawals_updated_at should exist")
	})

	t.Run("verify column types and constraints", func(t *testing.T) {
		// Check payments table columns
		rows, err := sqlDB.Query(`
			SELECT column_name, data_type, is_nullable, column_default
			FROM information_schema.columns
			WHERE table_name = 'payments'
			ORDER BY ordinal_position
		`)
		require.NoError(t, err)
		defer rows.Close()

		columns := make(map[string]struct {
			dataType    string
			isNullable  string
			hasDefault  bool
		})

		for rows.Next() {
			var colName, dataType, isNullable string
			var columnDefault *string
			require.NoError(t, rows.Scan(&colName, &dataType, &isNullable, &columnDefault))
			columns[colName] = struct {
				dataType    string
				isNullable  string
				hasDefault  bool
			}{
				dataType:    dataType,
				isNullable:  isNullable,
				hasDefault:  columnDefault != nil,
			}
		}

		// Verify critical columns
		assert.Equal(t, "bigint", columns["id"].dataType, "id should be bigint")
		assert.Equal(t, "uuid", columns["uuid"].dataType, "uuid should be uuid type")
		assert.Equal(t, "USER-DEFINED", columns["status"].dataType, "status should be enum type")
		assert.Equal(t, "numeric", columns["requested_amount"].dataType, "requested_amount should be numeric")
		assert.Equal(t, "NO", columns["user_id"].isNullable, "user_id should not be nullable")
		assert.Equal(t, "YES", columns["actual_amount"].isNullable, "actual_amount should be nullable")
	})

	t.Run("run all down migrations (rollback)", func(t *testing.T) {
		// Read and execute migration 003 down
		migration003Down, err := os.ReadFile("../../migrations/003_create_audit_logs.down.sql")
		require.NoError(t, err, "Failed to read migration 003 down")

		err = db.Exec(string(migration003Down)).Error
		require.NoError(t, err, "Failed to execute migration 003 down")

		// Read and execute migration 002 down
		migration002Down, err := os.ReadFile("../../migrations/002_create_withdrawals.down.sql")
		require.NoError(t, err, "Failed to read migration 002 down")

		err = db.Exec(string(migration002Down)).Error
		require.NoError(t, err, "Failed to execute migration 002 down")

		// Read and execute migration 001 down
		migration001Down, err := os.ReadFile("../../migrations/001_create_payments.down.sql")
		require.NoError(t, err, "Failed to read migration 001 down")

		err = db.Exec(string(migration001Down)).Error
		require.NoError(t, err, "Failed to execute migration 001 down")
	})

	t.Run("verify tables dropped after rollback", func(t *testing.T) {
		// Check payments table does not exist
		var paymentsExists bool
		err := sqlDB.QueryRow(`
			SELECT EXISTS (
				SELECT FROM information_schema.tables 
				WHERE table_schema = 'public' 
				AND table_name = 'payments'
			)
		`).Scan(&paymentsExists)
		require.NoError(t, err)
		assert.False(t, paymentsExists, "payments table should not exist after rollback")

		// Check withdrawals table does not exist
		var withdrawalsExists bool
		err = sqlDB.QueryRow(`
			SELECT EXISTS (
				SELECT FROM information_schema.tables 
				WHERE table_schema = 'public' 
				AND table_name = 'withdrawals'
			)
		`).Scan(&withdrawalsExists)
		require.NoError(t, err)
		assert.False(t, withdrawalsExists, "withdrawals table should not exist after rollback")

		// Check payment_audit_logs table does not exist
		var auditLogsExists bool
		err = sqlDB.QueryRow(`
			SELECT EXISTS (
				SELECT FROM information_schema.tables 
				WHERE table_schema = 'public' 
				AND table_name = 'payment_audit_logs'
			)
		`).Scan(&auditLogsExists)
		require.NoError(t, err)
		assert.False(t, auditLogsExists, "payment_audit_logs table should not exist after rollback")
	})

	t.Run("verify enums dropped after rollback", func(t *testing.T) {
		// Check payment_status enum does not exist
		var paymentStatusExists bool
		err := sqlDB.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM pg_type 
				WHERE typname = 'payment_status'
			)
		`).Scan(&paymentStatusExists)
		require.NoError(t, err)
		assert.False(t, paymentStatusExists, "payment_status enum should not exist after rollback")

		// Check withdrawal_status enum does not exist
		var withdrawalStatusExists bool
		err = sqlDB.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM pg_type 
				WHERE typname = 'withdrawal_status'
			)
		`).Scan(&withdrawalStatusExists)
		require.NoError(t, err)
		assert.False(t, withdrawalStatusExists, "withdrawal_status enum should not exist after rollback")
	})
}

// TestMigrations_IdempotentDown tests that down migrations can be run multiple times safely
func TestMigrations_IdempotentDown(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Create PostgreSQL container
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("payment_test"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err, "Failed to start PostgreSQL container")

	defer func() {
		_ = pgContainer.Terminate(ctx)
	}()

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "Failed to get connection string")

	db, err := gorm.Open(gormpostgres.Open(connStr), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "Failed to connect to database")

	sqlDB, err := db.DB()
	require.NoError(t, err, "Failed to get underlying sql.DB")
	defer sqlDB.Close()

	// Run up migrations first
	migration001Up, err := os.ReadFile("../../migrations/001_create_payments.up.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(migration001Up)).Error)

	// Run down migration twice - should not error
	migration001Down, err := os.ReadFile("../../migrations/001_create_payments.down.sql")
	require.NoError(t, err)

	err = db.Exec(string(migration001Down)).Error
	require.NoError(t, err, "First down migration should succeed")

	err = db.Exec(string(migration001Down)).Error
	require.NoError(t, err, "Second down migration should succeed (idempotent)")
}

// TestMigrations_CanReApply tests that migrations can be re-applied after rollback
func TestMigrations_CanReApply(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Create PostgreSQL container
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("payment_test"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err, "Failed to start PostgreSQL container")

	defer func() {
		_ = pgContainer.Terminate(ctx)
	}()

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "Failed to get connection string")

	db, err := gorm.Open(gormpostgres.Open(connStr), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "Failed to connect to database")

	sqlDB, err := db.DB()
	require.NoError(t, err, "Failed to get underlying sql.DB")
	defer sqlDB.Close()

	// First cycle: up then down
	migration001Up, err := os.ReadFile("../../migrations/001_create_payments.up.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(migration001Up)).Error, "First up migration should succeed")

	migration001Down, err := os.ReadFile("../../migrations/001_create_payments.down.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(migration001Down)).Error, "First down migration should succeed")

	// Second cycle: up again
	err = db.Exec(string(migration001Up)).Error
	require.NoError(t, err, "Second up migration should succeed")

	// Verify table exists
	var tableExists bool
	err = sqlDB.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'payments'
		)
	`).Scan(&tableExists)
	require.NoError(t, err)
	assert.True(t, tableExists, "payments table should exist after re-applying migration")
}
