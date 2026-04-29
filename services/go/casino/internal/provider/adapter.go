package provider

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// ProviderAdapter is the interface every game provider must implement.
// All methods must be safe for concurrent use.
type ProviderAdapter interface {
	// Name returns the provider slug (e.g. "pragmatic", "pgsoft").
	Name() string

	// BuildLaunchURL generates a signed game-launch URL for the given request.
	BuildLaunchURL(ctx context.Context, req LaunchRequest) (string, error)

	// GetGames returns the full game catalog from the provider.
	GetGames(ctx context.Context) ([]ProviderGame, error)

	// VerifyCallbackSignature returns true if the signature on the raw HTTP body is valid.
	VerifyCallbackSignature(body []byte, headers map[string]string) bool

	// ParseCallback parses a raw provider callback into a typed CallbackEvent.
	ParseCallback(body []byte) (*CallbackEvent, error)
}

// ─── Request / Response types ────────────────────────────────────────────────

// LaunchRequest carries everything needed to generate a game launch URL.
type LaunchRequest struct {
	UserID     string
	PlayerID   string // hashed/opaque ID sent to provider (never internal user_id)
	Token      string // short-lived session token
	GameID     string // provider's external game identifier
	Currency   string // ISO 4217 (e.g. "USD")
	Balance    decimal.Decimal
	Language   string // IETF BCP-47 (e.g. "en", "de")
	LobbyURL   string // URL to return to after game close
	DeviceType string // "desktop" | "mobile"
	IPAddress  string
	Demo       bool
}

// ProviderGame is the provider-agnostic game model returned by GetGames.
type ProviderGame struct {
	ExternalID          string
	Name                string
	Category            string   // "slot" | "table" | "live" | "crash"
	SubCategory         string
	Tags                []string
	RTP                 float64
	Volatility          string   // "low" | "medium" | "high"
	MinBet              decimal.Decimal
	MaxBet              decimal.Decimal
	ThumbnailURL        string
	BannerURL           string
	HasFreeSpins        bool
	HasBonusBuy         bool
	HasJackpot          bool
	HasLiveDealer       bool
	SupportedCurrencies []string
	RestrictedCountries []string
	IsDemoAvailable     bool
	ReleasedAt          time.Time
}

// CallbackEventType classifies an inbound provider callback.
type CallbackEventType string

const (
	CallbackBalance   CallbackEventType = "balance"
	CallbackBet       CallbackEventType = "bet"
	CallbackWin       CallbackEventType = "win"
	CallbackRollback  CallbackEventType = "rollback"
	CallbackFreeSpins CallbackEventType = "freespins"
	CallbackJackpot   CallbackEventType = "jackpot"
)

// CallbackEvent is the unified representation of any inbound provider callback.
type CallbackEvent struct {
	Type          CallbackEventType
	TransactionID string          // Provider's transaction ID (used for idempotency)
	RoundID       string          // Game round identifier
	SessionID     string          // Provider session ID
	PlayerID      string          // Opaque player ID (maps to our user)
	GameID        string          // Provider game ID
	Currency      string          // ISO 4217
	Amount        decimal.Decimal // Monetary amount (0 for balance queries)
	RefTransID    string          // Reference transaction for rollback/win
	Timestamp     time.Time       // When the event occurred on provider side
	Metadata      map[string]string
}

// CallbackResponse is returned to the provider after processing a callback.
type CallbackResponse struct {
	Balance       decimal.Decimal `json:"balance"`
	TransactionID string          `json:"transaction_id"`
}
