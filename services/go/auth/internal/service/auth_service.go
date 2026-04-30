package service

import (
	"crypto/rand"
	"crypto/sha256"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/opus-casino/auth/internal/crypto"
	"github.com/opus-casino/auth/internal/domain"
)

// AuthService handles authentication business logic
type AuthService struct {
	repo         authRepository
	jwtConfig    *crypto.JWTConfig
	log          *zap.Logger
	googleConfig googleOAuthConfig
	httpClient   *http.Client
}

type authRepository interface {
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	GetUserByIdentifier(ctx context.Context, identifier string) (*domain.User, error)
	GetUserByID(ctx context.Context, id string) (*domain.User, error)
	GetUserByGoogleSub(ctx context.Context, googleSub string) (*domain.User, error)
	CreateUser(ctx context.Context, user *domain.User) error
	CreateUserFromGoogle(ctx context.Context, email, googleSub, username string) (*domain.User, error)
	LinkGoogleSub(ctx context.Context, userID, googleSub string, emailVerified bool) error
	UpdateUser(ctx context.Context, user *domain.User) error
	UpdateLastLogin(ctx context.Context, userID string) error
	UpdatePassword(ctx context.Context, userID string, passwordHash string) error
	CreateSession(ctx context.Context, session *domain.Session) error
	GetSession(ctx context.Context, sessionID string) (*domain.Session, error)
	DeleteSession(ctx context.Context, sessionID string, userID string) error
	GetUserSessions(ctx context.Context, userID string) ([]string, error)
	DeleteAllUserSessions(ctx context.Context, userID string) error
	StoreRefreshToken(ctx context.Context, token string, userID string, sessionID string, ttl time.Duration) error
	GetRefreshToken(ctx context.Context, token string) (userID string, sessionID string, err error)
	DeleteRefreshToken(ctx context.Context, token string) error
	TrackLoginAttempt(ctx context.Context, email, ip string) (attempts int, locked bool, err error)
	IsAccountLocked(ctx context.Context, email string) (bool, error)
	ClearLoginAttempts(ctx context.Context, email, ip string) error
	StoreTempToken(ctx context.Context, token string, userID string, ttl time.Duration) error
	GetTempToken(ctx context.Context, token string) (string, error)
	DeleteTempToken(ctx context.Context, token string) error
}

type googleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	WebRedirect  string
}

// NewAuthService creates a new auth service
func NewAuthService(repo authRepository, jwtConfig *crypto.JWTConfig, log *zap.Logger) *AuthService {
	return &AuthService{
		repo:       repo,
		jwtConfig:  jwtConfig,
		log:        log,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *AuthService) ConfigureGoogleOAuth(clientID, clientSecret, redirectURI, webRedirect string) {
	s.googleConfig = googleOAuthConfig{
		ClientID:     strings.TrimSpace(clientID),
		ClientSecret: strings.TrimSpace(clientSecret),
		RedirectURI:  strings.TrimSpace(redirectURI),
		WebRedirect:  strings.TrimSpace(webRedirect),
	}
}

func validateRegisterRequest(req *domain.RegisterRequest) error {
	if req == nil {
		return fmt.Errorf("%w: request is required", domain.ErrValidation)
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Username = strings.TrimSpace(req.Username)
	req.CountryCode = strings.ToUpper(strings.TrimSpace(req.CountryCode))
	req.CurrencyCode = strings.ToUpper(strings.TrimSpace(req.CurrencyCode))

	if _, err := mail.ParseAddress(req.Email); err != nil {
		return fmt.Errorf("%w: invalid email", domain.ErrValidation)
	}
	if len(req.Password) < 8 {
		return fmt.Errorf("%w: password must be at least 8 characters", domain.ErrValidation)
	}
	if len(req.Username) < 3 || len(req.Username) > 30 {
		return fmt.Errorf("%w: username must be 3-30 characters", domain.ErrValidation)
	}
	if len(req.CountryCode) != 2 {
		return fmt.Errorf("%w: country_code must be ISO2", domain.ErrValidation)
	}
	if len(req.CurrencyCode) != 3 {
		return fmt.Errorf("%w: currency_code must be ISO3", domain.ErrValidation)
	}

	return nil
}

func (s *AuthService) Log() *zap.Logger {
	return s.log
}

func (s *AuthService) WebRedirectURL() string {
	if strings.TrimSpace(s.googleConfig.WebRedirect) == "" {
		return "http://localhost:3000"
	}
	return strings.TrimRight(s.googleConfig.WebRedirect, "/")
}

type googleOAuthState struct {
	CodeVerifier string `json:"code_verifier"`
	Nonce        string `json:"nonce"`
	CreatedAt    int64  `json:"created_at"`
}

type googleTokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
}

type googleTokenInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Aud           string `json:"aud"`
	Nonce         string `json:"nonce"`
}

func randomURLSafe(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func codeChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func normalizeGoogleUsername(email string) string {
	local := strings.Split(strings.ToLower(strings.TrimSpace(email)), "@")[0]
	local = strings.TrimSpace(local)
	if local == "" {
		return fmt.Sprintf("google_%d", time.Now().Unix())
	}
	return local
}

// BuildGoogleAuthURL creates Google OAuth authorization URL and stores PKCE/nonce state in Redis.
func (s *AuthService) BuildGoogleAuthURL(ctx context.Context) (string, error) {
	if s.googleConfig.ClientID == "" || s.googleConfig.ClientSecret == "" || s.googleConfig.RedirectURI == "" {
		return "", fmt.Errorf("%w: google oauth config is not set", domain.ErrInternal)
	}

	state, err := randomURLSafe(24)
	if err != nil {
		return "", fmt.Errorf("%w: failed to generate state", domain.ErrInternal)
	}
	nonce, err := randomURLSafe(24)
	if err != nil {
		return "", fmt.Errorf("%w: failed to generate nonce", domain.ErrInternal)
	}
	codeVerifier, err := randomURLSafe(48)
	if err != nil {
		return "", fmt.Errorf("%w: failed to generate pkce verifier", domain.ErrInternal)
	}

	statePayload, err := json.Marshal(googleOAuthState{
		CodeVerifier: codeVerifier,
		Nonce:        nonce,
		CreatedAt:    time.Now().Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("%w: failed to serialize oauth state", domain.ErrInternal)
	}

	if err := s.repo.StoreTempToken(ctx, state, string(statePayload), 10*time.Minute); err != nil {
		return "", fmt.Errorf("%w: failed to store oauth state", domain.ErrInternal)
	}

	v := url.Values{}
	v.Set("client_id", s.googleConfig.ClientID)
	v.Set("redirect_uri", s.googleConfig.RedirectURI)
	v.Set("response_type", "code")
	v.Set("scope", "openid email profile")
	v.Set("state", state)
	v.Set("nonce", nonce)
	v.Set("code_challenge", codeChallengeS256(codeVerifier))
	v.Set("code_challenge_method", "S256")
	v.Set("prompt", "select_account")

	return "https://accounts.google.com/o/oauth2/v2/auth?" + v.Encode(), nil
}

func (s *AuthService) exchangeGoogleToken(ctx context.Context, code, codeVerifier string) (*googleTokenResponse, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", s.googleConfig.ClientID)
	form.Set("client_secret", s.googleConfig.ClientSecret)
	form.Set("redirect_uri", s.googleConfig.RedirectURI)
	form.Set("grant_type", "authorization_code")
	form.Set("code_verifier", codeVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("google token exchange failed: %s", string(body))
	}

	var tokenResp googleTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, err
	}
	if tokenResp.IDToken == "" || tokenResp.AccessToken == "" {
		return nil, errors.New("missing google token fields")
	}
	return &tokenResp, nil
}

func (s *AuthService) validateGoogleIDToken(ctx context.Context, idToken, expectedNonce string) (*googleTokenInfo, error) {
	u := "https://oauth2.googleapis.com/tokeninfo?id_token=" + url.QueryEscape(idToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("google tokeninfo failed: %s", string(body))
	}

	var info googleTokenInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	if info.Aud != s.googleConfig.ClientID {
		return nil, errors.New("google aud mismatch")
	}
	if expectedNonce != "" && info.Nonce != expectedNonce {
		return nil, errors.New("google nonce mismatch")
	}
	if info.Sub == "" || info.Email == "" {
		return nil, errors.New("google token missing sub/email")
	}
	return &info, nil
}

// LoginWithGoogleCallback validates Google callback and issues platform session/tokens.
func (s *AuthService) LoginWithGoogleCallback(ctx context.Context, code, state, ipAddress string) (*domain.AuthResult, error) {
	if code == "" || state == "" {
		return nil, fmt.Errorf("%w: code and state are required", domain.ErrValidation)
	}

	storedStateJSON, err := s.repo.GetTempToken(ctx, state)
	if err != nil || storedStateJSON == "" {
		return nil, domain.ErrInvalidToken
	}
	_ = s.repo.DeleteTempToken(ctx, state)

	var statePayload googleOAuthState
	if err := json.Unmarshal([]byte(storedStateJSON), &statePayload); err != nil {
		return nil, fmt.Errorf("%w: invalid oauth state", domain.ErrValidation)
	}

	tokenResp, err := s.exchangeGoogleToken(ctx, code, statePayload.CodeVerifier)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to exchange google code", domain.ErrInternal)
	}
	tokenInfo, err := s.validateGoogleIDToken(ctx, tokenResp.IDToken, statePayload.Nonce)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to validate google token", domain.ErrInvalidToken)
	}
	if strings.ToLower(tokenInfo.EmailVerified) != "true" {
		return nil, fmt.Errorf("%w: google email is not verified", domain.ErrValidation)
	}

	user, err := s.repo.GetUserByGoogleSub(ctx, tokenInfo.Sub)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to lookup google user", domain.ErrInternal)
	}
	if user == nil {
		existingByEmail, err := s.repo.GetUserByEmail(ctx, strings.ToLower(tokenInfo.Email))
		if err != nil {
			return nil, fmt.Errorf("%w: failed to lookup user by email", domain.ErrInternal)
		}
		if existingByEmail != nil {
			if err := s.repo.LinkGoogleSub(ctx, existingByEmail.ID, tokenInfo.Sub, true); err != nil {
				return nil, fmt.Errorf("%w: failed to link google account", domain.ErrInternal)
			}
			user = existingByEmail
		} else {
			user, err = s.repo.CreateUserFromGoogle(
				ctx,
				strings.ToLower(tokenInfo.Email),
				tokenInfo.Sub,
				normalizeGoogleUsername(tokenInfo.Email),
			)
			if err != nil {
				return nil, fmt.Errorf("%w: failed to create user from google", domain.ErrInternal)
			}
		}
	}

	session, tokens, err := s.createSession(ctx, user, "google-oauth", ipAddress)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create google session", domain.ErrInternal)
	}

	return &domain.AuthResult{
		UserID:  user.ID,
		Tokens:  tokens,
		Session: session,
	}, nil
}

// Register creates a new user account
func (s *AuthService) Register(ctx context.Context, req *domain.RegisterRequest) (*domain.AuthResult, error) {
	if err := validateRegisterRequest(req); err != nil {
		return nil, err
	}

	// Check if user already exists
	existing, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to check existing user", domain.ErrInternal)
	}
	if existing != nil {
		return nil, domain.ErrUserAlreadyExists
	}

	// Hash password
	passwordHash, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to hash password", domain.ErrInternal)
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
		if errors.Is(err, domain.ErrUserAlreadyExists) {
			return nil, domain.ErrUserAlreadyExists
		}
		return nil, fmt.Errorf("%w: failed to create user", domain.ErrInternal)
	}

	// Create session
	session, tokens, err := s.createSession(ctx, user, req.DeviceID, req.IPAddress)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create session", domain.ErrInternal)
	}

	s.log.Info("User registered", zap.String("user_id", user.ID), zap.String("email", user.Email))

	return &domain.AuthResult{
		UserID:  user.ID,
		Tokens:  tokens,
		Session: session,
	}, nil
}

// Login authenticates a user
func (s *AuthService) Login(ctx context.Context, req *domain.LoginRequest) (*domain.AuthResult, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: request is required", domain.ErrValidation)
	}

	identifier := strings.TrimSpace(req.Identifier)
	if identifier == "" {
		identifier = strings.TrimSpace(req.Email)
	}
	identifier = strings.ToLower(identifier)
	if identifier == "" || req.Password == "" {
		return nil, fmt.Errorf("%w: identifier and password are required", domain.ErrValidation)
	}

	// Check if account is locked
	locked, err := s.repo.IsAccountLocked(ctx, identifier)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to check account lock", domain.ErrInternal)
	}
	if locked {
		return nil, domain.ErrAccountLocked
	}

	// Get user
	user, err := s.repo.GetUserByIdentifier(ctx, identifier)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get user", domain.ErrInternal)
	}
	if user == nil {
		// Track failed attempt (but don't reveal user doesn't exist)
		s.repo.TrackLoginAttempt(ctx, identifier, req.IPAddress)
		return nil, domain.ErrInvalidCredentials
	}

	// Verify password
	valid, err := crypto.VerifyPassword(req.Password, user.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to verify password", domain.ErrInternal)
	}
	if !valid {
		attempts, locked, err := s.repo.TrackLoginAttempt(ctx, identifier, req.IPAddress)
		if err != nil {
			s.log.Error("Failed to track login attempt", zap.Error(err))
		}
		if locked {
			s.log.Warn("Account locked due to failed attempts", zap.String("identifier", identifier), zap.Int("attempts", attempts))
			return nil, domain.ErrAccountLocked
		}
		return nil, domain.ErrInvalidCredentials
	}

	// Check 2FA
	if user.TwoFAEnabled {
		if req.TOTPCode == nil {
			// Generate temp token for 2FA flow
			tempToken, err := crypto.GenerateTempToken()
			if err != nil {
				return nil, fmt.Errorf("%w: failed to generate temp token", domain.ErrInternal)
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
			return nil, fmt.Errorf("%w: 2FA secret not found", domain.ErrInternal)
		}

		valid := crypto.ValidateTOTP(*user.TwoFASecret, *req.TOTPCode)
		if !valid {
			return nil, domain.ErrInvalidCredentials
		}
	}

	// Clear failed login attempts
	s.repo.ClearLoginAttempts(ctx, identifier, req.IPAddress)

	// Update last login
	s.repo.UpdateLastLogin(ctx, user.ID)

	// Create session
	session, tokens, err := s.createSession(ctx, user, req.DeviceID, req.IPAddress)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create session", domain.ErrInternal)
	}

	s.log.Info("User logged in", zap.String("user_id", user.ID), zap.String("email", user.Email))

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
	if err != nil || userID == "" {
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

	s.log.Info("User logged in with 2FA", zap.String("user_id", user.ID))

	return &domain.AuthResult{
		UserID:  user.ID,
		Tokens:  tokens,
		Session: session,
	}, nil
}

// RefreshTokens refreshes access and refresh tokens
func (s *AuthService) RefreshTokens(ctx context.Context, refreshToken, deviceID string) (*domain.TokenPair, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, fmt.Errorf("%w: refresh token is required", domain.ErrValidation)
	}

	// Validate refresh token
	claims, err := s.jwtConfig.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, domain.ErrInvalidRefreshToken
	}

	// Get refresh token from Redis
	userID, sessionID, err := s.repo.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, domain.ErrInvalidRefreshToken
	}
	if userID == "" || sessionID == "" {
		return nil, domain.ErrInvalidRefreshToken
	}

	// Delete old refresh token (rotation)
	s.repo.DeleteRefreshToken(ctx, refreshToken)

	// Get session
	session, err := s.repo.GetSession(ctx, sessionID)
	if err != nil || session == nil {
		return nil, domain.ErrInvalidRefreshToken
	}

	// Verify user ID matches
	if session.UserID != claims.UserID {
		return nil, domain.ErrInvalidRefreshToken
	}

	// Generate new tokens
	accessToken, err := s.jwtConfig.GenerateAccessToken(userID, sessionID, deviceID)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to generate access token", domain.ErrInternal)
	}

	newRefreshToken, err := s.jwtConfig.GenerateRefreshToken(userID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to generate refresh token", domain.ErrInternal)
	}

	// Store new refresh token
	err = s.repo.StoreRefreshToken(ctx, newRefreshToken, userID, sessionID, 7*24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to store refresh token", domain.ErrInternal)
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
func (s *AuthService) Logout(ctx context.Context, userID string, sessionID string) error {
	// Get session to verify ownership
	session, err := s.repo.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("%w: failed to get session", domain.ErrInternal)
	}

	if session != nil && session.UserID != userID {
		return domain.ErrForbidden
	}

	// Delete session
	s.repo.DeleteSession(ctx, sessionID, userID)

	s.log.Info("User logged out", zap.String("user_id", userID), zap.String("session_id", sessionID))

	return nil
}

// ValidateToken validates an access token
func (s *AuthService) ValidateToken(ctx context.Context, accessToken string) (*crypto.JWTClaims, error) {
	claims, err := s.jwtConfig.ValidateAccessToken(accessToken)
	if err != nil {
		return nil, domain.ErrInvalidToken
	}

	// Verify session still exists
	session, err := s.repo.GetSession(ctx, claims.SessionID)
	if err != nil || session == nil {
		return nil, domain.ErrInvalidToken
	}

	return claims, nil
}

// GetCurrentUser returns the authenticated user profile.
func (s *AuthService) GetCurrentUser(ctx context.Context, userID string) (*domain.User, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get user profile", domain.ErrInternal)
	}
	if user == nil {
		return nil, domain.ErrUserNotFound
	}

	return user, nil
}

// Enable2FA enables two-factor authentication for a user
func (s *AuthService) Enable2FA(ctx context.Context, userID string) (secret, qrURI, backupCodes string, err error) {
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
func (s *AuthService) Verify2FA(ctx context.Context, userID string, totpCode string) error {
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

	s.log.Info("2FA enabled", zap.String("user_id", userID))

	return nil
}

// Disable2FA disables two-factor authentication
func (s *AuthService) Disable2FA(ctx context.Context, userID string, totpCode string) error {
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

	s.log.Info("2FA disabled", zap.String("user_id", userID))

	return nil
}

// ChangePassword changes user password
func (s *AuthService) ChangePassword(ctx context.Context, userID string, currentPassword, newPassword string) error {
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

	s.log.Info("Password changed", zap.String("user_id", userID))

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
