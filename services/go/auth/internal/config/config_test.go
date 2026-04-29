package config

import "testing"

func TestLoad_UsesPortInGoogleRedirectURI(t *testing.T) {
	t.Setenv("PORT", "8083")
	t.Setenv("GOOGLE_REDIRECT_URI", "")

	cfg := Load()

	if cfg.GoogleRedirectURI != "http://localhost:8083/api/v1/auth/google/callback" {
		t.Fatalf("unexpected redirect uri: %s", cfg.GoogleRedirectURI)
	}
}

func TestLoad_PrefersExplicitGoogleRedirectURI(t *testing.T) {
	t.Setenv("PORT", "8083")
	t.Setenv("GOOGLE_REDIRECT_URI", "http://auth.local/api/v1/auth/google/callback")

	cfg := Load()

	if cfg.GoogleRedirectURI != "http://auth.local/api/v1/auth/google/callback" {
		t.Fatalf("unexpected redirect uri: %s", cfg.GoogleRedirectURI)
	}
}
