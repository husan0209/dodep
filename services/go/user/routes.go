package main

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v2"

	"github.com/opus-casino/user/internal/domain"
	"github.com/opus-casino/user/internal/service"
)

// setupRoutes mounts the authenticated player API.
//
// Route shape changed deliberately: the previous surface was
// `/api/v1/users/:id` with no authentication, so any anonymous caller could
// enumerate the `users.id` sequence and read or rewrite any player's profile,
// preferences and gambling limits. There is no internal consumer to preserve —
// no service, frontend or Helm chart calls this HTTP API, so the identifier is
// removed from the path entirely rather than merely checked against the token.
//
// `/api/v1/users/me` has no identifier in the URL at all, which is the literal
// reading of CONVENTIONS.md (NEVER-7). Service-to-service reads of another
// player go through the gRPC surface (internal/handlers), not these routes.
func setupRoutes(app *fiber.App, svc *service.UserService, verifier *tokenVerifier) {
	api := app.Group("/api/v1", AuthMiddleware(verifier))
	me := api.Group("/users/me")

	me.Get("/", func(c *fiber.Ctx) error {
		userID, ok := authenticatedUserID(c)
		if !ok {
			return unauthorized(c, errTokenIdentity)
		}
		user, err := svc.GetUser(c.Context(), userID)
		if err != nil {
			return respondUserError(c, err, "get user")
		}
		return c.JSON(user)
	})

	me.Put("/", func(c *fiber.Ctx) error {
		userID, ok := authenticatedUserID(c)
		if !ok {
			return unauthorized(c, errTokenIdentity)
		}
		var req domain.UpdateUserRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).
				JSON(fiber.Map{"error": "invalid request body"})
		}
		// Overwritten unconditionally: a client-supplied user_id in the body
		// must never be able to select whose profile is updated.
		req.UserID = userID

		user, err := svc.UpdateUser(c.Context(), &req)
		if err != nil {
			return respondUserError(c, err, "update user")
		}
		return c.JSON(user)
	})

	me.Get("/preferences", func(c *fiber.Ctx) error {
		userID, ok := authenticatedUserID(c)
		if !ok {
			return unauthorized(c, errTokenIdentity)
		}
		pref, err := svc.GetPreferences(c.Context(), userID)
		if err != nil {
			return respondUserError(c, err, "get preferences")
		}
		return c.JSON(pref)
	})

	me.Put("/preferences", func(c *fiber.Ctx) error {
		userID, ok := authenticatedUserID(c)
		if !ok {
			return unauthorized(c, errTokenIdentity)
		}
		var req domain.UserPreferences
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).
				JSON(fiber.Map{"error": "invalid request body"})
		}
		req.UserID = userID
		pref, err := svc.UpdatePreferences(c.Context(), &req)
		if err != nil {
			return respondUserError(c, err, "update preferences")
		}
		return c.JSON(pref)
	})

	me.Get("/limits", func(c *fiber.Ctx) error {
		userID, ok := authenticatedUserID(c)
		if !ok {
			return unauthorized(c, errTokenIdentity)
		}
		limits, err := svc.GetLimits(c.Context(), userID)
		if err != nil {
			return respondUserError(c, err, "get limits")
		}
		return c.JSON(limits)
	})

	me.Put("/limits", func(c *fiber.Ctx) error {
		userID, ok := authenticatedUserID(c)
		if !ok {
			return unauthorized(c, errTokenIdentity)
		}
		var req domain.SetLimitsRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).
				JSON(fiber.Map{"error": "invalid request body"})
		}
		req.UserID = userID

		// Rejects negative/!decimal limits and any attempt to clear
		// self-exclusion. See internal/domain/limits_validation.go.
		if err := req.Validate(); err != nil {
			return c.Status(fiber.StatusBadRequest).
				JSON(fiber.Map{"error": err.Error()})
		}

		limits, err := svc.SetLimits(c.Context(), &req)
		if err != nil {
			return respondUserError(c, err, "set limits")
		}
		return c.JSON(limits)
	})
}

// respondUserError maps domain outcomes onto status codes.
//
// The previous version answered 404 for every error from GetUser and 400 for
// every error from UpdateUser, so a dropped database connection was reported to
// clients as "user not found" — hiding a real incident behind a plausible
// 404. Internal failures are now 500 and logged; only genuine "no such user"
// is 404.
func respondUserError(c *fiber.Ctx, err error, op string) error {
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	case errors.Is(err, domain.ErrInvalidUserID):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user id"})
	default:
		// Log the cause; never forward storage internals to the client.
		slog.Error("user service request failed",
			slog.String("op", op),
			slog.String("error", err.Error()),
			slog.String("trace_hint", c.Get("X-Request-Id")),
		)
		return c.Status(fiber.StatusInternalServerError).
			JSON(fiber.Map{"error": "internal error"})
	}
}
