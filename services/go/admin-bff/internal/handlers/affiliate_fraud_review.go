package handlers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/opus-casino/admin-bff/internal/models"
	"github.com/opus-casino/admin-bff/internal/service"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Zone owner: affiliate fraud review workflow.
// fraud flag review workflow (resolve/dismiss)
// Extracted from affiliates.go so concurrent agents editing the
// affiliate CRUD file cannot silently drop this code.

// validFraudFlagTransitions encodes the review state machine:
//
//	open        → resolved | dismissed
//	in_review   → resolved | dismissed
//	resolved    → terminal
//	dismissed   → terminal
var validFraudFlagTransitions = map[string][]string{
	"open":      {"resolved", "dismissed"},
	"in_review": {"resolved", "dismissed"},
}

// resolveFraudFlagAction is the shared implementation behind
// POST /admin/affiliates/fraud-flags/:id/{resolve,dismiss}.
// The transition is guarded by the current status in the WHERE clause so two
// concurrent reviewers cannot both close the same flag (RowsAffected == 0 →
// 409). Review notes are mandatory for resolve (confirmed fraud) and optional
// for dismiss (false positive) — both are written to the audit trail because
// the fraud_flags table intentionally stores no free-text notes.
func resolveFraudFlagAction(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService, target string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var body struct {
			Notes string `json:"notes"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		notes := strings.TrimSpace(body.Notes)
		if target == "resolved" && notes == "" {
			return c.Status(400).JSON(fiber.Map{"error": "notes is required when resolving a flag"})
		}

		var flag models.FraudFlag
		if err := db.First(&flag, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(404).JSON(fiber.Map{"error": "not found"})
			}
			log.Error("get fraud flag failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		if allowed, ok := validFraudFlagTransitions[flag.Status]; !ok || !containsString(allowed, target) {
			return c.Status(409).JSON(fiber.Map{
				"error":          "invalid status transition",
				"current_status": flag.Status,
			})
		}

		res := db.Model(&models.FraudFlag{}).
			Where("id = ? AND status = ?", id, flag.Status).
			Updates(map[string]interface{}{
				"status":      target,
				"resolved_at": time.Now(),
				"resolved_by": adminIDInt64(c),
			})
		if res.Error != nil {
			log.Error("resolve fraud flag failed", zap.Error(res.Error))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		if res.RowsAffected == 0 {
			return c.Status(409).JSON(fiber.Map{"error": "flag already closed by another reviewer"})
		}

		logAudit(auditSvc, c, "affiliate.fraud_flag."+target, "affiliate_fraud_flag", id,
			fiber.Map{"notes": notes, "previous_status": flag.Status})
		return c.JSON(fiber.Map{"success": true})
	}
}

func resolveFraudFlag(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return resolveFraudFlagAction(db, log, auditSvc, "resolved")
}

func dismissFraudFlag(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return resolveFraudFlagAction(db, log, auditSvc, "dismissed")
}

func containsString(haystack []string, needle string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}
