package main

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opus-casino/auth/internal/service"
)

// AuthMiddleware extracts and validates JWT from Authorization header.
// On success, sets "user_id" (int64) and "session_id" (string) in Locals.
func AuthMiddleware(authService *service.AuthService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return writeAPIError(c, fiber.StatusUnauthorized, "AUTH_MISSING_TOKEN", "missing Authorization header", nil)
		}

		// Extract Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			return writeAPIError(c, fiber.StatusUnauthorized, "AUTH_INVALID_TOKEN", "invalid Authorization header format", nil)
		}

		token := parts[1]

		// Validate token via AuthService (checks JWT signature + session exists)
		claims, err := authService.ValidateToken(c.Context(), token)
		if err != nil {
			return writeAPIError(c, fiber.StatusUnauthorized, "AUTH_INVALID_TOKEN", "invalid or expired token", nil)
		}

		userID := claims.UserID
		if userID == "" {
			return writeAPIError(c, fiber.StatusUnauthorized, "AUTH_INVALID_TOKEN", "invalid token claims", nil)
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

// IdempotencyKeyHeader is the HTTP header carrying the idempotency key.
const IdempotencyKeyHeader = "X-Idempotency-Key"

// IdempotencyMiddleware validates the presence and format of X-Idempotency-Key
// for mutating requests and stores it in Locals.
func IdempotencyMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		method := c.Method()
		if method == fiber.MethodGet || method == fiber.MethodHead || method == fiber.MethodOptions {
			return c.Next()
		}

		key := c.Get(IdempotencyKeyHeader)
		if key == "" {
			return writeAPIError(c, fiber.StatusBadRequest, "IDEMPOTENCY_KEY_MISSING",
				"missing X-Idempotency-Key header for mutating request", nil)
		}

		if _, err := uuid.Parse(key); err != nil {
			return writeAPIError(c, fiber.StatusBadRequest, "IDEMPOTENCY_KEY_INVALID",
				"invalid X-Idempotency-Key format, expected UUID", nil)
		}

		c.Locals("idempotency_key", key)
		return c.Next()
	}
}
