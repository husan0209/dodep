package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/opus-casino/user/internal/domain"
	"github.com/opus-casino/user/internal/service"
)

// fakeRepo implements service.UserRepository in memory.
type fakeRepo struct {
	user       *domain.User
	updated    *domain.User
	prefs      *domain.UserPreferences
	limits     *domain.UserLimits
	updateSeen *domain.UpdateUserRequest
	upsertSeen *domain.UserPreferences
	setSeen    *domain.SetLimitsRequest
	fail       bool
}

func (f *fakeRepo) GetUserByID(_ context.Context, id int64) (*domain.User, error) {
	if f.fail {
		return nil, errBoom
	}
	if f.user == nil || f.user.ID != id {
		return nil, nil
	}
	return f.user, nil
}

func (f *fakeRepo) GetUserByEmail(_ context.Context, _ string) (*domain.User, error) {
	return f.user, nil
}

func (f *fakeRepo) UpdateUser(_ context.Context, req *domain.UpdateUserRequest) (*domain.User, error) {
	f.updateSeen = req
	if f.fail {
		return nil, errBoom
	}
	return f.updated, nil
}

func (f *fakeRepo) SoftDeleteUser(_ context.Context, _ int64) error { return nil }

func (f *fakeRepo) GetPreferences(_ context.Context, _ int64) (*domain.UserPreferences, error) {
	if f.fail {
		return nil, errBoom
	}
	return f.prefs, nil
}

func (f *fakeRepo) UpsertPreferences(_ context.Context, pref *domain.UserPreferences) error {
	f.upsertSeen = pref
	if f.fail {
		return errBoom
	}
	return nil
}

func (f *fakeRepo) GetLimits(_ context.Context, _ int64) (*domain.UserLimits, error) {
	if f.fail {
		return nil, errBoom
	}
	return f.limits, nil
}

func (f *fakeRepo) SetLimits(_ context.Context, _ int64, req *domain.SetLimitsRequest) error {
	f.setSeen = req
	if f.fail {
		return errBoom
	}
	return nil
}

func (f *fakeRepo) GetActivity(_ context.Context, _ int64, _, _ int) ([]map[string]interface{}, int, error) {
	return nil, 0, nil
}

var _ service.UserRepository = (*fakeRepo)(nil)

type boomError struct{}

func (boomError) Error() string { return "db is down" }

var errBoom error = boomError{}

func sampleUser() *domain.User {
	return &domain.User{
		ID: 42, Email: "user@example.com", Username: "player42",
		CountryCode: "UA", CurrencyCode: "USD",
		Status:    domain.UserStatusActive,
		KYCLevel:  domain.KYCLevelVerified,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
}

// authedTestApp mounts the routes behind the real middleware so the tests
// exercise the same path production takes. The token subject is `user-42`,
// which resolves to user id 42.
func authedTestApp(repo *fakeRepo) *fiber.App {
	log, _ := zap.NewDevelopment()
	svc := service.NewUserService(repo, log)
	app := fiber.New()
	setupRoutes(app, svc, NewTokenVerifier(testSecret, ""))
	return app
}

func authedRequest(t *testing.T, app *fiber.App, method, target, body string) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+signHS256(t, testSecret, nil))
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return resp
}

func anonRequest(t *testing.T, app *fiber.App, method, target, body string) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return resp
}

// TestRoutesRejectAnonymous is the regression test for the IDOR: before this
// change the API had no authentication at all and took the user id from the URL.
func TestRoutesRejectAnonymous(t *testing.T) {
	app := authedTestApp(&fakeRepo{user: sampleUser()})
	targets := []struct {
		method, path, body string
	}{
		{http.MethodGet, "/api/v1/users/me", ""},
		{http.MethodPut, "/api/v1/users/me", `{"username":"x"}`},
		{http.MethodGet, "/api/v1/users/me/preferences", ""},
		{http.MethodPut, "/api/v1/users/me/preferences", `{"language":"uk"}`},
		{http.MethodGet, "/api/v1/users/me/limits", ""},
		{http.MethodPut, "/api/v1/users/me/limits", `{"daily_deposit_limit":"10.00"}`},
	}
	for _, tc := range targets {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			if resp := anonRequest(t, app, tc.method, tc.path, tc.body); resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("expected 401 for anonymous request, got %d", resp.StatusCode)
			}
		})
	}
}

// TestLegacyUserIDRoutesGone pins the removal of the /users/:id surface. If a
// future change reintroduces an id-in-path route this test fails.
func TestLegacyUserIDRoutesGone(t *testing.T) {
	app := authedTestApp(&fakeRepo{user: sampleUser()})
	for _, path := range []string{
		"/api/v1/users/42",
		"/api/v1/users/42/preferences",
		"/api/v1/users/42/limits",
	} {
		if resp := authedRequest(t, app, http.MethodGet, path, ""); resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s must not exist, got %d", path, resp.StatusCode)
		}
	}
}

func TestGetMe(t *testing.T) {
	app := authedTestApp(&fakeRepo{user: sampleUser()})
	resp := authedRequest(t, app, http.MethodGet, "/api/v1/users/me", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var got domain.User
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if got.ID != 42 || got.Email != "user@example.com" {
		t.Fatalf("wrong user: %+v", got)
	}
}

func TestGetMeMapsErrorsToStatus(t *testing.T) {
	// A datastore failure must surface as 500, not 404. The previous handler
	// answered 404 for every error, masking outages.
	app := authedTestApp(&fakeRepo{fail: true})
	if resp := authedRequest(t, app, http.MethodGet, "/api/v1/users/me", ""); resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 on repository failure, got %d", resp.StatusCode)
	}

	// A genuinely absent user is 404.
	app = authedTestApp(&fakeRepo{user: nil})
	if resp := authedRequest(t, app, http.MethodGet, "/api/v1/users/me", ""); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown user, got %d", resp.StatusCode)
	}
}

func TestUpdateMe(t *testing.T) {
	repo := &fakeRepo{updated: sampleUser()}
	app := authedTestApp(repo)

	resp := authedRequest(t, app, http.MethodPut, "/api/v1/users/me", `{"username":"renamed"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if repo.updateSeen == nil || repo.updateSeen.UserID != 42 {
		t.Fatalf("user id must come from the token: %+v", repo.updateSeen)
	}
	if repo.updateSeen.Username == nil || *repo.updateSeen.Username != "renamed" {
		t.Fatalf("body not parsed: %+v", repo.updateSeen)
	}

	if resp := authedRequest(t, app, http.MethodPut, "/api/v1/users/me", `{broken`); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad body, got %d", resp.StatusCode)
	}
}

// TestUpdateMeIgnoresBodyUserID is the mass-assignment guard: a user_id in the
// payload must never select the record that gets updated.
func TestUpdateMeIgnoresBodyUserID(t *testing.T) {
	repo := &fakeRepo{updated: sampleUser()}
	app := authedTestApp(repo)

	resp := authedRequest(t, app, http.MethodPut, "/api/v1/users/me",
		`{"user_id":999,"username":"renamed"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if repo.updateSeen.UserID != 42 {
		t.Fatalf("body user_id leaked through: got %d, want 42", repo.updateSeen.UserID)
	}
}

// TestUpdateMeUnknownUserIs404 pins the fix for the `200 null` response: an
// UPDATE that matches no row is a 404, not a success with a null body.
func TestUpdateMeUnknownUserIs404(t *testing.T) {
	repo := &fakeRepo{updated: nil}
	app := authedTestApp(repo)

	resp := authedRequest(t, app, http.MethodPut, "/api/v1/users/me", `{"username":"renamed"}`)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 when no row was updated, got %d", resp.StatusCode)
	}
}

func TestPreferences(t *testing.T) {
	prefs := &domain.UserPreferences{UserID: 42, Language: "uk", Timezone: "Europe/Kyiv"}
	repo := &fakeRepo{prefs: prefs}
	app := authedTestApp(repo)

	resp := authedRequest(t, app, http.MethodGet, "/api/v1/users/me/preferences", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp = authedRequest(t, app, http.MethodPut, "/api/v1/users/me/preferences",
		`{"user_id":999,"language":"uk","timezone":"Europe/Kyiv"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if repo.upsertSeen == nil || repo.upsertSeen.UserID != 42 {
		t.Fatalf("preferences user id must come from the token: %+v", repo.upsertSeen)
	}

	resp = authedRequest(t, app, http.MethodPut, "/api/v1/users/me/preferences", `{oops`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad body, got %d", resp.StatusCode)
	}
}

func TestGetLimits(t *testing.T) {
	limits := &domain.UserLimits{UserID: 42, SelfExclusion: false}
	app := authedTestApp(&fakeRepo{limits: limits})
	resp := authedRequest(t, app, http.MethodGet, "/api/v1/users/me/limits", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// TestSetLimitsRejectsUnsafeValues covers the responsible-gambling bypasses
// that the unvalidated *string money fields previously allowed.
func TestSetLimitsRejectsUnsafeValues(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"negative deposit limit", `{"daily_deposit_limit":"-5000.00"}`},
		{"negative loss limit", `{"daily_loss_limit":"-1"}`},
		{"exponent notation", `{"daily_deposit_limit":"1e9"}`},
		{"not a number", `{"daily_deposit_limit":"abc"}`},
		{"three decimals", `{"daily_deposit_limit":"10.005"}`},
		{"leading plus", `{"daily_deposit_limit":"+10.00"}`},
		{"whitespace padded", `{"daily_deposit_limit":" 10.00 "}`},
		{"empty string", `{"daily_deposit_limit":""}`},
		{"session limit zero", `{"session_time_minutes":0}`},
		{"session limit negative", `{"session_time_minutes":-30}`},
		{"session limit too long", `{"session_time_minutes":100000}`},
		{"clearing self-exclusion", `{"self_exclusion":false}`},
		{"daily above weekly", `{"daily_deposit_limit":"500.00","weekly_deposit_limit":"100.00"}`},
		{"weekly above monthly", `{"weekly_bet_limit":"900.00","monthly_bet_limit":"100.00"}`},
		{"daily loss above weekly loss", `{"daily_loss_limit":"800","weekly_loss_limit":"200"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{limits: &domain.UserLimits{UserID: 42}}
			app := authedTestApp(repo)
			resp := authedRequest(t, app, http.MethodPut, "/api/v1/users/me/limits", tc.body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("expected 400 for %s, got %d", tc.body, resp.StatusCode)
			}
			if repo.setSeen != nil {
				t.Fatal("rejected limits must never reach the repository")
			}
		})
	}
}

func TestSetLimitsAcceptsValidValues(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"setting self-exclusion", `{"self_exclusion":true}`},
		{"setting and keeping self-exclusion", `{"self_exclusion":true,"daily_deposit_limit":"100.00"}`},
		{"integer amount", `{"daily_deposit_limit":"100"}`},
		{"one decimal", `{"daily_deposit_limit":"99.9"}`},
		{"monotonic family", `{"daily_deposit_limit":"100.00","weekly_deposit_limit":"500.00","monthly_deposit_limit":"1000.00"}`},
		{"equal bounds are fine", `{"daily_deposit_limit":"100.00","weekly_deposit_limit":"100.00"}`},
		{"session limit one minute", `{"session_time_minutes":1}`},
		{"session limit 24h", `{"session_time_minutes":1440}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{limits: &domain.UserLimits{UserID: 42}}
			app := authedTestApp(repo)
			resp := authedRequest(t, app, http.MethodPut, "/api/v1/users/me/limits", tc.body)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200 for %s, got %d", tc.body, resp.StatusCode)
			}
			if repo.setSeen == nil {
				t.Fatal("valid limits must reach the repository")
			}
			if repo.setSeen.UserID != 42 {
				t.Fatalf("limits user id must come from the token: %d", repo.setSeen.UserID)
			}
		})
	}
}

func TestSetLimitsMapsRepositoryFailureTo500(t *testing.T) {
	app := authedTestApp(&fakeRepo{fail: true})
	resp := authedRequest(t, app, http.MethodPut, "/api/v1/users/me/limits", `{"daily_deposit_limit":"10.00"}`)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

// TestErrorMappingHelper pins the sentinel-based status mapping.
func TestErrorMappingHelper(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"not found", domain.ErrUserNotFound, http.StatusNotFound},
		{"wrapped not found", errWrap(domain.ErrUserNotFound), http.StatusNotFound},
		{"invalid id", domain.ErrInvalidUserID, http.StatusBadRequest},
		{"wrapped invalid id", errWrap(domain.ErrInvalidUserID), http.StatusBadRequest},
		{"anything else is internal", errBoom, http.StatusInternalServerError},
		{"errors.Is is not string matching", errors.New("user not found"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/", func(c *fiber.Ctx) error { return respondUserError(c, tc.err, "op") })
			resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil), -1)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			if resp.StatusCode != tc.want {
				t.Fatalf("expected %d, got %d", tc.want, resp.StatusCode)
			}
		})
	}
}

func errWrap(err error) error {
	return fmt.Errorf("layer: %w", err)
}
