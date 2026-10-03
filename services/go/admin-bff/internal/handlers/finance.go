package handlers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/opus-casino/admin-bff/internal/models"
	"github.com/opus-casino/admin-bff/internal/service"
	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"
)

func RegisterFinanceRoutes(router fiber.Router, svc *service.FinanceService, auditSvc *service.AuditService, db *gorm.DB, log *zap.Logger) {
	finance := router.Group("/finance")

	finance.Get("/deposits", func(c *fiber.Ctx) error {
		status := parseTransactionStatus(c.Query("status", ""))
		pageSize := int32(c.QueryInt("page_size", 50))
		pageToken := c.Query("page_token", "")
		items, page, err := svc.ListDeposits(c.Context(), status, pageSize, pageToken)
		if err != nil {
			log.Error("list deposits failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "failed to list deposits"})
		}
		return c.JSON(fiber.Map{"data": items, "pagination": page})
	})

	// Review queue. Omitting player_id returns every user's withdrawals,
	// which is what the finance back office needs; the money itself and the
	// approve/reject decisions live in Payment Service.
	finance.Get("/withdrawals", func(c *fiber.Ctx) error {
		status := parseTransactionStatus(c.Query("status", ""))
		pageSize := int32(c.QueryInt("page_size", 50))
		if pageSize < 1 || pageSize > 200 {
			pageSize = 50
		}
		pageToken := c.Query("page_token", "")
		playerID := int64(0)
		if raw := c.Query("player_id", ""); raw != "" {
			parsed, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || parsed <= 0 {
				return c.Status(400).JSON(fiber.Map{"error": "invalid player_id"})
			}
			playerID = parsed
		}
		items, page, err := svc.ListWithdrawals(c.Context(), playerID, status, pageSize, pageToken)
		if err != nil {
			log.Error("list withdrawals failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "failed to list withdrawals"})
		}
		return c.JSON(fiber.Map{"data": items, "pagination": page})
	})

	// Single withdrawal (source of truth in Payment Service).
	finance.Get("/withdrawals/:id", func(c *fiber.Ctx) error {
		view, err := svc.GetWithdrawal(c.Context(), c.Params("id"))
		if err != nil {
			log.Error("get withdrawal failed", zap.Error(err), zap.String("id", c.Params("id")))
			return c.Status(404).JSON(fiber.Map{"error": "withdrawal not found"})
		}
		return c.JSON(fiber.Map{"data": view})
	})

	// Approve a withdrawal awaiting manual review. The decision executes in
	// Payment Service: it claims the request, executes the PSP payout and
	// settles the wallet reservation.
	finance.Post("/withdrawals/:id/approve", func(c *fiber.Ctx) error {
		adminID := adminIDInt64(c)
		if adminID == nil {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		view, err := svc.ApproveWithdrawal(c.Context(), *adminID, adminIDString(c), c.Params("id"))
		if err != nil {
			log.Error("approve withdrawal failed", zap.Error(err), zap.String("id", c.Params("id")))
			return c.Status(422).JSON(fiber.Map{"error": err.Error()})
		}
		logAudit(auditSvc, c, "withdrawal.approve", "withdrawal", c.Params("id"), fiber.Map{"status": view.Status})
		return c.JSON(fiber.Map{"success": true, "data": view})
	})

	// Reject a withdrawal awaiting manual review; the reserved funds are
	// returned to the player by Payment Service.
	finance.Post("/withdrawals/:id/reject", func(c *fiber.Ctx) error {
		var req struct {
			Reason string `json:"reason"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if req.Reason == "" {
			return c.Status(400).JSON(fiber.Map{"error": "reason is required"})
		}
		adminID := adminIDInt64(c)
		if adminID == nil {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		view, err := svc.RejectWithdrawal(c.Context(), *adminID, adminIDString(c), c.Params("id"), req.Reason)
		if err != nil {
			log.Error("reject withdrawal failed", zap.Error(err), zap.String("id", c.Params("id")))
			return c.Status(422).JSON(fiber.Map{"error": err.Error()})
		}
		logAudit(auditSvc, c, "withdrawal.reject", "withdrawal", c.Params("id"), fiber.Map{"reason": req.Reason})
		return c.JSON(fiber.Map{"success": true, "data": view})
	})

	// Financial summary for dashboard — computed from local deposits/withdrawals tables
	finance.Get("/summary", func(c *fiber.Ctx) error {
		dateFrom := c.Query("date_from", "")
		dateTo := c.Query("date_to", "")
		var start, end time.Time
		if dateFrom != "" {
			start, _ = time.Parse("2006-01-02", dateFrom)
		}
		if dateTo != "" {
			end, _ = time.Parse("2006-01-02", dateTo)
			end = end.Add(24 * time.Hour)
		}
		if dateFrom == "" {
			start = time.Now().AddDate(0, 0, -30)
		}
		if dateTo == "" {
			end = time.Now().Add(24 * time.Hour)
		}

		var deposits []models.Deposit
		var withdrawals []models.Withdrawal
		db.Where("created_at >= ? AND created_at < ?", start, end).Find(&deposits)
		db.Where("created_at >= ? AND created_at < ?", start, end).Find(&withdrawals)

		var totalDeposits, totalWithdrawals, pendingWithdrawalsAmount float64
		var pendingWithdrawalsCount int64
		for _, d := range deposits {
			totalDeposits += parseMoney(d.Amount)
		}
		for _, w := range withdrawals {
			amt := parseMoney(w.Amount)
			totalWithdrawals += amt
			if w.Status == "pending" || w.Status == "processing" {
				pendingWithdrawalsCount++
				pendingWithdrawalsAmount += amt
			}
		}
		netRev := totalDeposits - totalWithdrawals
		return c.JSON(fiber.Map{"data": fiber.Map{
			"total_deposits":             formatMoney(totalDeposits),
			"total_withdrawals":          formatMoney(totalWithdrawals),
			"net_revenue":                formatMoney(netRev),
			"ggr":                        formatMoney(netRev),
			"pending_withdrawals_count":  pendingWithdrawalsCount,
			"pending_withdrawals_amount": formatMoney(pendingWithdrawalsAmount),
		}})
	})
}

// parseTransactionStatus accepts either the numeric proto value or the
// status names the admin UI displays (Payment Service maps withdrawal states
// such as pending_review onto TRANSACTION_STATUS_PENDING).
func parseTransactionStatus(s string) commonv1.TransactionStatus {
	s = strings.TrimSpace(s)
	if s == "" {
		return commonv1.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED
	}
	if v, err := strconv.Atoi(s); err == nil {
		return commonv1.TransactionStatus(v)
	}
	switch strings.ToLower(s) {
	case "pending", "pending_review", "requires_review", "approved":
		return commonv1.TransactionStatus_TRANSACTION_STATUS_PENDING
	case "processing", "sending", "sent":
		return commonv1.TransactionStatus_TRANSACTION_STATUS_PROCESSING
	case "completed", "finished":
		return commonv1.TransactionStatus_TRANSACTION_STATUS_COMPLETED
	case "failed", "expired":
		return commonv1.TransactionStatus_TRANSACTION_STATUS_FAILED
	case "cancelled", "rejected":
		return commonv1.TransactionStatus_TRANSACTION_STATUS_CANCELLED
	default:
		return commonv1.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED
	}
}
