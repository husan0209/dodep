package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// Meta represents response metadata
type Meta struct {
	RequestID string `json:"request_id"`
	Timestamp string `json:"timestamp"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
	Meta  Meta        `json:"meta"`
}

// ErrorDetail represents error details
type ErrorDetail struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// SuccessResponse represents a success response
type SuccessResponse struct {
	Data interface{} `json:"data"`
	Meta Meta        `json:"meta"`
}

// NOTE: a generic `PaginatedResponse`/`Pagination` envelope and a `respondPaginated`
// helper used to live here. They were never called: the list endpoints
// (GetPaymentHistory, GetWithdrawalHistory) answer with their own typed bodies
// (PaymentHistoryResponse / WithdrawalHistoryResponse), which already carry
// `next_cursor` and `has_more`. Dead code that golangci-lint's `unused` linter
// rejected, so it is gone rather than kept as a second, contradictory shape.

// respondSuccess sends a success response
func respondSuccess(c *fiber.Ctx, status int, data interface{}) error {
	return c.Status(status).JSON(SuccessResponse{
		Data: data,
		Meta: Meta{
			RequestID: getRequestID(c),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	})
}

// respondError sends an error response
func respondError(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(ErrorResponse{
		Error: ErrorDetail{
			Code:    status * 10,
			Message: message,
		},
		Meta: Meta{
			RequestID: getRequestID(c),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	})
}

// getRequestID extracts request ID from context or generates a new one
func getRequestID(c *fiber.Ctx) string {
	if rid, ok := c.Locals("requestid").(string); ok && rid != "" {
		return rid
	}
	return uuid.New().String()
}
