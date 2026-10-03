package handlers

import "github.com/gofiber/fiber/v2"

// getUserID extracts the JWT-derived user id set by AuthMiddleware.
// NEVER-7: identity comes from the token, never from the request body.
func getUserID(c *fiber.Ctx) int64 {
	if v, ok := c.Locals("user_id").(int64); ok {
		return v
	}
	return 0
}

// getAdminID extracts the operator identity set by AdminMiddleware.
func getAdminID(c *fiber.Ctx) string {
	if v, ok := c.Locals("admin_id").(string); ok && v != "" {
		return v
	}
	return "admin"
}
