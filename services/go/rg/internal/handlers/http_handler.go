package handlers

import (
	"errors"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/shopspring/decimal"

	"github.com/opus-casino/rg/internal/domain"
	"github.com/opus-casino/rg/internal/service"
)

// Handler is the thin HTTP layer. Business logic lives in service.
type Handler struct {
	svc      *service.RGService
	validate *validator.Validate
}

// New builds a Handler. Panics on nil service (fail fast).
func New(svc *service.RGService) *Handler {
	if svc == nil {
		panic("rg: service is required")
	}
	return &Handler{svc: svc, validate: validator.New()}
}

// CheckRequest gates one gambling action. user_id comes from JWT, never body.
type CheckRequest struct {
	Channel  string `json:"channel" validate:"required,oneof=bet game_launch deposit withdrawal"`
	Amount   string `json:"amount" validate:"omitempty"`
	Currency string `json:"currency" validate:"omitempty,len=3"`
}

// Check handles POST /api/v1/rg/check.
func (h *Handler) Check(c *fiber.Ctx) error {
	var req CheckRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "RG_INVALID_BODY", "Invalid request body")
	}
	if err := h.validate.Struct(req); err != nil {
		return respondValidationError(c, err.(validator.ValidationErrors))
	}
	amount := decimal.Zero
	if req.Amount != "" {
		var err error
		amount, err = decimal.NewFromString(req.Amount)
		if err != nil || amount.IsNegative() {
			return respondError(c, fiber.StatusBadRequest, "RG_INVALID_AMOUNT", "amount must be a decimal >= 0")
		}
	}
	decision, err := h.svc.CheckPlayAllowed(c.Context(), service.CheckInput{
		UserID: getUserID(c), Channel: domain.Channel(req.Channel),
		Amount: amount, Currency: req.Currency,
	})
	if err != nil {
		return h.mapError(c, err)
	}
	// Denials are legitimate business outcomes, not client errors: 422 with a
	// machine-readable reason so callers can react (show remaining headroom,
	// trigger cooling-off UX). Hard blocks (exclusion / cool-off) are 403.
	status := fiber.StatusOK
	if !decision.Allowed {
		switch decision.Reason {
		case domain.ReasonSelfExcluded, domain.ReasonCoolOff:
			status = fiber.StatusForbidden
		default:
			status = fiber.StatusUnprocessableEntity
		}
	}
	out := fiber.Map{"allowed": decision.Allowed, "reason": string(decision.Reason)}
	if decision.Message != "" {
		out["message"] = decision.Message
	}
	if decision.Remaining != nil {
		out["remaining_amount"] = decision.Remaining.String()
	}
	return respondSuccess(c, status, out)
}

// SetLimitsRequest carries the requested controls (nil = no change).
type SetLimitsRequest struct {
	DepositDaily        *string `json:"deposit_daily"`
	DepositWeekly       *string `json:"deposit_weekly"`
	DepositMonthly      *string `json:"deposit_monthly"`
	LossDaily           *string `json:"loss_daily"`
	LossWeekly          *string `json:"loss_weekly"`
	LossMonthly         *string `json:"loss_monthly"`
	WagerDaily          *string `json:"wager_daily"`
	WagerWeekly         *string `json:"wager_weekly"`
	SessionMinutes      *int    `json:"session_minutes"`
	RealityCheckMinutes *int    `json:"reality_check_minutes"`
}

func parseDecimalPtr(raw *string, field string) (*decimal.Decimal, error) {
	if raw == nil {
		return nil, nil
	}
	d, err := decimal.NewFromString(*raw)
	if err != nil || d.IsNegative() {
		return nil, domain.NewValidationError(domain.FieldError{Field: field, Message: "must be a decimal >= 0"})
	}
	return &d, nil
}

// SetLimits handles PUT /api/v1/rg/limits.
func (h *Handler) SetLimits(c *fiber.Ctx) error {
	var req SetLimitsRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "RG_INVALID_BODY", "Invalid request body")
	}
	in := service.SetLimitsInput{UserID: getUserID(c)}
	fields := map[domain.LimitType]**decimal.Decimal{
		domain.LimitDepositDaily: &in.DepositDaily, domain.LimitDepositWeekly: &in.DepositWeekly,
		domain.LimitDepositMonthly: &in.DepositMonthly, domain.LimitLossDaily: &in.LossDaily,
		domain.LimitLossWeekly: &in.LossWeekly, domain.LimitLossMonthly: &in.LossMonthly,
		domain.LimitWagerDaily: &in.WagerDaily, domain.LimitWagerWeekly: &in.WagerWeekly,
	}
	raw := map[domain.LimitType]*string{
		domain.LimitDepositDaily: req.DepositDaily, domain.LimitDepositWeekly: req.DepositWeekly,
		domain.LimitDepositMonthly: req.DepositMonthly, domain.LimitLossDaily: req.LossDaily,
		domain.LimitLossWeekly: req.LossWeekly, domain.LimitLossMonthly: req.LossMonthly,
		domain.LimitWagerDaily: req.WagerDaily, domain.LimitWagerWeekly: req.WagerWeekly,
	}
	for t, r := range raw {
		v, err := parseDecimalPtr(r, string(t))
		if err != nil {
			return h.mapError(c, err)
		}
		*fields[t] = v
	}
	in.SessionMinutes = req.SessionMinutes
	in.RealityCheckMinutes = req.RealityCheckMinutes

	res, err := h.svc.SetLimits(c.Context(), in)
	if err != nil {
		return h.mapError(c, err)
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"limits":  toLimitsDTO(res.Limits),
		"pending": toPendingDTO(res.Pending),
	})
}

// GetLimits handles GET /api/v1/rg/limits.
func (h *Handler) GetLimits(c *fiber.Ctx) error {
	res, err := h.svc.GetLimits(c.Context(), getUserID(c))
	if err != nil {
		return h.mapError(c, err)
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"limits":  toLimitsDTO(res.Limits),
		"pending": toPendingDTO(res.Pending),
	})
}

// SelfExclusionRequest starts a self-exclusion. user_id from JWT.
type SelfExclusionRequest struct {
	Period string `json:"period" validate:"required,oneof=24h 7d 30d 6m 1y permanent"`
}

// StartSelfExclusion handles POST /api/v1/rg/self-exclusion.
func (h *Handler) StartSelfExclusion(c *fiber.Ctx) error {
	var req SelfExclusionRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "RG_INVALID_BODY", "Invalid request body")
	}
	if err := h.validate.Struct(req); err != nil {
		return respondValidationError(c, err.(validator.ValidationErrors))
	}
	excl, err := h.svc.StartSelfExclusion(c.Context(), service.StartSelfExclusionInput{
		UserID: getUserID(c), Period: domain.ExclusionPeriod(req.Period),
		Type: domain.ExclusionSelf, By: "self",
	})
	if err != nil {
		return h.mapError(c, err)
	}
	return respondSuccess(c, fiber.StatusCreated, toExclusionDTO(excl))
}

// RevokeRequest lifts an expired temporary exclusion (explicit confirm).
type RevokeRequest struct {
	Confirm bool `json:"confirm"`
}

// RevokeSelfExclusion handles POST /api/v1/rg/self-exclusion/revoke.
func (h *Handler) RevokeSelfExclusion(c *fiber.Ctx) error {
	var req RevokeRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "RG_INVALID_BODY", "Invalid request body")
	}
	if err := h.svc.RevokeSelfExclusion(c.Context(), getUserID(c), req.Confirm, "self"); err != nil {
		return h.mapError(c, err)
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"revoked": true})
}

// TimeoutRequest starts a cooling-off time-out.
type TimeoutRequest struct {
	Period string `json:"period" validate:"required,oneof=24h 48h 7d 30d"`
}

// StartTimeout handles POST /api/v1/rg/timeout.
func (h *Handler) StartTimeout(c *fiber.Ctx) error {
	var req TimeoutRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "RG_INVALID_BODY", "Invalid request body")
	}
	if err := h.validate.Struct(req); err != nil {
		return respondValidationError(c, err.(validator.ValidationErrors))
	}
	to, err := h.svc.StartTimeout(c.Context(), getUserID(c), domain.TimeoutPeriod(req.Period))
	if err != nil {
		return h.mapError(c, err)
	}
	return respondSuccess(c, fiber.StatusCreated, fiber.Map{
		"id": to.ID.String(), "until": to.Until.UTC().Format(time.RFC3339),
	})
}

// GetStatus handles GET /api/v1/rg/status.
func (h *Handler) GetStatus(c *fiber.Ctx) error {
	st, err := h.svc.GetStatus(c.Context(), getUserID(c))
	if err != nil {
		return h.mapError(c, err)
	}
	out := fiber.Map{
		"gambling_allowed":      st.GamblingAllowed,
		"blocked_reason":        string(st.BlockedReason),
		"limits":                toLimitsDTO(st.Limits),
		"pending":               toPendingDTO(st.Pending),
		"reality_check_minutes": st.Limits.EffectiveRealityCheck(),
	}
	if st.Exclusion != nil {
		out["exclusion"] = toExclusionDTO(st.Exclusion)
	}
	if st.Timeout != nil {
		out["timeout"] = fiber.Map{
			"id": st.Timeout.ID.String(), "until": st.Timeout.Until.UTC().Format(time.RFC3339),
		}
	}
	return respondSuccess(c, fiber.StatusOK, out)
}

// OperatorExclusionRequest is the admin variant (operator/regulatory type).
type OperatorExclusionRequest struct {
	UserID int64  `json:"user_id" validate:"required,min=1"`
	Period string `json:"period" validate:"required,oneof=24h 7d 30d 6m 1y permanent"`
	Type   string `json:"type" validate:"required,oneof=operator regulatory"`
}

// AdminStartExclusion handles POST /admin/rg/exclusions.
func (h *Handler) AdminStartExclusion(c *fiber.Ctx) error {
	var req OperatorExclusionRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "RG_INVALID_BODY", "Invalid request body")
	}
	if err := h.validate.Struct(req); err != nil {
		return respondValidationError(c, err.(validator.ValidationErrors))
	}
	excl, err := h.svc.StartSelfExclusion(c.Context(), service.StartSelfExclusionInput{
		UserID: req.UserID, Period: domain.ExclusionPeriod(req.Period),
		Type: domain.ExclusionType(req.Type), By: getAdminID(c),
	})
	if err != nil {
		return h.mapError(c, err)
	}
	return respondSuccess(c, fiber.StatusCreated, toExclusionDTO(excl))
}

// AdminApplyDue handles POST /admin/rg/apply-due (scheduler trigger).
func (h *Handler) AdminApplyDue(svc *service.RGService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		n, err := svc.ApplyDueIncreases(c.Context(), time.Now().UTC(), 500)
		if err != nil {
			return h.mapError(c, err)
		}
		return respondSuccess(c, fiber.StatusOK, fiber.Map{"applied": n})
	}
}

// Liveness handles GET /health. Readiness handles GET /ready.
func (h *Handler) Liveness(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok", "service": "rg"})
}

// Readiness handles GET /ready.
func (h *Handler) Readiness(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ready", "service": "rg"})
}

func toLimitsDTO(l *domain.RGLimits) fiber.Map {
	if l == nil {
		return fiber.Map{}
	}
	money := func(d decimal.Decimal) string { return d.String() }
	return fiber.Map{
		"deposit_daily": l.DepositDaily.String(), "deposit_weekly": money(l.DepositWeekly),
		"deposit_monthly": money(l.DepositMonthly), "loss_daily": money(l.LossDaily),
		"loss_weekly": money(l.LossWeekly), "loss_monthly": money(l.LossMonthly),
		"wager_daily": money(l.WagerDaily), "wager_weekly": money(l.WagerWeekly),
		"session_minutes": l.SessionMinutes, "reality_check_minutes": l.EffectiveRealityCheck(),
		"updated_at": l.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toPendingDTO(list []*domain.PendingChange) []fiber.Map {
	out := make([]fiber.Map, 0, len(list))
	for _, p := range list {
		out = append(out, fiber.Map{
			"id": p.ID.String(), "limit_type": string(p.LimitType),
			"old_value": p.OldValue.String(), "new_value": p.NewValue.String(),
			"effective_at": p.EffectiveAt.UTC().Format(time.RFC3339),
		})
	}
	return out
}

func toExclusionDTO(e *domain.Exclusion) fiber.Map {
	out := fiber.Map{
		"id": e.ID.String(), "status": string(e.Status), "type": string(e.Type),
		"permanent": e.Permanent, "created_at": e.CreatedAt.UTC().Format(time.RFC3339),
	}
	if e.Until != nil {
		out["until"] = e.Until.UTC().Format(time.RFC3339)
	}
	return out
}

func (h *Handler) mapError(c *fiber.Ctx, err error) error {
	var ve *domain.ValidationError
	if errors.As(err, &ve) {
		return respondErrorWithDetails(c, fiber.StatusBadRequest, "RG_VALIDATION_FAILED", "Validation failed", ve.Fields)
	}
	var de *domain.DetailedError
	var details interface{}
	if errors.As(err, &de) {
		details = de.Details
		err = de.Err
	}
	switch {
	case errors.Is(err, domain.ErrSelfExcluded):
		return respondError(c, fiber.StatusForbidden, "RG_SELF_EXCLUDED", "Gambling is blocked for this account")
	case errors.Is(err, domain.ErrCoolOffActive):
		return respondError(c, fiber.StatusForbidden, "RG_COOL_OFF", "Cooling-off period is active")
	case errors.Is(err, domain.ErrDepositLimit):
		return respondErrorWithDetails(c, fiber.StatusUnprocessableEntity, "RG_DEPOSIT_LIMIT", "Deposit limit would be exceeded", details)
	case errors.Is(err, domain.ErrLossLimit):
		return respondError(c, fiber.StatusUnprocessableEntity, "RG_LOSS_LIMIT", "Loss limit reached")
	case errors.Is(err, domain.ErrWagerLimit):
		return respondError(c, fiber.StatusUnprocessableEntity, "RG_WAGER_LIMIT", "Wager limit would be exceeded")
	case errors.Is(err, domain.ErrPermanentExclusion):
		return respondError(c, fiber.StatusUnprocessableEntity, "RG_PERMANENT", "Permanent exclusion cannot be revoked")
	case errors.Is(err, domain.ErrExclusionExists), errors.Is(err, domain.ErrTimeoutExists), errors.Is(err, domain.ErrConflict):
		return respondError(c, fiber.StatusConflict, "RG_CONFLICT", err.Error())
	case errors.Is(err, domain.ErrExclusionNotFound), errors.Is(err, domain.ErrNotFound):
		return respondError(c, fiber.StatusNotFound, "RG_NOT_FOUND", "Resource not found")
	case errors.Is(err, domain.ErrRevokeTooEarly), errors.Is(err, domain.ErrRevokeCooling), errors.Is(err, domain.ErrConfirmRequired):
		return respondErrorWithDetails(c, fiber.StatusUnprocessableEntity, "RG_REVOKE_DENIED", err.Error(), details)
	default:
		return respondError(c, fiber.StatusInternalServerError, "RG_INTERNAL_ERROR", "An internal error occurred")
	}
}
