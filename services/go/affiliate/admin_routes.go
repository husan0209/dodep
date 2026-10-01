package main

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/opus-casino/affiliate/internal/domain"
	"github.com/opus-casino/affiliate/internal/service"
)

// setupAdminRoutes registers admin-only REST API endpoints for affiliate management.
// The router is already prefixed with /admin and has AdminMiddleware applied.
func setupAdminRoutes(router fiber.Router, svc *service.AffiliateService) {
	affiliates := router.Group("/affiliates")

	// List all affiliate profiles (with pagination)
	affiliates.Get("/", func(c *fiber.Ctx) error {
		status := domain.AffiliateStatus(c.Query("status", ""))
		page, pageSize := parsePagination(c)
		offset := (page - 1) * pageSize

		profiles, total, err := svc.ListAffiliates(c.Context(), status, pageSize, offset)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "internal error"})
		}

		return c.JSON(paginatedResponse(profiles, total, page, pageSize))
	})

	// Approve affiliate enrollment
	affiliates.Post("/:user_id/approve", func(c *fiber.Ctx) error {
		adminUserID := getAdminUserID(c)
		userID, err := strconv.ParseInt(c.Params("user_id"), 10, 64)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid user_id"})
		}

		var req struct {
			CommissionPlanID string `json:"commission_plan_id"`
			CommissionRate   string `json:"commission_rate"`
			HoldPeriodDays   int    `json:"hold_period_days"`
			MinPayoutAmount  string `json:"min_payout_amount"`
			Currency         string `json:"currency"`
			RequireKYC       bool   `json:"require_kyc"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		planID := uuid.Nil
		if req.CommissionPlanID != "" {
			planID, _ = uuid.Parse(req.CommissionPlanID)
		}
		rate := decimal.RequireFromString("0.20") // default 20%
		if req.CommissionRate != "" {
			rate, _ = decimal.NewFromString(req.CommissionRate)
		}
		minPayout := decimal.RequireFromString("100")
		if req.MinPayoutAmount != "" {
			minPayout, _ = decimal.NewFromString(req.MinPayoutAmount)
		}
		holdDays := 14
		if req.HoldPeriodDays > 0 {
			holdDays = req.HoldPeriodDays
		}
		currency := "USD"
		if req.Currency != "" {
			currency = req.Currency
		}

		profile, err := svc.ApproveAffiliate(c.Context(), service.ApproveAffiliateInput{
			UserID:           userID,
			ApprovedBy:       adminUserID,
			CommissionPlanID: planID,
			CommissionRate:   rate,
			HoldPeriodDays:   holdDays,
			MinPayoutAmount:  minPayout,
			Currency:         currency,
			RequireKYC:       req.RequireKYC,
		})
		if err != nil {
			return handleDomainError(c, err)
		}

		return c.Status(201).JSON(profile)
	})

	// Reject affiliate enrollment
	affiliates.Post("/:user_id/reject", func(c *fiber.Ctx) error {
		adminUserID := getAdminUserID(c)
		userID, err := strconv.ParseInt(c.Params("user_id"), 10, 64)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid user_id"})
		}

		var req struct {
			ReviewNotes string `json:"review_notes"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		err = svc.RejectAffiliate(c.Context(), service.RejectAffiliateInput{
			UserID:      userID,
			RejectedBy:  adminUserID,
			ReviewNotes: req.ReviewNotes,
		})
		if err != nil {
			return handleDomainError(c, err)
		}

		return c.JSON(fiber.Map{"success": true})
	})

	// Suspend affiliate
	affiliates.Post("/:id/suspend", func(c *fiber.Ctx) error {
		affiliateID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid affiliate id"})
		}

		err = svc.SuspendAffiliate(c.Context(), affiliateID)
		if err != nil {
			return handleDomainError(c, err)
		}

		return c.JSON(fiber.Map{"success": true})
	})

	// Update commission rate
	affiliates.Put("/:id/commission-rate", func(c *fiber.Ctx) error {
		affiliateID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid affiliate id"})
		}

		var req struct {
			CommissionRate string `json:"commission_rate"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}
		rate, err := decimal.NewFromString(req.CommissionRate)
		if err != nil || rate.IsNegative() {
			return c.Status(400).JSON(fiber.Map{"error": "invalid commission_rate"})
		}

		err = svc.UpdateCommissionRate(c.Context(), affiliateID, rate)
		if err != nil {
			return handleDomainError(c, err)
		}

		return c.JSON(fiber.Map{"success": true})
	})

	// Create manual adjustment (credit/debit)
	affiliates.Post("/:id/adjustments", func(c *fiber.Ctx) error {
		adminUserID := getAdminUserID(c)
		affiliateID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid affiliate id"})
		}

		var req struct {
			AdjustmentType string `json:"adjustment_type"`
			Amount         string `json:"amount"`
			Reason         string `json:"reason"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		amount, err := decimal.NewFromString(req.Amount)
		if err != nil || amount.IsNegative() || amount.IsZero() {
			return c.Status(400).JSON(fiber.Map{"error": "invalid amount"})
		}

		adjustment, err := svc.CreateAdjustment(c.Context(), service.CreateAdjustmentInput{
			AffiliateID:    affiliateID,
			AdjustmentType: domain.AdjustmentType(req.AdjustmentType),
			Amount:         amount,
			Reason:         req.Reason,
			CreatedBy:      adminUserID,
		})
		if err != nil {
			return handleDomainError(c, err)
		}

		return c.Status(201).JSON(adjustment)
	})

	// Payout queue — list all payouts with filter
	affiliates.Get("/payouts", func(c *fiber.Ctx) error {
		status := domain.PayoutStatus(c.Query("status", ""))
		page, pageSize := parsePagination(c)
		offset := (page - 1) * pageSize

		payouts, total, err := svc.ListAllPayouts(c.Context(), status, pageSize, offset)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "internal error"})
		}

		return c.JSON(paginatedResponse(payouts, total, page, pageSize))
	})

	// Approve payout
	affiliates.Post("/payouts/:id/approve", func(c *fiber.Ctx) error {
		adminUserID := getAdminUserID(c)
		payoutID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid payout id"})
		}

		var req struct {
			ProviderReference string `json:"provider_reference"`
		}
		c.BodyParser(&req)

		payout, err := svc.ApproveAffiliatePayout(c.Context(), service.ApproveAffiliatePayoutInput{
			PayoutID:          payoutID,
			ApprovedBy:        adminUserID,
			ProviderReference: req.ProviderReference,
		})
		if err != nil {
			return handleDomainError(c, err)
		}

		return c.JSON(payout)
	})

	// Reject payout
	affiliates.Post("/payouts/:id/reject", func(c *fiber.Ctx) error {
		adminUserID := getAdminUserID(c)
		payoutID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid payout id"})
		}

		var req struct {
			RejectionReason string `json:"rejection_reason"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		payout, err := svc.RejectAffiliatePayout(c.Context(), service.RejectAffiliatePayoutInput{
			PayoutID:        payoutID,
			RejectedBy:      adminUserID,
			RejectionReason: req.RejectionReason,
		})
		if err != nil {
			return handleDomainError(c, err)
		}

		return c.JSON(payout)
	})

	// Fraud flags list
	affiliates.Get("/fraud-flags", func(c *fiber.Ctx) error {
		status := domain.FraudFlagStatus(c.Query("status", string(domain.FraudFlagStatusOpen)))
		page, pageSize := parsePagination(c)
		offset := (page - 1) * pageSize

		flags, total, err := svc.ListFraudFlags(c.Context(), status, pageSize, offset)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "internal error"})
		}

		return c.JSON(paginatedResponse(flags, total, page, pageSize))
	})

	// List commission plans (admin).
	affiliates.Get("/commission-plans", func(c *fiber.Ctx) error {
		plans, err := svc.ListCommissionPlans(c.Context())
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "internal error"})
		}

		return c.JSON(fiber.Map{"plans": plans})
	})

	// Change affiliate commission plan (applies to future accruals only).
	affiliates.Put("/:id/commission-plan", func(c *fiber.Ctx) error {
		affiliateID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid affiliate id"})
		}

		var req struct {
			CommissionPlanID string `json:"commission_plan_id"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}
		planID, err := uuid.Parse(req.CommissionPlanID)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid commission_plan_id"})
		}

		profile, err := svc.ChangeCommissionPlan(c.Context(), affiliateID, planID)
		if err != nil {
			return handleDomainError(c, err)
		}

		return c.JSON(profile)
	})

	// Reverse a commission (fraud / chargeback / correction).
	affiliates.Post("/:id/earnings/:earning_id/reverse", func(c *fiber.Ctx) error {
		adminUserID := getAdminUserID(c)
		affiliateID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid affiliate id"})
		}
		earningID, err := uuid.Parse(c.Params("earning_id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid earning id"})
		}

		var req struct {
			Reason string `json:"reason"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}
		if req.Reason == "" {
			return c.Status(400).JSON(fiber.Map{"error": "reason is required"})
		}

		earning, err := svc.ReverseCommission(c.Context(), service.ReverseCommissionInput{
			AffiliateID: affiliateID,
			EarningID:   earningID,
			Reason:      req.Reason,
			ReversedBy:  adminUserID,
		})
		if err != nil {
			return handleDomainError(c, err)
		}

		return c.JSON(earning)
	})

	// Affiliate ledger balances (admin/finance).
	affiliates.Get("/:id/ledger", func(c *fiber.Ctx) error {
		affiliateID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid affiliate id"})
		}

		balances, err := svc.ListLedgerBalances(c.Context(), affiliateID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "internal error"})
		}

		return c.JSON(fiber.Map{"affiliate_id": affiliateID.String(), "balances": balances})
	})

	// Ledger reconciliation: materialized vs derived balances.
	affiliates.Get("/:id/ledger/reconciliation", func(c *fiber.Ctx) error {
		affiliateID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid affiliate id"})
		}

		report, err := svc.ReconcileAffiliate(c.Context(), affiliateID)
		if err != nil {
			return handleDomainError(c, err)
		}

		return c.JSON(report)
	})

	// Get affiliate detail by ID (includes dashboard).
	// Keep this route after static admin paths like /payouts and /fraud-flags
	// so those paths are not captured as dynamic :id.
	affiliates.Get("/:id", func(c *fiber.Ctx) error {
		affiliateID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid affiliate id"})
		}

		profile, err := svc.GetProfileByID(c.Context(), affiliateID)
		if err != nil {
			return handleDomainError(c, err)
		}
		if profile == nil {
			return c.Status(404).JSON(fiber.Map{"error": "affiliate not found"})
		}

		dashboard, _ := svc.GetDashboard(c.Context(), affiliateID)

		return c.JSON(fiber.Map{
			"profile":   profile,
			"dashboard": dashboard,
		})
	})

	// Flag affiliate fraud (manual)
	affiliates.Post("/:id/fraud-flag", func(c *fiber.Ctx) error {
		affiliateID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid affiliate id"})
		}

		var req struct {
			ReferredUserID int64             `json:"referred_user_id"`
			FlagType       string            `json:"flag_type"`
			Severity       string            `json:"severity"`
			Details        map[string]string `json:"details"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		flag, err := svc.FlagAffiliateFraud(c.Context(), service.FlagAffiliateFraudInput{
			AffiliateID:    affiliateID,
			ReferredUserID: req.ReferredUserID,
			FlagType:       req.FlagType,
			Severity:       domain.FraudSeverity(req.Severity),
			Details:        req.Details,
		})
		if err != nil {
			return handleDomainError(c, err)
		}

		return c.Status(201).JSON(flag)
	})
}
