package main

import "github.com/gofiber/fiber/v2"

// writeAPIError renders the standard error envelope for package-main routes
// (middleware failures happen before the handlers package is involved).
func writeAPIError(c *fiber.Ctx, status int, code, message string, details fiber.Map) error {
	requestID := ""
	if v, ok := c.Locals("request_id").(string); ok {
		requestID = v
	}
	if requestID == "" {
		requestID = c.Get("X-Request-ID")
	}
	return c.Status(status).JSON(fiber.Map{
		"error": fiber.Map{
			"code": code, "message": message, "details": details,
		},
		"code": code, "message": message, "details": details,
		"request_id": requestID,
	})
}
