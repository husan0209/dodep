package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opus-casino/affiliate/internal/service"
)

// setupAdminReversalRoutes registers the manual commission reversal
// endpoints (fraud, chargeback, correction). Kept in a separate file so the
// admin route table stays readable.
func setupAdminReversalRoutes(affiliates fiber.Router, svc *service.AffiliateService) {
	// POST /admin/affiliates/:id/earnings/:earning_id/reverse
	affiliates.Post("/:id/earnings/:earning_id/reverse", func(c *fiber.Ctx) error {
		adminUserID := getAdminUserID(c)
		affiliateID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid affiliate id"})
		}
		earningID, err := uuid.Parse(c.Params("earning_id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid earning id"})
		}

		var req struct {
			Reason string `json:"reason"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}
		if req.Reason == "" {
			return c.Status(400).JSON(fiber.Map{"error": "reason is required"})
		}

		earning, err := svc.ReverseCommission(c.Context(), service.ReverseCommissionInput{
			AffiliateID: affiliateID,
			EarningID:   earningID,
			Reason:      req.Reason,
			ReversedBy:  adminUserID,
		})
		if err != nil {
			return handleDomainError(c, err)
		}

		return c.JSON(earning)
	})
}