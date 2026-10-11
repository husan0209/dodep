package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

// makeHS256Token builds a JWT signed with HS256 over the given claims.
func makeHS256Token(t *testing.T, secret string, claims map[string]interface{}) string {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		t.Fatalf("header marshal: %v", err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("payload marshal: %v", err)
	}
	signing := base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signing))
	return signing + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func TestExtractUserIDClaims(t *testing.T) {
	secret := "unit-test-secret"

	t.Run("uid claim wins", func(t *testing.T) {
		token := makeHS256Token(t, secret, map[string]interface{}{
			"uid": 42, "sub": "99", "exp": time.Now().Add(time.Hour).Unix(),
		})
		got, err := extractUserID(token, secret, "staging")
		if err != nil || got != 42 {
			t.Fatalf("expected uid 42, got %d %v", got, err)
		}
	})

	t.Run("sub as string", func(t *testing.T) {
		token := makeHS256Token(t, secret, map[string]interface{}{
			"sub": "7", "exp": time.Now().Add(time.Hour).Unix(),
		})
		got, err := extractUserID(token, secret, "staging")
		if err != nil || got != 7 {
			t.Fatalf("expected sub 7, got %d %v", got, err)
		}
	})

	t.Run("user_id string claim", func(t *testing.T) {
		token := makeHS256Token(t, secret, map[string]interface{}{
			"user_id": "1234", "exp": time.Now().Add(time.Hour).Unix(),
		})
		got, err := extractUserID(token, secret, "staging")
		if err != nil || got != 1234 {
			t.Fatalf("expected 1234, got %d %v", got, err)
		}
	})

	t.Run("expired token rejected", func(t *testing.T) {
		token := makeHS256Token(t, secret, map[string]interface{}{
			"uid": 5, "exp": time.Now().Add(-time.Minute).Unix(),
		})
		if _, err := extractUserID(token, secret, "staging"); err == nil {
			t.Fatal("expired token must be rejected")
		}
	})

	t.Run("tampered signature rejected", func(t *testing.T) {
		token := makeHS256Token(t, "secret-a", map[string]interface{}{"uid": 1})
		if _, err := extractUserID(token, secret, "staging"); err == nil {
			t.Fatal("token signed with another secret must be rejected")
		}
	})

	t.Run("malformed tokens rejected", func(t *testing.T) {
		for _, token := range []string{"", "abc", "a.b", "a.b.c.d", "..."} {
			if _, err := extractUserID(token, secret, "staging"); err == nil {
				t.Errorf("expected rejection for %q", token)
			}
		}
	})

	t.Run("non-numeric and zero identity rejected", func(t *testing.T) {
		token := makeHS256Token(t, secret, map[string]interface{}{"sub": "not-a-number"})
		if _, err := extractUserID(token, secret, "staging"); err == nil {
			t.Error("non-numeric sub must be rejected")
		}
		zero := makeHS256Token(t, secret, map[string]interface{}{"uid": 0, "sub": "0"})
		if _, err := extractUserID(zero, secret, "staging"); err == nil {
			t.Error("zero identity must be rejected")
		}
	})
}

func TestExtractUserIDFailsClosedOutsideDevelopment(t *testing.T) {
	// Default secret + non-development env: no trust, request rejected.
	token := makeHS256Token(t, "change-me-in-production", map[string]interface{}{"uid": 1})
	if _, err := extractUserID(token, "change-me-in-production", "production"); err == nil {
		t.Fatal("production must not accept unverifiable tokens")
	}
	// In development the same token parses (explicit local-dev fallback).
	if _, err := extractUserID(token, "change-me-in-production", "development"); err != nil {
		t.Fatalf("development fallback broken: %v", err)
	}
}

func TestAuthMiddleware(t *testing.T) {
	secret := "auth-secret"
	newApp := func() *fiber.App {
		app := fiber.New()
		app.Use(RequestIDMiddleware())
		app.Use(AuthMiddleware(secret, "staging"))
		app.Get("/probe", func(c *fiber.Ctx) error {
			return c.JSON(fiber.Map{"user_id": c.Locals("user_id")})
		})
		return app
	}

	call := func(app *fiber.App, header string) (int, string) {
		r := httptest.NewRequest(http.MethodGet, "/probe", nil)
		if header != "" {
			r.Header.Set("Authorization", header)
		}
		resp, err := app.Test(r, -1)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		buf := make([]byte, 256)
		n, _ := resp.Body.Read(buf)
		return resp.StatusCode, string(buf[:n])
	}

	token := makeHS256Token(t, secret, map[string]interface{}{"uid": 42})

	code, body := call(newApp(), "Bearer "+token)
	if code != 200 || !strings.Contains(body, `"user_id":42`) {
		t.Fatalf("valid token must pass identity: %d %s", code, body)
	}

	cases := []struct {
		name   string
		header string
		want   int
	}{
		{"missing header", "", http.StatusUnauthorized},
		{"wrong scheme", "Basic abc", http.StatusUnauthorized},
		{"no space", "Bearer", http.StatusUnauthorized},
		{"bad token", "Bearer garbage", http.StatusUnauthorized},
		{"wrong secret", "Bearer " + makeHS256Token(t, "other", map[string]interface{}{"uid": 1}), http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, body := call(newApp(), tc.header)
			if code != tc.want {
				t.Fatalf("expected %d, got %d: %s", tc.want, code, body)
			}
			if !strings.Contains(body, "RG_") {
				t.Fatalf("error must carry an RG_ code: %s", body)
			}
		})
	}
}

func TestAuthMiddlewareLowercaseBearer(t *testing.T) {
	secret := "auth-secret"
	token := makeHS256Token(t, secret, map[string]interface{}{"uid": 8})
	app := fiber.New()
	app.Use(AuthMiddleware(secret, "staging"))
	app.Get("/p", func(c *fiber.Ctx) error { return c.SendString("ok") })
	r := httptest.NewRequest(http.MethodGet, "/p", nil)
	r.Header.Set("Authorization", "bearer "+token)
	resp, err := app.Test(r, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("lowercase bearer must be accepted, got %d", resp.StatusCode)
	}
}

func TestAdminMiddlewareFailSecure(t *testing.T) {
	newApp := func(token string) *fiber.App {
		app := fiber.New()
		app.Use(AdminMiddleware(token))
		app.Get("/a", func(c *fiber.Ctx) error {
			return c.JSON(fiber.Map{"admin": c.Locals("admin_id")})
		})
		return app
	}
	call := func(app *fiber.App, header, adminID string) (int, string) {
		r := httptest.NewRequest(http.MethodGet, "/a", nil)
		if header != "" {
			r.Header.Set("Authorization", header)
		}
		if adminID != "" {
			r.Header.Set("X-Admin-ID", adminID)
		}
		resp, err := app.Test(r, -1)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		buf := make([]byte, 256)
		n, _ := resp.Body.Read(buf)
		return resp.StatusCode, string(buf[:n])
	}

	// Unset token → everything forbidden (fail-secure).
	if code, body := call(newApp(""), "Bearer whatever", ""); code != http.StatusForbidden ||
		!strings.Contains(body, "RG_ADMIN_DISABLED") {
		t.Fatalf("unset admin token must forbid: %d %s", code, body)
	}

	// Wrong token → unauthorized.
	if code, _ := call(newApp("real-token"), "Bearer wrong", ""); code != http.StatusUnauthorized {
		t.Fatalf("wrong admin token must be 401, got %d", code)
	}
	if code, _ := call(newApp("real-token"), "", ""); code != http.StatusUnauthorized {
		t.Fatalf("missing admin token must be 401, got %d", code)
	}

	// Correct token passes, admin id from header.
	code, body := call(newApp("real-token"), "Bearer real-token", "risk-7")
	if code != 200 || !strings.Contains(body, `"admin":"risk-7"`) {
		t.Fatalf("valid admin token must pass identity: %d %s", code, body)
	}

	// No X-Admin-ID → deterministic default.
	if code, body := call(newApp("real-token"), "Bearer real-token", ""); code != 200 ||
		!strings.Contains(body, `"admin":"admin"`) {
		t.Fatalf("default admin id expected: %d %s", code, body)
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	newApp := func() *fiber.App {
		app := fiber.New()
		app.Use(RequestIDMiddleware())
		app.Get("/r", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })
		return app
	}

	// Incoming id is preserved.
	r := httptest.NewRequest(http.MethodGet, "/r", nil)
	r.Header.Set("X-Request-ID", "req-abc")
	resp, err := newApp().Test(r, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || resp.Header.Get("X-Request-ID") != "req-abc" {
		t.Fatalf("request id must be echoed: %d %q", resp.StatusCode, resp.Header.Get("X-Request-ID"))
	}

	// Missing id is generated.
	r2 := httptest.NewRequest(http.MethodGet, "/r", nil)
	resp2, err := newApp().Test(r2, -1)
	if err != nil {
		t.Fatal(err)
	}
	if got := resp2.Header.Get("X-Request-ID"); got == "" {
		t.Fatal("request id must be generated when absent")
	}
}

func TestWriteAPIErrorEnvelope(t *testing.T) {
	app := fiber.New()
	app.Use(RequestIDMiddleware())
	app.Get("/e", func(c *fiber.Ctx) error {
		return writeAPIError(c, fiber.StatusTeapot, "RG_TEST", "boom", fiber.Map{"k": "v"})
	})
	r := httptest.NewRequest(http.MethodGet, "/e", nil)
	r.Header.Set("X-Request-ID", "req-err")
	resp, err := app.Test(r, -1)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 512)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	if resp.StatusCode != http.StatusTeapot {
		t.Fatalf("status must pass through, got %d", resp.StatusCode)
	}
	for _, want := range []string{"RG_TEST", "boom", "req-err", `"error"`} {
		if !strings.Contains(body, want) {
			t.Errorf("body must contain %q: %s", want, body)
		}
	}
}

func TestGetAdminIDDefault(t *testing.T) {
	app := fiber.New()
	app.Get("/x", func(c *fiber.Ctx) error {
		return c.SendString(getAdminID(c))
	})
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	resp, err := app.Test(r, -1)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 32)
	n, _ := resp.Body.Read(buf)
	if string(buf[:n]) != "admin" {
		t.Fatalf("expected default admin id, got %q", string(buf[:n]))
	}
}
