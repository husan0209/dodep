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

func RegisterFinanceRoutes(router fiber.Router, svc *service.FinanceService, db *gorm.DB, log *zap.Logger) {
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

	// The withdrawals review queue is registered by RegisterWithdrawalRoutes
	// under /admin/finance/withdrawals (and the legacy /admin/withdrawals
	// namespace) — do NOT also register GET /withdrawals here: the later
	// registration wins, which makes this endpoint's behavior depend on
	// route ordering in main.go.

	// Financial summary for dashboard — computed from the shared payments and
	// withdrawals tables (migrations 014/022).
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
			totalDeposits += parseMoney(d.FiatAmount)
		}
		for _, w := range withdrawals {
			amt := parseMoney(w.FiatAmount)
			totalWithdrawals += amt
			if w.Status == "pending_review" || w.Status == "held" {
				pendingWithdrawalsCount++
				pendingWithdrawalsAmount += amt
			}
		}
		netRev := totalDeposits - totalWithdrawals
		return c.JSON(fiber.Map{"data": fiber.Map{
			"total_deposits":           formatMoney(totalDeposits),
			"total_withdrawals":        formatMoney(totalWithdrawals),
			"net_revenue":              formatMoney(netRev),
			"ggr":                      formatMoney(netRev),
			"pending_withdrawals_count": pendingWithdrawalsCount,
			"pending_withdrawals_amount": formatMoney(pendingWithdrawalsAmount),
		}})
	})
}

func parseTransactionStatus(s string) commonv1.TransactionStatus {
	v, err := strconv.Atoi(s)
	if err != nil {
		return commonv1.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED
	}
	return commonv1.TransactionStatus(v)
}
