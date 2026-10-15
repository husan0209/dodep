package main

import (
	"github.com/gofiber/fiber/v2"

	"github.com/opus-casino/kyc/internal/handlers"
	"github.com/opus-casino/kyc/internal/ratelimit"
)

// setupRoutesOnGroup registers the KYC HTTP API on an authenticated group.
// The group must already carry AuthMiddleware, so user_id comes from JWT.
// writeLimiter throttles state-changing endpoints; nil disables the limit.
func setupRoutesOnGroup(api fiber.Router, h *handlers.Handler, writeLimiter *ratelimit.Limiter) {
	// ── User API ──
	api.Post("/documents", IdempotencyMiddleware(), writeGuard(writeLimiter), h.SubmitDocument)
	api.Get("/documents", h.ListDocuments)
	api.Get("/documents/:id", h.GetDocument)
	api.Delete("/documents/:id", h.DeleteDocument)

	api.Get("/status", h.GetStatus)
	api.Get("/level", h.GetLevel)
	api.Post("/upgrade", IdempotencyMiddleware(), writeGuard(writeLimiter), h.RequestUpgrade)

	// ── Admin API (handler enforces the admin claim) ──
	admin := api.Group("/admin")
	admin.Post("/documents/:id/verify", IdempotencyMiddleware(), h.VerifyDocument)
	admin.Post("/documents/:id/reject", IdempotencyMiddleware(), h.RejectDocument)
	admin.Get("/screenings", h.ListScreenings)
	admin.Post("/screenings/:id/resolve", IdempotencyMiddleware(), h.ResolveScreening)
	admin.Post("/sweep-expired", h.SweepExpired)
}

// setupWebhookRoutes registers provider webhooks (HMAC, no JWT).
// Mount on the bare app: POST /api/v1/kyc/webhooks/:provider.
func setupWebhookRoutes(app *fiber.App, h *handlers.Handler, webhookLimiter *ratelimit.Limiter) {
	mw := func(c *fiber.Ctx) error { return c.Next() }
	if webhookLimiter != nil {
		mw = webhookLimiter.Middleware(ratelimit.ByUserOrIP)
	}
	app.Post("/api/v1/kyc/webhooks/:provider", mw, h.Webhook)
}

// writeGuard applies the write rate limit when configured.
// Nil-tolerant so tests mount routes without throttling.
func writeGuard(l *ratelimit.Limiter) fiber.Handler {
	if l == nil {
		return func(c *fiber.Ctx) error { return c.Next() }
	}
	return l.Middleware(ratelimit.ByUserOrIP)
}
