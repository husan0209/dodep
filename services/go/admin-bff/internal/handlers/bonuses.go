package handlers

import (
	"math"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/opus-casino/admin-bff/internal/models"
	"gorm.io/gorm"
)

func RegisterBonusRoutes(router fiber.Router, db *gorm.DB) {
	bonuses := router.Group("/bonuses")

	// List bonuses
	bonuses.Get("/", func(c *fiber.Ctx) error {
		status := c.Query("status", "")
		bonusType := c.Query("type", "")
		page := c.QueryInt("page", 1)
		ps := c.QueryInt("page_size", 50)
		if page < 1 { page = 1 }
		if ps < 1 || ps > 200 { ps = 50 }

		q := db.Model(&models.Bonus{})
		if status != "" { q = q.Where("status = ?", status) }
		if bonusType != "" { q = q.Where("bonus_type = ?", bonusType) }

		var total int64
		q.Count(&total)
		var items []models.Bonus
		q.Order("created_at DESC").Limit(ps).Offset((page - 1) * ps).Find(&items)

		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{
				"page": page, "page_size": ps,
				"total": total, "total_pages": int(math.Ceil(float64(total)/float64(ps))),
			},
		})
	})

	// Get bonus detail
	bonuses.Get("/:id", func(c *fiber.Ctx) error {
		var b models.Bonus
		if err := db.Where("id = ?", c.Params("id")).First(&b).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(404).JSON(fiber.Map{"error": "not found"})
			}
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": b})
	})

	// Create bonus
	bonuses.Post("/", func(c *fiber.Ctx) error {
		var req models.Bonus
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		req.Status = "draft"
		req.CreatedAt = time.Now()
		req.UpdatedAt = time.Now()
		if err := db.Create(&req).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.Status(201).JSON(fiber.Map{"data": req})
	})

	// Update bonus (only draft or paused)
	bonuses.Put("/:id", func(c *fiber.Ctx) error {
		var b models.Bonus
		if err := db.Where("id = ?", c.Params("id")).First(&b).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(404).JSON(fiber.Map{"error": "not found"})
			}
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		if b.Status != "draft" && b.Status != "paused" {
			return c.Status(409).JSON(fiber.Map{"error": "can only edit draft or paused bonuses"})
		}
		var req models.Bonus
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		db.Model(&models.Bonus{}).Where("id = ?", c.Params("id")).Updates(&req)
		return c.JSON(fiber.Map{"success": true})
	})

	// Activate bonus
	bonuses.Post("/:id/activate", func(c *fiber.Ctx) error {
		now := time.Now()
		db.Model(&models.Bonus{}).Where("id = ?", c.Params("id")).Updates(map[string]any{
			"status": "active", "updated_at": now,
		})
		return c.JSON(fiber.Map{"success": true})
	})

	// Pause bonus
	bonuses.Post("/:id/pause", func(c *fiber.Ctx) error {
		now := time.Now()
		db.Model(&models.Bonus{}).Where("id = ?", c.Params("id")).Updates(map[string]any{
			"status": "paused", "updated_at": now,
		})
		return c.JSON(fiber.Map{"success": true})
	})

	// Clone bonus
	bonuses.Post("/:id/clone", func(c *fiber.Ctx) error {
		var original models.Bonus
		if err := db.Where("id = ?", c.Params("id")).First(&original).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(404).JSON(fiber.Map{"error": "not found"})
			}
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		original.ID = ""
		original.Status = "draft"
		original.Name = original.Name + " (Copy)"
		original.TotalIssued = 0
		original.TotalCost = "0"
		original.ConversionRatePct = nil
		original.CreatedAt = time.Now()
		original.UpdatedAt = time.Now()
		db.Create(&original)
		return c.Status(201).JSON(fiber.Map{"data": original})
	})

	// Bonus stats
	bonuses.Get("/:id/stats", func(c *fiber.Ctx) error {
		var totalIssued int64
		var activeCount int64
		var completedCount int64
		var expiredCount int64
		bonusID := c.Params("id")
		db.Model(&models.PlayerBonus{}).Where("bonus_id = ?", bonusID).Count(&totalIssued)
		db.Model(&models.PlayerBonus{}).Where("bonus_id = ? AND status = ?", bonusID, "active").Count(&activeCount)
		db.Model(&models.PlayerBonus{}).Where("bonus_id = ? AND status = ?", bonusID, "completed").Count(&completedCount)
		db.Model(&models.PlayerBonus{}).Where("bonus_id = ? AND status = ?", bonusID, "expired").Count(&expiredCount)
		return c.JSON(fiber.Map{"data": fiber.Map{
			"total_issued": totalIssued,
			"active":       activeCount,
			"completed":    completedCount,
			"expired":      expiredCount,
			"voided":       int64(0),
		}})
	})

	// Player bonuses list
	bonuses.Get("/player-bonuses", func(c *fiber.Ctx) error {
		status := c.Query("status", "")
		playerID := c.Query("player_id", "")
		bonusID := c.Query("bonus_id", "")
		page := c.QueryInt("page", 1)
		ps := c.QueryInt("page_size", 50)
		if page < 1 { page = 1 }
		if ps < 1 || ps > 200 { ps = 50 }

		q := db.Model(&models.PlayerBonus{})
		if status != "" { q = q.Where("status = ?", status) }
		if playerID != "" { q = q.Where("player_id = ?", playerID) }
		if bonusID != "" { q = q.Where("bonus_id = ?", bonusID) }

		var total int64
		q.Count(&total)
		var items []models.PlayerBonus
		q.Order("issued_at DESC").Limit(ps).Offset((page - 1) * ps).Find(&items)

		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{
				"page": page, "page_size": ps,
				"total": total, "total_pages": int(math.Ceil(float64(total)/float64(ps))),
			},
		})
	})

	// Void player bonus
	bonuses.Post("/player-bonuses/:id/void", func(c *fiber.Ctx) error {
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
		now := time.Now()
		db.Model(&models.PlayerBonus{}).Where("id = ?", c.Params("id")).Updates(map[string]any{
			"status":     "voided",
			"voided_at":  now,
			"voided_by":  adminID,
			"void_reason": req.Reason,
		})
		return c.JSON(fiber.Map{"success": true})
	})

	// Wagering monitor
	bonuses.Get("/wagering-monitor", func(c *fiber.Ctx) error {
		var items []models.WageringMonitor
		db.Where("abnormally_fast = ? OR near_completion = ? OR expires_soon = ?", true, true, true).Find(&items)
		return c.JSON(fiber.Map{"data": items})
	})
}
