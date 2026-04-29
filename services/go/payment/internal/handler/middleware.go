package handler

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

var (
	jwtValidator     *JWTValidator
	jwtValidatorOnce sync.Once
	jwtValidatorErr  error
)

// InitJWTValidator initializes the JWT validator singleton
func InitJWTValidator(publicKeyBase64 string) error {
	jwtValidatorOnce.Do(func() {
		jwtValidator, jwtValidatorErr = NewJWTValidator(publicKeyBase64)
	})
	return jwtValidatorErr
}

// AuthMiddleware returns an authentication middleware
func AuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Skip auth for webhooks and health
		if strings.HasPrefix(c.Path(), "/api/v1/payments/webhooks") ||
			c.Path() == "/healthz" ||
			c.Path() == "/readyz" {
			return c.Next()
		}

		// Check if validator is initialized
		if jwtValidator == nil {
			log.Error().Msg("JWT validator not initialized")
			return c.Status(500).JSON(ErrorResponse{
				Error: ErrorDetail{
					Code:    5001,
					Message: "authentication not configured",
				},
			})
		}

		// Get authorization header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(401).JSON(ErrorResponse{
				Error: ErrorDetail{
					Code:    4010,
					Message: "authorization required",
				},
			})
		}

		// Extract token
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return c.Status(401).JSON(ErrorResponse{
				Error: ErrorDetail{
					Code:    4011,
					Message: "invalid authorization format",
				},
			})
		}

		token := parts[1]

		// Validate JWT token
		claims, err := jwtValidator.ValidateToken(token)
		if err != nil {
			log.Debug().Err(err).Msg("JWT validation failed")
			return c.Status(401).JSON(ErrorResponse{
				Error: ErrorDetail{
					Code:    4012,
					Message: "invalid token",
				},
			})
		}

		// Set user info in context
		userID, err := strconv.ParseInt(claims.UserID, 10, 64)
		if err != nil {
			return c.Status(401).JSON(ErrorResponse{
				Error: ErrorDetail{
					Code:    4012,
					Message: "invalid token",
				},
			})
		}
		c.Locals("user_id", userID)
		if claims.Email != "" {
			c.Locals("user_email", claims.Email)
		}

		return c.Next()
	}
}

// LoggingMiddleware returns a logging middleware
func LoggingMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		err := c.Next()

		duration := time.Since(start)
		status := c.Response().StatusCode()

		// Get request ID from context if available
		requestID, _ := c.Locals("requestid").(string)

		logEvent := log.Info()
		if requestID != "" {
			logEvent = logEvent.Str("request_id", requestID)
		}

		logEvent.
			Str("method", c.Method()).
			Str("path", c.Path()).
			Int("status", status).
			Dur("duration", duration).
			Str("ip", c.IP()).
			Msg("HTTP request")

		return err
	}
}

// RateLimitMiddleware returns a rate limiting middleware placeholder
func RateLimitMiddleware() fiber.Handler {
	// TODO: Implement proper rate limiting with Redis
	return func(c *fiber.Ctx) error {
		return c.Next()
	}
}
