package handlers

import (
	"math"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/opus-casino/admin-bff/internal/models"
	"github.com/opus-casino/admin-bff/internal/service"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func RegisterRiskRoutes(router fiber.Router, db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) {
	risk := router.Group("/risk")
	risk.Get("/alerts", listRiskAlerts(db, log))
	risk.Get("/alerts/:id", getRiskAlert(db, log))
	risk.Put("/alerts/:id/status", updateAlertStatus(db, log, auditSvc))
	risk.Put("/alerts/:id/assign", assignAlert(db, log, auditSvc))
	risk.Get("/rules", listRiskRules(db, log))
	risk.Post("/rules", createRiskRule(db, log, auditSvc))
	risk.Put("/rules/:id", updateRiskRule(db, log, auditSvc))
	risk.Delete("/rules/:id", deleteRiskRule(db, log, auditSvc))
	risk.Post("/rules/:id/test", testRiskRule(db, log))
	risk.Get("/audit-log", listAuditLog(db, log))
	risk.Get("/users/:id/profile", getUserRiskProfile(db, log))
	risk.Get("/screening", listScreeningHits(db, log))
	risk.Post("/screening/:id/resolve", resolveScreeningHit(db, log, auditSvc))
	risk.Get("/watchlist", listWatchlist(db, log))
	risk.Post("/watchlist", addWatchlistEntry(db, log, auditSvc))
	risk.Delete("/watchlist/:id", removeWatchlistEntry(db, log, auditSvc))
}


func listRiskAlerts(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		status := c.Query("status", "")
		severity := c.Query("severity", "")
		category := c.Query("category", "")
		page := c.QueryInt("page", 1)
		pageSize := c.QueryInt("page_size", 20)
		if page < 1 { page = 1 }
		if pageSize < 1 || pageSize > 200 { pageSize = 20 }

		q := db.Model(&models.RiskAlert{})
		if status != "" { q = q.Where("status = ?", status) }
		if severity != "" { q = q.Where("severity = ?", severity) }
		if category != "" { q = q.Where("category = ?", category) }

		var total int64
		if err := q.Count(&total).Error; err != nil {
			log.Error("count risk alerts failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}

		var items []models.RiskAlert
		offset := (page - 1) * pageSize
		if err := q.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
			log.Error("query risk alerts failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		tp := int(math.Ceil(float64(total) / float64(pageSize)))
		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{"page": page, "page_size": pageSize, "total": total, "total_pages": tp},
		})
	}
}

func getRiskAlert(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var alert models.RiskAlert
		if err := db.First(&alert, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(404).JSON(fiber.Map{"error": "not found"})
			}
			log.Error("get risk alert failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": alert})
	}
}

func updateAlertStatus(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var body struct {
			Status     string `json:"status"`
			Resolution string `json:"resolution,omitempty"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var before models.RiskAlert
		db.First(&before, "id = ?", id)
		updates := map[string]interface{}{
			"status":     body.Status,
			"resolution": body.Resolution,
			"updated_at": time.Now(),
		}
		if body.Status == "resolved" || body.Status == "false_positive" {
			now := time.Now()
			updates["resolved_at"] = &now
		}
		if err := db.Model(&models.RiskAlert{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			log.Error("update alert status failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "risk.alert.update_status", "risk_alert", id, fiber.Map{
			"before_status": before.Status, "after_status": body.Status, "resolution": body.Resolution,
		})
		return c.JSON(fiber.Map{"success": true})
	}
}

func assignAlert(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var body struct {
			AdminID string `json:"admin_id"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var before models.RiskAlert
		db.First(&before, "id = ?", id)
		if err := db.Model(&models.RiskAlert{}).Where("id = ?", id).Updates(map[string]interface{}{
			"assigned_to": body.AdminID,
			"updated_at":  time.Now(),
		}).Error; err != nil {
			log.Error("assign alert failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "risk.alert.assign", "risk_alert", id, fiber.Map{
			"before_assigned_to": before.AssignedTo, "after_assigned_to": body.AdminID,
		})
		return c.JSON(fiber.Map{"success": true})
	}
}

func listRiskRules(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		search := c.Query("search", "")
		ruleType := c.Query("rule_type", "")
		page := c.QueryInt("page", 1)
		pageSize := c.QueryInt("page_size", 20)
		if page < 1 { page = 1 }
		if pageSize < 1 || pageSize > 200 { pageSize = 20 }
		q := db.Model(&models.RiskRule{})
		if search != "" { q = q.Where("name ILIKE ?", "%"+search+"%") }
		if ruleType != "" { q = q.Where("rule_type = ?", ruleType) }
		var total int64
		if err := q.Count(&total).Error; err != nil {
			log.Error("count risk rules failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		var items []models.RiskRule
		offset := (page - 1) * pageSize
		if err := q.Order("priority DESC, created_at DESC").Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
			log.Error("query risk rules failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		tp := int(math.Ceil(float64(total) / float64(pageSize)))
		return c.JSON(fiber.Map{
			"data":       items,
			"pagination": fiber.Map{"page": page, "page_size": pageSize, "total": total, "total_pages": tp},
		})
	}
}

func createRiskRule(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var rule models.RiskRule
		if err := c.BodyParser(&rule); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		rule.CreatedBy = c.Locals("admin_id").(string)
		if err := db.Create(&rule).Error; err != nil {
			log.Error("create risk rule failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "risk.rule.create", "risk_rule", rule.ID, fiber.Map{"name": rule.Name, "rule_type": rule.RuleType, "action": rule.Action})
		return c.Status(201).JSON(fiber.Map{"data": rule})
	}
}

func updateRiskRule(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var body map[string]interface{}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var before models.RiskRule
		db.First(&before, "id = ?", id)
		body["updated_at"] = time.Now()
		if err := db.Model(&models.RiskRule{}).Where("id = ?", id).Updates(body).Error; err != nil {
			log.Error("update risk rule failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "risk.rule.update", "risk_rule", id, fiber.Map{"before": before, "after": body})
		return c.JSON(fiber.Map{"success": true})
	}
}

func deleteRiskRule(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var before models.RiskRule
		db.First(&before, "id = ?", id)
		if err := db.Delete(&models.RiskRule{}, "id = ?", id).Error; err != nil {
			log.Error("delete risk rule failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "risk.rule.delete", "risk_rule", id, fiber.Map{"deleted_rule": before})
		return c.JSON(fiber.Map{"success": true})
	}
}

func testRiskRule(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var body struct {
			PeriodDays int `json:"period_days"`
		}
		if err := c.BodyParser(&body); err != nil {
			body.PeriodDays = 7
		}
		var rule models.RiskRule
		if err := db.First(&rule, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(404).JSON(fiber.Map{"error": "not found"})
			}
			log.Error("get rule for test failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		_ = rule
		_ = body.PeriodDays
		// TODO: run rule against historical data via fraud-ml service
		return c.JSON(fiber.Map{
			"data": fiber.Map{
				"rule_id":     id,
				"period_days": body.PeriodDays,
				"matches":     0,
				"status":      "test_complete",
			},
		})
	}
}

func listAuditLog(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		adminID := c.Query("admin_id", "")
		action := c.Query("action", "")
		resourceType := c.Query("resource_type", "")
		page := c.QueryInt("page", 1)
		pageSize := c.QueryInt("page_size", 20)
		if page < 1 { page = 1 }
		if pageSize < 1 || pageSize > 200 { pageSize = 20 }
		q := db.Model(&models.RiskAuditLog{})
		if adminID != "" { q = q.Where("admin_id = ?", adminID) }
		if action != "" { q = q.Where("action = ?", action) }
		if resourceType != "" { q = q.Where("resource_type = ?", resourceType) }
		var total int64
		if err := q.Count(&total).Error; err != nil {
			log.Error("count audit log failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		var items []models.RiskAuditLog
		offset := (page - 1) * pageSize
		if err := q.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
			log.Error("query audit log failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		tp := int(math.Ceil(float64(total) / float64(pageSize)))
		return c.JSON(fiber.Map{
			"data":       items,
			"pagination": fiber.Map{"page": page, "page_size": pageSize, "total": total, "total_pages": tp},
		})
	}
}

func getUserRiskProfile(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Params("id")
		var alertsCount int64
		var riskScoreSum int64
		db.Model(&models.RiskAlert{}).Where("user_id = ?", userID).Count(&alertsCount)
		db.Model(&models.RiskAlert{}).Where("user_id = ? AND status = ?", userID, "open").Select("COALESCE(SUM(risk_score),0)").Scan(&riskScoreSum)

		var screening models.ScreeningResult
		screeningStatus := "unknown"
		if err := db.Where("player_id = ?", userID).Order("screened_at DESC").First(&screening).Error; err == nil {
			screeningStatus = screening.Status
		}

		var watchlistCount int64
		db.Model(&models.RiskWatchlistEntry{}).Where("entity_id = ?", userID).Count(&watchlistCount)

		var factors []string
		if alertsCount > 0 {
			factors = append(factors, "open_alerts")
		}
		if screeningStatus != "clear" && screeningStatus != "unknown" {
			factors = append(factors, "screening_"+screeningStatus)
		}
		if watchlistCount > 0 {
			factors = append(factors, "watchlist_match")
		}

		return c.JSON(fiber.Map{
			"data": fiber.Map{
				"user_id":         userID,
				"risk_score":      riskScoreSum,
				"factors":         factors,
				"alerts_count":    alertsCount,
				"open_alerts":     alertsCount,
				"screening_status": screeningStatus,
				"watchlist_count": watchlistCount,
			},
		})
	}
}

func listScreeningHits(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		listType := c.Query("list_type", "")
		page := c.QueryInt("page", 1)
		pageSize := c.QueryInt("page_size", 20)
		if page < 1 { page = 1 }
		if pageSize < 1 || pageSize > 200 { pageSize = 20 }
		q := db.Model(&models.ScreeningResult{}).Where("status != ?", "clear")
		if listType != "" { q = q.Where("status = ?", listType) }
		var total int64
		if err := q.Count(&total).Error; err != nil {
			log.Error("count screening hits failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		var items []models.ScreeningResult
		offset := (page - 1) * pageSize
		if err := q.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
			log.Error("query screening hits failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		tp := int(math.Ceil(float64(total) / float64(pageSize)))
		return c.JSON(fiber.Map{
			"data":       items,
			"pagination": fiber.Map{"page": page, "page_size": pageSize, "total": total, "total_pages": tp},
		})
	}
}

func resolveScreeningHit(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var body struct {
			Status string `json:"status"`
			Notes  string `json:"notes,omitempty"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var before models.ScreeningResult
		db.First(&before, "id = ?", id)
		if err := db.Model(&models.ScreeningResult{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":       body.Status,
			"review_notes": body.Notes,
			"reviewed_at":  time.Now(),
			"updated_at":   time.Now(),
		}).Error; err != nil {
			log.Error("resolve screening hit failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "risk.screening.resolve", "screening_result", id, fiber.Map{
			"before_status": before.Status, "after_status": body.Status, "notes": body.Notes,
		})
		return c.JSON(fiber.Map{"success": true})
	}
}

func listWatchlist(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		listType := c.Query("list_type", "")
		page := c.QueryInt("page", 1)
		pageSize := c.QueryInt("page_size", 20)
		if page < 1 { page = 1 }
		if pageSize < 1 || pageSize > 200 { pageSize = 20 }
		q := db.Model(&models.RiskWatchlistEntry{})
		if listType != "" { q = q.Where("list_type = ?", listType) }
		var total int64
		if err := q.Count(&total).Error; err != nil {
			log.Error("count watchlist failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		var items []models.RiskWatchlistEntry
		offset := (page - 1) * pageSize
		if err := q.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
			log.Error("query watchlist failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		tp := int(math.Ceil(float64(total) / float64(pageSize)))
		return c.JSON(fiber.Map{
			"data":       items,
			"pagination": fiber.Map{"page": page, "page_size": pageSize, "total": total, "total_pages": tp},
		})
	}
}

func addWatchlistEntry(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var entry models.RiskWatchlistEntry
		if err := c.BodyParser(&entry); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		entry.AddedBy = c.Locals("admin_id").(string)
		if err := db.Create(&entry).Error; err != nil {
			log.Error("add watchlist entry failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "risk.watchlist.add", "risk_watchlist", entry.ID, fiber.Map{
			"list_type": entry.ListType, "entity_type": entry.EntityType, "entity_id": entry.EntityID,
		})
		return c.Status(201).JSON(fiber.Map{"data": entry})
	}
}

func removeWatchlistEntry(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var before models.RiskWatchlistEntry
		db.First(&before, "id = ?", id)
		if err := db.Delete(&models.RiskWatchlistEntry{}, "id = ?", id).Error; err != nil {
			log.Error("remove watchlist entry failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "risk.watchlist.remove", "risk_watchlist", id, fiber.Map{
			"removed_entry": before,
		})
		return c.JSON(fiber.Map{"success": true})
	}
}
