package main

import (
	"context"
	"encoding/json"
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

func testApp(repo *fakeRepo) *fiber.App {
	log, _ := zap.NewDevelopment()
	svc := service.NewUserService(repo, log)
	app := fiber.New()
	setupRoutes(app, svc)
	return app
}

func doRequest(t *testing.T, app *fiber.App, method, target, body string) *http.Response {
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

func TestHTTPGetUser(t *testing.T) {
	app := testApp(&fakeRepo{user: sampleUser()})
	resp := doRequest(t, app, http.MethodGet, "/api/v1/users/42", "")
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

	resp = doRequest(t, app, http.MethodGet, "/api/v1/users/abc", "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	resp = doRequest(t, app, http.MethodGet, "/api/v1/users/99", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestHTTPUpdateUser(t *testing.T) {
	repo := &fakeRepo{updated: sampleUser()}
	app := testApp(repo)
	resp := doRequest(t, app, http.MethodPut, "/api/v1/users/42", `{"username":"renamed"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if repo.updateSeen == nil || repo.updateSeen.UserID != 42 {
		t.Fatalf("user id must come from path: %+v", repo.updateSeen)
	}
	if repo.updateSeen.Username == nil || *repo.updateSeen.Username != "renamed" {
		t.Fatalf("body not parsed: %+v", repo.updateSeen)
	}

	resp = doRequest(t, app, http.MethodPut, "/api/v1/users/42", `{broken`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad body, got %d", resp.StatusCode)
	}

	app = testApp(&fakeRepo{fail: true})
	resp = doRequest(t, app, http.MethodPut, "/api/v1/users/42", `{"username":"x"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 on service error, got %d", resp.StatusCode)
	}
}

func TestHTTPPreferences(t *testing.T) {
	prefs := &domain.UserPreferences{UserID: 42, Language: "uk", Timezone: "Europe/Kyiv"}
	repo := &fakeRepo{prefs: prefs}
	app := testApp(repo)

	resp := doRequest(t, app, http.MethodGet, "/api/v1/users/42/preferences", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var got domain.UserPreferences
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil || got.Language != "uk" {
		t.Fatalf("wrong prefs: %+v %v", got, err)
	}

	resp = doRequest(t, app, http.MethodPut, "/api/v1/users/42/preferences", `{"language":"en","timezone":"UTC"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if repo.upsertSeen == nil || repo.upsertSeen.UserID != 42 || repo.upsertSeen.Language != "en" {
		t.Fatalf("prefs not forwarded: %+v", repo.upsertSeen)
	}

	app = testApp(&fakeRepo{fail: true})
	resp = doRequest(t, app, http.MethodGet, "/api/v1/users/42/preferences", "")
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestHTTPLimits(t *testing.T) {
	limits := &domain.UserLimits{UserID: 42, SelfExclusion: true, UpdatedAt: time.Now()}
	repo := &fakeRepo{prefs: &domain.UserPreferences{UserID: 42}, limits: limits}
	app := testApp(repo)

	resp := doRequest(t, app, http.MethodGet, "/api/v1/users/42/limits", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp = doRequest(t, app, http.MethodPut, "/api/v1/users/42/limits", `{"daily_deposit_limit":"100.00"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if repo.setSeen == nil || repo.setSeen.UserID != 42 ||
		repo.setSeen.DailyDepositLimit == nil || *repo.setSeen.DailyDepositLimit != "100.00" {
		t.Fatalf("limits not forwarded: %+v", repo.setSeen)
	}

	resp = doRequest(t, app, http.MethodPut, "/api/v1/users/42/limits", `{broken`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad body, got %d", resp.StatusCode)
	}

	app = testApp(&fakeRepo{fail: true})
	resp = doRequest(t, app, http.MethodGet, "/api/v1/users/42/limits", "")
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}
