package main

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/opus-casino/bonus/internal/domain"
	"github.com/opus-casino/bonus/internal/service"
	"github.com/opus-casino/bonus/internal/testutil"
)

const routesTestSecret = "routes-test-secret"

func testApp() (*fiber.App, *testutil.FakeBonusRepository) {
	repo := testutil.NewFakeBonusRepository()
	svc := service.NewBonusServiceWithRepository(repo, service.BonusConfig{
		WelcomePct:        100,
		WelcomeMaxUSD:     decimal.NewFromInt(200),
		WelcomeWagering:   30,
		WelcomeExpiryDays: 30,
	}, zap.NewNop())

	app := fiber.New()
	group := app.Group("/api/v1/bonuses", AuthMiddleware(routesTestSecret))
	setupRoutesOnGroup(group, svc, nil) // no throttling in handler tests
	return app, repo
}

func signedToken(t *testing.T, userID string) string {
	t.Helper()
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, JWTClaims{
		UserID:           userID,
		RegisteredClaims: jwt.RegisteredClaims{IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour))},
	})
	signed, err := token.SignedString([]byte(routesTestSecret))
	require.NoError(t, err)
	return signed
}

func doRequest(t *testing.T, app *fiber.App, method, target, token, idempotencyKey string) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if idempotencyKey != "" {
		req.Header.Set(IdempotencyKeyHeader, idempotencyKey)
	}
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var parsed map[string]interface{}
	if len(body) > 0 {
		require.NoError(t, json.Unmarshal(body, &parsed))
	}
	return resp.StatusCode, parsed
}

func TestRoutes_UnauthorizedWithoutToken(t *testing.T) {
	app, _ := testApp()
	status, _ := doRequest(t, app, "GET", "/api/v1/bonuses/", "", "")
	assert.Equal(t, 401, status)
}

func TestRoutes_ListAndWagering(t *testing.T) {
	app, repo := testApp()
	token := signedToken(t, "71")

	bonus := &domain.Bonus{
		ID: idemUUID("71-active"), UserID: 71, Type: domain.BonusTypeWelcome,
		Status: domain.BonusStatusActive, BonusAmount: decimal.NewFromInt(100),
		Currency: "USD", WageringRequired: decimal.NewFromInt(3000),
		WageringCompleted: decimal.NewFromInt(1500),
	}
	repo.Bonuses[bonus.ID] = bonus

	status, body := doRequest(t, app, "GET", "/api/v1/bonuses/", token, "")
	require.Equal(t, 200, status)
	assert.Equal(t, float64(1), body["total"])

	status, wagering := doRequest(t, app, "GET", "/api/v1/bonuses/"+bonus.ID.String()+"/wagering", token, "")
	require.Equal(t, 200, status)
	assert.Equal(t, "1500", wagering["completed"])
	assert.Equal(t, "1500", wagering["remaining"])
	assert.Equal(t, false, wagering["is_completed"])
}

func TestRoutes_ActivateRequiresIdempotencyKey(t *testing.T) {
	app, repo := testApp()
	token := signedToken(t, "72")

	pending := &domain.Bonus{
		ID: idemUUID("72-pending"), UserID: 72, Type: domain.BonusTypeReload,
		Status: domain.BonusStatusPending, BonusAmount: decimal.NewFromInt(25),
		Currency: "USD", WageringRequired: decimal.NewFromInt(750),
	}
	repo.Bonuses[pending.ID] = pending

	status, _ := doRequest(t, app, "POST", "/api/v1/bonuses/"+pending.ID.String()+"/activate", token, "")
	assert.Equal(t, 400, status, "mutating request without idempotency key must be rejected")

	status, body := doRequest(t, app, "POST", "/api/v1/bonuses/"+pending.ID.String()+"/activate", token, uuid.NewString())
	require.Equal(t, 200, status)
	assert.Equal(t, string(domain.BonusStatusActive), body["Status"])
}

func TestRoutes_CannotReadOtherUsersBonus(t *testing.T) {
	app, repo := testApp()
	token := signedToken(t, "73")

	other := &domain.Bonus{
		ID: idemUUID("74-other"), UserID: 74, Type: domain.BonusTypeWelcome,
		Status: domain.BonusStatusActive, BonusAmount: decimal.NewFromInt(10),
		Currency: "USD", WageringRequired: decimal.NewFromInt(300),
	}
	repo.Bonuses[other.ID] = other

	status, _ := doRequest(t, app, "GET", "/api/v1/bonuses/"+other.ID.String(), token, "")
	assert.Equal(t, 404, status, "user must not read another user's bonus")
}

// idemUUID derives a deterministic UUID from a name for stable test fixtures.
func idemUUID(name string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(name))
}
