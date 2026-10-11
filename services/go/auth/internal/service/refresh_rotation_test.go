package service

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/opus-casino/auth/internal/crypto"
	"github.com/opus-casino/auth/internal/domain"
)

// newRefreshService builds a service with a known JWT config so the test can
// mint real refresh tokens: RefreshTokens verifies the JWT signature before it
// ever touches storage.
func newRefreshService(t *testing.T, repo *mockAuthRepository) (*AuthService, *crypto.JWTConfig) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}
	cfg, err := crypto.NewEd25519JWTConfigFromBase64(
		base64.StdEncoding.EncodeToString(priv),
		base64.StdEncoding.EncodeToString(pub),
	)
	if err != nil {
		t.Fatalf("build jwt config: %v", err)
	}

	return NewAuthService(repo, cfg, zap.NewNop()), cfg
}

// TestRefreshTokensRevokesFamilyOnReuse: a replayed refresh token must not
// just fail — the whole token family and every session of the user has to be
// revoked, because the token is assumed to be stolen.
func TestRefreshTokensRevokesFamilyOnReuse(t *testing.T) {
	var (
		revokedFamily   bool
		revokedSessions bool
	)

	repo := &mockAuthRepository{
		getRefreshToken: func(ctx context.Context, token string) (string, string, error) {
			return "user-1", "sess-1", nil
		},
		rotateRefresh: func(ctx context.Context, current, next string, ttl time.Duration) (string, string, error) {
			return "", "", domain.ErrRefreshTokenReuse
		},
		revokeFamily: func(ctx context.Context, userID, sessionID string) error {
			revokedFamily = true
			return nil
		},
		deleteAllSessions: func(ctx context.Context, userID string) error {
			revokedSessions = true
			return nil
		},
	}

	svc, cfg := newRefreshService(t, repo)
	token, err := cfg.GenerateRefreshToken("user-1", "sess-1")
	if err != nil {
		t.Fatalf("mint refresh token: %v", err)
	}

	_, err = svc.RefreshTokens(context.Background(), token, "device-1")
	if err == nil {
		t.Fatal("reused refresh token must be rejected")
	}
	if !errors.Is(err, domain.ErrInvalidRefreshToken) {
		t.Fatalf("expected ErrInvalidRefreshToken, got %v", err)
	}
	if !revokedFamily {
		t.Fatal("token family must be revoked on reuse")
	}
	if !revokedSessions {
		t.Fatal("all user sessions must be revoked on reuse")
	}
}

// TestRefreshTokensRotatesWithoutRevoking: the happy path must not trigger
// any revocation.
func TestRefreshTokensRotatesWithoutRevoking(t *testing.T) {
	revoked := false

	repo := &mockAuthRepository{
		getRefreshToken: func(ctx context.Context, token string) (string, string, error) {
			return "user-1", "sess-1", nil
		},
		rotateRefresh: func(ctx context.Context, current, next string, ttl time.Duration) (string, string, error) {
			return "user-1", "sess-1", nil
		},
		revokeFamily: func(ctx context.Context, userID, sessionID string) error {
			revoked = true
			return nil
		},
		getSession: func(ctx context.Context, sessionID string) (*domain.Session, error) {
			return &domain.Session{ID: "sess-1", UserID: "user-1"}, nil
		},
	}

	svc, cfg := newRefreshService(t, repo)
	validToken, err := cfg.GenerateRefreshToken("user-1", "sess-1")
	if err != nil {
		t.Fatalf("mint refresh token: %v", err)
	}

	pair, err := svc.RefreshTokens(context.Background(), validToken, "device-1")
	if err != nil {
		t.Fatalf("RefreshTokens: %v", err)
	}
	if pair == nil || pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("expected a full token pair")
	}
	if revoked {
		t.Fatal("valid rotation must not revoke the family")
	}
}

// TestRefreshTokensRejectsForeignToken: a token whose Redis binding points at
// another user must be refused even though the JWT itself verifies.
func TestRefreshTokensRejectsForeignToken(t *testing.T) {
	repo := &mockAuthRepository{
		getRefreshToken: func(ctx context.Context, token string) (string, string, error) {
			// Storage says "user-2" while the token was issued for "user-1".
			return "user-2", "sess-2", nil
		},
		rotateRefresh: func(ctx context.Context, current, next string, ttl time.Duration) (string, string, error) {
			t.Fatal("rotation must not happen for a foreign token")
			return "", "", nil
		},
	}

	svc, cfg := newRefreshService(t, repo)
	foreignToken, err := cfg.GenerateRefreshToken("user-1", "sess-1")
	if err != nil {
		t.Fatalf("mint refresh token: %v", err)
	}
	if _, err := svc.RefreshTokens(context.Background(), foreignToken, "device-1"); err == nil {
		t.Fatal("token bound to another user must be rejected")
	}
}
