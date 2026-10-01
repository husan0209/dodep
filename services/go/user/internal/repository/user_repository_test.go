package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"

	"github.com/opus-casino/user/internal/domain"
)

func mustMock(t *testing.T) pgxmock.PgxPoolIface {
	t.Helper()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock pool failed: %v", err)
	}
	t.Cleanup(func() { mock.Close() })
	return mock
}

func expectOK(t *testing.T, mock pgxmock.PgxPoolIface) {
	t.Helper()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func strPtr(s string) *string { return &s }

var userColumns = []string{
	"id", "uuid", "email", "username", "first_name", "last_name", "phone",
	"date_of_birth", "country_code", "currency_code", "status", "kyc_level",
	"language", "timezone", "address", "city", "postal_code", "referral_code",
	"created_at", "updated_at", "last_login_at", "metadata",
}

func userRow() []any {
	now := time.Now()
	return []any{
		int64(42), "11111111-1111-1111-1111-111111111111", "user@example.com", "player42",
		strPtr("John"), strPtr("Doe"), strPtr("+380501234567"), strPtr("1990-01-01"),
		"UA", "USD", "active", int16(2),
		"en", "UTC", strPtr("Main st 1"), strPtr("Kyiv"), strPtr("01001"), strPtr("REF123"),
		now, now, (*time.Time)(nil), "{}",
	}
}

func TestGetUserByID(t *testing.T) {
	ctx := context.Background()

	t.Run("found", func(t *testing.T) {
		mock := mustMock(t)
		repo := NewUserRepository(mock)
		mock.ExpectQuery(regexp.QuoteMeta("FROM users WHERE id = $1")).WithArgs(int64(42)).
			WillReturnRows(pgxmock.NewRows(userColumns).AddRow(userRow()...))
		got, err := repo.GetUserByID(ctx, 42)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != 42 || got.Email != "user@example.com" || *got.FirstName != "John" {
			t.Fatalf("wrong user: %+v", got)
		}
		if got.Status != domain.UserStatusActive || got.KYCLevel != domain.KYCLevelVerified {
			t.Fatalf("wrong enums: %+v", got)
		}
		expectOK(t, mock)
	})

	t.Run("not found returns nil nil", func(t *testing.T) {
		mock := mustMock(t)
		repo := NewUserRepository(mock)
		mock.ExpectQuery(regexp.QuoteMeta("FROM users WHERE id = $1")).WithArgs(int64(99)).
			WillReturnRows(pgxmock.NewRows(userColumns))
		got, err := repo.GetUserByID(ctx, 99)
		if err != nil || got != nil {
			t.Fatalf("expected nil,nil got %+v %v", got, err)
		}
		expectOK(t, mock)
	})

	t.Run("db error", func(t *testing.T) {
		mock := mustMock(t)
		repo := NewUserRepository(mock)
		mock.ExpectQuery(regexp.QuoteMeta("FROM users WHERE id = $1")).WithArgs(int64(42)).
			WillReturnError(errors.New("connection refused"))
		if _, err := repo.GetUserByID(ctx, 42); err == nil {
			t.Fatal("expected error")
		}
		expectOK(t, mock)
	})
}

func TestGetUserByEmail(t *testing.T) {
	ctx := context.Background()
	mock := mustMock(t)
	repo := NewUserRepository(mock)
	mock.ExpectQuery(regexp.QuoteMeta("FROM users WHERE email = $1")).WithArgs("user@example.com").
		WillReturnRows(pgxmock.NewRows(userColumns).AddRow(userRow()...))
	got, err := repo.GetUserByEmail(ctx, "user@example.com")
	if err != nil || got.Username != "player42" {
		t.Fatalf("wrong user: %+v %v", got, err)
	}
	expectOK(t, mock)
}

func TestUpdateUser(t *testing.T) {
	ctx := context.Background()

	t.Run("ok re-reads", func(t *testing.T) {
		mock := mustMock(t)
		repo := NewUserRepository(mock)
		mock.ExpectExec(regexp.QuoteMeta("UPDATE users SET")).WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		mock.ExpectQuery(regexp.QuoteMeta("FROM users WHERE id = $1")).WithArgs(int64(42)).
			WillReturnRows(pgxmock.NewRows(userColumns).AddRow(userRow()...))
		got, err := repo.UpdateUser(ctx, &domain.UpdateUserRequest{UserID: 42, Username: strPtr("renamed")})
		if err != nil || got == nil {
			t.Fatalf("update failed: %+v %v", got, err)
		}
		expectOK(t, mock)
	})

	t.Run("exec error", func(t *testing.T) {
		mock := mustMock(t)
		repo := NewUserRepository(mock)
		mock.ExpectExec(regexp.QuoteMeta("UPDATE users SET")).WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnError(errors.New("db is down"))
		if _, err := repo.UpdateUser(ctx, &domain.UpdateUserRequest{UserID: 42}); err == nil {
			t.Fatal("expected error")
		}
		expectOK(t, mock)
	})
}

func TestSoftDeleteUser(t *testing.T) {
	ctx := context.Background()
	mock := mustMock(t)
	repo := NewUserRepository(mock)
	mock.ExpectExec(regexp.QuoteMeta("SET status = 'deleted'")).WithArgs(int64(42)).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	if err := repo.SoftDeleteUser(ctx, 42); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	expectOK(t, mock)
}

var prefsColumns = []string{
	"user_id", "language", "timezone", "currency_display", "marketing_emails",
	"sms_notifications", "push_notifications", "reality_check",
	"reality_check_interval_minutes", "auto_play", "sound_preference", "updated_at",
}

func TestGetPreferences(t *testing.T) {
	ctx := context.Background()

	t.Run("found", func(t *testing.T) {
		mock := mustMock(t)
		repo := NewUserRepository(mock)
		mock.ExpectQuery(regexp.QuoteMeta("FROM user_preferences WHERE user_id = $1")).WithArgs(int64(42)).WillReturnRows(
			pgxmock.NewRows(prefsColumns).AddRow(
				int64(42), "uk", "Europe/Kyiv", "USD", true, false, true, true, 30, false, "on", time.Now(),
			),
		)
		got, err := repo.GetPreferences(ctx, 42)
		if err != nil || got.Language != "uk" || !got.MarketingEmails {
			t.Fatalf("wrong prefs: %+v %v", got, err)
		}
		expectOK(t, mock)
	})

	t.Run("missing returns defaults", func(t *testing.T) {
		mock := mustMock(t)
		repo := NewUserRepository(mock)
		mock.ExpectQuery(regexp.QuoteMeta("FROM user_preferences WHERE user_id = $1")).WithArgs(int64(42)).
			WillReturnRows(pgxmock.NewRows(prefsColumns))
		got, err := repo.GetPreferences(ctx, 42)
		if err != nil || got.Language != "en" || got.Timezone != "UTC" {
			t.Fatalf("expected defaults, got %+v %v", got, err)
		}
		expectOK(t, mock)
	})
}

func TestUpsertPreferences(t *testing.T) {
	ctx := context.Background()
	mock := mustMock(t)
	repo := NewUserRepository(mock)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO user_preferences")).WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	err := repo.UpsertPreferences(ctx, &domain.UserPreferences{UserID: 42, Language: "en", Timezone: "UTC"})
	if err != nil {
		t.Fatalf("upsert failed: %v", err)
	}
	expectOK(t, mock)
}

var limitsColumns = []string{
	"user_id", "daily_deposit_limit", "weekly_deposit_limit", "monthly_deposit_limit",
	"daily_bet_limit", "weekly_bet_limit", "monthly_bet_limit",
	"daily_loss_limit", "weekly_loss_limit", "monthly_loss_limit",
	"session_time_limit_minutes", "session_time_limit_active",
	"self_exclusion", "self_exclusion_until", "updated_at",
}

func TestGetLimits(t *testing.T) {
	ctx := context.Background()

	t.Run("found with session limit", func(t *testing.T) {
		mock := mustMock(t)
		repo := NewUserRepository(mock)
		mock.ExpectQuery(regexp.QuoteMeta("FROM user_limits WHERE user_id = $1")).WithArgs(int64(42)).WillReturnRows(
			pgxmock.NewRows(limitsColumns).AddRow(
				int64(42), strPtr("100.00"), nil, nil, nil, nil, nil, nil, nil, nil,
				60, true, false, nil, time.Now(),
			),
		)
		got, err := repo.GetLimits(ctx, 42)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.SessionTimeLimit == nil || got.SessionTimeLimit.Minutes != 60 || !got.SessionTimeLimit.IsActive {
			t.Fatalf("wrong session limit: %+v", got.SessionTimeLimit)
		}
		if got.DailyDepositLimit == nil || got.DailyDepositLimit.Amount != "100.00" {
			t.Fatalf("wrong deposit limit: %+v", got.DailyDepositLimit)
		}
		expectOK(t, mock)
	})

	t.Run("missing returns empty", func(t *testing.T) {
		mock := mustMock(t)
		repo := NewUserRepository(mock)
		mock.ExpectQuery(regexp.QuoteMeta("FROM user_limits WHERE user_id = $1")).WithArgs(int64(42)).
			WillReturnRows(pgxmock.NewRows(limitsColumns))
		got, err := repo.GetLimits(ctx, 42)
		if err != nil || got.UserID != 42 || got.SessionTimeLimit != nil {
			t.Fatalf("expected empty limits, got %+v %v", got, err)
		}
		expectOK(t, mock)
	})
}

func TestSetLimits(t *testing.T) {
	ctx := context.Background()
	mock := mustMock(t)
	repo := NewUserRepository(mock)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO user_limits")).WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	minutes := 120
	err := repo.SetLimits(ctx, 42, &domain.SetLimitsRequest{
		UserID: 42, DailyDepositLimit: strPtr("250.00"), SessionTimeMinutes: &minutes,
	})
	if err != nil {
		t.Fatalf("set limits failed: %v", err)
	}
	expectOK(t, mock)
}

func TestGetActivity(t *testing.T) {
	ctx := context.Background()
	mock := mustMock(t)
	repo := NewUserRepository(mock)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM audit_log")).WithArgs(int64(42)).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))
	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta("FROM audit_log WHERE record_id")).WithArgs(int64(42), 20, 0).WillReturnRows(
		pgxmock.NewRows([]string{"id", "action", "old_data", "new_data", "user_id", "created_at"}).
			AddRow(int64(1), "login", "{}", "{}", int64(7), now).
			AddRow(int64(2), "deposit", "{}", "{}", nil, now),
	)
	items, total, err := repo.GetActivity(ctx, 42, 20, 0)
	if err != nil || total != 2 || len(items) != 2 {
		t.Fatalf("wrong activity: %+v %d %v", items, total, err)
	}
	if items[0]["action"] != "login" || items[1]["id"] != int64(2) {
		t.Fatalf("wrong mapping: %+v", items)
	}
	expectOK(t, mock)
}

func TestNewUserRepositoryRequiresPool(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil pool")
		}
	}()
	NewUserRepository(nil)
}
