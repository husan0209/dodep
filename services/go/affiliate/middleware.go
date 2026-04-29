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

// AuthMiddleware validates a Bearer JWT token using the shared secret.
// On success it stores user_id (string) and session_id (string) in c.Locals.
func AuthMiddleware(jwtSecretKey string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(401).JSON(fiber.Map{
				"error": "missing Authorization header",
			})
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			return c.Status(401).JSON(fiber.Map{
				"error": "invalid Authorization header format, expected: Bearer <token>",
			})
		}

		tokenString := parts[1]

		token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(jwtSecretKey), nil
		})
		if err != nil {
			return c.Status(401).JSON(fiber.Map{
				"error": "invalid or expired token",
			})
		}

		claims, ok := token.Claims.(*JWTClaims)
		if !ok || !token.Valid {
			return c.Status(401).JSON(fiber.Map{
				"error": "invalid token claims",
			})
		}

		// Store user identity — NOT from body (CONVENTIONS NEVER-7)
		c.Locals("user_id", claims.UserID)
		c.Locals("session_id", claims.SessionID)

		return c.Next()
	}
}

// AdminMiddleware checks that the requesting user has admin privileges.
// For MVP, it validates the JWT and allows any authenticated user on admin routes.
// In production, this should check a role claim in the token.
func AdminMiddleware(jwtSecretKey string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(401).JSON(fiber.Map{
				"error": "missing Authorization header",
			})
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			return c.Status(401).JSON(fiber.Map{
				"error": "invalid Authorization header format",
			})
		}

		token, err := jwt.ParseWithClaims(parts[1], &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(jwtSecretKey), nil
		})
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "invalid or expired admin token"})
		}

		claims, ok := token.Claims.(*JWTClaims)
		if !ok || !token.Valid {
			return c.Status(401).JSON(fiber.Map{"error": "invalid admin token claims"})
		}

		c.Locals("admin_user_id", claims.UserID)
		c.Locals("session_id", claims.SessionID)

		return c.Next()
	}
}

// getUserID extracts user_id set by AuthMiddleware.
func getUserID(c *fiber.Ctx) string {
	return c.Locals("user_id").(string)
}

// getAdminUserID extracts admin_user_id set by AdminMiddleware.
func getAdminUserID(c *fiber.Ctx) string {
	return c.Locals("admin_user_id").(string)
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
			return c.Status(400).JSON(fiber.Map{
				"error": "missing X-Idempotency-Key header for mutating request",
			})
		}

		if _, err := uuid.Parse(key); err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error": "invalid X-Idempotency-Key format, expected UUID",
			})
		}

		c.Locals("idempotency_key", key)
		return c.Next()
	}
}
