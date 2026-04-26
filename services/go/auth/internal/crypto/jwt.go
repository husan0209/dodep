package crypto

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims represents the JWT claims
type JWTClaims struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	DeviceID  string `json:"device_id,omitempty"`
	jwt.RegisteredClaims
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	SecretKey            string
	AccessTokenTTL       time.Duration
	RefreshTokenTTL      time.Duration
}

// DefaultJWTConfig returns default JWT configuration
func DefaultJWTConfig(secretKey string) *JWTConfig {
	return &JWTConfig{
		SecretKey:       secretKey,
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	}
}

// GenerateAccessToken generates a JWT access token
func (c *JWTConfig) GenerateAccessToken(userID string, sessionID, deviceID string) (string, error) {
	now := time.Now()
	claims := &JWTClaims{
		UserID:    userID,
		SessionID: sessionID,
		DeviceID:  deviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(c.AccessTokenTTL)),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "opus-casino-auth",
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(c.SecretKey))
}

// GenerateRefreshToken generates a JWT refresh token
func (c *JWTConfig) GenerateRefreshToken(userID string, sessionID string) (string, error) {
	now := time.Now()
	claims := &JWTClaims{
		UserID:    userID,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(c.RefreshTokenTTL)),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "opus-casino-auth",
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(c.SecretKey))
}

// ValidateAccessToken validates an access token and returns claims
func (c *JWTConfig) ValidateAccessToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(c.SecretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// ValidateRefreshToken validates a refresh token and returns claims
func (c *JWTConfig) ValidateRefreshToken(tokenString string) (*JWTClaims, error) {
	return c.ValidateAccessToken(tokenString) // Same validation logic
}
