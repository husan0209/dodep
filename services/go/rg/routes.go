package main

import (
	"github.com/gofiber/fiber/v2"

	"github.com/opus-casino/rg/internal/handlers"
	"github.com/opus-casino/rg/internal/service"
)

// setupRoutes wires player-facing RG endpoints.
// AuthMiddleware runs before these handlers, so user_id comes from JWT.
func setupRoutes(app *fiber.App, h *handlers.Handler, svc *service.RGService, auth fiber.Handler) {
	v1 := app.Group("/api/v1")

	rg := v1.Group("/rg", auth)
	rg.Post("/check", h.Check)
	rg.Put("/limits", h.SetLimits)
	rg.Get("/limits", h.GetLimits)
	rg.Post("/self-exclusion", h.StartSelfExclusion)
	rg.Post("/self-exclusion/revoke", h.RevokeSelfExclusion)
	rg.Post("/timeout", h.StartTimeout)
	rg.Get("/status", h.GetStatus)
}

// setupAdminRoutes wires operator endpoints behind AdminMiddleware.
func setupAdminRoutes(app *fiber.App, h *handlers.Handler, svc *service.RGService, admin fiber.Handler) {
	group := app.Group("/admin/rg", admin)
	group.Post("/exclusions", h.AdminStartExclusion)
	group.Post("/apply-due", h.AdminApplyDue(svc))
}
