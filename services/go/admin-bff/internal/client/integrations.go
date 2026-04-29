// Package client contains integration clients for third-party services.
// Each client is feature-flagged via environment variables:
//   SUMSUB_APP_TOKEN + SUMSUB_SECRET_KEY        → KYC / Identity
//   COMPLYADVANTAGE_API_KEY                     → PEP / Sanctions
//   CHAINALYSIS_API_KEY                         → Crypto compliance
//   SPORTRADAR_API_KEY                          → Live scores
//   VICTORIAMETRICS_URL                         → Provider health metrics
//   CLICKHOUSE_DSN                              → Analytics queries
//
// When the env var is absent the client returns ErrNotConfigured,
// and the calling handler returns a graceful degraded response.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

var ErrNotConfigured = errors.New("integration not configured — set required env variable")

// ── Sumsub (KYC) ──────────────────────────────────────────────────────────────

type SumsubClient struct {
	appToken  string
	secretKey string
	baseURL   string
	http      *http.Client
}

func NewSumsubClient() *SumsubClient {
	return &SumsubClient{
		appToken:  os.Getenv("SUMSUB_APP_TOKEN"),
		secretKey: os.Getenv("SUMSUB_SECRET_KEY"),
		baseURL:   "https://api.sumsub.com",
		http:      &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *SumsubClient) configured() bool {
	return c.appToken != "" && c.secretKey != ""
}

// GetApplicantStatus fetches a Sumsub applicant review status by player's externalUserId.
func (c *SumsubClient) GetApplicantStatus(ctx context.Context, externalUserID string) (map[string]any, error) {
	if !c.configured() {
		return nil, ErrNotConfigured
	}
	url := fmt.Sprintf("%s/resources/applicants/-;externalUserId=%s/one", c.baseURL, externalUserID)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("X-App-Token", c.appToken)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}

// GenerateAccessToken returns a Sumsub SDK access token for a player (for viewing applicant).
func (c *SumsubClient) GenerateAccessToken(ctx context.Context, externalUserID, levelName string) (string, error) {
	if !c.configured() {
		return "", ErrNotConfigured
	}
	url := fmt.Sprintf("%s/resources/accessTokens?userId=%s&levelName=%s", c.baseURL, externalUserID, levelName)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, nil)
	req.Header.Set("X-App-Token", c.appToken)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct {
		Token string `json:"token"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Token, nil
}

// ── ComplyAdvantage (PEP / Sanctions screening) ────────────────────────────────

type ComplyAdvantageClient struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

func NewComplyAdvantageClient() *ComplyAdvantageClient {
	return &ComplyAdvantageClient{
		apiKey:  os.Getenv("COMPLYADVANTAGE_API_KEY"),
		baseURL: "https://api.complyadvantage.com/searches",
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *ComplyAdvantageClient) configured() bool { return c.apiKey != "" }

type ScreeningRequest struct {
	SearchTerm string `json:"search_term"`
	Fuzziness  float64 `json:"fuzziness"` // 0.0 = exact, 1.0 = very fuzzy
	Filters    map[string]any `json:"filters"`
}

type ScreeningResponse struct {
	Status     string         `json:"status"` // clear|pep_match|sanctions_hit|review_required
	MatchScore float64        `json:"match_score"`
	Matches    []map[string]any `json:"matches"`
	RawPayload json.RawMessage `json:"raw_payload"`
}

// SearchPEPSanctions screens a player name/ID against PEP and sanctions lists.
func (c *ComplyAdvantageClient) SearchPEPSanctions(ctx context.Context, req ScreeningRequest) (*ScreeningResponse, error) {
	if !c.configured() {
		return nil, ErrNotConfigured
	}
	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(body))
	httpReq.Header.Set("Authorization", "Token "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var result ScreeningResponse
	json.Unmarshal(raw, &result)
	result.RawPayload = raw
	return &result, nil
}

// ── Chainalysis (Crypto compliance) ───────────────────────────────────────────

type ChainalysisClient struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

func NewChainalysisClient() *ChainalysisClient {
	return &ChainalysisClient{
		apiKey:  os.Getenv("CHAINALYSIS_API_KEY"),
		baseURL: "https://api.chainalysis.com/api/kyt/v2",
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *ChainalysisClient) configured() bool { return c.apiKey != "" }

type CryptoRiskResponse struct {
	Address    string   `json:"address"`
	RiskScore  int      `json:"risk_score"`  // 0-100
	Categories []string `json:"categories"`  // e.g. "darknet_market", "mixer"
	ClusterName string  `json:"cluster_name"`
}

// CheckAddress checks a crypto address risk score.
func (c *ChainalysisClient) CheckAddress(ctx context.Context, address, asset string) (*CryptoRiskResponse, error) {
	if !c.configured() {
		return nil, ErrNotConfigured
	}
	url := fmt.Sprintf("%s/address/%s", c.baseURL, address)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Token", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result CryptoRiskResponse
	json.NewDecoder(resp.Body).Decode(&result)
	return &result, nil
}

// ── Sportradar (Live Scores) ───────────────────────────────────────────────────

type SportradarClient struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

func NewSportradarClient() *SportradarClient {
	return &SportradarClient{
		apiKey:  os.Getenv("SPORTRADAR_API_KEY"),
		baseURL: "https://api.sportradar.us/soccer/trial/v4/en",
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *SportradarClient) configured() bool { return c.apiKey != "" }

type LiveScoreData struct {
	EventID    string `json:"event_id"`
	HomeTeam   string `json:"home_team"`
	AwayTeam   string `json:"away_team"`
	ScoreHome  int    `json:"score_home"`
	ScoreAway  int    `json:"score_away"`
	Minute     int    `json:"minute"`
	Period     string `json:"period"` // 1H|2H|HT|FT
	Status     string `json:"status"` // live|not_started|finished
}

// GetLiveScore fetches the current score for an event.
func (c *SportradarClient) GetLiveScore(ctx context.Context, eventID string) (*LiveScoreData, error) {
	if !c.configured() {
		return &LiveScoreData{
			EventID: eventID,
			Status:  "not_configured",
		}, ErrNotConfigured
	}
	url := fmt.Sprintf("%s/sport_events/%s/timeline.json?api_key=%s", c.baseURL, eventID, c.apiKey)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw map[string]any
	json.NewDecoder(resp.Body).Decode(&raw)

	// Parse Sportradar response structure
	result := &LiveScoreData{EventID: eventID, Status: "live"}
	if sport := raw["sport_event_status"]; sport != nil {
		if sm, ok := sport.(map[string]any); ok {
			if hs, ok := sm["home_score"].(float64); ok { result.ScoreHome = int(hs) }
			if as, ok := sm["away_score"].(float64); ok { result.ScoreAway = int(as) }
			if min, ok := sm["clock"].(map[string]any); ok {
				if m, ok := min["played"].(float64); ok { result.Minute = int(m) }
			}
			if s, ok := sm["status"].(string); ok { result.Status = s }
		}
	}
	return result, nil
}

// ── VictoriaMetrics (Provider Health) ─────────────────────────────────────────

type VictoriaMetricsClient struct {
	baseURL string
	http    *http.Client
}

func NewVictoriaMetricsClient() *VictoriaMetricsClient {
	return &VictoriaMetricsClient{
		baseURL: os.Getenv("VICTORIAMETRICS_URL"), // e.g. http://victoria:8428
		http:    &http.Client{Timeout: 3 * time.Second},
	}
}

func (c *VictoriaMetricsClient) configured() bool { return c.baseURL != "" }

type ProviderHealthMetrics struct {
	ProviderID   string  `json:"provider_id"`
	LatencyP99Ms float64 `json:"latency_p99_ms"`
	ErrorRatePct float64 `json:"error_rate_pct"`
	SuccessRate  float64 `json:"success_rate_pct"`
	UptimePct    float64 `json:"uptime_pct"`
}

// QueryProviderHealth runs a PromQL instant query for a provider's error rate.
func (c *VictoriaMetricsClient) QueryProviderHealth(ctx context.Context, providerID string) (*ProviderHealthMetrics, error) {
	if !c.configured() {
		return &ProviderHealthMetrics{ProviderID: providerID, SuccessRate: 100}, ErrNotConfigured
	}

	query := fmt.Sprintf(`1 - rate(casino_provider_errors_total{provider="%s"}[5m]) / rate(casino_provider_requests_total{provider="%s"}[5m])`, providerID, providerID)
	url := fmt.Sprintf("%s/api/v1/query?query=%s", c.baseURL, query)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	metrics := &ProviderHealthMetrics{ProviderID: providerID}
	if data, ok := result["data"].(map[string]any); ok {
		if results, ok := data["result"].([]any); ok && len(results) > 0 {
			if r, ok := results[0].(map[string]any); ok {
				if vals, ok := r["value"].([]any); ok && len(vals) == 2 {
					if v, ok := vals[1].(string); ok {
						fmt.Sscanf(v, "%f", &metrics.SuccessRate)
						metrics.SuccessRate *= 100
					}
				}
			}
		}
	}
	return metrics, nil
}
