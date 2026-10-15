package main

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWTClaims mirrors the auth service claim structure for shared-secret validation.
// Admin surfaces additionally read role/permission claims; tokens issued
// without them simply carry none and fail admin checks (fail-closed).
type JWTClaims struct {
	UserID      string   `json:"user_id"`
	SessionID   string   `json:"session_id"`
	DeviceID    string   `json:"device_id,omitempty"`
	Role        string   `json:"role,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	IsAdmin     bool     `json:"is_admin,omitempty"`
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
		if !ok || !token.Valid || claims.UserID == "" {
			return c.Status(401).JSON(fiber.Map{
				"error": "invalid token claims",
			})
		}

		// Store user identity — NOT from body (CONVENTIONS NEVER-7)
		c.Locals("user_id", claims.UserID)
		c.Locals("session_id", claims.SessionID)
		c.Locals("roles", claims.Roles)
		c.Locals("permissions", claims.Permissions)

		return c.Next()
	}
}

// affiliateAdminPermissions are the permission claims accepted on admin routes.
var affiliateAdminPermissions = map[string]bool{
	"affiliate.approve":        true,
	"affiliate.payout.approve": true,
	"affiliate.adjust":         true,
}

// hasAffiliateAdminPrivilege reports whether token claims grant affiliate admin
// rights: a superuser flag, an admin role, or any affiliate admin permission.
func hasAffiliateAdminPrivilege(claims *JWTClaims) bool {
	if claims == nil {
		return false
	}
	if claims.IsAdmin {
		return true
	}
	roles := append([]string{claims.Role}, claims.Roles...)
	for _, r := range roles {
		switch strings.ToLower(strings.TrimSpace(r)) {
		case "admin", "superadmin":
			return true
		}
	}
	for _, p := range claims.Permissions {
		np := strings.ToLower(strings.TrimSpace(p))
		if np == "*" || np == "all.*" || np == "affiliate.*" {
			return true
		}
		if affiliateAdminPermissions[np] {
			return true
		}
	}
	return false
}

// AdminMiddleware checks that the requesting user has admin privileges.
// It validates the JWT and then requires an admin role claim: is_admin, an
// admin/superadmin role, or one of the affiliate permissions
// (affiliate.approve, affiliate.payout.approve, affiliate.adjust).
// Any other authenticated user gets 403 — fail-closed, never fail-open.
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
		if !ok || !token.Valid || claims.UserID == "" {
			return c.Status(401).JSON(fiber.Map{"error": "invalid admin token claims"})
		}

		if !hasAffiliateAdminPrivilege(claims) {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden: affiliate admin privileges required"})
		}

		c.Locals("admin_user_id", claims.UserID)
		c.Locals("session_id", claims.SessionID)
		c.Locals("admin_roles", claims.Roles)
		c.Locals("admin_permissions", claims.Permissions)

		return c.Next()
	}
}

// getUserID extracts user_id set by AuthMiddleware ("" when absent).
func getUserID(c *fiber.Ctx) string {
	v, _ := c.Locals("user_id").(string)
	return v
}

// getAdminUserID extracts admin_user_id set by AdminMiddleware ("" when absent).
func getAdminUserID(c *fiber.Ctx) string {
	v, _ := c.Locals("admin_user_id").(string)
	return v
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
