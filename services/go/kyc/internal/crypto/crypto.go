// Package crypto provides field-level encryption for KYC PII
// (document numbers, extracted OCR data) following the envelope pattern
// in simplified form: AES-256-GCM with a service key from Vault/env.
//
// Key contract (KYC_FIELD_KEY): 32 bytes encoded as base64, hex, or raw.
// The ciphertext payload is base64(nonce|ciphertext) with a "v1:" prefix.
// Search hashes (SHA-256 hex of normalised value) allow duplicate detection
// without decrypting every row.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// ErrNoKey is returned when no usable key material is configured.
var ErrNoKey = errors.New("crypto: no field encryption key configured")

// Service encrypts and decrypts single PII fields.
type Service struct {
	gcm cipher.AEAD
}

// New creates a Service from raw 32-byte key material.
func New(key []byte) (*Service, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("crypto: key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: new gcm: %w", err)
	}
	return &Service{gcm: gcm}, nil
}

// ParseKey decodes KYC_FIELD_KEY: base64 (standard or raw), hex, or raw 32 bytes.
func ParseKey(raw string) ([]byte, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, ErrNoKey
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil && len(b) == 32 {
		return b, nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil && len(b) == 32 {
		return b, nil
	}
	if b, err := hex.DecodeString(s); err == nil && len(b) == 32 {
		return b, nil
	}
	if len(s) == 32 {
		return []byte(s), nil
	}
	return nil, fmt.Errorf("crypto: unusable key encoding (need 32 bytes as base64/hex/raw)")
}

// Ephemeral creates a Service with a random key for development/tests.
// Ciphertexts do not survive restarts — production must use ParseKey.
func Ephemeral() (*Service, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("crypto: random key: %w", err)
	}
	return New(key)
}

// Encrypt seals plaintext with a fresh random nonce.
func (s *Service) Encrypt(plaintext string) (string, error) {
	if s == nil || s.gcm == nil {
		return "", ErrNoKey
	}
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("crypto: nonce: %w", err)
	}
	sealed := s.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return "v1:" + base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt opens a payload produced by Encrypt.
func (s *Service) Decrypt(payload string) (string, error) {
	if s == nil || s.gcm == nil {
		return "", ErrNoKey
	}
	raw, ok := strings.CutPrefix(payload, "v1:")
	if !ok {
		return "", errors.New("crypto: unknown payload version")
	}
	sealed, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", fmt.Errorf("crypto: decode: %w", err)
	}
	if len(sealed) < s.gcm.NonceSize() {
		return "", errors.New("crypto: payload too short")
	}
	nonce, ct := sealed[:s.gcm.NonceSize()], sealed[s.gcm.NonceSize():]
	open, err := s.gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", errors.New("crypto: decryption failed")
	}
	return string(open), nil
}

// HashForSearch returns the SHA-256 hex of the normalised value for
// duplicate detection (document numbers). Normalisation: trim + uppercase.
func HashForSearch(value string) string {
	norm := strings.ToUpper(strings.TrimSpace(value))
	sum := sha256.Sum256([]byte(norm))
	return hex.EncodeToString(sum[:])
}

// MaskNumber masks a document number for logs (never log full PII).
// "AB1234567" -> "AB***4567".
func MaskNumber(v string) string {
	if len(v) <= 4 {
		return "***"
	}
	head := v[:2]
	if len(v) < 2 {
		head = v[:1]
	}
	return head + "***" + v[len(v)-4:]
}
