package handlers

import (
	"math"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	adminclient "github.com/opus-casino/admin-bff/internal/client"
	"github.com/opus-casino/admin-bff/internal/models"
	"github.com/opus-casino/admin-bff/internal/service"
	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"
	bettingv1 "github.com/opus-casino/proto/gen/go/betting/v1"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func RegisterSportsRoutes(router fiber.Router, db *gorm.DB, log *zap.Logger, bettingClient *adminclient.BettingClient, auditSvc *service.AuditService) {
	sports := router.Group("/sports")
	sports.Get("/events", listSportsEvents(db, log))
	sports.Get("/events/:id", getSportsEvent(db, log))
	sports.Patch("/events/:id", updateSportsEvent(db, log, auditSvc))
	sports.Post("/events/:id/suspend", suspendSportsEvent(db, log, auditSvc))
	sports.Get("/odds/snapshot", getOddsSnapshot(db, log, bettingClient))
	sports.Post("/odds/:market_id/override", overrideOdds(db, log, auditSvc))
	sports.Get("/margins", listMargins(db, log))
	sports.Put("/margins", updateMargin(db, log, auditSvc))
	sports.Get("/limits", listLimits(db, log))
	sports.Put("/limits", updateLimit(db, log, auditSvc))
	sports.Get("/liability", getLiability(db, log))
	sports.Get("/bets", listSportsBets(db, log, bettingClient))
	sports.Post("/bets/:id/void", voidBet(db, log, bettingClient, auditSvc))
	sports.Post("/bets/:id/resettle", resettleBet(db, log, bettingClient, auditSvc))
	sports.Get("/live-scores/:event_id", getLiveScore(db, log))
}

func listSportsEvents(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sport := c.Query("sport", "")
		league := c.Query("league", "")
		status := c.Query("status", "")
		page := c.QueryInt("page", 1)
		pageSize := c.QueryInt("page_size", 20)
		if page < 1 { page = 1 }
		if pageSize < 1 || pageSize > 200 { pageSize = 20 }
		q := db.Model(&models.SportsEvent{})
		if sport != "" { q = q.Where("sport = ?", sport) }
		if league != "" { q = q.Where("league = ?", league) }
		if status != "" { q = q.Where("status = ?", status) }
		var total int64
		if err := q.Count(&total).Error; err != nil {
			log.Error("count sports events failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		var items []models.SportsEvent
		offset := (page - 1) * pageSize
		if err := q.Order("start_time ASC").Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
			log.Error("query sports events failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		tp := int(math.Ceil(float64(total) / float64(pageSize)))
		return c.JSON(fiber.Map{
			"data": items,
			"pagination": fiber.Map{"page": page, "page_size": pageSize, "total": total, "total_pages": tp},
		})
	}
}

func getSportsEvent(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var event models.SportsEvent
		if err := db.First(&event, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(404).JSON(fiber.Map{"error": "not found"})
			}
			log.Error("get sports event failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		var markets []models.SportsMarket
		db.Where("event_id = ?", id).Find(&markets)
		return c.JSON(fiber.Map{"data": fiber.Map{"event": event, "markets": markets}})
	}
}

func updateSportsEvent(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var body map[string]interface{}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var before models.SportsEvent
		db.First(&before, "id = ?", id)
		body["updated_at"] = time.Now()
		if err := db.Model(&models.SportsEvent{}).Where("id = ?", id).Updates(body).Error; err != nil {
			log.Error("update sports event failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "sports.event.update", "sports_event", id, fiber.Map{"before": before, "after": body})
		return c.JSON(fiber.Map{"success": true})
	}
}

func suspendSportsEvent(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var body struct{ Reason string `json:"reason"` }
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var before models.SportsEvent
		db.First(&before, "id = ?", id)
		if err := db.Model(&models.SportsEvent{}).Where("id = ?", id).Updates(map[string]interface{}{
			"is_suspended": true, "suspend_reason": body.Reason, "updated_at": time.Now(),
		}).Error; err != nil {
			log.Error("suspend sports event failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "sports.event.suspend", "sports_event", id, fiber.Map{
			"before_suspended": before.IsSuspended, "reason": body.Reason,
		})
		return c.JSON(fiber.Map{"success": true})
	}
}

func getOddsSnapshot(db *gorm.DB, log *zap.Logger, bettingClient *adminclient.BettingClient) fiber.Handler {
	return func(c *fiber.Ctx) error {
		eventID := c.Query("event_id", "")
		marketID := c.Query("market_id", "")
		if eventID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "event_id required"})
		}
		resp, err := bettingClient.GetOdds(c.Context(), eventID, marketID)
		if err != nil {
			log.Error("fetch odds from betting-engine failed", zap.Error(err))
			return c.Status(502).JSON(fiber.Map{"error": "upstream unavailable", "details": err.Error()})
		}
		return c.JSON(fiber.Map{"data": resp})
	}
}

func overrideOdds(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		marketID := c.Params("market_id")
		var body struct {
			Selection string `json:"selection"`
			Odds      string `json:"odds"`
			Reason    string `json:"reason"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		override := models.OddsOverride{
			MarketID:  marketID,
			Selection: body.Selection,
			Odds:      body.Odds,
			Reason:    body.Reason,
			SetBy:     c.Locals("admin_id").(string),
		}
		if err := db.Create(&override).Error; err != nil {
			log.Error("create odds override failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "sports.odds.override", "odds_override", override.ID, fiber.Map{
			"market_id": marketID, "selection": body.Selection, "odds": body.Odds, "reason": body.Reason,
		})
		return c.JSON(fiber.Map{"data": override})
	}
}

func listMargins(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var items []models.MarginSetting
		if err := db.Find(&items).Error; err != nil {
			log.Error("list margins failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": items})
	}
}

func updateMargin(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var body models.MarginSetting
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		body.UpdatedBy = c.Locals("admin_id").(string)
		body.UpdatedAt = time.Now()
		var before models.MarginSetting
		db.First(&before, "id = ?", body.ID)
		if err := db.Save(&body).Error; err != nil {
			log.Error("update margin failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "sports.margin.update", "margin_setting", body.ID, fiber.Map{
			"before": before, "after": body,
		})
		return c.JSON(fiber.Map{"data": body})
	}
}

func listLimits(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var items []models.StakeLimit
		if err := db.Find(&items).Error; err != nil {
			log.Error("list limits failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": items})
	}
}

func updateLimit(db *gorm.DB, log *zap.Logger, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var body models.StakeLimit
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		body.UpdatedBy = c.Locals("admin_id").(string)
		body.UpdatedAt = time.Now()
		var before models.StakeLimit
		db.First(&before, "id = ?", body.ID)
		if err := db.Save(&body).Error; err != nil {
			log.Error("update limit failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		logAudit(auditSvc, c, "sports.limit.update", "stake_limit", body.ID, fiber.Map{
			"before": before, "after": body,
		})
		return c.JSON(fiber.Map{"data": body})
	}
}

func getLiability(db *gorm.DB, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var items []models.LiabilitySnapshot
		if err := db.Order("recorded_at DESC").Limit(1000).Find(&items).Error; err != nil {
			log.Error("get liability failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		return c.JSON(fiber.Map{"data": items})
	}
}

func listSportsBets(db *gorm.DB, log *zap.Logger, bettingClient *adminclient.BettingClient) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userIDStr := c.Query("user_id", "")
		statusStr := c.Query("status", "")
		cursor := c.Query("cursor", "")
		pageSize := int32(c.QueryInt("page_size", 20))
		var userID int64
		if userIDStr != "" {
			var err error
			userID, err = strconv.ParseInt(userIDStr, 10, 64)
			if err != nil {
				return c.Status(400).JSON(fiber.Map{"error": "invalid user_id"})
			}
		}
		status := commonv1.BetStatus_BET_STATUS_UNSPECIFIED
		if statusStr != "" {
			// Attempt to parse from known values; default remains UNSPECIFIED on unknown input.
			switch statusStr {
			case "pending":
				status = commonv1.BetStatus_BET_STATUS_PENDING
			case "settled":
				status = commonv1.BetStatus_BET_STATUS_SETTLED
			case "cancelled":
				status = commonv1.BetStatus_BET_STATUS_CANCELLED
			}
		}
		bets, pageResp, err := bettingClient.GetUserBets(c.Context(), userID, status, commonv1.BetType_BET_TYPE_UNSPECIFIED, pageSize, cursor)
		if err != nil {
			log.Error("list sports bets from betting-engine failed", zap.Error(err))
			return c.Status(502).JSON(fiber.Map{"error": "upstream unavailable", "details": err.Error()})
		}
		return c.JSON(fiber.Map{
			"data": bets,
			"pagination": fiber.Map{
				"next_cursor": pageResp.NextCursor,
				"has_more":   pageResp.HasMore,
				"total_count": pageResp.TotalCount,
			},
		})
	}
}

func voidBet(db *gorm.DB, log *zap.Logger, bettingClient *adminclient.BettingClient, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var body struct {
			Reason string `json:"reason"`
			UserID int64  `json:"user_id"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		if body.UserID == 0 {
			// Attempt to fetch bet upstream to resolve user_id.
			bet, err := bettingClient.GetBet(c.Context(), id)
			if err != nil || bet == nil || bet.UserId == nil {
				return c.Status(400).JSON(fiber.Map{"error": "user_id required (could not resolve from upstream)"})
			}
			var parseErr error
			body.UserID, parseErr = strconv.ParseInt(bet.UserId.Value, 10, 64)
			if parseErr != nil {
				return c.Status(400).JSON(fiber.Map{"error": "invalid user_id from upstream"})
			}
		}
		if err := bettingClient.CancelBet(c.Context(), id, body.UserID, body.Reason); err != nil {
			log.Error("void bet via betting-engine failed", zap.Error(err))
			return c.Status(502).JSON(fiber.Map{"error": "upstream void failed", "details": err.Error()})
		}
		logAudit(auditSvc, c, "sports.bet.void", "bet", id, fiber.Map{"reason": body.Reason, "user_id": body.UserID})
		return c.JSON(fiber.Map{"success": true, "bet_id": id, "action": "voided"})
	}
}

func resettleBet(db *gorm.DB, log *zap.Logger, bettingClient *adminclient.BettingClient, auditSvc *service.AuditService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var body struct {
			Result     string `json:"result"`
			ActualWin  string `json:"actual_win,omitempty"`
			Currency   string `json:"currency,omitempty"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		var resultEnum bettingv1.BetResult
		switch body.Result {
		case "won":
			resultEnum = bettingv1.BetResult_BET_RESULT_WON
		case "lost":
			resultEnum = bettingv1.BetResult_BET_RESULT_LOST
		case "void":
			resultEnum = bettingv1.BetResult_BET_RESULT_VOID
		case "half_won":
			resultEnum = bettingv1.BetResult_BET_RESULT_HALF_WON
		case "half_lost":
			resultEnum = bettingv1.BetResult_BET_RESULT_HALF_LOST
		default:
			return c.Status(400).JSON(fiber.Map{"error": "invalid result"})
		}
		var actualWin *commonv1.Money
		if body.ActualWin != "" {
			actualWin = &commonv1.Money{
				Amount:   body.ActualWin,
				Currency: body.Currency,
			}
		}
		if err := bettingClient.SettleBet(c.Context(), id, resultEnum, actualWin, "admin_resettle"); err != nil {
			log.Error("resettle bet via betting-engine failed", zap.Error(err))
			return c.Status(502).JSON(fiber.Map{"error": "upstream resettle failed", "details": err.Error()})
		}
		logAudit(auditSvc, c, "sports.bet.resettle", "bet", id, fiber.Map{
			"result": body.Result, "actual_win": body.ActualWin, "currency": body.Currency,
		})
		return c.JSON(fiber.Map{"success": true, "bet_id": id, "result": body.Result})
	}
}

func getLiveScore(db *gorm.DB, log *zap.Logger) fiber.Handler {
	sportradar := adminclient.NewSportradarClient()
	return func(c *fiber.Ctx) error {
		eventID := c.Params("event_id")
		score, err := sportradar.GetLiveScore(c.Context(), eventID)
		if err != nil {
			// Return stub data when not configured — non-fatal.
			return c.JSON(fiber.Map{"data": fiber.Map{
				"event_id":   eventID,
				"score_home": 0,
				"score_away": 0,
				"status":     "not_configured",
				"message":    "Set SPORTRADAR_API_KEY to enable live scores",
			}})
		}
		return c.JSON(fiber.Map{"data": score})
	}
}
