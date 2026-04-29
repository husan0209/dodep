package middleware

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

const (
	maintenanceKey    = "admin:maintenance"
	maintenanceEtaKey = "admin:maintenance:eta"
)

// MaintenanceMode returns 503 for all non-SUPER_ADMIN requests when
// the "admin:maintenance" key is set to "1" in Redis/DragonflyDB.
// SUPER_ADMIN always bypasses maintenance.
func MaintenanceMode(rdb *redis.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, _ := c.Locals("admin_role").(string)
		if role == "super_admin" || role == "SUPER_ADMIN" {
			return c.Next()
		}

		ctx, cancel := context.WithTimeout(c.Context(), 200*time.Millisecond)
		defer cancel()

		val, err := rdb.Get(ctx, maintenanceKey).Result()
		if err != nil || val != "1" {
			return c.Next()
		}

		eta, _ := rdb.Get(ctx, maintenanceEtaKey).Result()
		resp := fiber.Map{
			"error":   "MAINTENANCE_MODE",
			"message": "System is under scheduled maintenance. Please try again later.",
		}
		if eta != "" {
			resp["eta"] = eta
		}
		return c.Status(503).JSON(resp)
	}
}
