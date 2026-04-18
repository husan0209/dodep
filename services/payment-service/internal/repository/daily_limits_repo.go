package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
)

// dailyLimitsRepo implements DailyLimitsRepository using Redis
type dailyLimitsRepo struct {
	client *redis.Client
}

// NewDailyLimitsRepository creates a new daily limits repository
func NewDailyLimitsRepository(client *redis.Client) DailyLimitsRepository {
	return &dailyLimitsRepo{client: client}
}

// Increment adds amount to daily total and returns new total
func (r *dailyLimitsRepo) Increment(ctx context.Context, userID int64, operationType string, amount decimal.Decimal) (decimal.Decimal, error) {
	key := r.buildKey(userID, operationType)

	// Use INCRBYFLOAT for atomic increment
	result, err := r.client.IncrByFloat(ctx, key, amount.InexactFloat64()).Result()
	if err != nil {
		return decimal.Zero, fmt.Errorf("increment daily limit: %w", err)
	}

	// Set expiry to midnight UTC if this is a new key
	ttl := r.client.TTL(ctx, key).Val()
	if ttl == -1 { // Key exists but has no expiry
		midnight := r.secondsUntilMidnight()
		r.client.Expire(ctx, key, time.Duration(midnight)*time.Second)
	}

	return decimal.NewFromFloat(result), nil
}

// Get retrieves current daily total
func (r *dailyLimitsRepo) Get(ctx context.Context, userID int64, operationType string) (decimal.Decimal, error) {
	key := r.buildKey(userID, operationType)

	result, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return decimal.Zero, nil
		}
		return decimal.Zero, fmt.Errorf("get daily limit: %w", err)
	}

	amount, err := decimal.NewFromString(result)
	if err != nil {
		return decimal.Zero, fmt.Errorf("parse daily limit: %w", err)
	}

	return amount, nil
}

// Reset clears daily totals for a user (for testing)
func (r *dailyLimitsRepo) Reset(ctx context.Context, userID int64, operationType string) error {
	key := r.buildKey(userID, operationType)

	err := r.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("reset daily limit: %w", err)
	}

	return nil
}

func (r *dailyLimitsRepo) buildKey(userID int64, operationType string) string {
	today := time.Now().UTC().Format("2006-01-02")
	return fmt.Sprintf("daily_limit:%d:%s:%s", userID, operationType, today)
}

// secondsUntilMidnight calculates seconds until next midnight UTC
func (r *dailyLimitsRepo) secondsUntilMidnight() int {
	now := time.Now().UTC()
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	return int(midnight.Sub(now).Seconds())
}
