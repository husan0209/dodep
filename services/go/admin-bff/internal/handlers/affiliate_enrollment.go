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

// Zone owner: affiliate enrollment (approve / reject / suspend) and the
// affiliate id resolution used by the routes in affiliate_routes.go.
//
// Extracted from affiliates.go so concurrent agents editing the affiliate CRUD
// file cannot silently drop this code.

// rejectAffiliate moves an enrollment application to rejected. The review
// reason is mandatory and is preserved in the audit trail (the affiliates
// table has no notes column by design; audit log is the source of truth).
func rejectAffiliate(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		idOrUserID := c.Params("id")
		var body struct {
			ReviewNotes string `json:"review_notes"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if strings.TrimSpace(body.ReviewNotes) == "" {
			return c.Status(400).JSON(fiber.Map{"error": "review_notes is required"})
		}
		realID, err := resolveAffiliateID(db, idOrUserID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(404).JSON(fiber.Map{"error": "not found"})
			}
			log.Error("resolve affiliate failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		if err := db.Model(&models.Affiliate{}).Where("id = ?", realID).Updates(map[string]interface{}{
			"status": "rejected", "updated_at": time.Now(),
		}).Error; err != nil {
			log.Error("reject affiliate failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "affiliate.reject", "affiliate", realID,
			fiber.Map{"review_notes": body.ReviewNotes})
		return c.JSON(fiber.Map{"success": true})
	}
}

// resolveAffiliateParam rewrites the :id route parameter to the canonical
// affiliate id before the wrapped handler runs.
//
// The admin SPA addresses affiliates by player user_id (that is what the
// affiliate list returns), while the task contract and the reporting
// endpoints use the affiliate id. Rather than duplicating every handler, the
// id is normalised once here; when :id is already an affiliate id the lookup
// matches on the first query and the parameter is left untouched.
func resolveAffiliateParam(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		idOrUserID := c.Params("id")
		realID, err := resolveAffiliateID(db, idOrUserID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(404).JSON(fiber.Map{"error": "not found"})
			}
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		c.Params("id", realID)
		return c.Next()
	}
}

// resolveAffiliateID accepts either the affiliate UUID or the player user_id.
// The affiliate id wins when both match. Returns gorm.ErrRecordNotFound when
// neither matches, so callers can answer 404 instead of silently updating
// zero rows and reporting success.
func resolveAffiliateID(db *gorm.DB, idOrUserID string) (string, error) {
	var count int64
	if err := db.Model(&models.Affiliate{}).Where("id = ?", idOrUserID).Count(&count).Error; err != nil {
		return "", err
	}
	if count > 0 {
		return idOrUserID, nil
	}
	var aff models.Affiliate
	if err := db.Where("user_id = ?", idOrUserID).First(&aff).Error; err != nil {
		return "", err
	}
	return aff.ID, nil
}