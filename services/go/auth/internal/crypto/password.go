package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
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

// Argon2id parameter bounds enforced when verifying a stored hash.
// The parameters come from the stored hash, so they must be validated:
// unvalidated values let a malformed or tampered row request gigabytes of
// memory (OOM) or a zero-degree parallelism (panic inside argon2).
const (
	minArgonMemory     = 8 * 1024            // 8 MB
	maxArgonMemory     = 1024 * 1024         // 1 GB
	maxArgonIterations = 10
	maxArgonParallel   = 255
	// HashPassword always produces a 32 byte digest; a longer one is a
	// malformed or tampered row and must not reach argon2.IDKey, whose key
	// length parameter is a uint32 taken from the stored hash.
	maxArgonKeyLength = 64
)

// VerifyPassword verifies a password against its Argon2id hash
func VerifyPassword(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	// Expected layout: $argon2id$v=19$m=..,t=..,p=..$salt$hash
	if len(parts) != 6 || parts[0] != "" {
		return false, fmt.Errorf("invalid hash format")
	}
	if parts[1] != "argon2id" {
		return false, fmt.Errorf("unsupported hash algorithm: %s", parts[1])
	}
	if parts[2] != "v=19" {
		return false, fmt.Errorf("unsupported argon2 version: %s", parts[2])
	}

	// Parse and bound parameters before doing any work.
	var memory, iterations, parallelism uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, fmt.Errorf("failed to parse hash parameters: %w", err)
	}
	if memory < minArgonMemory || memory > maxArgonMemory {
		return false, fmt.Errorf("argon2 memory out of allowed range: %d", memory)
	}
	if iterations < 1 || iterations > maxArgonIterations {
		return false, fmt.Errorf("argon2 iterations out of allowed range: %d", iterations)
	}
	if parallelism < 1 || parallelism > maxArgonParallel {
		return false, fmt.Errorf("argon2 parallelism out of allowed range: %d", parallelism)
	}

	// Decode salt
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("failed to decode salt: %w", err)
	}
	if len(salt) < argonSaltLength {
		return false, fmt.Errorf("invalid salt length: %d", len(salt))
	}

	// Decode hash
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("failed to decode hash: %w", err)
	}
	if len(hash) < argonKeyLength {
		return false, fmt.Errorf("invalid hash length: %d", len(hash))
	}
	// Cap the key length as well: it feeds a uint32 argon2 parameter, and an
	// attacker-controlled row must not be able to request an arbitrary size.
	if len(hash) > maxArgonKeyLength {
		return false, fmt.Errorf("invalid hash length: %d", len(hash))
	}

	// Compute hash with same parameters (parallelism fits uint8 after the
	// range check above, key length after the bounds check above).
	otherHash := argon2.IDKey(
		[]byte(password),
		salt,
		iterations,
		memory,
		uint8(parallelism),
		uint32(len(hash)), // #nosec G115 -- bounded by argonKeyLength..maxArgonKeyLength above
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
