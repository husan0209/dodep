// Package ratelimit implements a bounded, dependency-free HTTP rate limiter
// for Bonus Service.
//
// Scope and trade-offs (deliberate, documented):
//   - State is per pod, in memory. It is a first line of defence against
//     abuse and accidental client loops, not a global quota: the platform-wide
//     limits are enforced at the edge (CloudFlare) and in the mesh (Istio).
//     Swapping in DragonflyDB later only requires replacing Store.
//   - Fixed window per key. Cheap, no allocations beyond the map entry, and
//     precise enough for abuse control; the window boundary can allow up to
//     2x the nominal rate in the worst case, which is acceptable here.
//   - The limiter fails OPEN: if the budget map is full, requests are served.
//     A limiter must never become an outage.
//
// The response contract matches the rest of the platform: HTTP 429 with
// Retry-After and X-RateLimit-Limit / -Remaining / -Reset headers.
package ratelimit

import (
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Config describes one limit bucket.
type Config struct {
	// Name identifies the bucket in logs and metrics (e.g. "read", "mutate").
	Name string
	// Limit is the maximum number of requests per Window for one key.
	Limit int
	// Window is the fixed window length. Must be > 0.
	Window time.Duration
	// maxKeys bounds the in-memory map. 0 means defaultMaxKeys.
	// When the bound is reached the oldest quarter of entries is evicted.
	maxKeys int
}

const (
	defaultMaxKeys = 50_000
	// evictFraction is the share of entries dropped when the map is full.
	evictFraction = 0.25
)

// Limiter is a fixed-window rate limiter safe for concurrent use.
type Limiter struct {
	cfg      Config
	maxKeys  int
	mu       sync.Mutex
	buckets  map[string]*bucket
	stopOnce sync.Once
	stopCh   chan struct{}
}

type bucket struct {
	count     int
	windowEnd time.Time
	// seq is only used to pick eviction victims deterministically.
	seq uint64
}

// New builds a Limiter. Invalid configuration is normalised rather than
// rejected, because a limiter that panics on boot is worse than a permissive
// one.
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
	max := cfg.maxKeys
	if max <= 0 {
		max = defaultMaxKeys
	}
	return &Limiter{
		cfg:     cfg,
		maxKeys: max,
		buckets: make(map[string]*bucket),
		stopCh:  make(chan struct{}),
	}
}

// Name returns the bucket name.
func (l *Limiter) Name() string { return l.cfg.Name }

// Result describes the decision for one request.
type Result struct {
	// Allowed is false when the caller must be rejected with 429.
	Allowed bool
	// Limit is the configured budget.
	Limit int
	// Remaining is the budget left in the current window.
	Remaining int
	// Reset is the Unix timestamp when the current window ends.
	Reset int64
	// RetryAfter is a human-readable hint for the 429 response.
	RetryAfter time.Duration
}

// Allow records a hit for key and reports whether it fits in the budget.
// now is injected so tests are deterministic.
func (l *Limiter) Allow(key string, now time.Time) Result {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.buckets) >= l.maxKeys {
		l.evictLocked()
	}

	b, ok := l.buckets[key]
	if !ok || !now.Before(b.windowEnd) {
		b = &bucket{count: 0, windowEnd: now.Add(l.cfg.Window), seq: l.nextSeq()}
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

// evictLocked drops the oldest quarter of the map. Called with the lock held.
func (l *Limiter) evictLocked() {
	target := len(l.buckets) - int(float64(len(l.buckets))*evictFraction)
	if target < 1 {
		target = 1
	}
	// Simple selection of the oldest entries by seq: the map is only walked
	// when the bound is hit, which is rare and bounded by maxKeys.
	type victim struct {
		key string
		seq uint64
	}
	victims := make([]victim, 0, target)
	for k, b := range l.buckets {
		victims = append(victims, victim{k, b.seq})
	}
	// Insertion-free selection: repeatedly pick the smallest seq.
	for i := 0; i < target && len(victims) > 0; i++ {
		minIdx := 0
		for j := 1; j < len(victims); j++ {
			if victims[j].seq < victims[minIdx].seq {
				minIdx = j
			}
		}
		delete(l.buckets, victims[minIdx].key)
		victims[minIdx] = victims[len(victims)-1]
		victims = victims[:len(victims)-1]
	}
}

func (l *Limiter) nextSeq() uint64 {
	return uint64(time.Now().UnixNano())
}

// Close stops any future background work. Kept for symmetry with services
// that need cleanup; the limiter itself holds no goroutines.
func (l *Limiter) Close() {
	l.stopOnce.Do(func() { close(l.stopCh) })
}

// KeyFunc derives the bucket key for a request. Returning an empty string
// disables limiting for that request (fail open for unidentifiable traffic).
type KeyFunc func(c *fiber.Ctx) string

// ByUserOrIP keys on the authenticated user when present, otherwise on the
// client IP. User-level keys are what actually bound abuse: many users can
// share one corporate NAT IP.
func ByUserOrIP(c *fiber.Ctx) string {
	if uid, ok := c.Locals("user_id").(string); ok && uid != "" {
		return "user:" + uid
	}
	return "ip:" + c.IP()
}

// Middleware enforces l and returns a Fiber handler.
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
				"limit":               res.Limit,
				"retry_after_seconds": retry,
			})
		}
		return c.Next()
	}
}
