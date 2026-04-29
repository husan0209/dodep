package handlers

import (
	"math"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/opus-casino/admin-bff/internal/models"
	"github.com/opus-casino/admin-bff/internal/service"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func RegisterCasinoRoutes(router fiber.Router, db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) {
	casino := router.Group("/casino")
	casino.Get("/games", listCasinoGames(db, log))
	casino.Post("/games/bulk-update", bulkUpdateCasinoGames(db, log, auditSvc))
	casino.Get("/games/:id", getCasinoGame(db, log))
	casino.Put("/games/:id", updateCasinoGame(db, log, auditSvc))
	casino.Put("/games/:id/rtp", updateGameRTP(db, log, auditSvc))
	casino.Get("/providers", listCasinoProviders(db, log))
	casino.Put("/providers/:id", updateCasinoProvider(db, log, auditSvc))
	casino.Get("/rtp-config", getRtpConfig(db, log))
	casino.Get("/rtp-report", getRtpReport(db, log))
	casino.Get("/sessions", listCasinoSessions(db, log))
	casino.Get("/jackpot-pools", listJackpotPools(db, log))
	casino.Post("/jackpot-pools", createJackpotPool(db, log, auditSvc))
	casino.Get("/provider-settlements", listProviderSettlements(db, log))
	casino.Post("/provider-settlements/:id/mark-paid", markSettlementPaid(db, log, auditSvc))
	casino.Get("/bets", listCasinoBets(db, log))
}

func listCasinoGames(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		category := c.Query("category", "")
		provider := c.Query("provider", "")
		search := c.Query("search", "")
		enabled := c.Query("enabled", "")
		country := c.Query("country_restriction", "")
		page := c.QueryInt("page", 1)
		pageSize := c.QueryInt("page_size", 20)
		if page < 1 {
			page = 1
		}
		if pageSize < 1 || pageSize > 200 {
			pageSize = 20
		}
		q := db.Model(&models.CasinoGame{})
		if category != "" {
			q = q.Where("category = ?", category)
		}
		if provider != "" {
			q = q.Where("provider_id = ?", provider)
		}
		if search != "" {
			q = q.Where("name ILIKE ?", "%"+search+"%")
		}
		if enabled != "" {
			q = q.Where("is_active = ?", enabled == "true")
		}
		if country != "" {
			q = q.Where("? = ANY(country_restrictions)", country)
		}
		var total int64
		if err := q.Count(&total).Error; err != nil {
			log.Error("count casino games failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		var items []models.CasinoGame
		offset := (page - 1) * pageSize
		if err := q.Order("popularity_score DESC, sort_weight DESC").Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
			log.Error("query casino games failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		tp := int(math.Ceil(float64(total) / float64(pageSize)))
		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{"page": page, "page_size": pageSize, "total": total, "total_pages": tp},
		})
	}
}

func getCasinoGame(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var game models.CasinoGame
		if err := db.First(&game, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(404).JSON(fiber.Map{"error": "not found"})
			}
			log.Error("get casino game failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": game})
	}
}

func updateCasinoGame(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var body map[string]interface{}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var before models.CasinoGame
		db.First(&before, "id = ?", id)
		body["updated_at"] = time.Now()
		if err := db.Model(&models.CasinoGame{}).Where("id = ?", id).Updates(body).Error; err != nil {
			log.Error("update casino game failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "casino.game.update", "casino_game", id, fiber.Map{"before": before, "after": body})
		return c.JSON(fiber.Map{"success": true})
	}
}

func bulkUpdateCasinoGames(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var body struct {
			IDs                 []string `json:"ids"`
			Category            *string  `json:"category,omitempty"`
			Badge               *string  `json:"badge,omitempty"`
			IsActive            *bool    `json:"is_active,omitempty"`
			CountryRestrictions []string `json:"country_restrictions,omitempty"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if len(body.IDs) == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "ids required"})
		}
		updates := map[string]interface{}{"updated_at": time.Now()}
		if body.Category != nil {
			updates["category"] = *body.Category
		}
		if body.Badge != nil {
			updates["badge"] = *body.Badge
		}
		if body.IsActive != nil {
			updates["is_active"] = *body.IsActive
		}
		if body.CountryRestrictions != nil {
			updates["country_restrictions"] = body.CountryRestrictions
		}
		if err := db.Model(&models.CasinoGame{}).Where("id IN ?", body.IDs).Updates(updates).Error; err != nil {
			log.Error("bulk update casino games failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "casino.game.bulk_update", "casino_game", "", fiber.Map{"ids": body.IDs, "updates": updates})
		return c.JSON(fiber.Map{"success": true, "updated_count": len(body.IDs)})
	}
}

func updateGameRTP(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		gameID := c.Params("id")
		var body struct {
			TargetRtp      float64  `json:"target_rtp"`
			ImpactEstimate *float64 `json:"impact_estimate,omitempty"`
			ConfirmedBy    *string  `json:"confirmed_by,omitempty"`
			Reason         string   `json:"reason"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		adminIDStr := adminIDString(c)
		if body.ConfirmedBy != nil && *body.ConfirmedBy == adminIDStr {
			return c.Status(400).JSON(fiber.Map{"error": "confirmed_by must be a different admin"})
		}
		var existing models.RtpConfig
		beforeRtp := 96.0
		if err := db.Where("game_id = ?", gameID).First(&existing).Error; err == nil {
			beforeRtp = existing.TargetRtp
		}
		now := time.Now()
		config := models.RtpConfig{
			GameID: &gameID, TargetRtp: body.TargetRtp, ImpactEstimate: body.ImpactEstimate,
			OverrideBy: &adminIDStr, OverrideAt: &now, ConfirmedBy: body.ConfirmedBy,
		}
		if body.ConfirmedBy != nil {
			config.ConfirmedAt = &now
		}
		if err := db.Save(&config).Error; err != nil {
			log.Error("update game rtp failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "casino.rtp.update", "rtp_config", config.ID, fiber.Map{
			"game_id": gameID, "before_rtp": beforeRtp, "after_rtp": body.TargetRtp,
			"reason": body.Reason, "confirmed_by": body.ConfirmedBy,
		})
		rtpAudit := models.RtpAuditLog{
			GameID: &gameID, BeforeRtp: beforeRtp, AfterRtp: body.TargetRtp,
			ImpactEstimate: body.ImpactEstimate, ChangedBy: adminIDStr,
			ConfirmedBy: body.ConfirmedBy, ConfirmedAt: config.ConfirmedAt, Reason: body.Reason,
		}
		go db.Create(&rtpAudit)
		return c.JSON(fiber.Map{"success": true, "data": config})
	}
}

func listCasinoProviders(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var items []models.CasinoProvider
		if err := db.Order("name ASC").Find(&items).Error; err != nil {
			log.Error("list providers failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": items})
	}
}

func updateCasinoProvider(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var body map[string]interface{}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var before models.CasinoProvider
		db.First(&before, "id = ?", id)
		body["updated_at"] = time.Now()
		if err := db.Model(&models.CasinoProvider{}).Where("id = ?", id).Updates(body).Error; err != nil {
			log.Error("update provider failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "casino.provider.update", "casino_provider", id, fiber.Map{"before": before, "after": body})
		return c.JSON(fiber.Map{"success": true})
	}
}

func getRtpConfig(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var items []models.RtpConfig
		if err := db.Find(&items).Error; err != nil {
			log.Error("get rtp config failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": items})
	}
}

func getRtpReport(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		provider := c.Query("provider", "")
		dateFrom := c.Query("date_from", "")
		dateTo := c.Query("date_to", "")
		type rtpRow struct {
			ProviderID   string  `json:"provider_id"`
			ProviderName string  `json:"provider_name"`
			GameCount    int64   `json:"game_count"`
			AvgRtp       float64 `json:"avg_rtp"`
			MinRtp       float64 `json:"min_rtp"`
			MaxRtp       float64 `json:"max_rtp"`
		}
		var rows []rtpRow
		var parts []string
		var args []interface{}
		parts = append(parts, "SELECT provider_id, provider_name, COUNT(*) as game_count, AVG(rtp) as avg_rtp, MIN(rtp) as min_rtp, MAX(rtp) as max_rtp FROM casino_games WHERE 1=1")
		if provider != "" {
			parts = append(parts, "AND provider_id = ?")
			args = append(args, provider)
		}
		if dateFrom != "" {
			parts = append(parts, "AND created_at >= ?")
			args = append(args, dateFrom)
		}
		if dateTo != "" {
			parts = append(parts, "AND created_at <= ?")
			args = append(args, dateTo)
		}
		sql := strings.Join(parts, " ") + " GROUP BY provider_id, provider_name ORDER BY avg_rtp DESC"
		if err := db.Raw(sql, args...).Scan(&rows).Error; err != nil {
			log.Error("rtp report failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": fiber.Map{"providers": rows}})
	}
}

func listCasinoSessions(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		gameID := c.Query("game_id", "")
		userID := c.Query("user_id", "")
		providerID := c.Query("provider_id", "")
		status := c.Query("status", "")
		page := c.QueryInt("page", 1)
		pageSize := c.QueryInt("page_size", 20)
		if page < 1 { page = 1 }
		if pageSize < 1 || pageSize > 200 { pageSize = 20 }
		q := db.Model(&models.CasinoGameSession{})
		if gameID != "" { q = q.Where("game_id = ?", gameID) }
		if userID != "" { q = q.Where("user_id = ?", userID) }
		if providerID != "" { q = q.Where("provider_id = ?", providerID) }
		if status != "" { q = q.Where("status = ?", status) }
		var total int64
		if err := q.Count(&total).Error; err != nil {
			log.Error("count casino sessions failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		var items []models.CasinoGameSession
		offset := (page - 1) * pageSize
		if err := q.Order("started_at DESC").Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
			log.Error("query casino sessions failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		tp := int(math.Ceil(float64(total) / float64(pageSize)))
		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{"page": page, "page_size": pageSize, "total": total, "total_pages": tp},
		})
	}
}

func listJackpotPools(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var items []models.JackpotPool
		if err := db.Find(&items).Error; err != nil {
			log.Error("list jackpot pools failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": items})
	}
}

func createJackpotPool(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var pool models.JackpotPool
		if err := c.BodyParser(&pool); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if err := db.Create(&pool).Error; err != nil {
			log.Error("create jackpot pool failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "casino.jackpot.create", "jackpot_pool", pool.ID, fiber.Map{"pool_name": pool.Name, "type": pool.Type})
		return c.Status(201).JSON(fiber.Map{"data": pool})
	}
}

func listProviderSettlements(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		providerID := c.Query("provider_id", "")
		page := c.QueryInt("page", 1)
		pageSize := c.QueryInt("page_size", 20)
		if page < 1 { page = 1 }
		if pageSize < 1 || pageSize > 200 { pageSize = 20 }
		q := db.Model(&models.ProviderSettlement{})
		if providerID != "" { q = q.Where("provider_id = ?", providerID) }
		var total int64
		if err := q.Count(&total).Error; err != nil {
			log.Error("count settlements failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		var items []models.ProviderSettlement
		offset := (page - 1) * pageSize
		if err := q.Order("period_end DESC").Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
			log.Error("query settlements failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		tp := int(math.Ceil(float64(total) / float64(pageSize)))
		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{"page": page, "page_size": pageSize, "total": total, "total_pages": tp},
		})
	}
}

func markSettlementPaid(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		now := time.Now()
		var before models.ProviderSettlement
		db.First(&before, "id = ?", id)
		if err := db.Model(&models.ProviderSettlement{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status": "paid", "paid_at": &now, "updated_at": now,
		}).Error; err != nil {
			log.Error("mark settlement paid failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "casino.settlement.mark_paid", "provider_settlement", id, fiber.Map{
			"before_status": before.Status, "after_status": "paid", "amount": before.GGR,
		})
		return c.JSON(fiber.Map{"success": true})
	}
}

func listCasinoBets(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		gameID := c.Query("game_id", "")
		userID := c.Query("user_id", "")
		providerID := c.Query("provider_id", "")
		status := c.Query("status", "")
		dateFrom := c.Query("date_from", "")
		dateTo := c.Query("date_to", "")
		page := c.QueryInt("page", 1)
		pageSize := c.QueryInt("page_size", 20)
		if page < 1 { page = 1 }
		if pageSize < 1 || pageSize > 200 { pageSize = 20 }
		q := db.Model(&models.CasinoGameRound{})
		if gameID != "" { q = q.Where("game_id = ?", gameID) }
		if userID != "" { q = q.Where("user_id = ?", userID) }
		if providerID != "" { q = q.Where("provider_id = ?", providerID) }
		if status != "" { q = q.Where("status = ?", status) }
		if dateFrom != "" { q = q.Where("created_at >= ?", dateFrom) }
		if dateTo != "" { q = q.Where("created_at <= ?", dateTo) }
		var total int64
		if err := q.Count(&total).Error; err != nil {
			log.Error("count casino bets failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		var items []models.CasinoGameRound
		offset := (page - 1) * pageSize
		if err := q.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
			log.Error("query casino bets failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		tp := int(math.Ceil(float64(total) / float64(pageSize)))
		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{"page": page, "page_size": pageSize, "total": total, "total_pages": tp},
		})
	}
}

func adminIDString(c *fiber.Ctx) string {
	adminIDRaw := c.Locals("admin_id")
	if v, ok := adminIDRaw.(string); ok { return v }
	if v, ok := adminIDRaw.(int64); ok { return string(rune(v)) }
	return ""
}

func adminIDInt64(c *fiber.Ctx) *int64 {
	adminIDRaw := c.Locals("admin_id")
	if v, ok := adminIDRaw.(int64); ok { return &v }
	return nil
}

func logAudit(auditSvc *service.AuditService, c *fiber.Ctx, action, resourceType, resourceID string, details map[string]interface{}) {
	ip := c.IP()
	ua := string(c.Request().Header.UserAgent())
	auditSvc.Log(c.Context(), adminIDInt64(c), action, resourceType, resourceID, details, ip, ua)
}
