package handler

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
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

// RateLimitMiddleware returns a Redis-backed rate limiting middleware.
//
// Scope: per authenticated user, falling back to the client IP for
// unauthenticated callers (webhooks). Fixed-window counters via INCR+EXPIRE:
// one round trip on the happy path, atomic in Redis, and the window expires
// on its own — no cleanup job.
//
// Limits come from env (PAYMENTS_RATE_LIMIT_MAX / PAYMENTS_RATE_LIMIT_WINDOW_SEC,
// default 300 requests / 60s per identity). A Redis outage fails OPEN with a
// loud error log: taking payment endpoints down because a cache hiccuped is
// worse than briefly exceeding the budget.
func RateLimitMiddleware(rdb *redis.Client) fiber.Handler {
	if rdb == nil {
		log.Warn().Msg("rate limiting disabled: redis client is nil")
		return func(c *fiber.Ctx) error { return c.Next() }
	}

	limit := envInt("PAYMENTS_RATE_LIMIT_MAX", 300)
	window := time.Duration(envInt("PAYMENTS_RATE_LIMIT_WINDOW_SEC", 60)) * time.Second

	return func(c *fiber.Ctx) error {
		identity := c.IP()
		if userID, ok := c.Locals("user_id").(int64); ok && userID > 0 {
			identity = "u:" + strconv.FormatInt(userID, 10)
		} else if userID, ok := c.Locals("user_id").(string); ok && userID != "" {
			identity = "u:" + userID
		}

		key := "ratelimit:payment:" + identity
		ctx := c.Context()

		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			// Fail open: a cache outage must not become a payments outage.
			log.Error().Err(err).Str("identity", identity).Msg("rate limiter redis error — allowing request")
			return c.Next()
		}
		if count == 1 {
			// First hit in the window: start the TTL. If this EXPIRE fails the
			// key lives forever, so treat it as a hard error and keep serving.
			if _, err := rdb.Expire(ctx, key, window).Result(); err != nil {
				log.Error().Err(err).Str("key", key).Msg("rate limiter expire failed")
			}
		}
		if int(count) > limit {
			ttl, err := rdb.TTL(ctx, key).Result()
			if err != nil || ttl < 0 {
				ttl = window
			}
			retryAfter := int(ttl.Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
			c.Set("Retry-After", strconv.Itoa(retryAfter))
			log.Warn().Str("identity", identity).Int("count", int(count)).Msg("rate limit exceeded")
			return c.Status(429).JSON(ErrorResponse{
				Error: ErrorDetail{
					Code:    4290,
					Message: "rate limit exceeded",
				},
			})
		}

		return c.Next()
	}
}

// envInt reads a positive integer from the environment with a fallback.
func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
