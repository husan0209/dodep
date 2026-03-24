package main

import (
	"github.com/gofiber/fiber/v2"

	"github.com/opus-casino/auth/internal/service"
)

func setupRoutes(app *fiber.App, authService *service.AuthService) {
	api := app.Group("/api/v1")

	// Auth routes
	auth := api.Group("/auth")
	auth.Post("/register", func(c *fiber.Ctx) error {
		var req struct {
			Email        string `json:"email"`
			Password     string `json:"password"`
			Username     string `json:"username"`
			CountryCode  string `json:"country_code"`
			CurrencyCode string `json:"currency_code"`
			DeviceID     string `json:"device_id"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		result, err := authService.Register(c.Context(), &service.RegisterRequest{
			Email:        req.Email,
			Password:     req.Password,
			Username:     req.Username,
			CountryCode:  req.CountryCode,
			CurrencyCode: req.CurrencyCode,
			DeviceID:     req.DeviceID,
			IPAddress:    c.IP(),
		})
		if err != nil {
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

		result, err := authService.Login(c.Context(), &service.LoginRequest{
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

	auth.Post("/logout", func(c *fiber.Ctx) error {
		var req struct {
			UserID    int64  `json:"user_id"`
			SessionID string `json:"session_id"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		if err := authService.Logout(c.Context(), req.UserID, req.SessionID); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"success": true})
	})

	auth.Post("/2fa/enable", func(c *fiber.Ctx) error {
		var req struct {
			UserID int64 `json:"user_id"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		secret, qrURI, backupCodes, err := authService.Enable2FA(c.Context(), req.UserID)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{
			"secret":       secret,
			"qr_code_uri":  qrURI,
			"backup_codes": backupCodes,
		})
	})

	auth.Post("/2fa/verify", func(c *fiber.Ctx) error {
		var req struct {
			UserID   int64  `json:"user_id"`
			TOTPCode string `json:"totp_code"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		if err := authService.Verify2FA(c.Context(), req.UserID, req.TOTPCode); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"success": true})
	})

	auth.Post("/2fa/disable", func(c *fiber.Ctx) error {
		var req struct {
			UserID   int64  `json:"user_id"`
			TOTPCode string `json:"totp_code"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		if err := authService.Disable2FA(c.Context(), req.UserID, req.TOTPCode); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"success": true})
	})

	auth.Post("/change-password", func(c *fiber.Ctx) error {
		var req struct {
			UserID          int64  `json:"user_id"`
			CurrentPassword string `json:"current_password"`
			NewPassword     string `json:"new_password"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		if err := authService.ChangePassword(c.Context(), req.UserID, req.CurrentPassword, req.NewPassword); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"success": true})
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
}
