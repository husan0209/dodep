package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/opus-casino/auth/internal/domain"
)

// newTestRepository connects to a real Redis instance (skips when absent).
// A real server is required: the rotation logic relies on GETDEL and TTL.
func newTestRepository(t *testing.T) *AuthRepository {
	t.Helper()

	addr := os.Getenv("AUTH_TEST_REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: os.Getenv("AUTH_TEST_REDIS_PASSWORD"),
		DB:       15, // dedicated test database
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available at %s, skipping integration test: %v", addr, err)
	}

	// Start from a clean database so assertions about key sets are stable.
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Skipf("cannot flush test database: %v", err)
	}

	t.Cleanup(func() {
		_ = client.FlushDB(context.Background()).Err()
		_ = client.Close()
	})

	// The session/user methods are not exercised here, so a nil pool is fine.
	return &AuthRepository{redis: client}
}

// refreshTokenActive reports whether a stored refresh token still resolves.
// GetRefreshToken signals "missing" with empty user/session IDs and a nil
// error, so callers must check the values, not only the error.
func refreshTokenActive(t *testing.T, repo *AuthRepository, token string) bool {
	t.Helper()
	userID, sessionID, err := repo.GetRefreshToken(context.Background(), token)
	if err != nil {
		t.Fatalf("GetRefreshToken(%s): %v", token, err)
	}
	return userID != "" && sessionID != ""
}

func TestLoginLockoutSurvivesIPRotation(t *testing.T) {
	repo := newTestRepository(t)
	ctx := context.Background()

	const email = "Player@Example.com"

	// Five failures spread over five different source IPs: the account counter
	// must still trip the lockout, otherwise brute force from a botnet is
	// unlimited (one counter per IP used to hide it).
	ips := []string{"10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.4", "10.0.0.5"}
	for i, ip := range ips {
		attempts, locked, err := repo.TrackLoginAttempt(ctx, email, ip)
		if err != nil {
			t.Fatalf("TrackLoginAttempt(%s): %v", ip, err)
		}
		if attempts != i+1 {
			t.Fatalf("expected account attempt count %d, got %d", i+1, attempts)
		}
		if i < 4 && locked {
			t.Fatalf("account must not lock before the threshold (attempt %d)", i+1)
		}
		if i == 4 && !locked {
			t.Fatal("account must lock after 5 failed attempts regardless of IP")
		}
	}

	locked, err := repo.IsAccountLocked(ctx, email)
	if err != nil {
		t.Fatalf("IsAccountLocked: %v", err)
	}
	if !locked {
		t.Fatal("account must be reported as locked (email is matched case-insensitively)")
	}

	// A different account is unaffected.
	otherLocked, err := repo.IsAccountLocked(ctx, "someone-else@example.com")
	if err != nil {
		t.Fatalf("IsAccountLocked(other): %v", err)
	}
	if otherLocked {
		t.Fatal("locking one account must not lock another")
	}
}

func TestLoginLockoutPerIPAcrossAccounts(t *testing.T) {
	repo := newTestRepository(t)
	ctx := context.Background()

	const ip = "203.0.113.7"

	// Ten failures for different accounts from one IP must lock that source,
	// so a single host cannot spray many accounts.
	var lockedAt int
	for i := 0; i < 12; i++ {
		_, locked, err := repo.TrackLoginAttempt(ctx, fmt.Sprintf("user%d@example.com", i), ip)
		if err != nil {
			t.Fatalf("TrackLoginAttempt: %v", err)
		}
		if locked && lockedAt == 0 {
			lockedAt = i + 1
		}
	}
	if lockedAt == 0 {
		t.Fatal("IP-based lockout never triggered")
	}
	if lockedAt > 12 {
		t.Fatalf("unexpected lockout trigger at %d", lockedAt)
	}
}

func TestSuccessfulLoginClearsAttemptsAndLock(t *testing.T) {
	repo := newTestRepository(t)
	ctx := context.Background()

	const email = "player@example.com"
	const ip = "198.51.100.4"

	for i := 0; i < 5; i++ {
		if _, _, err := repo.TrackLoginAttempt(ctx, email, ip); err != nil {
			t.Fatalf("TrackLoginAttempt: %v", err)
		}
	}
	locked, err := repo.IsAccountLocked(ctx, email)
	if err != nil || !locked {
		t.Fatalf("expected locked account, got locked=%v err=%v", locked, err)
	}

	if err := repo.ClearLoginAttempts(ctx, email, ip); err != nil {
		t.Fatalf("ClearLoginAttempts: %v", err)
	}

	locked, err = repo.IsAccountLocked(ctx, email)
	if err != nil {
		t.Fatalf("IsAccountLocked after clear: %v", err)
	}
	if locked {
		t.Fatal("successful login must lift the lockout")
	}

	// Counters are reset as well: five more failures are needed to lock again.
	for i := 0; i < 5; i++ {
		_, locked, err := repo.TrackLoginAttempt(ctx, email, ip)
		if err != nil {
			t.Fatalf("TrackLoginAttempt after clear: %v", err)
		}
		if i < 4 && locked {
			t.Fatalf("counters were not reset (locked at attempt %d)", i+1)
		}
	}
}

func TestRefreshTokenRotationDetectsReuse(t *testing.T) {
	repo := newTestRepository(t)
	ctx := context.Background()

	const (
		userID    = "user-1"
		sessionID = "sess-1"
	)
	ttl := 7 * 24 * time.Hour

	const tokenA, tokenB = "token-A", "token-B"
	if err := repo.StoreRefreshToken(ctx, tokenA, userID, sessionID, ttl); err != nil {
		t.Fatalf("StoreRefreshToken: %v", err)
	}

	// First rotation succeeds and issues token B.
	gotUser, gotSession, err := repo.RotateRefreshToken(ctx, tokenA, tokenB, ttl)
	if err != nil {
		t.Fatalf("RotateRefreshToken: %v", err)
	}
	if gotUser != userID || gotSession != sessionID {
		t.Fatalf("unexpected rotation result: %s/%s", gotUser, gotSession)
	}
	if !refreshTokenActive(t, repo, tokenB) {
		t.Fatal("new token must be active after rotation")
	}
	if refreshTokenActive(t, repo, tokenA) {
		t.Fatal("rotated token must no longer be active")
	}

	// Replaying the consumed token must be reported as reuse, not as an
	// ordinary invalid token — that is what triggers family revocation.
	_, _, err = repo.RotateRefreshToken(ctx, tokenA, "token-C", ttl)
	if !errors.Is(err, domain.ErrRefreshTokenReuse) {
		t.Fatalf("expected reuse error, got %v", err)
	}

	// An unknown token is simply invalid, never a reuse signal.
	_, _, err = repo.RotateRefreshToken(ctx, "never-issued", "token-D", ttl)
	if err != domain.ErrInvalidRefreshToken {
		t.Fatalf("expected ErrInvalidRefreshToken for unknown token, got %v", err)
	}
}

func TestRevokeRefreshTokenFamily(t *testing.T) {
	repo := newTestRepository(t)
	ctx := context.Background()

	const (
		userID    = "user-2"
		sessionID = "sess-2"
		otherUser = "user-3"
	)
	ttl := time.Hour

	tokens := []string{"t1", "t2", "t3"}
	for _, token := range tokens {
		if err := repo.StoreRefreshToken(ctx, token, userID, sessionID, ttl); err != nil {
			t.Fatalf("StoreRefreshToken: %v", err)
		}
	}
	if err := repo.StoreRefreshToken(ctx, "other", otherUser, sessionID, ttl); err != nil {
		t.Fatalf("StoreRefreshToken(other): %v", err)
	}

	if err := repo.RevokeRefreshTokenFamily(ctx, userID, ""); err != nil {
		t.Fatalf("RevokeRefreshTokenFamily: %v", err)
	}

	for _, token := range tokens {
		if refreshTokenActive(t, repo, token) {
			t.Fatalf("token %s of the revoked family must be gone", token)
		}
	}
	if !refreshTokenActive(t, repo, "other") {
		t.Fatal("another user's token must survive the revocation")
	}
}

func TestStoreRefreshTokenRejectsEmptyPayload(t *testing.T) {
	repo := newTestRepository(t)
	ctx := context.Background()

	// A payload without a session must not be rotatable.
	if err := repo.StoreRefreshToken(ctx, "broken", "user", "", time.Hour); err != nil {
		t.Fatalf("StoreRefreshToken: %v", err)
	}
	if _, _, err := repo.RotateRefreshToken(ctx, "broken", "next", time.Hour); err != domain.ErrInvalidRefreshToken {
		t.Fatalf("expected ErrInvalidRefreshToken for malformed payload, got %v", err)
	}
}