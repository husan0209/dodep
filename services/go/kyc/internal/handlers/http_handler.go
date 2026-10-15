package handlers

import (
	"errors"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/opus-casino/kyc/internal/domain"
	"github.com/opus-casino/kyc/internal/service"
)

// Handler serves the KYC HTTP API (Fiber). Thin by contract:
// parse → validate format → call service → map errors → respond.
type Handler struct {
	svc      *service.KYCService
	validate *validator.Validate
	log      *zap.Logger
}

// NewHandler creates the HTTP handler.
func NewHandler(svc *service.KYCService, log *zap.Logger) *Handler {
	if log == nil {
		log = zap.NewNop()
	}
	return &Handler{svc: svc, validate: validator.New(), log: log}
}

// ─── DTOs (snake_case on the wire, validated with struct tags) ───

type SubmitDocumentRequest struct {
	DocumentType   string `json:"document_type" validate:"required,oneof=passport drivers_license national_id residence_permit proof_of_address bank_statement selfie source_of_funds"`
	DocumentSide   string `json:"document_side" validate:"omitempty,oneof=front back single"`
	FileID         string `json:"file_id" validate:"required,max=255"`
	DocumentNumber string `json:"document_number" validate:"max=64"`
	IssuingCountry string `json:"issuing_country" validate:"omitempty,len=2"`
	ExpiryDate     string `json:"expiry_date" validate:"omitempty,datetime=2006-01-02"`
}

type UpgradeRequest struct {
	TargetLevel int `json:"target_level" validate:"required,min=1,max=4"`
}

type VerifyRequest struct {
	VerificationNotes string            `json:"verification_notes" validate:"max=2000"`
	ExtractedData     map[string]string `json:"extracted_data"`
	FullName          string            `json:"full_name" validate:"max=255"`
	BirthYear         int               `json:"birth_year" validate:"omitempty,min=1900,max=2100"`
	CountryCode       string            `json:"country_code" validate:"omitempty,len=2"`
}

type RejectRequest struct {
	RejectionReason string   `json:"rejection_reason" validate:"required,max=2000"`
	RequiredActions []string `json:"required_actions" validate:"max=10,dive,max=255"`
}

type ResolveScreeningRequest struct {
	Approve bool   `json:"approve"`
	Note    string `json:"note" validate:"max=2000"`
}

type ListQuery struct {
	Type   string `query:"type" validate:"omitempty,oneof=passport drivers_license national_id residence_permit proof_of_address bank_statement selfie source_of_funds"`
	Status string `query:"status" validate:"omitempty,oneof=draft submitted in_review approved rejected expired"`
	Limit  int    `query:"limit" validate:"omitempty,min=1,max=100"`
	Cursor string `query:"cursor" validate:"omitempty,uuid4"`
}

// ─── User endpoints (user_id from JWT, NEVER from body) ───

// SubmitDocument handles POST /api/v1/kyc/documents.
func (h *Handler) SubmitDocument(c *fiber.Ctx) error {
	userID, ok := userIDFromToken(c)
	if !ok {
		return respondError(c, fiber.StatusUnauthorized, "KYC_UNAUTHORIZED", "Not authenticated")
	}
	var req SubmitDocumentRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "KYC_INVALID_BODY", "Invalid request body")
	}
	if err := h.validate.Struct(req); err != nil {
		return respondValidationError(c, err.(validator.ValidationErrors))
	}
	var expires *time.Time
	if req.ExpiryDate != "" {
		t, _ := time.Parse("2006-01-02", req.ExpiryDate)
		expires = &t
	}
	doc, err := h.svc.SubmitDocument(c.Context(), service.SubmitInput{
		UserID:         userID,
		Type:           domain.DocumentType(req.DocumentType),
		Side:           sideOrDefault(req.DocumentSide),
		FileID:         req.FileID,
		DocumentNumber: req.DocumentNumber,
		IssuingCountry: req.IssuingCountry,
		ExpiresAt:      expires,
	})
	if err != nil {
		return h.mapError(c, err)
	}
	return respondSuccess(c, fiber.StatusCreated, toDocumentJSON(doc, false))
}

// GetStatus handles GET /api/v1/kyc/status.
func (h *Handler) GetStatus(c *fiber.Ctx) error {
	userID, ok := userIDFromToken(c)
	if !ok {
		return respondError(c, fiber.StatusUnauthorized, "KYC_UNAUTHORIZED", "Not authenticated")
	}
	st, err := h.svc.GetStatus(c.Context(), userID)
	if err != nil {
		return h.mapError(c, err)
	}
	return respondSuccess(c, fiber.StatusOK, toStatusJSON(st))
}

// ListDocuments handles GET /api/v1/kyc/documents.
func (h *Handler) ListDocuments(c *fiber.Ctx) error {
	userID, ok := userIDFromToken(c)
	if !ok {
		return respondError(c, fiber.StatusUnauthorized, "KYC_UNAUTHORIZED", "Not authenticated")
	}
	var q ListQuery
	if err := c.QueryParser(&q); err != nil {
		return respondError(c, fiber.StatusBadRequest, "KYC_INVALID_PARAMS", "Invalid query parameters")
	}
	if err := h.validate.Struct(q); err != nil {
		return respondValidationError(c, err.(validator.ValidationErrors))
	}
	st, err := h.svc.GetStatus(c.Context(), userID)
	if err != nil {
		return h.mapError(c, err)
	}
	items := filterDocs(st.Documents, q.Type, q.Status)
	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"documents": toSummaryList(items),
		"total":     len(items),
	})
}

// GetDocument handles GET /api/v1/kyc/documents/:id.
func (h *Handler) GetDocument(c *fiber.Ctx) error {
	userID, ok := userIDFromToken(c)
	if !ok {
		return respondError(c, fiber.StatusUnauthorized, "KYC_UNAUTHORIZED", "Not authenticated")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, "KYC_INVALID_ID", "Invalid document id")
	}
	st, err := h.svc.GetStatus(c.Context(), userID)
	if err != nil {
		return h.mapError(c, err)
	}
	for _, d := range st.Documents {
		if d.ID == id {
			return respondSuccess(c, fiber.StatusOK, toDocumentJSON(d, false))
		}
	}
	return respondError(c, fiber.StatusNotFound, "KYC_DOCUMENT_NOT_FOUND", "Document not found")
}

// DeleteDocument handles DELETE /api/v1/kyc/documents/:id.
func (h *Handler) DeleteDocument(c *fiber.Ctx) error {
	userID, ok := userIDFromToken(c)
	if !ok {
		return respondError(c, fiber.StatusUnauthorized, "KYC_UNAUTHORIZED", "Not authenticated")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, "KYC_INVALID_ID", "Invalid document id")
	}
	if err := h.svc.DeleteDocument(c.Context(), userID, id); err != nil {
		return h.mapError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// GetLevel handles GET /api/v1/kyc/level.
func (h *Handler) GetLevel(c *fiber.Ctx) error {
	userID, ok := userIDFromToken(c)
	if !ok {
		return respondError(c, fiber.StatusUnauthorized, "KYC_UNAUTHORIZED", "Not authenticated")
	}
	st, missing, err := h.svc.GetVerificationLevel(c.Context(), userID)
	if err != nil {
		return h.mapError(c, err)
	}
	limits := domain.LimitsForLevel(st.Record.Level)
	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"level":                st.Record.Level.String(),
		"level_code":           int(st.Record.Level),
		"limits":               limits,
		"pending_requirements": missing,
	})
}

// RequestUpgrade handles POST /api/v1/kyc/upgrade.
func (h *Handler) RequestUpgrade(c *fiber.Ctx) error {
	userID, ok := userIDFromToken(c)
	if !ok {
		return respondError(c, fiber.StatusUnauthorized, "KYC_UNAUTHORIZED", "Not authenticated")
	}
	var req UpgradeRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "KYC_INVALID_BODY", "Invalid request body")
	}
	if err := h.validate.Struct(req); err != nil {
		return respondValidationError(c, err.(validator.ValidationErrors))
	}
	missing, err := h.svc.RequestUpgrade(c.Context(), userID, domain.KYCLevel(req.TargetLevel))
	if err != nil {
		return h.mapError(c, err)
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"target_level":       domain.KYCLevel(req.TargetLevel).String(),
		"required_documents": missing,
	})
}

// ─── Admin endpoints (JWT + admin claim) ───

// VerifyDocument handles POST /api/v1/kyc/admin/documents/:id/verify.
func (h *Handler) VerifyDocument(c *fiber.Ctx) error {
	admin, ok := adminFromToken(c)
	if !ok {
		return respondError(c, fiber.StatusForbidden, "KYC_FORBIDDEN", "Admin access required")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, "KYC_INVALID_ID", "Invalid document id")
	}
	var req VerifyRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "KYC_INVALID_BODY", "Invalid request body")
	}
	if err := h.validate.Struct(req); err != nil {
		return respondValidationError(c, err.(validator.ValidationErrors))
	}
	doc, err := h.svc.VerifyDocument(c.Context(), service.VerifyInput{
		DocumentID: id, VerifiedBy: admin,
		Notes: req.VerificationNotes, ExtractedData: req.ExtractedData,
		FullName: req.FullName, BirthYear: req.BirthYear, CountryCode: req.CountryCode,
	})
	if err != nil {
		return h.mapError(c, err)
	}
	return respondSuccess(c, fiber.StatusOK, toDocumentJSON(doc, true))
}

// RejectDocument handles POST /api/v1/kyc/admin/documents/:id/reject.
func (h *Handler) RejectDocument(c *fiber.Ctx) error {
	admin, ok := adminFromToken(c)
	if !ok {
		return respondError(c, fiber.StatusForbidden, "KYC_FORBIDDEN", "Admin access required")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, "KYC_INVALID_ID", "Invalid document id")
	}
	var req RejectRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "KYC_INVALID_BODY", "Invalid request body")
	}
	if err := h.validate.Struct(req); err != nil {
		return respondValidationError(c, err.(validator.ValidationErrors))
	}
	doc, err := h.svc.RejectDocument(c.Context(), service.RejectInput{
		DocumentID: id, RejectedBy: admin,
		Reason: req.RejectionReason, RequiredActions: req.RequiredActions,
	})
	if err != nil {
		return h.mapError(c, err)
	}
	return respondSuccess(c, fiber.StatusOK, toDocumentJSON(doc, true))
}

// ListScreenings handles GET /api/v1/kyc/admin/screenings.
func (h *Handler) ListScreenings(c *fiber.Ctx) error {
	if _, ok := adminFromToken(c); !ok {
		return respondError(c, fiber.StatusForbidden, "KYC_FORBIDDEN", "Admin access required")
	}
	items, err := h.svc.ListPendingScreenings(c.Context(), 50)
	if err != nil {
		return h.mapError(c, err)
	}
	return respondSuccess(c, fiber.StatusOK, items)
}

// ResolveScreening handles POST /api/v1/kyc/admin/screenings/:id/resolve.
func (h *Handler) ResolveScreening(c *fiber.Ctx) error {
	admin, ok := adminFromToken(c)
	if !ok {
		return respondError(c, fiber.StatusForbidden, "KYC_FORBIDDEN", "Admin access required")
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return respondError(c, fiber.StatusBadRequest, "KYC_INVALID_ID", "Invalid screening id")
	}
	var req ResolveScreeningRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "KYC_INVALID_BODY", "Invalid request body")
	}
	if err := h.validate.Struct(req); err != nil {
		return respondValidationError(c, err.(validator.ValidationErrors))
	}
	sc, err := h.svc.ResolveScreening(c.Context(), admin, id, req.Approve, req.Note)
	if err != nil {
		return h.mapError(c, err)
	}
	return respondSuccess(c, fiber.StatusOK, sc)
}

// SweepExpired handles POST /api/v1/kyc/admin/sweep-expired (cron trigger).
func (h *Handler) SweepExpired(c *fiber.Ctx) error {
	if _, ok := adminFromToken(c); !ok {
		return respondError(c, fiber.StatusForbidden, "KYC_FORBIDDEN", "Admin access required")
	}
	n, err := h.svc.SweepExpired(c.Context())
	if err != nil {
		return h.mapError(c, err)
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"marked_expired": n})
}

// Webhook handles POST /api/v1/kyc/webhooks/:provider (HMAC, no JWT).
func (h *Handler) Webhook(c *fiber.Ctx) error {
	provider := c.Params("provider")
	if provider == "" {
		return respondError(c, fiber.StatusBadRequest, "KYC_INVALID_PROVIDER", "Unknown provider")
	}
	var payload struct {
		EventID        string   `json:"event_id"`
		ExternalUserID int64    `json:"external_user_id"`
		ReviewAnswer   string   `json:"review_answer"`
		Level          int      `json:"level"`
		RejectLabels   []string `json:"reject_labels"`
	}
	if err := c.BodyParser(&payload); err != nil {
		return respondError(c, fiber.StatusBadRequest, "KYC_INVALID_BODY", "Invalid webhook body")
	}
	err := h.svc.ProcessWebhook(c.Context(), service.WebhookInput{
		Provider: provider, EventID: payload.EventID,
		ExternalUserID: payload.ExternalUserID, ReviewAnswer: payload.ReviewAnswer,
		Level: payload.Level, RejectLabels: payload.RejectLabels,
		RawBody: c.Body(), Signature: c.Get("X-Webhook-Signature"),
	})
	if err != nil {
		// Duplicate deliveries are ACKed — providers retry aggressively.
		if errors.Is(err, domain.ErrDuplicateWebhook) {
			return c.SendStatus(fiber.StatusOK)
		}
		return h.mapError(c, err)
	}
	return c.SendStatus(fiber.StatusOK)
}

// Liveness answers GET /health.
func (h *Handler) Liveness(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok", "service": "kyc"})
}

// mapError converts domain errors to HTTP responses (single mapping place).
func (h *Handler) mapError(c *fiber.Ctx, err error) error {
	var ve *domain.ValidationError
	if errors.As(err, &ve) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{
				"code": "KYC_VALIDATION_FAILED", "message": "Validation failed",
				"details": ve.Fields,
			},
			"meta": buildMeta(c),
		})
	}
	switch {
	case errors.Is(err, domain.ErrDocumentNotFound),
		errors.Is(err, domain.ErrKYCNotFound),
		errors.Is(err, domain.ErrScreeningNotFound):
		return respondError(c, fiber.StatusNotFound, "KYC_NOT_FOUND", "Resource not found")
	case errors.Is(err, domain.ErrInvalidDocument),
		errors.Is(err, domain.ErrInvalidLevelTransition),
		errors.Is(err, domain.ErrInvalidUserID),
		errors.Is(err, domain.ErrUnsupportedProvider):
		return respondError(c, fiber.StatusBadRequest, "KYC_INVALID_REQUEST", err.Error())
	case errors.Is(err, domain.ErrDocumentAlreadyUsed):
		return respondError(c, fiber.StatusConflict, "KYC_DOCUMENT_CONFLICT", "Document is already registered to another user")
	case errors.Is(err, domain.ErrKYCAlreadyVerified):
		return respondError(c, fiber.StatusConflict, "KYC_ALREADY_VERIFIED", "Verification level already reached")
	case errors.Is(err, domain.ErrDocumentNotDeletable):
		return respondError(c, fiber.StatusConflict, "KYC_DOCUMENT_LOCKED", "Document cannot be deleted in its current status")
	case errors.Is(err, domain.ErrSanctionsMatch),
		errors.Is(err, domain.ErrKYCRejected):
		return respondError(c, fiber.StatusForbidden, "KYC_COMPLIANCE_HOLD", err.Error())
	case errors.Is(err, domain.ErrInvalidWebhookSig):
		return respondError(c, fiber.StatusUnauthorized, "KYC_INVALID_SIGNATURE", "Invalid webhook signature")
	case errors.Is(err, domain.ErrScreeningNotPending):
		return respondError(c, fiber.StatusConflict, "KYC_SCREENING_SETTLED", "Screening is already resolved")
	default:
		h.log.Error("kyc: internal error", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "KYC_INTERNAL_ERROR", "An internal error occurred")
	}
}
