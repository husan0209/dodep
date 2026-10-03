package handlers

// Withdrawal triage routes.
//
// The withdrawal lifecycle (request, review decision, PSP payout, wallet
// reservation) lives in Payment Service; this group only serves the operator
// triage view that adds the risk/KYC/AML checklist on top of it. Financial
// decisions go through /finance/withdrawals/:id/{approve,reject}.

import (
	"math"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/opus-casino/admin-bff/internal/models"
	"github.com/opus-casino/admin-bff/internal/service"
)

func RegisterWithdrawalRoutes(router fiber.Router, db *gorm.DB, financeSvc *service.FinanceService, auditSvc *service.AuditService, log *zap.Logger) {
	withdrawals := router.Group("/withdrawals")

	// Triage queue: oldest first, so the longest-waiting requests surface.
	withdrawals.Get("/", func(c *fiber.Ctx) error {
		status := c.Query("status", "")
		playerID := c.Query("player_id", "")
		page := c.QueryInt("page", 1)
		ps := c.QueryInt("page_size", 50)
		if page < 1 {
			page = 1
		}
		if ps < 1 || ps > 200 {
			ps = 50
		}

		q := db.Model(&models.Withdrawal{})
		if status != "" {
			q = q.Where("status = ?", status)
		}
		if playerID != "" {
			q = q.Where("player_id = ?", playerID)
		}

		var total int64
		q.Count(&total)
		var items []models.Withdrawal
		q.Order("created_at ASC").Limit(ps).Offset((page - 1) * ps).Find(&items)

		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{
				"page": page, "page_size": ps,
				"total": total, "total_pages": int(math.Ceil(float64(total) / float64(ps))),
			},
		})
	})

	// Triage detail: local checklist plus the authoritative Payment Service state.
	withdrawals.Get("/:id", func(c *fiber.Ctx) error {
		var w models.Withdrawal
		if err := db.Where("id = ?", c.Params("id")).First(&w).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(404).JSON(fiber.Map{"error": "not found"})
			}
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		resp := fiber.Map{"data": w}
		// The linked payment withdrawal is the source of truth for money state.
		if w.PaymentUUID != nil && *w.PaymentUUID != "" {
			if payment, err := financeSvc.GetWithdrawal(c.Context(), *w.PaymentUUID); err == nil {
				resp["payment"] = payment
			} else {
				log.Warn("linked payment withdrawal unavailable", zap.Error(err), zap.String("payment_uuid", *w.PaymentUUID))
			}
		}
		return c.JSON(resp)
	})

	// Approve from the triage view: :id is the local triage row, which is
	// linked to the Payment Service withdrawal that actually holds the funds.
	// The decision is executed by Payment Service; this handler only records
	// the triage outcome afterwards.
	withdrawals.Post("/:id/approve", func(c *fiber.Ctx) error {
		var w models.Withdrawal
		if err := db.Where("id = ?", c.Params("id")).First(&w).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(404).JSON(fiber.Map{"error": "not found"})
			}
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		if w.PaymentUUID == nil || *w.PaymentUUID == "" {
			return c.Status(422).JSON(fiber.Map{"error": "triage row is not linked to a payment withdrawal"})
		}
		adminID := adminIDInt64(c)
		if adminID == nil {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		adminName := adminIDString(c)

		decided, err := financeSvc.ApproveWithdrawal(c.Context(), *adminID, adminName, *w.PaymentUUID)
		if err != nil {
			log.Error("withdrawal approve failed", zap.Error(err), zap.String("payment_uuid", *w.PaymentUUID))
			return c.Status(422).JSON(fiber.Map{"error": err.Error()})
		}
		now := time.Now()
		if mirrorErr := db.Model(&models.Withdrawal{}).Where("id = ?", w.ID).Updates(map[string]any{
			"status":           "approved",
			"approved_by":      adminName,
			"approved_by_name": adminName,
			"approved_at":      now,
			"updated_at":       now,
		}).Error; mirrorErr != nil {
			log.Warn("triage mirror failed (payment decision succeeded)", zap.Error(mirrorErr))
		}
		logAudit(auditSvc, c, "withdrawal.approve", "withdrawal", *w.PaymentUUID, fiber.Map{"status": decided.Status})
		return c.JSON(fiber.Map{"success": true, "status": decided.Status})
	})

	// Reject from the triage view; Payment Service releases the reserved funds.
	withdrawals.Post("/:id/decline", func(c *fiber.Ctx) error {
		var req struct {
			Reason string `json:"reason"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if req.Reason == "" {
			return c.Status(400).JSON(fiber.Map{"error": "reason is required"})
		}
		var w models.Withdrawal
		if err := db.Where("id = ?", c.Params("id")).First(&w).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(404).JSON(fiber.Map{"error": "not found"})
			}
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		if w.PaymentUUID == nil || *w.PaymentUUID == "" {
			return c.Status(422).JSON(fiber.Map{"error": "triage row is not linked to a payment withdrawal"})
		}
		adminID := adminIDInt64(c)
		if adminID == nil {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		adminName := adminIDString(c)

		decided, err := financeSvc.RejectWithdrawal(c.Context(), *adminID, adminName, *w.PaymentUUID, req.Reason)
		if err != nil {
			log.Error("withdrawal reject failed", zap.Error(err), zap.String("payment_uuid", *w.PaymentUUID))
			return c.Status(422).JSON(fiber.Map{"error": err.Error()})
		}
		now := time.Now()
		if mirrorErr := db.Model(&models.Withdrawal{}).Where("id = ?", w.ID).Updates(map[string]any{
			"status":           "rejected",
			"rejected_by":      adminName,
			"rejected_by_name": adminName,
			"rejected_at":      now,
			"reject_reason":    req.Reason,
			"updated_at":       now,
		}).Error; mirrorErr != nil {
			log.Warn("triage mirror failed (payment decision succeeded)", zap.Error(mirrorErr))
		}
		logAudit(auditSvc, c, "withdrawal.reject", "withdrawal", *w.PaymentUUID, fiber.Map{"reason": req.Reason})
		return c.JSON(fiber.Map{"success": true, "status": decided.Status})
	})

	// Hold for review
	withdrawals.Post("/:id/hold", func(c *fiber.Ctx) error {
		var req struct {
			Reason string `json:"reason"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		now := time.Now()
		db.Model(&models.Withdrawal{}).Where("id = ?", c.Params("id")).Updates(map[string]any{
			"status":      "held",
			"hold_reason": req.Reason,
			"updated_at":  now,
		})
		return c.JSON(fiber.Map{"success": true})
	})

	// Auto-approve endpoint: triage pre-checks, then executes the decision in
	// Payment Service like a human approval (operator recorded as "system").
	withdrawals.Post("/:id/auto-approve", func(c *fiber.Ctx) error {
		var w models.Withdrawal
		if err := db.Where("id = ?", c.Params("id")).First(&w).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(404).JSON(fiber.Map{"error": "not found"})
			}
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		// Check auto-approve conditions
		if !(w.RiskScore < 20 && w.KycStatus == "verified" && w.WagerStatus == "completed" && w.AmlStatus == "clear") {
			return c.Status(422).JSON(fiber.Map{"error": "auto-approve conditions not met"})
		}
		if w.PaymentUUID == nil || *w.PaymentUUID == "" {
			return c.Status(422).JSON(fiber.Map{"error": "triage row is not linked to a payment withdrawal"})
		}
		adminID := adminIDInt64(c)
		if adminID == nil {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		decided, err := financeSvc.ApproveWithdrawal(c.Context(), *adminID, "system:auto-approve", *w.PaymentUUID)
		if err != nil {
			log.Error("auto-approve failed", zap.Error(err), zap.String("payment_uuid", *w.PaymentUUID))
			return c.Status(422).JSON(fiber.Map{"error": err.Error()})
		}
		now := time.Now()
		if mirrorErr := db.Model(&models.Withdrawal{}).Where("id = ?", w.ID).Updates(map[string]any{
			"status":           "approved",
			"approved_by":      "system",
			"approved_by_name": "auto",
			"approved_at":      now,
			"updated_at":       now,
		}).Error; mirrorErr != nil {
			log.Warn("triage mirror failed (payment decision succeeded)", zap.Error(mirrorErr))
		}
		logAudit(auditSvc, c, "withdrawal.auto_approve", "withdrawal", *w.PaymentUUID, fiber.Map{"status": decided.Status})
		return c.JSON(fiber.Map{"success": true, "auto_approved": true})
	})
}
