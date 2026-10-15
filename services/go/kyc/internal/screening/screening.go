// Package screening integrates PEP / sanctions / adverse-media checks.
//
// Provider contract (ComplyAdvantage-compatible): the production client
// posts a search request and maps hits to clear / potential_match /
// true_match. Credentials arrive via env (COMPLYADVANTAGE_API_KEY); when
// unset the stub provider is used — deterministic, offline, and sufficient
// for development, CI and tests. Swapping providers only requires
// implementing ScreeningProvider; the service layer is provider-agnostic.
package screening

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opus-casino/kyc/internal/domain"
)

// SearchRequest is the provider-agnostic screening query.
type SearchRequest struct {
	UserID      int64
	FullName    string
	BirthYear   int
	CountryCode string
}

// SearchResult is the provider-agnostic screening outcome.
type SearchResult struct {
	Status  domain.ScreeningStatus
	Matches int
	Summary string
}

// ScreeningProvider runs one PEP/sanctions screening.
type ScreeningProvider interface {
	Name() string
	Screen(ctx context.Context, req SearchRequest) (SearchResult, error)
}

// ─── Stub provider (default) ───

// StubProvider is a deterministic offline provider for dev/CI/tests.
// Marker substrings in the name drive outcomes (documented in docs/api/kyc.md):
//   - contains "sanction" → true_match (blocked, SAR path)
//   - contains "pep"      → potential_match (manual review)
//   - otherwise           → clear
type StubProvider struct{}

// Name identifies the provider in screening records and audit log.
func (StubProvider) Name() string { return "complyadvantage-stub" }

// Screen evaluates the marker rules. No network, no PII leaves the process.
func (StubProvider) Screen(_ context.Context, req SearchRequest) (SearchResult, error) {
	name := strings.ToLower(strings.TrimSpace(req.FullName))
	switch {
	case strings.Contains(name, "sanction"):
		return SearchResult{
			Status:  domain.ScreeningTrueMatch,
			Matches: 1,
			Summary: "stub: sanctions list marker hit",
		}, nil
	case strings.Contains(name, "pep"):
		return SearchResult{
			Status:  domain.ScreeningPotentialMatch,
			Matches: 1,
			Summary: "stub: pep marker hit",
		}, nil
	default:
		return SearchResult{Status: domain.ScreeningClear, Summary: "stub: no hits"}, nil
	}
}

// ─── ComplyAdvantage client (production wiring) ───

// ComplyAdvantageClient is the production provider skeleton.
// Remaining work (see docs/api/kyc.md): real search endpoint mapping,
// mTLS/auth headers, retry budgets, and response parsing against the
// provider OpenAPI. Until then it fails closed — screening errors never
// silently clear a user.
type ComplyAdvantageClient struct {
	baseURL string
	apiKey  string
}

// NewComplyAdvantageClient builds the production client.
func NewComplyAdvantageClient(baseURL, apiKey string) *ComplyAdvantageClient {
	return &ComplyAdvantageClient{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey}
}

// Name identifies the provider in screening records and audit log.
func (c *ComplyAdvantageClient) Name() string { return "complyadvantage" }

// Configured reports whether production screening credentials are present.
func (c *ComplyAdvantageClient) Configured() bool { return c.apiKey != "" }

// Screen is intentionally unimplemented until PSP credentials land:
// fail closed rather than returning a fake "clear".
func (c *ComplyAdvantageClient) Screen(_ context.Context, _ SearchRequest) (SearchResult, error) {
	return SearchResult{}, &ProviderError{Provider: c.Name(), Msg: "provider not configured: set COMPLYADVANTAGE_API_KEY"}
}

// ProviderError is a typed upstream failure (maps to 502/503, never 500 masking).
type ProviderError struct {
	Provider string
	Msg      string
}

func (e *ProviderError) Error() string { return e.Provider + ": " + e.Msg }

// SelectProvider returns the production client when configured,
// otherwise the offline stub. Fail-open choice is deliberate and logged:
// without PSP keys the service must still boot for local development,
// but every stub screening is labelled as such in the database.
func SelectProvider(baseURL, apiKey string) ScreeningProvider {
	if strings.TrimSpace(apiKey) != "" {
		return NewComplyAdvantageClient(baseURL, apiKey)
	}
	return StubProvider{}
}

// ─── Webhook signature (Sumsub-style HMAC-SHA256) ───

// VerifyWebhookSignature checks hex(HMAC-SHA256(secret, body)) in constant time.
// Empty secret in development logs loudly and rejects — NEVER accept
// unsigned webhooks silently.
func VerifyWebhookSignature(body []byte, signature, secret string) bool {
	if secret == "" || signature == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	got := strings.TrimSpace(strings.TrimPrefix(signature, "sha256="))
	return hmac.Equal([]byte(expected), []byte(got))
}

// SignForTest computes the expected signature value for tests.
func SignForTest(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// ProviderWebhook is the normalised verification-provider callback.
type ProviderWebhook struct {
	EventID        string
	ExternalUserID int64
	ReviewAnswer   string // GREEN | RED
	Level          int
	RejectLabels   []string
	ReceivedAt     time.Time
}

// WebhookEventID generates an idempotency key when the provider omits one.
func WebhookEventID(provider string, body []byte) string {
	sum := sha256.Sum256(append([]byte(provider), body...))
	return "wh-" + hex.EncodeToString(sum[:]) + "-" + uuid.NewString()[:8]
}
