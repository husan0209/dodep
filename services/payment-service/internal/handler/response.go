package handler

import (
	"github.com/gofiber/fiber/v2"
)

// ErrorResponse represents an error response
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// SuccessResponse represents a success response
type SuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

// respondSuccess sends a success response
func respondSuccess(c *fiber.Ctx, status int, data interface{}) error {
	return c.Status(status).JSON(SuccessResponse{
		Success: true,
		Data:    data,
	})
}

// respondError sends an error response
func respondError(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(ErrorResponse{
		Code:    status * 10,
		Message: message,
	})
}

// respondPaginated sends a paginated response
func respondPaginated(c *fiber.Ctx, items interface{}, nextCursor string, hasMore bool) error {
	return c.Status(200).JSON(fiber.Map{
		"success":     true,
		"data":        items,
		"next_cursor": nextCursor,
		"has_more":    hasMore,
	})
}
