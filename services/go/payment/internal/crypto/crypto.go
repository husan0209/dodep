// Package crypto provides AES-256-GCM field-level encryption for stored
// payment-method details. The key comes from PAYMENT_METHOD_ENCRYPTION_KEY
// (base64, 32 bytes); without it the service fails closed.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// envelope is the stored format for one encrypted value.
type envelope struct {
	Version    int    `json:"v"`
	Algorithm  string `json:"alg"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ct"`
}

// FieldEncryption encrypts single PII values with a fresh random nonce.
type FieldEncryption struct {
	key []byte
}

// NewFieldEncryption builds the service. Empty key yields an unconfigured
// instance whose Encrypt/Decrypt fail closed.
func NewFieldEncryption(keyB64 string) (*FieldEncryption, error) {
	if strings.TrimSpace(keyB64) == "" {
		return &FieldEncryption{}, nil
	}
	raw, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return nil, fmt.Errorf("decode field encryption key: %w", err)
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("field encryption key must be 32 bytes, got %d", len(raw))
	}
	return &FieldEncryption{key: raw}, nil
}

// Ready reports whether encryption is configured.
func (f *FieldEncryption) Ready() bool { return len(f.key) == 32 }

// Encrypt encrypts plaintext and returns the envelope JSON.
func (f *FieldEncryption) Encrypt(plaintext string) (string, error) {
	if !f.Ready() {
		return "", fmt.Errorf("field encryption is not configured")
	}
	block, err := aes.NewCipher(f.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	out, err := json.Marshal(envelope{
		Version:    1,
		Algorithm:  "AES-256-GCM",
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ct),
	})
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// Decrypt reverses Encrypt.
func (f *FieldEncryption) Decrypt(raw string) (string, error) {
	if !f.Ready() {
		return "", fmt.Errorf("field encryption is not configured")
	}
	var payload envelope
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", fmt.Errorf("decode payload: %w", err)
	}
	if payload.Algorithm != "AES-256-GCM" {
		return "", fmt.Errorf("unsupported algorithm: %s", payload.Algorithm)
	}
	nonce, err := base64.StdEncoding.DecodeString(payload.Nonce)
	if err != nil {
		return "", fmt.Errorf("decode nonce: %w", err)
	}
	ct, err := base64.StdEncoding.DecodeString(payload.Ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	block, err := aes.NewCipher(f.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(pt), nil
}

// MaskDetails returns a safe display value for stored details
// (e.g. card "4111111111111111" -> "****1111", crypto address -> head...tail).
func MaskDetails(value string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return ""
	}
	if len(v) <= 4 {
		return "****"
	}
	if len(v) > 20 {
		return v[:8] + "..." + v[len(v)-4:]
	}
	return "****" + v[len(v)-4:]
}
