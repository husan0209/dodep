package handlers

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opus-casino/proto/gen/go/bonus/v1"
	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"

	"github.com/opus-casino/bonus/internal/domain"
	"github.com/opus-casino/bonus/internal/service"
)

// BonusGRPCHandler implements the BonusService gRPC contract.
// It converts between proto and domain types at the boundary and
// delegates all business logic to the service layer.
type BonusGRPCHandler struct {
	pb.UnimplementedBonusServiceServer
	svc *service.BonusService
	log *zap.Logger
}

// NewBonusGRPCHandler creates a new gRPC handler.
func NewBonusGRPCHandler(svc *service.BonusService, log *zap.Logger) *BonusGRPCHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &BonusGRPCHandler{svc: svc, log: log}
}

func parseUserID(v *commonv1.UserId) (int64, error) {
	if v == nil || v.Value == "" {
		return 0, status.Error(codes.InvalidArgument, "user_id is required")
	}
	id, err := strconv.ParseInt(v.Value, 10, 64)
	if err != nil || id <= 0 {
		return 0, status.Error(codes.InvalidArgument, "user_id must be a positive integer")
	}
	return id, nil
}

func moneyOf(amount decimal.Decimal, currency string) *commonv1.Money {
	return &commonv1.Money{Amount: amount.String(), Currency: currency}
}

func toProtoBonus(b *domain.Bonus) *pb.UserBonus {
	required := moneyOf(b.WageringRequired, b.Currency)
	wagered := moneyOf(b.WageringCompleted, b.Currency)
	progress := 0.0
	if !b.WageringRequired.IsZero() {
		p, _ := b.WageringCompleted.Div(b.WageringRequired).Mul(decimal.NewFromInt(100)).Float64()
		if p > 100 {
			p = 100
		}
		progress = p
	}
	out := &pb.UserBonus{
		Id:              b.ID.String(),
		UserId:          &commonv1.UserId{Value: fmt.Sprintf("%d", b.UserID)},
		Name:            string(b.Type) + " bonus",
		BonusType:       domainBonusTypeToProto(b.Type),
		Status:          domainBonusStatusToProto(b.Status),
		BonusAmount:     moneyOf(b.BonusAmount, b.Currency),
		RemainingAmount: moneyOf(b.RemainingWagering(), b.Currency),
		Wagering: &pb.WageringProgress{
			BonusId:            b.ID.String(),
			RequiredAmount:     required,
			WageredAmount:      wagered,
			ProgressPercentage: progress,
			IsCompleted:        b.IsWageringComplete(),
			UpdatedAt:          timestamppb.New(b.UpdatedAt),
		},
		ActivatedAt: timestamppb.New(b.CreatedAt),
		ExpiresAt:   timestamppb.New(b.ExpiresAt),
	}
	if b.ActivatedAt != nil {
		out.ActivatedAt = timestamppb.New(*b.ActivatedAt)
	}
	if b.CompletedAt != nil {
		out.CompletedAt = timestamppb.New(*b.CompletedAt)
	}
	return out
}

func domainBonusTypeToProto(t domain.BonusType) pb.BonusType {
	switch t {
	case domain.BonusTypeWelcome:
		return pb.BonusType_BONUS_TYPE_WELCOME
	case domain.BonusTypeReload:
		return pb.BonusType_BONUS_TYPE_RELOAD
	case domain.BonusTypeCashback:
		return pb.BonusType_BONUS_TYPE_CASHBACK
	case domain.BonusTypeFreeSpins:
		return pb.BonusType_BONUS_TYPE_FREE_SPINS
	default:
		return pb.BonusType_BONUS_TYPE_UNSPECIFIED
	}
}

func domainBonusStatusToProto(s domain.BonusStatus) pb.BonusStatus {
	switch s {
	case domain.BonusStatusActive:
		return pb.BonusStatus_BONUS_STATUS_ACTIVE
	case domain.BonusStatusCompleted:
		return pb.BonusStatus_BONUS_STATUS_COMPLETED
	case domain.BonusStatusExpired:
		return pb.BonusStatus_BONUS_STATUS_EXPIRED
	case domain.BonusStatusCancelled:
		return pb.BonusStatus_BONUS_STATUS_CANCELLED
	default:
		return pb.BonusStatus_BONUS_STATUS_UNSPECIFIED
	}
}

func mapDomainError(err error) error {
	switch {
	case errors.Is(err, domain.ErrBonusNotFound):
		return status.Error(codes.NotFound, "bonus not found")
	case errors.Is(err, domain.ErrBonusNotActive):
		return status.Error(codes.FailedPrecondition, "bonus is not active")
	case errors.Is(err, domain.ErrInvalidBonusAmount):
		return status.Error(codes.InvalidArgument, "invalid bonus amount")
	case errors.Is(err, domain.ErrInvalidPromoCode):
		return status.Error(codes.InvalidArgument, "invalid promo code")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}

// GetActiveBonuses returns the user's active bonus (0 or 1 in current model).
func (h *BonusGRPCHandler) GetActiveBonuses(ctx context.Context, req *pb.GetActiveBonusesRequest) (*pb.GetActiveBonusesResponse, error) {
	userID, err := parseUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	bonus, err := h.svc.GetActiveBonus(ctx, userID)
	if err != nil {
		h.log.Error("GetActiveBonuses failed", zap.Error(err), zap.Int64("user_id", userID))
		return nil, mapDomainError(err)
	}
	resp := &pb.GetActiveBonusesResponse{}
	if bonus != nil {
		resp.Bonuses = []*pb.UserBonus{toProtoBonus(bonus)}
	}
	return resp, nil
}

// GetBonus returns a specific bonus.
func (h *BonusGRPCHandler) GetBonus(ctx context.Context, req *pb.GetBonusRequest) (*pb.GetBonusResponse, error) {
	userID, err := parseUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	bonusID, err := uuid.Parse(req.GetBonusId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "bonus_id must be UUID")
	}
	bonus, err := h.svc.GetBonus(ctx, userID, bonusID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return &pb.GetBonusResponse{Bonus: toProtoBonus(bonus)}, nil
}

// GetWageringProgress returns wagering progress for a bonus.
func (h *BonusGRPCHandler) GetWageringProgress(ctx context.Context, req *pb.GetWageringProgressRequest) (*pb.GetWageringProgressResponse, error) {
	userID, err := parseUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	bonusID, err := uuid.Parse(req.GetBonusId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "bonus_id must be UUID")
	}
	bonus, err := h.svc.GetBonus(ctx, userID, bonusID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return &pb.GetWageringProgressResponse{Progress: toProtoBonus(bonus).GetWagering()}, nil
}

// ActivateBonus activates a pending bonus.
func (h *BonusGRPCHandler) ActivateBonus(ctx context.Context, req *pb.ActivateBonusRequest) (*pb.ActivateBonusResponse, error) {
	userID, err := parseUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	bonusID, err := uuid.Parse(req.GetBonusId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "bonus_id must be UUID")
	}
	bonus, err := h.svc.ActivateBonus(ctx, userID, bonusID)
	if err != nil {
		return &pb.ActivateBonusResponse{
			Error: &commonv1.ErrorDetails{ErrorMessage: err.Error()},
		}, nil
	}
	return &pb.ActivateBonusResponse{Bonus: toProtoBonus(bonus)}, nil
}

// GetAvailableBonuses returns the currently claimable welcome offer template.
func (h *BonusGRPCHandler) GetAvailableBonuses(ctx context.Context, req *pb.GetAvailableBonusesRequest) (*pb.GetAvailableBonusesResponse, error) {
	userID, err := parseUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	existing, err := h.svc.GetActiveBonus(ctx, userID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	if existing != nil {
		return &pb.GetAvailableBonusesResponse{}, nil
	}
	return &pb.GetAvailableBonusesResponse{
		Bonuses: []*pb.AvailableBonus{
			{
				Id:                  "welcome-100",
				Name:                "Welcome Bonus 100%",
				Description:         "100% match on first deposit up to policy max",
				BonusType:           pb.BonusType_BONUS_TYPE_WELCOME,
				WageringRequirement: "30x",
				RequiresDeposit:     true,
			},
		},
	}, nil
}

// ClaimFreeSpins is not supported in the current bonus model.
func (h *BonusGRPCHandler) ClaimFreeSpins(ctx context.Context, req *pb.ClaimFreeSpinsRequest) (*pb.ClaimFreeSpinsResponse, error) {
	return &pb.ClaimFreeSpinsResponse{
		Error: &commonv1.ErrorDetails{ErrorMessage: "free spins are not supported in this release"},
	}, nil
}

// GetPromotions returns an empty promotion list (no promotion engine in MVP).
func (h *BonusGRPCHandler) GetPromotions(ctx context.Context, req *pb.GetPromotionsRequest) (*pb.GetPromotionsResponse, error) {
	return &pb.GetPromotionsResponse{}, nil
}

// GetPromotion returns not found (no promotion engine in MVP).
func (h *BonusGRPCHandler) GetPromotion(ctx context.Context, req *pb.GetPromotionRequest) (*pb.GetPromotionResponse, error) {
	return nil, status.Error(codes.NotFound, "promotion not found")
}
