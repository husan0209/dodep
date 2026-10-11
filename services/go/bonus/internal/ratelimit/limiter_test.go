package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixedNow(t time.Time) func() time.Time { return func() time.Time { return t } }

func TestAllow_WithinBudget(t *testing.T) {
	l := New(Config{Name: "read", Limit: 3, Window: time.Minute})
	now := time.Unix(1_700_000_000, 0)

	for i := 1; i <= 3; i++ {
		res := l.Allow("user:1", now)
		assert.True(t, res.Allowed, "request %d must be allowed", i)
		assert.Equal(t, 3-i, res.Remaining)
		assert.Equal(t, 3, res.Limit)
	}
	res := l.Allow("user:1", now)
	assert.False(t, res.Allowed, "4th request exceeds the budget")
	assert.Equal(t, 0, res.Remaining)
}

func TestAllow_WindowResets(t *testing.T) {
	l := New(Config{Limit: 1, Window: time.Minute})
	now := time.Unix(1_700_000_000, 0)

	assert.True(t, l.Allow("user:1", now).Allowed)
	assert.False(t, l.Allow("user:1", now).Allowed)

	// Next window: budget is fresh again.
	later := now.Add(time.Minute)
	assert.True(t, l.Allow("user:1", later).Allowed, "window boundary must reset the budget")
}

func TestAllow_KeysAreIsolated(t *testing.T) {
	l := New(Config{Limit: 1, Window: time.Minute})
	now := time.Unix(1_700_000_000, 0)

	assert.True(t, l.Allow("user:1", now).Allowed)
	assert.False(t, l.Allow("user:1", now).Allowed)
	assert.True(t, l.Allow("user:2", now).Allowed, "a different key has its own budget")
}

func TestAllow_ResetTimestampIsWindowEnd(t *testing.T) {
	l := New(Config{Limit: 5, Window: 30 * time.Second})
	now := time.Unix(1_700_000_000, 0)
	res := l.Allow("user:1", now)
	assert.Equal(t, now.Add(30*time.Second).Unix(), res.Reset)
}

func TestNew_NormalisesInvalidConfig(t *testing.T) {
	l := New(Config{}) // zero value must not panic and must stay usable
	assert.Equal(t, "default", l.Name())
	assert.True(t, l.Allow("k", time.Now()).Allowed)
}

func TestAllow_EvictsWhenMapIsFull(t *testing.T) {
	// maxKeys=4 → after 4 distinct keys the next hit triggers eviction.
	l := New(Config{Limit: 1, maxKeys: 4, Window: time.Minute})
	now := time.Unix(1_700_000_000, 0)
	for i := 0; i < 50; i++ {
		l.Allow("k"+strconv.Itoa(i), now.Add(time.Duration(i)*time.Millisecond))
	}
	assert.LessOrEqual(t, len(l.buckets), 4, "map must stay bounded under key churn")
}

func TestMiddleware_429WithHeaders(t *testing.T) {
	l := New(Config{Name: "mutate", Limit: 1, Window: time.Minute})
	app := fiber.New()
	app.Use(l.Middleware(nil))
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendString("ok") })

	// First request passes and carries the budget headers.
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil), -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "1", resp.Header.Get("X-RateLimit-Limit"))
	assert.Equal(t, "0", resp.Header.Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, resp.Header.Get("X-RateLimit-Reset"))

	// Second is rejected with 429 + Retry-After.
	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/x", nil), -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get("Retry-After"))
}

func TestMiddleware_EmptyKeyFailsOpen(t *testing.T) {
	l := New(Config{Limit: 1, Window: time.Minute})
	app := fiber.New()
	// A key func that cannot identify the caller must not lock the request out.
	app.Use(l.Middleware(func(*fiber.Ctx) string { return "" }))
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendString("ok") })

	for i := 0; i < 5; i++ {
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil), -1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}
}

func TestByUserOrIP_PrefersAuthenticatedUser(t *testing.T) {
	l := New(Config{Limit: 1, Window: time.Minute})
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", "42")
		return c.Next()
	})
	app.Use(l.Middleware(ByUserOrIP))
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendString("ok") })

	for i := 0; i < 3; i++ {
		_, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil), -1)
		require.NoError(t, err)
	}
	// All three hit the same "user:42" budget, so the third must be blocked.
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil), -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
}

func TestByUserOrIP_FallsBackToIP(t *testing.T) {
	app := fiber.New()
	var key string
	app.Use(func(c *fiber.Ctx) error {
		key = ByUserOrIP(c)
		return c.Next()
	})
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendString("ok") })

	_, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil), -1)
	require.NoError(t, err)
	assert.Contains(t, key, "ip:", "unauthenticated traffic must key on IP, got %q", key)
}

func TestClose_IsIdempotent(t *testing.T) {
	l := New(Config{})
	assert.NotPanics(t, func() {
		l.Close()
		l.Close()
	})
}

var _ = fixedNow
