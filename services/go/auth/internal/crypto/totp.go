package crypto

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"
	"time"

	"github.com/pquerna/otp"
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
		Digits:      otp.Digits(c.Digits),
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

// GenerateBackupCodes generates backup codes for 2FA recovery.
// Codes are guaranteed to be unique within the batch: a duplicate would
// silently reduce the effective number of usable recovery attempts.
func GenerateBackupCodes(count int) ([]string, error) {
	if count <= 0 {
		return nil, fmt.Errorf("backup code count must be positive")
	}

	codes := make([]string, 0, count)
	seen := make(map[string]struct{}, count)

	// Bounded retry loop: a collision needs a new random draw, but an
	// unbounded loop would hang if the generator ever misbehaved.
	const maxAttemptsPerCode = 16
	for len(codes) < count {
		for attempt := 0; attempt < maxAttemptsPerCode && len(codes) < count; attempt++ {
			b := make([]byte, 5)
			if _, err := rand.Read(b); err != nil {
				return nil, fmt.Errorf("failed to generate backup code: %w", err)
			}
			code := strings.TrimRight(base32.StdEncoding.EncodeToString(b), "=")
			if len(code) < 8 {
				continue
			}
			code = code[:8]
			if _, exists := seen[code]; exists {
				continue
			}
			seen[code] = struct{}{}
			codes = append(codes, code)
		}
		if len(codes) < count {
			// Every attempt collided — practically impossible, but fail loudly
			// instead of looping forever or returning duplicates.
			return nil, fmt.Errorf("failed to generate %d unique backup codes", count)
		}
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
