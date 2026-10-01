package config

import (
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg := Load()
	if cfg.GRPCPort != 9092 {
		t.Errorf("expected default grpc 9092, got %d", cfg.GRPCPort)
	}
	if cfg.HTTPPort != "8082" {
		t.Errorf("expected default http 8082, got %s", cfg.HTTPPort)
	}
	if cfg.RedisAddr != "localhost:6379" || cfg.RedisDB != 0 {
		t.Errorf("bad redis defaults: %+v", cfg)
	}
	if cfg.Env != "development" {
		t.Errorf("expected development default, got %s", cfg.Env)
	}
	if cfg.DatabaseURL == "" || cfg.JWTSecretKey == "" {
		t.Error("database url and jwt secret must have defaults")
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("GRPC_PORT", "50052")
	t.Setenv("PORT", "8085")
	t.Setenv("DATABASE_URL", "postgres://u:p@db:5432/x?sslmode=require")
	t.Setenv("REDIS_ADDR", "cache:6380")
	t.Setenv("REDIS_PASSWORD", "s3cret")
	t.Setenv("REDIS_DB", "2")
	t.Setenv("JWT_SECRET_KEY", "test-secret")
	t.Setenv("APP_ENV", "staging")

	cfg := Load()
	if cfg.GRPCPort != 50052 {
		t.Errorf("expected 50052, got %d", cfg.GRPCPort)
	}
	if cfg.HTTPPort != "8085" {
		t.Errorf("expected 8085, got %s", cfg.HTTPPort)
	}
	if cfg.DatabaseURL != "postgres://u:p@db:5432/x?sslmode=require" {
		t.Errorf("bad db url: %s", cfg.DatabaseURL)
	}
	if cfg.RedisAddr != "cache:6380" || cfg.RedisPassword != "s3cret" || cfg.RedisDB != 2 {
		t.Errorf("bad redis env: %+v", cfg)
	}
	if cfg.JWTSecretKey != "test-secret" || cfg.Env != "staging" {
		t.Errorf("bad misc env: %+v", cfg)
	}
}

func TestLoadBadIntFallsBack(t *testing.T) {
	t.Setenv("GRPC_PORT", "not-a-port")
	t.Setenv("REDIS_DB", "nan")
	cfg := Load()
	if cfg.GRPCPort != 0 {
		t.Errorf("expected 0 on bad grpc port (strconv.Atoi zero), got %d", cfg.GRPCPort)
	}
	if cfg.RedisDB != 0 {
		t.Errorf("expected 0 fallback, got %d", cfg.RedisDB)
	}
}

func TestLoadReadsEd25519PublicKey(t *testing.T) {
	t.Setenv("JWT_ED25519_PUBLIC_KEY", "base64-encoded-public-key")
	cfg := Load()
	if cfg.JWTEd25519PublicKey != "base64-encoded-public-key" {
		t.Fatalf("ed25519 public key not loaded: %+v", cfg)
	}
}

// TestValidateRejectsUnusableJWTConfig pins the fail-fast behaviour. Every
// player route is authenticated, so an unusable JWT key is not a degraded
// state: it means either all requests are rejected or tokens verify against a
// secret published in the repository.
func TestValidateRejectsUnusableJWTConfig(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want string
	}{
		{"production with placeholder secret", Config{Env: "production", JWTSecretKey: defaultJWTSecret}, "must be set"},
		{"production with empty secret", Config{Env: "production", JWTSecretKey: ""}, "must be set"},
		{"production with short secret", Config{Env: "production", JWTSecretKey: "twenty-bytes-of-junk!!"}, "too short"},
		{"staging with placeholder", Config{Env: "staging", JWTSecretKey: defaultJWTSecret}, "must be set"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if err == nil {
				t.Fatalf("expected rejection for %+v", tc.cfg)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q must mention %q", err, tc.want)
			}
		})
	}
}

func TestValidateAcceptsUsableJWTConfig(t *testing.T) {
	strong := strings.Repeat("k", MinJWTSecretLength)

	cases := []struct {
		name string
		cfg  Config
	}{
		{"development always passes", Config{Env: "development", JWTSecretKey: defaultJWTSecret}},
		{"production with strong hs256 secret", Config{Env: "production", JWTSecretKey: strong}},
		{"production with ed25519 key only", Config{Env: "production", JWTEd25519PublicKey: "key"}},
		{"production with both keys", Config{Env: "production", JWTSecretKey: strong, JWTEd25519PublicKey: "key"}},
		{"test env with strong secret", Config{Env: "test", JWTSecretKey: strong}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.cfg.Validate(); err != nil {
				t.Fatalf("expected acceptance, got %v", err)
			}
		})
	}
}

// TestValidateShortSecretReportedPrecisely covers the branch where a secret is
// present but too weak: the error must report the actual length rather than the
// generic "must be set" message, otherwise an operator cannot tell which of the
// two problems to fix.
func TestValidateShortSecretReportedPrecisely(t *testing.T) {
	cfg := Config{Env: "production", JWTSecretKey: "short"}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected rejection")
	}
	if !strings.Contains(err.Error(), "too short") {
		t.Fatalf("expected a length-specific error, got %v", err)
	}
}

func TestValidateNilConfig(t *testing.T) {
	var cfg *Config
	if err := cfg.Validate(); err == nil {
		t.Fatal("nil config must be rejected, not panic")
	}
}
