package amusnet

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
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

// Config holds Amusnet (EGT Interactive) API credentials.
// Amusnet requires mutual TLS (mTLS) in addition to HMAC-SHA256 signing.
type Config struct {
	Enabled         bool
	OperatorID      string
	SecretKey       string
	APIURL          string
	// ClientCertPath and ClientKeyPath are paths to the operator's mTLS certificate.
	// Amusnet provides these during onboarding. Mount via Docker secrets.
	ClientCertPath  string
	ClientKeyPath   string
	HTTPTimeout     time.Duration
	ReplayWindowSec int
}

// DefaultConfig returns safe defaults.
func DefaultConfig() Config {
	return Config{
		APIURL:          "https://api.amusnet.com",
		HTTPTimeout:     15 * time.Second,
		ReplayWindowSec: 180,
	}
}

// Adapter implements provider.ProviderAdapter for Amusnet (EGT Interactive) REST API.
//
// Security model:
//   - mTLS: operator presents a client certificate signed by Amusnet CA
//   - Request signing: HMAC-SHA256 of the request body using the shared secret_key
//   - Header: x-operator-id carries the operator identifier
type Adapter struct {
	cfg        Config
	httpClient *http.Client
	log        *zap.Logger
}

// New creates an Amusnet adapter. Loads mTLS cert if configured.
func New(cfg Config, log *zap.Logger) (*Adapter, error) {
	transport := &http.Transport{}

	// Load mTLS client certificate if provided
	if cfg.ClientCertPath != "" && cfg.ClientKeyPath != "" {
		cert, err := tls.LoadX509KeyPair(cfg.ClientCertPath, cfg.ClientKeyPath)
		if err != nil {
			return nil, fmt.Errorf("amusnet: load client cert: %w", err)
		}
		transport.TLSClientConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
		log.Info("Amusnet: mTLS client certificate loaded")
	}

	return &Adapter{
		cfg: cfg,
		log: log,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   cfg.HTTPTimeout,
		},
	}, nil
}

func (a *Adapter) Name() string { return "amusnet" }

// BuildLaunchURL constructs the Amusnet game launch URL.
func (a *Adapter) BuildLaunchURL(ctx context.Context, req provider.LaunchRequest) (string, error) {
	type launchReq struct {
		OperatorID string `json:"operator_id"`
		PlayerID   string `json:"player_id"`
		Token      string `json:"token"`
		GameID     string `json:"game_id"`
		Currency   string `json:"currency"`
		Language   string `json:"language"`
		LobbyURL   string `json:"lobby_url"`
		Platform   string `json:"platform"`
		Demo       bool   `json:"demo"`
	}

	payload, err := json.Marshal(launchReq{
		OperatorID: a.cfg.OperatorID,
		PlayerID:   req.PlayerID,
		Token:      req.Token,
		GameID:     req.GameID,
		Currency:   req.Currency,
		Language:   req.Language,
		LobbyURL:   req.LobbyURL,
		Platform:   req.DeviceType,
		Demo:       req.Demo,
	})
	if err != nil {
		return "", fmt.Errorf("amusnet: marshal launch: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.cfg.APIURL+"/api/wallet/launch",
		strings.NewReader(string(payload)))
	if err != nil {
		return "", fmt.Errorf("amusnet: build launch req: %w", err)
	}
	a.signRequest(httpReq, payload)

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("amusnet: launch http: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		LaunchURL string `json:"launch_url"`
		Error     string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("amusnet: unmarshal launch: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("amusnet: launch error: %s", result.Error)
	}

	return result.LaunchURL, nil
}

// GetGames fetches the Amusnet game catalog.
func (a *Adapter) GetGames(ctx context.Context) ([]provider.ProviderGame, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		a.cfg.APIURL+"/api/games?operator_id="+a.cfg.OperatorID, nil)
	if err != nil {
		return nil, fmt.Errorf("amusnet: build games req: %w", err)
	}
	a.signRequest(req, nil)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("amusnet: games http: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Games []struct {
			ID          string          `json:"id"`
			Name        string          `json:"name"`
			Category    string          `json:"category"`
			RTP         float64         `json:"rtp"`
			MinBet      decimal.Decimal `json:"min_bet"`
			MaxBet      decimal.Decimal `json:"max_bet"`
			ImageURL    string          `json:"image_url"`
			Currencies  []string        `json:"currencies"`
			IsDemo      bool            `json:"is_demo"`
			HasJackpot  bool            `json:"has_jackpot"`
			HasFreeGame bool            `json:"has_free_game"`
		} `json:"games"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("amusnet: unmarshal games: %w", err)
	}

	games := make([]provider.ProviderGame, 0, len(result.Games))
	for _, g := range result.Games {
		games = append(games, provider.ProviderGame{
			ExternalID:          g.ID,
			Name:                g.Name,
			Category:            strings.ToLower(g.Category),
			RTP:                 g.RTP,
			MinBet:              g.MinBet,
			MaxBet:              g.MaxBet,
			ThumbnailURL:        g.ImageURL,
			SupportedCurrencies: g.Currencies,
			IsDemoAvailable:     g.IsDemo,
			HasJackpot:          g.HasJackpot,
			HasFreeSpins:        g.HasFreeGame,
		})
	}

	return games, nil
}

// VerifyCallbackSignature checks HMAC-SHA256 in x-signature header.
func (a *Adapter) VerifyCallbackSignature(body []byte, headers map[string]string) bool {
	sig := headers["x-signature"]
	if sig == "" {
		sig = headers["X-Signature"]
	}
	if sig == "" {
		a.log.Warn("Amusnet: missing x-signature header")
		return false
	}

	mac := hmac.New(sha256.New, []byte(a.cfg.SecretKey))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(strings.ToLower(sig))) {
		a.log.Warn("Amusnet: signature mismatch")
		return false
	}
	return true
}

// amusnetCallback maps to Amusnet REST wallet callback.
type amusnetCallback struct {
	OperatorID    string          `json:"operator_id"`
	PlayerID      string          `json:"player_id"`
	Token         string          `json:"token"`
	GameID        string          `json:"game_id"`
	RoundID       string          `json:"round_id"`
	TransactionID string          `json:"transaction_id"`
	RefTransID    string          `json:"ref_transaction_id,omitempty"`
	Type          string          `json:"type"` // "balance","bet","win","rollback"
	Amount        decimal.Decimal `json:"amount"`
	Currency      string          `json:"currency"` // Note: Amusnet uses precision=2 for EUR
	Timestamp     int64           `json:"timestamp"`
}

// ParseCallback parses an Amusnet wallet callback.
func (a *Adapter) ParseCallback(body []byte) (*provider.CallbackEvent, error) {
	var cb amusnetCallback
	if err := json.Unmarshal(body, &cb); err != nil {
		return nil, fmt.Errorf("amusnet: parse callback: %w", err)
	}

	ts := time.Now()
	if cb.Timestamp > 0 {
		ts = time.UnixMilli(cb.Timestamp)
	}

	age := time.Since(ts)
	if age > time.Duration(a.cfg.ReplayWindowSec)*time.Second {
		return nil, fmt.Errorf("amusnet: replay detected (age=%s)", age)
	}

	return &provider.CallbackEvent{
		Type:          a.mapType(cb.Type),
		TransactionID: cb.TransactionID,
		RoundID:       cb.RoundID,
		SessionID:     cb.Token,
		PlayerID:      cb.PlayerID,
		GameID:        cb.GameID,
		Currency:      cb.Currency,
		Amount:        cb.Amount,
		RefTransID:    cb.RefTransID,
		Timestamp:     ts,
		Metadata:      map[string]string{"raw_type": cb.Type},
	}, nil
}

func (a *Adapter) mapType(t string) provider.CallbackEventType {
	switch strings.ToLower(t) {
	case "balance":
		return provider.CallbackBalance
	case "bet":
		return provider.CallbackBet
	case "win":
		return provider.CallbackWin
	case "rollback":
		return provider.CallbackRollback
	default:
		return provider.CallbackBet
	}
}

// signRequest attaches operator ID and HMAC signature headers.
func (a *Adapter) signRequest(req *http.Request, body []byte) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-operator-id", a.cfg.OperatorID)

	if len(body) > 0 {
		mac := hmac.New(sha256.New, []byte(a.cfg.SecretKey))
		mac.Write(body)
		req.Header.Set("x-signature", hex.EncodeToString(mac.Sum(nil)))
	}
}
