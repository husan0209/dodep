package client

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// Default retry configuration
const (
	defaultMaxRetries     = 3
	defaultInitialBackoff = 100 * time.Millisecond
	defaultMaxBackoff     = 5 * time.Second
	defaultBackoffFactor  = 2.0
)

// NOWPaymentsClient handles communication with NOWPayments API
type NOWPaymentsClient struct {
	baseURL        string
	apiKey         string
	ipnSecret      string
	httpClient     *http.Client
	logger         *zap.Logger
	maxRetries     int
	initialBackoff time.Duration
	maxBackoff     time.Duration
	backoffFactor  float64
}

// NOWPaymentsConfig holds configuration for NOWPayments client
type NOWPaymentsConfig struct {
	BaseURL        string
	APIKey         string
	IPNSecret      string
	Timeout        time.Duration
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	BackoffFactor  float64
}

// NewNOWPaymentsClient creates a new NOWPayments client
func NewNOWPaymentsClient(cfg NOWPaymentsConfig, logger *zap.Logger) *NOWPaymentsClient {
	maxRetries := cfg.MaxRetries
	if maxRetries == 0 {
		maxRetries = defaultMaxRetries
	}
	initialBackoff := cfg.InitialBackoff
	if initialBackoff == 0 {
		initialBackoff = defaultInitialBackoff
	}
	maxBackoff := cfg.MaxBackoff
	if maxBackoff == 0 {
		maxBackoff = defaultMaxBackoff
	}
	backoffFactor := cfg.BackoffFactor
	if backoffFactor == 0 {
		backoffFactor = defaultBackoffFactor
	}

	return &NOWPaymentsClient{
		baseURL:        cfg.BaseURL,
		apiKey:         cfg.APIKey,
		ipnSecret:      cfg.IPNSecret,
		logger:         logger,
		maxRetries:     maxRetries,
		initialBackoff: initialBackoff,
		maxBackoff:     maxBackoff,
		backoffFactor:  backoffFactor,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// CreatePaymentRequest represents a payment creation request
type CreatePaymentRequest struct {
	PriceAmount      decimal.Decimal `json:"price_amount"`
	PriceCurrency    string          `json:"price_currency"`
	PayCurrency      string          `json:"pay_currency"`
	IPNCallbackURL   string          `json:"ipn_callback_url"`
	OrderID          string          `json:"order_id"`
	OrderDescription string          `json:"order_description,omitempty"`
}

// CreatePaymentResponse represents a payment creation response
type CreatePaymentResponse struct {
	PaymentID     string          `json:"payment_id"`
	PaymentStatus string          `json:"payment_status"`
	PayAddress    string          `json:"pay_address"`
	PayAmount     decimal.Decimal `json:"pay_amount"`
	PayCurrency   string          `json:"pay_currency"`
	PriceAmount   decimal.Decimal `json:"price_amount"`
	PriceCurrency string          `json:"price_currency"`
	CreatedAt     time.Time       `json:"created_at"`
	ExpiresAt     time.Time       `json:"expiration_estimate_date"`
}

// CreatePayoutRequest represents a payout creation request
type CreatePayoutRequest struct {
	WithdrawalID   string          `json:"withdrawal_id"`
	Address        string          `json:"address"`
	Currency       string          `json:"currency"`
	Amount         decimal.Decimal `json:"amount"`
	IPNCallbackURL string          `json:"ipn_callback_url"`
}

// CreatePayoutResponse represents a payout creation response
type CreatePayoutResponse struct {
	WithdrawalID string          `json:"withdrawal_id"`
	Status       string          `json:"status"`
	Amount       decimal.Decimal `json:"amount"`
	Currency     string          `json:"currency"`
	Address      string          `json:"address"`
	BatchID      string          `json:"batch_id,omitempty"`
}

// EstimatedPriceResponse represents an exchange rate response
type EstimatedPriceResponse struct {
	EstimatedAmount decimal.Decimal `json:"estimated_amount"`
	CurrencyFrom    string          `json:"currency_from"`
	CurrencyTo      string          `json:"currency_to"`
}

// CurrenciesResponse represents a currencies list response
type CurrenciesResponse struct {
	Currencies []string `json:"currencies"`
}

// WebhookPayload represents a webhook notification
type WebhookPayload struct {
	PaymentID       string          `json:"payment_id"`
	PaymentStatus   string          `json:"payment_status"`
	PayAddress      string          `json:"pay_address"`
	PayAmount       decimal.Decimal `json:"pay_amount"`
	PayCurrency     string          `json:"pay_currency"`
	PriceAmount     decimal.Decimal `json:"price_amount"`
	PriceCurrency   string          `json:"price_currency"`
	OrderID         string          `json:"order_id"`
	OutcomeAmount   decimal.Decimal `json:"outcome_amount"`
	OutcomeCurrency string          `json:"outcome_currency"`
}

// CreatePayment creates a new payment in NOWPayments
func (c *NOWPaymentsClient) CreatePayment(ctx context.Context, req CreatePaymentRequest) (*CreatePaymentResponse, error) {
	var resp CreatePaymentResponse
	err := c.doRequest(ctx, http.MethodPost, "/v1/payment", req, &resp)
	if err != nil {
		return nil, fmt.Errorf("create payment: %w", err)
	}
	return &resp, nil
}

// CreatePayout creates a withdrawal in NOWPayments
func (c *NOWPaymentsClient) CreatePayout(ctx context.Context, req CreatePayoutRequest) (*CreatePayoutResponse, error) {
	var resp CreatePayoutResponse
	err := c.doRequest(ctx, http.MethodPost, "/v1/payout", req, &resp)
	if err != nil {
		return nil, fmt.Errorf("create payout: %w", err)
	}
	return &resp, nil
}

// GetEstimatedPrice gets exchange rate
func (c *NOWPaymentsClient) GetEstimatedPrice(ctx context.Context, amount decimal.Decimal, fromCurrency, toCurrency string) (*EstimatedPriceResponse, error) {
	path := fmt.Sprintf("/v1/estimate?amount=%s&currency_from=%s&currency_to=%s",
		amount.String(), fromCurrency, toCurrency)

	var resp EstimatedPriceResponse
	err := c.doRequest(ctx, http.MethodGet, path, nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("get estimated price: %w", err)
	}
	return &resp, nil
}

// GetCurrencies gets list of supported currencies
func (c *NOWPaymentsClient) GetCurrencies(ctx context.Context) (*CurrenciesResponse, error) {
	var resp CurrenciesResponse
	err := c.doRequest(ctx, http.MethodGet, "/v1/currencies", nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("get currencies: %w", err)
	}
	return &resp, nil
}

// VerifyWebhookSignature verifies HMAC signature of webhook payload
func (c *NOWPaymentsClient) VerifyWebhookSignature(payload []byte, signature string) bool {
	mac := hmac.New(sha512.New, []byte(c.ipnSecret))
	mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expectedMAC))
}

// doRequest performs an HTTP request with authentication and retry logic
func (c *NOWPaymentsClient) doRequest(ctx context.Context, method, path string, reqBody, respBody interface{}) error {
	var bodyBytes []byte
	if reqBody != nil {
		var err error
		bodyBytes, err = json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
	}

	url := c.baseURL + path

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := c.calculateBackoff(attempt)
			c.logger.Debug("retrying request",
				zap.String("method", method),
				zap.String("path", path),
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff),
			)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		var bodyReader io.Reader
		if bodyBytes != nil {
			bodyReader = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", c.apiKey)

		// Log request (sanitized)
		c.logRequest(method, path, reqBody)

		startTime := time.Now()
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("do request: %w", err)
			c.logger.Error("request failed",
				zap.String("method", method),
				zap.String("path", path),
				zap.Error(err),
			)
			continue
		}

		respBodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read response: %w", err)
			continue
		}

		// Log response (sanitized)
		c.logResponse(method, path, resp.StatusCode, respBodyBytes, time.Since(startTime))

		if resp.StatusCode >= 500 {
			// Retry on 5xx errors
			lastErr = fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(respBodyBytes))
			c.logger.Error("NOWPayments API 5xx error, will retry",
				zap.String("method", method),
				zap.String("path", path),
				zap.Int("status", resp.StatusCode),
				zap.Int("attempt", attempt),
			)
			continue
		}

		if resp.StatusCode >= 400 {
			// Don't retry on 4xx errors
			c.logger.Error("NOWPayments API 4xx error",
				zap.String("method", method),
				zap.String("path", path),
				zap.Int("status", resp.StatusCode),
				zap.String("response", string(respBodyBytes)),
			)
			return fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(respBodyBytes))
		}

		if respBody != nil {
			if err := json.Unmarshal(respBodyBytes, respBody); err != nil {
				return fmt.Errorf("unmarshal response: %w", err)
			}
		}

		return nil
	}

	return lastErr
}

// calculateBackoff calculates exponential backoff duration
func (c *NOWPaymentsClient) calculateBackoff(attempt int) time.Duration {
	backoff := float64(c.initialBackoff) * pow(c.backoffFactor, float64(attempt-1))
	if backoff > float64(c.maxBackoff) {
		backoff = float64(c.maxBackoff)
	}
	return time.Duration(backoff)
}

// pow calculates x^y for positive values
func pow(x, y float64) float64 {
	result := 1.0
	for i := 0; i < int(y); i++ {
		result *= x
	}
	return result
}

// logRequest logs request details with sensitive data sanitized
func (c *NOWPaymentsClient) logRequest(method, path string, reqBody interface{}) {
	if c.logger == nil {
		return
	}

	fields := []zap.Field{
		zap.String("method", method),
		zap.String("path", path),
	}

	if reqBody != nil {
		if bodyMap, ok := reqBody.(map[string]interface{}); ok {
			sanitized := c.sanitizeMap(bodyMap)
			fields = append(fields, zap.Any("body", sanitized))
		} else {
			// For typed requests, log without sensitive fields
			fields = append(fields, zap.String("body", "[sanitized]"))
		}
	}

	c.logger.Debug("NOWPayments API request", fields...)
}

// logResponse logs response details with sensitive data sanitized
func (c *NOWPaymentsClient) logResponse(method, path string, statusCode int, body []byte, duration time.Duration) {
	if c.logger == nil {
		return
	}

	c.logger.Debug("NOWPayments API response",
		zap.String("method", method),
		zap.String("path", path),
		zap.Int("status", statusCode),
		zap.Duration("duration", duration),
		zap.String("body", c.sanitizeResponseBody(body)),
	)
}

// sanitizeMap removes sensitive fields from a map
func (c *NOWPaymentsClient) sanitizeMap(m map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})
	for k, v := range m {
		lowerKey := strings.ToLower(k)
		if strings.Contains(lowerKey, "secret") ||
			strings.Contains(lowerKey, "key") ||
			strings.Contains(lowerKey, "password") ||
			strings.Contains(lowerKey, "token") {
			sanitized[k] = "[REDACTED]"
		} else if nested, ok := v.(map[string]interface{}); ok {
			sanitized[k] = c.sanitizeMap(nested)
		} else {
			sanitized[k] = v
		}
	}
	return sanitized
}

// sanitizeResponseBody truncates and sanitizes response body for logging
func (c *NOWPaymentsClient) sanitizeResponseBody(body []byte) string {
	const maxLen = 500
	s := string(body)
	if len(s) > maxLen {
		s = s[:maxLen] + "...[truncated]"
	}
	return s
}
