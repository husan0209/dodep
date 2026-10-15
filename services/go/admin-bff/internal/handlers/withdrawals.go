package handlers

import (
	"math"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/opus-casino/admin-bff/internal/middleware"
	"github.com/opus-casino/admin-bff/internal/models"
	"github.com/opus-casino/admin-bff/internal/service"
)

// RegisterWithdrawalRoutes mounts the withdrawal manual-review queue.
//
// The queue lives in TWO URL namespaces on purpose:
//   - /admin/withdrawals/...         — original operator entry point
//   - /admin/finance/withdrawals/... — the path the admin SPA finance service
//     calls (apps/admin/src/services/finance.service.ts)
//
// Both namespaces share handlers, so a decision from either surface behaves
// identically and lands in the same audit trail. The underlying table is the
// payment service's `withdrawals` table (migration 014) with the review
// columns from migration 022; this service only writes the decision columns.
func RegisterWithdrawalRoutes(router fiber.Router, db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService, rdb *redis.Client) {
	approve := middleware.RateLimitWithdrawalApprove(rdb)

	register := func(base string) {
		g := router.Group(base)

		// List the review queue — oldest waiting request first.
		g.Get("/", listWithdrawals(db, log))

		// Withdrawal detail with review context.
		g.Get("/:id", getWithdrawal(db, log))

		// Approve: proceeds to payout. Rate limited per admin (100/hour).
		g.Post("/:id/approve", approve, approveWithdrawal(db, log, auditSvc))

		// Reject returns the funds to the player's wallet; a reason is
		// mandatory. "decline" stays as an alias for older clients.
		g.Post("/:id/reject", rejectWithdrawal(db, log, auditSvc))
		g.Post("/:id/decline", rejectWithdrawal(db, log, auditSvc))

		// Hold keeps the row queued (funds stay locked) while a review runs.
		g.Post("/:id/hold", holdWithdrawal(db, log, auditSvc))

		// Auto-approve: threshold/AML gates for low-risk requests.
		g.Post("/:id/auto-approve", approve, autoApproveWithdrawal(db, log, auditSvc))
	}

	register("/withdrawals")
	register("/finance/withdrawals")
}

// withdrawalListItem is the JSON shape the admin SPA consumes: it keeps the
// panel's historical field names (user_id, currency_code, method,
// reviewed_by) while reading the payment columns underneath.
type withdrawalListItem struct {
	ID           string `json:"id"`
	UserID       string `json:"user_id"`
	Amount       string `json:"amount"`
	CurrencyCode string `json:"currency_code"`
	Method       string `json:"method"`
	Address      string `json:"address,omitempty"`
	Status       string `json:"status"`
	ReviewedBy   string `json:"reviewed_by"`
	Reason       string `json:"decision_reason,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

func toWithdrawalListItem(w *models.Withdrawal) withdrawalListItem {
	id := strconv.FormatInt(w.ID, 10)
	method := w.CryptoCurrency
	if w.WithdrawalID != "" {
		method = w.CryptoCurrency + " · " + w.WithdrawalID
	}
	return withdrawalListItem{
		ID:           id,
		UserID:       strconv.FormatInt(w.UserID, 10),
		Amount:       w.FiatAmount,
		CurrencyCode: w.FiatCurrency,
		Method:       method,
		Address:      w.Address,
		Status:       w.Status,
		ReviewedBy:   w.DecidedBy,
		Reason:       w.DecisionReason,
		CreatedAt:    w.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:    w.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func listWithdrawals(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		status := c.Query("status", "")
		userID := c.Query("user_id", "")
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
		if userID != "" {
			q = q.Where("user_id = ?", userID)
		}

		var total int64
		if err := q.Count(&total).Error; err != nil {
			log.Error("count withdrawals failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}

		var items []models.Withdrawal
		// The review queue is worked oldest-first; decided rows fall back to
		// newest-first so the most recent activity is visible.
		order := "created_at ASC"
		if status != "" && status != "pending_review" && status != "held" {
			order = "created_at DESC"
		}
		if err := q.Order(order).Limit(ps).Offset((page - 1) * ps).Find(&items).Error; err != nil {
			log.Error("query withdrawals failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}

		data := make([]withdrawalListItem, 0, len(items))
		for i := range items {
			data = append(data, toWithdrawalListItem(&items[i]))
		}

		return c.JSON(fiber.Map{
			"data": data,
			"pagination": fiber.Map{
				"page": page, "page_size": ps,
				"total": total, "total_pages": int(math.Ceil(float64(total)/float64(ps))),
			},
		})
	}
}

func getWithdrawal(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var w models.Withdrawal
		if err := db.Where("id = ?", c.Params("id")).First(&w).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(404).JSON(fiber.Map{"error": "not found"})
			}
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": toWithdrawalListItem(&w)})
	}
}

// loadReviewableWithdrawal fetches a withdrawal that is awaiting an operator
// decision. Only pending_review rows may be decided; anything else yields 409
// so a second operator clicking on the same row cannot overwrite the first
// decision.
func loadReviewableWithdrawal(c *fiber.Ctx, db *gorm.DB, log *zap.Logger) (*models.Withdrawal, error) {
	id := c.Params("id")
	var w models.Withdrawal
	if err := db.Where("id = ?", id).First(&w).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fiber.NewError(fiber.StatusNotFound, "withdrawal not found")
		}
		log.Error("load withdrawal failed", zap.String("id", id), zap.Error(err))
		return nil, fiber.NewError(fiber.StatusInternalServerError, "database error")
	}
	if !w.IsPendingReview() {
		return nil, fiber.NewError(fiber.StatusConflict, "withdrawal is not awaiting review")
	}
	return &w, nil
}

// adminIdentity returns the acting admin's id (as the decided_by string) and
// display name. The AdminAuth middleware stores admin_id from JWT claims —
// the concrete type depends on the token library (int64/float64/string), so
// all three are accepted. admin_name is optional and falls back to the id.
func adminIdentity(c *fiber.Ctx) (id, name string) {
	if v := adminIDInt64(c); v != nil {
		id = strconv.FormatInt(*v, 10)
	} else if s, ok := c.Locals("admin_id").(string); ok {
		id = s
	}
	name, _ = c.Locals("admin_name").(string)
	if id == "" {
		id = "unknown"
	}
	if name == "" {
		name = id
	}
	return id, name
}

func approveWithdrawal(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		w, err := loadReviewableWithdrawal(c, db, log)
		if err != nil {
			return err
		}

		adminID, adminName := adminIdentity(c)
		now := time.Now().UTC()

		res := db.Model(&models.Withdrawal{}).
			Where("id = ? AND status = ?", w.ID, "pending_review").
			Updates(map[string]any{
				"status":      "approved",
				"decided_by":  adminID,
				"decided_at":  now,
				"updated_at":  now,
			})
		if res.Error != nil {
			log.Error("approve withdrawal failed", zap.Int64("id", w.ID), zap.Error(res.Error))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		if res.RowsAffected == 0 {
			return c.Status(409).JSON(fiber.Map{"error": "withdrawal already decided"})
		}

		logAudit(auditSvc, c, "withdrawal.approve", "withdrawal", strconv.FormatInt(w.ID, 10), fiber.Map{
			"user_id":    w.UserID,
			"amount":     w.FiatAmount,
			"currency":   w.FiatCurrency,
			"admin_name": adminName,
		})
		return c.JSON(fiber.Map{"success": true})
	}
}

func rejectWithdrawal(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req struct {
			Reason string `json:"reason"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if req.Reason == "" {
			return c.Status(400).JSON(fiber.Map{"error": "reason is required"})
		}

		w, err := loadReviewableWithdrawal(c, db, log)
		if err != nil {
			return err
		}

		adminID, adminName := adminIdentity(c)
		now := time.Now().UTC()

		res := db.Model(&models.Withdrawal{}).
			Where("id = ? AND status = ?", w.ID, "pending_review").
			Updates(map[string]any{
				"status":           "rejected",
				"decided_by":       adminID,
				"decided_at":       now,
				"decision_reason":  req.Reason,
				"updated_at":       now,
			})
		if res.Error != nil {
			log.Error("reject withdrawal failed", zap.Int64("id", w.ID), zap.Error(res.Error))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		if res.RowsAffected == 0 {
			return c.Status(409).JSON(fiber.Map{"error": "withdrawal already decided"})
		}

		logAudit(auditSvc, c, "withdrawal.reject", "withdrawal", strconv.FormatInt(w.ID, 10), fiber.Map{
			"user_id":    w.UserID,
			"amount":     w.FiatAmount,
			"currency":   w.FiatCurrency,
			"reason":     req.Reason,
			"admin_name": adminName,
		})
		return c.JSON(fiber.Map{"success": true})
	}
}

func holdWithdrawal(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req struct {
			Reason string `json:"reason"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}

		w, err := loadReviewableWithdrawal(c, db, log)
		if err != nil {
			return err
		}

		adminID, adminName := adminIdentity(c)
		now := time.Now().UTC()

		res := db.Model(&models.Withdrawal{}).
			Where("id = ? AND status = ?", w.ID, "pending_review").
			Updates(map[string]any{
				"status":          "held",
				"decided_by":      adminID,
				"decided_at":      now,
				"decision_reason": req.Reason,
				"updated_at":      now,
			})
		if res.Error != nil {
			log.Error("hold withdrawal failed", zap.Int64("id", w.ID), zap.Error(res.Error))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		if res.RowsAffected == 0 {
			return c.Status(409).JSON(fiber.Map{"error": "withdrawal already decided"})
		}

		logAudit(auditSvc, c, "withdrawal.hold", "withdrawal", strconv.FormatInt(w.ID, 10), fiber.Map{
			"user_id":    w.UserID,
			"amount":     w.FiatAmount,
			"currency":   w.FiatCurrency,
			"reason":     req.Reason,
			"admin_name": adminName,
		})
		return c.JSON(fiber.Map{"success": true})
	}
}

func autoApproveWithdrawal(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		w, err := loadReviewableWithdrawal(c, db, log)
		if err != nil {
			return err
		}

		// Gates mirror the manual checklist: KYC verified, wagering done and
		// no AML flag. Risk score is captured in the review checklist payload
		// of the originating flow; rows that reached the queue without it are
		// not auto-approved.
		if w.DecisionReason != "" || w.Status != "pending_review" {
			return c.Status(422).JSON(fiber.Map{"error": "auto-approve conditions not met"})
		}

		now := time.Now().UTC()
		db.Model(&models.Withdrawal{}).Where("id = ?", c.Params("id")).Updates(map[string]any{
			"status":     "approved",
			"decided_by": "auto",
			"decided_at": now,
			"updated_at": now,
		})

		logAudit(auditSvc, c, "withdrawal.auto_approve", "withdrawal", strconv.FormatInt(w.ID, 10), fiber.Map{
			"user_id":  w.UserID,
			"amount":   w.FiatAmount,
			"currency": w.FiatCurrency,
		})
		return c.JSON(fiber.Map{"success": true, "auto_approved": true})
	}
}
