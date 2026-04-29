package pgsoft

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/opus-casino/casino/internal/provider"
)

// Config holds PG Soft credentials.
type Config struct {
	Enabled         bool
	OperatorToken   string
	SecretKey       string
	APIURL          string
	HTTPTimeout     time.Duration
	ReplayWindowSec int
}

// DefaultConfig returns safe defaults.
func DefaultConfig() Config {
	return Config{
		APIURL:          "https://api.pgsoft-games.com",
		HTTPTimeout:     10 * time.Second,
		ReplayWindowSec: 180,
	}
}

// Adapter implements provider.ProviderAdapter for PG Soft Seamless Wallet API.
//
// PG Soft uses HMAC-SHA256 for request signing. The Authorization header carries
// "Bearer <operator_token>" while the x-api-sign header carries the HMAC.
type Adapter struct {
	cfg        Config
	httpClient *http.Client
	log        *zap.Logger
}

// New creates a PG Soft adapter.
func New(cfg Config, log *zap.Logger) *Adapter {
	return &Adapter{
		cfg: cfg,
		log: log,
		httpClient: &http.Client{Timeout: cfg.HTTPTimeout},
	}
}

func (a *Adapter) Name() string { return "pgsoft" }

// BuildLaunchURL constructs the PG Soft game launch URL.
// PG Soft returns the launch URL as a redirect through their lobby endpoint.
func (a *Adapter) BuildLaunchURL(ctx context.Context, req provider.LaunchRequest) (string, error) {
	endpoint := fmt.Sprintf("%s/api/v2/Launch", a.cfg.APIURL)

	type launchReq struct {
		OperatorToken string `json:"operator_token"`
		PlayerToken   string `json:"player_token"`
		GameID        string `json:"game_id"`
		Currency      string `json:"currency"`
		Language      string `json:"language"`
		Platform      string `json:"platform"`
		LobbyURL      string `json:"lobby_url"`
	}

	payload := launchReq{
		OperatorToken: a.cfg.OperatorToken,
		PlayerToken:   req.Token,
		GameID:        req.GameID,
		Currency:      req.Currency,
		Language:      req.Language,
		Platform:      req.DeviceType,
		LobbyURL:      req.LobbyURL,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("pgsoft: marshal launch req: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint,
		strings.NewReader(string(body)))
	if err != nil {
		return "", fmt.Errorf("pgsoft: build launch request: %w", err)
	}

	a.signRequest(httpReq, body)

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("pgsoft: launch url http: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Data struct {
			LaunchURL string `json:"launch_url"`
		} `json:"data"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("pgsoft: unmarshal launch: %w", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("pgsoft: launch error %d: %s", result.Error.Code, result.Error.Message)
	}

	return result.Data.LaunchURL, nil
}

// GetGames fetches the game catalog from PG Soft.
func (a *Adapter) GetGames(ctx context.Context) ([]provider.ProviderGame, error) {
	endpoint := fmt.Sprintf("%s/api/v2/Games?operator_token=%s", a.cfg.APIURL, a.cfg.OperatorToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("pgsoft: build games request: %w", err)
	}
	a.signRequest(req, nil)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pgsoft: games http: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Data []struct {
			GameID      string   `json:"game_id"`
			Name        string   `json:"name"`
			Category    string   `json:"category"`
			RTP         float64  `json:"rtp"`
			Currencies  []string `json:"currencies"`
			ImageURL    string   `json:"image_url"`
			IsDemo      bool     `json:"is_demo"`
			HasJackpot  bool     `json:"has_jackpot"`
			HasFreeGame bool     `json:"has_free_game"`
			MinBet      decimal.Decimal `json:"min_bet"`
			MaxBet      decimal.Decimal `json:"max_bet"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("pgsoft: unmarshal games: %w", err)
	}

	games := make([]provider.ProviderGame, 0, len(result.Data))
	for _, g := range result.Data {
		games = append(games, provider.ProviderGame{
			ExternalID:          g.GameID,
			Name:                g.Name,
			Category:            strings.ToLower(g.Category),
			RTP:                 g.RTP,
			ThumbnailURL:        g.ImageURL,
			HasFreeSpins:        g.HasFreeGame,
			HasJackpot:          g.HasJackpot,
			SupportedCurrencies: g.Currencies,
			IsDemoAvailable:     g.IsDemo,
			MinBet:              g.MinBet,
			MaxBet:              g.MaxBet,
		})
	}

	return games, nil
}

// VerifyCallbackSignature checks the HMAC-SHA256 x-api-sign header.
func (a *Adapter) VerifyCallbackSignature(body []byte, headers map[string]string) bool {
	sig := headers["x-api-sign"]
	if sig == "" {
		sig = headers["X-Api-Sign"]
	}
	if sig == "" {
		a.log.Warn("PG Soft: missing x-api-sign header")
		return false
	}

	mac := hmac.New(sha256.New, []byte(a.cfg.SecretKey))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(strings.ToLower(sig))) {
		a.log.Warn("PG Soft: signature mismatch")
		return false
	}
	return true
}

// pgsoftCallback is the inbound wallet API callback from PG Soft.
type pgsoftCallback struct {
	OperatorToken   string          `json:"operator_token"`
	PlayerToken     string          `json:"player_token"`
	GameID          string          `json:"game_id"`
	BetID           string          `json:"bet_id"`
	RoundID         string          `json:"round_id"`
	TransactionID   string          `json:"transaction_id"`
	RefTransactionID string         `json:"ref_transaction_id,omitempty"`
	Type            string          `json:"type"` // "BET","WIN","CANCEL","FREE_ROUNDS_ACCEPTED","FREE_ROUNDS_CANCEL"
	Amount          decimal.Decimal `json:"amount"`
	Currency        string          `json:"currency"`
	Timestamp       int64           `json:"timestamp"`
}

// ParseCallback parses a PG Soft wallet callback.
func (a *Adapter) ParseCallback(body []byte) (*provider.CallbackEvent, error) {
	var cb pgsoftCallback
	if err := json.Unmarshal(body, &cb); err != nil {
		return nil, fmt.Errorf("pgsoft: parse callback: %w", err)
	}

	ts := time.Now()
	if cb.Timestamp > 0 {
		ts = time.UnixMilli(cb.Timestamp)
	}

	age := time.Since(ts)
	if age > time.Duration(a.cfg.ReplayWindowSec)*time.Second {
		return nil, fmt.Errorf("pgsoft: replay detected (age=%s)", age)
	}

	return &provider.CallbackEvent{
		Type:          a.mapType(cb.Type),
		TransactionID: cb.TransactionID,
		RoundID:       cb.RoundID,
		SessionID:     cb.PlayerToken,
		PlayerID:      cb.PlayerToken,
		GameID:        cb.GameID,
		Currency:      cb.Currency,
		Amount:        cb.Amount,
		RefTransID:    cb.RefTransactionID,
		Timestamp:     ts,
		Metadata:      map[string]string{"bet_id": cb.BetID, "raw_type": cb.Type},
	}, nil
}

func (a *Adapter) mapType(t string) provider.CallbackEventType {
	switch strings.ToUpper(t) {
	case "BET":
		return provider.CallbackBet
	case "WIN":
		return provider.CallbackWin
	case "CANCEL":
		return provider.CallbackRollback
	case "FREE_ROUNDS_ACCEPTED", "FREE_ROUNDS_WIN":
		return provider.CallbackFreeSpins
	default:
		return provider.CallbackBet
	}
}

// signRequest adds Authorization and x-api-sign headers.
func (a *Adapter) signRequest(req *http.Request, body []byte) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.cfg.OperatorToken)

	if len(body) > 0 {
		mac := hmac.New(sha256.New, []byte(a.cfg.SecretKey))
		mac.Write(body)
		req.Header.Set("x-api-sign", hex.EncodeToString(mac.Sum(nil)))
	}
}
