package handlers

import (
	"fmt"
	"math"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/opus-casino/admin-bff/internal/models"
	"github.com/opus-casino/admin-bff/internal/service"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// CRMSegment represents a saved player segment definition.
type CRMSegment struct {
	ID          string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name        string    `gorm:"type:varchar(255);not null" json:"name"`
	Description *string   `gorm:"type:text" json:"description,omitempty"`
	Conditions  []byte    `gorm:"type:jsonb;not null" json:"conditions"` // []SegmentCondition
	PlayerCount int64     `gorm:"-" json:"player_count"` // computed on GET
	CreatedBy   string    `gorm:"type:varchar(36);not null" json:"created_by"`
	CreatedAt   time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt   time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

func (CRMSegment) TableName() string { return "crm_segments" }

// CRMTrigger is an event-based automation rule.
type CRMTrigger struct {
	ID              string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name            string    `gorm:"type:varchar(255);not null" json:"name"`
	TriggerEvent    string    `gorm:"type:varchar(100);not null" json:"trigger_event"` // ftd_completed|no_redeposit_after_Xdays|...
	TriggerParams   []byte    `gorm:"type:jsonb" json:"trigger_params,omitempty"`
	Actions         []byte    `gorm:"type:jsonb;not null" json:"actions"`              // []TriggerAction
	DelaySeconds    int       `gorm:"not null;default:0" json:"delay_seconds"`
	IsActive        bool      `gorm:"not null;default:false" json:"is_active"`
	FiredCount30d   int64     `gorm:"-" json:"fired_count_30d"`
	CreatedBy       string    `gorm:"type:varchar(36);not null" json:"created_by"`
	CreatedAt       time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt       time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

func (CRMTrigger) TableName() string { return "crm_triggers" }

// CRMCampaign is a one-off manual send.
type CRMCampaign struct {
	ID            string     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name          string     `gorm:"type:varchar(255);not null" json:"name"`
	SegmentID     *string    `gorm:"type:uuid" json:"segment_id,omitempty"`
	Channel       string     `gorm:"type:varchar(20);not null" json:"channel"` // email|sms|push|all
	TemplateID    *string    `gorm:"type:uuid" json:"template_id,omitempty"`
	Status        string     `gorm:"type:varchar(30);not null;default:'draft'" json:"status"` // draft|scheduled|sending|sent|failed
	ScheduledAt   *time.Time `json:"scheduled_at,omitempty"`
	SentAt        *time.Time `json:"sent_at,omitempty"`
	ReachCount    int        `gorm:"not null;default:0" json:"reach_count"`
	DeliveredCount int       `gorm:"not null;default:0" json:"delivered_count"`
	CreatedBy     string     `gorm:"type:varchar(36);not null" json:"created_by"`
	CreatedAt     time.Time  `gorm:"not null;default:now()" json:"created_at"`
}

func (CRMCampaign) TableName() string { return "crm_campaigns" }

// CRMTemplate stores email/sms/push templates.
type CRMTemplate struct {
	ID        string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name      string    `gorm:"type:varchar(255);not null" json:"name"`
	Channel   string    `gorm:"type:varchar(20);not null" json:"channel"` // email|sms|push
	Language  string    `gorm:"type:char(5);not null;default:'en'" json:"language"`
	Subject   *string   `gorm:"type:varchar(255)" json:"subject,omitempty"`
	Body      string    `gorm:"type:text;not null" json:"body"`
	CreatedBy string    `gorm:"type:varchar(36);not null" json:"created_by"`
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

func (CRMTemplate) TableName() string { return "crm_templates" }

// RegisterCRMRoutes mounts all CRM endpoints.
func RegisterCRMRoutes(router fiber.Router, db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) {
	db.AutoMigrate(&CRMSegment{}, &CRMTrigger{}, &CRMCampaign{}, &CRMTemplate{})

	crm := router.Group("/crm")

	// ── Segments ──────────────────────────────────────────────────────────────
	crm.Get("/segments", func(c *fiber.Ctx) error {
		page := c.QueryInt("page", 1)
		ps := c.QueryInt("page_size", 50)
		if page < 1 { page = 1 }
		if ps < 1 || ps > 200 { ps = 50 }
		var total int64
		db.Model(&CRMSegment{}).Count(&total)
		var items []CRMSegment
		db.Order("created_at DESC").Limit(ps).Offset((page - 1) * ps).Find(&items)
		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{
				"page": page, "page_size": ps, "total": total,
				"total_pages": int(math.Ceil(float64(total) / float64(ps))),
			},
		})
	})

	crm.Post("/segments", func(c *fiber.Ctx) error {
		var seg CRMSegment
		if err := c.BodyParser(&seg); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		seg.CreatedBy = fmt.Sprintf("%v", c.Locals("admin_id"))
		seg.CreatedAt = time.Now()
		seg.UpdatedAt = time.Now()
		if err := db.Create(&seg).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "crm.segment.create", "crm_segment", seg.ID, fiber.Map{"name": seg.Name})
		return c.Status(201).JSON(fiber.Map{"data": seg})
	})

	crm.Put("/segments/:id", func(c *fiber.Ctx) error {
		var body map[string]any
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		body["updated_at"] = time.Now()
		db.Model(&CRMSegment{}).Where("id = ?", c.Params("id")).Updates(body)
		return c.JSON(fiber.Map{"success": true})
	})

	crm.Delete("/segments/:id", func(c *fiber.Ctx) error {
		db.Where("id = ?", c.Params("id")).Delete(&CRMSegment{})
		return c.JSON(fiber.Map{"success": true})
	})

	// ── Triggers ──────────────────────────────────────────────────────────────
	crm.Get("/triggers", func(c *fiber.Ctx) error {
		var items []CRMTrigger
		db.Order("created_at DESC").Find(&items)
		return c.JSON(fiber.Map{"data": items})
	})

	crm.Post("/triggers", func(c *fiber.Ctx) error {
		var t CRMTrigger
		if err := c.BodyParser(&t); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		t.CreatedBy = fmt.Sprintf("%v", c.Locals("admin_id"))
		t.CreatedAt = time.Now()
		t.UpdatedAt = time.Now()
		db.Create(&t)
		logAudit(auditSvc, c, "crm.trigger.create", "crm_trigger", t.ID, fiber.Map{"name": t.Name, "event": t.TriggerEvent})
		return c.Status(201).JSON(fiber.Map{"data": t})
	})

	crm.Put("/triggers/:id", func(c *fiber.Ctx) error {
		var body map[string]any
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		body["updated_at"] = time.Now()
		db.Model(&CRMTrigger{}).Where("id = ?", c.Params("id")).Updates(body)
		return c.JSON(fiber.Map{"success": true})
	})

	crm.Post("/triggers/:id/activate", func(c *fiber.Ctx) error {
		db.Model(&CRMTrigger{}).Where("id = ?", c.Params("id")).Updates(map[string]any{
			"is_active": true, "updated_at": time.Now(),
		})
		return c.JSON(fiber.Map{"success": true})
	})

	crm.Post("/triggers/:id/deactivate", func(c *fiber.Ctx) error {
		db.Model(&CRMTrigger{}).Where("id = ?", c.Params("id")).Updates(map[string]any{
			"is_active": false, "updated_at": time.Now(),
		})
		return c.JSON(fiber.Map{"success": true})
	})

	// ── Campaigns ─────────────────────────────────────────────────────────────
	crm.Get("/campaigns", func(c *fiber.Ctx) error {
		status := c.Query("status", "")
		page := c.QueryInt("page", 1)
		ps := c.QueryInt("page_size", 50)
		if page < 1 { page = 1 }
		if ps < 1 || ps > 200 { ps = 50 }
		q := db.Model(&CRMCampaign{})
		if status != "" { q = q.Where("status = ?", status) }
		var total int64
		q.Count(&total)
		var items []CRMCampaign
		q.Order("created_at DESC").Limit(ps).Offset((page - 1) * ps).Find(&items)
		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{"page": page, "page_size": ps, "total": total,
				"total_pages": int(math.Ceil(float64(total) / float64(ps)))},
		})
	})

	crm.Post("/campaigns", func(c *fiber.Ctx) error {
		var camp CRMCampaign
		if err := c.BodyParser(&camp); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		camp.Status = "draft"
		camp.CreatedBy = fmt.Sprintf("%v", c.Locals("admin_id"))
		camp.CreatedAt = time.Now()

		// Estimate reach count: count players NOT in suppression for this channel.
		if camp.SegmentID != nil {
			var reach int64
			db.Table("deposits").Select("COUNT(DISTINCT player_id)").Scan(&reach)
			var suppressed int64
			db.Model(&models.CommunicationSuppression{}).
				Where("channel IS NULL OR channel = ?", camp.Channel).
				Where("expires_at IS NULL OR expires_at > ?", time.Now()).
				Select("COUNT(DISTINCT player_id)").Scan(&suppressed)
			camp.ReachCount = int(reach - suppressed)
		}

		db.Create(&camp)
		logAudit(auditSvc, c, "crm.campaign.create", "crm_campaign", camp.ID, fiber.Map{"name": camp.Name, "channel": camp.Channel})
		return c.Status(201).JSON(fiber.Map{"data": camp})
	})

	crm.Post("/campaigns/:id/launch", func(c *fiber.Ctx) error {
		now := time.Now()
		db.Model(&CRMCampaign{}).Where("id = ?", c.Params("id")).Updates(map[string]any{
			"status": "sending", "sent_at": &now,
		})
		// TODO: enqueue actual send job via job queue / Redpanda.
		logAudit(auditSvc, c, "crm.campaign.launch", "crm_campaign", c.Params("id"), nil)
		return c.JSON(fiber.Map{"success": true, "status": "sending"})
	})

	// ── Templates ─────────────────────────────────────────────────────────────
	crm.Get("/templates", func(c *fiber.Ctx) error {
		channel := c.Query("channel", "")
		var items []CRMTemplate
		q := db.Model(&CRMTemplate{})
		if channel != "" { q = q.Where("channel = ?", channel) }
		q.Order("created_at DESC").Find(&items)
		return c.JSON(fiber.Map{"data": items})
	})

	crm.Post("/templates", func(c *fiber.Ctx) error {
		var t CRMTemplate
		if err := c.BodyParser(&t); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		t.CreatedBy = fmt.Sprintf("%v", c.Locals("admin_id"))
		t.CreatedAt = time.Now()
		t.UpdatedAt = time.Now()
		db.Create(&t)
		return c.Status(201).JSON(fiber.Map{"data": t})
	})

	crm.Put("/templates/:id", func(c *fiber.Ctx) error {
		var body map[string]any
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		body["updated_at"] = time.Now()
		db.Model(&CRMTemplate{}).Where("id = ?", c.Params("id")).Updates(body)
		return c.JSON(fiber.Map{"success": true})
	})

	crm.Delete("/templates/:id", func(c *fiber.Ctx) error {
		db.Where("id = ?", c.Params("id")).Delete(&CRMTemplate{})
		return c.JSON(fiber.Map{"success": true})
	})

	// ── Global Suppression List (A7.1) ─────────────────────────────────────────
	crm.Get("/suppressions", func(c *fiber.Ctx) error {
		reason := c.Query("reason", "")
		channel := c.Query("channel", "")
		page := c.QueryInt("page", 1)
		ps := c.QueryInt("page_size", 50)
		if page < 1 { page = 1 }
		if ps < 1 || ps > 200 { ps = 50 }
		q := db.Model(&models.CommunicationSuppression{})
		if reason != "" { q = q.Where("reason = ?", reason) }
		if channel != "" { q = q.Where("channel = ?", channel) }
		var total int64
		q.Count(&total)
		var items []models.CommunicationSuppression
		q.Order("created_at DESC").Limit(ps).Offset((page - 1) * ps).Find(&items)
		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{"page": page, "page_size": ps, "total": total,
				"total_pages": int(math.Ceil(float64(total) / float64(ps)))},
		})
	})

	crm.Post("/suppressions", func(c *fiber.Ctx) error {
		var req struct {
			PlayerID  string     `json:"player_id"`
			Reason    string     `json:"reason"`
			Channel   *string    `json:"channel"`
			ExpiresAt *time.Time `json:"expires_at"`
		}
		if err := c.BodyParser(&req); err != nil || req.PlayerID == "" || req.Reason == "" {
			return c.Status(400).JSON(fiber.Map{"error": "player_id and reason required"})
		}
		sup := models.CommunicationSuppression{
			PlayerID:  req.PlayerID,
			Reason:    req.Reason,
			Channel:   req.Channel,
			AddedBy:   fmt.Sprintf("%v", c.Locals("admin_id")),
			ExpiresAt: req.ExpiresAt,
			CreatedAt: time.Now(),
		}
		db.Create(&sup)
		logAudit(auditSvc, c, "crm.suppression.add", "suppression", sup.ID, fiber.Map{
			"player_id": req.PlayerID, "reason": req.Reason, "channel": req.Channel,
		})
		return c.Status(201).JSON(fiber.Map{"data": sup})
	})

	crm.Delete("/suppressions/:id", func(c *fiber.Ctx) error {
		db.Where("id = ?", c.Params("id")).Delete(&models.CommunicationSuppression{})
		return c.JSON(fiber.Map{"success": true})
	})

	// ── RG + Marketing Filter preview (A7.2) ────────────────────────────────────
	crm.Post("/segments/rg-filter-preview", func(c *fiber.Ctx) error {
		var req struct {
			PlayerIDs   []string `json:"player_ids"`
			RGThreshold int      `json:"rg_threshold"` // default 70
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if req.RGThreshold == 0 {
			req.RGThreshold = 70
		}

		// Count suppressions that would be applied.
		var suppressedCount int64
		db.Model(&models.CommunicationSuppression{}).
			Where("player_id IN ? AND (expires_at IS NULL OR expires_at > ?)", req.PlayerIDs, time.Now()).
			Select("COUNT(DISTINCT player_id)").Scan(&suppressedCount)

		willReceive := int64(len(req.PlayerIDs)) - suppressedCount
		if willReceive < 0 { willReceive = 0 }

		return c.JSON(fiber.Map{
			"data": fiber.Map{
				"total_target":    len(req.PlayerIDs),
				"will_receive":    willReceive,
				"rg_excluded":     suppressedCount,
				"suppressed_total": suppressedCount,
			},
		})
	})

	// ── Check suppression for a single player+channel (utility endpoint) ───────
	crm.Get("/suppress-check", func(c *fiber.Ctx) error {
		playerID := c.Query("player_id", "")
		channel := c.Query("channel", "")
		if playerID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "player_id required"})
		}
		var sup models.CommunicationSuppression
		err := db.Where("player_id = ? AND (channel IS NULL OR channel = ?) AND (expires_at IS NULL OR expires_at > ?)",
			playerID, channel, time.Now()).First(&sup).Error
		if err != nil {
			return c.JSON(fiber.Map{"data": fiber.Map{"suppressed": false}})
		}
		return c.JSON(fiber.Map{"data": fiber.Map{
			"suppressed": true,
			"reason":     sup.Reason,
			"channel":    sup.Channel,
		}})
	})

	log.Info("CRM routes registered")
}
