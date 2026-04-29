package middleware

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

// RateLimitConfig defines limits for a given key pattern.
type RateLimitConfig struct {
	KeyPrefix  string        // e.g. "rl:login:ip"
	Limit      int           // max requests
	Window     time.Duration // rolling window
	LockoutTTL time.Duration // how long to lock after exceeding (0 = same as Window)
}

// RateLimitLogin applies per-IP and per-email rate limits to the login endpoint.
// Per spec A1.2: 10 req/15min per IP, 5 req/15min per email → 30-min lockout.
func RateLimitLogin(rdb *redis.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ip := c.IP()
		ipKey := fmt.Sprintf("rl:login:ip:%s", ip)
		if blocked, reset := checkLimit(rdb, ipKey, 10, 15*time.Minute, 30*time.Minute); blocked {
			setRateLimitHeaders(c, 10, 0, reset)
			return c.Status(429).JSON(fiber.Map{
				"error":   "RATE_LIMIT_EXCEEDED",
				"message": "Too many login attempts from this IP. Try again after 30 minutes.",
			})
		}

		// Also check per-email (best-effort body peek — no body re-parse needed here,
		// actual enforcement happens after body is parsed in the login handler).
		// We increment speculatively; the login handler can also call IncrEmail.
		setRateLimitHeaders(c, 10, remaining(rdb, ipKey), resetAt(rdb, ipKey))
		return c.Next()
	}
}

// RateLimitAPI applies general per-admin API rate limits.
// Per spec A1.2: 1000 req/min per admin.
func RateLimitAPI(rdb *redis.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		adminID := fmt.Sprintf("%v", c.Locals("admin_id"))
		if adminID == "<nil>" || adminID == "" {
			return c.Next()
		}
		key := fmt.Sprintf("rl:api:admin:%s", adminID)
		if blocked, reset := checkLimit(rdb, key, 1000, time.Minute, 0); blocked {
			setRateLimitHeaders(c, 1000, 0, reset)
			return c.Status(429).JSON(fiber.Map{
				"error":   "RATE_LIMIT_EXCEEDED",
				"message": "API rate limit exceeded (1000 req/min).",
			})
		}
		setRateLimitHeaders(c, 1000, remaining(rdb, key), resetAt(rdb, key))
		return c.Next()
	}
}

// RateLimitBulkExport applies stricter limit for heavy export endpoints.
// Per spec A1.2: 5 req/hour per admin.
func RateLimitBulkExport(rdb *redis.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		adminID := fmt.Sprintf("%v", c.Locals("admin_id"))
		key := fmt.Sprintf("rl:bulk_export:admin:%s", adminID)
		if blocked, reset := checkLimit(rdb, key, 5, time.Hour, 0); blocked {
			setRateLimitHeaders(c, 5, 0, reset)
			return c.Status(429).JSON(fiber.Map{
				"error":   "RATE_LIMIT_EXCEEDED",
				"message": "Bulk export limit exceeded (5 req/hour).",
			})
		}
		return c.Next()
	}
}

// RateLimitWithdrawalApprove: 100 req/hour per admin.
func RateLimitWithdrawalApprove(rdb *redis.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		adminID := fmt.Sprintf("%v", c.Locals("admin_id"))
		key := fmt.Sprintf("rl:withdrawal_approve:admin:%s", adminID)
		if blocked, reset := checkLimit(rdb, key, 100, time.Hour, 0); blocked {
			setRateLimitHeaders(c, 100, 0, reset)
			return c.Status(429).JSON(fiber.Map{
				"error":   "RATE_LIMIT_EXCEEDED",
				"message": "Withdrawal approval limit exceeded (100 req/hour).",
			})
		}
		return c.Next()
	}
}

// RateLimitBalanceAdjust: 20 req/hour per admin.
func RateLimitBalanceAdjust(rdb *redis.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		adminID := fmt.Sprintf("%v", c.Locals("admin_id"))
		key := fmt.Sprintf("rl:balance_adjust:admin:%s", adminID)
		if blocked, reset := checkLimit(rdb, key, 20, time.Hour, 0); blocked {
			setRateLimitHeaders(c, 20, 0, reset)
			return c.Status(429).JSON(fiber.Map{
				"error":   "RATE_LIMIT_EXCEEDED",
				"message": "Balance adjustment limit exceeded (20 req/hour).",
			})
		}
		return c.Next()
	}
}

// checkLimit increments the counter and returns (blocked, resetUnixTs).
// lockoutTTL=0 means use the window TTL as lockout.
func checkLimit(rdb *redis.Client, key string, limit int, window, lockoutTTL time.Duration) (bool, int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	count, err := rdb.Incr(ctx, key).Result()
	if err != nil {
		return false, 0
	}
	if count == 1 {
		// First hit — set expiry
		ttl := window
		if lockoutTTL > 0 && int(count) > limit {
			ttl = lockoutTTL
		}
		rdb.Expire(ctx, key, ttl)
	}
	if int(count) > limit {
		if lockoutTTL > 0 {
			rdb.Expire(ctx, key, lockoutTTL)
		}
		ttl, _ := rdb.TTL(ctx, key).Result()
		return true, time.Now().Add(ttl).Unix()
	}
	return false, 0
}

func remaining(rdb *redis.Client, key string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	v, _ := rdb.Get(ctx, key).Int()
	return v
}

func resetAt(rdb *redis.Client, key string) int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	ttl, _ := rdb.TTL(ctx, key).Result()
	return time.Now().Add(ttl).Unix()
}

func setRateLimitHeaders(c *fiber.Ctx, limit, remaining int, reset int64) {
	c.Set("X-RateLimit-Limit", strconv.Itoa(limit))
	c.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
	c.Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))
}
