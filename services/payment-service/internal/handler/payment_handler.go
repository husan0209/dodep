package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/platform/services/payment-service/internal/domain"
	"github.com/platform/services/payment-service/internal/service"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// PaymentHandler handles payment HTTP requests
type PaymentHandler struct {
	paymentSvc *service.PaymentService
	logger     *zap.Logger
}

// NewPaymentHandler creates a new payment handler
func NewPaymentHandler(paymentSvc *service.PaymentService, logger *zap.Logger) *PaymentHandler {
	return &PaymentHandler{
		paymentSvc: paymentSvc,
		logger:     logger,
	}
}

// DepositRequest represents a deposit request
type DepositRequest struct {
	Amount         string `json:"amount" validate:"required,gt=0"`
	Currency       string `json:"currency" validate:"required"`
	IdempotencyKey string `json:"idempotency_key" validate:"required"`
}

// DepositResponse represents a deposit response
type DepositResponse struct {
	PaymentUUID string `json:"payment_uuid"`
	PaymentID   string `json:"payment_id"`
	PayAddress  string `json:"pay_address"`
	PayAmount   string `json:"pay_amount"`
	PayCurrency string `json:"pay_currency"`
	FiatAmount  string `json:"fiat_amount"`
	ExpiresAt   string `json:"expires_at"`
}

// InitiateDeposit handles POST /api/v1/payments/deposit
func (h *PaymentHandler) InitiateDeposit(c *fiber.Ctx) error {
	var req DepositRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, 400, "invalid request body")
	}

	// Parse amount
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		return respondError(c, 400, "invalid amount format")
	}

	// Get user ID from context (set by auth middleware)
	userID := c.Locals("user_id").(int64)

	// Create deposit request
	result, err := h.paymentSvc.InitiateDeposit(c.Context(), service.InitiateDepositRequest{
		UserID:         userID,
		Amount:         amount,
		Currency:       domain.CryptoCurrency(req.Currency),
		IdempotencyKey: req.IdempotencyKey,
		IPAddress:      c.IP(),
		UserAgent:      c.Get("User-Agent"),
	})
	if err != nil {
		return h.handleError(c, err)
	}

	return respondSuccess(c, 201, DepositResponse{
		PaymentUUID: result.PaymentUUID,
		PaymentID:   result.PaymentID,
		PayAddress:  result.PayAddress,
		PayAmount:   result.PayAmount.String(),
		PayCurrency: result.PayCurrency,
		FiatAmount:  result.FiatAmount.String(),
		ExpiresAt:   result.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// GetPayment handles GET /api/v1/payments/:id
func (h *PaymentHandler) GetPayment(c *fiber.Ctx) error {
	paymentUUID := c.Params("id")
	if paymentUUID == "" {
		return respondError(c, 400, "payment id required")
	}

	payment, err := h.paymentSvc.GetPayment(c.Context(), paymentUUID)
	if err != nil {
		return h.handleError(c, err)
	}

	return respondSuccess(c, 200, payment)
}

// PaymentHistoryResponse represents payment history response
type PaymentHistoryResponse struct {
	Items      []PaymentItem `json:"items"`
	NextCursor string        `json:"next_cursor"`
	HasMore    bool          `json:"has_more"`
}

// PaymentItem represents a payment item in history
type PaymentItem struct {
	PaymentUUID string `json:"payment_uuid"`
	Amount      string `json:"amount"`
	Currency    string `json:"currency"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}

// GetPaymentHistory handles GET /api/v1/payments/history
func (h *PaymentHandler) GetPaymentHistory(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(int64)

	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	cursor := c.Query("cursor")
	status := c.Query("status")

	result, err := h.paymentSvc.ListPayments(c.Context(), service.ListPaymentsRequest{
		UserID: userID,
		Limit:  limit,
		Cursor: cursor,
		Status: status,
	})
	if err != nil {
		return h.handleError(c, err)
	}

	items := make([]PaymentItem, len(result.Items))
	for i, p := range result.Items {
		items[i] = PaymentItem{
			PaymentUUID: p.UUID.String(),
			Amount:      p.FiatAmount.String(),
			Currency:    p.CryptoCurrency,
			Status:      string(p.Status),
			CreatedAt:   p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	return respondSuccess(c, 200, PaymentHistoryResponse{
		Items:      items,
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	})
}

// PaymentMethodsResponse represents available payment methods
type PaymentMethodsResponse struct {
	Currencies []CurrencyInfo `json:"currencies"`
}

// CurrencyInfo represents currency information
type CurrencyInfo struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Network     string `json:"network"`
	MinAmount   string `json:"min_amount"`
	IsAvailable bool   `json:"is_available"`
}

// GetPaymentMethods handles GET /api/v1/payments/methods
func (h *PaymentHandler) GetPaymentMethods(c *fiber.Ctx) error {
	currencies := domain.SupportedDepositCurrencies()

	items := make([]CurrencyInfo, len(currencies))
	for i, c := range currencies {
		items[i] = CurrencyInfo{
			Code:        string(c),
			Name:        c.String(),
			Network:     c.Network(),
			MinAmount:   "10.00",
			IsAvailable: true,
		}
	}

	return respondSuccess(c, 200, PaymentMethodsResponse{Currencies: items})
}

// handleError maps domain errors to HTTP responses
func (h *PaymentHandler) handleError(c *fiber.Ctx, err error) error {
	status := domain.HTTPStatus(err)
	code := domain.GetErrorCode(err)
	message := err.Error()

	h.logger.Error("Payment handler error",
		zap.Error(err),
		zap.Int("status", status),
		zap.Int("code", code),
	)

	return c.Status(status).JSON(ErrorResponse{
		Code:    code,
		Message: message,
	})
}
