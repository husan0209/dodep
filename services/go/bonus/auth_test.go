package main

import (
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/opus-casino/bonus/internal/domain"
	"github.com/opus-casino/bonus/internal/service"
	"github.com/opus-casino/bonus/internal/testutil"
)

// testAppNoAuth mounts the bonus routes WITHOUT AuthMiddleware to prove the
// handlers fail closed when no identity is present (regression: the old
// parseTokenUserID returned c.JSON() — nil on success — as error, so callers
// continued with userID 0).
func testAppNoAuth() *fiber.App {
	repo := testutil.NewFakeBonusRepository()
	svc := service.NewBonusServiceWithRepository(repo, service.BonusConfig{
		WelcomePct:        100,
		WelcomeMaxUSD:     decimal.NewFromInt(200),
		WelcomeWagering:   30,
		WelcomeExpiryDays: 30,
	}, zap.NewNop())

	app := fiber.New()
	group := app.Group("/api/v1/bonuses") // no AuthMiddleware on purpose
	setupRoutesOnGroup(group, svc, nil)
	return app
}

func TestUserIDFromToken_MissingMiddlewareFailsClosed(t *testing.T) {
	app := testAppNoAuth()

	for _, target := range []string{
		"/api/v1/bonuses/",
		"/api/v1/bonuses/active",
	} {
		status, body := doRequest(t, app, "GET", target, "", "")
		assert.Equal(t, 401, status, "GET %s without identity must be 401", target)
		require.Contains(t, body["error"], "unauthorized")
	}

	// Mutating endpoint without identity must also be 401 (idempotency key is
	// supplied so the request reaches the identity check).
	status, _ := doRequest(t, app, "POST", "/api/v1/bonuses/"+uuid.NewString()+"/activate", "", uuid.NewString())
	assert.Equal(t, 401, status)
}

func TestUserIDFromToken_InvalidIdentitiesRejected(t *testing.T) {
	app, _ := testApp()

	for _, badID := range []string{"0", "-5", "abc", "12x"} {
		token := signedToken(t, badID)
		status, body := doRequest(t, app, "GET", "/api/v1/bonuses/", token, "")
		assert.Equal(t, 401, status, "user_id %q must be rejected", badID)
		require.Contains(t, body["error"], "unauthorized")
	}
}

func TestUserIDFromToken_ValidIdentityPasses(t *testing.T) {
	app, repo := testApp()
	token := signedToken(t, "91")

	repo.Bonuses[idemUUID("91-one")] = &domain.Bonus{
		ID: idemUUID("91-one"), UserID: 91, Type: domain.BonusTypeWelcome,
		Status: domain.BonusStatusActive, BonusAmount: decimal.NewFromInt(10),
		Currency: "USD", WageringRequired: decimal.NewFromInt(300),
	}

	status, body := doRequest(t, app, "GET", "/api/v1/bonuses/", token, "")
	require.Equal(t, 200, status)
	assert.Equal(t, float64(1), body["total"])
}
