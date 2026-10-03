package handlers

import (
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

// SuccessResponse is the standard success envelope.
type SuccessResponse struct {
	Data interface{} `json:"data"`
	Meta Meta        `json:"meta"`
}

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
	Meta  Meta      `json:"meta"`
}

// ErrorBody carries a namespaced error code (RG_*).
type ErrorBody struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// Meta carries request correlation data.
type Meta struct {
	RequestID string `json:"request_id"`
	Timestamp string `json:"timestamp"`
}

func respondSuccess(c *fiber.Ctx, status int, data interface{}) error {
	return c.Status(status).JSON(SuccessResponse{Data: data, Meta: buildMeta(c)})
}

func respondError(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(ErrorResponse{
		Error: ErrorBody{Code: code, Message: message},
		Meta:  buildMeta(c),
	})
}

func respondErrorWithDetails(c *fiber.Ctx, status int, code, message string, details interface{}) error {
	return c.Status(status).JSON(ErrorResponse{
		Error: ErrorBody{Code: code, Message: message, Details: details},
		Meta:  buildMeta(c),
	})
}

func respondValidationError(c *fiber.Ctx, errs validator.ValidationErrors) error {
	fields := make([]map[string]string, 0, len(errs))
	for _, e := range errs {
		fields = append(fields, map[string]string{"field": e.Field(), "message": validationMessage(e)})
	}
	return respondErrorWithDetails(c, fiber.StatusBadRequest, "RG_VALIDATION_FAILED", "Validation failed", fields)
}

func buildMeta(c *fiber.Ctx) Meta {
	reqID := c.Get("X-Request-ID")
	if reqID == "" {
		if v, ok := c.Locals("request_id").(string); ok {
			reqID = v
		}
	}
	return Meta{RequestID: reqID, Timestamp: time.Now().UTC().Format(time.RFC3339Nano)}
}

func validationMessage(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return "This field is required"
	case "oneof":
		return "Must be one of: " + e.Param()
	case "len":
		return "Must be exactly " + e.Param() + " characters"
	case "min":
		return "Must be at least " + e.Param()
	default:
		return "Invalid value"
	}
}
