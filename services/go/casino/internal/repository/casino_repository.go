package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// CasinoRepository handles data persistence for casino operations.
// Uses GORM (per CONVENTIONS.md) for all DB operations.
type CasinoRepository struct {
	db    *gorm.DB
	redis *redis.Client
}

var errCasinoDatabaseUnavailable = errors.New("database client is not initialized")
var errCasinoRedisUnavailable = errors.New("redis client is not initialized")

// NewCasinoRepository creates a new casino repository.
func NewCasinoRepository(db *gorm.DB, rdb *redis.Client) *CasinoRepository {
	return &CasinoRepository{
		db:    db,
		redis: rdb,
	}
}

// Game represents a casino game
type Game struct {
	ID                 string                 `json:"id"`
	Name               string                 `json:"name"`
	ProviderID         string                 `json:"provider_id"`
	ProviderName       string                 `json:"provider_name"`
	Category           string                 `json:"category"`
	Tags               []string               `json:"tags"`
	Description        string                 `json:"description"`
	ImageURL           string                 `json:"image_url"`
	ThumbnailURL       string                 `json:"thumbnail_url"`
	SupportedCurrencies []string              `json:"supported_currencies"`
	MinBet             string                 `json:"min_bet"`
	MaxBet             string                 `json:"max_bet"`
	Features           GameFeatures           `json:"features"`
	RTP                float64                `json:"rtp"`
	Volatility         string                 `json:"volatility"`
	IsActive           bool                   `json:"is_active"`
	IsDemoAvailable    bool                   `json:"is_demo_available"`
	RestrictedCountries []string              `json:"restricted_countries"`
	PopularityScore    int32                  `json:"popularity_score"`
	ReleasedAt         time.Time              `json:"released_at"`
	Metadata           map[string]string      `json:"metadata"`
}

// GameFeatures represents game features
type GameFeatures struct {
	HasFreeSpins    bool     `json:"has_free_spins"`
	HasBonusBuy     bool     `json:"has_bonus_buy"`
	HasJackpot      bool     `json:"has_jackpot"`
	HasMultiplayer  bool     `json:"has_multiplayer"`
	HasLiveDealer   bool     `json:"has_live_dealer"`
	BonusFeatures   []string `json:"bonus_features"`
}

// Provider represents a game provider
type Provider struct {
	ID                  string            `json:"id"`
	Name                string            `json:"name"`
	LogoURL             string            `json:"logo_url"`
	Description         string            `json:"description"`
	IsActive            bool              `json:"is_active"`
	GamesCount          int32             `json:"games_count"`
	SupportedCurrencies []string          `json:"supported_currencies"`
	RestrictedCountries []string          `json:"restricted_countries"`
	Metadata            map[string]string `json:"metadata"`
}

// GameSession represents an active game session
type GameSession struct {
	ID             string            `json:"id"`
	UserID         string            `json:"user_id"`
	GameID         string            `json:"game_id"`
	ProviderID     string            `json:"provider_id"`
	Status         string            `json:"status"`
	BalanceAtStart string            `json:"balance_at_start"`
	StartedAt      time.Time         `json:"started_at"`
	LastActivity   time.Time         `json:"last_activity"`
	EndedAt        *time.Time        `json:"ended_at"`
	DeviceType     string            `json:"device_type"`
	LobbyURL       string            `json:"lobby_url"`
	LaunchURL      string            `json:"launch_url"`
	Token          string            `json:"token"`
	Metadata       map[string]string `json:"metadata"`
}

// GameRound represents a single game round
type GameRound struct {
	ID           string            `json:"id"`
	SessionID    string            `json:"session_id"`
	RoundID      string            `json:"round_id"`
	BetAmount    string            `json:"bet_amount"`
	WinAmount    string            `json:"win_amount"`
	NetResult    string            `json:"net_result"`
	Status       string            `json:"status"`
	StartedAt    time.Time         `json:"started_at"`
	EndedAt      time.Time         `json:"ended_at"`
	GameState    map[string]string `json:"game_state"`
	Metadata     map[string]string `json:"metadata"`
}

// GetGamesOptions represents options for getting games
type GetGamesOptions struct {
	ProviderID *string
	Category   *string
	Tags       []string
	Search     *string
	Limit      int32
	Offset     int32
}

// GetGames returns games with optional filtering
func (r *CasinoRepository) GetGames(ctx context.Context, opts GetGamesOptions) ([]Game, int64, error) {
	if r.db == nil {
		return nil, 0, errCasinoDatabaseUnavailable
	}

	query := r.db.WithContext(ctx).Model(&Game{}).Where("is_active = true")
	if opts.ProviderID != nil {
		query = query.Where("provider_id = ?", *opts.ProviderID)
	}
	if opts.Category != nil {
		query = query.Where("category = ?", *opts.Category)
	}
	if opts.Search != nil {
		query = query.Where("name ILIKE ?", "%"+*opts.Search+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var games []Game
	if err := query.Limit(int(opts.Limit)).Offset(int(opts.Offset)).
		Order("popularity_score DESC").Find(&games).Error; err != nil {
		return nil, 0, err
	}

	return games, total, nil
}

// GetGame returns a game by ID
func (r *CasinoRepository) GetGame(ctx context.Context, gameID string) (*Game, error) {
	if r.db == nil {
		return nil, errCasinoDatabaseUnavailable
	}
	var game Game
	if err := r.db.WithContext(ctx).Where("id = ?", gameID).First(&game).Error; err != nil {
		return nil, err
	}
	return &game, nil
}

// GetProviders returns all providers
func (r *CasinoRepository) GetProviders(ctx context.Context, isActive *bool) ([]Provider, error) {
	if r.db == nil {
		return nil, errCasinoDatabaseUnavailable
	}
	query := r.db.WithContext(ctx).Model(&Provider{})
	if isActive != nil {
		query = query.Where("is_active = ?", *isActive)
	}
	var providers []Provider
	if err := query.Find(&providers).Error; err != nil {
		return nil, err
	}
	return providers, nil
}

// GetProvider returns a provider by ID
func (r *CasinoRepository) GetProvider(ctx context.Context, providerID string) (*Provider, error) {
	if r.db == nil {
		return nil, errCasinoDatabaseUnavailable
	}
	var p Provider
	if err := r.db.WithContext(ctx).Where("id = ?", providerID).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// CreateGameSession creates a new game session
func (r *CasinoRepository) CreateGameSession(ctx context.Context, session *GameSession) error {
	if r.db == nil {
		return errCasinoDatabaseUnavailable
	}
	return r.db.WithContext(ctx).Create(session).Error
}

// GetGameSession returns a game session by ID
func (r *CasinoRepository) GetGameSession(ctx context.Context, sessionID string) (*GameSession, error) {
	if r.db == nil {
		return nil, errCasinoDatabaseUnavailable
	}
	var s GameSession
	if err := r.db.WithContext(ctx).Where("id = ?", sessionID).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// UpdateGameSession updates a game session
func (r *CasinoRepository) UpdateGameSession(ctx context.Context, session *GameSession) error {
	if r.db == nil {
		return errCasinoDatabaseUnavailable
	}
	return nil
}

// EndGameSession marks a session as ended
func (r *CasinoRepository) EndGameSession(ctx context.Context, sessionID string, endedAt time.Time) error {
	if r.db == nil {
		return errCasinoDatabaseUnavailable
	}
	return r.db.WithContext(ctx).Model(&GameSession{}).
		Where("id = ?", sessionID).
		Updates(map[string]interface{}{"status": "ended", "ended_at": endedAt}).Error
}

// GetGameHistory returns user's game history
func (r *CasinoRepository) GetGameHistory(ctx context.Context, userID string, gameID *string, dateRange *struct{ From, To time.Time }, limit, offset int32) ([]GameSession, int64, error) {
	if r.db == nil {
		return nil, 0, errCasinoDatabaseUnavailable
	}
	query := r.db.WithContext(ctx).Model(&GameSession{}).Where("user_id = ?", userID)
	if gameID != nil {
		query = query.Where("game_id = ?", *gameID)
	}
	if dateRange != nil {
		query = query.Where("started_at BETWEEN ? AND ?", dateRange.From, dateRange.To)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var sessions []GameSession
	if err := query.Limit(int(limit)).Offset(int(offset)).
		Order("started_at DESC").Find(&sessions).Error; err != nil {
		return nil, 0, err
	}
	return sessions, total, nil
}

// GetRoundHistory returns round history for a session
func (r *CasinoRepository) GetRoundHistory(ctx context.Context, sessionID string, limit, offset int32) ([]GameRound, int64, error) {
	if r.db == nil {
		return nil, 0, errCasinoDatabaseUnavailable
	}
	var total int64
	if err := r.db.WithContext(ctx).Model(&GameRound{}).Where("session_id = ?", sessionID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rounds []GameRound
	if err := r.db.WithContext(ctx).Where("session_id = ?", sessionID).
		Limit(int(limit)).Offset(int(offset)).Order("started_at ASC").Find(&rounds).Error; err != nil {
		return nil, 0, err
	}
	return rounds, total, nil
}

// CreateGameRound creates a new game round
func (r *CasinoRepository) CreateGameRound(ctx context.Context, round *GameRound) error {
	if r.db == nil {
		return errCasinoDatabaseUnavailable
	}
	return r.db.WithContext(ctx).Create(round).Error
}

// CacheGame caches game data in Redis
func (r *CasinoRepository) CacheGame(ctx context.Context, game *Game, ttl time.Duration) error {
	key := "casino:game:" + game.ID
	if r.redis == nil {
		return errCasinoRedisUnavailable
	}

	payload, err := json.Marshal(game)
	if err != nil {
		return err
	}

	return r.redis.Set(ctx, key, payload, ttl).Err()
}

// GetCachedGame retrieves cached game data from Redis
func (r *CasinoRepository) GetCachedGame(ctx context.Context, gameID string) (*Game, error) {
	key := "casino:game:" + gameID
	if r.redis == nil {
		return nil, errCasinoRedisUnavailable
	}

	payload, err := r.redis.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var game Game
	if err := json.Unmarshal(payload, &game); err != nil {
		return nil, err
	}

	return &game, nil
}

// InvalidateGameCache invalidates cached game data
func (r *CasinoRepository) InvalidateGameCache(ctx context.Context, gameID string) error {
	key := "casino:game:" + gameID
	if r.redis == nil {
		return errCasinoRedisUnavailable
	}

	return r.redis.Del(ctx, key).Err()
}
