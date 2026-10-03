package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"math"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters (OWASP recommended)
const (
	argonMemory      = 64 * 1024 // 64MB
	argonIterations  = 3
	argonParallelism = 4
	argonSaltLength  = 16
	argonKeyLength   = 32
)

// HashPassword hashes a password using Argon2id
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		argonIterations,
		argonMemory,
		argonParallelism,
		argonKeyLength,
	)

	// Encode as base64
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	// Format: $argon2id$v=19$m=65536,t=3,p=4$salt$hash
	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemory,
		argonIterations,
		argonParallelism,
		b64Salt,
		b64Hash,
	)

	return encoded, nil
}

// checkedUint32 converts a non-negative int to uint32, rejecting values that
// do not fit rather than wrapping them.
func checkedUint32(v int) (uint32, error) {
	if v < 0 || uint64(v) > math.MaxUint32 {
		return 0, fmt.Errorf("value %d does not fit in uint32", v)
	}
	return uint32(v), nil // #nosec G115 -- range checked immediately above
}

// VerifyPassword verifies a password against its Argon2id hash
func VerifyPassword(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false, fmt.Errorf("invalid hash format")
	}

	// Parse parameters
	var memory, iterations, parallelism uint32
	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism)
	if err != nil {
		return false, fmt.Errorf("failed to parse hash parameters: %w", err)
	}

	// Decode salt
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("failed to decode salt: %w", err)
	}

	// Decode hash
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("failed to decode hash: %w", err)
	}

	// argon2 takes parallelism as a uint8 and the output length as a uint32,
	// but both values here are parsed out of the stored hash string, where
	// nothing bounds them. A wrapped value would silently verify against
	// different parameters than the hash was created with, so each is range
	// checked and a malformed hash is rejected instead.
	if parallelism == 0 || parallelism > math.MaxUint8 {
		return false, fmt.Errorf("invalid argon2 parallelism: %d", parallelism)
	}
	hashLen, err := checkedUint32(len(hash))
	if err != nil || hashLen == 0 {
		return false, fmt.Errorf("invalid argon2 hash length: %d", len(hash))
	}

	// Compute hash with same parameters
	otherHash := argon2.IDKey(
		[]byte(password),
		salt,
		iterations,
		memory,
		uint8(parallelism), // #nosec G115 -- bounded to 1..=MaxUint8 immediately above
		hashLen,
	)

	// Constant-time comparison
	if subtle.ConstantTimeCompare(hash, otherHash) == 1 {
		return true, nil
	}
	return false, nil
}

// GenerateRandomBytes generates cryptographically secure random bytes
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// GenerateRandomString generates a cryptographically secure random string
func GenerateRandomString(n int) (string, error) {
	b, err := GenerateRandomBytes(n)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
