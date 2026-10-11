package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// Authentication contract for the User Service.
//
// Why this file exists: every other Go service in the platform
// (auth, bonus, kyc, affiliate, admin-bff) authenticates its HTTP surface,
// while User Service accepted `user_id` straight from the URL path with no
// token at all. Any anonymous caller could read or rewrite another player's
// profile, preferences and gambling limits by incrementing an integer — a
// textbook IDOR on the most personal endpoints in the system, and a direct
// violation of CONVENTIONS.md (NEVER-7: user_id is never taken from the URL,
// query string or body).
//
// Verification strategy mirrors the auth service, which can sign with either
// HS256 (shared secret) or Ed25519. Both are supported here; the algorithm is
// pinned from the token header so an attacker cannot downgrade to `none` or
// substitute an RSA public key as the HMAC secret.
const (
	// authTokenIssuer mirrors tokenIssuer in services/go/auth/internal/crypto.
	// A correctly signed token from any other issuer is still rejected.
	// #nosec G101 -- this is a token issuer identifier, not a credential.
	authTokenIssuer = "opus-casino-auth"

	// tokenTypeAccess selects access tokens. The auth service signs access and
	// refresh tokens with the SAME secret and the same claim set, differing
	// only in `token_type` and TTL. Without this check a refresh token — valid
	// for 30 days and normally kept in a less protected store than an access
	// token — would be accepted as an API credential.
	tokenTypeAccess = "access"

	// minJWTSecretLength mirrors auth.MinHS256SecretLength. 32 bytes is the
	// practical floor for an HMAC-SHA256 key.
	minJWTSecretLength = 32

	// defaultJWTSecret is the placeholder shipped in both the auth and user
	// service configs. Verifying against it would let anyone who has read the
	// repository mint valid tokens for arbitrary users.
	defaultJWTSecret = "change-me-in-production"
)

// Context keys for values proven by the verified token.
const (
	userIDLocalKey    = "auth_user_id"
	sessionIDLocalKey = "auth_session_id"
)

// userClaims mirrors the auth service claim set. jwt.RegisteredClaims brings
// exp/nbf/iss/sub and gives typed, validated parsing instead of the untyped
// map that makes claim checks easy to get wrong.
type userClaims struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	DeviceID  string `json:"device_id,omitempty"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

var (
	errMissingAuthHeader = errors.New("missing Authorization header")
	errBadAuthHeader     = errors.New("invalid Authorization header, expected: Bearer <token>")
	errTokenInvalid      = errors.New("invalid or expired token")
	errTokenNotAccess    = errors.New("refresh tokens cannot be used as API credentials")
	errTokenIdentity     = errors.New("token does not carry a usable user identity")
	errNoUsableSecret    = errors.New("no usable JWT verification key configured")
)

// tokenVerifier holds the key material and fails closed: when no usable key is
// configured every request is rejected rather than verified against a weak or
// well-known one.
type tokenVerifier struct {
	secret     string
	ed25519Pub ed25519.PublicKey
}

// NewTokenVerifier builds a verifier from configuration. Unusable key
// material is dropped rather than fatal so the process can still start and
// serve /health, but every authenticated route then returns 401.
func NewTokenVerifier(secret, ed25519PublicKeyB64 string) *tokenVerifier {
	v := &tokenVerifier{}
	if len(secret) >= minJWTSecretLength && secret != defaultJWTSecret {
		v.secret = secret
	}
	v.ed25519Pub = decodeEd25519PublicKey(ed25519PublicKeyB64)
	return v
}

// decodeEd25519PublicKey accepts any standard base64 alphabet variant so an
// operator does not have to guess which one the deployment pipeline emitted.
func decodeEd25519PublicKey(s string) ed25519.PublicKey {
	if s == "" {
		return nil
	}
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding,
		base64.URLEncoding, base64.RawURLEncoding,
	} {
		raw, err := enc.DecodeString(s)
		if err == nil && len(raw) == ed25519.PublicKeySize {
			return ed25519.PublicKey(raw)
		}
	}
	return nil
}

// usable reports whether the verifier can validate at least one algorithm.
func (v *tokenVerifier) usable() bool {
	return v.secret != "" || v.ed25519Pub != nil
}

func (v *tokenVerifier) verify(tokenStr string) (*userClaims, error) {
	if !v.usable() {
		return nil, errNoUsableSecret
	}

	claims := &userClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		switch t.Method.(type) {
		case *jwt.SigningMethodEd25519:
			if v.ed25519Pub == nil {
				return nil, errors.New("ed25519 public key not configured")
			}
			return v.ed25519Pub, nil
		case *jwt.SigningMethodHMAC:
			if v.secret == "" {
				return nil, errors.New("hs256 secret not configured")
			}
			// The auth service only issues HS256. Accepting HS384/HS512 would
			// let an attacker who knows the secret pick a weaker variant, and
			// it would accept tokens no legitimate issuer ever produced.
			if t.Method.Alg() != "HS256" {
				return nil, fmt.Errorf("unexpected hmac algorithm %q", t.Method.Alg())
			}
			return []byte(v.secret), nil
		default:
			// Catches `none` and every asymmetric method; those are the
			// classic JWT confusion vectors.
			return nil, fmt.Errorf("unexpected signing method %q", t.Header["alg"])
		}
	},
		jwt.WithValidMethods([]string{"HS256", "EdDSA"}),
	)
	if err != nil {
		return nil, errTokenInvalid
	}

	if claims.Issuer != authTokenIssuer {
		return nil, errTokenInvalid
	}
	if claims.TokenType != tokenTypeAccess {
		return nil, errTokenNotAccess
	}
	// A token with no expiry would be a permanent credential.
	if claims.ExpiresAt == nil {
		return nil, errTokenInvalid
	}
	// The auth service guarantees sub mirrors user_id; a mismatch means the
	// claim set was tampered with or assembled by a different issuer.
	if claims.UserID == "" || claims.Subject != claims.UserID {
		return nil, errTokenIdentity
	}
	return claims, nil
}

// bearerToken extracts the credential from an Authorization header. The scheme
// match is case-insensitive per RFC 7235; anything with extra or missing parts
// is rejected rather than guessed at.
func bearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	return parts[1], true
}

// unauthorized renders a 401 without leaking which check failed beyond what the
// client needs to fix it.
func unauthorized(c *fiber.Ctx, err error) error {
	return c.Status(fiber.StatusUnauthorized).
		JSON(fiber.Map{"error": err.Error()})
}

// AuthMiddleware rejects any request without a valid access token and stores
// the identity proven by that token. The stored user id is the ONLY accepted
// source of identity for downstream handlers.
func AuthMiddleware(v *tokenVerifier) fiber.Handler {
	return func(c *fiber.Ctx) error {
		raw := c.Get(fiber.HeaderAuthorization)
		if raw == "" {
			return unauthorized(c, errMissingAuthHeader)
		}
		tokenStr, ok := bearerToken(raw)
		if !ok {
			return unauthorized(c, errBadAuthHeader)
		}

		claims, err := v.verify(tokenStr)
		if err != nil {
			return unauthorized(c, err)
		}

		// users.id is a BIGSERIAL, which the auth service carries in the token
		// as its decimal string. A non-numeric subject means the token was
		// minted for a different subject type and must not be coerced to 0.
		id, err := strconv.ParseInt(claims.UserID, 10, 64)
		if err != nil || id <= 0 {
			return unauthorized(c, errTokenIdentity)
		}

		c.Locals(userIDLocalKey, id)
		c.Locals(sessionIDLocalKey, claims.SessionID)
		return c.Next()
	}
}

// authenticatedUserID returns the identity proven by the Bearer token.
// Handlers must not read a user id from the URL, query string or body; a value
// obtained that way is attacker-controlled.
func authenticatedUserID(c *fiber.Ctx) (int64, bool) {
	id, ok := c.Locals(userIDLocalKey).(int64)
	return id, ok && id > 0
}

// authenticatedSessionID returns the session id from the token, used to bind
// RG/session-limit decisions to the actual session rather than the player.
func authenticatedSessionID(c *fiber.Ctx) string {
	id, _ := c.Locals(sessionIDLocalKey).(string)
	return id
}

// SecurityHeaders applies the baseline hardening used by the other Go
// services. This API is JSON-only and never framed.
func SecurityHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		h := &c.Response().Header
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		h.Set("X-DNS-Prefetch-Control", "off")
		return c.Next()
	}
}
