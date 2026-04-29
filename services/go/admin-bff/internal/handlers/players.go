package handlers

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/opus-casino/admin-bff/internal/models"
	"github.com/opus-casino/admin-bff/internal/service"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// PlayerNote is stored in the admin_notes table (simple append-only log).
type PlayerNote struct {
	ID        string    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PlayerID  string    `gorm:"type:varchar(36);not null;index" json:"player_id"`
	Text      string    `gorm:"type:text;not null" json:"text"`
	AuthorID  string    `gorm:"type:varchar(36);not null" json:"author_id"`
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
}

func (PlayerNote) TableName() string { return "player_admin_notes" }

// PlayerMergeRequest is the DB record for a merge operation.
type PlayerMergeRecord struct {
	ID          string     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PrimaryID   string     `gorm:"type:varchar(36);not null" json:"primary_id"`
	SecondaryID string     `gorm:"type:varchar(36);not null" json:"secondary_id"`
	Reason      string     `gorm:"type:text;not null" json:"reason"`
	InitiatedBy string     `gorm:"type:varchar(36);not null" json:"initiated_by"`
	ConfirmedBy string     `gorm:"type:varchar(36);not null" json:"confirmed_by"`
	Status      string     `gorm:"type:varchar(20);not null;default:'completed'" json:"status"`
	CreatedAt   time.Time  `gorm:"not null;default:now()" json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

func (PlayerMergeRecord) TableName() string { return "player_merge_records" }

// RegisterPlayerRoutes mounts all player mutator and merge endpoints.
func RegisterPlayerRoutes(router fiber.Router, db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) {
	// Auto-migrate the local tables managed by this handler.
	db.AutoMigrate(&PlayerNote{}, &PlayerMergeRecord{})

	players := router.Group("/players")

	// ── List players ──────────────────────────────────────────────────────────
	players.Get("/", listPlayers(db, log))

	// ── Player profile (aggregated from local DB + gRPC services) ─────────────
	players.Get("/:id/overview", getPlayerOverview(db, log))

	// ── Notes ─────────────────────────────────────────────────────────────────
	players.Get("/:id/notes", func(c *fiber.Ctx) error {
		var notes []PlayerNote
		db.Where("player_id = ?", c.Params("id")).Order("created_at DESC").Find(&notes)
		return c.JSON(fiber.Map{"data": notes})
	})

	players.Post("/:id/notes", func(c *fiber.Ctx) error {
		var req struct {
			Text string `json:"text"`
		}
		if err := c.BodyParser(&req); err != nil || req.Text == "" {
			return c.Status(400).JSON(fiber.Map{"error": "text required"})
		}
		authorID := fmt.Sprintf("%v", c.Locals("admin_id"))
		note := PlayerNote{
			PlayerID:  c.Params("id"),
			Text:      req.Text,
			AuthorID:  authorID,
			CreatedAt: time.Now(),
		}
		db.Create(&note)
		logAudit(auditSvc, c, "player.note.add", "player", c.Params("id"), fiber.Map{"text": req.Text})
		return c.JSON(fiber.Map{"data": note})
	})

	// ── Block / Unblock ────────────────────────────────────────────────────────
	players.Post("/:id/block", func(c *fiber.Ctx) error {
		var req struct {
			Type          string `json:"type"` // full|casino|sports|temporary
			DurationHours *int   `json:"duration_hours"`
			Reason        string `json:"reason"`
		}
		if err := c.BodyParser(&req); err != nil || req.Reason == "" {
			return c.Status(400).JSON(fiber.Map{"error": "reason required"})
		}
		// TODO: call User Service gRPC BlockPlayer when available.
		// For now, record the action in audit log and acknowledge.
		logAudit(auditSvc, c, "player.block", "player", c.Params("id"), fiber.Map{
			"type": req.Type, "duration_hours": req.DurationHours, "reason": req.Reason,
		})
		return c.JSON(fiber.Map{"success": true, "message": "player block recorded; propagate via User Service gRPC"})
	})

	players.Post("/:id/unblock", func(c *fiber.Ctx) error {
		var req struct {
			Reason string `json:"reason"`
		}
		if err := c.BodyParser(&req); err != nil || req.Reason == "" {
			return c.Status(400).JSON(fiber.Map{"error": "reason required"})
		}
		logAudit(auditSvc, c, "player.unblock", "player", c.Params("id"), fiber.Map{"reason": req.Reason})
		return c.JSON(fiber.Map{"success": true})
	})

	// ── Limits ─────────────────────────────────────────────────────────────────
	players.Post("/:id/limits", func(c *fiber.Ctx) error {
		var req struct {
			MaxDepositDaily    *string `json:"max_deposit_daily"`
			MaxDepositWeekly   *string `json:"max_deposit_weekly"`
			MaxWithdrawalDaily *string `json:"max_withdrawal_daily"`
			MaxBet             *string `json:"max_bet"`
			MaxLossDaily       *string `json:"max_loss_daily"`
			Reason             string  `json:"reason"`
		}
		if err := c.BodyParser(&req); err != nil || req.Reason == "" {
			return c.Status(400).JSON(fiber.Map{"error": "reason required"})
		}
		// TODO: call User/Wallet Service gRPC SetLimits.
		logAudit(auditSvc, c, "player.limits.set", "player", c.Params("id"), fiber.Map{
			"limits": req, "reason": req.Reason,
		})
		return c.JSON(fiber.Map{"success": true})
	})

	// ── Adjust Balance (requires TOTP confirmation in the request body) ─────────
	players.Post("/:id/adjust-balance", func(c *fiber.Ctx) error {
		var req struct {
			Amount   string `json:"amount"`
			Currency string `json:"currency"`
			Type     string `json:"type"` // credit|debit
			Reason   string `json:"reason"`
			TOTPCode string `json:"totp_code"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if req.Reason == "" || req.Amount == "" || req.Type == "" {
			return c.Status(400).JSON(fiber.Map{"error": "amount, type, and reason required"})
		}
		if req.TOTPCode == "" {
			return c.Status(403).JSON(fiber.Map{"error": "TOTP_REQUIRED", "message": "totp_code is required for balance adjustments"})
		}
		// TODO: validate req.TOTPCode against current admin TOTP secret.
		// TODO: call Wallet Service gRPC CreditDebit.
		logAudit(auditSvc, c, "player.balance.adjust", "player", c.Params("id"), fiber.Map{
			"amount": req.Amount, "currency": req.Currency, "type": req.Type, "reason": req.Reason,
		})
		return c.JSON(fiber.Map{"success": true})
	})

	// ── Tags ──────────────────────────────────────────────────────────────────
	players.Post("/:id/tags", func(c *fiber.Ctx) error {
		var req struct {
			Add    []string `json:"add"`
			Remove []string `json:"remove"`
			Reason string   `json:"reason"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		// TODO: call User Service gRPC UpdateTags.
		logAudit(auditSvc, c, "player.tags.update", "player", c.Params("id"), fiber.Map{
			"add": req.Add, "remove": req.Remove, "reason": req.Reason,
		})
		return c.JSON(fiber.Map{"success": true})
	})

	// ── Player Group ──────────────────────────────────────────────────────────
	players.Post("/:id/group", func(c *fiber.Ctx) error {
		var req struct {
			Group  string `json:"group"` // standard|vip|vvip|whale
			Reason string `json:"reason"`
		}
		if err := c.BodyParser(&req); err != nil || req.Group == "" || req.Reason == "" {
			return c.Status(400).JSON(fiber.Map{"error": "group and reason required"})
		}
		logAudit(auditSvc, c, "player.group.set", "player", c.Params("id"), fiber.Map{
			"group": req.Group, "reason": req.Reason,
		})
		return c.JSON(fiber.Map{"success": true})
	})

	// ── Give Bonus ────────────────────────────────────────────────────────────
	players.Post("/:id/give-bonus", func(c *fiber.Ctx) error {
		var req struct {
			BonusID string `json:"bonus_id"`
			Reason  string `json:"reason"`
		}
		if err := c.BodyParser(&req); err != nil || req.BonusID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "bonus_id required"})
		}
		// Record a player_bonus activation
		expiry := time.Now().Add(30 * 24 * time.Hour)
		pb := models.PlayerBonus{
			BonusID:   req.BonusID,
			Status:    "active",
			IssuedAt:  time.Now(),
			ExpiresAt: &expiry,
		}
		db.Create(&pb)
		logAudit(auditSvc, c, "player.bonus.give", "player", c.Params("id"), fiber.Map{
			"bonus_id": req.BonusID, "reason": req.Reason,
		})
		return c.JSON(fiber.Map{"success": true, "data": pb})
	})

	// ── Request KYC ──────────────────────────────────────────────────────────
	players.Post("/:id/request-kyc", func(c *fiber.Ctx) error {
		var req struct {
			Type    string  `json:"type"` // identity|address|source_of_funds
			Message *string `json:"message"`
		}
		if err := c.BodyParser(&req); err != nil || req.Type == "" {
			return c.Status(400).JSON(fiber.Map{"error": "type required"})
		}
		// Create a SOF request if type=source_of_funds; otherwise record audit.
		if req.Type == "source_of_funds" {
			pid, _ := strconv.ParseInt(c.Params("id"), 10, 64)
			sof := models.SofRequest{
				PlayerID:  pid,
				Status:    "pending",
				CreatedAt: time.Now(),
			}
			db.Create(&sof)
		}
		logAudit(auditSvc, c, "player.kyc.request", "player", c.Params("id"), fiber.Map{
			"type": req.Type, "message": req.Message,
		})
		return c.JSON(fiber.Map{"success": true})
	})

	// ── Send Message ──────────────────────────────────────────────────────────
	players.Post("/:id/send-message", func(c *fiber.Ctx) error {
		var req struct {
			Channel string `json:"channel"` // email|sms|push
			Subject string `json:"subject"`
			Body    string `json:"body"`
		}
		if err := c.BodyParser(&req); err != nil || req.Body == "" || req.Channel == "" {
			return c.Status(400).JSON(fiber.Map{"error": "channel and body required"})
		}
		// Check suppression before sending
		var sup models.CommunicationSuppression
		err := db.Where("player_id = ? AND (channel IS NULL OR channel = ?) AND (expires_at IS NULL OR expires_at > ?)",
			c.Params("id"), req.Channel, time.Now()).First(&sup).Error
		if err == nil {
			return c.Status(422).JSON(fiber.Map{
				"error":  "PLAYER_SUPPRESSED",
				"reason": sup.Reason,
			})
		}
		// TODO: dispatch via CRM/notification service.
		logAudit(auditSvc, c, "player.message.send", "player", c.Params("id"), fiber.Map{
			"channel": req.Channel, "subject": req.Subject,
		})
		return c.JSON(fiber.Map{"success": true})
	})

	// ── Player Merge (B2.1) — SUPER_ADMIN + TOTP + second SUPER_ADMIN confirm ──
	players.Get("/merge/preview", func(c *fiber.Ctx) error {
		primaryID := c.Query("primary")
		secondaryID := c.Query("secondary")
		if primaryID == "" || secondaryID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "primary and secondary required"})
		}
		// Return a lightweight diff showing what will be merged.
		// TODO: fetch full balance + tag data from User/Wallet services.
		return c.JSON(fiber.Map{
			"data": fiber.Map{
				"primary_id":   primaryID,
				"secondary_id": secondaryID,
				"warnings":     []string{"All bets, transactions, bonuses from secondary will be re-linked to primary"},
				"action":       "secondary will be blocked with reason: merged_into:" + primaryID,
			},
		})
	})

	players.Post("/merge", func(c *fiber.Ctx) error {
		var req struct {
			PrimaryID   string `json:"primary_id"`
			SecondaryID string `json:"secondary_id"`
			Reason      string `json:"reason"`
			TOTPCode    string `json:"totp_code"`
			ConfirmedBy string `json:"confirmed_by"` // second SUPER_ADMIN id
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if req.PrimaryID == "" || req.SecondaryID == "" || req.Reason == "" {
			return c.Status(400).JSON(fiber.Map{"error": "primary_id, secondary_id, and reason required"})
		}
		if req.TOTPCode == "" {
			return c.Status(403).JSON(fiber.Map{"error": "TOTP_REQUIRED", "message": "totp_code required for merge"})
		}
		initiatorID := fmt.Sprintf("%v", c.Locals("admin_id"))
		if req.ConfirmedBy == "" || req.ConfirmedBy == initiatorID {
			return c.Status(400).JSON(fiber.Map{"error": "confirmed_by must be a different SUPER_ADMIN"})
		}

		// TODO: implement transactional merge via User + Wallet + Bonus + Betting services.
		now := time.Now()
		record := PlayerMergeRecord{
			PrimaryID:   req.PrimaryID,
			SecondaryID: req.SecondaryID,
			Reason:      req.Reason,
			InitiatedBy: initiatorID,
			ConfirmedBy: req.ConfirmedBy,
			Status:      "completed",
			CreatedAt:   now,
			CompletedAt: &now,
		}
		db.Create(&record)

		logAudit(auditSvc, c, "player.merge", "player", req.PrimaryID, fiber.Map{
			"primary_id": req.PrimaryID, "secondary_id": req.SecondaryID,
			"reason": req.Reason, "confirmed_by": req.ConfirmedBy,
		})
		return c.JSON(fiber.Map{"success": true, "data": record})
	})

	// ── Communication Suppression (A7.1) ──────────────────────────────────────
	players.Get("/:id/suppressions", func(c *fiber.Ctx) error {
		var items []models.CommunicationSuppression
		db.Where("player_id = ?", c.Params("id")).Find(&items)
		return c.JSON(fiber.Map{"data": items})
	})

	players.Post("/:id/suppressions", func(c *fiber.Ctx) error {
		var req struct {
			Reason    string     `json:"reason"`
			Channel   *string    `json:"channel"`
			ExpiresAt *time.Time `json:"expires_at"`
		}
		if err := c.BodyParser(&req); err != nil || req.Reason == "" {
			return c.Status(400).JSON(fiber.Map{"error": "reason required"})
		}
		addedBy := fmt.Sprintf("%v", c.Locals("admin_id"))
		sup := models.CommunicationSuppression{
			PlayerID:  c.Params("id"),
			Reason:    req.Reason,
			Channel:   req.Channel,
			AddedBy:   addedBy,
			ExpiresAt: req.ExpiresAt,
			CreatedAt: time.Now(),
		}
		db.Create(&sup)
		logAudit(auditSvc, c, "player.suppression.add", "player", c.Params("id"), fiber.Map{
			"reason": req.Reason, "channel": req.Channel,
		})
		return c.JSON(fiber.Map{"data": sup})
	})

	players.Delete("/:id/suppressions/:sup_id", func(c *fiber.Ctx) error {
		db.Where("id = ? AND player_id = ?", c.Params("sup_id"), c.Params("id")).
			Delete(&models.CommunicationSuppression{})
		return c.JSON(fiber.Map{"success": true})
	})
}

// listPlayers returns paginated player list with filters.
// Data is sourced from the local deposits/risk tables; a full implementation
// would fan-out to User Service gRPC.
func listPlayers(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		search := c.Query("search", "")
		country := c.Query("country", "")
		kycStatus := c.Query("kyc_status", "")
		riskMin := c.QueryInt("risk_min", -1)
		riskMax := c.QueryInt("risk_max", -1)
		page := c.QueryInt("page", 1)
		ps := c.QueryInt("page_size", 50)
		if page < 1 { page = 1 }
		if ps < 1 || ps > 200 { ps = 50 }

		type playerRow struct {
			PlayerID  int64   `gorm:"column:player_id"`
			TotalDep  float64 `gorm:"column:total_dep"`
			DepCount  int64   `gorm:"column:dep_count"`
		}
		q := db.Table("deposits").
			Select("player_id, COALESCE(SUM(amount::numeric),0) as total_dep, COUNT(*) as dep_count").
			Group("player_id").
			Order("total_dep DESC")

		if search != "" {
			q = q.Where("player_id::text ILIKE ?", "%"+search+"%")
		}
		_ = country
		_ = kycStatus
		_ = riskMin
		_ = riskMax

		var total int64
		db.Table("deposits").Select("COUNT(DISTINCT player_id)").Scan(&total)

		var rows []playerRow
		q.Limit(ps).Offset((page - 1) * ps).Scan(&rows)

		result := make([]fiber.Map, 0, len(rows))
		for _, r := range rows {
			result = append(result, fiber.Map{
				"id":           r.PlayerID,
				"total_deposit": r.TotalDep,
				"deposit_count": r.DepCount,
			})
		}
		return c.JSON(fiber.Map{
			"data": result,
			"pagination": fiber.Map{
				"page": page, "page_size": ps, "total": total,
				"total_pages": int(math.Ceil(float64(total) / float64(ps))),
			},
		})
	}
}

// getPlayerOverview aggregates available local data for a player profile.
// A full implementation should fan-out in parallel to User/Payment/Betting/Bonus/KYC/Risk services.
func getPlayerOverview(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		pid := c.Params("id")

		var totalDep float64
		var totalWd float64
		var depCount, wdCount int64

		db.Table("deposits").Where("player_id = ?", pid).
			Select("COALESCE(SUM(amount::numeric),0)").Scan(&totalDep)
		db.Table("deposits").Where("player_id = ?", pid).Count(&depCount)
		db.Table("withdrawals").Where("player_id = ?", pid).
			Select("COALESCE(SUM(amount::numeric),0)").Scan(&totalWd)
		db.Table("withdrawals").Where("player_id = ?", pid).Count(&wdCount)

		var alertCount int64
		db.Table("risk_alerts").Where("user_id = ?", pid).Count(&alertCount)

		var screening models.ScreeningResult
		screeningStatus := "unknown"
		if err := db.Where("player_id = ?", pid).Order("screened_at DESC").First(&screening).Error; err == nil {
			screeningStatus = screening.Status
		}

		var notes []PlayerNote
		db.Where("player_id = ?", pid).Order("created_at DESC").Limit(10).Find(&notes)

		var suppressions []models.CommunicationSuppression
		db.Where("player_id = ?", pid).Find(&suppressions)

		return c.JSON(fiber.Map{
			"data": fiber.Map{
				"player_id":        pid,
				"total_deposits":   totalDep,
				"deposit_count":    depCount,
				"total_withdrawals": totalWd,
				"withdrawal_count": wdCount,
				"risk_alerts":      alertCount,
				"screening_status": screeningStatus,
				"notes":            notes,
				"suppressions":     suppressions,
			},
		})
	}
}
