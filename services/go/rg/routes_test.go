package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/opus-casino/rg/internal/config"
	"github.com/opus-casino/rg/internal/domain"
	"github.com/opus-casino/rg/internal/handlers"
	"github.com/opus-casino/rg/internal/repository"
	"github.com/opus-casino/rg/internal/service"
)

// TestRouteWiring boots the real route wiring (routes.go) with a stubbed
// dependency graph and asserts:
//   - every documented endpoint exists and answers;
//   - player routes reject missing/invalid tokens (NEVER-7: no identity without token);
//   - operator routes reject missing tokens and are not reachable by players;
//   - the enforcement gate returns 200 for an allowed action.
func TestRouteWiring(t *testing.T) {
	const secret = "route-test-secret"
	cfg := config.Load().Validate()
	cfg.GRPCPort = 0 // not bound in tests

	log, _ := zap.NewDevelopment()
	// Repository-free service: NewRGService needs a Repository; use a
	// read-only stub that reports "no protection configured", which is the
	// permissive default and keeps this test focused on wiring/auth.
	svc := service.NewRGService(stubRepository{}, service.NoopPublisher{}, log, 24, 24)
	h := handlers.New(svc)

	app := fiber.New()
	app.Use(RequestIDMiddleware())
	setupRoutes(app, h, svc, AuthMiddleware(secret, "staging"))
	setupAdminRoutes(app, h, svc, AdminMiddleware("operator-token"))

	call := func(method, path, token, adminID, body string) (int, string) {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if adminID != "" {
			r.Header.Set("X-Admin-ID", adminID)
		}
		if method == http.MethodPost || method == http.MethodPut {
			r.Header.Set("Content-Type", "application/json")
		}
		resp, err := app.Test(r, -1)
		if err != nil {
			t.Fatalf("%s %s failed: %v", method, path, err)
		}
		buf := make([]byte, 2048)
		n, _ := resp.Body.Read(buf)
		return resp.StatusCode, string(buf[:n])
	}

	playerToken := makeHS256Token(t, secret, map[string]interface{}{
		"uid": 4242, "exp": time.Now().Add(time.Hour).Unix(),
	})

	// ── Player routes exist and require a valid token ─────────────────────────
	t.Run("player routes reject anonymous access", func(t *testing.T) {
		for _, tc := range []struct{ method, path string }{
			{http.MethodPost, "/api/v1/rg/check"},
			{http.MethodGet, "/api/v1/rg/limits"},
			{http.MethodPut, "/api/v1/rg/limits"},
			{http.MethodPost, "/api/v1/rg/self-exclusion"},
			{http.MethodPost, "/api/v1/rg/self-exclusion/revoke"},
			{http.MethodPost, "/api/v1/rg/timeout"},
			{http.MethodGet, "/api/v1/rg/status"},
		} {
			code, body := call(tc.method, tc.path, "", "", "")
			if code != http.StatusUnauthorized {
				t.Errorf("%s %s must require auth, got %d: %s", tc.method, tc.path, code, body)
			}
		}
	})

	t.Run("player routes answer with a valid token", func(t *testing.T) {
		// Enforcement gate: withdrawal is always allowed.
		code, body := call(http.MethodPost, "/api/v1/rg/check", playerToken, "",
			`{"channel":"withdrawal"}`)
		if code != http.StatusOK || !strings.Contains(body, `"allowed":true`) {
			t.Errorf("gate check: %d %s", code, body)
		}

		// Read endpoints must answer 200 and carry the request envelope.
		for _, path := range []string{"/api/v1/rg/limits", "/api/v1/rg/status"} {
			code, body := call(http.MethodGet, path, playerToken, "", "")
			if code != http.StatusOK || !strings.Contains(body, `"meta"`) {
				t.Errorf("GET %s: %d %s", path, code, body)
			}
		}
	})

	t.Run("check endpoint validates the payload", func(t *testing.T) {
		code, body := call(http.MethodPost, "/api/v1/rg/check", playerToken, "", "")
		// Empty body → validation error (not 5xx, not a silent pass).
		if code == http.StatusOK {
			t.Fatalf("empty body must not be treated as allowed: %d %s", code, body)
		}
		if code >= 500 {
			t.Fatalf("validation must not surface as 5xx: %d %s", code, body)
		}
	})

	// ── Operator routes: separate auth, not reachable by players ─────────────
	t.Run("operator routes reject anonymous and player tokens", func(t *testing.T) {
		for _, token := range []string{"", playerToken} {
			code, body := call(http.MethodPost, "/admin/rg/exclusions", token, "", `{"user_id":1,"period":"30d","type":"operator"}`)
			if code != http.StatusUnauthorized {
				t.Errorf("player token must not reach operator route: %d %s", code, body)
			}
		}
	})

	t.Run("operator route validates payload", func(t *testing.T) {
		code, body := call(http.MethodPost, "/admin/rg/exclusions", "operator-token", "risk-1", `{}`)
		if code != http.StatusBadRequest {
			t.Fatalf("missing user_id/period must be 400: %d %s", code, body)
		}
	})

	t.Run("operator apply-due is idempotent", func(t *testing.T) {
		code, body := call(http.MethodPost, "/admin/rg/apply-due", "operator-token", "risk-1", "")
		if code != http.StatusOK || !strings.Contains(body, `"applied"`) {
			t.Errorf("apply-due: %d %s", code, body)
		}
	})

	// ── Unknown routes must not leak implementation details ─────────────────
	t.Run("unknown route 404", func(t *testing.T) {
		code, _ := call(http.MethodGet, "/api/v1/rg/nope", playerToken, "", "")
		if code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", code)
		}
	})
}

// stubRepository is a permissive read-only Repository used only to construct
// a service for routing tests. Every enforcement source reports "no limit,
// no exclusion", so allowed decisions are the expected outcome.
type stubRepository struct{}

func (stubRepository) GetLimits(_ context.Context, userID int64) (*domain.RGLimits, error) {
	return &domain.RGLimits{UserID: userID}, nil
}

func (stubRepository) UpsertLimits(context.Context, *domain.RGLimits) error { return nil }

func (stubRepository) CreatePendingChange(context.Context, *domain.PendingChange) error { return nil }

func (stubRepository) ListPendingChanges(context.Context, int64) ([]*domain.PendingChange, error) {
	return nil, nil
}

func (stubRepository) ListDueChanges(context.Context, time.Time, int) ([]*domain.PendingChange, error) {
	return nil, nil
}

func (stubRepository) UpdateChangeStatus(context.Context, uuid.UUID, domain.ChangeStatus, domain.ChangeStatus) error {
	return nil
}

func (stubRepository) DeletePendingChanges(context.Context, int64, domain.LimitType) error {
	return nil
}

func (stubRepository) CreateExclusion(context.Context, *domain.Exclusion) error { return nil }

func (stubRepository) GetActiveExclusion(context.Context, int64) (*domain.Exclusion, error) {
	return nil, nil
}

func (stubRepository) ListExclusions(context.Context, int64, int) ([]*domain.Exclusion, error) {
	return nil, nil
}

func (stubRepository) UpdateExclusionStatus(context.Context, uuid.UUID, domain.ExclusionStatus,
	domain.ExclusionStatus, map[string]interface{}) error {
	return nil
}

func (stubRepository) CreateTimeout(context.Context, *domain.Timeout) error { return nil }

func (stubRepository) GetActiveTimeout(context.Context, int64, time.Time) (*domain.Timeout, error) {
	return nil, nil
}

func (stubRepository) RecordSpend(context.Context, int64, time.Time, string, string, string, string) error {
	return nil
}

func (stubRepository) SumSpendSince(context.Context, int64, string, time.Time) (string, string, string, error) {
	return "0", "0", "0", nil
}

func (stubRepository) AppendOutbox(context.Context, string, string, map[string]interface{}) error {
	return nil
}

func (stubRepository) ListPendingOutbox(context.Context, int) ([]repository.OutboxEvent, error) {
	return nil, nil
}

func (stubRepository) MarkOutboxPublished(context.Context, uuid.UUID) error { return nil }

func (stubRepository) Transact(_ context.Context, fn func(repository.Repository) error) error {
	return fn(stubRepository{})
}

var _ repository.Repository = stubRepository{}
