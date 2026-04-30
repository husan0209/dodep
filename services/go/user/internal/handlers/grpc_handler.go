package handlers

import (
	"context"
	"fmt"
	"strconv"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"
	pb "github.com/opus-casino/proto/gen/go/user/v1"

	"github.com/opus-casino/user/internal/domain"
	"github.com/opus-casino/user/internal/service"
)

// parseUID converts a string user ID to int64.
func parseUID(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

// ptr returns a pointer to the given int64 value (for optional PageResponse.TotalCount).
func ptr(v int64) *int64 { return &v }

type UserGRPCHandler struct {
	pb.UnimplementedUserServiceServer
	service *service.UserService
	log     *zap.Logger
}

func NewUserGRPCHandler(svc *service.UserService, log *zap.Logger) *UserGRPCHandler {
	return &UserGRPCHandler{service: svc, log: log}
}

func (h *UserGRPCHandler) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	user, err := h.service.GetUser(ctx, parseUID(req.UserId.Value))
	if err != nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	return &pb.GetUserResponse{User: toProtoUser(user)}, nil
}

func (h *UserGRPCHandler) GetUserByEmail(ctx context.Context, req *pb.GetUserByEmailRequest) (*pb.GetUserByEmailResponse, error) {
	user, err := h.service.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return &pb.GetUserByEmailResponse{
			Error: &commonv1.ErrorDetails{ ErrorMessage: err.Error()},
		}, nil
	}
	return &pb.GetUserByEmailResponse{User: toProtoUser(user)}, nil
}

func (h *UserGRPCHandler) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UpdateUserResponse, error) {
	updateReq := &domain.UpdateUserRequest{UserID: parseUID(req.UserId.Value)}
	if req.Username != nil { updateReq.Username = req.Username }
	if req.FirstName != nil { updateReq.FirstName = req.FirstName }
	if req.LastName != nil { updateReq.LastName = req.LastName }
	if req.Phone != nil { updateReq.Phone = req.Phone }
	if req.DateOfBirth != nil { updateReq.DateOfBirth = req.DateOfBirth }
	if req.Address != nil { updateReq.Address = req.Address }
	if req.City != nil { updateReq.City = req.City }
	if req.PostalCode != nil { updateReq.PostalCode = req.PostalCode }
	if req.Language != nil { updateReq.Language = req.Language }
	if req.Timezone != nil { updateReq.Timezone = req.Timezone }

	user, err := h.service.UpdateUser(ctx, updateReq)
	if err != nil {
		return &pb.UpdateUserResponse{
			Error: &commonv1.ErrorDetails{ ErrorMessage: err.Error()},
		}, nil
	}
	return &pb.UpdateUserResponse{User: toProtoUser(user)}, nil
}

func (h *UserGRPCHandler) DeleteUser(ctx context.Context, req *pb.DeleteUserRequest) (*pb.DeleteUserResponse, error) {
	if err := h.service.DeleteUser(ctx, parseUID(req.UserId.Value), req.Reason); err != nil {
		return &pb.DeleteUserResponse{
			Success: false,
			Error:   &commonv1.ErrorDetails{ ErrorMessage: err.Error()},
		}, nil
	}
	return &pb.DeleteUserResponse{Success: true}, nil
}

func (h *UserGRPCHandler) GetPreferences(ctx context.Context, req *pb.GetPreferencesRequest) (*pb.GetPreferencesResponse, error) {
	pref, err := h.service.GetPreferences(ctx, parseUID(req.UserId.Value))
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get preferences")
	}
	return &pb.GetPreferencesResponse{Preferences: toProtoPreferences(pref)}, nil
}

func (h *UserGRPCHandler) UpdatePreferences(ctx context.Context, req *pb.UpdatePreferencesRequest) (*pb.UpdatePreferencesResponse, error) {
	pref := &domain.UserPreferences{UserID: parseUID(req.UserId.Value)}
	if req.Language != nil { pref.Language = *req.Language }
	if req.Timezone != nil { pref.Timezone = *req.Timezone }
	if req.CurrencyDisplay != nil { pref.CurrencyDisplay = *req.CurrencyDisplay }
	if req.MarketingEmails != nil { pref.MarketingEmails = *req.MarketingEmails }
	if req.SmsNotifications != nil { pref.SMSNotifications = *req.SmsNotifications }
	if req.PushNotifications != nil { pref.PushNotifications = *req.PushNotifications }
	if req.RealityCheck != nil { pref.RealityCheck = *req.RealityCheck }
	if req.RealityCheckIntervalMinutes != nil { pref.RealityCheckIntervalMinutes = int(*req.RealityCheckIntervalMinutes) }
	if req.AutoPlay != nil { pref.AutoPlay = *req.AutoPlay }
	if req.SoundPreference != nil { pref.SoundPreference = *req.SoundPreference }

	updated, err := h.service.UpdatePreferences(ctx, pref)
	if err != nil {
		return &pb.UpdatePreferencesResponse{
			Error: &commonv1.ErrorDetails{ ErrorMessage: err.Error()},
		}, nil
	}
	return &pb.UpdatePreferencesResponse{Preferences: toProtoPreferences(updated)}, nil
}

func (h *UserGRPCHandler) GetLimits(ctx context.Context, req *pb.GetLimitsRequest) (*pb.GetLimitsResponse, error) {
	limits, err := h.service.GetLimits(ctx, parseUID(req.UserId.Value))
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get limits")
	}
	return &pb.GetLimitsResponse{Limits: toProtoLimits(limits)}, nil
}

func (h *UserGRPCHandler) SetLimits(ctx context.Context, req *pb.SetLimitsRequest) (*pb.SetLimitsResponse, error) {
	setReq := &domain.SetLimitsRequest{UserID: parseUID(req.UserId.Value)}
	if req.DailyDepositLimit != nil {
		v := req.DailyDepositLimit.GetAmount().GetAmount()
		setReq.DailyDepositLimit = &v
	}
	if req.MonthlyDepositLimit != nil {
		v := req.MonthlyDepositLimit.GetAmount().GetAmount()
		setReq.MonthlyDepositLimit = &v
	}
	if req.DailyLossLimit != nil {
		v := req.DailyLossLimit.GetAmount().GetAmount()
		setReq.DailyLossLimit = &v
	}
	if req.SessionTimeLimit != nil {
		v := int(req.SessionTimeLimit.Minutes)
		setReq.SessionTimeMinutes = &v
	}
	if req.SelfExclusion != nil {
		setReq.SelfExclusion = req.SelfExclusion
	}
	if req.SelfExclusionUntil != nil {
		t := req.SelfExclusionUntil.AsTime()
		setReq.SelfExclusionUntil = &t
	}

	limits, err := h.service.SetLimits(ctx, setReq)
	if err != nil {
		return &pb.SetLimitsResponse{
			Error: &commonv1.ErrorDetails{ ErrorMessage: err.Error()},
		}, nil
	}
	return &pb.SetLimitsResponse{Limits: toProtoLimits(limits)}, nil
}

func (h *UserGRPCHandler) GetActivity(ctx context.Context, req *pb.GetActivityRequest) (*pb.GetActivityResponse, error) {
	limit := 20
	offset := 0
	if req.Pagination != nil {
		limit = int(req.Pagination.PageSize)
	}

	activities, total, err := h.service.GetActivity(ctx, parseUID(req.UserId.Value), limit, offset)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get activity")
	}

	var pbActivities []*pb.ActivityEntry
	for _, a := range activities {
		pbActivities = append(pbActivities, &pb.ActivityEntry{
			Id:      fmt.Sprintf("%v", a["id"]),
			UserId:  &commonv1.UserId{Value: req.UserId.Value},
			Description: fmt.Sprintf("%v", a["action"]),
		})
	}

	return &pb.GetActivityResponse{
		Activities: pbActivities,
		Pagination: &commonv1.PageResponse{TotalCount: ptr(int64(total))},
	}, nil
}

func toProtoUser(user *domain.User) *pb.User {
	return &pb.User{
		Id:          &commonv1.UserId{Value: fmt.Sprintf("%d", user.ID)},
		Email:       user.Email,
		Username:    user.Username,
		Country:     user.CountryCode,
		Currency:    user.CurrencyCode,
		Status:      pb.UserStatus(pb.UserStatus_value[string(user.Status)]),
		KycLevel:    pb.KycLevel(user.KYCLevel),
		CreatedAt:   timestamppb.New(user.CreatedAt),
		UpdatedAt:   timestamppb.New(user.UpdatedAt),
	}
}

func toProtoPreferences(pref *domain.UserPreferences) *pb.UserPreferences {
	return &pb.UserPreferences{
		UserId:                      &commonv1.UserId{Value: fmt.Sprintf("%d", pref.UserID)},
		Language:                    pref.Language,
		Timezone:                    pref.Timezone,
		CurrencyDisplay:             pref.CurrencyDisplay,
		MarketingEmails:             pref.MarketingEmails,
		SmsNotifications:            pref.SMSNotifications,
		PushNotifications:           pref.PushNotifications,
		RealityCheck:                pref.RealityCheck,
		RealityCheckIntervalMinutes: int32(pref.RealityCheckIntervalMinutes),
		AutoPlay:                    pref.AutoPlay,
		SoundPreference:             pref.SoundPreference,
		UpdatedAt:                   timestamppb.New(pref.UpdatedAt),
	}
}

func toProtoLimits(limits *domain.UserLimits) *pb.UserLimits {
	result := &pb.UserLimits{
		UserId:    &commonv1.UserId{Value: fmt.Sprintf("%d", limits.UserID)},
		UpdatedAt: timestamppb.New(limits.UpdatedAt),
	}
	if limits.SessionTimeLimit != nil {
		result.SessionTimeLimit = &pb.TimeLimit{
			Minutes:  int32(limits.SessionTimeLimit.Minutes),
			IsActive: limits.SessionTimeLimit.IsActive,
		}
	}
	return result
}


