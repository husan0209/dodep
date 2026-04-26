package main

import (
	"crypto/sha256"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/opus-casino/affiliate/internal/domain"
	"github.com/opus-casino/affiliate/internal/service"
)

// setupRoutes registers the partner-facing REST API endpoints on a pre-authed router group.
func setupRoutes(router fiber.Router, svc *service.AffiliateService) {

	router.Post("/enroll", func(c *fiber.Ctx) error {
		userID := getUserID(c)
		uid, err := strconv.ParseInt(userID, 10, 64)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid user_id in token"})
		}

		var req struct {
			Reason string `json:"reason"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		result, err := svc.EnrollAffiliate(c.Context(), service.EnrollAffiliateInput{
			UserID: uid,
			Reason: req.Reason,
		})
		if err != nil {
			return handleDomainError(c, err)
		}

		return c.Status(201).JSON(result)
	})

	router.Get("/profile", func(c *fiber.Ctx) error {
		userID := getUserID(c)
		uid, err := strconv.ParseInt(userID, 10, 64)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid user_id in token"})
		}

		profile, err := svc.GetProfileByUserID(c.Context(), uid)
		if err != nil {
			return handleDomainError(c, err)
		}
		if profile == nil {
			return c.Status(404).JSON(fiber.Map{"error": "affiliate profile not found"})
		}

		return c.JSON(profile)
	})

	router.Get("/dashboard", func(c *fiber.Ctx) error {
		affiliateID, err := getAffiliateIDFromToken(c, svc)
		if err != nil {
			return err
		}

		dashboard, err := svc.GetDashboard(c.Context(), affiliateID)
		if err != nil {
			return handleDomainError(c, err)
		}

		return c.JSON(dashboard)
	})

	router.Get("/links", func(c *fiber.Ctx) error {
		affiliateID, err := getAffiliateIDFromToken(c, svc)
		if err != nil {
			return err
		}

		links, err := svc.ListAffiliateLinks(c.Context(), affiliateID)
		if err != nil {
			return handleDomainError(c, err)
		}

		return c.JSON(fiber.Map{"links": links})
	})

	router.Post("/links", func(c *fiber.Ctx) error {
		affiliateID, err := getAffiliateIDFromToken(c, svc)
		if err != nil {
			return err
		}

		var req struct {
			CampaignName string `json:"campaign_name"`
			LandingPage  string `json:"landing_page"`
			UTMSource    string `json:"utm_source"`
			UTMMedium    string `json:"utm_medium"`
			UTMCampaign  string `json:"utm_campaign"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		link, err := svc.CreateAffiliateLink(c.Context(), service.CreateAffiliateLinkInput{
			AffiliateID:  affiliateID,
			CampaignName: req.CampaignName,
			LandingPage:  req.LandingPage,
			UTMSource:    req.UTMSource,
			UTMMedium:    req.UTMMedium,
			UTMCampaign:  req.UTMCampaign,
		})
		if err != nil {
			return handleDomainError(c, err)
		}

		return c.Status(201).JSON(link)
	})

	router.Get("/earnings", func(c *fiber.Ctx) error {
		affiliateID, err := getAffiliateIDFromToken(c, svc)
		if err != nil {
			return err
		}

		status := domain.EarningStatus(c.Query("status", ""))
		limit, _ := strconv.Atoi(c.Query("limit", "50"))

		earnings, err := svc.ListAffiliateEarnings(c.Context(), affiliateID, status, limit)
		if err != nil {
			return handleDomainError(c, err)
		}

		return c.JSON(fiber.Map{"earnings": earnings})
	})

	router.Get("/payouts", func(c *fiber.Ctx) error {
		affiliateID, err := getAffiliateIDFromToken(c, svc)
		if err != nil {
			return err
		}

		status := domain.PayoutStatus(c.Query("status", ""))
		page, pageSize := parsePagination(c)
		offset := (page - 1) * pageSize

		payouts, total, err := svc.ListPayoutsByAffiliate(c.Context(), affiliateID, status, pageSize, offset)
		if err != nil {
			return handleDomainError(c, err)
		}

		return c.JSON(paginatedResponse(payouts, total, page, pageSize))
	})

	router.Get("/payout-methods", func(c *fiber.Ctx) error {
		affiliateID, err := getAffiliateIDFromToken(c, svc)
		if err != nil {
			return err
		}

		methods, err := svc.ListPayoutMethods(c.Context(), affiliateID)
		if err != nil {
			return handleDomainError(c, err)
		}

		return c.JSON(fiber.Map{"methods": methods})
	})

	router.Post("/payout-methods", func(c *fiber.Ctx) error {
		affiliateID, err := getAffiliateIDFromToken(c, svc)
		if err != nil {
			return err
		}

		var req struct {
			MethodType    string `json:"method_type"`
			DisplayName   string `json:"display_name"`
			DetailsMasked string `json:"details_masked"`
			IsDefault     bool   `json:"is_default"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		method, err := svc.CreatePayoutMethod(c.Context(), service.CreatePayoutMethodInput{
			AffiliateID:   affiliateID,
			MethodType:    domain.PayoutMethodType(req.MethodType),
			DisplayName:   req.DisplayName,
			DetailsMasked: req.DetailsMasked,
			IsDefault:     req.IsDefault,
		})
		if err != nil {
			return handleDomainError(c, err)
		}

		return c.Status(201).JSON(method)
	})

	router.Put("/payout-methods/:id", func(c *fiber.Ctx) error {
		affiliateID, err := getAffiliateIDFromToken(c, svc)
		if err != nil {
			return err
		}
		methodID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid method id"})
		}

		var req struct {
			MethodType    string `json:"method_type"`
			DisplayName   string `json:"display_name"`
			DetailsMasked string `json:"details_masked"`
			IsDefault     bool   `json:"is_default"`
			IsVerified    bool   `json:"is_verified"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		method, err := svc.UpdatePayoutMethod(c.Context(), service.UpdatePayoutMethodInput{
			AffiliateID:   affiliateID,
			MethodID:      methodID,
			MethodType:    domain.PayoutMethodType(req.MethodType),
			DisplayName:   req.DisplayName,
			DetailsMasked: req.DetailsMasked,
			IsDefault:     req.IsDefault,
			IsVerified:    req.IsVerified,
		})
		if err != nil {
			return handleDomainError(c, err)
		}

		return c.JSON(method)
	})

	router.Post("/payouts/request", func(c *fiber.Ctx) error {
		affiliateID, err := getAffiliateIDFromToken(c, svc)
		if err != nil {
			return err
		}

		var req struct {
			MethodID       string `json:"method_id"`
			Amount         string `json:"amount"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		methodID, err := uuid.Parse(req.MethodID)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid method_id"})
		}
		amount, err := decimal.NewFromString(req.Amount)
		if err != nil || amount.IsNegative() || amount.IsZero() {
			return c.Status(400).JSON(fiber.Map{"error": "invalid amount"})
		}

		kycApproved := strings.EqualFold(c.Get("X-KYC-Approved", "false"), "true")
		payout, err := svc.RequestPayout(c.Context(), service.RequestPayoutInput{
			AffiliateID:    affiliateID,
			MethodID:       methodID,
			Amount:         amount,
			IdempotencyKey: req.IdempotencyKey,
			KYCApproved:    kycApproved,
			HasOpenFraud:   false,
		})
		if err != nil {
			return handleDomainError(c, err)
		}

		return c.Status(201).JSON(payout)
	})

	router.Get("/reports", func(c *fiber.Ctx) error {
		affiliateID, err := getAffiliateIDFromToken(c, svc)
		if err != nil {
			return err
		}

		dashboard, err := svc.GetDashboard(c.Context(), affiliateID)
		if err != nil {
			return handleDomainError(c, err)
		}
		earnings, err := svc.ListAffiliateEarnings(c.Context(), affiliateID, "", 100)
		if err != nil {
			return handleDomainError(c, err)
		}

		return c.JSON(fiber.Map{
			"summary":  dashboard,
			"earnings": earnings,
		})
	})
}

// trackClickHandler creates a click handler for /r/:affiliate_code[/:campaign].
func trackClickHandler(svc *service.AffiliateService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		affiliateCode := c.Params("affiliate_code")
		campaign := c.Params("campaign", "")

		ipHash := hashString(c.IP())
		uaHash := hashString(c.Get("User-Agent"))

		click, err := svc.TrackAffiliateClick(c.Context(), service.TrackAffiliateClickInput{
			AffiliateCode:     affiliateCode,
			Campaign:          campaign,
			LandingPage:       c.Query("landing", "/"),
			IPHash:            ipHash,
			UserAgentHash:     uaHash,
			DeviceFingerprint: c.Get("X-Device-Fingerprint", ""),
			CountryCode:       c.Get("CF-IPCountry", "XX"),
		})
		if err != nil {
			// Silent fail for tracking — don't block user
			return c.Redirect("/", 302)
		}

		// Set affiliate cookie for later attribution
		c.Cookie(&fiber.Cookie{
			Name:     "aff_click",
			Value:    click.ClickID,
			MaxAge:   30 * 24 * 60 * 60, // 30 days
			HTTPOnly: true,
			Secure:   true,
			SameSite: "Lax",
		})

		landingPage := c.Query("landing", "/")
		return c.Redirect(landingPage, 302)
	}
}

// getAffiliateIDFromToken resolves the affiliate UUID from the JWT user_id.
func getAffiliateIDFromToken(c *fiber.Ctx, svc *service.AffiliateService) (uuid.UUID, error) {
	userID := getUserID(c)
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return uuid.Nil, fiber.NewError(400, "invalid user_id in token")
	}

	profile, err := svc.GetProfileByUserID(c.Context(), uid)
	if err != nil {
		return uuid.Nil, fiber.NewError(500, "internal error")
	}
	if profile == nil {
		return uuid.Nil, fiber.NewError(404, "affiliate profile not found")
	}

	return profile.ID, nil
}

func parsePagination(c *fiber.Ctx) (int, int) {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

func paginatedResponse(data any, total int64, page int, pageSize int) fiber.Map {
	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	if totalPages == 0 {
		totalPages = 1
	}
	return fiber.Map{
		"data": data,
		"pagination": fiber.Map{
			"page":       page,
			"page_size":  pageSize,
			"total":      total,
			"total_pages": totalPages,
		},
	}
}

// handleDomainError maps domain errors to HTTP status codes.
func handleDomainError(c *fiber.Ctx, err error) error {
	switch err {
	case domain.ErrEnrollmentAlreadyPending:
		return c.Status(409).JSON(fiber.Map{"error": err.Error()})
	case domain.ErrAffiliateNotFound, domain.ErrEnrollmentNotFound,
		domain.ErrAffiliateLinkNotFound, domain.ErrPayoutMethodNotFound,
		domain.ErrPayoutNotFound:
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	case domain.ErrAffiliateAlreadyExists, domain.ErrAttributionAlreadyBound:
		return c.Status(409).JSON(fiber.Map{"error": err.Error()})
	case domain.ErrSelfReferral, domain.ErrInvalidPayoutAmount,
		domain.ErrMinPayoutNotReached, domain.ErrInvalidPayoutStatus,
		domain.ErrInvalidCommissionAmount:
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	case domain.ErrAffiliateKYCRequired, domain.ErrAffiliateFraudBlocked,
		domain.ErrAffiliateInactive:
		return c.Status(403).JSON(fiber.Map{"error": err.Error()})
	default:
		return c.Status(500).JSON(fiber.Map{"error": "internal server error"})
	}
}

func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:8])
}
