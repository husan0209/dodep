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
	Error  ErrorDetail `json:"error"`
	Meta   Meta        `json:"meta"`
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

// PaginatedResponse represents a paginated response
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Pagination Pagination  `json:"pagination"`
	Meta       Meta        `json:"meta"`
}

// Pagination represents pagination info
type Pagination struct {
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

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

// respondPaginated sends a paginated response
func respondPaginated(c *fiber.Ctx, items interface{}, nextCursor string, hasMore bool) error {
	return c.Status(200).JSON(PaginatedResponse{
		Data: items,
		Pagination: Pagination{
			NextCursor: nextCursor,
			HasMore:    hasMore,
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
