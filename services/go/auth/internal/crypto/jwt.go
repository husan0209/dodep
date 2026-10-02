package crypto

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims represents the JWT claims
type JWTClaims struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	DeviceID  string `json:"device_id,omitempty"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

// MinHS256SecretLength is the minimum accepted length of the shared HS256
// secret. 32 bytes of entropy is the practical floor for HMAC-SHA256 keys;
// anything shorter is rejected instead of silently signing with weak keys.
const MinHS256SecretLength = 32

// tokenIssuer is the `iss` claim every token of this service carries.
const tokenIssuer = "opus-casino-auth" // #nosec G101 -- issuer name, not a credential

type jwtMode int

const (
	jwtModeHS256 jwtMode = iota
	jwtModeEdDSA
)

// JWTConfig holds JWT configuration
type JWTConfig struct {
	mode            jwtMode
	SecretKey       string // HS256 only (development fallback)
	Ed25519Private  ed25519.PrivateKey
	Ed25519Public   ed25519.PublicKey
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

// DefaultJWTConfig returns default JWT configuration.
// The secret must be non-empty and at least MinHS256SecretLength bytes.
func DefaultJWTConfig(secretKey string) (*JWTConfig, error) {
	if len(secretKey) < MinHS256SecretLength {
		return nil, fmt.Errorf(
			"HS256 JWT secret must be at least %d bytes, got %d",
			MinHS256SecretLength, len(secretKey),
		)
	}
	return &JWTConfig{
		mode:            jwtModeHS256,
		SecretKey:       secretKey,
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	}, nil
}

// NewEd25519JWTConfigFromBase64 creates an EdDSA (Ed25519) JWT config from base64-encoded keys.
func NewEd25519JWTConfigFromBase64(privateKeyBase64, publicKeyBase64 string) (*JWTConfig, error) {
	privBytes, err := base64.StdEncoding.DecodeString(privateKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("decode ed25519 private key: %w", err)
	}
	pubBytes, err := base64.StdEncoding.DecodeString(publicKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("decode ed25519 public key: %w", err)
	}
	if len(privBytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid ed25519 private key size: expected %d, got %d", ed25519.PrivateKeySize, len(privBytes))
	}
	if len(pubBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid ed25519 public key size: expected %d, got %d", ed25519.PublicKeySize, len(pubBytes))
	}

	// The public key must belong to the private key, otherwise tokens cannot
	// be verified and the failure would only surface at runtime.
	private := ed25519.PrivateKey(privBytes)
	derivedPublic, ok := private.Public().(ed25519.PublicKey)
	if !ok || !derivedPublic.Equal(ed25519.PublicKey(pubBytes)) {
		return nil, fmt.Errorf("ed25519 public key does not match the private key")
	}

	return &JWTConfig{
		mode:            jwtModeEdDSA,
		Ed25519Private:  private,
		Ed25519Public:   ed25519.PublicKey(pubBytes),
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	}, nil
}

// NewJWTConfigFromEnv selects EdDSA (preferred) or HS256 (dev fallback).
// In non-development environments, EdDSA keys MUST be present.
func NewJWTConfigFromEnv(env, hs256Secret, ed25519PrivB64, ed25519PubB64 string) (*JWTConfig, error) {
	if ed25519PrivB64 != "" && ed25519PubB64 != "" {
		return NewEd25519JWTConfigFromBase64(ed25519PrivB64, ed25519PubB64)
	}
	if ed25519PrivB64 != "" || ed25519PubB64 != "" {
		return nil, fmt.Errorf(
			"incomplete ed25519 key pair: both private and public keys must be provided",
		)
	}
	if env != "development" {
		return nil, fmt.Errorf("missing Ed25519 keys for JWT (required when APP_ENV != development)")
	}
	return DefaultJWTConfig(hs256Secret)
}

// GenerateAccessToken generates a JWT access token
func (c *JWTConfig) GenerateAccessToken(userID string, sessionID, deviceID string) (string, error) {
	return c.sign(userID, sessionID, deviceID, "access", c.AccessTokenTTL)
}

// GenerateRefreshToken generates a JWT refresh token
func (c *JWTConfig) GenerateRefreshToken(userID string, sessionID string) (string, error) {
	return c.sign(userID, sessionID, "", "refresh", c.RefreshTokenTTL)
}

// newTokenID returns a unique token identifier (RFC 7519 `jti`).
// It is required for refresh-token reuse detection and audit correlation.
func newTokenID() (string, error) {
	b, err := GenerateRandomBytes(16)
	if err != nil {
		return "", fmt.Errorf("generate token id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (c *JWTConfig) sign(
	userID string,
	sessionID string,
	deviceID string,
	tokenType string,
	ttl time.Duration,
) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("user_id is required")
	}
	tokenID, err := newTokenID()
	if err != nil {
		return "", err
	}

	now := time.Now()
	claims := &JWTClaims{
		UserID:    userID,
		SessionID: sessionID,
		DeviceID:  deviceID,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    tokenIssuer,
			Subject:   userID,
		},
	}

	switch c.mode {
	case jwtModeEdDSA:
		return jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).SignedString(c.Ed25519Private)
	default:
		return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(c.SecretKey))
	}
}

// ValidateAccessToken validates an access token and returns claims
func (c *JWTConfig) ValidateAccessToken(tokenString string) (*JWTClaims, error) {
	return c.validate(tokenString, "access")
}

// ValidateRefreshToken validates a refresh token and returns claims
func (c *JWTConfig) ValidateRefreshToken(tokenString string) (*JWTClaims, error) {
	return c.validate(tokenString, "refresh")
}

// validate parses a token, pinning the signing algorithm to the configured
// mode, and enforces the claim invariants every consumer relies on.
func (c *JWTConfig) validate(tokenString string, expectedType string) (*JWTClaims, error) {
	if strings.TrimSpace(tokenString) == "" {
		return nil, fmt.Errorf("invalid token: empty")
	}

	claims := &JWTClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		switch c.mode {
		case jwtModeEdDSA:
			// Pin the exact algorithm: no "none", no HMAC, no RSA.
			if token.Method != jwt.SigningMethodEdDSA {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return c.Ed25519Public, nil
		default:
			if token.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			if len(c.SecretKey) < MinHS256SecretLength {
				return nil, fmt.Errorf("HS256 JWT secret is too short to verify tokens")
			}
			return []byte(c.SecretKey), nil
		}
	}, jwt.WithValidMethods([]string{
		jwt.SigningMethodEdDSA.Alg(),
		jwt.SigningMethodHS256.Alg(),
	}))
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid token: not valid")
	}

	if claims.TokenType != expectedType {
		return nil, fmt.Errorf("invalid token_type: expected %s", expectedType)
	}
	if claims.Issuer != tokenIssuer {
		return nil, fmt.Errorf("invalid issuer")
	}
	// Subject must mirror user_id; a mismatch means a tampered claim set.
	if claims.UserID == "" || claims.Subject != claims.UserID {
		return nil, fmt.Errorf("invalid subject: does not match user_id")
	}
	if claims.ID == "" {
		return nil, fmt.Errorf("invalid token: missing jti")
	}

	return claims, nil
}