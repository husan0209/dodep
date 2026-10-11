package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// testSecret satisfies the 32-byte minimum, which the verifier enforces.
const testSecret = "unit-test-hs256-secret-32-bytes-minimum"

func signHS256(t *testing.T, secret string, mutate func(*userClaims)) string {
	t.Helper()
	now := time.Now()
	claims := &userClaims{
		UserID:    "42",
		SessionID: "sess-1",
		TokenType: tokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    authTokenIssuer,
			Subject:   "42",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	if mutate != nil {
		mutate(claims)
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func signEd25519(t *testing.T, priv ed25519.PrivateKey, mutate func(*userClaims)) string {
	t.Helper()
	now := time.Now()
	claims := &userClaims{
		UserID:    "42",
		SessionID: "sess-1",
		TokenType: tokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    authTokenIssuer,
			Subject:   "42",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	if mutate != nil {
		mutate(claims)
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).SignedString(priv)
	if err != nil {
		t.Fatalf("sign ed25519 token: %v", err)
	}
	return signed
}

// authedApp mounts a single endpoint behind the middleware and reports both the
// status and the identity the handler observed.
func authedApp(t *testing.T, verifier *tokenVerifier) *fiber.App {
	t.Helper()
	app := fiber.New()
	app.Get("/probe", AuthMiddleware(verifier), func(c *fiber.Ctx) error {
		id, ok := authenticatedUserID(c)
		if !ok {
			return c.Status(http.StatusInternalServerError).SendString("no identity")
		}
		return c.SendString(fmt.Sprintf("%d|%s", id, authenticatedSessionID(c)))
	})
	return app
}

func probe(t *testing.T, app *fiber.App, header string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	buf := make([]byte, 256)
	n, _ := resp.Body.Read(buf)
	return resp.StatusCode, string(buf[:n])
}

func TestAuthAcceptsValidAccessToken(t *testing.T) {
	app := authedApp(t, NewTokenVerifier(testSecret, ""))
	token := signHS256(t, testSecret, nil)

	code, body := probe(t, app, "Bearer "+token)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", code, body)
	}
	if body != "42|sess-1" {
		t.Fatalf("identity not propagated from claims: %q", body)
	}
}

func TestAuthHeaderCases(t *testing.T) {
	app := authedApp(t, NewTokenVerifier(testSecret, ""))
	token := signHS256(t, testSecret, nil)

	cases := []struct {
		name   string
		header string
		want   int
	}{
		{"missing header", "", http.StatusUnauthorized},
		{"scheme only", "Bearer", http.StatusUnauthorized},
		{"no scheme", token, http.StatusUnauthorized},
		{"wrong scheme", "Basic " + token, http.StatusUnauthorized},
		{"empty bearer", "Bearer ", http.StatusUnauthorized},
		{"too many parts", "Bearer a b", http.StatusUnauthorized},
		{"garbage token", "Bearer not-a-jwt", http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if code, body := probe(t, app, tc.header); code != tc.want {
				t.Fatalf("expected %d, got %d (%s)", tc.want, code, body)
			}
		})
	}
}

// TestAuthRejectsSignatureConfusion pins the algorithm-confusion defences.
func TestAuthRejectsSignatureConfusion(t *testing.T) {
	app := authedApp(t, NewTokenVerifier(testSecret, ""))

	// alg:none with an otherwise well-formed claim set.
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(
		`{"user_id":"42","sub":"42","iss":%q,"token_type":"access","exp":%d}`,
		authTokenIssuer, time.Now().Add(time.Hour).Unix())))
	noneToken := header + "." + payload + "."

	cases := []struct {
		name  string
		token string
	}{
		{"alg none", noneToken},
		{"signed with a different secret", signHS256(t, "another-secret-32-bytes-long-xxxxx", nil)},
		{"hs512 instead of hs256", mustSign(t, jwt.SigningMethodHS512, testSecret)},
		{"hs384 instead of hs256", mustSign(t, jwt.SigningMethodHS384, testSecret)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if code, _ := probe(t, app, "Bearer "+tc.token); code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", code)
			}
		})
	}
}

// mustSign issues an HS-family token with a caller-chosen algorithm, standing
// in for an attacker who knows the secret and picks a different variant.
func mustSign(t *testing.T, method jwt.SigningMethod, secret string) string {
	t.Helper()
	now := time.Now()
	claims := &userClaims{
		UserID:    "42",
		TokenType: tokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    authTokenIssuer,
			Subject:   "42",
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}
	signed, err := jwt.NewWithClaims(method, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return signed
}

// TestAuthRejectsRefreshTokens is the regression test for the finding that the
// auth service signs access and refresh tokens with the same secret, so a
// refresh token (30-day TTL) would otherwise pass as an API credential.
func TestAuthRejectsRefreshTokens(t *testing.T) {
	app := authedApp(t, NewTokenVerifier(testSecret, ""))
	refresh := signHS256(t, testSecret, func(c *userClaims) {
		c.TokenType = "refresh"
		c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour))
	})
	if code, body := probe(t, app, "Bearer "+refresh); code != http.StatusUnauthorized {
		t.Fatalf("refresh token must not authenticate: got %d (%s)", code, body)
	}
}

func TestAuthRejectsMalformedClaims(t *testing.T) {
	app := authedApp(t, NewTokenVerifier(testSecret, ""))

	cases := []struct {
		name   string
		mutate func(*userClaims)
	}{
		{"wrong issuer", func(c *userClaims) { c.Issuer = "some-other-service" }},
		{"empty issuer", func(c *userClaims) { c.Issuer = "" }},
		{"sub does not mirror user_id", func(c *userClaims) { c.Subject = "99" }},
		{"empty user_id", func(c *userClaims) { c.UserID = "" }},
		{"non numeric user_id", func(c *userClaims) { c.UserID = "550e8400-e29b-41d4-a716-446655440000" }},
		{"zero user_id", func(c *userClaims) { c.UserID = "0"; c.Subject = "0" }},
		{"negative user_id", func(c *userClaims) { c.UserID = "-5"; c.Subject = "-5" }},
		{"expired", func(c *userClaims) {
			c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))
		}},
		{"not yet valid", func(c *userClaims) {
			c.NotBefore = jwt.NewNumericDate(time.Now().Add(time.Hour))
		}},
		{"no expiry at all", func(c *userClaims) { c.ExpiresAt = nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token := signHS256(t, testSecret, tc.mutate)
			if code, body := probe(t, app, "Bearer "+token); code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d (%s)", code, body)
			}
		})
	}
}

func TestEd25519Verification(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pubB64 := base64.StdEncoding.EncodeToString(pub)

	verifier := NewTokenVerifier("", pubB64)
	if verifier.ed25519Pub == nil {
		t.Fatal("ed25519 public key was not loaded from base64")
	}
	app := authedApp(t, verifier)

	if code, body := probe(t, app, "Bearer "+signEd25519(t, priv, nil)); code != http.StatusOK {
		t.Fatalf("ed25519 token rejected: %d (%s)", code, body)
	}

	// An EdDSA token must not verify when only an HS256 secret is configured,
	// and vice versa: the verifier must not silently fall back.
	hsOnly := authedApp(t, NewTokenVerifier(testSecret, ""))
	if code, _ := probe(t, hsOnly, "Bearer "+signEd25519(t, priv, nil)); code != http.StatusUnauthorized {
		t.Fatalf("ed25519 token accepted without a public key: %d", code)
	}
	edOnly := authedApp(t, NewTokenVerifier("", pubB64))
	if code, _ := probe(t, edOnly, "Bearer "+signHS256(t, testSecret, nil)); code != http.StatusUnauthorized {
		t.Fatalf("hs256 token accepted without a secret: %d", code)
	}
}

func TestVerifierRejectsUnusableKeys(t *testing.T) {
	cases := []struct {
		name       string
		secret     string
		publicKeyB string
	}{
		{"default placeholder", defaultJWTSecret, ""},
		{"too short", "short-secret", ""},
		{"empty", "", ""},
		{"malformed ed25519", testSecret, "not-base64-key-material"},
		{"ed25519 wrong length", testSecret, base64.StdEncoding.EncodeToString(make([]byte, 16))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := NewTokenVerifier(tc.secret, tc.publicKeyB)
			// Key material that is not exactly ed25519.PublicKeySize bytes
			// must not register as a key. A correctly sized blob is accepted:
			// whether the bytes are a *real* key cannot be determined here,
			// and verification will simply fail later.
			if v.ed25519Pub != nil {
				t.Fatal("malformed ed25519 key must not be accepted")
			}
			if tc.secret != testSecret && v.usable() {
				t.Fatalf("verifier must fail closed, secret=%q usable=%v", tc.secret, v.usable())
			}
		})
	}
}

func TestVerifierFailsClosedWithNoKeys(t *testing.T) {
	app := authedApp(t, NewTokenVerifier(defaultJWTSecret, ""))
	token := signHS256(t, testSecret, nil)
	if code, _ := probe(t, app, "Bearer "+token); code != http.StatusUnauthorized {
		t.Fatalf("verifier with unusable keys must reject everything, got %d", code)
	}
}

func TestSecurityHeadersApplied(t *testing.T) {
	app := fiber.New()
	app.Use(SecurityHeaders())
	app.Get("/", func(c *fiber.Ctx) error { return c.SendString("ok") })

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil), -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	want := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
	}
	for k, v := range want {
		if got := resp.Header.Get(k); got != v {
			t.Errorf("header %s = %q, want %q", k, got, v)
		}
	}
}
