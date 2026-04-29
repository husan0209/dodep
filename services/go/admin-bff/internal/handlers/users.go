package handlers

import (
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/opus-casino/admin-bff/internal/service"
	userv1 "github.com/opus-casino/proto/gen/go/user/v1"
)

func RegisterUserRoutes(router fiber.Router, svc *service.UsersService, log *zap.Logger) {
	users := router.Group("/users")

	users.Get("/:id", func(c *fiber.Ctx) error {
		id, err := c.ParamsInt("id")
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid user id"})
		}
		user, err := svc.GetUser(c.Context(), int64(id))
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "user not found"})
		}
		return c.JSON(fiber.Map{"data": user})
	})

	users.Patch("/:id", func(c *fiber.Ctx) error {
		id, err := c.ParamsInt("id")
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid user id"})
		}
		var req userv1.UpdateUserRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		adminID := int64(c.Locals("admin_id").(float64))
		if err := svc.UpdateUser(c.Context(), adminID, int64(id), &req); err != nil {
			log.Error("update user failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "update failed"})
		}
		return c.JSON(fiber.Map{"data": fiber.Map{"success": true}})
	})

	users.Get("/:id/activity", func(c *fiber.Ctx) error {
		id, err := c.ParamsInt("id")
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid user id"})
		}
		pageSize := c.QueryInt("page_size", 50)
		pageToken := c.Query("page_token", "")
		activities, page, err := svc.GetActivity(c.Context(), int64(id), int32(pageSize), pageToken)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to get activity"})
		}
		return c.JSON(fiber.Map{"data": activities, "pagination": page})
	})
}
