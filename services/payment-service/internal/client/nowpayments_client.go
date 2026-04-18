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
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// NOWPaymentsClient handles communication with NOWPayments API
type NOWPaymentsClient struct {
	baseURL    string
	apiKey     string
	ipnSecret  string
	httpClient *http.Client
	logger     *zap.Logger
}

// NOWPaymentsConfig holds configuration for NOWPayments client
type NOWPaymentsConfig struct {
	BaseURL   string
	APIKey    string
	IPNSecret string
	Timeout   time.Duration
	MaxRetries int
	RetryDelay time.Duration
}

// NewNOWPaymentsClient creates a new NOWPayments client
func NewNOWPaymentsClient(cfg NOWPaymentsConfig, logger *zap.Logger) *NOWPaymentsClient {
	return &NOWPaymentsClient{
		baseURL:   cfg.BaseURL,
		apiKey:    cfg.APIKey,
		ipnSecret: cfg.IPNSecret,
		logger:    logger,
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

// doRequest performs an HTTP request with authentication
func (c *NOWPaymentsClient) doRequest(ctx context.Context, method, path string, reqBody, respBody interface{}) error {
	var bodyReader io.Reader
	if reqBody != nil {
		body, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(body)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		c.logger.Error("NOWPayments API error",
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
