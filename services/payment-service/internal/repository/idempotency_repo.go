package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// idempotencyRepo implements IdempotencyRepository using Redis
type idempotencyRepo struct {
	client *redis.Client
}

// NewIdempotencyRepository creates a new idempotency repository
func NewIdempotencyRepository(client *redis.Client) IdempotencyRepository {
	return &idempotencyRepo{client: client}
}

// Get retrieves a cached response by key
func (r *idempotencyRepo) Get(ctx context.Context, key string) ([]byte, bool, error) {
	fullKey := r.keyPrefix() + key

	result, err := r.client.Get(ctx, fullKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("get idempotency key: %w", err)
	}

	return result, true, nil
}

// Set stores a response with TTL
func (r *idempotencyRepo) Set(ctx context.Context, key string, value []byte, ttlSeconds int) error {
	fullKey := r.keyPrefix() + key

	err := r.client.Set(ctx, fullKey, value, time.Duration(ttlSeconds)*time.Second).Err()
	if err != nil {
		return fmt.Errorf("set idempotency key: %w", err)
	}

	return nil
}

// SetNX stores only if key doesn't exist (returns true if set)
func (r *idempotencyRepo) SetNX(ctx context.Context, key string, value []byte, ttlSeconds int) (bool, error) {
	fullKey := r.keyPrefix() + key

	result, err := r.client.SetNX(ctx, fullKey, value, time.Duration(ttlSeconds)*time.Second).Result()
	if err != nil {
		return false, fmt.Errorf("setnx idempotency key: %w", err)
	}

	return result, nil
}

// Delete removes a key
func (r *idempotencyRepo) Delete(ctx context.Context, key string) error {
	fullKey := r.keyPrefix() + key

	err := r.client.Del(ctx, fullKey).Err()
	if err != nil {
		return fmt.Errorf("delete idempotency key: %w", err)
	}

	return nil
}

func (r *idempotencyRepo) keyPrefix() string {
	return "idempotency:"
}
