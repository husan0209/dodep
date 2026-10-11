package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opus-casino/rg/internal/domain"
)

// newMockGorm builds a GORM handle backed by go-sqlmock. This exercises the
// real SQL GORM emits (text, args, upsert clauses) without a live database.
func newMockGorm(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}
	return gdb, mock
}

func newRepo(t *testing.T) (*GormRepository, sqlmock.Sqlmock) {
	gdb, mock := newMockGorm(t)
	return &GormRepository{db: gdb}, mock
}

func mustUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatalf("bad uuid %q: %v", s, err)
	}
	return id
}

// q builds a regexp that matches GORM's PostgreSQL quoting (double quotes).
func q(literal string) string { return regexp.QuoteMeta(literal) }

var limitCols = []string{
	"user_id", "deposit_daily", "deposit_weekly", "deposit_monthly",
	"loss_daily", "loss_weekly", "loss_monthly",
	"wager_daily", "wager_weekly", "session_minutes", "reality_check_minutes", "updated_at",
}

var pendingCols = []string{
	"id", "user_id", "limit_type", "old_value", "new_value", "status", "requested_at", "effective_at",
}

var exclCols = []string{
	"id", "user_id", "type", "status", "until", "permanent",
	"created_by", "created_at", "revoked_at", "revoked_by",
}

func TestGetLimitsFound(t *testing.T) {
	repo, mock := newRepo(t)
	mock.ExpectQuery(q(`SELECT * FROM "rg_service_limits" WHERE user_id = $1`)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows(limitCols).AddRow(
			int64(7), "100.00000000", "0.00000000", "0.00000000",
			"40.00000000", "0.00000000", "0.00000000",
			"50.00000000", "0.00000000", 60, 30, time.Now(),
		))
	got, err := repo.GetLimits(context.Background(), 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.DepositDaily.String() != "100" || got.LossDaily.String() != "40" ||
		got.WagerDaily.String() != "50" || got.SessionMinutes != 60 || got.RealityCheckMinutes != 30 {
		t.Fatalf("wrong mapping: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestGetLimitsMissingReturnsZeroed(t *testing.T) {
	repo, mock := newRepo(t)
	mock.ExpectQuery(q(`FROM "rg_service_limits"`)).
		WithArgs(int64(99)).
		WillReturnRows(sqlmock.NewRows(limitCols))
	got, err := repo.GetLimits(context.Background(), 99)
	if err != nil || got.UserID != 99 || !got.DepositDaily.IsZero() {
		t.Fatalf("expected zeroed limits: %+v %v", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestGetLimitsDBError(t *testing.T) {
	repo, mock := newRepo(t)
	mock.ExpectQuery(q(`FROM "rg_service_limits"`)).
		WithArgs(int64(7)).
		WillReturnError(context.DeadlineExceeded)
	if _, err := repo.GetLimits(context.Background(), 7); err == nil {
		t.Fatal("expected error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestUpsertLimitsUsesSingleUpsertStatement(t *testing.T) {
	repo, mock := newRepo(t)
	mock.ExpectBegin()
	// One INSERT ... ON CONFLICT DO UPDATE RETURNING (no UPDATE-then-INSERT race).
	mock.ExpectQuery(q(`INSERT INTO "rg_service_limits"`)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(int64(7)))
	mock.ExpectCommit()
	err := repo.UpsertLimits(context.Background(), &domain.RGLimits{
		UserID: 7, DepositDaily: decimal.NewFromInt(100), LossDaily: decimal.NewFromInt(40),
		SessionMinutes: 60, RealityCheckMinutes: 30,
	})
	if err != nil {
		t.Fatalf("upsert failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestCreatePendingChange(t *testing.T) {
	repo, mock := newRepo(t)
	now := time.Now().UTC()
	id := "11111111-1111-4111-8111-111111111111"

	mock.ExpectBegin()
	mock.ExpectExec(q(`INSERT INTO "rg_service_pending_changes"`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	err := repo.CreatePendingChange(context.Background(), &domain.PendingChange{
		ID: mustUUID(t, id), UserID: 7, LimitType: domain.LimitDepositDaily,
		OldValue: decimal.NewFromInt(100), NewValue: decimal.NewFromInt(900),
		Status: domain.ChangePending, RequestedAt: now, EffectiveAt: now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestListPendingChanges(t *testing.T) {
	repo, mock := newRepo(t)
	now := time.Now().UTC()
	effective := now.Add(24 * time.Hour)

	mock.ExpectQuery(q(`FROM "rg_service_pending_changes" WHERE user_id = $1 AND status = $2`)).
		WithArgs(int64(7), "pending").
		WillReturnRows(sqlmock.NewRows(pendingCols).AddRow(
			"11111111-1111-4111-8111-111111111111", int64(7), "deposit_daily",
			"100.00000000", "900.00000000", "pending", now, effective,
		))
	list, err := repo.ListPendingChanges(context.Background(), 7)
	if err != nil || len(list) != 1 {
		t.Fatalf("bad list: %+v %v", list, err)
	}
	if list[0].LimitType != domain.LimitDepositDaily || list[0].NewValue.String() != "900" {
		t.Fatalf("wrong mapping: %+v", list[0])
	}
	if list[0].IsDue(now) {
		t.Fatal("future effective_at must not be due")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestListDueChanges(t *testing.T) {
	repo, mock := newRepo(t)
	now := time.Now().UTC()
	past := now.Add(-time.Hour)

	mock.ExpectQuery(q(`FROM "rg_service_pending_changes" WHERE status = $1 AND effective_at <= $2`)).
		WithArgs("pending", past).
		WillReturnRows(sqlmock.NewRows(pendingCols).AddRow(
			"11111111-1111-4111-8111-111111111111", int64(7), "deposit_daily",
			"100.00000000", "900.00000000", "pending", past.Add(-24*time.Hour), past,
		))
	due, err := repo.ListDueChanges(context.Background(), past, 500)
	if err != nil || len(due) != 1 {
		t.Fatalf("expected one due change: %+v %v", due, err)
	}
	if !due[0].IsDue(past) {
		t.Fatal("past change must be due")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestUpdateChangeStatusGuarded(t *testing.T) {
	repo, mock := newRepo(t)
	id := mustUUID(t, "11111111-1111-4111-8111-111111111111")

	mock.ExpectBegin()
	mock.ExpectExec(q(`UPDATE "rg_service_pending_changes" SET`)).
		WithArgs("applied", id, "pending").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := repo.UpdateChangeStatus(context.Background(), id, domain.ChangePending, domain.ChangeApplied); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectExec(q(`UPDATE "rg_service_pending_changes" SET`)).
		WithArgs("applied", id, "pending").
		WillReturnResult(sqlmock.NewResult(0, 0))
	// The guard is detected after the statement, so GORM still commits the
	// (no-op) transaction; the repository turns RowsAffected==0 into a conflict.
	mock.ExpectCommit()
	if err := repo.UpdateChangeStatus(context.Background(), id, domain.ChangePending, domain.ChangeApplied); err != domain.ErrConflict {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestDeletePendingChangesScoped(t *testing.T) {
	repo, mock := newRepo(t)
	mock.ExpectBegin()
	mock.ExpectExec(q(`DELETE FROM "rg_service_pending_changes" WHERE user_id = $1 AND limit_type = $2 AND status = $3`)).
		WithArgs(int64(7), "deposit_daily", "pending").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := repo.DeletePendingChanges(context.Background(), 7, domain.LimitDepositDaily); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestCreateAndReadExclusion(t *testing.T) {
	repo, mock := newRepo(t)
	id := "22222222-2222-4222-8222-222222222222"
	until := time.Now().UTC().Add(24 * time.Hour)

	mock.ExpectBegin()
	mock.ExpectExec(q(`INSERT INTO "rg_service_exclusions"`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := repo.CreateExclusion(context.Background(), &domain.Exclusion{
		ID: mustUUID(t, id), UserID: 7, Type: domain.ExclusionSelf, Status: domain.ExclusionActive,
		Until: &until, CreatedBy: "self", CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	mock.ExpectQuery(q(`FROM "rg_service_exclusions" WHERE user_id = $1 AND status = $2`)).
		WithArgs(int64(7), "active").
		WillReturnRows(sqlmock.NewRows(exclCols).AddRow(
			id, int64(7), "self", "active", until, false, "self", time.Now(), nil, "",
		))
	got, err := repo.GetActiveExclusion(context.Background(), 7)
	if err != nil || got == nil {
		t.Fatalf("expected exclusion: %+v %v", got, err)
	}
	if got.Permanent || !got.IsActiveAt(time.Now()) {
		t.Fatalf("wrong mapping: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestGetActiveExclusionNone(t *testing.T) {
	repo, mock := newRepo(t)
	mock.ExpectQuery(q(`FROM "rg_service_exclusions" WHERE user_id = $1 AND status = $2`)).
		WithArgs(int64(8), "active").
		WillReturnRows(sqlmock.NewRows(exclCols))
	got, err := repo.GetActiveExclusion(context.Background(), 8)
	if err != nil || got != nil {
		t.Fatalf("expected nil,nil got %+v %v", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestListExclusions(t *testing.T) {
	repo, mock := newRepo(t)
	mock.ExpectQuery(q(`FROM "rg_service_exclusions" WHERE user_id = $1`)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows(exclCols).AddRow(
			"22222222-2222-4222-8222-222222222222", int64(7), "self", "active",
			time.Now().Add(time.Hour), false, "self", time.Now(), nil, "",
		))
	list, err := repo.ListExclusions(context.Background(), 7, 20)
	if err != nil || len(list) != 1 {
		t.Fatalf("bad list: %+v %v", list, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestUpdateExclusionStatusConflict(t *testing.T) {
	repo, mock := newRepo(t)
	id := mustUUID(t, "22222222-2222-4222-8222-222222222222")
	mock.ExpectBegin()
	mock.ExpectExec(q(`UPDATE "rg_service_exclusions" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	// Guard detected post-statement: GORM commits, repository reports conflict.
	mock.ExpectCommit()
	err := repo.UpdateExclusionStatus(context.Background(), id, domain.ExclusionActive,
		domain.ExclusionRevoked, map[string]interface{}{"revoked_by": "admin"})
	if err != domain.ErrConflict {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestCreateAndReadTimeout(t *testing.T) {
	repo, mock := newRepo(t)
	id := "44444444-4444-4444-8444-444444444444"
	until := time.Now().UTC().Add(24 * time.Hour)

	mock.ExpectBegin()
	mock.ExpectExec(q(`INSERT INTO "rg_service_timeouts"`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := repo.CreateTimeout(context.Background(), &domain.Timeout{
		ID: mustUUID(t, id), UserID: 7, Until: until, CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	mock.ExpectQuery(q(`FROM "rg_service_timeouts" WHERE user_id = $1 AND until > $2`)).WithArgs(int64(7), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "until", "created_at"}).
			AddRow(id, int64(7), until, time.Now()))
	got, err := repo.GetActiveTimeout(context.Background(), 7, time.Now().UTC())
	if err != nil || got == nil {
		t.Fatalf("bad timeout: %+v %v", got, err)
	}
	if !got.IsActiveAt(time.Now().UTC()) {
		t.Fatal("future timeout must be active")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestRecordSpendValidation(t *testing.T) {
	repo, _ := newRepo(t)
	ctx := context.Background()
	if err := repo.RecordSpend(ctx, 7, time.Now(), "US", "0", "0", "0"); err == nil {
		t.Error("bad currency must be rejected")
	}
	if err := repo.RecordSpend(ctx, 7, time.Now(), "USD", "-1", "0", "0"); err == nil {
		t.Error("negative deposits must be rejected")
	}
	if err := repo.RecordSpend(ctx, 7, time.Now(), "USD", "0", "not-a-number", "0"); err == nil {
		t.Error("non-numeric wagers must be rejected")
	}
	if err := repo.RecordSpend(ctx, 7, time.Now(), "USD", "0", "0", "-2"); err == nil {
		t.Error("negative payouts must be rejected")
	}
}

func TestRecordSpendUpserts(t *testing.T) {
	repo, mock := newRepo(t)
	day := time.Now().UTC().Truncate(24 * time.Hour)
	mock.ExpectBegin()
	mock.ExpectExec(q(`INSERT INTO "rg_service_spend_daily"`)).
		WithArgs(int64(7), day, "USD", "10", "20", "5", "10", "5", "20").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	err := repo.RecordSpend(context.Background(), 7, time.Now(), "USD", "10", "20", "5")
	if err != nil {
		t.Fatalf("record spend failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestSumSpendSince(t *testing.T) {
	repo, mock := newRepo(t)
	since := time.Now().UTC().Add(-48 * time.Hour).Truncate(24 * time.Hour)

	mock.ExpectQuery(`COALESCE\(SUM\(deposits\),0\) AS deposits, COALESCE\(SUM\(wagers\),0\) AS wagers, COALESCE\(SUM\(payouts\),0\) AS payouts FROM "rg_service_spend_daily" WHERE user_id = \$1 AND currency = \$2 AND day >= \$3`).
		WithArgs(int64(7), "USD", since).
		WillReturnRows(sqlmock.NewRows([]string{"deposits", "wagers", "payouts"}).
			AddRow(decimal.NewFromInt(100), decimal.NewFromInt(500), decimal.NewFromInt(200)))
	dep, wag, pay, err := repo.SumSpendSince(context.Background(), 7, "USD", since)
	if err != nil || dep != "100" || wag != "500" || pay != "200" {
		t.Fatalf("bad sums: %q %q %q %v", dep, wag, pay, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestOutboxRoundTrip(t *testing.T) {
	repo, mock := newRepo(t)
	ctx := context.Background()
	id := "55555555-5555-4555-8555-555555555555"

	mock.ExpectBegin()
	mock.ExpectExec(q(`INSERT INTO "rg_service_outbox"`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := repo.AppendOutbox(ctx, "rg.limit.set", "7", map[string]interface{}{"user_id": 7}); err != nil {
		t.Fatalf("append failed: %v", err)
	}

	mock.ExpectQuery(q(`FROM "rg_service_outbox" WHERE published_at IS NULL`)).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"id", "topic", "event_key", "payload", "created_at"}).
			AddRow(id, "rg.limit.set", "7", `{"user_id":7}`, time.Now()))
	events, err := repo.ListPendingOutbox(ctx, 100)
	if err != nil || len(events) != 1 {
		t.Fatalf("bad outbox list: %+v %v", events, err)
	}
	if events[0].Topic != "rg.limit.set" || events[0].EventKey != "7" {
		t.Fatalf("wrong mapping: %+v", events[0])
	}

	mock.ExpectBegin()
	mock.ExpectExec(q(`UPDATE "rg_service_outbox" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := repo.MarkOutboxPublished(ctx, mustUUID(t, id)); err != nil {
		t.Fatalf("mark published failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestTransactCommitsAndRollsBack(t *testing.T) {
	repo, mock := newRepo(t)

	mock.ExpectBegin()
	mock.ExpectRollback()
	if err := repo.Transact(context.Background(), func(Repository) error {
		return domain.ErrConflict
	}); err != domain.ErrConflict {
		t.Fatalf("expected callback error, got %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectCommit()
	if err := repo.Transact(context.Background(), func(Repository) error { return nil }); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestNewGormRepositoryRequiresDB(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil db")
		}
	}()
	NewGormRepository(nil)
}
