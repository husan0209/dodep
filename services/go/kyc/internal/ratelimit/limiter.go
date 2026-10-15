// Package ratelimit implements a bounded, dependency-free HTTP rate limiter.
//
// Scope (deliberate): per-pod in-memory fixed window — first line of defence
// against abuse and client loops, not a global quota (global limits live at
// the edge/mesh). Fails OPEN: a full map serves requests rather than causing
// an outage. 429 responses carry Retry-After + X-RateLimit-* headers.
package ratelimit

import (
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Config describes one limit bucket.
type Config struct {
	Name string
	// Limit is the maximum requests per Window for one key.
	Limit int
	// Window is the fixed window length.
	Window time.Duration
}

const defaultMaxKeys = 50000

// Limiter is a fixed-window rate limiter safe for concurrent use.
type Limiter struct {
	cfg     Config
	maxKeys int
	mu      sync.Mutex
	buckets map[string]*bucket
}

// Result describes the decision for one request.
type Result struct {
	Allowed    bool
	Limit      int
	Remaining  int
	Reset      int64
	RetryAfter time.Duration
}

type bucket struct {
	count     int
	windowEnd time.Time
	seq       uint64
}

// New builds a Limiter. Invalid configuration is normalised, never panics.
func New(cfg Config) *Limiter {
	if cfg.Limit <= 0 {
		cfg.Limit = 60
	}
	if cfg.Window <= 0 {
		cfg.Window = time.Minute
	}
	if cfg.Name == "" {
		cfg.Name = "default"
	}
	return &Limiter{cfg: cfg, maxKeys: defaultMaxKeys, buckets: map[string]*bucket{}}
}

// Allow records a hit for key. now is injected for deterministic tests.
func (l *Limiter) Allow(key string, now time.Time) Result {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.buckets) >= l.maxKeys {
		// Evict an arbitrary quarter; the limiter must never grow unbounded.
		n := 0
		for k := range l.buckets {
			delete(l.buckets, k)
			if n++; n >= l.maxKeys/4 {
				break
			}
		}
	}
	b, ok := l.buckets[key]
	if !ok || !now.Before(b.windowEnd) {
		b = &bucket{windowEnd: now.Add(l.cfg.Window), seq: uint64(now.UnixNano())}
		l.buckets[key] = b
	}
	b.count++

	remaining := l.cfg.Limit - b.count
	if remaining < 0 {
		remaining = 0
	}
	return Result{
		Allowed:    b.count <= l.cfg.Limit,
		Limit:      l.cfg.Limit,
		Remaining:  remaining,
		Reset:      b.windowEnd.Unix(),
		RetryAfter: b.windowEnd.Sub(now),
	}
}

// Close releases limiter resources (no goroutines; kept for symmetry).
func (l *Limiter) Close() {}

// KeyFunc derives the bucket key for a request.
type KeyFunc func(c *fiber.Ctx) string

// ByUserOrIP keys on the authenticated user when present, else client IP.
func ByUserOrIP(c *fiber.Ctx) string {
	if uid, ok := c.Locals("user_id").(string); ok && uid != "" {
		return "user:" + uid
	}
	return "ip:" + c.IP()
}

// Middleware enforces the limit and returns a Fiber handler.
func (l *Limiter) Middleware(keyFn KeyFunc) fiber.Handler {
	if keyFn == nil {
		keyFn = ByUserOrIP
	}
	return func(c *fiber.Ctx) error {
		key := keyFn(c)
		if key == "" {
			return c.Next()
		}
		res := l.Allow(key, time.Now())
		c.Set("X-RateLimit-Limit", strconv.Itoa(res.Limit))
		c.Set("X-RateLimit-Remaining", strconv.Itoa(res.Remaining))
		c.Set("X-RateLimit-Reset", strconv.FormatInt(res.Reset, 10))
		if !res.Allowed {
			retry := int(res.RetryAfter.Seconds())
			if retry < 1 {
				retry = 1
			}
			c.Set("Retry-After", strconv.Itoa(retry))
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":               "RATE_LIMIT_EXCEEDED",
				"message":             "Too many requests, please retry later.",
				"retry_after_seconds": retry,
			})
		}
		return c.Next()
	}
}
