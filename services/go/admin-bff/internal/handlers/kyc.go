package handlers

import (
	"math"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/opus-casino/admin-bff/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func RegisterKycRoutes(router fiber.Router, db *gorm.DB, log *zap.Logger) {
	kyc := router.Group("/kyc")

	kyc.Get("/queue", func(c *fiber.Ctx) error {
		status := c.Query("status", "pending")
		priority := c.Query("priority", "")
		page := c.QueryInt("page", 1)
		pageSize := c.QueryInt("page_size", 50)
		if page < 1 { page = 1 }
		if pageSize < 1 || pageSize > 200 { pageSize = 50 }

		q := db.Model(&models.KycReview{})
		if status != "" { q = q.Where("status = ?", status) }
		if priority != "" { q = q.Where("priority = ?", priority) }

		var total int64
		if err := q.Count(&total).Error; err != nil {
			log.Error("count kyc reviews failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		var items []models.KycReview
		offset := (page - 1) * pageSize
		if err := q.Order("priority DESC, created_at ASC").Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
			log.Error("query kyc reviews failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		tp := int(math.Ceil(float64(total) / float64(pageSize)))
		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{"page": page, "page_size": pageSize, "total": total, "total_pages": tp},
		})
	})

	kyc.Post("/reviews/:id/assign", func(c *fiber.Ctx) error {
		id := c.Params("id")
		var req struct { AssignedTo string `json:"assigned_to"` }
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if err := db.Model(&models.KycReview{}).Where("id = ?", id).Updates(map[string]any{
			"assigned_to": req.AssignedTo, "status": "in_review",
		}).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"success": true})
	})

	kyc.Post("/reviews/:id/decision", func(c *fiber.Ctx) error {
		id := c.Params("id")
		var req struct { Decision string `json:"decision"`; Reason string `json:"reason"` }
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		st := req.Decision
		if st == "resubmission" { st = "resubmission_requested" }
		if err := db.Model(&models.KycReview{}).Where("id = ?", id).Updates(map[string]any{
			"status": st, "decision_reason": req.Reason, "reviewed_at": time.Now(),
		}).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		var r models.KycReview
		if err := db.Select("document_id").Where("id = ?", id).First(&r).Error; err == nil {
			ds := "pending"
			if st == "approved" { ds = "verified" } else if st == "rejected" { ds = "rejected" }
			db.Model(&models.KycDocument{}).Where("id = ?", r.DocumentID).Update("status", ds)
		}
		return c.JSON(fiber.Map{"success": true})
	})

	kyc.Get("/documents/:id", func(c *fiber.Ctx) error {
		var d models.KycDocument
		if err := db.Where("id = ?", c.Params("id")).First(&d).Error; err != nil {
			if err == gorm.ErrRecordNotFound { return c.Status(404).JSON(fiber.Map{"error": "not found"}) }
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": d})
	})

	kyc.Get("/players/:player_id/documents", func(c *fiber.Ctx) error {
		var docs []models.KycDocument
		if err := db.Where("player_id = ?", c.Params("player_id")).Order("created_at DESC").Find(&docs).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": docs})
	})

	kyc.Get("/sof/requests", func(c *fiber.Ctx) error {
		st := c.Query("status", "")
		page := c.QueryInt("page", 1)
		ps := c.QueryInt("page_size", 50)
		if page < 1 { page = 1 }
		if ps < 1 || ps > 200 { ps = 50 }
		q := db.Model(&models.SofRequest{})
		if st != "" { q = q.Where("status = ?", st) }
		var total int64
		q.Count(&total)
		var items []models.SofRequest
		q.Order("created_at DESC").Limit(ps).Offset((page - 1) * ps).Find(&items)
		ids := make([]string, 0, len(items))
		for _, it := range items { ids = append(ids, it.ID) }
		var docs []models.SofDocument
		if len(ids) > 0 { db.Where("request_id IN ?", ids).Find(&docs) }
		dm := make(map[string][]models.SofDocument)
		for _, d := range docs { dm[d.RequestID] = append(dm[d.RequestID], d) }
		type resp struct {
			models.SofRequest
			Documents []models.SofDocument `json:"documents"`
		}
		out := make([]resp, 0, len(items))
		for _, it := range items { out = append(out, resp{SofRequest: it, Documents: dm[it.ID]}) }
		return c.JSON(fiber.Map{
			"data": out,
			"pagination": fiber.Map{"page": page, "page_size": ps, "total": total, "total_pages": int(math.Ceil(float64(total)/float64(ps)))},
		})
	})

	kyc.Post("/sof/requests/:id/review", func(c *fiber.Ctx) error {
		var req struct { Decision string `json:"decision"`; Notes string `json:"notes"` }
		if err := c.BodyParser(&req); err != nil { return c.Status(400).JSON(fiber.Map{"error": "invalid body"}) }
		st := req.Decision
		if st == "approve" { st = "approved" } else if st == "reject" { st = "rejected" }
		db.Model(&models.SofRequest{}).Where("id = ?", c.Params("id")).Updates(map[string]any{
			"status": st, "notes": req.Notes, "reviewed_at": time.Now(),
		})
		return c.JSON(fiber.Map{"success": true})
	})

	kyc.Get("/screenings", func(c *fiber.Ctx) error {
		st := c.Query("status", "")
		page := c.QueryInt("page", 1)
		ps := c.QueryInt("page_size", 50)
		if page < 1 { page = 1 }
		if ps < 1 || ps > 200 { ps = 50 }
		q := db.Model(&models.ScreeningResult{})
		if st != "" { q = q.Where("status = ?", st) }
		var total int64
		q.Count(&total)
		var items []models.ScreeningResult
		q.Order("screened_at DESC").Limit(ps).Offset((page - 1) * ps).Find(&items)
		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{"page": page, "page_size": ps, "total": total, "total_pages": int(math.Ceil(float64(total)/float64(ps)))},
		})
	})

	kyc.Get("/screenings/player/:player_id", func(c *fiber.Ctx) error {
		var r models.ScreeningResult
		if err := db.Where("player_id = ?", c.Params("player_id")).Order("screened_at DESC").First(&r).Error; err != nil {
			if err == gorm.ErrRecordNotFound { return c.JSON(fiber.Map{"data": nil}) }
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": r})
	})

	kyc.Post("/screenings/player/:player_id/rescreen", func(c *fiber.Ctx) error {
		playerID := c.Params("player_id")
		playerIDInt, _ := strconv.ParseInt(playerID, 10, 64)
		screening := models.ScreeningResult{
			PlayerID:   playerIDInt,
			Status:     "review_required",
			ScreenedAt: time.Now(),
			ScreenedBy: "manual",
		}
		if err := db.Create(&screening).Error; err != nil { return c.Status(500).JSON(fiber.Map{"error": "database error"}) }
		return c.JSON(fiber.Map{"success": true, "data": screening})
	})

	kyc.Post("/screenings/:id/review", func(c *fiber.Ctx) error {
		var req struct { Decision string `json:"decision"`; Notes string `json:"notes"` }
		c.BodyParser(&req)
		db.Model(&models.ScreeningResult{}).Where("id = ?", c.Params("id")).Updates(map[string]any{
			"status": req.Decision, "review_notes": req.Notes, "reviewed_at": time.Now(),
		})
		return c.JSON(fiber.Map{"success": true})
	})

	kyc.Get("/expiry-stats", func(c *fiber.Ctx) error {
		n := time.Now()
		var e30, e7, ex int64
		db.Model(&models.KycDocument{}).Where("expires_at BETWEEN ? AND ?", n, n.AddDate(0,0,30)).Count(&e30)
		db.Model(&models.KycDocument{}).Where("expires_at BETWEEN ? AND ?", n, n.AddDate(0,0,7)).Count(&e7)
		db.Model(&models.KycDocument{}).Where("expires_at < ?", n).Count(&ex)
		return c.JSON(fiber.Map{"data": fiber.Map{"expiring_30d": e30, "expiring_7d": e7, "expired": ex}})
	})

	kyc.Get("/expiring", func(c *fiber.Ctx) error {
		page := c.QueryInt("page", 1)
		ps := c.QueryInt("page_size", 50)
		if page < 1 { page = 1 }
		if ps < 1 || ps > 200 { ps = 50 }
		n := time.Now()
		q := db.Model(&models.KycDocument{}).Where("expires_at BETWEEN ? AND ?", n, n.AddDate(0,0,30))
		var total int64
		q.Count(&total)
		var items []models.KycDocument
		q.Order("expires_at ASC").Limit(ps).Offset((page-1)*ps).Find(&items)
		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{"page": page, "page_size": ps, "total": total, "total_pages": int(math.Ceil(float64(total)/float64(ps)))},
		})
	})

	kyc.Get("/team-stats", func(c *fiber.Ctx) error {
		td := time.Now().Format("2006-01-02")
		var qd int64
		db.Model(&models.KycReview{}).Where("status IN ?", []string{"pending","in_review"}).Count(&qd)
		var avg int64
		db.Raw("SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (reviewed_at - created_at))/60),0) FROM kyc_reviews WHERE reviewed_at IS NOT NULL AND DATE(created_at)=?", td).Scan(&avg)
		var sb int64
		db.Model(&models.KycReview{}).Where("wait_time_minutes > 120 AND status IN ?", []string{"pending","in_review"}).Count(&sb)
		var off []models.KycTeamMetric
		db.Where("metric_date = ?", td).Find(&off)
		return c.JSON(fiber.Map{"data": fiber.Map{
			"today": fiber.Map{"queue_depth": qd, "avg_review_minutes": avg, "sla_breaches": sb},
			"officers": off,
		}})
	})

	kyc.Get("/rg/alerts", func(c *fiber.Ctx) error {
		sev := c.Query("severity", "")
		ack := c.Query("acknowledged", "")
		page := c.QueryInt("page", 1)
		ps := c.QueryInt("page_size", 50)
		if page < 1 { page = 1 }
		if ps < 1 || ps > 200 { ps = 50 }
		q := db.Model(&models.RgAlert{})
		if sev != "" { q = q.Where("severity = ?", sev) }
		if ack == "false" || ack == "0" { q = q.Where("acknowledged_by IS NULL") }
		if ack == "true" || ack == "1" { q = q.Where("acknowledged_by IS NOT NULL") }
		var total int64
		q.Count(&total)
		var items []models.RgAlert
		q.Order("created_at DESC").Limit(ps).Offset((page-1)*ps).Find(&items)
		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{"page": page, "page_size": ps, "total": total, "total_pages": int(math.Ceil(float64(total)/float64(ps)))},
		})
	})

	kyc.Post("/rg/alerts/:id/acknowledge", func(c *fiber.Ctx) error {
		db.Model(&models.RgAlert{}).Where("id = ?", c.Params("id")).Updates(map[string]any{
			"acknowledged_by": "admin-id", "acknowledged_at": time.Now(),
		})
		return c.JSON(fiber.Map{"success": true})
	})

	kyc.Get("/rg/players/:player_id/limits", func(c *fiber.Ctx) error {
		var l models.RgLimit
		if err := db.Where("player_id = ?", c.Params("player_id")).First(&l).Error; err != nil {
			if err == gorm.ErrRecordNotFound { return c.JSON(fiber.Map{"data": fiber.Map{}}) }
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": l})
	})

	kyc.Put("/rg/players/:player_id/limits", func(c *fiber.Ctx) error {
		var req models.RgLimit
		if err := c.BodyParser(&req); err != nil { return c.Status(400).JSON(fiber.Map{"error": "invalid body"}) }
		pidStr := c.Params("player_id")
		pidInt, _ := strconv.ParseInt(pidStr, 10, 64)
		var ex models.RgLimit
		if err := db.Where("player_id = ?", pidInt).First(&ex).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				req.PlayerID = pidInt
				db.Create(&req)
				return c.JSON(fiber.Map{"data": req})
			}
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		db.Model(&ex).Updates(&req)
		return c.JSON(fiber.Map{"data": ex})
	})
}
