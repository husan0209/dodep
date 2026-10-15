package main

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/opus-casino/kyc/internal/cache"
	"github.com/opus-casino/kyc/internal/crypto"
	"github.com/opus-casino/kyc/internal/handlers"
	"github.com/opus-casino/kyc/internal/screening"
	"github.com/opus-casino/kyc/internal/service"
	"github.com/opus-casino/kyc/internal/testutil"
)

const testJWTSecret = "test-secret-0123456789abcdef"

func signToken(t *testing.T, userID, role string, admin bool) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, JWTClaims{
		UserID:  userID,
		Role:    role,
		IsAdmin: admin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	s, err := tok.SignedString([]byte(testJWTSecret))
	require.NoError(t, err)
	return s
}

func TestAuthMiddleware_Valid(t *testing.T) {
	app := fiber.New()
	app.Use(AuthMiddleware(testJWTSecret))
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"user_id": getUserID(c), "admin": c.Locals("is_admin")})
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+signToken(t, "42", "", false))
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestAuthMiddleware_AdminClaim(t *testing.T) {
	app := fiber.New()
	app.Use(AuthMiddleware(testJWTSecret))
	app.Get("/", func(c *fiber.Ctx) error {
		admin, _ := c.Locals("is_admin").(bool)
		if !admin {
			return c.SendStatus(403)
		}
		return c.SendStatus(200)
	})

	for _, tok := range []string{
		signToken(t, "1", "admin", false),
		signToken(t, "1", "", true),
	} {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
		_ = resp.Body.Close()
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+signToken(t, "1", "user", false))
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 403, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestAuthMiddleware_Rejections(t *testing.T) {
	app := fiber.New()
	app.Use(AuthMiddleware(testJWTSecret))
	app.Get("/", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	cases := []struct {
		name   string
		header string
	}{
		{"missing header", ""},
		{"bad scheme", "Token abc"},
		{"garbage token", "Bearer garbage"},
		{"wrong secret", "Bearer " + mustSignHS256(t, "other-secret")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			resp, err := app.Test(req)
			require.NoError(t, err)
			assert.Equal(t, 401, resp.StatusCode)
			_ = resp.Body.Close()
		})
	}
}

func mustSignHS256(t *testing.T, secret string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, JWTClaims{
		UserID:           "1",
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	})
	s, err := tok.SignedString([]byte(secret))
	require.NoError(t, err)
	return s
}

func TestIdempotencyMiddleware(t *testing.T) {
	app := fiber.New()
	app.Post("/", IdempotencyMiddleware(), func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest("POST", "/", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
	_ = resp.Body.Close()

	req = httptest.NewRequest("POST", "/", nil)
	req.Header.Set(IdempotencyKeyHeader, "not-a-uuid")
	resp, err = app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
	_ = resp.Body.Close()

	req = httptest.NewRequest("POST", "/", nil)
	req.Header.Set(IdempotencyKeyHeader, "123e4567-e89b-12d3-a456-426614174000")
	resp, err = app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestSecurityHeaders(t *testing.T) {
	app := fiber.New()
	app.Use(SecurityHeaders())
	app.Get("/", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	_ = resp.Body.Close()
}

func TestSetupRoutes_Mounts(t *testing.T) {
	repo := testutil.NewFakeRepo()
	cr, err := crypto.Ephemeral()
	require.NoError(t, err)
	svc := service.NewKYCService(repo, cache.NewMemory(), screening.StubProvider{}, cr,
		service.Config{}, zap.NewNop())
	h := handlers.NewHandler(svc, nil)

	app := fiber.New()
	protected := app.Group("/api/v1/kyc", AuthMiddleware(testJWTSecret))
	setupRoutesOnGroup(protected, h, nil)

	// No token → 401 from AuthMiddleware (route exists, auth enforced).
	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/kyc/level", nil))
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, 401, resp.StatusCode)

	// Valid token → passes auth (handler returns 200 with empty record).
	req := httptest.NewRequest("GET", "/api/v1/kyc/level", nil)
	req.Header.Set("Authorization", "Bearer "+signToken(t, "42", "", false))
	resp, err = app.Test(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
}
