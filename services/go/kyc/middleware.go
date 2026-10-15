package main

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWTClaims mirrors the auth-service claim shape for shared-secret validation.
// Admin rights arrive as role=admin or is_admin=true.
type JWTClaims struct {
	UserID  string `json:"user_id"`
	Role    string `json:"role,omitempty"`
	IsAdmin bool   `json:"is_admin,omitempty"`
	jwt.RegisteredClaims
}

// AuthMiddleware validates a Bearer JWT using the shared secret.
// On success stores user_id (string) and is_admin (bool) in c.Locals.
// user_id NEVER comes from the request body (CONVENTIONS NEVER-7).
func AuthMiddleware(jwtSecretKey string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(401).JSON(fiber.Map{"error": "missing Authorization header"})
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			return c.Status(401).JSON(fiber.Map{"error": "invalid Authorization header format, expected: Bearer <token>"})
		}
		token, err := jwt.ParseWithClaims(parts[1], &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(jwtSecretKey), nil
		})
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "invalid or expired token"})
		}
		claims, ok := token.Claims.(*JWTClaims)
		if !ok || !token.Valid || claims.UserID == "" {
			return c.Status(401).JSON(fiber.Map{"error": "invalid token claims"})
		}
		c.Locals("user_id", claims.UserID)
		c.Locals("is_admin", claims.IsAdmin || strings.EqualFold(claims.Role, "admin"))
		return c.Next()
	}
}

// getUserID extracts user_id set by AuthMiddleware.
func getUserID(c *fiber.Ctx) string {
	v, _ := c.Locals("user_id").(string)
	return v
}

// SecurityHeaders applies baseline hardening to every response.
func SecurityHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		h := &c.Response().Header
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		h.Set("X-DNS-Prefetch-Control", "off")
		return c.Next()
	}
}

// IdempotencyKeyHeader carries the idempotency key for mutating requests.
const IdempotencyKeyHeader = "X-Idempotency-Key"

// IdempotencyMiddleware enforces UUID idempotency keys on mutating requests.
func IdempotencyMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		switch c.Method() {
		case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
			return c.Next()
		}
		key := c.Get(IdempotencyKeyHeader)
		if key == "" {
			return c.Status(400).JSON(fiber.Map{"error": "missing X-Idempotency-Key header for mutating request"})
		}
		if _, err := uuid.Parse(key); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid X-Idempotency-Key format, expected UUID"})
		}
		c.Locals("idempotency_key", key)
		return c.Next()
	}
}
