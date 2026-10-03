package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"

	"github.com/opus-casino/auth/internal/crypto"
	"github.com/opus-casino/auth/internal/domain"
)

// These tests pin down how the service reacts to storage failures on
// security-relevant writes. Reporting success while Redis/Postgres rejected
// the write is how a logout leaves a session alive, a 2FA toggle pretends to
// be on and a password change keeps old sessions usable, so the rule is:
// state-changing failures are returned, bookkeeping failures are logged.

var errStorage = errors.New("storage unavailable")

// userWithHash builds a user whose password hash really verifies, so the tests
// reach the code path after the password check instead of failing early.
func userWithHash(t *testing.T, id, email, password string) *domain.User {
	t.Helper()

	hash, err := crypto.HashPassword(password)
	require.NoError(t, err)

	return &domain.User{ID: id, Email: email, PasswordHash: hash}
}

// TestLogoutReportsSessionDeletionFailure: a logout that could not remove the
// session must not answer "success", otherwise the client believes it is
// logged out while the session is still usable.
func TestLogoutReportsSessionDeletionFailure(t *testing.T) {
	repo := &mockAuthRepository{
		getSession: func(context.Context, string) (*domain.Session, error) {
			return &domain.Session{ID: "sess-1", UserID: "user-1"}, nil
		},
		deleteSession: func(context.Context, string, string) error {
			return errStorage
		},
	}
	svc := newTestService(repo)

	err := svc.Logout(context.Background(), "user-1", "sess-1")
	require.Error(t, err, "a failed session delete must be reported")
	require.ErrorIs(t, err, domain.ErrInternal)
}

// TestLogoutSucceedsWhenSessionDeleteWorks is the control case for the test
// above: the same wiring must still produce a clean logout.
func TestLogoutSucceedsWhenSessionDeleteWorks(t *testing.T) {
	deleted := false
	repo := &mockAuthRepository{
		getSession: func(context.Context, string) (*domain.Session, error) {
			return &domain.Session{ID: "sess-1", UserID: "user-1"}, nil
		},
		deleteSession: func(_ context.Context, sessionID, userID string) error {
			deleted = sessionID == "sess-1" && userID == "user-1"
			return nil
		},
	}
	svc := newTestService(repo)

	require.NoError(t, svc.Logout(context.Background(), "user-1", "sess-1"))
	require.True(t, deleted, "session must actually be deleted")
}

// TestChangePasswordReportsUpdateFailure: the caller must know that the old
// password is still in place, otherwise it tells the user the account is
// secured while nothing changed.
func TestChangePasswordReportsUpdateFailure(t *testing.T) {
	user := userWithHash(t, "user-1", "user@example.com", "OldPassword123")

	repo := &mockAuthRepository{
		getUserByID: func(context.Context, string) (*domain.User, error) { return user, nil },
		updatePassword: func(context.Context, string, string) error {
			return errStorage
		},
		deleteAllSessions: func(context.Context, string) error {
			t.Fatal("sessions must not be revoked when the password was not changed")
			return nil
		},
	}
	svc := newTestService(repo)

	err := svc.ChangePassword(context.Background(), "user-1", "OldPassword123", "NewPassword123")
	require.Error(t, err)
	require.ErrorIs(t, err, errStorage)
}

// TestChangePasswordKeepsSuccessWhenSessionRevocationFails: the password is
// already changed at that point, so returning an error would be a lie. The
// failure has to reach the logs instead.
func TestChangePasswordKeepsSuccessWhenSessionRevocationFails(t *testing.T) {
	user := userWithHash(t, "user-1", "user@example.com", "OldPassword123")

	repo := &mockAuthRepository{
		getUserByID: func(context.Context, string) (*domain.User, error) { return user, nil },
		updatePassword: func(_ context.Context, _ string, newHash string) error {
			_, verifyErr := crypto.VerifyPassword("NewPassword123", newHash)
			require.NoError(t, verifyErr, "the stored hash must verify against the new password")
			return nil
		},
		deleteAllSessions: func(context.Context, string) error { return errStorage },
	}
	svc := newTestService(repo)

	require.NoError(t, svc.ChangePassword(context.Background(), "user-1", "OldPassword123", "NewPassword123"))
}

// TestVerify2FAReportsPersistFailure: 2FA was confirmed but the flag did not
// reach storage. Answering "ok" leaves the user believing a second factor
// protects the account.
func TestVerify2FAReportsPersistFailure(t *testing.T) {
	secret, _, err := crypto.DefaultTOTPConfig("user@example.com").GenerateSecret()
	require.NoError(t, err)
	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)

	user := &domain.User{ID: "user-1", Email: "user@example.com", TwoFASecret: &secret}
	repo := &mockAuthRepository{
		getUserByID: func(context.Context, string) (*domain.User, error) { return user, nil },
		updateUser:   func(context.Context, *domain.User) error { return errStorage },
	}
	svc := newTestService(repo)

	require.ErrorIs(t, svc.Verify2FA(context.Background(), "user-1", code), errStorage)
}

// TestDisable2FAReportsPersistFailure is the mirror image: a secret that stays
// active while the caller was told it was removed.
func TestDisable2FAReportsPersistFailure(t *testing.T) {
	secret, _, err := crypto.DefaultTOTPConfig("user@example.com").GenerateSecret()
	require.NoError(t, err)
	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)

	user := &domain.User{ID: "user-1", Email: "user@example.com", TwoFAEnabled: true, TwoFASecret: &secret}
	repo := &mockAuthRepository{
		getUserByID: func(context.Context, string) (*domain.User, error) { return user, nil },
		updateUser:   func(context.Context, *domain.User) error { return errStorage },
	}
	svc := newTestService(repo)

	require.ErrorIs(t, svc.Disable2FA(context.Background(), "user-1", code), errStorage)
}

// TestEnable2FAReportsPersistFailure: the secret is worthless if it is never
// stored, so the enrollment has to fail loudly.
func TestEnable2FAReportsPersistFailure(t *testing.T) {
	user := &domain.User{ID: "user-1", Email: "user@example.com"}
	repo := &mockAuthRepository{
		getUserByID: func(context.Context, string) (*domain.User, error) { return user, nil },
		updateUser:   func(context.Context, *domain.User) error { return errStorage },
	}
	svc := newTestService(repo)

	secret, qrURI, backupCodes, err := svc.Enable2FA(context.Background(), "user-1")
	require.ErrorIs(t, err, errStorage)
	require.Empty(t, secret, "a secret that was not persisted must not be returned")
	require.Empty(t, qrURI)
	require.Empty(t, backupCodes)
}

// TestLogin2FAFailsWhenTempTokenNotStored: the temp token is the only handle
// the client has for the second step, so handing one out that Redis never
// received strands the login after a successful password check.
func TestLogin2FAFailsWhenTempTokenNotStored(t *testing.T) {
	user := userWithHash(t, "user-1", "user@example.com", "Password123")
	user.TwoFAEnabled = true

	repo := &mockAuthRepository{
		getUserByIdentifier: func(context.Context, string) (*domain.User, error) { return user, nil },
		storeTempToken:      func(context.Context, string, string, time.Duration) error { return errStorage },
	}
	svc := newTestService(repo)

	result, err := svc.Login(context.Background(), &domain.LoginRequest{
		Email:    "user@example.com",
		Password: "Password123",
	})
	require.ErrorIs(t, err, domain.ErrInternal)
	require.Nil(t, result, "no temp token may be returned when it was not stored")
}

// TestLoginUnknownIdentifierTrackingFailureStaysGeneric: failing to record the
// attempt must not change the answer, otherwise the error becomes an
// account-existence oracle.
func TestLoginUnknownIdentifierTrackingFailureStaysGeneric(t *testing.T) {
	repo := &mockAuthRepository{
		getUserByIdentifier: func(context.Context, string) (*domain.User, error) { return nil, nil },
		trackAttempt: func(context.Context, string, string) (int, bool, error) {
			return 0, false, errStorage
		},
	}
	svc := newTestService(repo)

	_, err := svc.Login(context.Background(), &domain.LoginRequest{
		Email:    "ghost@example.com",
		Password: "Password123",
	})
	require.ErrorIs(t, err, domain.ErrInvalidCredentials)
}

// TestLoginSucceedsWhenLockoutBookkeepingFails: clearing the counters and the
// last-login timestamp are bookkeeping. Losing them must not deny a login that
// presented the correct password.
func TestLoginSucceedsWhenLockoutBookkeepingFails(t *testing.T) {
	user := userWithHash(t, "user-1", "user@example.com", "Password123")

	repo := &mockAuthRepository{
		getUserByIdentifier: func(context.Context, string) (*domain.User, error) { return user, nil },
		clearAttempts:       func(context.Context, string, string) error { return errStorage },
		updateLastLogin:     func(context.Context, string) error { return errStorage },
		storeRefreshToken: func(context.Context, string, string, string, time.Duration) error {
			return nil
		},
		createSession: func(_ context.Context, session *domain.Session) error {
			session.ID = "sess-1"
			return nil
		},
	}
	svc := newTestService(repo)

	result, err := svc.Login(context.Background(), &domain.LoginRequest{
		Email:    "user@example.com",
		Password: "Password123",
	})
	require.NoError(t, err)
	require.NotNil(t, result.Tokens)
}

// TestRefreshTokensSurvivesSessionActivityWriteFailure: the rotation already
// happened, so rejecting the new pair would leave the client with a token it
// can never use again. Only the last-activity timestamp is lost.
func TestRefreshTokensSurvivesSessionActivityWriteFailure(t *testing.T) {
	repo := &mockAuthRepository{
		getRefreshToken: func(context.Context, string) (string, string, error) {
			return "user-1", "sess-1", nil
		},
		rotateRefresh: func(context.Context, string, string, time.Duration) (string, string, error) {
			return "user-1", "sess-1", nil
		},
		getSession: func(context.Context, string) (*domain.Session, error) {
			return &domain.Session{ID: "sess-1", UserID: "user-1"}, nil
		},
		createSession: func(context.Context, *domain.Session) error { return errStorage },
	}

	svc, cfg := newRefreshService(t, repo)
	token, err := cfg.GenerateRefreshToken("user-1", "sess-1")
	require.NoError(t, err)

	pair, err := svc.RefreshTokens(context.Background(), token, "device-1")
	require.NoError(t, err)
	require.NotEmpty(t, pair.AccessToken)
	require.NotEmpty(t, pair.RefreshToken)
}
