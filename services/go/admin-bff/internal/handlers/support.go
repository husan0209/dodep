package handlers

import (
	"math"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/opus-casino/admin-bff/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func RegisterSupportRoutes(router fiber.Router, db *gorm.DB, log *zap.Logger) {
	tickets := router.Group("/tickets")

	// List tickets
	tickets.Get("/", func(c *fiber.Ctx) error {
		status := c.Query("status", "")
		priority := c.Query("priority", "")
		category := c.Query("category", "")
		assignedTo := c.Query("assigned_to", "")
		page := c.QueryInt("page", 1)
		ps := c.QueryInt("page_size", 50)
		if page < 1 { page = 1 }
		if ps < 1 || ps > 200 { ps = 50 }

		q := db.Model(&models.SupportTicket{})
		if status != "" { q = q.Where("status = ?", status) }
		if priority != "" { q = q.Where("priority = ?", priority) }
		if category != "" { q = q.Where("category = ?", category) }
		if assignedTo != "" { q = q.Where("assigned_to = ?", assignedTo) }

		var total int64
		q.Count(&total)
		var items []models.SupportTicket
		q.Order("created_at DESC").Limit(ps).Offset((page - 1) * ps).Find(&items)

		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{"page": page, "page_size": ps, "total": total, "total_pages": int(math.Ceil(float64(total)/float64(ps)))},
		})
	})

	// Create ticket
	tickets.Post("/", func(c *fiber.Ctx) error {
		var req models.SupportTicket
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		req.Status = "open"
		req.CreatedAt = time.Now()
		req.UpdatedAt = time.Now()
		if err := db.Create(&req).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": req})
	})

	// Get ticket detail
	tickets.Get("/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")
		var t models.SupportTicket
		if err := db.Where("id = ?", id).First(&t).Error; err != nil {
			if err == gorm.ErrRecordNotFound { return c.Status(404).JSON(fiber.Map{"error": "not found"}) }
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		var messages []models.SupportMessage
		db.Where("ticket_id = ?", id).Order("created_at ASC").Find(&messages)
		var links []models.TicketLink
		db.Where("ticket_id = ?", id).Find(&links)
		return c.JSON(fiber.Map{"data": fiber.Map{
			"ticket": t, "messages": messages, "links": links,
		}})
	})

	// Update ticket status / priority / assign
	tickets.Put("/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")
		var req struct {
			Status       string `json:"status"`
			Priority     string `json:"priority"`
			AssignedTo   string `json:"assigned_to"`
			AssignedToName string `json:"assigned_to_name"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		updates := map[string]any{}
		if req.Status != "" { updates["status"] = req.Status }
		if req.Priority != "" { updates["priority"] = req.Priority }
		if req.AssignedTo != "" { updates["assigned_to"] = req.AssignedTo }
		if req.AssignedToName != "" { updates["assigned_to_name"] = req.AssignedToName }
		if len(updates) == 0 { return c.JSON(fiber.Map{"success": true}) }
		if err := db.Model(&models.SupportTicket{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"success": true})
	})

	// Get messages
	tickets.Get("/:id/messages", func(c *fiber.Ctx) error {
		id := c.Params("id")
		var messages []models.SupportMessage
		db.Where("ticket_id = ?", id).Order("created_at ASC").Find(&messages)
		return c.JSON(fiber.Map{"data": messages})
	})

	// Add message
	tickets.Post("/:id/messages", func(c *fiber.Ctx) error {
		id := c.Params("id")
		var req models.SupportMessage
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		req.TicketID = id
		req.CreatedAt = time.Now()
		if err := db.Create(&req).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		// Update ticket preview and count
		db.Model(&models.SupportTicket{}).Where("id = ?", id).Updates(map[string]any{
			"last_message_preview": req.Body,
			"last_message_at":    time.Now(),
			"message_count":      gorm.Expr("message_count + 1"),
		})
		return c.JSON(fiber.Map{"data": req})
	})

	// Link entity
	tickets.Post("/:id/links", func(c *fiber.Ctx) error {
		id := c.Params("id")
		var req models.TicketLink
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		req.TicketID = id
		req.CreatedAt = time.Now()
		if err := db.Create(&req).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": req})
	})

	// SLA config
	pm := router.Group("/support")
	pm.Get("/sla-config", func(c *fiber.Ctx) error {
		var items []models.SLAConfig
		if err := db.Order("category ASC").Find(&items).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": items})
	})

	pm.Put("/sla-config/:id", func(c *fiber.Ctx) error {
		var req models.SLAConfig
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		db.Model(&models.SLAConfig{}).Where("id = ?", c.Params("id")).Updates(&req)
		return c.JSON(fiber.Map{"success": true})
	})

	// Stats
	pm.Get("/stats", func(c *fiber.Ctx) error {
		var openCount, pendingCount, resolvedToday, slaBreaches int64
		db.Model(&models.SupportTicket{}).Where("status = ?", "open").Count(&openCount)
		db.Model(&models.SupportTicket{}).Where("status = ?", "pending_internal").Count(&pendingCount)
		today := time.Now().Format("2006-01-02")
		db.Model(&models.SupportTicket{}).Where("status = ? AND DATE(resolved_at) = ?", "resolved", today).Count(&resolvedToday)
		db.Model(&models.SupportTicket{}).Where("is_sla_breach = ?", true).Count(&slaBreaches)
		return c.JSON(fiber.Map{"data": fiber.Map{
			"open": openCount, "pending": pendingCount,
			"resolved_today": resolvedToday, "sla_breaches": slaBreaches,
		}})
	})

	// Team dashboard
	pm.Get("/team-dashboard", func(c *fiber.Ctx) error {
		var agents []models.AgentWorkload
		db.Order("open_tickets DESC").Find(&agents)
		var categories []fiber.Map
		db.Raw("SELECT category, COUNT(*) as cnt FROM support_tickets WHERE status IN ('open','pending_internal','pending_player') GROUP BY category").Scan(&categories)
		return c.JSON(fiber.Map{"data": fiber.Map{
			"agents": agents, "categories": categories,
		}})
	})
}
