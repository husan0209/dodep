package middleware

import (
	"crypto/ed25519"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// loadEd25519PublicKey loads an Ed25519 public key from a PEM file path.
// Returns nil if path is empty or file unreadable (falls back to HMAC).
func loadEd25519PublicKey(path string) ed25519.PublicKey {
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil
	}
	return ed25519.PublicKey(block.Bytes)
}

func AdminAuth(jwtSecret string, db *pgxpool.Pool) fiber.Handler {
	// Load Ed25519 public key once at startup (optional).
	ed25519Pub := loadEd25519PublicKey(os.Getenv("JWT_ED25519_PUBLIC_KEY_FILE"))

	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		tokenStr := ""
		if authHeader != "" {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
			if tokenStr == authHeader {
				return c.Status(401).JSON(fiber.Map{"error": "invalid authorization format"})
			}
		} else {
			// Fallback for WebSocket / SSE clients that cannot set custom headers.
			tokenStr = c.Query("token", "")
			if tokenStr == "" {
				return c.Status(401).JSON(fiber.Map{"error": "missing authorization header"})
			}
		}

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			switch t.Method.(type) {
			case *jwt.SigningMethodEd25519:
				if ed25519Pub == nil {
					return nil, errors.New("ed25519 public key not configured")
				}
				return ed25519Pub, nil
			case *jwt.SigningMethodHMAC:
				return []byte(jwtSecret), nil
			default:
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
		})
		if err != nil || !token.Valid {
			return c.Status(401).JSON(fiber.Map{"error": "invalid token"})
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return c.Status(401).JSON(fiber.Map{"error": "invalid claims"})
		}

		c.Locals("admin_id", claims["admin_id"])
		c.Locals("admin_role", claims["role"])
		c.Locals("permissions", claims["permissions"])

		return c.Next()
	}
}

func RequirePermission(perm string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		perms, ok := c.Locals("permissions").([]interface{})
		if !ok {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		for _, p := range perms {
			if p == perm {
				return c.Next()
			}
		}
		return c.Status(403).JSON(fiber.Map{"error": "permission denied"})
	}
}
