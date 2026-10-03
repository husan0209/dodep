package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opus-casino/auth/internal/crypto"
	"github.com/opus-casino/auth/internal/domain"
)

func TestResetPassword_RejectsUnknownToken(t *testing.T) {
	repo := &mockAuthRepository{
		getTempToken: func(context.Context, string) (string, error) { return "", nil },
	}
	svc := newTestService(repo)

	err := svc.ResetPassword(context.Background(), "no-such-token", "new-password-1")
	require.ErrorIs(t, err, domain.ErrInvalidToken)
}

func TestResetPassword_ValidatesInput(t *testing.T) {
	svc := newTestService(&mockAuthRepository{})

	require.ErrorIs(t,
		svc.ResetPassword(context.Background(), "", "new-password-1"),
		domain.ErrValidation)
	require.ErrorIs(t,
		svc.ResetPassword(context.Background(), "token", "short"),
		domain.ErrValidation)
}

func TestResetPassword_UpdatesHashAndRevokesSessions(t *testing.T) {
	var (
		storedHash    string
		tokenConsumed bool
		sessionsGone  []string
	)

	repo := &mockAuthRepository{
		getTempToken: func(_ context.Context, token string) (string, error) {
			require.Equal(t, "reset-token", token)
			return "user-1", nil
		},
		updatePassword: func(_ context.Context, userID, hash string) error {
			require.Equal(t, "user-1", userID)
			storedHash = hash
			return nil
		},
		deleteTempToken: func(_ context.Context, token string) error {
			require.Equal(t, "reset-token", token)
			tokenConsumed = true
			return nil
		},
		deleteAllSessions: func(_ context.Context, userID string) error {
			sessionsGone = append(sessionsGone, userID)
			return nil
		},
	}
	svc := newTestService(repo)

	require.NoError(t, svc.ResetPassword(context.Background(), "reset-token", "brand-new-password"))

	// The new password must be stored hashed, never in the clear.
	require.NotEmpty(t, storedHash)
	require.NotEqual(t, "brand-new-password", storedHash)
	valid, err := crypto.VerifyPassword("brand-new-password", storedHash)
	require.NoError(t, err)
	require.True(t, valid)

	require.True(t, tokenConsumed, "the reset token must be single use")
	require.Equal(t, []string{"user-1"}, sessionsGone)
}

// A password that could not be stored must not consume the token, or the user
// is left holding a token that no longer works.
func TestResetPassword_KeepsTokenWhenUpdateFails(t *testing.T) {
	tokenConsumed := false

	repo := &mockAuthRepository{
		getTempToken: func(context.Context, string) (string, error) { return "user-1", nil },
		updatePassword: func(context.Context, string, string) error {
			return errors.New("db down")
		},
		deleteTempToken: func(context.Context, string) error {
			tokenConsumed = true
			return nil
		},
	}
	svc := newTestService(repo)

	err := svc.ResetPassword(context.Background(), "reset-token", "brand-new-password")
	require.ErrorIs(t, err, domain.ErrInternal)
	require.False(t, tokenConsumed)
}

func TestResetPasswordRequest_IssuesTokenForKnownUser(t *testing.T) {
	var issuedFor string

	repo := &mockAuthRepository{
		getUserByEmail: func(_ context.Context, email string) (*domain.User, error) {
			require.Equal(t, "player@example.com", email)
			return &domain.User{ID: "user-1", Email: email}, nil
		},
		storeTempToken: func(_ context.Context, token, userID string, _ time.Duration) error {
			require.NotEmpty(t, token)
			issuedFor = userID
			return nil
		},
	}
	svc := newTestService(repo)

	require.NoError(t, svc.ResetPasswordRequest(context.Background(), "player@example.com", "127.0.0.1"))
	require.Equal(t, "user-1", issuedFor)
}

// An unknown address must not be distinguishable from a known one, and no
// token may be issued for it.
func TestResetPasswordRequest_UnknownUserIsIndistinguishable(t *testing.T) {
	repo := &mockAuthRepository{
		getUserByEmail: func(context.Context, string) (*domain.User, error) { return nil, nil },
		storeTempToken: func(context.Context, string, string, time.Duration) error {
			t.Fatal("no token may be issued for an unknown address")
			return nil
		},
	}
	svc := newTestService(repo)

	require.NoError(t, svc.ResetPasswordRequest(context.Background(), "nobody@example.com", "127.0.0.1"))
}