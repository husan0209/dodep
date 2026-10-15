package main

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/opus-casino/bonus/internal/domain"
	"github.com/opus-casino/bonus/internal/ratelimit"
	"github.com/opus-casino/bonus/internal/service"
)

// setupRoutesOnGroup registers Bonus HTTP endpoints on an authenticated group.
// The group must already have AuthMiddleware applied, so user_id comes from JWT.
// writeLimiter throttles state-changing endpoints; nil disables that limit.
func setupRoutesOnGroup(api fiber.Router, svc *service.BonusService, writeLimiter *ratelimit.Limiter) {

	api.Get("/", func(c *fiber.Ctx) error {
		userID, ok := userIDFromToken(c)
		if !ok {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		limit, _ := strconv.Atoi(c.Query("limit", "20"))
		offset, _ := strconv.Atoi(c.Query("offset", "0"))
		bonuses, total, err := svc.ListBonuses(c.Context(), userID, limit, offset)
		if err != nil {
			return writeBonusError(c, err)
		}
		return c.JSON(fiber.Map{"bonuses": bonuses, "total": total})
	})

	api.Get("/active", func(c *fiber.Ctx) error {
		userID, ok := userIDFromToken(c)
		if !ok {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		bonus, err := svc.GetActiveBonus(c.Context(), userID)
		if err != nil {
			return writeBonusError(c, err)
		}
		if bonus == nil {
			return c.JSON(fiber.Map{"bonus": nil})
		}
		return c.JSON(bonus)
	})

	api.Get("/:id", func(c *fiber.Ctx) error {
		userID, ok := userIDFromToken(c)
		if !ok {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		bonusID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid bonus id"})
		}
		bonus, err := svc.GetBonus(c.Context(), userID, bonusID)
		if err != nil {
			return writeBonusError(c, err)
		}
		return c.JSON(bonus)
	})

	api.Get("/:id/wagering", func(c *fiber.Ctx) error {
		userID, ok := userIDFromToken(c)
		if !ok {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		bonusID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid bonus id"})
		}
		bonus, err := svc.GetBonus(c.Context(), userID, bonusID)
		if err != nil {
			return writeBonusError(c, err)
		}
		progress := 0.0
		if !bonus.WageringRequired.IsZero() {
			p, _ := bonus.WageringCompleted.Div(bonus.WageringRequired).Mul(decimal.NewFromInt(100)).Float64()
			progress = p
		}
		return c.JSON(fiber.Map{
			"bonus_id":            bonus.ID.String(),
			"required":            bonus.WageringRequired.String(),
			"completed":           bonus.WageringCompleted.String(),
			"remaining":           bonus.RemainingWagering().String(),
			"progress_percentage": progress,
			"is_completed":        bonus.IsWageringComplete(),
		})
	})

	api.Post("/:id/activate", IdempotencyMiddleware(), writeGuard(writeLimiter), func(c *fiber.Ctx) error {
		userID, ok := userIDFromToken(c)
		if !ok {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		bonusID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid bonus id"})
		}
		bonus, err := svc.ActivateBonus(c.Context(), userID, bonusID)
		if err != nil {
			return writeBonusError(c, err)
		}
		return c.Status(200).JSON(bonus)
	})

	api.Post("/:id/cancel", IdempotencyMiddleware(), writeGuard(writeLimiter), func(c *fiber.Ctx) error {
		userID, ok := userIDFromToken(c)
		if !ok {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		bonusID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid bonus id"})
		}
		bonus, err := svc.CancelBonus(c.Context(), userID, bonusID)
		if err != nil {
			return writeBonusError(c, err)
		}
		return c.JSON(bonus)
	})
}

// writeGuard applies the write rate limit when a limiter is configured.
// Keeping it nil-tolerant lets tests mount routes without any throttling.
func writeGuard(l *ratelimit.Limiter) fiber.Handler {
	if l == nil {
		return func(c *fiber.Ctx) error { return c.Next() }
	}
	return l.Middleware(ratelimit.ByUserOrIP)
}

// userIDFromToken reads user_id from JWT middleware locals (NEVER from body).
// It returns (id, true) on success and (0, false) when the request is
// unauthenticated — the caller renders the 401 response itself.
//
// NEVER return c.JSON() as an error here: Fiber returns nil after a
// successful JSON write, so `if err != nil` callers would mistake the 401
// response for success and continue with userID 0 (auth bypass).
func userIDFromToken(c *fiber.Ctx) (int64, bool) {
	raw := getUserID(c)
	if raw == "" {
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func writeBonusError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, domain.ErrBonusNotFound):
		return c.Status(404).JSON(fiber.Map{"error": "bonus not found"})
	case errors.Is(err, domain.ErrBonusNotActive):
		return c.Status(409).JSON(fiber.Map{"error": "bonus is not active"})
	case errors.Is(err, domain.ErrBonusAlreadyExists):
		return c.Status(409).JSON(fiber.Map{"error": "bonus already exists"})
	case errors.Is(err, domain.ErrInvalidBonusAmount):
		return c.Status(400).JSON(fiber.Map{"error": "invalid bonus amount"})
	default:
		return c.Status(500).JSON(fiber.Map{"error": "internal server error"})
	}
}
