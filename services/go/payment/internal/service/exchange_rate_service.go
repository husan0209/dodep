package service

import (
	"context"
	"fmt"

	"github.com/opus-casino/payment/internal/client"
	"github.com/opus-casino/payment/internal/repository"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// ExchangeRateService handles exchange rate retrieval with caching
type ExchangeRateService struct {
	rateRepo  repository.ExchangeRateRepository
	npClient  *client.NOWPaymentsClient
	logger    *zap.Logger
	cacheTTL  int // Cache TTL in seconds
}

// ExchangeRateConfig holds configuration for the exchange rate service
type ExchangeRateConfig struct {
	// CacheTTL is the time-to-live for cached exchange rates in seconds
	CacheTTL int
}

// DefaultExchangeRateConfig returns the default configuration
func DefaultExchangeRateConfig() ExchangeRateConfig {
	return ExchangeRateConfig{
		CacheTTL: 60, // 60 seconds as per design
	}
}

// NewExchangeRateService creates a new exchange rate service
func NewExchangeRateService(
	rateRepo repository.ExchangeRateRepository,
	npClient *client.NOWPaymentsClient,
	logger *zap.Logger,
	cfg ExchangeRateConfig,
) *ExchangeRateService {
	return &ExchangeRateService{
		rateRepo: rateRepo,
		npClient: npClient,
		logger:   logger,
		cacheTTL: cfg.CacheTTL,
	}
}

// GetExchangeRate retrieves the exchange rate with cache-first strategy
// Returns the rate for converting fromCurrency to toCurrency
func (s *ExchangeRateService) GetExchangeRate(
	ctx context.Context,
	fromCurrency, toCurrency string,
) (decimal.Decimal, error) {
	// Try cache first
	cachedRate, err := s.rateRepo.Get(ctx, fromCurrency, toCurrency)
	if err != nil {
		s.logger.Warn("Failed to get cached exchange rate, fetching fresh",
			zap.String("from_currency", fromCurrency),
			zap.String("to_currency", toCurrency),
			zap.Error(err),
		)
		// Fall through to fetch fresh rate
	}

	if cachedRate != nil {
		s.logger.Debug("Exchange rate cache hit",
			zap.String("from_currency", fromCurrency),
			zap.String("to_currency", toCurrency),
			zap.String("rate", cachedRate.String()),
		)
		return *cachedRate, nil
	}

	// Cache miss or stale - fetch fresh rate from NOWPayments
	rate, err := s.fetchAndCacheRate(ctx, fromCurrency, toCurrency)
	if err != nil {
		// If we have a stale cached rate, return it as fallback
		if cachedRate != nil {
			s.logger.Warn("Using stale cached rate as fallback",
				zap.String("from_currency", fromCurrency),
				zap.String("to_currency", toCurrency),
				zap.String("rate", cachedRate.String()),
				zap.Error(err),
			)
			return *cachedRate, nil
		}
		return decimal.Zero, fmt.Errorf("get exchange rate: %w", err)
	}

	return rate, nil
}

// fetchAndCacheRate fetches a fresh rate from NOWPayments and caches it
func (s *ExchangeRateService) fetchAndCacheRate(
	ctx context.Context,
	fromCurrency, toCurrency string,
) (decimal.Decimal, error) {
	// Use a base amount of 1 to get the pure exchange rate
	baseAmount := decimal.NewFromInt(1)

	resp, err := s.npClient.GetEstimatedPrice(ctx, baseAmount, fromCurrency, toCurrency)
	if err != nil {
		return decimal.Zero, fmt.Errorf("fetch exchange rate from NOWPayments: %w", err)
	}

	rate := resp.EstimatedAmount

	// Cache the rate
	if err := s.rateRepo.Set(ctx, fromCurrency, toCurrency, rate, s.cacheTTL); err != nil {
		s.logger.Warn("Failed to cache exchange rate",
			zap.String("from_currency", fromCurrency),
			zap.String("to_currency", toCurrency),
			zap.String("rate", rate.String()),
			zap.Error(err),
		)
		// Don't fail the request if caching fails
	}

	s.logger.Info("Exchange rate fetched and cached",
		zap.String("from_currency", fromCurrency),
		zap.String("to_currency", toCurrency),
		zap.String("rate", rate.String()),
		zap.Int("ttl_seconds", s.cacheTTL),
	)

	return rate, nil
}

// ConvertToFiat converts a crypto amount to fiat currency
// Uses cached exchange rate with automatic refresh on stale data
func (s *ExchangeRateService) ConvertToFiat(
	ctx context.Context,
	cryptoAmount decimal.Decimal,
	cryptoCurrency, fiatCurrency string,
) (decimal.Decimal, error) {
	if cryptoAmount.IsZero() {
		return decimal.Zero, nil
	}

	rate, err := s.GetExchangeRate(ctx, cryptoCurrency, fiatCurrency)
	if err != nil {
		return decimal.Zero, fmt.Errorf("get exchange rate for conversion: %w", err)
	}

	fiatAmount := cryptoAmount.Mul(rate)

	s.logger.Debug("Converted crypto to fiat",
		zap.String("crypto_amount", cryptoAmount.String()),
		zap.String("crypto_currency", cryptoCurrency),
		zap.String("fiat_amount", fiatAmount.String()),
		zap.String("fiat_currency", fiatCurrency),
		zap.String("rate", rate.String()),
	)

	return fiatAmount, nil
}

// ConvertFromFiat converts a fiat amount to crypto currency
// Uses cached exchange rate with automatic refresh on stale data
func (s *ExchangeRateService) ConvertFromFiat(
	ctx context.Context,
	fiatAmount decimal.Decimal,
	fiatCurrency, cryptoCurrency string,
) (decimal.Decimal, error) {
	if fiatAmount.IsZero() {
		return decimal.Zero, nil
	}

	rate, err := s.GetExchangeRate(ctx, fiatCurrency, cryptoCurrency)
	if err != nil {
		return decimal.Zero, fmt.Errorf("get exchange rate for conversion: %w", err)
	}

	cryptoAmount := fiatAmount.Mul(rate)

	s.logger.Debug("Converted fiat to crypto",
		zap.String("fiat_amount", fiatAmount.String()),
		zap.String("fiat_currency", fiatCurrency),
		zap.String("crypto_amount", cryptoAmount.String()),
		zap.String("crypto_currency", cryptoCurrency),
		zap.String("rate", rate.String()),
	)

	return cryptoAmount, nil
}

// RefreshRate forces a refresh of the exchange rate cache
func (s *ExchangeRateService) RefreshRate(
	ctx context.Context,
	fromCurrency, toCurrency string,
) (decimal.Decimal, error) {
	// Delete existing cache
	if err := s.rateRepo.Delete(ctx, fromCurrency, toCurrency); err != nil {
		s.logger.Warn("Failed to delete cached rate before refresh",
			zap.String("from_currency", fromCurrency),
			zap.String("to_currency", toCurrency),
			zap.Error(err),
		)
	}

	// Fetch fresh rate
	return s.fetchAndCacheRate(ctx, fromCurrency, toCurrency)
}

// GetRateWithAmount gets the estimated price for a specific amount
// This is useful when you need the exact conversion for a specific transaction
func (s *ExchangeRateService) GetRateWithAmount(
	ctx context.Context,
	amount decimal.Decimal,
	fromCurrency, toCurrency string,
) (decimal.Decimal, error) {
	if amount.IsZero() {
		return decimal.Zero, nil
	}

	resp, err := s.npClient.GetEstimatedPrice(ctx, amount, fromCurrency, toCurrency)
	if err != nil {
		return decimal.Zero, fmt.Errorf("get estimated price: %w", err)
	}

	return resp.EstimatedAmount, nil
}
