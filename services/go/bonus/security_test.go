package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/opus-casino/bonus/internal/domain"
	"github.com/opus-casino/bonus/internal/ratelimit"
	"github.com/opus-casino/bonus/internal/service"
	"github.com/opus-casino/bonus/internal/testutil"
)

func TestSecurityHeaders_AppliedToEveryResponse(t *testing.T) {
	app := fiber.New()
	app.Use(SecurityHeaders())
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendString("ok") })

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil), -1)
	require.NoError(t, err)

	for header, want := range map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"Referrer-Policy":           "no-referrer",
		"X-Dns-Prefetch-Control":    "off",
		"Permissions-Policy":        "camera=(), microphone=(), geolocation=()",
		"Strict-Transport-Security": "max-age=63072000; includeSubDomains",
	} {
		assert.Equal(t, want, resp.Header.Get(header), "header %s", header)
	}
}

func TestSecurityHeaders_AppliedOnErrorsToo(t *testing.T) {
	app := fiber.New()
	app.Use(SecurityHeaders())
	app.Get("/boom", func(_ *fiber.Ctx) error { return fiber.ErrTeapot })

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/boom", nil), -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusTeapot, resp.StatusCode)
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"), "headers must survive error responses")
}

func TestBodyLimit_RejectsOversizedBody(t *testing.T) {
	app := fiber.New(fiber.Config{BodyLimit: 1024}) // 1 KiB for the test
	app.Use(SecurityHeaders())
	app.Post("/x", func(c *fiber.Ctx) error { return c.SendString("ok") })

	huge := strings.NewReader(strings.Repeat("a", 4096))
	req := httptest.NewRequest(http.MethodPost, "/x", huge)
	// fasthttp aborts the read before the handler runs, so the request fails
	// instead of reaching the route: that IS the protection.
	_, err := app.Test(req, -1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "body size exceeds the given limit")
}

// TestWriteRateLimit_AppliedToMutatingRoutes proves the write limiter is
// actually mounted on activate/cancel (the money-adjacent endpoints).
func TestWriteRateLimit_AppliedToMutatingRoutes(t *testing.T) {
	repo := testutil.NewFakeBonusRepository()
	svc := service.NewBonusServiceWithRepository(repo, service.BonusConfig{
		WelcomePct:        100,
		WelcomeMaxUSD:     decimal.NewFromInt(200),
		WelcomeWagering:   30,
		WelcomeExpiryDays: 30,
	}, zap.NewNop())

	app := fiber.New()
	group := app.Group("/api/v1/bonuses", AuthMiddleware(routesTestSecret))
	limiter := ratelimit.New(ratelimit.Config{Name: "write", Limit: 2, Window: time.Minute})
	setupRoutesOnGroup(group, svc, limiter)

	// Seed a pending bonus owned by user 81.
	pending := &domain.Bonus{
		ID:               idemUUID("81-pending"),
		UserID:           81,
		Type:             domain.BonusTypeReload,
		Status:           domain.BonusStatusPending,
		BonusAmount:      decimal.NewFromInt(25),
		Currency:         "USD",
		WageringRequired: decimal.NewFromInt(750),
	}
	repo.Bonuses[pending.ID] = pending

	token := signedToken(t, "81")
	url := "/api/v1/bonuses/" + pending.ID.String() + "/cancel"

	// First two calls pass the limiter (404/409 are business errors, not 429).
	for i := 0; i < 2; i++ {
		status, _ := doRequest(t, app, "POST", url, token, uuid.NewString())
		assert.NotEqual(t, http.StatusTooManyRequests, status, "call %d must not be rate limited", i+1)
	}
	// Third call is throttled.
	status, body := doRequest(t, app, "POST", url, token, uuid.NewString())
	assert.Equal(t, http.StatusTooManyRequests, status)
	assert.Equal(t, "RATE_LIMIT_EXCEEDED", body["error"])
}
