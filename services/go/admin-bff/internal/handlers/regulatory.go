package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/opus-casino/admin-bff/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func RegisterRegulatoryRoutes(router fiber.Router, db *gorm.DB, log *zap.Logger) {
	reg := router.Group("/regulatory")

	// --- Regulatory Reports ---
	reg.Get("/reports", func(c *fiber.Ctx) error {
		status := c.Query("status", "")
		jurisdiction := c.Query("jurisdiction", "")
		page := c.QueryInt("page", 1)
		ps := c.QueryInt("page_size", 50)
		if page < 1 { page = 1 }
		if ps < 1 || ps > 200 { ps = 50 }
		q := db.Model(&models.RegulatoryReport{})
		if status != "" { q = q.Where("status = ?", status) }
		if jurisdiction != "" { q = q.Where("jurisdiction = ?", jurisdiction) }
		var total int64
		q.Count(&total)
		var items []models.RegulatoryReport
		q.Order("created_at DESC").Limit(ps).Offset((page - 1) * ps).Find(&items)
		return c.JSON(fiber.Map{
			"data": items,
			"meta": fiber.Map{"total": total, "page": page, "page_size": ps},
		})
	})

	reg.Post("/reports", func(c *fiber.Ctx) error {
		var req struct {
			Jurisdiction string    `json:"jurisdiction"`
			ReportType string    `json:"report_type"`
			PeriodStart time.Time `json:"period_start"`
			PeriodEnd   time.Time `json:"period_end"`
			Notes       string    `json:"notes"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		r := models.RegulatoryReport{
			Jurisdiction: req.Jurisdiction,
			ReportType:   req.ReportType,
			PeriodStart:  req.PeriodStart,
			PeriodEnd:    req.PeriodEnd,
			Status:       "draft",
			Notes:        &req.Notes,
			CreatedAt:    time.Now(),
		}
		if err := db.Create(&r).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.Status(201).JSON(fiber.Map{"data": r})
	})

	reg.Get("/reports/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")
		var r models.RegulatoryReport
		if err := db.First(&r, "id = ?", id).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		return c.JSON(fiber.Map{"data": r})
	})

	reg.Put("/reports/:id/status", func(c *fiber.Ctx) error {
		id := c.Params("id")
		var req struct {
			Status       string `json:"status"`
			RegulatorRef string `json:"regulator_ref"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var r models.RegulatoryReport
		if err := db.First(&r, "id = ?", id).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		r.Status = req.Status
		now := time.Now()
		if req.Status == "submitted" {
			r.SubmittedAt = &now
		}
		if req.RegulatorRef != "" {
			r.RegulatorRef = &req.RegulatorRef
		}
		if err := db.Save(&r).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": r})
	})

	// --- SAR Reports ---
	reg.Get("/sar", func(c *fiber.Ctx) error {
		status := c.Query("status", "")
		page := c.QueryInt("page", 1)
		ps := c.QueryInt("page_size", 50)
		if page < 1 { page = 1 }
		if ps < 1 || ps > 200 { ps = 50 }
		q := db.Model(&models.SARReport{})
		if status != "" { q = q.Where("status = ?", status) }
		var total int64
		q.Count(&total)
		var items []models.SARReport
		q.Order("created_at DESC").Limit(ps).Offset((page - 1) * ps).Find(&items)
		return c.JSON(fiber.Map{
			"data": items,
			"meta": fiber.Map{"total": total, "page": page, "page_size": ps},
		})
	})

	reg.Post("/sar", func(c *fiber.Ctx) error {
		var req struct {
			Jurisdiction   string  `json:"jurisdiction"`
			PlayerID       int64   `json:"player_id"`
			TriggerType    string  `json:"trigger_type"`
			TriggerAlertID *string `json:"trigger_alert_id"`
			AmountInvolved string  `json:"amount_involved"`
			Currency       string  `json:"currency"`
			Description    string  `json:"description"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		sar := models.SARReport{
			Jurisdiction: req.Jurisdiction,
			PlayerID:     req.PlayerID,
			TriggerType:  req.TriggerType,
			Status:       "draft",
			Description:  req.Description,
			CreatedAt:    time.Now(),
		}
		if req.TriggerAlertID != nil {
			sar.TriggerAlertID = req.TriggerAlertID
		}
		if req.AmountInvolved != "" {
			sar.AmountInvolved = &req.AmountInvolved
		}
		if req.Currency != "" {
			sar.Currency = &req.Currency
		}
		if err := db.Create(&sar).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.Status(201).JSON(fiber.Map{"data": sar})
	})

	reg.Put("/sar/:id/status", func(c *fiber.Ctx) error {
		id := c.Params("id")
		var req struct {
			Status string `json:"status"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var sar models.SARReport
		if err := db.First(&sar, "id = ?", id).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		sar.Status = req.Status
		now := time.Now()
		if req.Status == "submitted" {
			sar.SubmittedAt = &now
		}
		if err := db.Save(&sar).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": sar})
	})

	// --- Player Complaints ---
	reg.Get("/complaints", func(c *fiber.Ctx) error {
		status := c.Query("status", "")
		page := c.QueryInt("page", 1)
		ps := c.QueryInt("page_size", 50)
		if page < 1 { page = 1 }
		if ps < 1 || ps > 200 { ps = 50 }
		q := db.Model(&models.PlayerComplaint{})
		if status != "" { q = q.Where("status = ?", status) }
		var total int64
		q.Count(&total)
		var items []models.PlayerComplaint
		q.Order("created_at DESC").Limit(ps).Offset((page - 1) * ps).Find(&items)
		return c.JSON(fiber.Map{
			"data": items,
			"meta": fiber.Map{"total": total, "page": page, "page_size": ps},
		})
	})

	reg.Post("/complaints", func(c *fiber.Ctx) error {
		var req struct {
			PlayerID    int64   `json:"player_id"`
			TicketID    *string `json:"ticket_id"`
			Category    string  `json:"category"`
			Description string  `json:"description"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		compl := models.PlayerComplaint{
			PlayerID:    req.PlayerID,
			Category:    req.Category,
			Description: req.Description,
			Status:      "open",
			CreatedAt:   time.Now(),
		}
		if req.TicketID != nil {
			compl.TicketID = req.TicketID
		}
		if err := db.Create(&compl).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.Status(201).JSON(fiber.Map{"data": compl})
	})

	reg.Put("/complaints/:id/status", func(c *fiber.Ctx) error {
		id := c.Params("id")
		var req struct {
			Status     string `json:"status"`
			Resolution string `json:"resolution"`
			ADRRef     string `json:"adr_ref"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var compl models.PlayerComplaint
		if err := db.First(&compl, "id = ?", id).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		compl.Status = req.Status
		if req.Resolution != "" {
			compl.Resolution = &req.Resolution
		}
		if req.ADRRef != "" {
			compl.ADRRef = &req.ADRRef
		}
		if req.Status == "resolved" || req.Status == "escalated_to_adr" {
			now := time.Now()
			compl.ResolvedAt = &now
		}
		if err := db.Save(&compl).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": compl})
	})

	// --- Tax Configuration ---
	reg.Get("/tax-config", func(c *fiber.Ctx) error {
		var items []models.TaxConfig
		if err := db.Find(&items).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": items})
	})

	reg.Put("/tax-config", func(c *fiber.Ctx) error {
		var req struct {
			ID           string  `json:"id"`
			Jurisdiction string  `json:"jurisdiction"`
			TaxType      string  `json:"tax_type"`
			TaxBase      string  `json:"tax_base"`
			Rate         string  `json:"rate"`
			Currency     string  `json:"currency"`
			EffectiveFrom string `json:"effective_from"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var tc models.TaxConfig
		if req.ID != "" {
			if err := db.First(&tc, "id = ?", req.ID).Error; err == nil {
				if req.Jurisdiction != "" { tc.Jurisdiction = req.Jurisdiction }
				if req.TaxType != "" { tc.TaxType = req.TaxType }
				if req.TaxBase != "" { tc.TaxBase = req.TaxBase }
				if req.Rate != "" { tc.Rate = req.Rate }
				if req.Currency != "" { tc.Currency = req.Currency }
				if err := db.Save(&tc).Error; err != nil {
					return c.Status(500).JSON(fiber.Map{"error": "database error"})
				}
				return c.JSON(fiber.Map{"data": tc})
			}
		}
		tc = models.TaxConfig{
			Jurisdiction: req.Jurisdiction,
			TaxType:      req.TaxType,
			TaxBase:      req.TaxBase,
			Rate:         req.Rate,
			Currency:     req.Currency,
			CreatedAt:    time.Now(),
		}
		if req.EffectiveFrom != "" {
			if t, err := time.Parse("2006-01-02", req.EffectiveFrom); err == nil {
				tc.EffectiveFrom = t
			}
		}
		if err := db.Create(&tc).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.Status(201).JSON(fiber.Map{"data": tc})
	})

	// --- Player Funds Reconciliation ---
	reg.Get("/player-funds", func(c *fiber.Ctx) error {
		var totalDeposits, totalWithdrawals float64
		db.Raw(`SELECT COALESCE(SUM(amount::numeric), 0) FROM deposits WHERE status = 'completed'`).Scan(&totalDeposits)
		db.Raw(`SELECT COALESCE(SUM(amount::numeric), 0) FROM withdrawals WHERE status = 'completed'`).Scan(&totalWithdrawals)
		playerBalances := totalDeposits - totalWithdrawals

		var segregatedFunds float64
		db.Raw(`SELECT COALESCE(SUM(balance::numeric), 0) FROM crypto_wallets`).Scan(&segregatedFunds)

		liabilities := playerBalances
		var ratio float64 = 1.0
		if liabilities > 0 {
			ratio = segregatedFunds / liabilities
		}

		return c.JSON(fiber.Map{
			"data": fiber.Map{
				"total_player_balances": formatMoney(playerBalances),
				"funds_in_segregated":   formatMoney(segregatedFunds),
				"segregation_ratio":     ratio,
				"liabilities_total":   formatMoney(liabilities),
			},
		})
	})
}
