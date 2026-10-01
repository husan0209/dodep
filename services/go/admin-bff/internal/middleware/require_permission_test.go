package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// setupPermApp builds a Fiber app with canned permissions in Locals,
// exercising RequirePermission through a real HTTP request.
func setupPermApp(t *testing.T, perms interface{}, handler fiber.Handler) *fiber.App {
	t.Helper()
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if perms != nil {
			c.Locals("permissions", perms)
		}
		return c.Next()
	})
	app.Get("/guarded", handler, func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	return app
}

func TestRequirePermission(t *testing.T) {
	tests := []struct {
		name       string
		perms      interface{}
		required   string
		wantStatus int
	}{
		{
			name:       "exact permission granted ([]interface{})",
			perms:      []interface{}{"affiliate.view", "affiliate.manage"},
			required:   "affiliate.manage",
			wantStatus: 200,
		},
		{
			name:       "exact permission granted ([]string)",
			perms:      []string{"affiliate.payout.approve"},
			required:   "affiliate.payout.approve",
			wantStatus: 200,
		},
		{
			name:       "different permission denied (vertical escalation attempt)",
			perms:      []interface{}{"affiliate.view"},
			required:   "affiliate.payout.approve",
			wantStatus: 403,
		},
		{
			name:       "coarse legacy permission does not imply granular",
			perms:      []interface{}{"affiliate.manage"},
			required:   "affiliate.payout.approve",
			wantStatus: 403,
		},
		{
			name:       "empty permission set denied",
			perms:      []interface{}{},
			required:   "affiliate.view",
			wantStatus: 403,
		},
		{
			name:       "missing permissions claim denied (deny by default)",
			perms:      nil,
			required:   "affiliate.view",
			wantStatus: 403,
		},
		{
			name:       "malformed permissions type denied",
			perms:      "affiliate.view",
			required:   "affiliate.view",
			wantStatus: 403,
		},
		{
			name:       "non-string entries ignored",
			perms:      []interface{}{42, true, "affiliate.view"},
			required:   "affiliate.view",
			wantStatus: 200,
		},
		{
			name:       "fraud review isolated from payout approval",
			perms:      []interface{}{"affiliate.view", "affiliate.fraud.review"},
			required:   "affiliate.adjust",
			wantStatus: 403,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := setupPermApp(t, tt.perms, RequirePermission(tt.required))
			req := httptest.NewRequest("GET", "/guarded", nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("got status %d, want %d", resp.StatusCode, tt.wantStatus)
			}
		})
	}
}
