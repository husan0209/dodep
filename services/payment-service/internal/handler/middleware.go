package handler

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SetupMiddleware sets up all middleware
func SetupMiddleware(app *fiber.App, logger *zap.Logger) {
	// Request ID
	app.Use(requestid.New(requestid.Config{
		Generator: func() string {
			return uuid.New().String()
		},
	}))

	// CORS
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization,X-Request-ID",
	}))

	// Rate limiting
	app.Use(limiter.New(limiter.Config{
		Max:        100,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(429).JSON(ErrorResponse{
				Code:    4290,
				Message: "rate limit exceeded",
			})
		},
	}))

	// Logging
	app.Use(loggingMiddleware(logger))

	// Authentication
	app.Use(authMiddleware(logger))
}

// loggingMiddleware logs all requests
func loggingMiddleware(logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		err := c.Next()

		duration := time.Since(start)
		status := c.Response().StatusCode()

		logger.Info("HTTP request",
			zap.String("method", c.Method()),
			zap.String("path", c.Path()),
			zap.Int("status", status),
			zap.Duration("duration", duration),
			zap.String("request_id", c.Locals("requestid").(string)),
			zap.String("ip", c.IP()),
		)

		return err
	}
}

// authMiddleware validates authentication
func authMiddleware(logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Skip auth for webhooks and health
		if strings.HasPrefix(c.Path(), "/api/v1/payments/webhooks") ||
			c.Path() == "/health" ||
			c.Path() == "/ready" {
			return c.Next()
		}

		// Get authorization header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(401).JSON(ErrorResponse{
				Code:    4010,
				Message: "authorization required",
			})
		}

		// Extract token
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return c.Status(401).JSON(ErrorResponse{
				Code:    4011,
				Message: "invalid authorization format",
			})
		}

		token := parts[1]

		// TODO: Validate JWT token and extract user ID
		// For now, use a placeholder
		userID, err := validateToken(token)
		if err != nil {
			return c.Status(401).JSON(ErrorResponse{
				Code:    4012,
				Message: "invalid token",
			})
		}

		// Set user ID in context
		c.Locals("user_id", userID)

		return c.Next()
	}
}

// validateToken validates a JWT token and returns user ID
// TODO: Implement actual JWT validation
func validateToken(token string) (int64, error) {
	// Placeholder - in production, validate JWT and extract claims
	return 1, nil
}
