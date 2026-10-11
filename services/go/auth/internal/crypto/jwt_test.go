package crypto

import (
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func testEd25519Config(t *testing.T) *JWTConfig {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}
	cfg, err := NewEd25519JWTConfigFromBase64(
		base64.StdEncoding.EncodeToString(priv),
		base64.StdEncoding.EncodeToString(pub),
	)
	if err != nil {
		t.Fatalf("NewEd25519JWTConfigFromBase64: %v", err)
	}
	return cfg
}

func testHS256Config(t *testing.T) *JWTConfig {
	t.Helper()
	cfg, err := DefaultJWTConfig(strings.Repeat("s", MinHS256SecretLength))
	if err != nil {
		t.Fatalf("DefaultJWTConfig: %v", err)
	}
	return cfg
}

func TestEd25519KeyPairValidation(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	// A public key that does not belong to the private key must be rejected
	// at configuration time, not silently at first verification.
	otherPub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	_, err = NewEd25519JWTConfigFromBase64(
		base64.StdEncoding.EncodeToString(priv),
		base64.StdEncoding.EncodeToString(otherPub),
	)
	if err == nil {
		t.Fatal("expected mismatch between private and public key to fail")
	}

	if _, err := NewEd25519JWTConfigFromBase64(
		base64.StdEncoding.EncodeToString(priv),
		base64.StdEncoding.EncodeToString(pub[:10]),
	); err == nil {
		t.Fatal("expected wrong public key size to fail")
	}
}

func TestNewJWTConfigFromEnvRequiresKeysOutsideDevelopment(t *testing.T) {
	if _, err := NewJWTConfigFromEnv("production", "secret", "", ""); err == nil {
		t.Fatal("expected production without ed25519 keys to fail")
	}
	if _, err := NewJWTConfigFromEnv("development", "short", "", ""); err == nil {
		t.Fatal("expected weak hs256 secret to fail in development")
	}
	if _, err := NewJWTConfigFromEnv("development", strings.Repeat("s", MinHS256SecretLength), "", ""); err != nil {
		t.Fatalf("expected development with long secret to succeed: %v", err)
	}

	// Half a key pair must never silently fall back to HS256.
	if _, err := NewJWTConfigFromEnv("development", strings.Repeat("s", MinHS256SecretLength), "privonly", ""); err == nil {
		t.Fatal("expected incomplete ed25519 key pair to fail")
	}
}

func TestAccessTokenRoundTrip(t *testing.T) {
	for name, cfg := range map[string]*JWTConfig{
		"ed25519": testEd25519Config(t),
		"hs256":   testHS256Config(t),
	} {
		t.Run(name, func(t *testing.T) {
			token, err := cfg.GenerateAccessToken("42", "sess-1", "device-1")
			if err != nil {
				t.Fatalf("GenerateAccessToken: %v", err)
			}

			claims, err := cfg.ValidateAccessToken(token)
			if err != nil {
				t.Fatalf("ValidateAccessToken: %v", err)
			}
			if claims.UserID != "42" || claims.SessionID != "sess-1" || claims.DeviceID != "device-1" {
				t.Fatalf("unexpected claims: %+v", claims)
			}
			if claims.TokenType != "access" {
				t.Fatalf("expected access token type, got %s", claims.TokenType)
			}
			if claims.ID == "" {
				t.Fatal("token must carry a jti")
			}
			if claims.Subject != "42" {
				t.Fatalf("subject must mirror user_id, got %s", claims.Subject)
			}
		})
	}
}

func TestTokensHaveUniqueJTI(t *testing.T) {
	cfg := testEd25519Config(t)
	seen := map[string]struct{}{}
	for i := 0; i < 50; i++ {
		token, err := cfg.GenerateAccessToken("42", "sess-1", "device-1")
		if err != nil {
			t.Fatalf("GenerateAccessToken: %v", err)
		}
		claims, err := cfg.ValidateAccessToken(token)
		if err != nil {
			t.Fatalf("ValidateAccessToken: %v", err)
		}
		if _, dup := seen[claims.ID]; dup {
			t.Fatal("jti must be unique per token")
		}
		seen[claims.ID] = struct{}{}
	}
}

func TestRefreshTokenCannotBeUsedAsAccessToken(t *testing.T) {
	cfg := testEd25519Config(t)

	refresh, err := cfg.GenerateRefreshToken("42", "sess-1")
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	if _, err := cfg.ValidateAccessToken(refresh); err == nil {
		t.Fatal("refresh token must not validate as an access token")
	}

	access, err := cfg.GenerateAccessToken("42", "sess-1", "device-1")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	if _, err := cfg.ValidateRefreshToken(access); err == nil {
		t.Fatal("access token must not validate as a refresh token")
	}

	if _, err := cfg.ValidateRefreshToken(refresh); err != nil {
		t.Fatalf("refresh token must validate as refresh token: %v", err)
	}
}

func TestValidateRejectsAlgorithmConfusion(t *testing.T) {
	edCfg := testEd25519Config(t)

	// An HS256 token must not be accepted by an Ed25519 verifier.
	hsCfg := testHS256Config(t)
	hsToken, err := hsCfg.GenerateAccessToken("42", "sess-1", "device-1")
	if err != nil {
		t.Fatalf("GenerateAccessToken (hs256): %v", err)
	}
	if _, err := edCfg.ValidateAccessToken(hsToken); err == nil {
		t.Fatal("ed25519 config must reject an hs256 token")
	}

	// alg=none must never be accepted.
	unsigned := jwt.NewWithClaims(jwt.SigningMethodNone, &JWTClaims{
		UserID:    "42",
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "forged",
			Issuer:    "opus-casino-auth",
			Subject:   "42",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	noneToken, err := unsigned.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("build alg=none token: %v", err)
	}
	for name, cfg := range map[string]*JWTConfig{"ed25519": edCfg, "hs256": hsCfg} {
		if _, err := cfg.ValidateAccessToken(noneToken); err == nil {
			t.Fatalf("%s config accepted an alg=none token", name)
		}
	}
}

func TestValidateRejectsForeignIssuerAndTamperedClaims(t *testing.T) {
	cfg := testEd25519Config(t)

	foreign, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, &JWTClaims{
		UserID: "42", TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ID: "x", Issuer: "evil-issuer", Subject: "42",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}).SignedString(cfg.Ed25519Private)
	if err != nil {
		t.Fatalf("sign foreign token: %v", err)
	}
	if _, err := cfg.ValidateAccessToken(foreign); err == nil {
		t.Fatal("expected foreign issuer to be rejected")
	}

	// subject/user_id mismatch must be rejected even with a valid signature.
	mismatch, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, &JWTClaims{
		UserID: "42", TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ID: "x", Issuer: "opus-casino-auth", Subject: "999",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}).SignedString(cfg.Ed25519Private)
	if err != nil {
		t.Fatalf("sign mismatch token: %v", err)
	}
	if _, err := cfg.ValidateAccessToken(mismatch); err == nil {
		t.Fatal("expected subject/user_id mismatch to be rejected")
	}

	// Missing jti must be rejected (audit + reuse detection rely on it).
	noJTI, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, &JWTClaims{
		UserID: "42", TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: "opus-casino-auth", Subject: "42",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}).SignedString(cfg.Ed25519Private)
	if err != nil {
		t.Fatalf("sign no-jti token: %v", err)
	}
	if _, err := cfg.ValidateAccessToken(noJTI); err == nil {
		t.Fatal("expected token without jti to be rejected")
	}
}

func TestValidateRejectsExpiredAndEmpty(t *testing.T) {
	cfg := testEd25519Config(t)
	if _, err := cfg.ValidateAccessToken(""); err == nil {
		t.Fatal("empty token must be rejected")
	}
	if _, err := cfg.ValidateAccessToken("   "); err == nil {
		t.Fatal("blank token must be rejected")
	}
	if _, err := cfg.ValidateAccessToken("not-a-jwt"); err == nil {
		t.Fatal("garbage token must be rejected")
	}

	expired, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, &JWTClaims{
		UserID: "42", TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ID: "x", Issuer: "opus-casino-auth", Subject: "42",
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}).SignedString(cfg.Ed25519Private)
	if err != nil {
		t.Fatalf("sign expired token: %v", err)
	}
	if _, err := cfg.ValidateAccessToken(expired); err == nil {
		t.Fatal("expired token must be rejected")
	}
}

func TestTokenSignedByAnotherKeyIsRejected(t *testing.T) {
	cfg := testEd25519Config(t)
	other := testEd25519Config(t)

	token, err := other.GenerateAccessToken("42", "sess-1", "device-1")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	if _, err := cfg.ValidateAccessToken(token); err == nil {
		t.Fatal("token signed by a foreign key must be rejected")
	}
}

func TestGenerateRequiresUserID(t *testing.T) {
	cfg := testEd25519Config(t)
	if _, err := cfg.GenerateAccessToken("", "sess-1", "device-1"); err == nil {
		t.Fatal("empty user_id must be rejected")
	}
}