package amatic

import (
	"context"
	"crypto/md5" //nolint:gosec // Amatic specifies MD5
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/opus-casino/casino/internal/provider"
)

// Config holds Amatic Industries Cashier API credentials.
type Config struct {
	Enabled         bool
	OperatorID      string
	APIPassword     string
	SecretKey       string // Used for callback signature (MD5)
	APIURL          string
	HTTPTimeout     time.Duration
	ReplayWindowSec int
}

// DefaultConfig returns safe defaults.
func DefaultConfig() Config {
	return Config{
		APIURL:          "https://api.amatic-industries.com",
		HTTPTimeout:     10 * time.Second,
		ReplayWindowSec: 180,
	}
}

// Adapter implements provider.ProviderAdapter for Amatic Industries Cashier API v2.
//
// Amatic uses synchronous request/response only (no async settlement).
// Signature: MD5(api_password + operatorId + methodSpecificParams).
type Adapter struct {
	cfg Config
	log *zap.Logger
}

// New creates an Amatic adapter.
func New(cfg Config, log *zap.Logger) *Adapter {
	return &Adapter{cfg: cfg, log: log}
}

func (a *Adapter) Name() string { return "amatic" }

// BuildLaunchURL constructs the Amatic game launch URL.
// Amatic uses a redirect-based launch with a signed token.
func (a *Adapter) BuildLaunchURL(_ context.Context, req provider.LaunchRequest) (string, error) {
	token := a.sign(req.PlayerID + req.Token + req.GameID)

	u := fmt.Sprintf("%s/lobby?operatorId=%s&playerId=%s&sessionToken=%s&gameId=%s&lang=%s&currency=%s&lobbyUrl=%s",
		a.cfg.APIURL,
		a.cfg.OperatorID,
		req.PlayerID,
		req.Token,
		req.GameID,
		req.Language,
		req.Currency,
		req.LobbyURL,
	)

	if req.Demo {
		u += "&mode=demo"
	}

	u += "&hash=" + token

	return u, nil
}

// GetGames returns an empty slice — Amatic delivers game catalogs via static list or email.
// The catalog should be maintained manually in the DB for Amatic.
func (a *Adapter) GetGames(_ context.Context) ([]provider.ProviderGame, error) {
	a.log.Warn("Amatic: GetGames not available via API — use static catalog import")
	return nil, nil
}

// VerifyCallbackSignature validates Amatic callback by MD5 checksum.
// Amatic includes a "key" field in the callback body = MD5(api_password + sorted_params).
func (a *Adapter) VerifyCallbackSignature(body []byte, _ map[string]string) bool {
	var params map[string]interface{}
	if err := json.Unmarshal(body, &params); err != nil {
		a.log.Warn("Amatic: cannot parse callback", zap.Error(err))
		return false
	}

	receivedKey, _ := params["key"].(string)
	if receivedKey == "" {
		a.log.Warn("Amatic: missing key in callback")
		return false
	}

	// Build the signature string per Amatic spec
	// MD5(api_password + operatorId + playerId + transactionId)
	playerID, _ := params["playerId"].(string)
	txID, _ := params["transactionId"].(string)

	input := a.cfg.APIPassword + a.cfg.OperatorID + playerID + txID
	//nolint:gosec
	expected := fmt.Sprintf("%x", md5.Sum([]byte(input)))

	if !strings.EqualFold(expected, receivedKey) {
		a.log.Warn("Amatic: key mismatch")
		return false
	}
	return true
}

// amaticCallback maps to Amatic Cashier API callback fields.
type amaticCallback struct {
	Method        string          `json:"method"`        // "getBalance","withdraw","deposit","cancel"
	OperatorID    string          `json:"operatorId"`
	PlayerID      string          `json:"playerId"`
	SessionToken  string          `json:"sessionToken"`
	GameID        string          `json:"gameId"`
	RoundID       string          `json:"roundId"`
	TransactionID string          `json:"transactionId"`
	RefTransID    string          `json:"referenceTransactionId,omitempty"`
	Amount        decimal.Decimal `json:"amount"`
	Currency      string          `json:"currency"`
	Timestamp     int64           `json:"timestamp"`
	Key           string          `json:"key"`
}

// ParseCallback parses an Amatic Cashier API callback.
func (a *Adapter) ParseCallback(body []byte) (*provider.CallbackEvent, error) {
	var cb amaticCallback
	if err := json.Unmarshal(body, &cb); err != nil {
		return nil, fmt.Errorf("amatic: parse callback: %w", err)
	}

	ts := time.Now()
	if cb.Timestamp > 0 {
		ts = time.UnixMilli(cb.Timestamp)
	}

	age := time.Since(ts)
	if age > time.Duration(a.cfg.ReplayWindowSec)*time.Second {
		return nil, fmt.Errorf("amatic: replay detected (age=%s)", age)
	}

	return &provider.CallbackEvent{
		Type:          a.mapMethod(cb.Method),
		TransactionID: cb.TransactionID,
		RoundID:       cb.RoundID,
		SessionID:     cb.SessionToken,
		PlayerID:      cb.PlayerID,
		GameID:        cb.GameID,
		Currency:      cb.Currency,
		Amount:        cb.Amount,
		RefTransID:    cb.RefTransID,
		Timestamp:     ts,
		Metadata:      map[string]string{"method": cb.Method},
	}, nil
}

func (a *Adapter) mapMethod(m string) provider.CallbackEventType {
	switch strings.ToLower(m) {
	case "getbalance":
		return provider.CallbackBalance
	case "withdraw":
		return provider.CallbackBet
	case "deposit":
		return provider.CallbackWin
	case "cancel":
		return provider.CallbackRollback
	default:
		return provider.CallbackBet
	}
}

// sign generates MD5(api_password + data).
func (a *Adapter) sign(data string) string {
	//nolint:gosec
	return fmt.Sprintf("%x", md5.Sum([]byte(a.cfg.APIPassword+data)))
}
