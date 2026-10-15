// Package cache provides short-lived KYC status caching on DragonflyDB
// (Redis API) with an in-memory fallback for development and tests.
//
// Contract: KYC level is NEVER cached longer than 60 seconds — a level can
// change at any moment (review decision, expiry sweep) and payment-service
// limit checks must observe it promptly.
package cache

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/opus-casino/kyc/internal/domain"
)

// StatusTTL bounds how long a verification status may be served from cache.
const StatusTTL = 60 * time.Second

// WebhookTTL bounds idempotency markers for provider webhooks (24h,
// matching the platform idempotency convention).
const WebhookTTL = 24 * time.Hour

// CachedStatus is the serialised verification snapshot.
type CachedStatus struct {
	Level           domain.KYCLevel           `json:"level"`
	Status          domain.VerificationStatus `json:"status"`
	ScreeningStatus domain.ScreeningStatus    `json:"screening_status"`
	UpdatedAt       time.Time                 `json:"updated_at"`
}

// StatusCache abstracts verification-status caching.
// Implementations: RedisStatusCache (DragonflyDB) and MemoryStatusCache.
type StatusCache interface {
	GetStatus(ctx context.Context, userID int64) (*CachedStatus, error)
	SetStatus(ctx context.Context, userID int64, s CachedStatus) error
	Invalidate(ctx context.Context, userID int64) error
	HasProcessedWebhook(ctx context.Context, eventID string) (bool, error)
	MarkWebhookProcessed(ctx context.Context, eventID string) error
	Close() error
}

func statusKey(userID int64) string {
	return "kyc:status:" + strconv.FormatInt(userID, 10)
}

func webhookKey(eventID string) string {
	return "kyc:webhook:" + eventID
}

// ─── Redis ───

// RedisStatusCache stores snapshots in DragonflyDB/Redis.
type RedisStatusCache struct {
	rdb *redis.Client
}

// NewRedis creates a cache over an existing client (owned by main).
func NewRedis(rdb *redis.Client) *RedisStatusCache {
	return &RedisStatusCache{rdb: rdb}
}

// Ping verifies connectivity; callers fall back to memory on error.
func (c *RedisStatusCache) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

func (c *RedisStatusCache) GetStatus(ctx context.Context, userID int64) (*CachedStatus, error) {
	raw, err := c.rdb.Get(ctx, statusKey(userID)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	var s CachedStatus
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, nil // corrupt entry → treat as miss
	}
	return &s, nil
}

func (c *RedisStatusCache) SetStatus(ctx context.Context, userID int64, s CachedStatus) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, statusKey(userID), raw, StatusTTL).Err()
}

func (c *RedisStatusCache) Invalidate(ctx context.Context, userID int64) error {
	return c.rdb.Del(ctx, statusKey(userID)).Err()
}

func (c *RedisStatusCache) HasProcessedWebhook(ctx context.Context, eventID string) (bool, error) {
	n, err := c.rdb.Exists(ctx, webhookKey(eventID)).Result()
	return n > 0, err
}

func (c *RedisStatusCache) MarkWebhookProcessed(ctx context.Context, eventID string) error {
	return c.rdb.Set(ctx, webhookKey(eventID), "1", WebhookTTL).Err()
}

// Close releases the underlying client.
func (c *RedisStatusCache) Close() error {
	if c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}

// ─── Memory (fallback / tests) ───

type memEntry struct {
	raw     []byte
	expires time.Time
}

// MemoryStatusCache is a process-local StatusCache.
// Per-pod state by design: global quotas live at the edge/mesh.
type MemoryStatusCache struct {
	mu   sync.Mutex
	data map[string]memEntry
}

// NewMemory creates an empty in-memory cache.
func NewMemory() *MemoryStatusCache {
	return &MemoryStatusCache{data: map[string]memEntry{}}
}

func (c *MemoryStatusCache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.data[key]
	if !ok || time.Now().After(e.expires) {
		delete(c.data, key)
		return nil, false
	}
	return e.raw, true
}

func (c *MemoryStatusCache) set(key string, raw []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Bound the map: evict expired entries when it grows too large.
	if len(c.data) > 50000 {
		now := time.Now()
		for k, e := range c.data {
			if now.After(e.expires) {
				delete(c.data, k)
			}
		}
	}
	c.data[key] = memEntry{raw: raw, expires: time.Now().Add(ttl)}
}

func (c *MemoryStatusCache) GetStatus(_ context.Context, userID int64) (*CachedStatus, error) {
	raw, ok := c.get(statusKey(userID))
	if !ok {
		return nil, nil
	}
	var s CachedStatus
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, nil
	}
	return &s, nil
}

func (c *MemoryStatusCache) SetStatus(_ context.Context, userID int64, s CachedStatus) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	c.set(statusKey(userID), raw, StatusTTL)
	return nil
}

func (c *MemoryStatusCache) Invalidate(_ context.Context, userID int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, statusKey(userID))
	return nil
}

func (c *MemoryStatusCache) HasProcessedWebhook(_ context.Context, eventID string) (bool, error) {
	_, ok := c.get(webhookKey(eventID))
	return ok, nil
}

func (c *MemoryStatusCache) MarkWebhookProcessed(_ context.Context, eventID string) error {
	c.set(webhookKey(eventID), []byte("1"), WebhookTTL)
	return nil
}

// Close is a no-op for the memory implementation.
func (c *MemoryStatusCache) Close() error { return nil }
