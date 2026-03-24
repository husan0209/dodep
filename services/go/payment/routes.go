package main

import (
	"github.com/gofiber/fiber/v2"

	"github.com/opus-casino/payment/internal/domain"
	"github.com/opus-casino/payment/internal/service"
)

func setupRoutes(app *fiber.App, svc *service.PaymentService) {
	api := app.Group("/api/v1")

	payments := api.Group("/payments")

	payments.Get("/methods", func(c *fiber.Ctx) error {
		userID := c.Locals("user_id").(int64)
		methods, err := svc.GetPaymentMethods(c.Context(), userID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to get payment methods"})
		}
		return c.JSON(methods)
	})

	payments.Post("/deposit", func(c *fiber.Ctx) error {
		var req domain.CreateDepositRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}
		req.UserID = c.Locals("user_id").(int64)
		deposit, err := svc.CreateDeposit(c.Context(), &req)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(201).JSON(deposit)
	})

	payments.Get("/deposits/:id", func(c *fiber.Ctx) error {
		userID := c.Locals("user_id").(int64)
		depositID := c.Params("id")
		deposit, err := svc.GetDeposit(c.Context(), userID, depositID)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "deposit not found"})
		}
		return c.JSON(deposit)
	})

	payments.Get("/deposits", func(c *fiber.Ctx) error {
		userID := c.Locals("user_id").(int64)
		limit := c.QueryInt("limit", 20)
		offset := c.QueryInt("offset", 0)
		deposits, total, err := svc.ListDeposits(c.Context(), userID, limit, offset)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to list deposits"})
		}
		return c.JSON(fiber.Map{"deposits": deposits, "total": total})
	})

	payments.Post("/withdraw", func(c *fiber.Ctx) error {
		var req domain.RequestWithdrawalRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}
		req.UserID = c.Locals("user_id").(int64)
		withdrawal, err := svc.RequestWithdrawal(c.Context(), &req)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(201).JSON(withdrawal)
	})

	payments.Get("/withdrawals/:id", func(c *fiber.Ctx) error {
		userID := c.Locals("user_id").(int64)
		withdrawalID := c.Params("id")
		w, err := svc.GetWithdrawal(c.Context(), userID, withdrawalID)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "withdrawal not found"})
		}
		return c.JSON(w)
	})

	payments.Get("/withdrawals", func(c *fiber.Ctx) error {
		userID := c.Locals("user_id").(int64)
		limit := c.QueryInt("limit", 20)
		offset := c.QueryInt("offset", 0)
		withdrawals, total, err := svc.ListWithdrawals(c.Context(), userID, limit, offset)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to list withdrawals"})
		}
		return c.JSON(fiber.Map{"withdrawals": withdrawals, "total": total})
	})

	payments.Post("/withdrawals/:id/cancel", func(c *fiber.Ctx) error {
		userID := c.Locals("user_id").(int64)
		withdrawalID := c.Params("id")
		if err := svc.CancelWithdrawal(c.Context(), userID, withdrawalID); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"success": true})
	})

	// Webhook endpoint (no auth, provider calls this)
	payments.Post("/webhook/:provider", func(c *fiber.Ctx) error {
		provider := c.Params("provider")
		var event domain.WebhookEvent
		if err := c.BodyParser(&event); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid webhook body"})
		}
		event.Provider = provider
		if err := svc.HandleWebhook(c.Context(), &event); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "webhook processing failed"})
		}
		return c.JSON(fiber.Map{"received": true})
	})
}
