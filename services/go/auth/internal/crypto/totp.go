package crypto

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"
	"time"

	"github.com/pquerna/otp/totp"
)

// TOTPConfig holds TOTP configuration
type TOTPConfig struct {
	Issuer      string
	AccountName string
	SecretSize  uint
	Digits      uint
	Period      uint
}

// DefaultTOTPConfig returns default TOTP configuration
func DefaultTOTPConfig(email string) *TOTPConfig {
	return &TOTPConfig{
		Issuer:      "Opus Casino",
		AccountName: email,
		SecretSize:  20,
		Digits:      6,
		Period:      30,
	}
}

// GenerateSecret generates a new TOTP secret
func (c *TOTPConfig) GenerateSecret() (secret string, qrURI string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      c.Issuer,
		AccountName: c.AccountName,
		SecretSize:  c.SecretSize,
		Digits:      totp.DigitsSix,
		Period:      c.Period,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to generate TOTP key: %w", err)
	}

	return key.Secret(), key.URL(), nil
}

// ValidateTOTP validates a TOTP code against a secret
func ValidateTOTP(secret, code string) bool {
	return totp.Validate(code, secret)
}

// GenerateBackupCodes generates backup codes for 2FA recovery
func GenerateBackupCodes(count int) ([]string, error) {
	codes := make([]string, count)
	for i := 0; i < count; i++ {
		b := make([]byte, 5)
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("failed to generate backup code: %w", err)
		}
		code := base32.StdEncoding.EncodeToString(b)
		code = strings.TrimRight(code, "=")
		codes[i] = code[:8]
	}
	return codes, nil
}

// GenerateTempToken generates a temporary token for 2FA flow
func GenerateTempToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate temp token: %w", err)
	}
	return base32.StdEncoding.EncodeToString(b), nil
}

// GetTOTPExpiry returns the expiry time for a TOTP code
func GetTOTPExpiry() time.Time {
	return time.Now().Add(5 * time.Minute)
}
