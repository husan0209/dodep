package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/opus-casino/admin-bff/internal/middleware"
	"github.com/opus-casino/admin-bff/internal/service"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Zone owner: affiliate route table (all /admin/affiliates endpoints).
// Extracted from affiliates.go so that agents editing the affiliate CRUD
// handlers cannot silently drop routes, permissions or handlers that live in
// affiliate_reporting.go / affiliate_payout_flow.go / affiliate_fraud_review.go.
//
// DO NOT move handler calls out of this file — the map below is the single
// place where an affiliate endpoint exists, and the admin SPA depends on it.
//
// Affiliate permission matrix (tasks/задача.md, least privilege):
//
//	affiliate.view            — read profiles, lists, stats, players
//	affiliate.manage          — create/update profile, rate, postback config
//	affiliate.approve         — approve / reject / suspend
//	affiliate.adjust          — manual adjustments
//	affiliate.payout.approve  — payout approve / reject / mark paid
//	affiliate.fraud.review    — fraud flags review
//
// Enforcement is server-side (OWASP API5:2023 BFLA): hiding a button in the
// SPA is not an authorization control.
func RegisterAffiliateRoutes(router fiber.Router, db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) {
	aff := router.Group("/affiliates")
	aff.Get("", middleware.RequirePermission("affiliate.view"), listAffiliates(db, log))
	aff.Post("", middleware.RequirePermission("affiliate.manage"), createAffiliate(db, log, auditSvc))
	// Static collection routes MUST be registered before "/:id" patterns
	// to avoid Fiber matching them as ":id".
	aff.Get("/payouts", middleware.RequirePermission("affiliate.view"), listPayouts(db, log))
	aff.Post("/payouts/:id/approve", middleware.RequirePermission("affiliate.payout.approve"), approvePayout(db, log, auditSvc))
	aff.Post("/payouts/:id/reject", middleware.RequirePermission("affiliate.payout.approve"), rejectPayout(db, log, auditSvc))
	aff.Post("/payouts/:id/paid", middleware.RequirePermission("affiliate.payout.approve"), markPayoutPaid(db, log, auditSvc))
	aff.Get("/fraud-flags", middleware.RequirePermission("affiliate.fraud.review"), listFraudFlags(db, log))
	aff.Post("/fraud-flags/:id/resolve", middleware.RequirePermission("affiliate.fraud.review"), resolveFraudFlag(db, log, auditSvc))
	aff.Post("/fraud-flags/:id/dismiss", middleware.RequirePermission("affiliate.fraud.review"), dismissFraudFlag(db, log, auditSvc))
	aff.Get("/:id", middleware.RequirePermission("affiliate.view"), getAffiliate(db, log))
	aff.Put("/:id", middleware.RequirePermission("affiliate.manage"), updateAffiliate(db, log, auditSvc))
	aff.Post("/:id/approve", middleware.RequirePermission("affiliate.approve"), resolveAffiliateParam(db), approveAffiliate(db, log, auditSvc))
	aff.Post("/:id/reject", middleware.RequirePermission("affiliate.approve"), rejectAffiliate(db, log, auditSvc))
	aff.Post("/:id/suspend", middleware.RequirePermission("affiliate.approve"), resolveAffiliateParam(db), suspendAffiliate(db, log, auditSvc))
	aff.Put("/:id/commission-rate", middleware.RequirePermission("affiliate.manage"), updateCommissionRate(db, log, auditSvc))
	aff.Get("/:id/players", middleware.RequirePermission("affiliate.view"), listAffiliatePlayers(db, log))
	aff.Get("/:id/stats", middleware.RequirePermission("affiliate.view"), getAffiliateStats(db, log))
	aff.Post("/:id/calculate-period", middleware.RequirePermission("affiliate.manage"), calculatePeriod(db, log, auditSvc))
	aff.Get("/:id/postback-config", middleware.RequirePermission("affiliate.view"), getPostbackConfig(db, log))
	aff.Put("/:id/postback-config", middleware.RequirePermission("affiliate.manage"), updatePostbackConfig(db, log, auditSvc))
}
