package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/opus-casino/admin-bff/internal/models"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Zone owner: affiliate read models / reporting.
// GET /admin/affiliates/:id/{stats,players}
// Extracted from affiliates.go so concurrent agents editing the
// affiliate CRUD file cannot silently drop this code.

// listAffiliatePlayers returns the referred players of an affiliate.
// Referred players are PII: the response exposes a stable pseudonymous id
// (sha256 of the referred user id, truncated) instead of the raw id, so the
// admin UI can correlate per-player revenue without exposing player identity.
func listAffiliatePlayers(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		profileID, err := resolveServiceProfileID(db, c.Params("id"))
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(404).JSON(fiber.Map{"error": "not found"})
			}
			log.Error("resolve affiliate profile failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		page := c.QueryInt("page", 1)
		pageSize := c.QueryInt("page_size", 20)
		if page < 1 {
			page = 1
		}
		if pageSize < 1 || pageSize > 200 {
			pageSize = 20
		}
		q := db.Model(&models.AffiliateAttribution{}).Where("affiliate_id = ?", profileID)
		var total int64
		if err := q.Count(&total).Error; err != nil {
			log.Error("count referred players failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		var rows []models.AffiliateAttribution
		if err := q.Order("attributed_at DESC").
			Limit(pageSize).Offset((page - 1) * pageSize).
			Find(&rows).Error; err != nil {
			log.Error("query referred players failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		// Per-player revenue from the earnings ledger (source of truth).
		type revenue struct {
			ReferredUserID int64           `gorm:"column:referred_user_id"`
			NGR            decimal.Decimal `gorm:"column:ngr"`
			Commission     decimal.Decimal `gorm:"column:commission"`
		}
		var rev []revenue
		if len(rows) > 0 {
			ids := make([]int64, 0, len(rows))
			for _, r := range rows {
				ids = append(ids, r.ReferredUserID)
			}
			if err := db.Model(&models.AffiliateEarning{}).
				Select("referred_user_id, COALESCE(SUM(ngr_amount), 0) AS ngr, COALESCE(SUM(commission_amount), 0) AS commission").
				Where("affiliate_id = ? AND referred_user_id IN ?", profileID, ids).
				Group("referred_user_id").
				Scan(&rev).Error; err != nil {
				log.Error("aggregate referred player revenue failed", zap.Error(err))
				return c.Status(500).JSON(fiber.Map{"error": "database error"})
			}
		}
		revByUser := make(map[int64]revenue, len(rev))
		for _, r := range rev {
			revByUser[r.ReferredUserID] = r
		}

		items := make([]fiber.Map, 0, len(rows))
		for _, r := range rows {
			r := r
			item := fiber.Map{
				"player_ref":    pseudonymizePlayer(r.ReferredUserID),
				"attributed_at": r.AttributedAt,
				"ftd_qualified": r.IsFTDQualified,
			}
			if r.FTDAt != nil {
				item["ftd_at"] = r.FTDAt
			}
			if rv, ok := revByUser[r.ReferredUserID]; ok {
				item["ngr_amount"] = rv.NGR.StringFixed(8)
				item["commission_amount"] = rv.Commission.StringFixed(8)
			} else {
				item["ngr_amount"] = decimal.Zero.StringFixed(8)
				item["commission_amount"] = decimal.Zero.StringFixed(8)
			}
			items = append(items, item)
		}

		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{
				"page": page, "page_size": pageSize,
				"total": total, "total_pages": int(math.Ceil(float64(total) / float64(pageSize))),
			},
		})
	}
}

// getAffiliateStats aggregates funnel + revenue for one affiliate over an
// optional day window (default: all time). All monetary values are NUMERIC
// decimals rendered as strings (CONVENTIONS NEVER-6).
func getAffiliateStats(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		profileID, err := resolveServiceProfileID(db, c.Params("id"))
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(404).JSON(fiber.Map{"error": "not found"})
			}
			log.Error("resolve affiliate profile failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}

		aggQ := db.Model(&models.AffiliateDailyAggregate{}).Where("affiliate_id = ?", profileID)
		if days := c.QueryInt("days", 0); days > 0 {
			aggQ = aggQ.Where("report_date >= CURRENT_DATE - ?", days)
		}
		var funnel struct {
			Clicks        int64 `gorm:"column:clicks"`
			Registrations int64 `gorm:"column:registrations"`
			FTDCount      int64 `gorm:"column:ftd_count"`
			ActivePlayers int64 `gorm:"column:active_players"`
		}
		if err := aggQ.Select(
			"COALESCE(SUM(clicks), 0) AS clicks, COALESCE(SUM(registrations), 0) AS registrations, " +
				"COALESCE(SUM(ftd_count), 0) AS ftd_count, COALESCE(SUM(active_players), 0) AS active_players",
		).Scan(&funnel).Error; err != nil {
			log.Error("aggregate affiliate funnel failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}

		var revenue struct {
			GGR      decimal.Decimal `gorm:"column:ggr"`
			NGR      decimal.Decimal `gorm:"column:ngr"`
			Accrued  decimal.Decimal `gorm:"column:accrued"`
			Reversed decimal.Decimal `gorm:"column:reversed"`
		}
		if err := db.Model(&models.AffiliateDailyAggregate{}).
			Select("COALESCE(SUM(ggr_amount), 0) AS ggr, COALESCE(SUM(ngr_amount), 0) AS ngr, " +
				"COALESCE(SUM(commission_accrued), 0) AS accrued, COALESCE(SUM(commission_reversed), 0) AS reversed").
			Where(func() *gorm.DB {
				q := db.Model(&models.AffiliateDailyAggregate{}).Where("affiliate_id = ?", profileID)
				if days := c.QueryInt("days", 0); days > 0 {
					q = q.Where("report_date >= CURRENT_DATE - ?", days)
				}
				return q
			}()).
			Scan(&revenue).Error; err != nil {
			log.Error("aggregate affiliate revenue failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}

		// Commission by lifecycle bucket comes from the earnings ledger, which
		// is the source of truth (pending / available / paid / reversed).
		type bucket struct {
			Status string          `gorm:"column:status"`
			Total  decimal.Decimal `gorm:"column:total"`
		}
		var buckets []bucket
		if err := db.Model(&models.AffiliateEarning{}).
			Select("status, COALESCE(SUM(commission_amount), 0) AS total").
			Where("affiliate_id = ?", profileID).
			Group("status").
			Scan(&buckets).Error; err != nil {
			log.Error("aggregate affiliate commission buckets failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		byStatus := make(map[string]decimal.Decimal, len(buckets))
		for _, b := range buckets {
			byStatus[b.Status] = b.Total
		}

		var paidOut decimal.Decimal
		if err := db.Model(&models.AffiliatePayoutAmount{}).
			Select("COALESCE(SUM(amount), 0)").
			Where("affiliate_id = ? AND status = 'paid'", profileID).
			Scan(&paidOut).Error; err != nil {
			log.Error("aggregate affiliate paid payouts failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}

		var referredCount int64
		if err := db.Model(&models.AffiliateAttribution{}).
			Where("affiliate_id = ?", profileID).
			Count(&referredCount).Error; err != nil {
			log.Error("count referred players failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}

		var openFlags int64
		if err := db.Model(&models.FraudFlag{}).
			Where("affiliate_id = ? AND status IN ?", profileID, []string{"open", "in_review"}).
			Count(&openFlags).Error; err != nil {
			log.Error("count open fraud flags failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}

		// Players = total attributions; this is the stable cohort size, whereas
		// active_players is a per-day sum and would double-count across days.
		players := referredCount

		conversion := func(num, den int64) float64 {
			if den == 0 {
				return 0
			}
			return float64(num) / float64(den)
		}

		return c.JSON(fiber.Map{"data": fiber.Map{
			"clicks":               funnel.Clicks,
			"registrations":        funnel.Registrations,
			"ftd_count":            funnel.FTDCount,
			"players":              players,
			"active_players":       funnel.ActivePlayers,
			"ggr":                  revenue.GGR.StringFixed(8),
			"ngr":                  revenue.NGR.StringFixed(8),
			"commission_accrued":   revenue.Accrued.StringFixed(8),
			"commission_reversed":  revenue.Reversed.StringFixed(8),
			"commission_pending":   byStatus["pending"].StringFixed(8),
			"commission_available": byStatus["available"].StringFixed(8),
			"commission_paid":      paidOut.StringFixed(8),
			"owed":                 byStatus["available"].StringFixed(8),
			"open_fraud_flags":     openFlags,
			"click_to_reg_rate":    conversion(funnel.Registrations, funnel.Clicks),
			"reg_to_ftd_rate":      conversion(funnel.FTDCount, funnel.Registrations),
		}})
	}
}

// resolveServiceProfileID maps an admin-side affiliate reference (admin
// affiliates.id or user_id) to the affiliate-service profile id used by
// affiliate_attributions / affiliate_daily_aggregates / affiliate_earnings.
// The two schemas use different UUID spaces, so the join must go through
// the player's user_id (canonical schema stores it as BIGINT).
func resolveServiceProfileID(db *gorm.DB, idOrUserID string) (string, error) {
	var aff models.Affiliate
	if err := db.First(&aff, "id = ?", idOrUserID).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return "", err
		}
		if err := db.First(&aff, "user_id = ?", idOrUserID).Error; err != nil {
			return "", err
		}
	}
	userID, err := strconv.ParseInt(strings.TrimSpace(aff.UserID), 10, 64)
	if err != nil {
		return "", gorm.ErrRecordNotFound
	}
	var profile models.AffiliateProfile
	if err := db.Where("user_id = ?", userID).First(&profile).Error; err != nil {
		return "", err
	}
	return profile.ID, nil
}

// pseudonymizePlayer returns a stable, non-reversible short handle for a
// referred player id. Used so the admin UI can correlate per-player revenue
// without exposing player identifiers (data minimisation).
func pseudonymizePlayer(userID int64) string {
	sum := sha256.Sum256([]byte(strconv.FormatInt(userID, 10)))
	return "plr_" + hex.EncodeToString(sum[:6])
}
