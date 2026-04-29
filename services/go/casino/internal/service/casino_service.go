package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/opus-casino/casino/internal/provider"
	"github.com/opus-casino/casino/internal/repository"
)

// UserServiceClient is the interface casino service needs from user service.
type UserServiceClient interface {
	GetKYCLevel(ctx context.Context, userID int64) (int, error)
	GetCountry(ctx context.Context, userID int64) (string, error)
	IsSelfExcluded(ctx context.Context, userID int64) (bool, error)
}

// CasinoService handles casino business logic
type CasinoService struct {
	repo      *repository.CasinoRepository
	registry  *provider.Registry
	wallet    WalletClient
	userSvc   UserServiceClient
	log       *zap.Logger
}

// NewCasinoService creates a new casino service.
func NewCasinoService(
	repo *repository.CasinoRepository,
	registry *provider.Registry,
	wallet WalletClient,
	userSvc UserServiceClient,
	log *zap.Logger,
) *CasinoService {
	return &CasinoService{
		repo:     repo,
		registry: registry,
		wallet:   wallet,
		userSvc:  userSvc,
		log:      log,
	}
}

// Game represents a casino game
type Game struct {
	ID                  string                 `json:"id"`
	Name                string                 `json:"name"`
	ProviderID          string                 `json:"provider_id"`
	ProviderName        string                 `json:"provider_name"`
	Category            string                 `json:"category"`
	Tags                []string               `json:"tags"`
	Description         string                 `json:"description"`
	ImageURL            string                 `json:"image_url"`
	ThumbnailURL        string                 `json:"thumbnail_url"`
	SupportedCurrencies []string               `json:"supported_currencies"`
	MinBet              string                 `json:"min_bet"`
	MaxBet              string                 `json:"max_bet"`
	Features            GameFeatures           `json:"features"`
	RTP                 float64                `json:"rtp"`
	Volatility          string                 `json:"volatility"`
	IsActive            bool                   `json:"is_active"`
	IsDemoAvailable     bool                   `json:"is_demo_available"`
	RestrictedCountries []string               `json:"restricted_countries"`
	PopularityScore     int32                  `json:"popularity_score"`
	ReleasedAt          time.Time              `json:"released_at"`
	Metadata            map[string]string      `json:"metadata"`
}

// GameFeatures represents game features
type GameFeatures struct {
	HasFreeSpins   bool     `json:"has_free_spins"`
	HasBonusBuy    bool     `json:"has_bonus_buy"`
	HasJackpot     bool     `json:"has_jackpot"`
	HasMultiplayer bool     `json:"has_multiplayer"`
	HasLiveDealer  bool     `json:"has_live_dealer"`
	BonusFeatures  []string `json:"bonus_features"`
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
	ID        string            `json:"id"`
	SessionID string            `json:"session_id"`
	RoundID   string            `json:"round_id"`
	BetAmount string            `json:"bet_amount"`
	WinAmount string            `json:"win_amount"`
	NetResult string            `json:"net_result"`
	Status    string            `json:"status"`
	StartedAt time.Time         `json:"started_at"`
	EndedAt   time.Time         `json:"ended_at"`
	GameState map[string]string `json:"game_state"`
	Metadata  map[string]string `json:"metadata"`
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

// GetGamesResult represents paginated games result
type GetGamesResult struct {
	Games      []Game
	TotalCount int64
}

// GetGameHistoryOptions represents options for getting game history
type GetGameHistoryOptions struct {
	UserID    string
	GameID    *string
	DateFrom  *time.Time
	DateTo    *time.Time
	Limit     int32
	Offset    int32
}

// GetGameHistoryResult represents paginated game history result
type GetGameHistoryResult struct {
	Sessions   []GameSession
	TotalCount int64
}

// GetGames returns games with optional filtering
func (s *CasinoService) GetGames(ctx context.Context, opts GetGamesOptions) (*GetGamesResult, error) {
	repoOpts := repository.GetGamesOptions{
		ProviderID: opts.ProviderID,
		Category:   opts.Category,
		Tags:       opts.Tags,
		Search:     opts.Search,
		Limit:      opts.Limit,
		Offset:     opts.Offset,
	}

	games, total, err := s.repo.GetGames(ctx, repoOpts)
	if err != nil {
		s.log.Error("Failed to get games", zap.Error(err), zap.Any("options", opts))
		return nil, err
	}

	result := make([]Game, len(games))
	for i, g := range games {
		result[i] = Game{
			ID:                  g.ID,
			Name:                g.Name,
			ProviderID:          g.ProviderID,
			ProviderName:        g.ProviderName,
			Category:            g.Category,
			Tags:                g.Tags,
			Description:         g.Description,
			ImageURL:            g.ImageURL,
			ThumbnailURL:        g.ThumbnailURL,
			SupportedCurrencies: g.SupportedCurrencies,
			MinBet:              g.MinBet,
			MaxBet:              g.MaxBet,
			Features: GameFeatures{
				HasFreeSpins:   g.Features.HasFreeSpins,
				HasBonusBuy:    g.Features.HasBonusBuy,
				HasJackpot:     g.Features.HasJackpot,
				HasMultiplayer: g.Features.HasMultiplayer,
				HasLiveDealer:  g.Features.HasLiveDealer,
				BonusFeatures:  g.Features.BonusFeatures,
			},
			RTP:                 g.RTP,
			Volatility:          g.Volatility,
			IsActive:            g.IsActive,
			IsDemoAvailable:     g.IsDemoAvailable,
			RestrictedCountries: g.RestrictedCountries,
			PopularityScore:     g.PopularityScore,
			ReleasedAt:          g.ReleasedAt,
			Metadata:            g.Metadata,
		}
	}

	return &GetGamesResult{
		Games:      result,
		TotalCount: total,
	}, nil
}

// GetGame returns a game by ID
func (s *CasinoService) GetGame(ctx context.Context, gameID string) (*Game, error) {
	// Try to get from cache first
	cachedGame, err := s.repo.GetCachedGame(ctx, gameID)
	if err == nil && cachedGame != nil {
		s.log.Debug("Game retrieved from cache", zap.String("game_id", gameID))
		return &Game{
			ID:                  cachedGame.ID,
			Name:                cachedGame.Name,
			ProviderID:          cachedGame.ProviderID,
			ProviderName:        cachedGame.ProviderName,
			Category:            cachedGame.Category,
			Tags:                cachedGame.Tags,
			Description:         cachedGame.Description,
			ImageURL:            cachedGame.ImageURL,
			ThumbnailURL:        cachedGame.ThumbnailURL,
			SupportedCurrencies: cachedGame.SupportedCurrencies,
			MinBet:              cachedGame.MinBet,
			MaxBet:              cachedGame.MaxBet,
			Features: GameFeatures{
				HasFreeSpins:   cachedGame.Features.HasFreeSpins,
				HasBonusBuy:    cachedGame.Features.HasBonusBuy,
				HasJackpot:     cachedGame.Features.HasJackpot,
				HasMultiplayer: cachedGame.Features.HasMultiplayer,
				HasLiveDealer:  cachedGame.Features.HasLiveDealer,
				BonusFeatures:  cachedGame.Features.BonusFeatures,
			},
			RTP:                 cachedGame.RTP,
			Volatility:          cachedGame.Volatility,
			IsActive:            cachedGame.IsActive,
			IsDemoAvailable:     cachedGame.IsDemoAvailable,
			RestrictedCountries: cachedGame.RestrictedCountries,
			PopularityScore:     cachedGame.PopularityScore,
			ReleasedAt:          cachedGame.ReleasedAt,
			Metadata:            cachedGame.Metadata,
		}, nil
	}

	// Fallback to database
	game, err := s.repo.GetGame(ctx, gameID)
	if err != nil {
		s.log.Error("Failed to get game", zap.String("game_id", gameID), zap.Error(err))
		return nil, err
	}

	// Cache the game for future requests
	if err := s.repo.CacheGame(ctx, game, 5*time.Minute); err != nil {
		s.log.Warn("Failed to cache game", zap.String("game_id", gameID), zap.Error(err))
	}

	return &Game{
		ID:                  game.ID,
		Name:                game.Name,
		ProviderID:          game.ProviderID,
		ProviderName:        game.ProviderName,
		Category:            game.Category,
		Tags:                game.Tags,
		Description:         game.Description,
		ImageURL:            game.ImageURL,
		ThumbnailURL:        game.ThumbnailURL,
		SupportedCurrencies: game.SupportedCurrencies,
		MinBet:              game.MinBet,
		MaxBet:              game.MaxBet,
		Features: GameFeatures{
			HasFreeSpins:   game.Features.HasFreeSpins,
			HasBonusBuy:    game.Features.HasBonusBuy,
			HasJackpot:     game.Features.HasJackpot,
			HasMultiplayer: game.Features.HasMultiplayer,
			HasLiveDealer:  game.Features.HasLiveDealer,
			BonusFeatures:  game.Features.BonusFeatures,
		},
		RTP:                 game.RTP,
		Volatility:          game.Volatility,
		IsActive:            game.IsActive,
		IsDemoAvailable:     game.IsDemoAvailable,
		RestrictedCountries: game.RestrictedCountries,
		PopularityScore:     game.PopularityScore,
		ReleasedAt:          game.ReleasedAt,
		Metadata:            game.Metadata,
	}, nil
}

// GetProviders returns all providers
func (s *CasinoService) GetProviders(ctx context.Context, isActive *bool) ([]Provider, error) {
	providers, err := s.repo.GetProviders(ctx, isActive)
	if err != nil {
		s.log.Error("Failed to get providers", zap.Error(err))
		return nil, err
	}

	result := make([]Provider, len(providers))
	for i, p := range providers {
		result[i] = Provider{
			ID:                  p.ID,
			Name:                p.Name,
			LogoURL:             p.LogoURL,
			Description:         p.Description,
			IsActive:            p.IsActive,
			GamesCount:          p.GamesCount,
			SupportedCurrencies: p.SupportedCurrencies,
			RestrictedCountries: p.RestrictedCountries,
			Metadata:            p.Metadata,
		}
	}

	return result, nil
}

// GetProvider returns a provider by ID
func (s *CasinoService) GetProvider(ctx context.Context, providerID string) (*Provider, error) {
	provider, err := s.repo.GetProvider(ctx, providerID)
	if err != nil {
		s.log.Error("Failed to get provider", zap.String("provider_id", providerID), zap.Error(err))
		return nil, err
	}

	return &Provider{
		ID:                  provider.ID,
		Name:                provider.Name,
		LogoURL:             provider.LogoURL,
		Description:         provider.Description,
		IsActive:            provider.IsActive,
		GamesCount:          provider.GamesCount,
		SupportedCurrencies: provider.SupportedCurrencies,
		RestrictedCountries: provider.RestrictedCountries,
		Metadata:            provider.Metadata,
	}, nil
}

// LaunchGameRequest represents a request to launch a game
type LaunchGameRequest struct {
	UserID     string
	GameID     string
	DeviceType string
	LobbyURL   string
}

// LaunchGameResult represents the result of launching a game
type LaunchGameResult struct {
	Session   *GameSession
	LaunchURL string
	Token     string
}

// LaunchGame launches a game session.
// It validates the game, checks country restrictions and self-exclusion,
// fetches the current balance, generates a signed launch URL via the provider adapter,
// and persists the session.
func (s *CasinoService) LaunchGame(ctx context.Context, req *LaunchGameRequest) (*LaunchGameResult, error) {
	// 1. Validate game exists and is active
	game, err := s.GetGame(ctx, req.GameID)
	if err != nil {
		return nil, errors.New("game not found")
	}
	if !game.IsActive {
		return nil, errors.New("game is not active")
	}

	// 2. Parse internal user_id
	var userIDInt int64
	if _, err := fmt.Sscanf(req.UserID, "%d", &userIDInt); err != nil {
		return nil, fmt.Errorf("invalid user_id format: %w", err)
	}

	// 3. Check self-exclusion
	if s.userSvc != nil {
		excluded, err := s.userSvc.IsSelfExcluded(ctx, userIDInt)
		if err != nil {
			s.log.Warn("Failed to check self-exclusion", zap.Error(err))
		} else if excluded {
			return nil, errors.New("user is self-excluded")
		}
	}

	// 4. Check country restriction
	if s.userSvc != nil && len(game.RestrictedCountries) > 0 {
		country, err := s.userSvc.GetCountry(ctx, userIDInt)
		if err != nil {
			s.log.Warn("Failed to get user country for geo-check", zap.Error(err))
		} else {
			for _, blocked := range game.RestrictedCountries {
				if strings.EqualFold(blocked, country) {
					return nil, fmt.Errorf("game not available in your country (%s)", country)
				}
			}
		}
	}

	// 5. Get current balance
	balanceStr := "0"
	if s.wallet != nil {
		balance, err := s.wallet.GetBalance(ctx, userIDInt, "USD")
		if err != nil {
			s.log.Warn("Failed to get balance for game launch", zap.Error(err))
		} else {
			balanceStr = balance.StringFixed(2)
		}
	}

	// 6. Build session token and opaque player_id (never expose internal user_id to provider)
	sessionToken := uuid.New().String()
	playerID := fmt.Sprintf("p%d_%s", userIDInt, sessionToken[:8])

	// 7. Get signed launch URL from provider adapter
	var launchURL string
	if s.registry != nil {
		adapter, err := s.registry.Get(game.ProviderID)
		if err != nil {
			return nil, fmt.Errorf("provider %q not available: %w", game.ProviderID, err)
		}

		launchURL, err = adapter.BuildLaunchURL(ctx, provider.LaunchRequest{
			UserID:     req.UserID,
			PlayerID:   playerID,
			Token:      sessionToken,
			GameID:     req.GameID,
			Currency:   "USD",
			Balance:    decimal.RequireFromString(balanceStr),
			Language:   "en",
			LobbyURL:   req.LobbyURL,
			DeviceType: req.DeviceType,
		})
		if err != nil {
			return nil, fmt.Errorf("build launch url: %w", err)
		}
	} else {
		launchURL = fmt.Sprintf("/games/demo?game=%s&token=%s", req.GameID, sessionToken)
	}

	// 8. Persist the session
	session := &repository.GameSession{
		ID:             uuid.New().String(),
		UserID:         req.UserID,
		GameID:         req.GameID,
		ProviderID:     game.ProviderID,
		Status:         "active",
		BalanceAtStart: balanceStr,
		DeviceType:     req.DeviceType,
		LobbyURL:       req.LobbyURL,
		LaunchURL:      launchURL,
		Token:          sessionToken,
		StartedAt:      time.Now(),
		LastActivity:   time.Now(),
		Metadata:       map[string]string{"player_id": playerID},
	}

	if err := s.repo.CreateGameSession(ctx, session); err != nil {
		s.log.Error("Failed to create game session", zap.Error(err))
		return nil, err
	}

	s.log.Info("Game session created",
		zap.String("session_id", session.ID),
		zap.String("user_id", req.UserID),
		zap.String("game_id", req.GameID),
		zap.String("provider", game.ProviderID))

	return &LaunchGameResult{
		Session: &GameSession{
			ID:             session.ID,
			UserID:         session.UserID,
			GameID:         session.GameID,
			ProviderID:     session.ProviderID,
			Status:         session.Status,
			BalanceAtStart: session.BalanceAtStart,
			StartedAt:      session.StartedAt,
			LastActivity:   session.LastActivity,
			DeviceType:     session.DeviceType,
			LobbyURL:       session.LobbyURL,
			LaunchURL:      session.LaunchURL,
			Token:          session.Token,
			Metadata:       session.Metadata,
		},
		LaunchURL: launchURL,
		Token:     sessionToken,
	}, nil
}

// GetGameSession returns a game session by ID
func (s *CasinoService) GetGameSession(ctx context.Context, sessionID string) (*GameSession, error) {
	session, err := s.repo.GetGameSession(ctx, sessionID)
	if err != nil {
		s.log.Error("Failed to get game session", zap.String("session_id", sessionID), zap.Error(err))
		return nil, err
	}

	return &GameSession{
		ID:             session.ID,
		UserID:         session.UserID,
		GameID:         session.GameID,
		ProviderID:     session.ProviderID,
		Status:         session.Status,
		BalanceAtStart: session.BalanceAtStart,
		StartedAt:      session.StartedAt,
		LastActivity:   session.LastActivity,
		EndedAt:        session.EndedAt,
		DeviceType:     session.DeviceType,
		LobbyURL:       session.LobbyURL,
		LaunchURL:      session.LaunchURL,
		Token:          session.Token,
		Metadata:       session.Metadata,
	}, nil
}

// EndGameSessionRequest represents a request to end a game session
type EndGameSessionRequest struct {
	SessionID string
}

// EndGameSessionResult represents the result of ending a game session
type EndGameSessionResult struct {
	Success bool
	Summary *GameSessionSummary
}

// GameSessionSummary represents a summary of a game session
type GameSessionSummary struct {
	SessionID     string
	GameID        string
	GameName      string
	TotalBet      string
	TotalWin      string
	NetResult     string
	RoundsPlayed  int32
	StartedAt     time.Time
	EndedAt       time.Time
	DurationSecs  int64
}

// EndGameSession ends a game session
func (s *CasinoService) EndGameSession(ctx context.Context, req *EndGameSessionRequest) (*EndGameSessionResult, error) {
	session, err := s.repo.GetGameSession(ctx, req.SessionID)
	if err != nil {
		return nil, errors.New("session not found")
	}

	endedAt := time.Now()
	if err := s.repo.EndGameSession(ctx, req.SessionID, endedAt); err != nil {
		s.log.Error("Failed to end game session", zap.Error(err))
		return nil, err
	}

	// Get round history for summary
	rounds, _, err := s.repo.GetRoundHistory(ctx, req.SessionID, 1000, 0)
	if err != nil {
		s.log.Error("Failed to get round history", zap.Error(err))
	}

	// Calculate totals using decimal arithmetic
	totalBetDec := decimal.Zero
	totalWinDec := decimal.Zero
	for _, round := range rounds {
		if b, err := decimal.NewFromString(round.BetAmount); err == nil {
			totalBetDec = totalBetDec.Add(b)
		}
		if w, err := decimal.NewFromString(round.WinAmount); err == nil {
			totalWinDec = totalWinDec.Add(w)
		}
	}
	totalBet := totalBetDec.StringFixed(2)
	totalWin := totalWinDec.StringFixed(2)
	netResult := totalWinDec.Sub(totalBetDec).StringFixed(2)

	s.log.Info("Game session ended",
		zap.String("session_id", req.SessionID),
		zap.String("game_id", session.GameID))

	return &EndGameSessionResult{
		Success: true,
		Summary: &GameSessionSummary{
			SessionID:     req.SessionID,
			GameID:        session.GameID,
			GameName:      session.GameID,
			TotalBet:      totalBet,
			TotalWin:      totalWin,
			NetResult:     netResult,
			RoundsPlayed:  int32(len(rounds)),
			StartedAt:     session.StartedAt,
			EndedAt:       endedAt,
			DurationSecs:  int64(endedAt.Sub(session.StartedAt).Seconds()),
		},
	}, nil
}

// GetGameHistory returns user's game history
func (s *CasinoService) GetGameHistory(ctx context.Context, opts GetGameHistoryOptions) (*GetGameHistoryResult, error) {
	var dateRange *struct{ From, To time.Time }
	if opts.DateFrom != nil && opts.DateTo != nil {
		dateRange = &struct{ From, To time.Time }{
			From: *opts.DateFrom,
			To:   *opts.DateTo,
		}
	}

	sessions, total, err := s.repo.GetGameHistory(ctx, opts.UserID, opts.GameID, dateRange, opts.Limit, opts.Offset)
	if err != nil {
		s.log.Error("Failed to get game history", zap.Error(err))
		return nil, err
	}

	result := make([]GameSession, len(sessions))
	for i, session := range sessions {
		result[i] = GameSession{
			ID:             session.ID,
			UserID:         session.UserID,
			GameID:         session.GameID,
			ProviderID:     session.ProviderID,
			Status:         session.Status,
			BalanceAtStart: session.BalanceAtStart,
			StartedAt:      session.StartedAt,
			LastActivity:   session.LastActivity,
			EndedAt:        session.EndedAt,
			DeviceType:     session.DeviceType,
			LobbyURL:       session.LobbyURL,
			LaunchURL:      session.LaunchURL,
			Token:          session.Token,
			Metadata:       session.Metadata,
		}
	}

	return &GetGameHistoryResult{
		Sessions:   result,
		TotalCount: total,
	}, nil
}

// GetRoundHistory returns round history for a session
func (s *CasinoService) GetRoundHistory(ctx context.Context, sessionID string, limit, offset int32) ([]GameRound, int64, error) {
	rounds, total, err := s.repo.GetRoundHistory(ctx, sessionID, limit, offset)
	if err != nil {
		s.log.Error("Failed to get round history", zap.String("session_id", sessionID), zap.Error(err))
		return nil, 0, err
	}

	result := make([]GameRound, len(rounds))
	for i, round := range rounds {
		result[i] = GameRound{
			ID:        round.ID,
			SessionID: round.SessionID,
			RoundID:   round.RoundID,
			BetAmount: round.BetAmount,
			WinAmount: round.WinAmount,
			NetResult: round.NetResult,
			Status:    round.Status,
			StartedAt: round.StartedAt,
			EndedAt:   round.EndedAt,
			GameState: round.GameState,
			Metadata:  round.Metadata,
		}
	}

	return result, total, nil
}
