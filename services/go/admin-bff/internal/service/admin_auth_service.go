package service

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"image/png"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp"
	otptotp "github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/opus-casino/admin-bff/internal/client"
	"github.com/opus-casino/admin-bff/internal/models"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrSessionRevoked     = errors.New("session revoked")
	ErrTokenExpired       = errors.New("token expired")
	ErrTOTPRequired       = errors.New("totp_required")
	ErrInvalidTOTP        = errors.New("invalid or expired totp challenge")
)

// LoginResult is returned by Login.
type LoginResult struct {
	Admin          *models.AdminUser
	AccessToken    string
	RefreshToken   string
	TOTPRequired   bool   // true when TOTP is enabled; caller must present challenge_token + totp_code
	ChallengeToken string // short-lived opaque token for the second step
}

type AdminAuthService struct {
	db            *gorm.DB
	jwtSecret     string // legacy HMAC secret — used as fallback if ed25519 key not set
	ed25519Priv   ed25519.PrivateKey
	ed25519Pub    ed25519.PublicKey
	jwtExpiry     time.Duration
	refreshExpiry time.Duration
	totpVault     *client.VaultTransitClient
	totpVaultErr  error
}

func NewAdminAuthService(db *gorm.DB, jwtSecret string) *AdminAuthService {
	svc := &AdminAuthService{
		db:            db,
		jwtSecret:     jwtSecret,
		jwtExpiry:     15 * time.Minute,
		refreshExpiry: 7 * 24 * time.Hour,
	}
	if vaultClient, err := client.NewVaultTransitClient(); err == nil {
		svc.totpVault = vaultClient
	} else if !errors.Is(err, client.ErrNotConfigured) {
		svc.totpVaultErr = err
	}
	// Load Ed25519 keys from env-specified PEM files (JWT_ED25519_PRIVATE_KEY_FILE).
	// If not set, falls back to HMAC HS256 for backward compatibility.
	if privFile := os.Getenv("JWT_ED25519_PRIVATE_KEY_FILE"); privFile != "" {
		priv, pub, err := loadEd25519Keys(privFile)
		if err == nil {
			svc.ed25519Priv = priv
			svc.ed25519Pub = pub
		}
	}
	return svc
}

// GenerateEd25519KeyFiles generates a new Ed25519 key pair and writes PEM files.
// Call once during initial setup: GenerateEd25519KeyFiles("jwt.ed25519.priv", "jwt.ed25519.pub")
func GenerateEd25519KeyFiles(privPath, pubPath string) error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	privBlock := &pem.Block{Type: "PRIVATE KEY", Bytes: priv}
	if err := os.WriteFile(privPath, pem.EncodeToMemory(privBlock), 0600); err != nil {
		return err
	}
	pubBlock := &pem.Block{Type: "PUBLIC KEY", Bytes: pub}
	return os.WriteFile(pubPath, pem.EncodeToMemory(pubBlock), 0644)
}

func loadEd25519Keys(privPath string) (ed25519.PrivateKey, ed25519.PublicKey, error) {
	raw, err := os.ReadFile(privPath)
	if err != nil {
		return nil, nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, nil, errors.New("invalid PEM")
	}
	priv := ed25519.PrivateKey(block.Bytes)
	pub := priv.Public().(ed25519.PublicKey)
	return priv, pub, nil
}

func (s *AdminAuthService) encryptTOTPSecret(ctx context.Context, secret string) (string, error) {
	if secret == "" {
		return "", errors.New("totp secret required")
	}
	if s.totpVaultErr != nil {
		return "", fmt.Errorf("vault transit unavailable: %w", s.totpVaultErr)
	}
	if s.totpVault == nil {
		return secret, nil
	}
	return s.totpVault.Encrypt(ctx, secret)
}

func (s *AdminAuthService) decryptTOTPSecret(ctx context.Context, secret string) (string, error) {
	if secret == "" {
		return "", errors.New("totp secret missing")
	}
	if !strings.HasPrefix(secret, "vault:") {
		return secret, nil
	}
	if s.totpVaultErr != nil {
		return "", fmt.Errorf("vault transit unavailable: %w", s.totpVaultErr)
	}
	if s.totpVault == nil {
		return "", errors.New("vault transit not configured")
	}
	return s.totpVault.Decrypt(ctx, secret)
}

// Login verifies admin credentials.
// When TOTP is enabled it returns TOTPRequired=true and a short-lived ChallengeToken
// instead of the full token pair. The caller must then call VerifyTOTP.
func (s *AdminAuthService) Login(ctx context.Context, email, password, ip, ua string) (*LoginResult, error) {
	var admin models.AdminUser
	if err := s.db.WithContext(ctx).Where("email = ?", email).First(&admin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("db: %w", err)
	}
	if admin.Status != "active" {
		return nil, errors.New("account inactive or suspended")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// If TOTP is enabled, issue a challenge token — do NOT issue JWT yet.
	if admin.TOTPEnabled {
		challengeToken, err := s.generateChallengeToken(ctx, admin.ID)
		if err != nil {
			return nil, fmt.Errorf("generate challenge: %w", err)
		}
		return &LoginResult{
			Admin:          &admin,
			TOTPRequired:   true,
			ChallengeToken: challengeToken,
		}, nil
	}

	// No TOTP — issue tokens immediately.
	access, refresh, err := s.issueTokenPair(ctx, &admin, ip, ua)
	if err != nil {
		return nil, err
	}
	return &LoginResult{Admin: &admin, AccessToken: access, RefreshToken: refresh}, nil
}

// VerifyTOTP completes the second step of login: validates the challenge token
// and the TOTP code, then issues the full token pair.
//
// NOTE: Real TOTP code validation requires a TOTP library (e.g. github.com/pquerna/otp).
// The current implementation validates the challenge token and enforces the flow.
// Wire up actual TOTP code verification once the otp library is added to go.mod.
func (s *AdminAuthService) VerifyTOTP(ctx context.Context, challengeToken, totpCode, ip, ua string) (*LoginResult, error) {
	// Find and validate the challenge
	var challenges []models.TOTPChallenge
	if err := s.db.WithContext(ctx).
		Where("expires_at > ? AND used_at IS NULL", time.Now()).
		Order("created_at DESC").
		Find(&challenges).Error; err != nil {
		return nil, ErrInvalidTOTP
	}

	var matched *models.TOTPChallenge
	for i := range challenges {
		if err := bcrypt.CompareHashAndPassword([]byte(challenges[i].Token), []byte(challengeToken)); err == nil {
			matched = &challenges[i]
			break
		}
	}
	if matched == nil {
		return nil, ErrInvalidTOTP
	}

	// Mark challenge as used
	now := time.Now()
	s.db.WithContext(ctx).Model(matched).Update("used_at", now)

	var admin models.AdminUser
	if err := s.db.WithContext(ctx).First(&admin, matched.AdminID).Error; err != nil {
		return nil, fmt.Errorf("find admin: %w", err)
	}

	// Validate TOTP code against stored secret.
	if totpCode == "" {
		return nil, errors.New("totp_code required")
	}
	if admin.TOTPSecret == nil || !admin.TOTPEnabled {
		return nil, errors.New("totp not configured for this admin")
	}
	secret, err := s.decryptTOTPSecret(ctx, *admin.TOTPSecret)
	if err != nil {
		return nil, err
	}
	valid := otptotp.Validate(totpCode, secret)
	if !valid {
		return nil, errors.New("invalid totp code")
	}

	access, refresh, err := s.issueTokenPair(ctx, &admin, ip, ua)
	if err != nil {
		return nil, err
	}
	return &LoginResult{Admin: &admin, AccessToken: access, RefreshToken: refresh}, nil
}

// issueTokenPair generates access+refresh tokens, creates a session, and updates last_login.
func (s *AdminAuthService) issueTokenPair(ctx context.Context, admin *models.AdminUser, ip, ua string) (string, string, error) {
	now := time.Now()
	admin.LastLoginAt = &now
	admin.LastLoginIP = &ip
	s.db.WithContext(ctx).Model(admin).Updates(map[string]any{
		"last_login_at": now,
		"last_login_ip": ip,
	})

	accessToken, err := s.generateToken(admin.ID, admin.Role, admin.Permissions, s.jwtExpiry)
	if err != nil {
		return "", "", fmt.Errorf("generate access token: %w", err)
	}
	refreshToken, err := s.generateToken(admin.ID, admin.Role, admin.Permissions, s.refreshExpiry)
	if err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(refreshToken), bcrypt.DefaultCost)
	sess := models.AdminSession{
		AdminID:          admin.ID,
		RefreshTokenHash: string(hash),
		IPAddress:        &ip,
		UserAgent:        &ua,
		ExpiresAt:        now.Add(s.refreshExpiry),
	}
	if err := s.db.WithContext(ctx).Create(&sess).Error; err != nil {
		return "", "", fmt.Errorf("create session: %w", err)
	}
	return accessToken, refreshToken, nil
}

// generateChallengeToken stores a bcrypt-hashed challenge in totp_challenges and returns the raw token.
func (s *AdminAuthService) generateChallengeToken(ctx context.Context, adminID int64) (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.MinCost)
	if err != nil {
		return "", err
	}
	challenge := models.TOTPChallenge{
		AdminID:   adminID,
		Token:     string(hash),
		ExpiresAt: time.Now().Add(5 * time.Minute),
		CreatedAt: time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(&challenge).Error; err != nil {
		return "", fmt.Errorf("store challenge: %w", err)
	}
	return token, nil
}

// Refresh rotates the token pair given a valid refresh token.
func (s *AdminAuthService) Refresh(ctx context.Context, refreshToken string) (string, string, error) {
	claims, err := s.parseToken(refreshToken)
	if err != nil {
		return "", "", ErrTokenExpired
	}
	adminID := int64(claims["admin_id"].(float64))

	var sess models.AdminSession
	if err := s.db.WithContext(ctx).Where("admin_id = ? AND revoked_at IS NULL AND expires_at > ?", adminID, time.Now()).Order("created_at DESC").First(&sess).Error; err != nil {
		return "", "", ErrSessionRevoked
	}
	if err := bcrypt.CompareHashAndPassword([]byte(sess.RefreshTokenHash), []byte(refreshToken)); err != nil {
		return "", "", ErrSessionRevoked
	}

	var admin models.AdminUser
	if err := s.db.WithContext(ctx).First(&admin, adminID).Error; err != nil {
		return "", "", fmt.Errorf("find admin: %w", err)
	}

	now := time.Now()
	s.db.WithContext(ctx).Model(&sess).Update("revoked_at", now)

	access, _ := s.generateToken(admin.ID, admin.Role, admin.Permissions, s.jwtExpiry)
	refresh, _ := s.generateToken(admin.ID, admin.Role, admin.Permissions, s.refreshExpiry)
	hash, _ := bcrypt.GenerateFromPassword([]byte(refresh), bcrypt.DefaultCost)
	newSess := models.AdminSession{
		AdminID:          admin.ID,
		RefreshTokenHash: string(hash),
		IPAddress:        sess.IPAddress,
		UserAgent:        sess.UserAgent,
		ExpiresAt:        now.Add(s.refreshExpiry),
	}
	s.db.WithContext(ctx).Create(&newSess)
	return access, refresh, nil
}

// Logout revokes the session.
func (s *AdminAuthService) Logout(ctx context.Context, adminID int64) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&models.AdminSession{}).
		Where("admin_id = ? AND revoked_at IS NULL", adminID).
		Update("revoked_at", now).Error
}

// TOTPSetupResult contains the provisioning URI and QR code PNG (base64).
type TOTPSetupResult struct {
	Secret       string `json:"secret"`        // base32 secret — show once, then discard from response
	QRPNG        string `json:"qr_png_b64"`    // base64-encoded PNG for QR scanner
	ProvisionURI string `json:"provision_uri"` // otpauth:// URI
}

// GenerateTOTPSetup creates a new TOTP key for the given admin and returns the setup info.
// The secret is NOT stored yet; call EnableTOTP once the first code is verified.
func (s *AdminAuthService) GenerateTOTPSetup(ctx context.Context, adminID int64) (*TOTPSetupResult, error) {
	var admin models.AdminUser
	if err := s.db.WithContext(ctx).First(&admin, adminID).Error; err != nil {
		return nil, fmt.Errorf("find admin: %w", err)
	}

	key, err := otptotp.Generate(otptotp.GenerateOpts{
		Issuer:      "DOD Admin Panel",
		AccountName: admin.Email,
		Algorithm:   otp.AlgorithmSHA1,
		Digits:      otp.DigitsSix,
		Period:      30,
	})
	if err != nil {
		return nil, fmt.Errorf("generate otp key: %w", err)
	}

	// Render QR code as PNG, encode to base64
	img, err := key.Image(200, 200)
	if err != nil {
		return nil, fmt.Errorf("render qr: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode qr png: %w", err)
	}
	qrB64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	return &TOTPSetupResult{
		Secret:       key.Secret(),
		QRPNG:        qrB64,
		ProvisionURI: key.URL(),
	}, nil
}

// EnableTOTP stores the TOTP secret after the admin has verified the first code.
// Returns error if the provided code does not match the secret.
func (s *AdminAuthService) EnableTOTP(ctx context.Context, adminID int64, secret, code string) error {
	if !otptotp.Validate(code, secret) {
		return errors.New("invalid totp code — please try scanning again")
	}
	storedSecret, err := s.encryptTOTPSecret(ctx, secret)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Model(&models.AdminUser{}).Where("id = ?", adminID).Updates(map[string]any{
		"totp_secret":  storedSecret,
		"totp_enabled": true,
		"updated_at":   time.Now(),
	}).Error
}

// DisableTOTP wipes the TOTP secret (SUPER_ADMIN use only via reset endpoint).
func (s *AdminAuthService) DisableTOTP(ctx context.Context, adminID int64) error {
	return s.db.WithContext(ctx).Model(&models.AdminUser{}).Where("id = ?", adminID).Updates(map[string]any{
		"totp_secret":  nil,
		"totp_enabled": false,
		"updated_at":   time.Now(),
	}).Error
}

// Me returns the admin profile.
func (s *AdminAuthService) Me(ctx context.Context, adminID int64) (*models.AdminUser, error) {
	var admin models.AdminUser
	if err := s.db.WithContext(ctx).First(&admin, adminID).Error; err != nil {
		return nil, fmt.Errorf("find admin: %w", err)
	}
	return &admin, nil
}

func (s *AdminAuthService) generateToken(adminID int64, role string, perms []string, expiry time.Duration) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"admin_id":    adminID,
		"role":        role,
		"permissions": perms,
		"iat":         now.Unix(),
		"exp":         now.Add(expiry).Unix(),
	}
	// Use Ed25519 when key is loaded; fall back to HS256 for dev/migration period.
	if s.ed25519Priv != nil {
		token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
		return token.SignedString(s.ed25519Priv)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *AdminAuthService) parseToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		switch t.Method.(type) {
		case *jwt.SigningMethodEd25519:
			if s.ed25519Pub == nil {
				return nil, errors.New("ed25519 public key not configured")
			}
			return s.ed25519Pub, nil
		case *jwt.SigningMethodHMAC:
			return []byte(s.jwtSecret), nil
		default:
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}
