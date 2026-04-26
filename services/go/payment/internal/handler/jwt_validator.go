package handler

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// JWTClaims represents JWT claims
type JWTClaims struct {
	Sub   int64  `json:"sub"`   // User ID
	Email string `json:"email"` // User email
	Exp   int64  `json:"exp"`   // Expiration time
	Iat   int64  `json:"iat"`   // Issued at
}

// JWTValidator validates JWT tokens using Ed25519
type JWTValidator struct {
	publicKey ed25519.PublicKey
}

// NewJWTValidator creates a new JWT validator
func NewJWTValidator(publicKeyBase64 string) (*JWTValidator, error) {
	publicKeyBytes, err := base64.StdEncoding.DecodeString(publicKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("decode public key: %w", err)
	}

	if len(publicKeyBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size: expected %d, got %d", ed25519.PublicKeySize, len(publicKeyBytes))
	}

	return &JWTValidator{
		publicKey: ed25519.PublicKey(publicKeyBytes),
	}, nil
}

// ValidateToken validates a JWT token and returns claims
func (v *JWTValidator) ValidateToken(tokenString string) (*JWTClaims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format: expected 3 parts, got %d", len(parts))
	}

	// Decode header and payload
	header, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode header: %w", err)
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}

	// Decode signature
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("decode signature: %w", err)
	}

	// Verify header
	var headerMap map[string]interface{}
	if err := json.Unmarshal(header, &headerMap); err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}

	alg, ok := headerMap["alg"].(string)
	if !ok || alg != "EdDSA" {
		return nil, fmt.Errorf("invalid algorithm: expected EdDSA, got %v", alg)
	}

	// Verify signature
	message := parts[0] + "." + parts[1]
	if !ed25519.Verify(v.publicKey, []byte(message), signature) {
		return nil, fmt.Errorf("invalid signature")
	}

	// Parse claims
	var claims JWTClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}

	// Check expiration
	if claims.Exp > 0 && time.Now().Unix() > claims.Exp {
		return nil, fmt.Errorf("token expired")
	}

	return &claims, nil
}
