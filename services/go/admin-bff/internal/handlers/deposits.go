package handlers

import (
	"math"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/opus-casino/admin-bff/internal/models"
	"gorm.io/gorm"
)

func RegisterDepositRoutes(router fiber.Router, db *gorm.DB) {
	deposits := router.Group("/deposits")

	// List deposits with filters
	deposits.Get("/", func(c *fiber.Ctx) error {
		status := c.Query("status", "")
		method := c.Query("method", "")
		gateway := c.Query("gateway", "")
		playerID := c.Query("player_id", "")
		page := c.QueryInt("page", 1)
		ps := c.QueryInt("page_size", 50)
		if page < 1 { page = 1 }
		if ps < 1 || ps > 200 { ps = 50 }

		q := db.Model(&models.Deposit{})
		if status != "" { q = q.Where("status = ?", status) }
		if method != "" { q = q.Where("method = ?", method) }
		if gateway != "" { q = q.Where("gateway = ?", gateway) }
		if playerID != "" { q = q.Where("player_id = ?", playerID) }

		var total int64
		q.Count(&total)
		var items []models.Deposit
		q.Order("created_at DESC").Limit(ps).Offset((page - 1) * ps).Find(&items)

		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{
				"page": page, "page_size": ps,
				"total": total, "total_pages": int(math.Ceil(float64(total)/float64(ps))),
			},
		})
	})

	// Get deposit detail
	deposits.Get("/:id", func(c *fiber.Ctx) error {
		var d models.Deposit
		if err := db.Where("id = ?", c.Params("id")).First(&d).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(404).JSON(fiber.Map{"error": "not found"})
			}
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": d})
	})

	// Manual credit (rare case when gateway credited but callback missing)
	deposits.Post("/:id/manual-credit", func(c *fiber.Ctx) error {
		var req struct {
			Reason string `json:"reason"`
			TOTP   string `json:"totp"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if req.Reason == "" {
			return c.Status(400).JSON(fiber.Map{"error": "reason is required"})
		}

		now := time.Now()
		db.Model(&models.Deposit{}).Where("id = ?", c.Params("id")).Updates(map[string]any{
			"status":      "completed",
			"credited_at":  now,
			"notes":        gorm.Expr("COALESCE(notes, '') || '\n[Manual Credit: ' || ? || ']'", req.Reason),
			"updated_at":  now,
		})
		return c.JSON(fiber.Map{"success": true})
	})
}
