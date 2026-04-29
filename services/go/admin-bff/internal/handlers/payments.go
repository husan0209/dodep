package handlers

import (
	"math"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/opus-casino/admin-bff/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func RegisterPaymentRoutes(router fiber.Router, db *gorm.DB, log *zap.Logger) {
	pm := router.Group("/payments")

	// Chargeback webhook (internal endpoint from payment gateway)
	pm.Post("/internal/webhooks/chargeback", func(c *fiber.Ctx) error {
		var req struct {
			PlayerID      int64   `json:"player_id"`
			TransactionID string  `json:"transaction_id"`
			Amount        string  `json:"amount"`
			Currency      string  `json:"currency"`
			Gateway       string  `json:"gateway"`
			GatewayCbID   string  `json:"gateway_cb_id"`
			ReasonCode    string  `json:"reason_code"`
			ReasonText    string  `json:"reason_text"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		cb := models.Chargeback{
			PlayerID:      req.PlayerID,
			TransactionID: req.TransactionID,
			Amount:        req.Amount,
			Currency:      req.Currency,
			Gateway:       req.Gateway,
			GatewayCbID:   &req.GatewayCbID,
			ReasonCode:    &req.ReasonCode,
			ReasonText:    &req.ReasonText,
			Status:        "received",
			ReceivedAt:    time.Now(),
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		if err := db.Create(&cb).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		// Auto-actions (placeholder — real integration with player/wallet services)
		autoActions := fiber.Map{
			"block_player":       true,
			"freeze_withdrawals": true,
			"add_tag":            "chargeback",
			"add_risk_score":     25,
			"notify_role":        "FINANCE_MANAGER",
		}
		log.Info("Chargeback auto-actions triggered",
			zap.String("chargeback_id", cb.ID),
			zap.Int64("player_id", req.PlayerID),
			zap.Any("actions", autoActions),
		)
		return c.Status(201).JSON(fiber.Map{"success": true, "chargeback_id": cb.ID, "auto_actions": autoActions})
	})

	// Chargebacks
	pm.Get("/chargebacks", func(c *fiber.Ctx) error {
		status := c.Query("status", "")
		assignedTo := c.Query("assigned_to", "")
		page := c.QueryInt("page", 1)
		ps := c.QueryInt("page_size", 50)
		if page < 1 { page = 1 }
		if ps < 1 || ps > 200 { ps = 50 }
		q := db.Model(&models.Chargeback{})
		if status != "" { q = q.Where("status = ?", status) }
		if assignedTo != "" { q = q.Where("assigned_to = ?", assignedTo) }
		var total int64
		q.Count(&total)
		var items []models.Chargeback
		q.Order("received_at DESC").Limit(ps).Offset((page - 1) * ps).Find(&items)
		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{"page": page, "page_size": ps, "total": total, "total_pages": int(math.Ceil(float64(total)/float64(ps)))},
		})
	})

	pm.Get("/chargebacks/:id", func(c *fiber.Ctx) error {
		var cb models.Chargeback
		if err := db.Where("id = ?", c.Params("id")).First(&cb).Error; err != nil {
			if err == gorm.ErrRecordNotFound { return c.Status(404).JSON(fiber.Map{"error": "not found"}) }
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": cb})
	})

	pm.Put("/chargebacks/:id/assign", func(c *fiber.Ctx) error {
		var req struct { AssignedTo string `json:"assigned_to"` }
		if err := c.BodyParser(&req); err != nil { return c.Status(400).JSON(fiber.Map{"error": "invalid body"}) }
		db.Model(&models.Chargeback{}).Where("id = ?", c.Params("id")).Updates(map[string]any{
			"assigned_to": req.AssignedTo, "assigned_to_name": req.AssignedTo,
		})
		return c.JSON(fiber.Map{"success": true})
	})

	pm.Post("/chargebacks/:id/fight", func(c *fiber.Ctx) error {
		var req struct { Evidence []any `json:"evidence"` }
		if err := c.BodyParser(&req); err != nil { return c.Status(400).JSON(fiber.Map{"error": "invalid body"}) }
		db.Model(&models.Chargeback{}).Where("id = ?", c.Params("id")).Updates(map[string]any{
			"status": "fighting", "fight_evidence": req.Evidence,
		})
		return c.JSON(fiber.Map{"success": true})
	})

	pm.Post("/chargebacks/:id/accept", func(c *fiber.Ctx) error {
		db.Model(&models.Chargeback{}).Where("id = ?", c.Params("id")).Updates(map[string]any{
			"status": "accepted", "resolved_at": time.Now(),
		})
		return c.JSON(fiber.Map{"success": true})
	})

	pm.Post("/chargebacks/:id/won", func(c *fiber.Ctx) error {
		db.Model(&models.Chargeback{}).Where("id = ?", c.Params("id")).Updates(map[string]any{
			"status": "won", "resolved_at": time.Now(),
		})
		return c.JSON(fiber.Map{"success": true})
	})

	pm.Get("/chargebacks/stats", func(c *fiber.Ctx) error {
		var totalThisMonth int64
		var amountThisMonth string
		var foughtCount int64
		var wonCount int64
		today := time.Now()
		firstDay := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
		db.Model(&models.Chargeback{}).Where("received_at >= ?", firstDay).Count(&totalThisMonth)
		db.Raw("SELECT COALESCE(SUM(amount),0)::text FROM chargebacks WHERE received_at >= ?", firstDay).Scan(&amountThisMonth)
		db.Model(&models.Chargeback{}).Where("received_at >= ? AND status IN ('fighting','won','lost')", firstDay).Count(&foughtCount)
		db.Model(&models.Chargeback{}).Where("received_at >= ? AND status = 'won'", firstDay).Count(&wonCount)
		cbRate := 0.0
		fightWinRate := 0.0
		if foughtCount > 0 {
			fightWinRate = float64(wonCount) / float64(foughtCount) * 100.0
		}
		return c.JSON(fiber.Map{"data": fiber.Map{
			"total_this_month":    totalThisMonth,
			"amount_this_month":   amountThisMonth,
			"cb_rate_pct":         cbRate,
			"fight_win_rate_pct":  fightWinRate,
			"fought_count":        foughtCount,
			"won_count":           wonCount,
		}})
	})

	// Balance sheet
	pm.Get("/balance-sheet", func(c *fiber.Ctx) error {
		var totalDeposits, totalWithdrawals, totalChargebacks, platformHold string
		var playerBalances, bonusBalances, pendingWithdrawals string
		db.Raw("SELECT COALESCE(SUM(amount),0)::text FROM p2p_transactions WHERE type='deposit' AND status='completed'").Scan(&totalDeposits)
		db.Raw("SELECT COALESCE(SUM(amount),0)::text FROM p2p_transactions WHERE type='withdrawal' AND status='completed'").Scan(&totalWithdrawals)
		db.Raw("SELECT COALESCE(SUM(amount),0)::text FROM chargebacks WHERE status IN ('received','under_review','fighting')").Scan(&totalChargebacks)
		db.Raw("SELECT COALESCE(SUM(balance),0)::text FROM crypto_wallets").Scan(&platformHold)
		// Liabilities (placeholders — real data from wallet/player services)
		db.Raw("SELECT '0'::text").Scan(&playerBalances)
		db.Raw("SELECT '0'::text").Scan(&bonusBalances)
		db.Raw("SELECT COALESCE(SUM(amount),0)::text FROM withdrawals WHERE status IN ('pending','under_review','approved','held')").Scan(&pendingWithdrawals)
		coverageRatio := 1.0 // placeholder until real integration
		status := "OK"
		if coverageRatio < 1.0 {
			status = "CRITICAL"
		} else if coverageRatio < 1.2 {
			status = "WARNING"
		}
		return c.JSON(fiber.Map{"data": fiber.Map{
			"as_of": time.Now().UTC().Format(time.RFC3339),
			"liabilities": fiber.Map{
				"player_balances":     playerBalances,
				"bonus_balances":      bonusBalances,
				"pending_withdrawals": pendingWithdrawals,
				"total":               "0",
			},
			"assets": fiber.Map{
				"total_deposits":      totalDeposits,
				"total_withdrawals":   totalWithdrawals,
				"platform_hold":       platformHold,
				"total":               "0",
			},
			"coverage_ratio":     coverageRatio,
			"status":             status,
			"total_chargebacks":  totalChargebacks,
		}})
	})

	// Crypto wallets with IsLow calculation
	pm.Get("/crypto-wallets", func(c *fiber.Ctx) error {
		var items []models.CryptoWallet
		if err := db.Order("coin ASC").Find(&items).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		type enrichedCryptoWallet struct {
			models.CryptoWallet
			IsLow           bool   `json:"is_low"`
			HotWalletThreshold string `json:"hot_wallet_threshold"`
		}
		var enriched []enrichedCryptoWallet
		for _, w := range items {
			e := enrichedCryptoWallet{CryptoWallet: w, IsLow: false, HotWalletThreshold: "0"}
			if w.WalletType == "hot" {
				// Threshold = daily_withdrawal_avg * 0.20
				// IsLow = balance < threshold
				// simplified: always set based on placeholder calculation
				// Real decimal arithmetic requires proper decimal library
				e.IsLow = false // placeholder: implement with proper decimal comparison
			}
			enriched = append(enriched, e)
		}
		return c.JSON(fiber.Map{"data": enriched})
	})

	pm.Post("/crypto-wallets/:id/refresh", func(c *fiber.Ctx) error {
		db.Model(&models.CryptoWallet{}).Where("id = ?", c.Params("id")).Update("last_updated", time.Now())
		return c.JSON(fiber.Map{"success": true})
	})

	// P2P
	pm.Get("/p2p", func(c *fiber.Ctx) error {
		status := c.Query("status", "")
		typ := c.Query("type", "")
		page := c.QueryInt("page", 1)
		ps := c.QueryInt("page_size", 50)
		if page < 1 { page = 1 }
		if ps < 1 || ps > 200 { ps = 50 }
		q := db.Model(&models.P2PTransaction{})
		if status != "" { q = q.Where("status = ?", status) }
		if typ != "" { q = q.Where("type = ?", typ) }
		var total int64
		q.Count(&total)
		var items []models.P2PTransaction
		q.Order("created_at DESC").Limit(ps).Offset((page - 1) * ps).Find(&items)
		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{"page": page, "page_size": ps, "total": total, "total_pages": int(math.Ceil(float64(total)/float64(ps)))},
		})
	})

	pm.Post("/p2p/:id/confirm", func(c *fiber.Ctx) error {
		db.Model(&models.P2PTransaction{}).Where("id = ?", c.Params("id")).Updates(map[string]any{
			"status": "confirmed", "confirmed_at": time.Now(), "confirmed_by": "admin-id",
		})
		return c.JSON(fiber.Map{"success": true})
	})

	pm.Post("/p2p/:id/reject", func(c *fiber.Ctx) error {
		var req struct { Reason string `json:"reason"` }
		c.BodyParser(&req)
		db.Model(&models.P2PTransaction{}).Where("id = ?", c.Params("id")).Updates(map[string]any{
			"status": "rejected", "notes": req.Reason,
		})
		return c.JSON(fiber.Map{"success": true})
	})

	pm.Post("/p2p/:id/mark-sent", func(c *fiber.Ctx) error {
		db.Model(&models.P2PTransaction{}).Where("id = ?", c.Params("id")).Update("status", "sent")
		return c.JSON(fiber.Map{"success": true})
	})

	// Reconciliation
	pm.Get("/reconciliation", func(c *fiber.Ctx) error {
		from := c.Query("from", "")
		to := c.Query("to", "")
		page := c.QueryInt("page", 1)
		ps := c.QueryInt("page_size", 50)
		if page < 1 { page = 1 }
		if ps < 1 || ps > 200 { ps = 50 }
		q := db.Model(&models.ReconciliationRecord{})
		if from != "" { q = q.Where("recon_date >= ?", from) }
		if to != "" { q = q.Where("recon_date <= ?", to) }
		var total int64
		q.Count(&total)
		var items []models.ReconciliationRecord
		q.Order("recon_date DESC").Limit(ps).Offset((page - 1) * ps).Find(&items)
		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{"page": page, "page_size": ps, "total": total, "total_pages": int(math.Ceil(float64(total)/float64(ps)))},
		})
	})

	pm.Post("/reconciliation/run", func(c *fiber.Ctx) error {
		var req struct { Date string `json:"date"` }
		if err := c.BodyParser(&req); err != nil { return c.Status(400).JSON(fiber.Map{"error": "invalid body"}) }
		date, _ := time.Parse("2006-01-02", req.Date)
		rec := models.ReconciliationRecord{
			ReconDate:  date, Status: "pending",
			CreatedAt:  time.Now(), UpdatedAt: time.Now(),
		}
		db.Create(&rec)
		return c.JSON(fiber.Map{"success": true, "data": rec})
	})

	// Payment method configs
	pm.Get("/method-configs", func(c *fiber.Ctx) error {
		var items []models.PaymentMethodConfig
		if err := db.Order("country_code, priority DESC").Find(&items).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": items})
	})

	pm.Put("/method-configs/:id", func(c *fiber.Ctx) error {
		var req models.PaymentMethodConfig
		if err := c.BodyParser(&req); err != nil { return c.Status(400).JSON(fiber.Map{"error": "invalid body"}) }
		db.Model(&models.PaymentMethodConfig{}).Where("id = ?", c.Params("id")).Updates(&req)
		return c.JSON(fiber.Map{"success": true})
	})
}
