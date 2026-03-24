package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/opus-casino/auth/internal/crypto"
	"github.com/opus-casino/auth/internal/domain"
	"github.com/opus-casino/auth/internal/repository"
)

// AuthService handles authentication business logic
type AuthService struct {
	repo      *repository.AuthRepository
	jwtConfig *crypto.JWTConfig
	log       *zap.Logger
}

// NewAuthService creates a new auth service
func NewAuthService(repo *repository.AuthRepository, jwtSecretKey string, log *zap.Logger) *AuthService {
	return &AuthService{
		repo:      repo,
		jwtConfig: crypto.DefaultJWTConfig(jwtSecretKey),
		log:       log,
	}
}

// Register creates a new user account
func (s *AuthService) Register(ctx context.Context, req *domain.RegisterRequest) (*domain.AuthResult, error) {
	// Check if user already exists
	existing, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("email already registered")
	}

	// Hash password
	passwordHash, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := &domain.User{
		UUID:          uuid.New().String(),
		Email:         req.Email,
		Username:      req.Username,
		PasswordHash:  passwordHash,
		Status:        domain.UserStatusActive,
		CountryCode:   req.CountryCode,
		CurrencyCode:  req.CurrencyCode,
		TwoFAEnabled:  false,
		EmailVerified: false,
		PhoneVerified: false,
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Create session
	session, tokens, err := s.createSession(ctx, user, req.DeviceID, req.IPAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	s.log.Info("User registered", zap.Int64("user_id", user.ID), zap.String("email", user.Email))

	return &domain.AuthResult{
		UserID:  user.ID,
		Tokens:  tokens,
		Session: session,
	}, nil
}

// Login authenticates a user
func (s *AuthService) Login(ctx context.Context, req *domain.LoginRequest) (*domain.AuthResult, error) {
	// Check if account is locked
	locked, err := s.repo.IsAccountLocked(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check account lock: %w", err)
	}
	if locked {
		return nil, fmt.Errorf("account is locked, try again later")
	}

	// Get user
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		// Track failed attempt (but don't reveal user doesn't exist)
		s.repo.TrackLoginAttempt(ctx, req.Email, req.IPAddress)
		return nil, fmt.Errorf("invalid credentials")
	}

	// Verify password
	valid, err := crypto.VerifyPassword(req.Password, user.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("failed to verify password: %w", err)
	}
	if !valid {
		attempts, locked, err := s.repo.TrackLoginAttempt(ctx, req.Email, req.IPAddress)
		if err != nil {
			s.log.Error("Failed to track login attempt", zap.Error(err))
		}
		if locked {
			s.log.Warn("Account locked due to failed attempts", zap.String("email", req.Email), zap.Int("attempts", attempts))
			return nil, fmt.Errorf("account is locked due to too many failed attempts")
		}
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check 2FA
	if user.TwoFAEnabled {
		if req.TOTPCode == nil {
			// Generate temp token for 2FA flow
			tempToken, err := crypto.GenerateTempToken()
			if err != nil {
				return nil, fmt.Errorf("failed to generate temp token: %w", err)
			}

			s.repo.StoreTempToken(ctx, tempToken, user.ID, 5*time.Minute)

			return &domain.AuthResult{
				UserID:      user.ID,
				Requires2FA: true,
				TempToken:   tempToken,
			}, nil
		}

		// Verify TOTP code
		if user.TwoFASecret == nil {
			return nil, fmt.Errorf("2FA secret not found")
		}

		valid := crypto.ValidateTOTP(*user.TwoFASecret, *req.TOTPCode)
		if !valid {
			return nil, fmt.Errorf("invalid 2FA code")
		}
	}

	// Clear failed login attempts
	s.repo.ClearLoginAttempts(ctx, req.Email, req.IPAddress)

	// Update last login
	s.repo.UpdateLastLogin(ctx, user.ID)

	// Create session
	session, tokens, err := s.createSession(ctx, user, req.DeviceID, req.IPAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	s.log.Info("User logged in", zap.Int64("user_id", user.ID), zap.String("email", user.Email))

	return &domain.AuthResult{
		UserID:  user.ID,
		Tokens:  tokens,
		Session: session,
	}, nil
}

// LoginWith2FA completes login with 2FA verification
func (s *AuthService) LoginWith2FA(ctx context.Context, tempToken, totpCode string) (*domain.AuthResult, error) {
	// Get user ID from temp token
	userID, err := s.repo.GetTempToken(ctx, tempToken)
	if err != nil || userID == 0 {
		return nil, fmt.Errorf("invalid or expired temp token")
	}

	// Delete temp token (one-time use)
	s.repo.DeleteTempToken(ctx, tempToken)

	// Get user
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Verify TOTP code
	if user.TwoFASecret == nil {
		return nil, fmt.Errorf("2FA secret not found")
	}

	valid := crypto.ValidateTOTP(*user.TwoFASecret, totpCode)
	if !valid {
		return nil, fmt.Errorf("invalid 2FA code")
	}

	// Clear failed login attempts
	s.repo.ClearLoginAttempts(ctx, user.Email, "")

	// Update last login
	s.repo.UpdateLastLogin(ctx, user.ID)

	// Create session
	session, tokens, err := s.createSession(ctx, user, "", "")
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	s.log.Info("User logged in with 2FA", zap.Int64("user_id", user.ID))

	return &domain.AuthResult{
		UserID:  user.ID,
		Tokens:  tokens,
		Session: session,
	}, nil
}

// RefreshTokens refreshes access and refresh tokens
func (s *AuthService) RefreshTokens(ctx context.Context, refreshToken, deviceID string) (*domain.TokenPair, error) {
	// Validate refresh token
	claims, err := s.jwtConfig.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	// Get refresh token from Redis
	userID, sessionID, err := s.repo.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, fmt.Errorf("refresh token not found or expired")
	}

	// Delete old refresh token (rotation)
	s.repo.DeleteRefreshToken(ctx, refreshToken)

	// Get session
	session, err := s.repo.GetSession(ctx, sessionID)
	if err != nil || session == nil {
		return nil, fmt.Errorf("session not found or expired")
	}

	// Verify user ID matches
	if session.UserID != claims.UserID {
		return nil, fmt.Errorf("token user mismatch")
	}

	// Generate new tokens
	accessToken, err := s.jwtConfig.GenerateAccessToken(userID, sessionID, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	newRefreshToken, err := s.jwtConfig.GenerateRefreshToken(userID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Store new refresh token
	err = s.repo.StoreRefreshToken(ctx, newRefreshToken, userID, sessionID, 7*24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	// Update session activity
	session.LastActivity = time.Now()
	s.repo.CreateSession(ctx, session)

	return &domain.TokenPair{
		AccessToken:      accessToken,
		RefreshToken:     newRefreshToken,
		ExpiresIn:        900,   // 15 minutes
		RefreshExpiresIn: 604800, // 7 days
		TokenType:        "Bearer",
	}, nil
}

// Logout invalidates a user session
func (s *AuthService) Logout(ctx context.Context, userID int64, sessionID string) error {
	// Get session to verify ownership
	session, err := s.repo.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	if session != nil && session.UserID != userID {
		return fmt.Errorf("session does not belong to user")
	}

	// Delete session
	s.repo.DeleteSession(ctx, sessionID, userID)

	s.log.Info("User logged out", zap.Int64("user_id", userID), zap.String("session_id", sessionID))

	return nil
}

// ValidateToken validates an access token
func (s *AuthService) ValidateToken(ctx context.Context, accessToken string) (*crypto.JWTClaims, error) {
	claims, err := s.jwtConfig.ValidateAccessToken(accessToken)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	// Verify session still exists
	session, err := s.repo.GetSession(ctx, claims.SessionID)
	if err != nil || session == nil {
		return nil, fmt.Errorf("session expired or not found")
	}

	return claims, nil
}

// Enable2FA enables two-factor authentication for a user
func (s *AuthService) Enable2FA(ctx context.Context, userID int64) (secret, qrURI, backupCodes string, err error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		return "", "", "", fmt.Errorf("user not found")
	}

	if user.TwoFAEnabled {
		return "", "", "", fmt.Errorf("2FA is already enabled")
	}

	// Generate TOTP secret
	config := crypto.DefaultTOTPConfig(user.Email)
	secret, qrURI, err = config.GenerateSecret()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate TOTP secret: %w", err)
	}

	// Generate backup codes
	codes, err := crypto.GenerateBackupCodes(10)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate backup codes: %w", err)
	}

	backupCodesBytes, _ := json.Marshal(codes)
	backupCodes = string(backupCodesBytes)

	// Store secret temporarily (not enabled until verified)
	user.TwoFASecret = &secret
	s.repo.UpdateUser(ctx, user)

	return secret, qrURI, backupCodes, nil
}

// Verify2FA verifies a TOTP code and completes 2FA setup
func (s *AuthService) Verify2FA(ctx context.Context, userID int64, totpCode string) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		return fmt.Errorf("user not found")
	}

	if user.TwoFAEnabled {
		return fmt.Errorf("2FA is already enabled")
	}

	if user.TwoFASecret == nil {
		return fmt.Errorf("2FA not initiated, call Enable2FA first")
	}

	// Verify TOTP code
	valid := crypto.ValidateTOTP(*user.TwoFASecret, totpCode)
	if !valid {
		return fmt.Errorf("invalid 2FA code")
	}

	// Enable 2FA
	user.TwoFAEnabled = true
	s.repo.UpdateUser(ctx, user)

	s.log.Info("2FA enabled", zap.Int64("user_id", userID))

	return nil
}

// Disable2FA disables two-factor authentication
func (s *AuthService) Disable2FA(ctx context.Context, userID int64, totpCode string) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		return fmt.Errorf("user not found")
	}

	if !user.TwoFAEnabled {
		return fmt.Errorf("2FA is not enabled")
	}

	// Verify TOTP code
	if user.TwoFASecret == nil {
		return fmt.Errorf("2FA secret not found")
	}

	valid := crypto.ValidateTOTP(*user.TwoFASecret, totpCode)
	if !valid {
		return fmt.Errorf("invalid 2FA code")
	}

	// Disable 2FA
	user.TwoFAEnabled = false
	user.TwoFASecret = nil
	s.repo.UpdateUser(ctx, user)

	s.log.Info("2FA disabled", zap.Int64("user_id", userID))

	return nil
}

// ChangePassword changes user password
func (s *AuthService) ChangePassword(ctx context.Context, userID int64, currentPassword, newPassword string) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		return fmt.Errorf("user not found")
	}

	// Verify current password
	valid, err := crypto.VerifyPassword(currentPassword, user.PasswordHash)
	if err != nil {
		return fmt.Errorf("failed to verify password: %w", err)
	}
	if !valid {
		return fmt.Errorf("current password is incorrect")
	}

	// Hash new password
	newHash, err := crypto.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	// Update password
	s.repo.UpdatePassword(ctx, userID, newHash)

	// Revoke all sessions (force re-login)
	s.repo.DeleteAllUserSessions(ctx, userID)

	s.log.Info("Password changed", zap.Int64("user_id", userID))

	return nil
}

// ResetPasswordRequest initiates password reset flow
func (s *AuthService) ResetPasswordRequest(ctx context.Context, email, ip string) error {
	// Always return success to prevent email enumeration
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil || user == nil {
		return nil
	}

	// In production: send email with reset token
	// For now: just log
	s.log.Info("Password reset requested", zap.String("email", email), zap.String("ip", ip))

	return nil
}

// ResetPassword completes password reset with token
func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	// In production: validate reset token from Redis/DB
	// For now: placeholder
	return fmt.Errorf("password reset not implemented")
}

// createSession creates a session and tokens for a user
func (s *AuthService) createSession(ctx context.Context, user *domain.User, deviceID, ipAddress string) (*domain.Session, *domain.TokenPair, error) {
	sessionID := uuid.New().String()

	session := &domain.Session{
		ID:           sessionID,
		UserID:       user.ID,
		DeviceID:     deviceID,
		IPAddress:    ipAddress,
		Country:      user.CountryCode,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(30 * 24 * time.Hour),
		LastActivity: time.Now(),
		IsActive:     true,
	}

	// Store session
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, nil, fmt.Errorf("failed to store session: %w", err)
	}

	// Generate tokens
	accessToken, err := s.jwtConfig.GenerateAccessToken(user.ID, sessionID, deviceID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.jwtConfig.GenerateRefreshToken(user.ID, sessionID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Store refresh token
	err = s.repo.StoreRefreshToken(ctx, refreshToken, user.ID, sessionID, 7*24*time.Hour)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	tokens := &domain.TokenPair{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		ExpiresIn:        900,   // 15 minutes
		RefreshExpiresIn: 604800, // 7 days
		TokenType:        "Bearer",
	}

	return session, tokens, nil
}
