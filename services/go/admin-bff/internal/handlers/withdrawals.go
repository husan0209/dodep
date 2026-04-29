package handlers

import (
	"math"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/opus-casino/admin-bff/internal/models"
	"gorm.io/gorm"
)

func RegisterWithdrawalRoutes(router fiber.Router, db *gorm.DB) {
	withdrawals := router.Group("/withdrawals")

	// List withdrawals (queue) — default sort by wait time desc
	withdrawals.Get("/", func(c *fiber.Ctx) error {
		status := c.Query("status", "")
		playerID := c.Query("player_id", "")
		page := c.QueryInt("page", 1)
		ps := c.QueryInt("page_size", 50)
		if page < 1 { page = 1 }
		if ps < 1 || ps > 200 { ps = 50 }

		q := db.Model(&models.Withdrawal{})
		if status != "" { q = q.Where("status = ?", status) }
		if playerID != "" { q = q.Where("player_id = ?", playerID) }

		var total int64
		q.Count(&total)
		var items []models.Withdrawal
		q.Order("created_at ASC").Limit(ps).Offset((page - 1) * ps).Find(&items)

		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{
				"page": page, "page_size": ps,
				"total": total, "total_pages": int(math.Ceil(float64(total)/float64(ps))),
			},
		})
	})

	// Get withdrawal detail with checklist
	withdrawals.Get("/:id", func(c *fiber.Ctx) error {
		var w models.Withdrawal
		if err := db.Where("id = ?", c.Params("id")).First(&w).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(404).JSON(fiber.Map{"error": "not found"})
			}
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": w})
	})

	// Approve withdrawal
	withdrawals.Post("/:id/approve", func(c *fiber.Ctx) error {
		adminID := c.Locals("admin_id")
		adminName := c.Locals("admin_name")
		now := time.Now()
		db.Model(&models.Withdrawal{}).Where("id = ?", c.Params("id")).Updates(map[string]any{
			"status":           "approved",
			"approved_by":      adminID,
			"approved_by_name": adminName,
			"approved_at":      now,
			"updated_at":       now,
		})
		return c.JSON(fiber.Map{"success": true})
	})

	// Decline withdrawal
	withdrawals.Post("/:id/decline", func(c *fiber.Ctx) error {
		var req struct {
			Reason string `json:"reason"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if req.Reason == "" {
			return c.Status(400).JSON(fiber.Map{"error": "reason is required"})
		}
		adminID := c.Locals("admin_id")
		adminName := c.Locals("admin_name")
		now := time.Now()
		db.Model(&models.Withdrawal{}).Where("id = ?", c.Params("id")).Updates(map[string]any{
			"status":           "rejected",
			"rejected_by":      adminID,
			"rejected_by_name": adminName,
			"rejected_at":      now,
			"reject_reason":    req.Reason,
			"updated_at":       now,
		})
		return c.JSON(fiber.Map{"success": true})
	})

	// Hold for review
	withdrawals.Post("/:id/hold", func(c *fiber.Ctx) error {
		var req struct {
			Reason string `json:"reason"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		now := time.Now()
		db.Model(&models.Withdrawal{}).Where("id = ?", c.Params("id")).Updates(map[string]any{
			"status":      "held",
			"hold_reason": req.Reason,
			"updated_at":  now,
		})
		return c.JSON(fiber.Map{"success": true})
	})

	// Auto-approve endpoint (for internal trigger)
	withdrawals.Post("/:id/auto-approve", func(c *fiber.Ctx) error {
		var w models.Withdrawal
		if err := db.Where("id = ?", c.Params("id")).First(&w).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(404).JSON(fiber.Map{"error": "not found"})
			}
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		// Check auto-approve conditions
		if w.RiskScore < 20 && w.KycStatus == "verified" && w.WagerStatus == "completed" && w.AmlStatus == "clear" {
			now := time.Now()
			db.Model(&models.Withdrawal{}).Where("id = ?", c.Params("id")).Updates(map[string]any{
				"status":           "approved",
				"approved_by":      "auto",
				"approved_by_name": "auto",
				"approved_at":      now,
				"updated_at":       now,
			})
			return c.JSON(fiber.Map{"success": true, "auto_approved": true})
		}
		return c.Status(422).JSON(fiber.Map{"error": "auto-approve conditions not met"})
	})
}
