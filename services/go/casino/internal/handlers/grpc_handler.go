package handlers

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opus-casino/casino/internal/proto/casino/v1"
	"github.com/opus-casino/casino/internal/service"
	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"
)

// CasinoGRPCHandler handles gRPC requests for Casino Service
type CasinoGRPCHandler struct {
	pb.UnimplementedCasinoServiceServer
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
func (h *CasinoGRPCHandler) GetGames(ctx context.Context, req *pb.GetGamesRequest) (*pb.GetGamesResponse, error) {
	opts := service.GetGamesOptions{
		Limit:  req.Pagination.Limit,
		Offset: req.Pagination.Offset,
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

	games := make([]*pb.Game, len(result.Games))
	for i, game := range result.Games {
		games[i] = toProtoGame(&game)
	}

	return &pb.GetGamesResponse{
		Games: games,
		Pagination: &commonv1.PageResponse{
			Total: result.TotalCount,
			Limit: req.Pagination.Limit,
			Offset: req.Pagination.Offset,
		},
	}, nil
}

// GetGame returns game details
func (h *CasinoGRPCHandler) GetGame(ctx context.Context, req *pb.GetGameRequest) (*pb.GetGameResponse, error) {
	game, err := h.service.GetGame(ctx, req.GameId)
	if err != nil {
		h.log.Error("GetGame failed", zap.String("game_id", req.GameId), zap.Error(err))
		return nil, status.Error(codes.NotFound, "game not found")
	}

	return &pb.GetGameResponse{
		Game: toProtoGame(game),
	}, nil
}

// LaunchGame launches a game session
func (h *CasinoGRPCHandler) LaunchGame(ctx context.Context, req *pb.LaunchGameRequest) (*pb.LaunchGameResponse, error) {
	if req.UserId == nil || req.UserId.Value == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	serviceReq := &service.LaunchGameRequest{
		UserID:     uint64(req.UserId.Value),
		GameID:     req.GameId,
		DeviceType: req.DeviceType,
		LobbyURL:   req.LobbyUrl,
	}

	result, err := h.service.LaunchGame(ctx, serviceReq)
	if err != nil {
		h.log.Error("LaunchGame failed", zap.Error(err))
		return &pb.LaunchGameResponse{
			Error: &commonv1.ErrorDetails{
				Code:    "LAUNCH_FAILED",
				Message: err.Error(),
			},
		}, nil
	}

	return &pb.LaunchGameResponse{
		Session:   toProtoGameSession(result.Session),
		LaunchUrl: result.LaunchURL,
		Token:     result.Token,
	}, nil
}

// GetGameSession returns game session details
func (h *CasinoGRPCHandler) GetGameSession(ctx context.Context, req *pb.GetGameSessionRequest) (*pb.GetGameSessionResponse, error) {
	session, err := h.service.GetGameSession(ctx, req.SessionId)
	if err != nil {
		h.log.Error("GetGameSession failed", zap.String("session_id", req.SessionId), zap.Error(err))
		return nil, status.Error(codes.NotFound, "session not found")
	}

	return &pb.GetGameSessionResponse{
		Session: toProtoGameSession(session),
	}, nil
}

// EndGameSession ends a game session
func (h *CasinoGRPCHandler) EndGameSession(ctx context.Context, req *pb.EndGameSessionRequest) (*pb.EndGameSessionResponse, error) {
	serviceReq := &service.EndGameSessionRequest{
		SessionID: req.SessionId,
	}

	result, err := h.service.EndGameSession(ctx, serviceReq)
	if err != nil {
		h.log.Error("EndGameSession failed", zap.String("session_id", req.SessionId), zap.Error(err))
		return &pb.EndGameSessionResponse{
			Success: false,
			Error: &commonv1.ErrorDetails{
				Code:    "END_SESSION_FAILED",
				Message: err.Error(),
			},
		}, nil
	}

	return &pb.EndGameSessionResponse{
		Success: result.Success,
		Summary: toProtoGameSessionSummary(result.Summary),
	}, nil
}

// GetGameHistory returns user's game history
func (h *CasinoGRPCHandler) GetGameHistory(ctx context.Context, req *pb.GetGameHistoryRequest) (*pb.GetGameHistoryResponse, error) {
	opts := service.GetGameHistoryOptions{
		UserID: req.UserId.Value,
		Limit:  req.Pagination.Limit,
		Offset: req.Pagination.Offset,
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

	sessions := make([]*pb.GameSessionSummary, len(result.Sessions))
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

	return &pb.GetGameHistoryResponse{
		Sessions: sessions,
		Pagination: &commonv1.PageResponse{
			Total:  result.TotalCount,
			Limit:  req.Pagination.Limit,
			Offset: req.Pagination.Offset,
		},
	}, nil
}

// GetRoundHistory returns round history for a game session
func (h *CasinoGRPCHandler) GetRoundHistory(ctx context.Context, req *pb.GetRoundHistoryRequest) (*pb.GetRoundHistoryResponse, error) {
	rounds, total, err := h.service.GetRoundHistory(ctx, req.SessionId, req.Pagination.Limit, req.Pagination.Offset)
	if err != nil {
		h.log.Error("GetRoundHistory failed", zap.String("session_id", req.SessionId), zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to get round history")
	}

	pbRounds := make([]*pb.GameRound, len(rounds))
	for i, round := range rounds {
		pbRounds[i] = toProtoGameRound(&round)
	}

	return &pb.GetRoundHistoryResponse{
		Rounds: pbRounds,
		Pagination: &commonv1.PageResponse{
			Total:  total,
			Limit:  req.Pagination.Limit,
			Offset: req.Pagination.Offset,
		},
	}, nil
}

// GetProviders returns game providers
func (h *CasinoGRPCHandler) GetProviders(ctx context.Context, req *pb.GetProvidersRequest) (*pb.GetProvidersResponse, error) {
	var isActive *bool
	if req.IsActive != nil {
		isActive = req.IsActive
	}

	providers, err := h.service.GetProviders(ctx, isActive)
	if err != nil {
		h.log.Error("GetProviders failed", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to get providers")
	}

	pbProviders := make([]*pb.Provider, len(providers))
	for i, provider := range providers {
		pbProviders[i] = toProtoProvider(&provider)
	}

	return &pb.GetProvidersResponse{
		Providers: pbProviders,
	}, nil
}

// GetProvider returns provider details
func (h *CasinoGRPCHandler) GetProvider(ctx context.Context, req *pb.GetProviderRequest) (*pb.GetProviderResponse, error) {
	provider, err := h.service.GetProvider(ctx, req.ProviderId)
	if err != nil {
		h.log.Error("GetProvider failed", zap.String("provider_id", req.ProviderId), zap.Error(err))
		return nil, status.Error(codes.NotFound, "provider not found")
	}

	return &pb.GetProviderResponse{
		Provider: toProtoProvider(provider),
	}, nil
}

// ============ Helper functions ============

func toProtoGame(game *service.Game) *pb.Game {
	return &pb.Game{
		Id:                  game.ID,
		Name:                game.Name,
		ProviderId:          game.ProviderID,
		ProviderName:        game.ProviderName,
		Category:            pb.GameCategory(pb.GameCategory_value[game.Category]),
		Tags:                game.Tags,
		Description:         game.Description,
		ImageUrl:            game.ImageURL,
		ThumbnailUrl:        game.ThumbnailURL,
		SupportedCurrencies: game.SupportedCurrencies,
		BetRange: &pb.MoneyRange{
			Min: &commonv1.Money{Value: game.MinBet},
			Max: &commonv1.Money{Value: game.MaxBet},
		},
		Features: &pb.GameFeatures{
			HasFreeSpins:   game.Features.HasFreeSpins,
			HasBonusBuy:    game.Features.HasBonusBuy,
			HasJackpot:     game.Features.HasJackpot,
			HasMultiplayer: game.Features.HasMultiplayer,
			HasLiveDealer:  game.Features.HasLiveDealer,
			BonusFeatures:  game.Features.BonusFeatures,
		},
		Stats: &pb.GameStats{
			Rtp:         game.RTP,
			Volatility:  game.Volatility,
			TotalPlays:  0,
			TotalPaid:   nil,
			BiggestWin:  nil,
		},
		IsActive:           game.IsActive,
		IsDemoAvailable:    game.IsDemoAvailable,
		RestrictedCountries: game.RestrictedCountries,
		PopularityScore:    game.PopularityScore,
		ReleasedAt:         timestamppb.New(game.ReleasedAt),
		Metadata:           game.Metadata,
	}
}

func toProtoProvider(provider *service.Provider) *pb.Provider {
	return &pb.Provider{
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

func toProtoGameSession(session *service.GameSession) *pb.GameSession {
	return &pb.GameSession{
		Id:         session.ID,
		UserId:     &commonv1.UserId{Value: int64(session.UserID)},
		GameId:     session.GameID,
		ProviderId: session.ProviderID,
		Status:     pb.GameSessionStatus(pb.GameSessionStatus_value[session.Status]),
		BalanceAtStart: &commonv1.Money{Value: session.BalanceAtStart},
		StartedAt:  timestamppb.New(session.StartedAt),
		LastActivity: timestamppb.New(session.LastActivity),
		EndedAt:    nil,
		DeviceType: session.DeviceType,
		LobbyUrl:   session.LobbyURL,
		LaunchUrl:  session.LaunchURL,
		Token:      session.Token,
		Metadata:   session.Metadata,
	}
}

func toProtoGameSessionSummary(summary *service.GameSessionSummary) *pb.GameSessionSummary {
	return &pb.GameSessionSummary{
		SessionId:    summary.SessionID,
		GameId:       summary.GameID,
		GameName:     summary.GameName,
		TotalBet:     &commonv1.Money{Value: summary.TotalBet},
		TotalWin:     &commonv1.Money{Value: summary.TotalWin},
		NetResult:    &commonv1.Money{Value: summary.NetResult},
		RoundsPlayed: summary.RoundsPlayed,
		StartedAt:    timestamppb.New(summary.StartedAt),
		EndedAt:      timestamppb.New(summary.EndedAt),
		DurationSeconds: summary.DurationSecs,
	}
}

func toProtoGameRound(round *service.GameRound) *pb.GameRound {
	return &pb.GameRound{
		Id:         round.ID,
		SessionId:  round.SessionID,
		RoundId:    round.RoundID,
		BetAmount:  &commonv1.Money{Value: round.BetAmount},
		WinAmount:  &commonv1.Money{Value: round.WinAmount},
		NetResult:  &commonv1.Money{Value: round.NetResult},
		Status:     pb.GameRoundStatus(pb.GameRoundStatus_value[round.Status]),
		StartedAt:  timestamppb.New(round.StartedAt),
		EndedAt:    timestamppb.New(round.EndedAt),
		GameState:  round.GameState,
		Metadata:   round.Metadata,
	}
}
