package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/opus-casino/kyc/internal/cache"
	"github.com/opus-casino/kyc/internal/crypto"
	"github.com/opus-casino/kyc/internal/domain"
	"github.com/opus-casino/kyc/internal/screening"
	"github.com/opus-casino/kyc/internal/service"
	"github.com/opus-casino/kyc/internal/testutil"
)

func testService(repo *testutil.FakeRepo, secret string) *service.KYCService {
	cr, err := crypto.Ephemeral()
	if err != nil {
		panic(err)
	}
	return service.NewKYCService(repo, cache.NewMemory(), screening.StubProvider{}, cr,
		service.Config{WebhookSecret: secret}, zap.NewNop())
}

func testApp(svc *service.KYCService, admin bool) (*fiber.App, *Handler) {
	h := NewHandler(svc, nil)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", "42")
		c.Locals("is_admin", admin)
		return c.Next()
	})
	app.Post("/documents", h.SubmitDocument)
	app.Get("/status", h.GetStatus)
	app.Get("/documents/:id", h.GetDocument)
	app.Delete("/documents/:id", h.DeleteDocument)
	app.Get("/level", h.GetLevel)
	app.Post("/upgrade", h.RequestUpgrade)
	app.Post("/admin/documents/:id/verify", h.VerifyDocument)
	app.Post("/admin/documents/:id/reject", h.RejectDocument)
	app.Post("/webhooks/:provider", h.Webhook)
	return app, h
}

func submitViaService(t *testing.T, svc *service.KYCService, userID int64) uuid.UUID {
	t.Helper()
	future := ptrFuture()
	doc, err := svc.SubmitDocument(context.Background(), service.SubmitInput{
		UserID: userID, Type: domain.DocPassport, Side: domain.SideFront,
		FileID: "f", DocumentNumber: "N-HTTP-1",
		IssuingCountry: "DE", ExpiresAt: future,
	})
	require.NoError(t, err)
	return doc.ID
}

func TestHTTP_SubmitAndStatus(t *testing.T) {
	repo := testutil.NewFakeRepo()
	app, _ := testApp(testService(repo, "s"), false)

	req := httptest.NewRequest("POST", "/documents", bytes.NewBufferString(`{
		"document_type": "passport", "document_side": "front",
		"file_id": "file-1", "document_number": "AB1",
		"issuing_country": "DE", "expiry_date": "2030-01-01"
	}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 201, resp.StatusCode)
	var created map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	_ = resp.Body.Close()

	req = httptest.NewRequest("GET", "/status", nil)
	resp, err = app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestHTTP_SubmitValidationError(t *testing.T) {
	repo := testutil.NewFakeRepo()
	app, _ := testApp(testService(repo, "s"), false)

	req := httptest.NewRequest("POST", "/documents", bytes.NewBufferString(`{"document_type": "nope"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestHTTP_GetDocumentOwnerAnd404(t *testing.T) {
	repo := testutil.NewFakeRepo()
	svc := testService(repo, "s")
	app, _ := testApp(svc, false)
	id := submitViaService(t, svc, 42)

	resp, err := app.Test(httptest.NewRequest("GET", "/documents/"+id.String(), nil))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	_ = resp.Body.Close()

	resp, err = app.Test(httptest.NewRequest("GET", "/documents/"+uuid.NewString(), nil))
	require.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
	_ = resp.Body.Close()

	resp, err = app.Test(httptest.NewRequest("GET", "/documents/not-a-uuid", nil))
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestHTTP_AdminVerifyRequiresAdmin(t *testing.T) {
	repo := testutil.NewFakeRepo()
	svc := testService(repo, "s")
	app, _ := testApp(svc, false) // not admin
	id := submitViaService(t, svc, 42)

	req := httptest.NewRequest("POST", "/admin/documents/"+id.String()+"/verify", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 403, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestHTTP_AdminVerifyAndReject(t *testing.T) {
	repo := testutil.NewFakeRepo()
	svc := testService(repo, "s")
	app, _ := testApp(svc, true) // admin
	id := submitViaService(t, svc, 42)

	req := httptest.NewRequest("POST", "/admin/documents/"+id.String()+"/verify",
		bytes.NewBufferString(`{"full_name": "John Smith"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	_ = resp.Body.Close()

	id2 := submitViaService(t, svc, 42)
	req = httptest.NewRequest("POST", "/admin/documents/"+id2.String()+"/reject",
		bytes.NewBufferString(`{"rejection_reason": "blurry", "required_actions": ["re-upload"]}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestHTTP_UpgradeAndLevel(t *testing.T) {
	repo := testutil.NewFakeRepo()
	app, _ := testApp(testService(repo, "s"), false)

	req := httptest.NewRequest("POST", "/upgrade", bytes.NewBufferString(`{"target_level": 1}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	_ = resp.Body.Close()

	resp, err = app.Test(httptest.NewRequest("GET", "/level", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestHTTP_WebhookBadSignature(t *testing.T) {
	repo := testutil.NewFakeRepo()
	app, _ := testApp(testService(repo, "secret"), false)

	req := httptest.NewRequest("POST", "/webhooks/sumsub",
		bytes.NewBufferString(`{"external_user_id": 1, "review_answer": "GREEN"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", "bad")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode)
	_ = resp.Body.Close()
}
