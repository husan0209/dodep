package main

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/opus-casino/auth/internal/domain"
	"github.com/opus-casino/auth/internal/service"
)

func setupRoutes(app *fiber.App, authService *service.AuthService) {
	api := app.Group("/api/v1")

	// ===== PUBLIC routes (no auth required) =====
	auth := api.Group("/auth")
	auth.Post("/register", func(c *fiber.Ctx) error {
		var req struct {
			Email        string `json:"email"`
			Password     string `json:"password"`
			Username     string `json:"username"`
			CountryCode  string `json:"country_code" json:"countryCode"`
			CurrencyCode string `json:"currency_code" json:"currencyCode"`
			DeviceID     string `json:"device_id" json:"deviceId"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		result, err := authService.Register(c.Context(), &domain.RegisterRequest{
			Email:        req.Email,
			Password:     req.Password,
			Username:     req.Username,
			CountryCode:  req.CountryCode,
			CurrencyCode: req.CurrencyCode,
			DeviceID:     req.DeviceID,
			IPAddress:    c.IP(),
		})
		if err != nil {
			authService.Log().Error("Register attempt failed", zap.Error(err), zap.String("email", req.Email))
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}

		return c.Status(201).JSON(result)
	})

	auth.Post("/login", func(c *fiber.Ctx) error {
		var req struct {
			Email      string  `json:"email"`
			Password   string  `json:"password"`
			DeviceID   string  `json:"device_id"`
			TOTPCode   *string `json:"totp_code,omitempty"`
			RememberMe bool    `json:"remember_me"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		result, err := authService.Login(c.Context(), &domain.LoginRequest{
			Email:      req.Email,
			Password:   req.Password,
			DeviceID:   req.DeviceID,
			IPAddress:  c.IP(),
			TOTPCode:   req.TOTPCode,
			RememberMe: req.RememberMe,
		})
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(result)
	})

	auth.Post("/refresh", func(c *fiber.Ctx) error {
		var req struct {
			RefreshToken string `json:"refresh_token"`
			DeviceID     string `json:"device_id"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		tokens, err := authService.RefreshTokens(c.Context(), req.RefreshToken, req.DeviceID)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(tokens)
	})

	auth.Post("/validate", func(c *fiber.Ctx) error {
		var req struct {
			AccessToken string `json:"access_token"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		claims, err := authService.ValidateToken(c.Context(), req.AccessToken)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{
				"valid": false,
				"error": err.Error(),
			})
		}

		return c.JSON(fiber.Map{
			"valid":      true,
			"user_id":    claims.UserID,
			"session_id": claims.SessionID,
		})
	})

	// ===== PROTECTED routes (JWT required — user_id comes from token) =====
	protected := auth.Group("", AuthMiddleware(authService))

	protected.Post("/logout", func(c *fiber.Ctx) error {
		userID := getUserID(c)
		sessionID := getSessionID(c)

		if err := authService.Logout(c.Context(), userID, sessionID); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"success": true})
	})

	protected.Post("/2fa/enable", func(c *fiber.Ctx) error {
		userID := getUserID(c)

		secret, qrURI, backupCodes, err := authService.Enable2FA(c.Context(), userID)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{
			"secret":       secret,
			"qr_code_uri":  qrURI,
			"backup_codes": backupCodes,
		})
	})

	protected.Post("/2fa/verify", func(c *fiber.Ctx) error {
		userID := getUserID(c)

		var req struct {
			TOTPCode string `json:"totp_code"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		if err := authService.Verify2FA(c.Context(), userID, req.TOTPCode); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"success": true})
	})

	protected.Post("/2fa/disable", func(c *fiber.Ctx) error {
		userID := getUserID(c)

		var req struct {
			TOTPCode string `json:"totp_code"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		if err := authService.Disable2FA(c.Context(), userID, req.TOTPCode); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"success": true})
	})

	protected.Post("/change-password", func(c *fiber.Ctx) error {
		userID := getUserID(c)

		var req struct {
			CurrentPassword string `json:"current_password"`
			NewPassword     string `json:"new_password"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		if err := authService.ChangePassword(c.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"success": true})
	})

	// ===== ADMIN routes (temporary for testing) =====
	admin := api.Group("/admin/auth")

	admin.Post("/login", func(c *fiber.Ctx) error {
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		// Simple test admin credentials
		if req.Email == "testadmin123@test.com" && req.Password == "Admin@123456" {
			return c.JSON(fiber.Map{
				"data": fiber.Map{
					"access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiYWRtaW4tdGVzdCIsInJvbGUiOiJhZG1pbiIsImlzcyI6Im9wdXMtY2FzaW5vLWF1dGgiLCJleHAiOjk5OTk5OTk5OTksImlhdCI6MTc3NzE5ODg2Nn0.test_signature",
					"refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJvcHVzLWNhc2luby1hdXRoIiwiZXhwIjo5OTk5OTk5OTk5LCJpYXQiOjE3NzcxOTg4NjZ9.test_refresh",
					"admin": fiber.Map{
						"id":    "admin-test",
						"email": req.Email,
						"name":  "Test Admin",
						"role":  "admin",
						"permissions": []string{
							"users.read", "users.write", "users.delete",
							"finance.read", "finance.write",
							"casino.read", "casino.write",
							"bonuses.read", "bonuses.write",
							"risk.read", "risk.write",
							"affiliates.read", "affiliates.write",
							"system.read", "system.write",
						},
					},
					"expires_in": 3600,
				},
				"meta": fiber.Map{
					"request_id": c.Get("X-Request-ID", "admin-login"),
					"timestamp":  time.Now().UTC().Format(time.RFC3339),
				},
			})
		}

		return c.Status(401).JSON(fiber.Map{"error": "invalid credentials"})
	})

	admin.Post("/refresh", func(c *fiber.Ctx) error {
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		return c.JSON(fiber.Map{
			"data": fiber.Map{
				"access_token":  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiYWRtaW4tdGVzdCIsInJvbGUiOiJhZG1pbiIsImlzcyI6Im9wdXMtY2FzaW5vLWF1dGgiLCJleHAiOjk5OTk5OTk5OTksImlhdCI6MTc3NzE5ODg2Nn0.test_signature",
				"refresh_token": req.RefreshToken,
			},
			"meta": fiber.Map{
				"request_id": c.Get("X-Request-ID", "admin-refresh"),
				"timestamp":  time.Now().UTC().Format(time.RFC3339),
			},
		})
	})

	systemAdmin := api.Group("/admin/system")
	systemAdmin.Get("/dashboard", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"data": fiber.Map{
				"total_users":             12500,
				"active_users_today":      1200,
				"new_users_today":         45,
				"total_deposits_today":    "45000.00",
				"total_withdrawals_today": "12000.00",
				"bets_placed_today":       4500,
				"ggr_today":               "15000.00",
				"open_fraud_alerts":       3,
				"pending_withdrawals":     12,
				"pending_kyc_reviews":     8,
			},
		})
	})

	financeAdmin := api.Group("/admin/finance")
	financeAdmin.Get("/summary", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"data": fiber.Map{
				"total_deposits":             "1250000.00",
				"total_withdrawals":          "450000.00",
				"net_revenue":                "800000.00",
				"ggr":                        "850000.00",
				"pending_withdrawals_count":  12,
				"pending_withdrawals_amount": "15000.00",
			},
		})
	})
}

