package main

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/opus-casino/auth/internal/domain"
	"github.com/opus-casino/auth/internal/service"
)

func setupRoutes(app *fiber.App, authService *service.AuthService) {
	api := app.Group("/api/v1")

	// ===== PUBLIC routes (no auth required) =====
	auth := api.Group("/auth")
	auth.Post("/register", func(c *fiber.Ctx) error {
		requestID := getOrCreateRequestID(c)
		startedAt := time.Now()
		var req struct {
			Email        string `json:"email"`
			Password     string `json:"password"`
			Username     string `json:"username"`
			CountryCode  string `json:"country_code"`
			CountryCode2 string `json:"countryCode"`
			CurrencyCode string `json:"currency_code"`
			CurrencyCode2 string `json:"currencyCode"`
			DeviceID     string `json:"device_id"`
			DeviceID2    string `json:"deviceId"`
		}

		if err := c.BodyParser(&req); err != nil {
			authService.Log().Warn("Register body parse failed",
				zap.Error(err),
				zap.String("content_type", c.Get("Content-Type")),
				zap.Int("body_len", len(c.Body())),
				zap.String("request_id", requestID),
			)
			return writeAPIError(c, fiber.StatusBadRequest, "AUTH_INVALID_REQUEST", "invalid request body", fiber.Map{"reason": err.Error()})
		}
		if req.CountryCode == "" {
			req.CountryCode = req.CountryCode2
		}
		if req.CurrencyCode == "" {
			req.CurrencyCode = req.CurrencyCode2
		}
		if req.DeviceID == "" {
			req.DeviceID = req.DeviceID2
		}

		authService.Log().Info("Register request received",
			zap.String("request_id", requestID),
			zap.String("origin", c.Get("Origin")),
			zap.String("ip", c.IP()),
			zap.String("email", req.Email),
			zap.String("username", req.Username),
			zap.String("country_code", req.CountryCode),
			zap.String("currency_code", req.CurrencyCode),
		)

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
			status, code, message := mapAuthError(err)
			authService.Log().Warn("Register failed",
				zap.Int("status", status),
				zap.String("code", code),
				zap.String("request_id", requestID),
				zap.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
				zap.String("email", req.Email),
			)
			return writeAPIError(c, status, code, message, nil)
		}

		authService.Log().Info("Register succeeded",
			zap.String("request_id", requestID),
			zap.String("user_id", result.UserID),
			zap.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
		)

		return c.Status(201).JSON(result)
	})

	auth.Post("/login", func(c *fiber.Ctx) error {
		var req struct {
			Identifier string  `json:"identifier"`
			Username   string  `json:"username"`
			Email      string  `json:"email"`
			Password   string  `json:"password"`
			DeviceID   string  `json:"device_id"`
			DeviceID2  string  `json:"deviceId"`
			TOTPCode   *string `json:"totp_code,omitempty"`
			RememberMe bool    `json:"remember_me"`
		}

		if err := c.BodyParser(&req); err != nil {
			authService.Log().Warn("Login body parse failed",
				zap.Error(err),
				zap.String("content_type", c.Get("Content-Type")),
				zap.Int("body_len", len(c.Body())),
				zap.String("request_id", getOrCreateRequestID(c)),
			)
			return writeAPIError(c, fiber.StatusBadRequest, "AUTH_INVALID_REQUEST", "invalid request body", fiber.Map{"reason": err.Error()})
		}
		if req.Identifier == "" {
			req.Identifier = req.Username
		}
		if req.DeviceID == "" {
			req.DeviceID = req.DeviceID2
		}

		result, err := authService.Login(c.Context(), &domain.LoginRequest{
			Identifier: req.Identifier,
			Email:      req.Email,
			Password:   req.Password,
			DeviceID:   req.DeviceID,
			IPAddress:  c.IP(),
			TOTPCode:   req.TOTPCode,
			RememberMe: req.RememberMe,
		})
		if err != nil {
			status, code, message := mapAuthError(err)
			authService.Log().Warn("Login failed",
				zap.Int("status", status),
				zap.String("code", code),
				zap.String("request_id", getOrCreateRequestID(c)),
				zap.String("identifier", req.Identifier),
				zap.String("email", req.Email),
				zap.String("ip", c.IP()),
			)
			return writeAPIError(c, status, code, message, nil)
		}

		return c.JSON(result)
	})

	auth.Post("/refresh", func(c *fiber.Ctx) error {
		var req struct {
			RefreshToken string `json:"refresh_token"`
			DeviceID     string `json:"device_id"`
		}

		if err := c.BodyParser(&req); err != nil {
			return writeAPIError(c, fiber.StatusBadRequest, "AUTH_INVALID_REQUEST", "invalid request body", nil)
		}

		tokens, err := authService.RefreshTokens(c.Context(), req.RefreshToken, req.DeviceID)
		if err != nil {
			status, code, message := mapAuthError(err)
			return writeAPIError(c, status, code, message, nil)
		}

		return c.JSON(tokens)
	})

	auth.Get("/google/start", func(c *fiber.Ctx) error {
		authURL, err := authService.BuildGoogleAuthURL(c.Context())
		if err != nil {
			status, code, message := mapAuthError(err)
			return writeAPIError(c, status, code, message, nil)
		}
		return c.Redirect(authURL, fiber.StatusTemporaryRedirect)
	})

	auth.Get("/google/callback", func(c *fiber.Ctx) error {
		code := strings.TrimSpace(c.Query("code"))
		state := strings.TrimSpace(c.Query("state"))
		if code == "" || state == "" {
			return writeAPIError(c, fiber.StatusBadRequest, "AUTH_OAUTH_INVALID_CALLBACK", "missing code or state", nil)
		}

		result, err := authService.LoginWithGoogleCallback(c.Context(), code, state, c.IP())
		if err != nil {
			status, errCode, message := mapAuthError(err)
			authService.Log().Warn("Google callback failed",
				zap.Int("status", status),
				zap.String("code", errCode),
				zap.String("request_id", getOrCreateRequestID(c)),
			)
			q := url.Values{}
			q.Set("error_code", errCode)
			q.Set("error_message", message)
			return c.Redirect(authService.WebRedirectURL()+"/login?"+q.Encode(), fiber.StatusTemporaryRedirect)
		}

		q := url.Values{}
		q.Set("access_token", result.Tokens.AccessToken)
		q.Set("refresh_token", result.Tokens.RefreshToken)
		q.Set("expires_in", fmt.Sprintf("%d", result.Tokens.ExpiresIn))
		q.Set("refresh_expires_in", fmt.Sprintf("%d", result.Tokens.RefreshExpiresIn))
		q.Set("token_type", result.Tokens.TokenType)
		q.Set("user_id", result.UserID)

		return c.Redirect(authService.WebRedirectURL()+"/auth/google/callback?"+q.Encode(), fiber.StatusTemporaryRedirect)
	})

	auth.Post("/validate", func(c *fiber.Ctx) error {
		var req struct {
			AccessToken string `json:"access_token"`
		}

		if err := c.BodyParser(&req); err != nil {
			return writeAPIError(c, fiber.StatusBadRequest, "AUTH_INVALID_REQUEST", "invalid request body", nil)
		}

		claims, err := authService.ValidateToken(c.Context(), req.AccessToken)
		if err != nil {
			return writeAPIError(c, fiber.StatusUnauthorized, "AUTH_INVALID_TOKEN", "invalid token", fiber.Map{"valid": false})
		}

		return c.JSON(fiber.Map{
			"valid":      true,
			"user_id":    claims.UserID,
			"session_id": claims.SessionID,
		})
	})

	// ===== PROTECTED routes (JWT required — user_id comes from token) =====
	protected := auth.Group("", AuthMiddleware(authService))

	protected.Get("/me", func(c *fiber.Ctx) error {
		userID := getUserID(c)
		user, err := authService.GetCurrentUser(c.Context(), userID)
		if err != nil {
			status, code, message := mapAuthError(err)
			return writeAPIError(c, status, code, message, nil)
		}

		return c.JSON(user)
	})

	protected.Post("/logout", func(c *fiber.Ctx) error {
		userID := getUserID(c)
		sessionID := getSessionID(c)

		if err := authService.Logout(c.Context(), userID, sessionID); err != nil {
			status, code, message := mapAuthError(err)
			return writeAPIError(c, status, code, message, nil)
		}

		return c.JSON(fiber.Map{"success": true})
	})

	protected.Post("/2fa/enable", func(c *fiber.Ctx) error {
		userID := getUserID(c)

		secret, qrURI, backupCodes, err := authService.Enable2FA(c.Context(), userID)
		if err != nil {
			return writeAPIError(c, fiber.StatusBadRequest, "AUTH_2FA_ENABLE_FAILED", err.Error(), nil)
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
			return writeAPIError(c, fiber.StatusBadRequest, "AUTH_INVALID_REQUEST", "invalid request body", nil)
		}

		if err := authService.Verify2FA(c.Context(), userID, req.TOTPCode); err != nil {
			return writeAPIError(c, fiber.StatusBadRequest, "AUTH_2FA_VERIFY_FAILED", err.Error(), nil)
		}

		return c.JSON(fiber.Map{"success": true})
	})

	protected.Post("/2fa/disable", func(c *fiber.Ctx) error {
		userID := getUserID(c)

		var req struct {
			TOTPCode string `json:"totp_code"`
		}

		if err := c.BodyParser(&req); err != nil {
			return writeAPIError(c, fiber.StatusBadRequest, "AUTH_INVALID_REQUEST", "invalid request body", nil)
		}

		if err := authService.Disable2FA(c.Context(), userID, req.TOTPCode); err != nil {
			return writeAPIError(c, fiber.StatusBadRequest, "AUTH_2FA_DISABLE_FAILED", err.Error(), nil)
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
			return writeAPIError(c, fiber.StatusBadRequest, "AUTH_INVALID_REQUEST", "invalid request body", nil)
		}

		if err := authService.ChangePassword(c.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
			return writeAPIError(c, fiber.StatusBadRequest, "AUTH_CHANGE_PASSWORD_FAILED", err.Error(), nil)
		}

		return c.JSON(fiber.Map{"success": true})
	})

	// Admin authentication is intentionally NOT served from this service.
	// All admin login / refresh / session management lives in admin-bff
	// (services/go/admin-bff) using its own dedicated credential store and
	// stricter security controls (Argon2id, Ed25519, audit logging, 2FA).
	// Do NOT add admin endpoints here — see docs/security/admin-auth.md.
}

func writeAPIError(c *fiber.Ctx, status int, code, message string, details fiber.Map) error {
	requestID := getOrCreateRequestID(c)

	return c.Status(status).JSON(fiber.Map{
		"error": fiber.Map{
			"code":    code,
			"message": message,
			"details": details,
		},
		"code":       code,
		"message":    message,
		"details":    details,
		"request_id": requestID,
	})
}

func getOrCreateRequestID(c *fiber.Ctx) string {
	requestID := c.Get("X-Request-ID")
	if requestID == "" {
		requestID = uuid.NewString()
	}
	return requestID
}

func mapAuthError(err error) (status int, code string, message string) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		return fiber.StatusUnprocessableEntity, "AUTH_VALIDATION_FAILED", err.Error()
	case errors.Is(err, domain.ErrUserAlreadyExists):
		return fiber.StatusConflict, "USER_ALREADY_EXISTS", "user already exists"
	case errors.Is(err, domain.ErrInvalidCredentials):
		return fiber.StatusUnauthorized, "AUTH_INVALID_CREDENTIALS", "invalid credentials"
	case errors.Is(err, domain.ErrAccountLocked):
		return fiber.StatusForbidden, "AUTH_ACCOUNT_LOCKED", "account is locked"
	case errors.Is(err, domain.ErrInvalidToken):
		return fiber.StatusUnauthorized, "AUTH_INVALID_TOKEN", "invalid token"
	case errors.Is(err, domain.ErrInvalidRefreshToken):
		return fiber.StatusUnauthorized, "AUTH_INVALID_REFRESH_TOKEN", "invalid or expired refresh token"
	case errors.Is(err, domain.ErrUserNotFound):
		return fiber.StatusNotFound, "AUTH_USER_NOT_FOUND", "user not found"
	case errors.Is(err, domain.ErrForbidden):
		return fiber.StatusForbidden, "AUTH_FORBIDDEN", "forbidden"
	default:
		return fiber.StatusInternalServerError, "INTERNAL_ERROR", "internal server error"
	}
}
