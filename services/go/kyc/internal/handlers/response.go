package handlers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/opus-casino/kyc/internal/domain"
	"github.com/opus-casino/kyc/internal/service"
)

// SuccessResponse is the standard success envelope.
type SuccessResponse struct {
	Data interface{} `json:"data"`
	Meta Meta        `json:"meta"`
}

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
	Meta  Meta      `json:"meta"`
}

// ErrorBody carries a namespaced code (KYC_*: 7000-7999).
type ErrorBody struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// Meta carries request correlation data.
type Meta struct {
	RequestID string `json:"request_id"`
	Timestamp string `json:"timestamp"`
}

func respondSuccess(c *fiber.Ctx, status int, data interface{}) error {
	return c.Status(status).JSON(SuccessResponse{Data: data, Meta: buildMeta(c)})
}

func respondError(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(ErrorResponse{
		Error: ErrorBody{Code: code, Message: message},
		Meta:  buildMeta(c),
	})
}

func respondValidationError(c *fiber.Ctx, errs interface{}) error {
	return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
		Error: ErrorBody{Code: "KYC_VALIDATION_FAILED", Message: "Validation failed", Details: errs},
		Meta:  buildMeta(c),
	})
}

func buildMeta(c *fiber.Ctx) Meta {
	return Meta{
		RequestID: c.GetRespHeader("X-Request-ID", ""),
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	}
}

// userIDFromToken reads the user identity set by AuthMiddleware (JWT).
func userIDFromToken(c *fiber.Ctx) (int64, bool) {
	raw, _ := c.Locals("user_id").(string)
	if raw == "" {
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// adminFromToken returns the admin identity when the JWT carries admin rights.
func adminFromToken(c *fiber.Ctx) (string, bool) {
	raw, _ := c.Locals("user_id").(string)
	if raw == "" {
		return "", false
	}
	isAdmin, _ := c.Locals("is_admin").(bool)
	if !isAdmin {
		return "", false
	}
	return raw, true
}

func sideOrDefault(s string) domain.DocumentSide {
	if s == "" {
		return domain.SideSingle
	}
	return domain.DocumentSide(s)
}

// toStatusJSON renders the verification snapshot for REST clients.
func toStatusJSON(st *service.Status) fiber.Map {
	rec := st.Record
	return fiber.Map{
		"user_id":          rec.UserID,
		"level":            rec.Level.String(),
		"level_code":       int(rec.Level),
		"target_level":     rec.TargetLevel.String(),
		"status":           string(rec.Status),
		"screening_status": string(rec.ScreeningStatus),
		"rejection_reason": rec.RejectionReason,
		"submitted_at":     tsOrNil(rec.SubmittedAt),
		"verified_at":      tsOrNil(rec.VerifiedAt),
		"last_reviewed_at": tsOrNil(rec.LastReviewedAt),
		"reviewed_by":      rec.ReviewedBy,
		"documents":        toSummaryList(st.Documents),
		"limits":           domain.LimitsForLevel(rec.Level),
	}
}

func toSummaryList(docs []*domain.Document) []fiber.Map {
	out := make([]fiber.Map, 0, len(docs))
	for _, d := range docs {
		out = append(out, fiber.Map{
			"id": d.ID.String(), "document_type": string(d.Type),
			"status":           string(d.Status),
			"submitted_at":     d.SubmittedAt.UTC().Format(time.RFC3339),
			"reviewed_at":      tsOrNil(d.ReviewedAt),
			"rejection_reason": d.RejectionReason,
		})
	}
	return out
}

// toDocumentJSON renders one document. includePII is true only for admin
// review surfaces (admin panel); user-facing responses never carry the
// document number (PII minimisation).
func toDocumentJSON(d *domain.Document, includePII bool) fiber.Map {
	m := fiber.Map{
		"id": d.ID.String(), "document_type": string(d.Type),
		"document_side": string(d.Side), "status": string(d.Status),
		"file_id": d.FileID, "issuing_country": d.IssuingCountry,
		"expiry_date":  tsOrNil(d.ExpiresAt),
		"submitted_at": d.SubmittedAt.UTC().Format(time.RFC3339),
		"reviewed_at":  tsOrNil(d.ReviewedAt), "reviewed_by": d.ReviewedBy,
		"rejection_reason": d.RejectionReason, "required_actions": d.RequiredActions,
		"extracted_data": d.ExtractedData, "verification_notes": d.Notes,
	}
	if includePII {
		m["document_number"] = d.DocumentNumber
	}
	return m
}

func filterDocs(docs []*domain.Document, docType, status string) []*domain.Document {
	if docType == "" && status == "" {
		return docs
	}
	out := make([]*domain.Document, 0, len(docs))
	for _, d := range docs {
		if docType != "" && string(d.Type) != docType {
			continue
		}
		if status != "" && string(d.Status) != status {
			continue
		}
		out = append(out, d)
	}
	return out
}

func tsOrNil(t *time.Time) interface{} {
	if t == nil || t.IsZero() {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}
