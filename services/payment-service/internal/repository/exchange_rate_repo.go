package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
)

// exchangeRateRepo implements ExchangeRateRepository using Redis
type exchangeRateRepo struct {
	client *redis.Client
}

// NewExchangeRateRepository creates a new exchange rate repository
func NewExchangeRateRepository(client *redis.Client) ExchangeRateRepository {
	return &exchangeRateRepo{client: client}
}

// Get retrieves a cached exchange rate
func (r *exchangeRateRepo) Get(ctx context.Context, fromCurrency, toCurrency string) (*decimal.Decimal, error) {
	key := r.buildKey(fromCurrency, toCurrency)

	result, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("get exchange rate: %w", err)
	}

	rate, err := decimal.NewFromString(result)
	if err != nil {
		return nil, fmt.Errorf("parse exchange rate: %w", err)
	}

	return &rate, nil
}

// Set stores an exchange rate with TTL
func (r *exchangeRateRepo) Set(ctx context.Context, fromCurrency, toCurrency string, rate decimal.Decimal, ttlSeconds int) error {
	key := r.buildKey(fromCurrency, toCurrency)

	err := r.client.Set(ctx, key, rate.String(), time.Duration(ttlSeconds)*time.Second).Err()
	if err != nil {
		return fmt.Errorf("set exchange rate: %w", err)
	}

	return nil
}

// Delete removes a cached rate
func (r *exchangeRateRepo) Delete(ctx context.Context, fromCurrency, toCurrency string) error {
	key := r.buildKey(fromCurrency, toCurrency)

	err := r.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("delete exchange rate: %w", err)
	}

	return nil
}

func (r *exchangeRateRepo) buildKey(from, to string) string {
	return fmt.Sprintf("exchange_rate:%s:%s", from, to)
}
