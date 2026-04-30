package main

import (
	"github.com/gofiber/fiber/v2"

	"github.com/opus-casino/user/internal/domain"
	"github.com/opus-casino/user/internal/service"
)

func setupRoutes(app *fiber.App, svc *service.UserService) {
	api := app.Group("/api/v1")

	users := api.Group("/users")
	users.Get("/:id", func(c *fiber.Ctx) error {
		id, err := c.ParamsInt("id")
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid user id"})
		}
		user, err := svc.GetUser(c.Context(), int64(id))
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "user not found"})
		}
		return c.JSON(user)
	})

	users.Put("/:id", func(c *fiber.Ctx) error {
		id, err := c.ParamsInt("id")
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid user id"})
		}
		var req domain.UpdateUserRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}
		req.UserID = int64(id)
		user, err := svc.UpdateUser(c.Context(), &req)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(user)
	})

	users.Get("/:id/preferences", func(c *fiber.Ctx) error {
		id, err := c.ParamsInt("id")
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid user id"})
		}
		pref, err := svc.GetPreferences(c.Context(), int64(id))
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to get preferences"})
		}
		return c.JSON(pref)
	})

	users.Put("/:id/preferences", func(c *fiber.Ctx) error {
		id, err := c.ParamsInt("id")
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid user id"})
		}
		var req domain.UserPreferences
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}
		req.UserID = int64(id)
		pref, err := svc.UpdatePreferences(c.Context(), &req)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(pref)
	})

	users.Get("/:id/limits", func(c *fiber.Ctx) error {
		id, err := c.ParamsInt("id")
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid user id"})
		}
		limits, err := svc.GetLimits(c.Context(), int64(id))
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to get limits"})
		}
		return c.JSON(limits)
	})

	users.Put("/:id/limits", func(c *fiber.Ctx) error {
		id, err := c.ParamsInt("id")
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid user id"})
		}
		var req domain.SetLimitsRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}
		req.UserID = int64(id)
		limits, err := svc.SetLimits(c.Context(), &req)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(limits)
	})
}
