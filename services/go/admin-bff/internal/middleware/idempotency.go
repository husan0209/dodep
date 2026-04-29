package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// IdempotencyKeyHeader is the HTTP header carrying the idempotency key.
const IdempotencyKeyHeader = "X-Idempotency-Key"

// ExtractIdempotencyKey reads and validates the idempotency key from the request.
// It stores the key in c.Locals("idempotency_key") so handlers can use it.
// GET/HEAD/OPTIONS requests are skipped.
func ExtractIdempotencyKey() fiber.Handler {
	return func(c *fiber.Ctx) error {
		method := c.Method()
		if method == fiber.MethodGet || method == fiber.MethodHead || method == fiber.MethodOptions {
			return c.Next()
		}

		key := c.Get(IdempotencyKeyHeader)
		if key == "" {
			return c.Status(400).JSON(fiber.Map{
				"error": "missing X-Idempotency-Key header for mutating request",
			})
		}

		if _, err := uuid.Parse(key); err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error": "invalid X-Idempotency-Key format, expected UUID",
			})
		}

		c.Locals("idempotency_key", key)
		return c.Next()
	}
}
