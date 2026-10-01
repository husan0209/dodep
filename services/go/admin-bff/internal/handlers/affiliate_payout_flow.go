package handlers

import (
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/opus-casino/admin-bff/internal/models"
	"github.com/opus-casino/admin-bff/internal/service"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Zone owner: affiliate payout money flow.
// payout approve/reject/paid state machine
// Extracted from affiliates.go so concurrent agents editing the
// affiliate CRUD file cannot silently drop this code.

// validPayoutTransitions encodes the affiliate payout state machine
// (models.AffiliatePayout.Status, admin schema payout_status):
//
//	pending  → approved | rejected
//	approved → paid
//	rejected → terminal
//	paid     → terminal
//
// A payout can no longer be rejected once money has been released, and an
// already-rejected/paid payout must not be re-opened: that is a compliance
// boundary, not a UI nicety.
var validPayoutTransitions = map[string][]string{
	"pending":  {"approved", "rejected"},
	"approved": {"paid"},
}

// transitionPayout applies a guarded status transition to a payout.
// Returns the previous status on success so callers can audit it.
// 404 when the payout does not exist, 409 on an illegal transition or when a
// concurrent reviewer already moved the row.
func transitionPayout(
	c *fiber.Ctx,
	db *gorm.DB,
	id, target string,
	extra map[string]interface{},
) (string, error) {
	var payout models.AffiliatePayout
	if err := db.First(&payout, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", errAffiliatePayoutNotFound
		}
		return "", err
	}
	allowed, known := validPayoutTransitions[payout.Status]
	if !known || !containsString(allowed, target) {
		return "", errInvalidPayoutTransition
	}

	updates := map[string]interface{}{
		"status":     target,
		"updated_at": time.Now(),
	}
	for k, v := range extra {
		updates[k] = v
	}
	// Guard on the observed status so two reviewers cannot both transition.
	res := db.Model(&models.AffiliatePayout{}).
		Where("id = ? AND status = ?", id, payout.Status).
		Updates(updates)
	if res.Error != nil {
		return "", res.Error
	}
	if res.RowsAffected == 0 {
		return "", errPayoutAlreadyTransitioned
	}
	return payout.Status, nil
}

var (
	errAffiliatePayoutNotFound   = errors.New("affiliate payout not found")
	errInvalidPayoutTransition   = errors.New("invalid payout status transition")
	errPayoutAlreadyTransitioned = errors.New("payout already transitioned by another reviewer")
)

func approvePayout(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var body struct {
			ProviderReference string `json:"provider_reference"`
		}
		if err := c.BodyParser(&body); err != nil {
			body.ProviderReference = ""
		}
		prev, err := transitionPayout(c, db, id, "approved", map[string]interface{}{
			"provider_reference": body.ProviderReference,
		})
		if err != nil {
			return writePayoutTransitionError(c, log, err)
		}
		logAudit(auditSvc, c, "affiliate.payout.approve", "affiliate_payout", id,
			fiber.Map{"provider_reference": body.ProviderReference, "previous_status": prev})
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
		if strings.TrimSpace(body.Reason) == "" {
			return c.Status(400).JSON(fiber.Map{"error": "rejection_reason is required"})
		}
		prev, err := transitionPayout(c, db, id, "rejected", map[string]interface{}{
			"rejection_reason": body.Reason,
		})
		if err != nil {
			return writePayoutTransitionError(c, log, err)
		}
		logAudit(auditSvc, c, "affiliate.payout.reject", "affiliate_payout", id,
			fiber.Map{"reason": body.Reason, "previous_status": prev})
		return c.JSON(fiber.Map{"success": true})
	}
}

// markPayoutPaid releases an approved payout to the partner. It is separate
// from approve on purpose: approve is a compliance decision, paid is the
// money movement (payout_service in the affiliate domain).
func markPayoutPaid(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var body struct {
			ProviderReference string `json:"provider_reference"`
		}
		if err := c.BodyParser(&body); err != nil {
			body.ProviderReference = ""
		}
		prev, err := transitionPayout(c, db, id, "paid", map[string]interface{}{
			"provider_reference": body.ProviderReference,
		})
		if err != nil {
			return writePayoutTransitionError(c, log, err)
		}
		logAudit(auditSvc, c, "affiliate.payout.paid", "affiliate_payout", id,
			fiber.Map{"provider_reference": body.ProviderReference, "previous_status": prev})
		return c.JSON(fiber.Map{"success": true})
	}
}

func writePayoutTransitionError(c *fiber.Ctx, log *zap.Logger, err error) error {
	switch {
	case errors.Is(err, errAffiliatePayoutNotFound):
		return c.Status(404).JSON(fiber.Map{"error": "not found"})
	case errors.Is(err, errInvalidPayoutTransition):
		return c.Status(409).JSON(fiber.Map{"error": "invalid payout status transition"})
	case errors.Is(err, errPayoutAlreadyTransitioned):
		return c.Status(409).JSON(fiber.Map{"error": "payout already transitioned by another reviewer"})
	default:
		log.Error("payout transition failed", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{"error": "database error"})
	}
}
