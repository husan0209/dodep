package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/opus-casino/casino/internal/service"
)

// ProviderHandler handles inbound callbacks from game providers.
// Each provider has its own route so nginx can apply per-provider IP allowlists.
type ProviderHandler struct {
	integrationSvc *service.IntegrationService
	log            *zap.Logger
}

// NewProviderHandler creates a new ProviderHandler.
func NewProviderHandler(integrationSvc *service.IntegrationService, log *zap.Logger) *ProviderHandler {
	return &ProviderHandler{
		integrationSvc: integrationSvc,
		log:            log,
	}
}

// RegisterRoutes registers all provider callback routes on the Fiber app.
// Routes intentionally do NOT require JWT auth — they are secured by:
//   - nginx IP allowlist per provider
//   - HMAC signature verification inside ProcessCallback
func (h *ProviderHandler) RegisterRoutes(app *fiber.App) {
	cb := app.Group("/api/v1/casino/providers")

	// Pragmatic Play Seamless Wallet API v3
	cb.Post("/pragmatic/authenticate", h.handleCallback("pragmatic"))
	cb.Post("/pragmatic/balance", h.handleCallback("pragmatic"))
	cb.Post("/pragmatic/bet", h.handleCallback("pragmatic"))
	cb.Post("/pragmatic/result", h.handleCallback("pragmatic"))
	cb.Post("/pragmatic/refund", h.handleCallback("pragmatic"))
	cb.Post("/pragmatic/jackpotWin", h.handleCallback("pragmatic"))
	cb.Post("/pragmatic/promoWin", h.handleCallback("pragmatic"))

	// PG Soft Wallet API
	cb.Get("/pgsoft/balance", h.handleCallback("pgsoft"))
	cb.Post("/pgsoft/bet", h.handleCallback("pgsoft"))
	cb.Post("/pgsoft/win", h.handleCallback("pgsoft"))
	cb.Post("/pgsoft/cancel", h.handleCallback("pgsoft"))
	cb.Post("/pgsoft/freeRoundsAccept", h.handleCallback("pgsoft"))
	cb.Post("/pgsoft/freeRoundsCancel", h.handleCallback("pgsoft"))

	// Amatic Cashier API
	cb.Post("/amatic/getBalance", h.handleCallback("amatic"))
	cb.Post("/amatic/withdraw", h.handleCallback("amatic"))
	cb.Post("/amatic/deposit", h.handleCallback("amatic"))
	cb.Post("/amatic/cancel", h.handleCallback("amatic"))

	// Amusnet REST Wallet API
	cb.Post("/amusnet/balance", h.handleCallback("amusnet"))
	cb.Post("/amusnet/transaction", h.handleCallback("amusnet"))
	cb.Post("/amusnet/rollback", h.handleCallback("amusnet"))
}

// handleCallback returns a Fiber handler that processes a provider callback.
func (h *ProviderHandler) handleCallback(providerName string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Collect headers for signature verification
		headers := make(map[string]string)
		c.Request().Header.VisitAll(func(key, val []byte) {
			headers[string(key)] = string(val)
		})

		body := c.Body()

		resp, err := h.integrationSvc.ProcessCallback(c.Context(), providerName, body, headers)
		if err != nil {
			h.log.Warn("Provider callback failed",
				zap.String("provider", providerName),
				zap.String("path", c.Path()),
				zap.Duration("duration", time.Since(start)),
				zap.Error(err))

			// Most providers expect a specific error format
			return c.Status(fiber.StatusOK).JSON(fiber.Map{
				"error":   1,
				"message": err.Error(),
			})
		}

		h.log.Debug("Provider callback ok",
			zap.String("provider", providerName),
			zap.String("path", c.Path()),
			zap.Duration("duration", time.Since(start)))

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"balance":        resp.Balance.StringFixed(8),
			"transaction_id": resp.TransactionID,
			"error":          0,
		})
	}
}
