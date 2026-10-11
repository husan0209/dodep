package main

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWTClaims mirrors the auth service claim structure for shared-secret validation.
type JWTClaims struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	DeviceID  string `json:"device_id,omitempty"`
	jwt.RegisteredClaims
}

// AuthMiddleware validates a Bearer JWT using the shared secret.
// On success stores user_id (string) in c.Locals. user_id NEVER comes from body.
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
		return c.Next()
	}
}

// getUserID extracts user_id set by AuthMiddleware.
func getUserID(c *fiber.Ctx) string {
	v, _ := c.Locals("user_id").(string)
	return v
}

// SecurityHeaders applies the baseline browser/edge hardening for every
// response. The API is JSON-only and never framed, so the strictest sensible
// policy is applied: no framing, no MIME sniffing, no referrer leakage.
func SecurityHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// fasthttp exposes the response header struct directly (no getter).
		h := &c.Response().Header
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		// The service serves API traffic only; camera/mic/geo are unnecessary.
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		// HSTS is enforced by the mesh/edge for browser traffic; repeating it
		// here keeps the header set correct if the port is ever exposed
		// directly through a load balancer.
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
		method := c.Method()
		if method == fiber.MethodGet || method == fiber.MethodHead || method == fiber.MethodOptions {
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
