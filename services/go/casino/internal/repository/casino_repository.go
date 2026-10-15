package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CasinoRepository handles data persistence for casino operations.
// Uses GORM (per CONVENTIONS.md) for all DB operations.
//
// Expected PostgreSQL schema: libs/migrations/postgresql/011_casino_tables.sql
// (tables: game_providers, games, game_sessions, game_rounds).
// Money columns are NUMERIC(18,8); amounts travel as decimal strings
// (NEVER float64, per CONVENTIONS.md NEVER-6).
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

// Game represents a casino game (table: games).
type Game struct {
	ID                  string            `json:"id" gorm:"column:id;primaryKey;type:text"`
	ExternalID          string            `json:"external_id" gorm:"column:external_id;type:text;not null"`
	Name                string            `json:"name" gorm:"column:name;type:text;not null"`
	ProviderID          string            `json:"provider_id" gorm:"column:provider_id;type:text;not null;index"`
	ProviderName        string            `json:"provider_name" gorm:"column:provider_name;type:text;not null"`
	Category            string            `json:"category" gorm:"column:category;type:text;not null;default:slot;index"`
	Tags                []string          `json:"tags" gorm:"column:tags;type:text[];not null"`
	Description         string            `json:"description" gorm:"column:description;type:text"`
	ImageURL            string            `json:"image_url" gorm:"column:image_url;type:text"`
	ThumbnailURL        string            `json:"thumbnail_url" gorm:"column:thumbnail_url;type:text"`
	SupportedCurrencies []string          `json:"supported_currencies" gorm:"column:supported_currencies;type:text[];not null"`
	MinBet              string            `json:"min_bet" gorm:"column:min_bet;type:numeric(18,8);not null"`
	MaxBet              string            `json:"max_bet" gorm:"column:max_bet;type:numeric(18,8);not null"`
	Features            GameFeatures      `json:"features" gorm:"column:features;type:jsonb;serializer:json;not null"`
	RTP                 float64           `json:"rtp" gorm:"column:rtp;type:float;not null;default:0"`
	Volatility          string            `json:"volatility" gorm:"column:volatility;type:text"`
	IsActive            bool              `json:"is_active" gorm:"column:is_active;not null;default:true;index"`
	IsDemoAvailable     bool              `json:"is_demo_available" gorm:"column:is_demo_available;not null;default:false"`
	RestrictedCountries []string          `json:"restricted_countries" gorm:"column:restricted_countries;type:text[];not null"`
	PopularityScore     int32             `json:"popularity_score" gorm:"column:popularity_score;not null;default:0"`
	ReleasedAt          time.Time         `json:"released_at" gorm:"column:released_at;type:timestamptz"`
	Metadata            map[string]string `json:"metadata" gorm:"column:metadata;type:jsonb;serializer:json;not null"`
	CreatedAt           time.Time         `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt           time.Time         `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

// TableName maps Game to the games table (migration 011).
func (Game) TableName() string { return "games" }

// GameFeatures represents game features (stored as JSONB in games.features).
type GameFeatures struct {
	HasFreeSpins   bool     `json:"has_free_spins"`
	HasBonusBuy    bool     `json:"has_bonus_buy"`
	HasJackpot     bool     `json:"has_jackpot"`
	HasMultiplayer bool     `json:"has_multiplayer"`
	HasLiveDealer  bool     `json:"has_live_dealer"`
	BonusFeatures  []string `json:"bonus_features"`
}

// Provider represents a game provider (table: game_providers).
type Provider struct {
	ID                  string            `json:"id" gorm:"column:id;primaryKey;type:text"`
	Name                string            `json:"name" gorm:"column:name;type:text;not null"`
	Slug                string            `json:"slug" gorm:"column:slug;type:text;not null;uniqueIndex"`
	LogoURL             string            `json:"logo_url" gorm:"column:logo_url;type:text"`
	Description         string            `json:"description" gorm:"column:description;type:text"`
	IsActive            bool              `json:"is_active" gorm:"column:is_active;not null;default:false"`
	GamesCount          int32             `json:"games_count" gorm:"column:games_count;not null;default:0"`
	SupportedCurrencies []string          `json:"supported_currencies" gorm:"column:supported_currencies;type:text[];not null"`
	RestrictedCountries []string          `json:"restricted_countries" gorm:"column:restricted_countries;type:text[];not null"`
	Metadata            map[string]string `json:"metadata" gorm:"column:metadata;type:jsonb;serializer:json;not null"`
	CreatedAt           time.Time         `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt           time.Time         `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

// TableName maps Provider to the game_providers table (migration 011).
func (Provider) TableName() string { return "game_providers" }

// GameSession represents an active game session (table: game_sessions).
type GameSession struct {
	ID             string            `json:"id" gorm:"column:id;primaryKey;type:text"`
	UserID         string            `json:"user_id" gorm:"column:user_id;type:text;not null;index"`
	GameID         string            `json:"game_id" gorm:"column:game_id;type:text;not null;index"`
	ProviderID     string            `json:"provider_id" gorm:"column:provider_id;type:text;not null"`
	Status         string            `json:"status" gorm:"column:status;type:text;not null;default:active;index"`
	BalanceAtStart string            `json:"balance_at_start" gorm:"column:balance_at_start;type:numeric(18,8);not null"`
	StartedAt      time.Time         `json:"started_at" gorm:"column:started_at;type:timestamptz;not null"`
	LastActivity   time.Time         `json:"last_activity" gorm:"column:last_activity;type:timestamptz;not null"`
	EndedAt        *time.Time        `json:"ended_at" gorm:"column:ended_at;type:timestamptz"`
	DeviceType     string            `json:"device_type" gorm:"column:device_type;type:text"`
	LobbyURL       string            `json:"lobby_url" gorm:"column:lobby_url;type:text"`
	LaunchURL      string            `json:"launch_url" gorm:"column:launch_url;type:text;not null"`
	Token          string            `json:"token" gorm:"column:token;type:text;not null;uniqueIndex"`
	Metadata       map[string]string `json:"metadata" gorm:"column:metadata;type:jsonb;serializer:json;not null"`
}

// TableName maps GameSession to the game_sessions table (migration 011).
func (GameSession) TableName() string { return "game_sessions" }

// GameRound represents a single game round (table: game_rounds).
type GameRound struct {
	ID           string            `json:"id" gorm:"column:id;primaryKey;type:text"`
	SessionID    string            `json:"session_id" gorm:"column:session_id;type:text;not null;index"`
	RoundID      string            `json:"round_id" gorm:"column:round_id;type:text;not null"`
	BetAmount    string            `json:"bet_amount" gorm:"column:bet_amount;type:numeric(18,8);not null"`
	WinAmount    string            `json:"win_amount" gorm:"column:win_amount;type:numeric(18,8);not null"`
	NetResult    string            `json:"net_result" gorm:"column:net_result;type:numeric(18,8);not null"`
	Status       string            `json:"status" gorm:"column:status;type:text;not null;default:completed"`
	StartedAt    time.Time         `json:"started_at" gorm:"column:started_at;type:timestamptz;not null"`
	EndedAt      time.Time         `json:"ended_at" gorm:"column:ended_at;type:timestamptz;not null"`
	GameState    map[string]string `json:"game_state" gorm:"column:game_state;type:jsonb;serializer:json;not null"`
	Metadata     map[string]string `json:"metadata" gorm:"column:metadata;type:jsonb;serializer:json;not null"`
}

// TableName maps GameRound to the game_rounds table (migration 011).
func (GameRound) TableName() string { return "game_rounds" }

// GetGamesOptions represents options for getting games
type GetGamesOptions struct {
	ProviderID *string
	Category   *string
	Tags       []string
	Search     *string
	Limit      int32
	Offset     int32
}

// normalizePage applies default/max page sizes and clamps negative offsets.
func normalizePage(limit, offset int32) (int, int) {
	const defaultLimit = 20
	const maxLimit = 100
	l := int(limit)
	switch {
	case l <= 0:
		l = defaultLimit
	case l > maxLimit:
		l = maxLimit
	}
	o := int(offset)
	if o < 0 {
		o = 0
	}
	return l, o
}

// normalizeDecimalAmount validates a money string with shopspring/decimal
// (NEVER float64) and normalizes empty input to "0".
func normalizeDecimalAmount(raw string) (string, error) {
	if raw == "" {
		return "0", nil
	}
	d, err := decimal.NewFromString(raw)
	if err != nil {
		return "", fmt.Errorf("invalid decimal amount %q: %w", raw, err)
	}
	return d.String(), nil
}

// slugify derives a URL-safe provider slug from an arbitrary string.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	return s
}

// GetGames returns games with optional filtering (provider, category, tags,
// full-text search on name) and offset pagination.
func (r *CasinoRepository) GetGames(ctx context.Context, opts GetGamesOptions) ([]Game, int64, error) {
	if r.db == nil {
		return nil, 0, errCasinoDatabaseUnavailable
	}
	limit, offset := normalizePage(opts.Limit, opts.Offset)

	query := r.db.WithContext(ctx).Model(&Game{}).Where("is_active = ?", true)
	if opts.ProviderID != nil {
		query = query.Where("provider_id = ?", *opts.ProviderID)
	}
	if opts.Category != nil {
		query = query.Where("category = ?", *opts.Category)
	}
	if len(opts.Tags) > 0 {
		// TEXT[] overlap: game must have at least one of the requested tags.
		query = query.Where("tags && ?", opts.Tags)
	}
	if opts.Search != nil {
		query = query.Where("name ILIKE ?", "%"+*opts.Search+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count games: %w", err)
	}

	var games []Game
	if err := query.Limit(limit).Offset(offset).
		Order("popularity_score DESC").Find(&games).Error; err != nil {
		return nil, 0, fmt.Errorf("list games: %w", err)
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
		return nil, fmt.Errorf("get game %q: %w", gameID, err)
	}
	return &game, nil
}

// UpsertGame inserts a game or updates it when the id already exists.
// Used by the catalog sync from the aggregator (daily full sync + webhooks).
func (r *CasinoRepository) UpsertGame(ctx context.Context, game *Game) error {
	if r.db == nil {
		return errCasinoDatabaseUnavailable
	}
	if game == nil || game.ID == "" {
		return errors.New("game id is required")
	}
	if game.ExternalID == "" {
		game.ExternalID = game.ID
	}
	var err error
	if game.MinBet, err = normalizeDecimalAmount(game.MinBet); err != nil {
		return err
	}
	if game.MaxBet, err = normalizeDecimalAmount(game.MaxBet); err != nil {
		return err
	}
	if game.Tags == nil {
		game.Tags = []string{}
	}
	if game.SupportedCurrencies == nil {
		game.SupportedCurrencies = []string{}
	}
	if game.RestrictedCountries == nil {
		game.RestrictedCountries = []string{}
	}
	if game.Metadata == nil {
		game.Metadata = map[string]string{}
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(game).Error
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
	if err := query.Order("name ASC").Find(&providers).Error; err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
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
		return nil, fmt.Errorf("get provider %q: %w", providerID, err)
	}
	return &p, nil
}

// UpsertProvider inserts a provider or updates it when the id already exists.
func (r *CasinoRepository) UpsertProvider(ctx context.Context, p *Provider) error {
	if r.db == nil {
		return errCasinoDatabaseUnavailable
	}
	if p == nil || p.ID == "" {
		return errors.New("provider id is required")
	}
	if p.Slug == "" {
		base := p.Name
		if base == "" {
			base = p.ID
		}
		p.Slug = slugify(base)
	}
	if p.SupportedCurrencies == nil {
		p.SupportedCurrencies = []string{}
	}
	if p.RestrictedCountries == nil {
		p.RestrictedCountries = []string{}
	}
	if p.Metadata == nil {
		p.Metadata = map[string]string{}
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(p).Error
}

// CreateGameSession creates a new game session
func (r *CasinoRepository) CreateGameSession(ctx context.Context, session *GameSession) error {
	if r.db == nil {
		return errCasinoDatabaseUnavailable
	}
	if session == nil || session.ID == "" {
		return errors.New("session id is required")
	}
	var err error
	if session.BalanceAtStart, err = normalizeDecimalAmount(session.BalanceAtStart); err != nil {
		return fmt.Errorf("invalid balance_at_start: %w", err)
	}
	if session.Metadata == nil {
		session.Metadata = map[string]string{}
	}
	if err := r.db.WithContext(ctx).Create(session).Error; err != nil {
		return fmt.Errorf("create game session: %w", err)
	}
	return nil
}

// GetGameSession returns a game session by ID
func (r *CasinoRepository) GetGameSession(ctx context.Context, sessionID string) (*GameSession, error) {
	if r.db == nil {
		return nil, errCasinoDatabaseUnavailable
	}
	var s GameSession
	if err := r.db.WithContext(ctx).Where("id = ?", sessionID).First(&s).Error; err != nil {
		return nil, fmt.Errorf("get game session %q: %w", sessionID, err)
	}
	return &s, nil
}

// UpdateGameSession persists mutable session fields (status, last activity,
// end timestamp). It updates only explicit columns, never the whole row.
func (r *CasinoRepository) UpdateGameSession(ctx context.Context, session *GameSession) error {
	if r.db == nil {
		return errCasinoDatabaseUnavailable
	}
	if session == nil || session.ID == "" {
		return errors.New("session id is required")
	}
	updates := map[string]interface{}{
		"status":        session.Status,
		"last_activity": session.LastActivity,
	}
	if session.EndedAt != nil {
		updates["ended_at"] = session.EndedAt
	}
	res := r.db.WithContext(ctx).Model(&GameSession{}).
		Where("id = ?", session.ID).
		Updates(updates)
	if res.Error != nil {
		return fmt.Errorf("update game session %q: %w", session.ID, res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("game session %q not found: %w", session.ID, gorm.ErrRecordNotFound)
	}
	return nil
}

// EndGameSession marks a session as ended
func (r *CasinoRepository) EndGameSession(ctx context.Context, sessionID string, endedAt time.Time) error {
	if r.db == nil {
		return errCasinoDatabaseUnavailable
	}
	res := r.db.WithContext(ctx).Model(&GameSession{}).
		Where("id = ?", sessionID).
		Updates(map[string]interface{}{"status": "ended", "ended_at": endedAt})
	if res.Error != nil {
		return fmt.Errorf("end game session %q: %w", sessionID, res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("game session %q not found: %w", sessionID, gorm.ErrRecordNotFound)
	}
	return nil
}

// GetSessionsByUser returns a user's game sessions (newest first) with pagination.
func (r *CasinoRepository) GetSessionsByUser(ctx context.Context, userID string, limit, offset int32) ([]GameSession, int64, error) {
	if r.db == nil {
		return nil, 0, errCasinoDatabaseUnavailable
	}
	l, o := normalizePage(limit, offset)
	query := r.db.WithContext(ctx).Model(&GameSession{}).Where("user_id = ?", userID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count sessions for user: %w", err)
	}
	var sessions []GameSession
	if err := query.Limit(l).Offset(o).
		Order("started_at DESC").Find(&sessions).Error; err != nil {
		return nil, 0, fmt.Errorf("list sessions for user: %w", err)
	}
	return sessions, total, nil
}

// GetGameHistory returns user's game history
func (r *CasinoRepository) GetGameHistory(ctx context.Context, userID string, gameID *string, dateRange *struct{ From, To time.Time }, limit, offset int32) ([]GameSession, int64, error) {
	if r.db == nil {
		return nil, 0, errCasinoDatabaseUnavailable
	}
	l, o := normalizePage(limit, offset)
	query := r.db.WithContext(ctx).Model(&GameSession{}).Where("user_id = ?", userID)
	if gameID != nil {
		query = query.Where("game_id = ?", *gameID)
	}
	if dateRange != nil {
		query = query.Where("started_at BETWEEN ? AND ?", dateRange.From, dateRange.To)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count game history: %w", err)
	}
	var sessions []GameSession
	if err := query.Limit(l).Offset(o).
		Order("started_at DESC").Find(&sessions).Error; err != nil {
		return nil, 0, fmt.Errorf("list game history: %w", err)
	}
	return sessions, total, nil
}

// GetRoundHistory returns round history for a session
func (r *CasinoRepository) GetRoundHistory(ctx context.Context, sessionID string, limit, offset int32) ([]GameRound, int64, error) {
	if r.db == nil {
		return nil, 0, errCasinoDatabaseUnavailable
	}
	l, o := normalizePage(limit, offset)
	base := r.db.WithContext(ctx).Model(&GameRound{}).Where("session_id = ?", sessionID)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count round history: %w", err)
	}
	var rounds []GameRound
	if err := r.db.WithContext(ctx).Where("session_id = ?", sessionID).
		Limit(l).Offset(o).Order("started_at ASC").Find(&rounds).Error; err != nil {
		return nil, 0, fmt.Errorf("list round history: %w", err)
	}
	return rounds, total, nil
}

// CreateGameRound creates a new game round
func (r *CasinoRepository) CreateGameRound(ctx context.Context, round *GameRound) error {
	if r.db == nil {
		return errCasinoDatabaseUnavailable
	}
	if round == nil || round.ID == "" {
		return errors.New("round id is required")
	}
	var err error
	if round.BetAmount, err = normalizeDecimalAmount(round.BetAmount); err != nil {
		return fmt.Errorf("invalid bet_amount: %w", err)
	}
	if round.WinAmount, err = normalizeDecimalAmount(round.WinAmount); err != nil {
		return fmt.Errorf("invalid win_amount: %w", err)
	}
	if round.NetResult, err = normalizeDecimalAmount(round.NetResult); err != nil {
		return fmt.Errorf("invalid net_result: %w", err)
	}
	if round.GameState == nil {
		round.GameState = map[string]string{}
	}
	if round.Metadata == nil {
		round.Metadata = map[string]string{}
	}
	if err := r.db.WithContext(ctx).Create(round).Error; err != nil {
		return fmt.Errorf("create game round: %w", err)
	}
	return nil
}

// GameCacheKey returns the Redis key for a cached game.
func GameCacheKey(gameID string) string {
	return "casino:game:" + gameID
}

// CacheGame caches game data in Redis
func (r *CasinoRepository) CacheGame(ctx context.Context, game *Game, ttl time.Duration) error {
	if r.redis == nil {
		return errCasinoRedisUnavailable
	}

	payload, err := json.Marshal(game)
	if err != nil {
		return fmt.Errorf("marshal game for cache: %w", err)
	}

	return r.redis.Set(ctx, GameCacheKey(game.ID), payload, ttl).Err()
}

// GetCachedGame retrieves cached game data from Redis.
// Returns (nil, nil) on cache miss.
func (r *CasinoRepository) GetCachedGame(ctx context.Context, gameID string) (*Game, error) {
	if r.redis == nil {
		return nil, errCasinoRedisUnavailable
	}

	payload, err := r.redis.Get(ctx, GameCacheKey(gameID)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get cached game %q: %w", gameID, err)
	}

	var game Game
	if err := json.Unmarshal(payload, &game); err != nil {
		return nil, fmt.Errorf("unmarshal cached game %q: %w", gameID, err)
	}

	return &game, nil
}

// InvalidateGameCache invalidates cached game data
func (r *CasinoRepository) InvalidateGameCache(ctx context.Context, gameID string) error {
	if r.redis == nil {
		return errCasinoRedisUnavailable
	}

	return r.redis.Del(ctx, GameCacheKey(gameID)).Err()
}
