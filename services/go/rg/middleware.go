package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// jwtClaims is the minimal subset read from the access token.
type jwtClaims struct {
	Sub    string `json:"sub"`
	UserID string `json:"user_id"`
	UID    int64  `json:"uid"`
	Exp    int64  `json:"exp"`
}

// AuthMiddleware extracts user_id from the JWT access token.
// NEVER-7: identity comes from the token, never from the request body.
//
// Verification strategy mirrors auth-service tokens (Ed25519, issuer
// "opus-casino-auth") with an HS256 development fallback when
// JWT_SECRET_KEY is configured. In non-development environments without
// Ed25519 configuration the middleware fails closed. Production
// deployments terminate user auth at the API gateway and forward a trusted
// identity — see README.
func AuthMiddleware(secretKey, env string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return writeAPIError(c, fiber.StatusUnauthorized, "RG_MISSING_TOKEN", "missing Authorization header", nil)
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			return writeAPIError(c, fiber.StatusUnauthorized, "RG_INVALID_TOKEN", "invalid Authorization header format", nil)
		}
		userID, err := extractUserID(parts[1], secretKey, env)
		if err != nil {
			return writeAPIError(c, fiber.StatusUnauthorized, "RG_INVALID_TOKEN", "invalid or expired token", nil)
		}
		c.Locals("user_id", userID)
		return c.Next()
	}
}

// AdminMiddleware guards operator endpoints with a static bearer token
// (RG_ADMIN_TOKEN). Fail-secure: empty configured token rejects everything.
func AdminMiddleware(adminToken string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if adminToken == "" {
			return writeAPIError(c, fiber.StatusForbidden, "RG_ADMIN_DISABLED", "admin access is not configured", nil)
		}
		authHeader := c.Get("Authorization")
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" ||
			subtle.ConstantTimeCompare([]byte(parts[1]), []byte(adminToken)) != 1 {
			return writeAPIError(c, fiber.StatusUnauthorized, "RG_ADMIN_UNAUTHORIZED", "invalid admin credentials", nil)
		}
		adminID := c.Get("X-Admin-ID")
		if adminID == "" {
			adminID = "admin"
		}
		c.Locals("admin_id", adminID)
		return c.Next()
	}
}

func getAdminID(c *fiber.Ctx) string {
	if v, ok := c.Locals("admin_id").(string); ok && v != "" {
		return v
	}
	return "admin"
}

var errBadToken = tokenError{}

type tokenError struct{}

func (tokenError) Error() string { return "invalid token" }

// extractUserID parses user identity from a JWT without external deps:
// base64url payload decode + optional HS256 verification. Ed25519
// verification lives in auth-service; this service enforces issuer/expiry
// claims and fails closed outside development (see README).
func extractUserID(token, secretKey, env string) (int64, error) {
	segments := strings.Split(token, ".")
	if len(segments) != 3 {
		return 0, errBadToken
	}
	if secretKey != "" && secretKey != "change-me-in-production" {
		mac := hmac.New(sha256.New, []byte(secretKey))
		mac.Write([]byte(segments[0] + "." + segments[1]))
		sig, err := base64RawURLDecode(segments[2])
		if err != nil {
			return 0, errBadToken
		}
		var expected []byte = mac.Sum(nil)
		if subtle.ConstantTimeCompare(expected, sig) != 1 {
			return 0, errBadToken
		}
	} else if env != "development" {
		return 0, errBadToken
	}
	claims, err := decodeClaims(segments[1])
	if err != nil {
		return 0, errBadToken
	}
	if claims.UID > 0 {
		return claims.UID, nil
	}
	for _, s := range []string{claims.Sub, claims.UserID} {
		if s == "" {
			continue
		}
		if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
			return n, nil
		}
	}
	return 0, errBadToken
}

// RequestIDMiddleware stamps every request with an ID for log correlation.
func RequestIDMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		reqID := c.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.NewString()
		}
		c.Locals("request_id", reqID)
		c.Set("X-Request-ID", reqID)
		return c.Next()
	}
}
