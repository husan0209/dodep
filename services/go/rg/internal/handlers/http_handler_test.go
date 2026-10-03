package handlers

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/opus-casino/rg/internal/domain"
	"github.com/opus-casino/rg/internal/repository"
	"github.com/opus-casino/rg/internal/service"
)

// fakeRepo is an in-memory repository.Repository for handler tests.
type fakeRepo struct {
	mu       sync.Mutex
	limits   map[int64]*domain.RGLimits
	pending  map[uuid.UUID]*domain.PendingChange
	excl     map[int64][]*domain.Exclusion
	timeouts map[int64][]*domain.Timeout
	spend    map[int64]map[string]*daySpend
	outbox   []string
}

type daySpend struct{ dep, wag, pay decimal.Decimal }

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		limits:   map[int64]*domain.RGLimits{},
		pending:  map[uuid.UUID]*domain.PendingChange{},
		excl:     map[int64][]*domain.Exclusion{},
		timeouts: map[int64][]*domain.Timeout{},
		spend:    map[int64]map[string]*daySpend{},
	}
}

func (f *fakeRepo) GetLimits(_ context.Context, userID int64) (*domain.RGLimits, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if l, ok := f.limits[userID]; ok {
		cp := *l
		return &cp, nil
	}
	return &domain.RGLimits{UserID: userID}, nil
}

func (f *fakeRepo) UpsertLimits(_ context.Context, limits *domain.RGLimits) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *limits
	f.limits[limits.UserID] = &cp
	return nil
}

func (f *fakeRepo) CreatePendingChange(_ context.Context, ch *domain.PendingChange) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *ch
	f.pending[ch.ID] = &cp
	return nil
}

func (f *fakeRepo) ListPendingChanges(_ context.Context, userID int64) ([]*domain.PendingChange, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.PendingChange
	for _, ch := range f.pending {
		if ch.UserID == userID && ch.Status == domain.ChangePending {
			cp := *ch
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (f *fakeRepo) ListDueChanges(_ context.Context, now time.Time, limit int) ([]*domain.PendingChange, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.PendingChange
	for _, ch := range f.pending {
		if ch.Status == domain.ChangePending && !now.Before(ch.EffectiveAt) {
			cp := *ch
			out = append(out, &cp)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (f *fakeRepo) UpdateChangeStatus(_ context.Context, id uuid.UUID, from domain.ChangeStatus, to domain.ChangeStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	ch, ok := f.pending[id]
	if !ok || ch.Status != from {
		return domain.ErrConflict
	}
	ch.Status = to
	return nil
}

func (f *fakeRepo) DeletePendingChanges(_ context.Context, userID int64, limitType domain.LimitType) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, ch := range f.pending {
		if ch.UserID == userID && ch.LimitType == limitType && ch.Status == domain.ChangePending {
			delete(f.pending, id)
		}
	}
	return nil
}

func (f *fakeRepo) CreateExclusion(_ context.Context, e *domain.Exclusion) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *e
	f.excl[e.UserID] = append(f.excl[e.UserID], &cp)
	return nil
}

func (f *fakeRepo) GetActiveExclusion(_ context.Context, userID int64) (*domain.Exclusion, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	list := f.excl[userID]
	for i := len(list) - 1; i >= 0; i-- {
		if list[i].Status == domain.ExclusionActive {
			cp := *list[i]
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *fakeRepo) ListExclusions(_ context.Context, userID int64, _ int) ([]*domain.Exclusion, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.Exclusion
	for _, e := range f.excl[userID] {
		cp := *e
		out = append(out, &cp)
	}
	return out, nil
}

func (f *fakeRepo) UpdateExclusionStatus(_ context.Context, id uuid.UUID, from domain.ExclusionStatus, to domain.ExclusionStatus, patch map[string]interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, list := range f.excl {
		for _, e := range list {
			if e.ID == id {
				if e.Status != from || !e.Status.CanTransitionTo(to, e.Permanent) {
					return domain.ErrConflict
				}
				e.Status = to
				if v, ok := patch["revoked_by"].(string); ok {
					e.RevokedBy = v
				}
				return nil
			}
		}
	}
	return domain.ErrConflict
}

func (f *fakeRepo) CreateTimeout(_ context.Context, t *domain.Timeout) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *t
	f.timeouts[t.UserID] = append(f.timeouts[t.UserID], &cp)
	return nil
}

func (f *fakeRepo) GetActiveTimeout(_ context.Context, userID int64, now time.Time) (*domain.Timeout, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, t := range f.timeouts[userID] {
		if t.IsActiveAt(now) {
			cp := *t
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *fakeRepo) RecordSpend(_ context.Context, userID int64, day time.Time, currency, dep, wag, pay string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.spend[userID] == nil {
		f.spend[userID] = map[string]*daySpend{}
	}
	k := day.UTC().Truncate(24*time.Hour).Format("2006-01-02") + "|" + currency
	ds := f.spend[userID][k]
	if ds == nil {
		ds = &daySpend{}
		f.spend[userID][k] = ds
	}
	ds.dep = ds.dep.Add(decOrZero(dep))
	ds.wag = ds.wag.Add(decOrZero(wag))
	ds.pay = ds.pay.Add(decOrZero(pay))
	return nil
}

func (f *fakeRepo) SumSpendSince(_ context.Context, userID int64, currency string, since time.Time) (string, string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var dep, wag, pay decimal.Decimal
	sinceKey := since.UTC().Truncate(24 * time.Hour).Format("2006-01-02")
	for k, ds := range f.spend[userID] {
		parts := strings.SplitN(k, "|", 2)
		if len(parts) != 2 || parts[1] != currency || parts[0] < sinceKey {
			continue
		}
		dep = dep.Add(ds.dep)
		wag = wag.Add(ds.wag)
		pay = pay.Add(ds.pay)
	}
	return dep.String(), wag.String(), pay.String(), nil
}

func decOrZero(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero
	}
	return d
}

func (f *fakeRepo) AppendOutbox(_ context.Context, topic, _ string, _ map[string]interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.outbox = append(f.outbox, topic)
	return nil
}

func (f *fakeRepo) ListPendingOutbox(_ context.Context, _ int) ([]repository.OutboxEvent, error) {
	return nil, nil
}

func (f *fakeRepo) MarkOutboxPublished(_ context.Context, _ uuid.UUID) error { return nil }

func (f *fakeRepo) Transact(_ context.Context, fn func(repository.Repository) error) error {
	return fn(f)
}

var _ repository.Repository = (*fakeRepo)(nil)

func newTestApp() (*fiber.App, *fakeRepo, *service.RGService) {
	log, _ := zap.NewDevelopment()
	repo := newFakeRepo()
	svc := service.NewRGService(repo, service.NoopPublisher{}, log, 24, 24)
	h := New(svc)

	// Mirror routes.go wiring with an identity-injecting stub auth middleware:
	// handler tests exercise status codes/mapping, middleware is covered
	// separately in the package-main tests.
	userStub := func(c *fiber.Ctx) error {
		c.Locals("user_id", int64(42))
		return c.Next()
	}
	adminStub := func(c *fiber.Ctx) error {
		c.Locals("admin_id", "admin-1")
		return c.Next()
	}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("request_id", "req-test")
		return c.Next()
	})
	app.Get("/health", h.Liveness)
	app.Get("/ready", h.Readiness)

	rg := app.Group("/api/v1/rg", userStub)
	rg.Post("/check", h.Check)
	rg.Put("/limits", h.SetLimits)
	rg.Get("/limits", h.GetLimits)
	rg.Post("/self-exclusion", h.StartSelfExclusion)
	rg.Post("/self-exclusion/revoke", h.RevokeSelfExclusion)
	rg.Post("/timeout", h.StartTimeout)
	rg.Get("/status", h.GetStatus)

	adminGroup := app.Group("/admin/rg", adminStub)
	adminGroup.Post("/exclusions", h.AdminStartExclusion)
	adminGroup.Post("/apply-due", h.AdminApplyDue(svc))

	return app, repo, svc
}

func doJSON(t *testing.T, app *fiber.App, method, path, body string) (int, string) {
	t.Helper()
	var req *httptest.ResponseRecorder
	_ = req
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(r, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	buf := make([]byte, 0, 512)
	tmp := make([]byte, 512)
	for {
		n, err := resp.Body.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
	}
	return resp.StatusCode, string(buf)
}

func TestHealthAndReady(t *testing.T) {
	app, _, _ := newTestApp()
	if code, body := doJSON(t, app, "GET", "/health", ""); code != 200 || !strings.Contains(body, `"service":"rg"`) {
		t.Errorf("health: %d %s", code, body)
	}
	if code, _ := doJSON(t, app, "GET", "/ready", ""); code != 200 {
		t.Errorf("ready: %d", code)
	}
}

func TestCheckEndpoint(t *testing.T) {
	app, _, _ := newTestApp()

	// Withdrawal is always allowed → 200.
	code, body := doJSON(t, app, "POST", "/api/v1/rg/check", `{"channel":"withdrawal"}`)
	if code != 200 || !strings.Contains(body, `"allowed":true`) {
		t.Errorf("withdrawal check: %d %s", code, body)
	}

	// Bad channel → 400.
	if code, body := doJSON(t, app, "POST", "/api/v1/rg/check", `{"channel":"slot"}`); code != 400 ||
		!strings.Contains(body, "RG_VALIDATION_FAILED") {
		t.Errorf("bad channel: %d %s", code, body)
	}

	// Broken JSON → 400.
	if code, _ := doJSON(t, app, "POST", "/api/v1/rg/check", `{broken`); code != 400 {
		t.Errorf("broken body: %d", code)
	}

	// Bad amount → 400.
	if code, body := doJSON(t, app, "POST", "/api/v1/rg/check", `{"channel":"bet","amount":"abc"}`); code != 400 ||
		!strings.Contains(body, "RG_INVALID_AMOUNT") {
		t.Errorf("bad amount: %d %s", code, body)
	}

	// Negative amount → 400.
	if code, _ := doJSON(t, app, "POST", "/api/v1/rg/check", `{"channel":"bet","amount":"-5"}`); code != 400 {
		t.Errorf("negative amount: %d", code)
	}

	// Excluded user → 403 with reason code.
	if code, body := doJSON(t, app, "POST", "/api/v1/rg/self-exclusion", `{"period":"permanent"}`); code != 201 {
		t.Fatalf("exclude: %d %s", code, body)
	}
	code, body = doJSON(t, app, "POST", "/api/v1/rg/check", `{"channel":"bet","amount":"10"}`)
	if code != 403 || !strings.Contains(body, "RG_SELF_EXCLUDED") {
		t.Errorf("excluded bet must be 403: %d %s", code, body)
	}
}

func TestLimitsEndpoints(t *testing.T) {
	app, _, _ := newTestApp()

	code, body := doJSON(t, app, "PUT", "/api/v1/rg/limits", `{"deposit_daily":"100","session_minutes":60}`)
	if code != 200 || !strings.Contains(body, `"deposit_daily":"100"`) {
		t.Fatalf("set limits: %d %s", code, body)
	}

	code, body = doJSON(t, app, "GET", "/api/v1/rg/limits", "")
	if code != 200 || !strings.Contains(body, `"session_minutes":60`) {
		t.Errorf("get limits: %d %s", code, body)
	}

	// Increase → staged as pending, not effective.
	if code, body := doJSON(t, app, "PUT", "/api/v1/rg/limits", `{"deposit_daily":"900"}`); code != 200 ||
		!strings.Contains(body, `"limit_type":"deposit_daily"`) {
		t.Errorf("increase must be pending: %d %s", code, body)
	}
	if _, body := doJSON(t, app, "GET", "/api/v1/rg/limits", ""); !strings.Contains(body, `"deposit_daily":"100"`) {
		t.Errorf("effective limit must stay 100 during cooling: %s", body)
	}

	// Bad value → 400.
	if code, _ := doJSON(t, app, "PUT", "/api/v1/rg/limits", `{"deposit_daily":"-1"}`); code != 400 {
		t.Errorf("negative limit: %d", code)
	}
	if code, _ := doJSON(t, app, "PUT", "/api/v1/rg/limits", `{"reality_check_minutes":5}`); code != 400 {
		t.Errorf("reality check below 15: %d", code)
	}
}

func TestSelfExclusionEndpoints(t *testing.T) {
	app, _, _ := newTestApp()

	// Bad period → 400.
	if code, _ := doJSON(t, app, "POST", "/api/v1/rg/self-exclusion", `{"period":"3y"}`); code != 400 {
		t.Errorf("bad period: %d", code)
	}

	if code, body := doJSON(t, app, "POST", "/api/v1/rg/self-exclusion", `{"period":"30d"}`); code != 201 ||
		!strings.Contains(body, `"permanent":false`) {
		t.Fatalf("start exclusion: %d %s", code, body)
	}

	// Duplicate → 409.
	if code, body := doJSON(t, app, "POST", "/api/v1/rg/self-exclusion", `{"period":"30d"}`); code != 409 ||
		!strings.Contains(body, "RG_CONFLICT") {
		t.Errorf("duplicate exclusion: %d %s", code, body)
	}

	// Revoke without confirm → 422.
	if code, body := doJSON(t, app, "POST", "/api/v1/rg/self-exclusion/revoke", `{"confirm":false}`); code != 422 ||
		!strings.Contains(body, "RG_REVOKE_DENIED") {
		t.Errorf("revoke needs confirm: %d %s", code, body)
	}

	// Revoke while active → 422.
	if code, _ := doJSON(t, app, "POST", "/api/v1/rg/self-exclusion/revoke", `{"confirm":true}`); code != 422 {
		t.Errorf("revoke while active: %d", code)
	}
}

func TestPermanentExclusionRevokeForbidden(t *testing.T) {
	app, _, _ := newTestApp()
	if code, _ := doJSON(t, app, "POST", "/api/v1/rg/self-exclusion", `{"period":"permanent"}`); code != 201 {
		t.Fatal("exclude failed")
	}
	code, body := doJSON(t, app, "POST", "/api/v1/rg/self-exclusion/revoke", `{"confirm":true}`)
	if code != 422 || !strings.Contains(body, "RG_PERMANENT") {
		t.Errorf("permanent must never revoke: %d %s", code, body)
	}
}

func TestTimeoutEndpoint(t *testing.T) {
	app, _, _ := newTestApp()

	if code, _ := doJSON(t, app, "POST", "/api/v1/rg/timeout", `{"period":"7d"}`); code != 201 {
		t.Error("start timeout failed")
	}
	// Duplicate → 409.
	if code, body := doJSON(t, app, "POST", "/api/v1/rg/timeout", `{"period":"24h"}`); code != 409 ||
		!strings.Contains(body, "RG_CONFLICT") {
		t.Errorf("duplicate timeout: %d %s", code, body)
	}
	// Bet now blocked with cool-off.
	if code, body := doJSON(t, app, "POST", "/api/v1/rg/check", `{"channel":"bet","amount":"1"}`); code != 403 ||
		!strings.Contains(body, "RG_COOL_OFF") {
		t.Errorf("timeout must block bet: %d %s", code, body)
	}
	// Bad period → 400.
	if code, _ := doJSON(t, app, "POST", "/api/v1/rg/timeout", `{"period":"1y"}`); code != 400 {
		t.Errorf("bad timeout period: %d", code)
	}
}

func TestStatusEndpoint(t *testing.T) {
	app, _, _ := newTestApp()

	code, body := doJSON(t, app, "GET", "/api/v1/rg/status", "")
	if code != 200 || !strings.Contains(body, `"gambling_allowed":true`) {
		t.Fatalf("clean status: %d %s", code, body)
	}
	if !strings.Contains(body, `"reality_check_minutes":60`) {
		t.Errorf("default reality check must be exposed: %s", body)
	}

	if code, _ := doJSON(t, app, "POST", "/api/v1/rg/self-exclusion", `{"period":"7d"}`); code != 201 {
		t.Fatal("exclude failed")
	}
	code, body = doJSON(t, app, "GET", "/api/v1/rg/status", "")
	if code != 200 || !strings.Contains(body, `"gambling_allowed":false`) ||
		!strings.Contains(body, "RG_SELF_EXCLUDED") {
		t.Errorf("excluded status: %d %s", code, body)
	}
}

func TestAdminEndpoints(t *testing.T) {
	app, _, _ := newTestApp()

	// Operator exclusion for another user.
	code, body := doJSON(t, app, "POST", "/admin/rg/exclusions", `{"user_id":7,"period":"30d","type":"operator"}`)
	if code != 201 || !strings.Contains(body, `"type":"operator"`) {
		t.Fatalf("admin exclusion: %d %s", code, body)
	}

	// Validation: bad type / missing user.
	if code, _ := doJSON(t, app, "POST", "/admin/rg/exclusions", `{"user_id":7,"period":"30d","type":"nope"}`); code != 400 {
		t.Errorf("bad type: %d", code)
	}
	if code, _ := doJSON(t, app, "POST", "/admin/rg/exclusions", `{"period":"30d","type":"operator"}`); code != 400 {
		t.Errorf("missing user_id: %d", code)
	}

	// apply-due is idempotent when nothing is due.
	if code, body := doJSON(t, app, "POST", "/admin/rg/apply-due", ""); code != 200 ||
		!strings.Contains(body, `"applied":0`) {
		t.Errorf("apply-due: %d %s", code, body)
	}
}

func TestDepositLimitViaHTTP(t *testing.T) {
	app, _, svc := newTestApp()

	if code, _ := doJSON(t, app, "PUT", "/api/v1/rg/limits", `{"deposit_daily":"100"}`); code != 200 {
		t.Fatal("set limits failed")
	}
	if err := svc.RecordSpend(context.Background(), service.RecordSpendInput{
		UserID: 42, Kind: service.SpendDeposit, Amount: decimal.NewFromInt(90), Currency: "USD",
	}); err != nil {
		t.Fatal(err)
	}

	// Within limit.
	if code, body := doJSON(t, app, "POST", "/api/v1/rg/check", `{"channel":"deposit","amount":"5","currency":"USD"}`); code != 200 ||
		!strings.Contains(body, `"allowed":true`) {
		t.Errorf("5 must pass: %d %s", code, body)
	}
	// Over limit → 422 with details.
	code, body := doJSON(t, app, "POST", "/api/v1/rg/check", `{"channel":"deposit","amount":"20","currency":"USD"}`)
	if code != 422 || !strings.Contains(body, "RG_DEPOSIT_LIMIT") {
		t.Errorf("over limit must be 422: %d %s", code, body)
	}
}

func TestNewHandlerRequiresService(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil service")
		}
	}()
	New(nil)
}

func TestMapErrorUnknownFallsBackTo500(t *testing.T) {
	app := fiber.New()
	log, _ := zap.NewDevelopment()
	svc := service.NewRGService(newFakeRepo(), service.NoopPublisher{}, log, 24, 24)
	hh := New(svc)
	app.Get("/boom", func(c *fiber.Ctx) error {
		return hh.mapError(c, errors.New("unexpected"))
	})
	code, body := doJSON(t, app, "GET", "/boom", "")
	if code != 500 || !strings.Contains(body, "RG_INTERNAL_ERROR") {
		t.Errorf("unknown error must map to 500: %d %s", code, body)
	}
}
