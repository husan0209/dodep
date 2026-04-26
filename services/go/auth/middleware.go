package main

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/opus-casino/auth/internal/service"
)

// AuthMiddleware extracts and validates JWT from Authorization header.
// On success, sets "user_id" (int64) and "session_id" (string) in Locals.
func AuthMiddleware(authService *service.AuthService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(401).JSON(fiber.Map{
				"error": "missing Authorization header",
			})
		}

		// Extract Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			return c.Status(401).JSON(fiber.Map{
				"error": "invalid Authorization header format, expected: Bearer <token>",
			})
		}

		token := parts[1]

		// Validate token via AuthService (checks JWT signature + session exists)
		claims, err := authService.ValidateToken(c.Context(), token)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{
				"error": "invalid or expired token",
			})
		}

		userID := claims.UserID
		if userID == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid token claims",
			})
		}

		// Store user identity in request context — NOT from body
		c.Locals("user_id", userID)
		c.Locals("session_id", claims.SessionID)

		return c.Next()
	}
}

// getUserID extracts user_id set by AuthMiddleware. Panics if middleware not applied.
func getUserID(c *fiber.Ctx) string {
	return c.Locals("user_id").(string)
}

// getSessionID extracts session_id set by AuthMiddleware.
func getSessionID(c *fiber.Ctx) string {
	return c.Locals("session_id").(string)
}
