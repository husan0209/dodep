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

func RegisterAffiliateRoutes(router fiber.Router, db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) {
	aff := router.Group("/affiliates")
	aff.Get("", listAffiliates(db, log))
	aff.Post("", createAffiliate(db, log, auditSvc))
	// Static collection routes MUST be registered before "/:id" patterns
	// to avoid Fiber matching them as ":id".
	aff.Get("/payouts", listPayouts(db, log))
	aff.Post("/payouts/:id/approve", approvePayout(db, log, auditSvc))
	aff.Post("/payouts/:id/reject", rejectPayout(db, log, auditSvc))
	aff.Get("/fraud-flags", listFraudFlags(db, log))
	aff.Get("/:id", getAffiliate(db, log))
	aff.Put("/:id", updateAffiliate(db, log, auditSvc))
	aff.Post("/:id/approve", approveAffiliate(db, log, auditSvc))
	aff.Post("/:id/suspend", suspendAffiliate(db, log, auditSvc))
	aff.Put("/:id/commission-rate", updateCommissionRate(db, log, auditSvc))
	aff.Get("/:id/players", listAffiliatePlayers(db, log))
	aff.Get("/:id/stats", getAffiliateStats(db, log))
	aff.Post("/:id/calculate-period", calculatePeriod(db, log, auditSvc))
	aff.Get("/:id/postback-config", getPostbackConfig(db, log))
	aff.Put("/:id/postback-config", updatePostbackConfig(db, log, auditSvc))
}

func listAffiliates(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		status := c.Query("status", "")
		search := c.Query("search", "")
		page := c.QueryInt("page", 1)
		pageSize := c.QueryInt("page_size", 20)
		if page < 1 { page = 1 }
		if pageSize < 1 || pageSize > 200 { pageSize = 20 }
		q := db.Model(&models.Affiliate{})
		if status != "" { q = q.Where("status = ?", status) }
		if search != "" { q = q.Where("user_id ILIKE ?", "%"+search+"%") }
		var total int64
		if err := q.Count(&total).Error; err != nil {
			log.Error("count affiliates failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		var items []models.Affiliate
		offset := (page - 1) * pageSize
		if err := q.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
			log.Error("query affiliates failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		tp := int(math.Ceil(float64(total) / float64(pageSize)))
		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{"page": page, "page_size": pageSize, "total": total, "total_pages": tp},
		})
	}
}

func getAffiliate(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var aff models.Affiliate
		if err := db.First(&aff, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(404).JSON(fiber.Map{"error": "not found"})
			}
			log.Error("get affiliate failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": aff})
	}
}

func createAffiliate(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var aff models.Affiliate
		if err := c.BodyParser(&aff); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		aff.Status = "pending"
		aff.CreatedAt = time.Now()
		aff.UpdatedAt = time.Now()
		if err := db.Create(&aff).Error; err != nil {
			log.Error("create affiliate failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "affiliate.create", "affiliate", aff.ID, fiber.Map{"user_id": aff.UserID, "deal_type": aff.DealType})
		return c.Status(201).JSON(fiber.Map{"data": aff})
	}
}

func updateAffiliate(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var body map[string]interface{}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var before models.Affiliate
		db.First(&before, "id = ?", id)
		body["updated_at"] = time.Now()
		if err := db.Model(&models.Affiliate{}).Where("id = ?", id).Updates(body).Error; err != nil {
			log.Error("update affiliate failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "affiliate.update", "affiliate", id, fiber.Map{"before": before, "after": body})
		return c.JSON(fiber.Map{"success": true})
	}
}

func approveAffiliate(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var body struct {
			CommissionRate  *float64 `json:"commission_rate,omitempty"`
			HoldPeriodDays  *int     `json:"hold_period_days,omitempty"`
			MinPayoutAmount *string  `json:"min_payout_amount,omitempty"`
			Currency        *string  `json:"currency,omitempty"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		updates := map[string]interface{}{"status": "active", "updated_at": time.Now()}
		if body.CommissionRate != nil { updates["revenue_share_pct"] = *body.CommissionRate }
		if body.HoldPeriodDays != nil { updates["hold_period_days"] = *body.HoldPeriodDays }
		if body.MinPayoutAmount != nil { updates["min_payout_amount"] = *body.MinPayoutAmount }
		if body.Currency != nil { updates["currency"] = *body.Currency }
		if err := db.Model(&models.Affiliate{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			log.Error("approve affiliate failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "affiliate.approve", "affiliate", id, fiber.Map{"updates": updates})
		return c.JSON(fiber.Map{"success": true})
	}
}

func suspendAffiliate(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if err := db.Model(&models.Affiliate{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status": "suspended", "updated_at": time.Now(),
		}).Error; err != nil {
			log.Error("suspend affiliate failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "affiliate.suspend", "affiliate", id, nil)
		return c.JSON(fiber.Map{"success": true})
	}
}

func updateCommissionRate(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var body struct {
			Rate float64 `json:"commission_rate"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if err := db.Model(&models.Affiliate{}).Where("id = ?", id).Updates(map[string]interface{}{
			"revenue_share_pct": body.Rate, "updated_at": time.Now(),
		}).Error; err != nil {
			log.Error("update commission rate failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "affiliate.commission_rate.update", "affiliate", id, fiber.Map{"rate": body.Rate})
		return c.JSON(fiber.Map{"success": true})
	}
}

func listAffiliatePlayers(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"data": []fiber.Map{}, "pagination": fiber.Map{"page": 1, "page_size": 20, "total": 0}})
	}
}

func getAffiliateStats(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"data": fiber.Map{"ngr": "0", "players": 0, "owed": "0"}})
	}
}

func listPayouts(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		status := c.Query("status", "")
		page := c.QueryInt("page", 1)
		pageSize := c.QueryInt("page_size", 20)
		if page < 1 { page = 1 }
		if pageSize < 1 || pageSize > 200 { pageSize = 20 }
		q := db.Model(&models.AffiliatePayout{})
		if status != "" { q = q.Where("status = ?", status) }
		var total int64
		if err := q.Count(&total).Error; err != nil {
			log.Error("count payouts failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		var items []models.AffiliatePayout
		offset := (page - 1) * pageSize
		if err := q.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
			log.Error("query payouts failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		tp := int(math.Ceil(float64(total) / float64(pageSize)))
		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{"page": page, "page_size": pageSize, "total": total, "total_pages": tp},
		})
	}
}

func approvePayout(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var body struct {
			ProviderReference string `json:"provider_reference"`
		}
		if err := c.BodyParser(&body); err != nil {
			body.ProviderReference = ""
		}
		if err := db.Model(&models.AffiliatePayout{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status": "approved", "provider_reference": body.ProviderReference, "updated_at": time.Now(),
		}).Error; err != nil {
			log.Error("approve payout failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "affiliate.payout.approve", "affiliate_payout", id, fiber.Map{"provider_reference": body.ProviderReference})
		return c.JSON(fiber.Map{"success": true})
	}
}

func rejectPayout(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var body struct {
			Reason string `json:"rejection_reason"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if err := db.Model(&models.AffiliatePayout{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status": "rejected", "rejection_reason": body.Reason, "updated_at": time.Now(),
		}).Error; err != nil {
			log.Error("reject payout failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "affiliate.payout.reject", "affiliate_payout", id, fiber.Map{"reason": body.Reason})
		return c.JSON(fiber.Map{"success": true})
	}
}

func listFraudFlags(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		status := c.Query("status", "")
		page := c.QueryInt("page", 1)
		pageSize := c.QueryInt("page_size", 20)
		if page < 1 { page = 1 }
		if pageSize < 1 || pageSize > 200 { pageSize = 20 }
		q := db.Model(&models.FraudFlag{})
		if status != "" { q = q.Where("status = ?", status) }
		var total int64
		if err := q.Count(&total).Error; err != nil {
			log.Error("count fraud flags failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		var items []models.FraudFlag
		offset := (page - 1) * pageSize
		if err := q.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
			log.Error("query fraud flags failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		tp := int(math.Ceil(float64(total) / float64(pageSize)))
		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{"page": page, "page_size": pageSize, "total": total, "total_pages": tp},
		})
	}
}

func calculatePeriod(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		logAudit(auditSvc, c, "affiliate.calculate_period", "affiliate", id, nil)
		return c.JSON(fiber.Map{"success": true, "message": "period calculation queued"})
	}
}

func getPostbackConfig(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var aff models.Affiliate
		if err := db.First(&aff, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(404).JSON(fiber.Map{"error": "not found"})
			}
			log.Error("get postback config failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": aff.PostbackConfigs})
	}
}

func updatePostbackConfig(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var configs []models.PostbackConfig
		if err := c.BodyParser(&configs); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if err := db.Model(&models.Affiliate{}).Where("id = ?", id).Updates(map[string]interface{}{
			"postback_configs": configs, "updated_at": time.Now(),
		}).Error; err != nil {
			log.Error("update postback config failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "affiliate.postback_config.update", "affiliate", id, fiber.Map{"configs": configs})
		return c.JSON(fiber.Map{"success": true})
	}
}
