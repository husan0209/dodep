package pragmatic

import (
	"context"
	"crypto/md5" //nolint:gosec
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/opus-casino/casino/internal/provider"
)

// Adapter implements provider.ProviderAdapter for Pragmatic Play Seamless Wallet API v3.
type Adapter struct {
	cfg    Config
	client *Client
	log    *zap.Logger
}

// New creates a new Pragmatic Play adapter.
func New(cfg Config, log *zap.Logger) *Adapter {
	return &Adapter{
		cfg:    cfg,
		client: NewClient(cfg, log),
		log:    log,
	}
}

func (a *Adapter) Name() string { return "pragmatic" }

// BuildLaunchURL returns a signed Pragmatic Play game URL.
func (a *Adapter) BuildLaunchURL(ctx context.Context, req provider.LaunchRequest) (string, error) {
	platform := "web"
	if req.DeviceType == "mobile" {
		platform = "mobile"
	}

	gameURL, err := a.client.GetLaunchURL(ctx, LaunchURLOptions{
		Symbol:           req.GameID,
		Token:            req.Token,
		Language:         req.Language,
		Currency:         req.Currency,
		LobbyURL:         req.LobbyURL,
		ExternalPlayerID: req.PlayerID,
		Platform:         platform,
		Demo:             req.Demo,
	})
	if err != nil {
		return "", fmt.Errorf("pragmatic: build launch url: %w", err)
	}

	return gameURL, nil
}

// GetGames fetches the Pragmatic Play game catalog and converts to provider.ProviderGame.
func (a *Adapter) GetGames(ctx context.Context) ([]provider.ProviderGame, error) {
	raw, err := a.client.GetGames(ctx)
	if err != nil {
		return nil, err
	}

	games := make([]provider.ProviderGame, 0, len(raw.GameList))
	for _, g := range raw.GameList {
		var releasedAt time.Time
		if g.ReleasedDate != "" {
			releasedAt, _ = time.Parse("2006-01-02", g.ReleasedDate)
		}

		games = append(games, provider.ProviderGame{
			ExternalID:          g.GameID,
			Name:                g.GameName,
			Category:            strings.ToLower(g.TypeName),
			SubCategory:         strings.ToLower(g.SubTypeName),
			RTP:                 g.RTP,
			Volatility:          strings.ToLower(g.Volatility),
			MinBet:              g.MinBetValue,
			MaxBet:              g.MaxBetValue,
			ThumbnailURL:        g.ImageURL,
			HasFreeSpins:        g.FrbAvailable,
			HasBonusBuy:         g.BonusBuyEnabled,
			HasJackpot:          g.HasJackpot,
			SupportedCurrencies: g.Currencies,
			RestrictedCountries: g.Countries,
			IsDemoAvailable:     g.DemoAvailable,
			ReleasedAt:          releasedAt,
		})
	}

	return games, nil
}

// VerifyCallbackSignature validates the MD5 hash in a Pragmatic Play callback.
//
// Pragmatic Play passes all params + their secret key, sorted by key, then MD5'd.
// The hash arrives as a query/body param named "hash".
func (a *Adapter) VerifyCallbackSignature(body []byte, headers map[string]string) bool {
	var params map[string]interface{}
	if err := json.Unmarshal(body, &params); err != nil {
		a.log.Warn("Pragmatic: cannot parse callback for signature check", zap.Error(err))
		return false
	}

	// Extract and remove the hash param
	receivedHash, ok := params["hash"].(string)
	if !ok || receivedHash == "" {
		a.log.Warn("Pragmatic: missing hash in callback")
		return false
	}
	delete(params, "hash")

	// Sort remaining keys and concat values
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(fmt.Sprintf("%v", params[k]))
	}
	sb.WriteString(a.cfg.SecretKey)

	//nolint:gosec
	expected := fmt.Sprintf("%x", md5.Sum([]byte(sb.String())))

	if expected != strings.ToLower(receivedHash) {
		a.log.Warn("Pragmatic: signature mismatch",
			zap.String("expected", expected),
			zap.String("received", receivedHash))
		return false
	}

	return true
}

// ─── Inbound callback types ─────────────────────────────────────────────────

type pragmaticCallback struct {
	Type          string          `json:"type"`           // "balance","bet","result","refund","jackpotWin","promoWin"
	Token         string          `json:"token"`          // session token = player_id for us
	PromoCode     string          `json:"promoCode,omitempty"`
	GameID        string          `json:"gameId"`
	RoundID       string          `json:"roundId"`
	TransactionID string          `json:"transactionId"`
	RefTransID    string          `json:"referenceTransactionId,omitempty"`
	Currency      string          `json:"currency"`
	Amount        decimal.Decimal `json:"amount"`
	Timestamp     int64           `json:"timestamp"` // Unix millis
	Hash          string          `json:"hash"`
}

// ParseCallback parses the raw Pragmatic Play callback body.
func (a *Adapter) ParseCallback(body []byte) (*provider.CallbackEvent, error) {
	var cb pragmaticCallback
	if err := json.Unmarshal(body, &cb); err != nil {
		return nil, fmt.Errorf("pragmatic: parse callback: %w", err)
	}

	ts := time.Now()
	if cb.Timestamp > 0 {
		ts = time.UnixMilli(cb.Timestamp)
	}

	// Check replay window
	age := time.Since(ts)
	replayWindow := time.Duration(a.cfg.ReplayWindowSec) * time.Second
	if age > replayWindow {
		return nil, fmt.Errorf("pragmatic: callback replay detected (age=%s, window=%s)", age, replayWindow)
	}

	evType := a.mapCallbackType(cb.Type)

	return &provider.CallbackEvent{
		Type:          evType,
		TransactionID: cb.TransactionID,
		RoundID:       cb.RoundID,
		SessionID:     cb.Token,
		PlayerID:      cb.Token, // Pragmatic uses token as player identifier
		GameID:        cb.GameID,
		Currency:      cb.Currency,
		Amount:        cb.Amount,
		RefTransID:    cb.RefTransID,
		Timestamp:     ts,
		Metadata:      map[string]string{"raw_type": cb.Type},
	}, nil
}

func (a *Adapter) mapCallbackType(t string) provider.CallbackEventType {
	switch strings.ToLower(t) {
	case "balance":
		return provider.CallbackBalance
	case "bet":
		return provider.CallbackBet
	case "result":
		return provider.CallbackWin
	case "refund":
		return provider.CallbackRollback
	case "jackpotwin":
		return provider.CallbackJackpot
	case "promowin":
		return provider.CallbackWin
	default:
		return provider.CallbackBet
	}
}
