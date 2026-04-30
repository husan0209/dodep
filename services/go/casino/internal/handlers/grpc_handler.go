package handlers

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/opus-casino/casino/internal/service"
	casinov1 "github.com/opus-casino/proto/gen/go/casino/v1"
	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"
)

// CasinoGRPCHandler handles gRPC requests for Casino Service
type CasinoGRPCHandler struct {
	casinov1.UnimplementedCasinoServiceServer
	service *service.CasinoService
	log     *zap.Logger
}

// NewCasinoGRPCHandler creates a new gRPC handler
func NewCasinoGRPCHandler(service *service.CasinoService) *CasinoGRPCHandler {
	return &CasinoGRPCHandler{
		service: service,
	}
}

// GetGames returns available casino games
func (h *CasinoGRPCHandler) GetGames(ctx context.Context, req *casinov1.GetGamesRequest) (*casinov1.GetGamesResponse, error) {
	pageSize := int32(50)
	if req.Pagination != nil && req.Pagination.PageSize > 0 {
		pageSize = req.Pagination.PageSize
	}
	opts := service.GetGamesOptions{
		Limit:  pageSize,
		Offset: 0,
		Search: req.Search,
	}

	if req.ProviderId != nil {
		opts.ProviderID = req.ProviderId
	}
	if req.Category != nil {
		catStr := req.Category.String()
		opts.Category = &catStr
	}
	if len(req.Tags) > 0 {
		opts.Tags = req.Tags
	}

	result, err := h.service.GetGames(ctx, opts)
	if err != nil {
		h.log.Error("GetGames failed", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to get games")
	}

	games := make([]*casinov1.Game, len(result.Games))
	for i, game := range result.Games {
		games[i] = toProtoGame(&game)
	}

	return &casinov1.GetGamesResponse{
		Games:      games,
		Pagination: &commonv1.PageResponse{TotalCount: &result.TotalCount},
	}, nil
}

// GetGame returns game details
func (h *CasinoGRPCHandler) GetGame(ctx context.Context, req *casinov1.GetGameRequest) (*casinov1.GetGameResponse, error) {
	game, err := h.service.GetGame(ctx, req.GameId)
	if err != nil {
		h.log.Error("GetGame failed", zap.String("game_id", req.GameId), zap.Error(err))
		return nil, status.Error(codes.NotFound, "game not found")
	}

	return &casinov1.GetGameResponse{
		Game: toProtoGame(game),
	}, nil
}

// LaunchGame launches a game session
func (h *CasinoGRPCHandler) LaunchGame(ctx context.Context, req *casinov1.LaunchGameRequest) (*casinov1.LaunchGameResponse, error) {
	if req.UserId == nil || req.UserId.Value == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	serviceReq := &service.LaunchGameRequest{
		UserID:     fmt.Sprintf("%s", req.UserId.GetValue()),
		GameID:     req.GameId,
		DeviceType: req.DeviceType,
		LobbyURL:   req.LobbyUrl,
	}

	result, err := h.service.LaunchGame(ctx, serviceReq)
	if err != nil {
		h.log.Error("LaunchGame failed", zap.Error(err))
		return &casinov1.LaunchGameResponse{
			Error: &commonv1.ErrorDetails{
				ErrorMessage: err.Error(),
			},
		}, nil
	}

	return &casinov1.LaunchGameResponse{
		Session:   toProtoGameSession(result.Session),
		LaunchUrl: result.LaunchURL,
		Token:     result.Token,
	}, nil
}

// GetGameSession returns game session details
func (h *CasinoGRPCHandler) GetGameSession(ctx context.Context, req *casinov1.GetGameSessionRequest) (*casinov1.GetGameSessionResponse, error) {
	session, err := h.service.GetGameSession(ctx, req.SessionId)
	if err != nil {
		h.log.Error("GetGameSession failed", zap.String("session_id", req.SessionId), zap.Error(err))
		return nil, status.Error(codes.NotFound, "session not found")
	}

	return &casinov1.GetGameSessionResponse{
		Session: toProtoGameSession(session),
	}, nil
}

// EndGameSession ends a game session
func (h *CasinoGRPCHandler) EndGameSession(ctx context.Context, req *casinov1.EndGameSessionRequest) (*casinov1.EndGameSessionResponse, error) {
	serviceReq := &service.EndGameSessionRequest{
		SessionID: req.SessionId,
	}

	result, err := h.service.EndGameSession(ctx, serviceReq)
	if err != nil {
		h.log.Error("EndGameSession failed", zap.String("session_id", req.SessionId), zap.Error(err))
		return &casinov1.EndGameSessionResponse{
			Success: false,
			Error: &commonv1.ErrorDetails{
				ErrorMessage: err.Error(),
			},
		}, nil
	}

	return &casinov1.EndGameSessionResponse{
		Success: result.Success,
		Summary: toProtoGameSessionSummary(result.Summary),
	}, nil
}

// GetGameHistory returns user's game history
func (h *CasinoGRPCHandler) GetGameHistory(ctx context.Context, req *casinov1.GetGameHistoryRequest) (*casinov1.GetGameHistoryResponse, error) {
	pageSize := int32(20)
	if req.Pagination != nil && req.Pagination.PageSize > 0 {
		pageSize = req.Pagination.PageSize
	}
	opts := service.GetGameHistoryOptions{
		UserID: req.UserId.GetValue(),
		Limit:  pageSize,
		Offset: 0,
	}

	if req.GameId != nil {
		opts.GameID = req.GameId
	}
	if req.DateRange != nil {
		from := req.DateRange.From.AsTime()
		to := req.DateRange.To.AsTime()
		opts.DateFrom = &from
		opts.DateTo = &to
	}

	result, err := h.service.GetGameHistory(ctx, opts)
	if err != nil {
		h.log.Error("GetGameHistory failed", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to get game history")
	}

	sessions := make([]*casinov1.GameSessionSummary, len(result.Sessions))
	for i, session := range result.Sessions {
		sessions[i] = toProtoGameSessionSummary(&service.GameSessionSummary{
			SessionID:     session.ID,
			GameID:        session.GameID,
			GameName:      session.GameID,
			TotalBet:      session.BalanceAtStart,
			StartedAt:     session.StartedAt,
			EndedAt:       *session.EndedAt,
			DurationSecs:  int64(session.EndedAt.Sub(session.StartedAt).Seconds()),
		})
	}

	return &casinov1.GetGameHistoryResponse{
		Sessions:   sessions,
		Pagination: &commonv1.PageResponse{TotalCount: &result.TotalCount},
	}, nil
}

// GetRoundHistory returns round history for a game session
func (h *CasinoGRPCHandler) GetRoundHistory(ctx context.Context, req *casinov1.GetRoundHistoryRequest) (*casinov1.GetRoundHistoryResponse, error) {
	var pageSize int32 = 100
	if req.Pagination != nil && req.Pagination.PageSize > 0 {
		pageSize = req.Pagination.PageSize
	}
	rounds, total, err := h.service.GetRoundHistory(ctx, req.SessionId, pageSize, 0)
	if err != nil {
		h.log.Error("GetRoundHistory failed", zap.String("session_id", req.SessionId), zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to get round history")
	}

	pbRounds := make([]*casinov1.GameRound, len(rounds))
	for i, round := range rounds {
		pbRounds[i] = toProtoGameRound(&round)
	}

	return &casinov1.GetRoundHistoryResponse{
		Rounds:     pbRounds,
		Pagination: &commonv1.PageResponse{TotalCount: &total},
	}, nil
}

// GetProviders returns game providers
func (h *CasinoGRPCHandler) GetProviders(ctx context.Context, req *casinov1.GetProvidersRequest) (*casinov1.GetProvidersResponse, error) {
	var isActive *bool
	if req.IsActive != nil {
		isActive = req.IsActive
	}

	providers, err := h.service.GetProviders(ctx, isActive)
	if err != nil {
		h.log.Error("GetProviders failed", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to get providers")
	}

	pbProviders := make([]*casinov1.Provider, len(providers))
	for i, provider := range providers {
		pbProviders[i] = toProtoProvider(&provider)
	}

	return &casinov1.GetProvidersResponse{
		Providers: pbProviders,
	}, nil
}

// GetProvider returns provider details
func (h *CasinoGRPCHandler) GetProvider(ctx context.Context, req *casinov1.GetProviderRequest) (*casinov1.GetProviderResponse, error) {
	provider, err := h.service.GetProvider(ctx, req.ProviderId)
	if err != nil {
		h.log.Error("GetProvider failed", zap.String("provider_id", req.ProviderId), zap.Error(err))
		return nil, status.Error(codes.NotFound, "provider not found")
	}

	return &casinov1.GetProviderResponse{
		Provider: toProtoProvider(provider),
	}, nil
}

// ============ Helper functions ============

func toProtoGame(game *service.Game) *casinov1.Game {
	catVal := casinov1.GameCategory(casinov1.GameCategory_value[game.Category])
	return &casinov1.Game{
		Id:                  game.ID,
		Name:                game.Name,
		ProviderId:          game.ProviderID,
		ProviderName:        game.ProviderName,
		Category:            catVal,
		Tags:                game.Tags,
		Description:         game.Description,
		ImageUrl:            game.ImageURL,
		ThumbnailUrl:        game.ThumbnailURL,
		SupportedCurrencies: game.SupportedCurrencies,
		BetRange: &casinov1.MoneyRange{
			Min: &commonv1.Money{Amount: game.MinBet, Currency: "USD"},
			Max: &commonv1.Money{Amount: game.MaxBet, Currency: "USD"},
		},
		Features: &casinov1.GameFeatures{
			HasFreeSpins: game.Features.HasFreeSpins,
			HasBonusBuy:  game.Features.HasBonusBuy,
			HasJackpot:   game.Features.HasJackpot,
		},
		Stats: &casinov1.GameStats{
			Rtp:        game.RTP,
			Volatility: game.Volatility,
		},
		IsActive:            game.IsActive,
		IsDemoAvailable:     game.IsDemoAvailable,
		RestrictedCountries: game.RestrictedCountries,
		PopularityScore:     game.PopularityScore,
		ReleasedAt:          timestamppb.New(game.ReleasedAt),
		Metadata:            game.Metadata,
	}
}

func toProtoProvider(provider *service.Provider) *casinov1.Provider {
	return &casinov1.Provider{
		Id:                  provider.ID,
		Name:                provider.Name,
		LogoUrl:             provider.LogoURL,
		Description:         provider.Description,
		IsActive:            provider.IsActive,
		GamesCount:          provider.GamesCount,
		SupportedCurrencies: provider.SupportedCurrencies,
		RestrictedCountries: provider.RestrictedCountries,
		Metadata:            provider.Metadata,
	}
}

func toProtoGameSession(session *service.GameSession) *casinov1.GameSession {
	return &casinov1.GameSession{
		Id:             session.ID,
		UserId:         &commonv1.UserId{Value: session.UserID},
		GameId:         session.GameID,
		ProviderId:     session.ProviderID,
		Status:         casinov1.GameSessionStatus(casinov1.GameSessionStatus_value[session.Status]),
		BalanceAtStart: &commonv1.Money{Amount: session.BalanceAtStart, Currency: "USD"},
		StartedAt:      timestamppb.New(session.StartedAt),
		LastActivity:   timestamppb.New(session.LastActivity),
		DeviceType:     session.DeviceType,
		LobbyUrl:       session.LobbyURL,
		LaunchUrl:      session.LaunchURL,
		Token:          session.Token,
		Metadata:       session.Metadata,
	}
}

func toProtoGameSessionSummary(summary *service.GameSessionSummary) *casinov1.GameSessionSummary {
	return &casinov1.GameSessionSummary{
		SessionId:       summary.SessionID,
		GameId:          summary.GameID,
		GameName:        summary.GameName,
		TotalBet:        &commonv1.Money{Amount: summary.TotalBet, Currency: "USD"},
		TotalWin:        &commonv1.Money{Amount: summary.TotalWin, Currency: "USD"},
		NetResult:       &commonv1.Money{Amount: summary.NetResult, Currency: "USD"},
		RoundsPlayed:    summary.RoundsPlayed,
		StartedAt:       timestamppb.New(summary.StartedAt),
		EndedAt:         timestamppb.New(summary.EndedAt),
		DurationSeconds: summary.DurationSecs,
	}
}

func toProtoGameRound(round *service.GameRound) *casinov1.GameRound {
	return &casinov1.GameRound{
		Id:        round.ID,
		SessionId: round.SessionID,
		RoundId:   round.RoundID,
		BetAmount: &commonv1.Money{Amount: round.BetAmount, Currency: "USD"},
		WinAmount: &commonv1.Money{Amount: round.WinAmount, Currency: "USD"},
		NetResult: &commonv1.Money{Amount: round.NetResult, Currency: "USD"},
		Status:    casinov1.GameRoundStatus(casinov1.GameRoundStatus_value[round.Status]),
		StartedAt: timestamppb.New(round.StartedAt),
		EndedAt:   timestamppb.New(round.EndedAt),
		GameState: round.GameState,
		Metadata:  round.Metadata,
	}
}
