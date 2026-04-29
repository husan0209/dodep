package crypto

import (
	"crypto/ed25519"
	"fmt"
	"encoding/base64"
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

type jwtMode int

const (
	jwtModeHS256 jwtMode = iota
	jwtModeEdDSA
)

// JWTConfig holds JWT configuration
type JWTConfig struct {
	mode           jwtMode
	SecretKey       string // HS256 only (development fallback)
	Ed25519Private  ed25519.PrivateKey
	Ed25519Public   ed25519.PublicKey
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

// DefaultJWTConfig returns default JWT configuration
func DefaultJWTConfig(secretKey string) *JWTConfig {
	return &JWTConfig{
		mode:            jwtModeHS256,
		SecretKey:       secretKey,
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	}
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

	return &JWTConfig{
		mode:            jwtModeEdDSA,
		Ed25519Private:  ed25519.PrivateKey(privBytes),
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
	if env != "development" {
		return nil, fmt.Errorf("missing Ed25519 keys for JWT (required when APP_ENV != development)")
	}
	return DefaultJWTConfig(hs256Secret), nil
}

// GenerateAccessToken generates a JWT access token
func (c *JWTConfig) GenerateAccessToken(userID string, sessionID, deviceID string) (string, error) {
	now := time.Now()
	claims := &JWTClaims{
		UserID:    userID,
		SessionID: sessionID,
		DeviceID:  deviceID,
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(c.AccessTokenTTL)),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "opus-casino-auth",
			Subject:   userID,
		},
	}

	var token *jwt.Token
	switch c.mode {
	case jwtModeEdDSA:
		token = jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
		return token.SignedString(c.Ed25519Private)
	default:
		token = jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		return token.SignedString([]byte(c.SecretKey))
	}
}

// GenerateRefreshToken generates a JWT refresh token
func (c *JWTConfig) GenerateRefreshToken(userID string, sessionID string) (string, error) {
	now := time.Now()
	claims := &JWTClaims{
		UserID:    userID,
		SessionID: sessionID,
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(c.RefreshTokenTTL)),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "opus-casino-auth",
			Subject:   userID,
		},
	}

	var token *jwt.Token
	switch c.mode {
	case jwtModeEdDSA:
		token = jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
		return token.SignedString(c.Ed25519Private)
	default:
		token = jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		return token.SignedString([]byte(c.SecretKey))
	}
}

// ValidateAccessToken validates an access token and returns claims
func (c *JWTConfig) ValidateAccessToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		switch c.mode {
		case jwtModeEdDSA:
			if _, ok := token.Method.(*jwt.SigningMethodEd25519); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return c.Ed25519Public, nil
		default:
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(c.SecretKey), nil
		}
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	if claims.TokenType != "access" {
		return nil, fmt.Errorf("invalid token_type: expected access")
	}
	if claims.Issuer != "opus-casino-auth" {
		return nil, fmt.Errorf("invalid issuer")
	}

	return claims, nil
}

// ValidateRefreshToken validates a refresh token and returns claims
func (c *JWTConfig) ValidateRefreshToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		switch c.mode {
		case jwtModeEdDSA:
			if _, ok := token.Method.(*jwt.SigningMethodEd25519); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return c.Ed25519Public, nil
		default:
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(c.SecretKey), nil
		}
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	if claims.TokenType != "refresh" {
		return nil, fmt.Errorf("invalid token_type: expected refresh")
	}
	if claims.Issuer != "opus-casino-auth" {
		return nil, fmt.Errorf("invalid issuer")
	}

	return claims, nil
}
