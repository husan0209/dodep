package handler

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/opus-casino/payment/internal/domain"
	"github.com/opus-casino/payment/internal/service"
	"github.com/rs/zerolog/log"
	"github.com/shopspring/decimal"
)

// WithdrawalHandler handles withdrawal HTTP requests
type WithdrawalHandler struct {
	withdrawalSvc *service.WithdrawalService
}

// NewWithdrawalHandler creates a new withdrawal handler
func NewWithdrawalHandler(withdrawalSvc *service.WithdrawalService) *WithdrawalHandler {
	return &WithdrawalHandler{
		withdrawalSvc: withdrawalSvc,
	}
}

// WithdrawRequest represents a withdrawal request
type WithdrawRequest struct {
	Amount         string `json:"amount" validate:"required,gt=0"`
	Currency       string `json:"currency" validate:"required"`
	Address        string `json:"address" validate:"required"`
	IdempotencyKey string `json:"idempotency_key" validate:"required"`
}

// WithdrawResponse represents a withdrawal response
type WithdrawResponse struct {
	WithdrawalUUID string `json:"withdrawal_uuid"`
	WithdrawalID   string `json:"withdrawal_id"`
	Amount         string `json:"amount"`
	FiatAmount     string `json:"fiat_amount"`
	Currency       string `json:"currency"`
	Address        string `json:"address"`
	Status         string `json:"status"`
}

// InitiateWithdrawal handles POST /api/v1/payments/withdraw
func (h *WithdrawalHandler) InitiateWithdrawal(c *fiber.Ctx) error {
	var req WithdrawRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, 400, "invalid request body")
	}

	// Parse amount
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		return respondError(c, 400, "invalid amount format")
	}

	// Get user ID from context
	userID := c.Locals("user_id").(int64)

	// Create withdrawal request
	result, err := h.withdrawalSvc.InitiateWithdrawal(c.Context(), service.InitiateWithdrawalRequest{
		UserID:         userID,
		Amount:         amount,
		Currency:       domain.CryptoCurrency(req.Currency),
		Address:        req.Address,
		IdempotencyKey: req.IdempotencyKey,
		IPAddress:      c.IP(),
		UserAgent:      c.Get("User-Agent"),
	})
	if err != nil {
		return h.handleError(c, err)
	}

	return respondSuccess(c, 201, WithdrawResponse{
		WithdrawalUUID: result.WithdrawalUUID,
		WithdrawalID:   result.WithdrawalID,
		Amount:         result.Amount.String(),
		FiatAmount:     result.FiatAmount.String(),
		Currency:       result.Currency,
		Address:        result.Address,
		Status:         result.Status,
	})
}

// GetWithdrawal handles GET /api/v1/payments/withdrawals/:id
func (h *WithdrawalHandler) GetWithdrawal(c *fiber.Ctx) error {
	withdrawalUUID := c.Params("id")
	if withdrawalUUID == "" {
		return respondError(c, 400, "withdrawal id required")
	}

	withdrawal, err := h.withdrawalSvc.GetWithdrawal(c.Context(), withdrawalUUID)
	if err != nil {
		return h.handleError(c, err)
	}

	return respondSuccess(c, 200, withdrawal)
}

// WithdrawalHistoryResponse represents withdrawal history response
type WithdrawalHistoryResponse struct {
	Items      []WithdrawalItem `json:"items"`
	NextCursor string           `json:"next_cursor"`
	HasMore    bool             `json:"has_more"`
}

// WithdrawalItem represents a withdrawal item in history
type WithdrawalItem struct {
	WithdrawalUUID string `json:"withdrawal_uuid"`
	Amount         string `json:"amount"`
	Currency       string `json:"currency"`
	Address        string `json:"address"`
	Status         string `json:"status"`
	CreatedAt      string `json:"created_at"`
}

// GetWithdrawalHistory handles GET /api/v1/payments/withdrawals/history
func (h *WithdrawalHandler) GetWithdrawalHistory(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(int64)

	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	cursor := c.Query("cursor")
	status := c.Query("status")

	result, err := h.withdrawalSvc.ListWithdrawals(c.Context(), service.ListPaymentsRequest{
		UserID: userID,
		Limit:  limit,
		Cursor: cursor,
		Status: status,
	})
	if err != nil {
		return h.handleError(c, err)
	}

	items := make([]WithdrawalItem, len(result.Items))
	for i, w := range result.Items {
		items[i] = WithdrawalItem{
			WithdrawalUUID: w.UUID.String(),
			Amount:         w.FiatAmount.String(),
			Currency:       w.CryptoCurrency,
			Address:        maskAddress(w.Address),
			Status:         string(w.Status),
			CreatedAt:      w.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	return respondSuccess(c, 200, WithdrawalHistoryResponse{
		Items:      items,
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	})
}

// handleError maps domain errors to HTTP responses
func (h *WithdrawalHandler) handleError(c *fiber.Ctx, err error) error {
	status := domain.HTTPStatus(err)
	code := domain.GetErrorCode(err)
	message := err.Error()

	log.Error().
		Err(err).
		Int("status", status).
		Int("code", code).
		Msg("Withdrawal handler error")

	return c.Status(status).JSON(ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
		Meta: Meta{
			RequestID: getRequestID(c),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	})
}

// maskAddress masks a crypto address for display
func maskAddress(addr string) string {
	if len(addr) <= 12 {
		return addr
	}
	return addr[:8] + "..." + addr[len(addr)-4:]
}
