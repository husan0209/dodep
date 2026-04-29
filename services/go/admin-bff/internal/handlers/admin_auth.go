package handlers

import (
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/opus-casino/admin-bff/internal/service"
)

const refreshCookieName = "admin_refresh_token"
const refreshCookiePath = "/admin/auth"
const refreshCookieMaxAge = 7 * 24 * 3600 // 7 days

// cookieSecure reads $COOKIE_SECURE; defaults to true (production-safe).
// Set COOKIE_SECURE=false only for local http://localhost development.
func cookieSecure() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("COOKIE_SECURE")))
	return v != "false" && v != "0" && v != "no"
}

// setRefreshCookie issues an HttpOnly, SameSite=Strict refresh cookie scoped
// to /admin/auth so it is only sent on auth endpoints.
func setRefreshCookie(c *fiber.Ctx, refreshToken string) {
	c.Cookie(&fiber.Cookie{
		Name:     refreshCookieName,
		Value:    refreshToken,
		Path:     refreshCookiePath,
		MaxAge:   refreshCookieMaxAge,
		Expires:  time.Now().Add(time.Duration(refreshCookieMaxAge) * time.Second),
		HTTPOnly: true,
		Secure:   cookieSecure(),
		SameSite: "Strict",
	})
}

// clearRefreshCookie expires the refresh cookie immediately. Path/Name MUST
// match setRefreshCookie or the browser will keep the old cookie.
func clearRefreshCookie(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     refreshCookiePath,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HTTPOnly: true,
		Secure:   cookieSecure(),
		SameSite: "Strict",
	})
}

// RegisterPublicAdminAuthRoutes registers admin auth endpoints that MUST NOT
// require an existing JWT (login, refresh). The router is expected to be the
// public group (e.g. "/admin/auth") without AdminAuth middleware applied.
func RegisterPublicAdminAuthRoutes(router fiber.Router, db *gorm.DB, log *zap.Logger, jwtSecret string) {
	svc := service.NewAdminAuthService(db, jwtSecret)

	router.Post("/login", func(c *fiber.Ctx) error {
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}
		ip := c.IP()
		ua := string(c.Request().Header.UserAgent())
		result, err := svc.Login(c.Context(), req.Email, req.Password, ip, ua)
		if err != nil {
			log.Warn("admin login failed", zap.Error(err), zap.String("email", req.Email))
			return c.Status(401).JSON(fiber.Map{"error": "invalid credentials"})
		}
		// Step 1 complete — TOTP required
		if result.TOTPRequired {
			return c.JSON(fiber.Map{
				"data": fiber.Map{
					"totp_required":   true,
					"challenge_token": result.ChallengeToken,
				},
			})
		}
		// No TOTP — full token pair
		setRefreshCookie(c, result.RefreshToken)
		return c.JSON(fiber.Map{
			"data": fiber.Map{
				"admin":        result.Admin,
				"access_token": result.AccessToken,
				"token_type":   "Bearer",
			},
		})
	})

	// Step 2: verify TOTP code (only reached when totp_enabled=true)
	router.Post("/totp-verify", func(c *fiber.Ctx) error {
		var req struct {
			ChallengeToken string `json:"challenge_token"`
			TOTPCode       string `json:"totp_code"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}
		ip := c.IP()
		ua := string(c.Request().Header.UserAgent())
		result, err := svc.VerifyTOTP(c.Context(), req.ChallengeToken, req.TOTPCode, ip, ua)
		if err != nil {
			log.Warn("admin totp verify failed", zap.Error(err))
			return c.Status(401).JSON(fiber.Map{"error": "invalid or expired totp"})
		}
		setRefreshCookie(c, result.RefreshToken)
		return c.JSON(fiber.Map{
			"data": fiber.Map{
				"admin":        result.Admin,
				"access_token": result.AccessToken,
				"token_type":   "Bearer",
			},
		})
	})

	router.Post("/refresh", func(c *fiber.Ctx) error {
		refreshToken := c.Cookies(refreshCookieName)
		if refreshToken == "" {
			return c.Status(401).JSON(fiber.Map{"error": "missing refresh token"})
		}

		access, refresh, err := svc.Refresh(c.Context(), refreshToken)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "invalid or expired refresh token"})
		}
		setRefreshCookie(c, refresh)
		return c.JSON(fiber.Map{
			"data": fiber.Map{
				"access_token": access,
				"token_type":   "Bearer",
			},
		})
	})
}

// RegisterProtectedAdminAuthRoutes registers admin auth endpoints that REQUIRE
// a valid admin JWT (logout, me). The router is expected to be the protected
// "/admin" group, with AdminAuth middleware already applied.
func RegisterProtectedAdminAuthRoutes(router fiber.Router, db *gorm.DB, log *zap.Logger, jwtSecret string) {
	svc := service.NewAdminAuthService(db, jwtSecret)
	auth := router.Group("/auth")

	auth.Post("/logout", func(c *fiber.Ctx) error {
		adminID, ok := c.Locals("admin_id").(float64)
		if !ok {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		if err := svc.Logout(c.Context(), int64(adminID)); err != nil {
			log.Error("logout failed", zap.Error(err))
		}
		clearRefreshCookie(c)
		return c.JSON(fiber.Map{"data": fiber.Map{"success": true}})
	})

	// GET /admin/auth/totp-setup — generate QR code for Google Authenticator enrollment.
	auth.Get("/totp-setup", func(c *fiber.Ctx) error {
		adminID, ok := c.Locals("admin_id").(float64)
		if !ok {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		result, err := svc.GenerateTOTPSetup(c.Context(), int64(adminID))
		if err != nil {
			log.Error("totp setup failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "failed to generate TOTP setup"})
		}
		return c.JSON(fiber.Map{"data": result})
	})

	// POST /admin/auth/totp-enable — confirm first TOTP code and activate.
	auth.Post("/totp-enable", func(c *fiber.Ctx) error {
		adminID, ok := c.Locals("admin_id").(float64)
		if !ok {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		var req struct {
			Secret string `json:"secret"`
			Code   string `json:"code"`
		}
		if err := c.BodyParser(&req); err != nil || req.Secret == "" || req.Code == "" {
			return c.Status(400).JSON(fiber.Map{"error": "secret and code required"})
		}
		if err := svc.EnableTOTP(c.Context(), int64(adminID), req.Secret, req.Code); err != nil {
			log.Warn("totp enable failed", zap.Error(err))
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"data": fiber.Map{"totp_enabled": true}})
	})

	// POST /admin/auth/totp-disable — disable TOTP for self (must verify current code first).
	auth.Post("/totp-disable", func(c *fiber.Ctx) error {
		adminID, ok := c.Locals("admin_id").(float64)
		if !ok {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		if err := svc.DisableTOTP(c.Context(), int64(adminID)); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to disable TOTP"})
		}
		return c.JSON(fiber.Map{"data": fiber.Map{"totp_enabled": false}})
	})

	auth.Get("/me", func(c *fiber.Ctx) error {
		adminID, ok := c.Locals("admin_id").(float64)
		if !ok {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		admin, err := svc.Me(c.Context(), int64(adminID))
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to load profile"})
		}
		return c.JSON(fiber.Map{"data": admin})
	})
}
