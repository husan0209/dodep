package pragmatic

import (
	"context"
	"crypto/md5" //nolint:gosec // Pragmatic Play requires MD5 for hash parameter
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

const (
	pathGetGames  = "/IntegrationService/v3/http/CasinoGameAPI/getCasinoGames/"
	pathLaunchURL = "/IntegrationService/v3/http/CasinoGameAPI/game/url/"
)

// Client handles outbound calls to Pragmatic Play Integration Service.
type Client struct {
	cfg        Config
	httpClient *http.Client
	log        *zap.Logger
}

// NewClient creates a new Pragmatic Play API client.
func NewClient(cfg Config, log *zap.Logger) *Client {
	return &Client{
		cfg: cfg,
		log: log,
		httpClient: &http.Client{
			Timeout: cfg.HTTPTimeout,
		},
	}
}

// ─── Outbound API ─────────────────────────────────────────────────────────

// GetGamesResponse is the raw Pragmatic Play game list response.
type GetGamesResponse struct {
	Error    int    `json:"error"`
	GameList []struct {
		GameID          string          `json:"gameID"`
		GameName        string          `json:"gameName"`
		TypeName        string          `json:"typeName"`
		SubTypeName     string          `json:"subTypeName"`
		Technology      string          `json:"technology"`
		Platform        string          `json:"platform"`
		Currencies      []string        `json:"currencies"`
		Countries       []string        `json:"countries"`       // Blocked countries
		ImageURL        string          `json:"image"`
		HasJackpot      bool            `json:"hasJackpot"`
		FrbAvailable    bool            `json:"frbAvailable"`    // Free rounds bonus
		BonusBuyEnabled bool            `json:"bonusBuyEnabled"`
		RTP             float64         `json:"rtp"`
		Volatility      string          `json:"volatility"`
		MinBetValue     decimal.Decimal `json:"minBetValue"`
		MaxBetValue     decimal.Decimal `json:"maxBetValue"`
		DemoAvailable   bool            `json:"demoAvailable"`
		ReleasedDate    string          `json:"releasedDate"` // "YYYY-MM-DD"
	} `json:"gameList"`
}

// GetGames fetches the full game catalog from Pragmatic Play.
func (c *Client) GetGames(ctx context.Context) (*GetGamesResponse, error) {
	params := c.buildParams(map[string]string{
		"secureLogin": c.cfg.AgentID,
	})

	endpoint := c.cfg.APIURL + pathGetGames + "?" + params.Encode()

	c.log.Debug("Pragmatic: fetching game catalog", zap.String("url", endpoint))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("pragmatic: build request: %w", err)
	}

	resp, err := c.doWithRetry(req, 3)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("pragmatic: read body: %w", err)
	}

	var result GetGamesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("pragmatic: unmarshal: %w", err)
	}

	if result.Error != 0 {
		return nil, fmt.Errorf("pragmatic: API error code %d", result.Error)
	}

	c.log.Info("Pragmatic: fetched game catalog", zap.Int("count", len(result.GameList)))
	return &result, nil
}

// GetLaunchURLResponse is the raw launch URL response.
type GetLaunchURLResponse struct {
	Error   int    `json:"error"`
	GameURL string `json:"gameURL"`
}

// GetLaunchURL generates a signed game launch URL.
func (c *Client) GetLaunchURL(ctx context.Context, opts LaunchURLOptions) (string, error) {
	params := c.buildParams(map[string]string{
		"secureLogin":  c.cfg.AgentID,
		"symbol":       opts.Symbol,
		"token":        opts.Token,
		"language":     opts.Language,
		"cur":          opts.Currency,
		"lobbyURL":     opts.LobbyURL,
		"externalPlayerId": opts.ExternalPlayerID,
		"platform":     opts.Platform, // "web" or "mobile"
	})
	if opts.Demo {
		params.Set("mode", "demo")
	}

	endpoint := c.cfg.APIURL + pathLaunchURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("pragmatic: build launch url request: %w", err)
	}

	resp, err := c.doWithRetry(req, 3)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("pragmatic: read launch url body: %w", err)
	}

	var result GetLaunchURLResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("pragmatic: unmarshal launch url: %w", err)
	}

	if result.Error != 0 {
		return "", fmt.Errorf("pragmatic: launch url error code %d", result.Error)
	}

	return result.GameURL, nil
}

// LaunchURLOptions holds parameters for GetLaunchURL.
type LaunchURLOptions struct {
	Symbol           string
	Token            string
	Language         string
	Currency         string
	LobbyURL         string
	ExternalPlayerID string
	Platform         string // "web" | "mobile"
	Demo             bool
}

// ─── Internal helpers ─────────────────────────────────────────────────────

// buildParams builds a url.Values with hash parameter (MD5 of sorted values + secret).
// Pragmatic Play signs requests by: MD5(concat(sorted_values) + secret_key)
func (c *Client) buildParams(kv map[string]string) url.Values {
	params := url.Values{}
	for k, v := range kv {
		if v != "" {
			params.Set(k, v)
		}
	}

	// Collect values of non-hash params in sorted key order for hash computation
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(params.Get(k))
	}
	sb.WriteString(c.cfg.SecretKey)

	//nolint:gosec // Pragmatic Play specifies MD5 — cannot be changed
	hash := fmt.Sprintf("%x", md5.Sum([]byte(sb.String())))
	params.Set("hash", hash)

	return params
}

// doWithRetry executes an HTTP request with exponential backoff on 5xx errors.
func (c *Client) doWithRetry(req *http.Request, maxAttempts int) (*http.Response, error) {
	var lastErr error
	backoff := 200 * time.Millisecond

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(backoff):
				backoff *= 2
			}
			// Clone request body if needed (GET has no body)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("pragmatic: http do: %w", err)
			continue
		}

		if resp.StatusCode >= 500 {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("pragmatic: server error %d", resp.StatusCode)
			c.log.Warn("Pragmatic: retrying on 5xx",
				zap.Int("status", resp.StatusCode),
				zap.Int("attempt", attempt+1))
			continue
		}

		return resp, nil
	}

	return nil, lastErr
}
