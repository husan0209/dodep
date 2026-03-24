package handlers

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opus-casino/proto/gen/go/notification/v1"
	"github.com/opus-casino/notification/internal/service"
	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"
)

// NotificationGRPCHandler handles gRPC requests for Notification Service
type NotificationGRPCHandler struct {
	pb.UnimplementedNotificationServiceServer
	service *service.NotificationService
	log     *zap.Logger
}

// NewNotificationGRPCHandler creates a new gRPC handler
func NewNotificationGRPCHandler(service *service.NotificationService) *NotificationGRPCHandler {
	return &NotificationGRPCHandler{
		service: service,
	}
}

// SendNotification sends a notification to user
func (h *NotificationGRPCHandler) SendNotification(ctx context.Context, req *pb.SendNotificationRequest) (*pb.SendNotificationResponse, error) {
	if req.UserId == nil || req.UserId.Value == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	serviceReq := &service.SendNotificationRequest{
		UserID:      uint64(req.UserId.Value),
		Channel:     req.Channel.String(),
		Type:        req.Type.String(),
		Subject:     req.Subject,
		Message:     req.Message,
		Data:        req.Data,
		TemplateID:  req.TemplateId,
		Priority:    req.Priority.String(),
		ReferenceID: req.ReferenceId,
	}

	if req.SendAt != nil {
		sendAt := req.SendAt.AsTime()
		serviceReq.SendAt = &sendAt
	}

	result, err := h.service.SendNotification(ctx, serviceReq)
	if err != nil {
		h.log.Error("SendNotification failed", zap.Error(err))
		return &pb.SendNotificationResponse{
			Error: &commonv1.ErrorDetails{
				Code:    "SEND_FAILED",
				Message: err.Error(),
			},
		}, nil
	}

	var errorDetails *commonv1.ErrorDetails
	if result.Error != nil {
		errorDetails = &commonv1.ErrorDetails{
			Code:    "SEND_FAILED",
			Message: result.Error.Error(),
		}
	}

	return &pb.SendNotificationResponse{
		NotificationId: result.NotificationID,
		Error:          errorDetails,
	}, nil
}

// SendBulkNotification sends notifications to multiple users
func (h *NotificationGRPCHandler) SendBulkNotification(ctx context.Context, req *pb.SendBulkNotificationRequest) (*pb.SendBulkNotificationResponse, error) {
	userIDs := make([]uint64, len(req.UserIds))
	for i, id := range req.UserIds {
		userIDs[i] = uint64(id.Value)
	}

	var userSegment *string
	if req.UserSegment != nil {
		userSegment = req.UserSegment
	}

	serviceReq := &service.SendBulkNotificationRequest{
		UserIDs:     userIDs,
		UserSegment: userSegment,
		Channel:     req.Channel.String(),
		Type:        req.Type.String(),
		Subject:     req.Subject,
		Message:     req.Message,
		Data:        req.Data,
		TemplateID:  req.TemplateId,
		Priority:    req.Priority.String(),
	}

	result, err := h.service.SendBulkNotification(ctx, serviceReq)
	if err != nil {
		h.log.Error("SendBulkNotification failed", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to send bulk notifications")
	}

	return &pb.SendBulkNotificationResponse{
		QueuedCount: result.QueuedCount,
		FailedCount: result.FailedCount,
		BatchId:     result.BatchID,
	}, nil
}

// GetNotification returns notification details
func (h *NotificationGRPCHandler) GetNotification(ctx context.Context, req *pb.GetNotificationRequest) (*pb.GetNotificationResponse, error) {
	notif, err := h.service.GetNotification(ctx, req.NotificationId)
	if err != nil {
		h.log.Error("GetNotification failed", zap.String("notification_id", req.NotificationId), zap.Error(err))
		return nil, status.Error(codes.NotFound, "notification not found")
	}

	return &pb.GetNotificationResponse{
		Notification: toProtoNotification(notif),
	}, nil
}

// GetUserNotifications returns user's notifications
func (h *NotificationGRPCHandler) GetUserNotifications(ctx context.Context, req *pb.GetUserNotificationsRequest) (*pb.GetUserNotificationsResponse, error) {
	if req.UserId == nil || req.UserId.Value == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	serviceReq := &service.GetUserNotificationsRequest{
		UserID:   uint64(req.UserId.Value),
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
	}

	if req.Type != nil {
		typeStr := req.Type.String()
		serviceReq.Type = &typeStr
	}
	if req.IsRead != nil {
		serviceReq.IsRead = req.IsRead
	}
	if req.DateRange != nil {
		from := req.DateRange.From.AsTime()
		to := req.DateRange.To.AsTime()
		serviceReq.DateFrom = &from
		serviceReq.DateTo = &to
	}

	result, err := h.service.GetUserNotifications(ctx, serviceReq)
	if err != nil {
		h.log.Error("GetUserNotifications failed", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to get user notifications")
	}

	notifications := make([]*pb.Notification, len(result.Notifications))
	for i, notif := range result.Notifications {
		notifications[i] = toProtoNotification(&notif)
	}

	return &pb.GetUserNotificationsResponse{
		Notifications: notifications,
		Pagination: &commonv1.PageResponse{
			Total:  result.TotalCount,
			Limit:  req.Pagination.Limit,
			Offset: req.Pagination.Offset,
		},
		UnreadCount: result.UnreadCount,
	}, nil
}

// MarkAsRead marks notification as read
func (h *NotificationGRPCHandler) MarkAsRead(ctx context.Context, req *pb.MarkAsReadRequest) (*pb.MarkAsReadResponse, error) {
	if req.UserId == nil || req.UserId.Value == 0 {
		return &pb.MarkAsReadResponse{
			Success: false,
			Error: &commonv1.ErrorDetails{
				Code:    "INVALID_ARGUMENT",
				Message: "user_id is required",
			},
		}, nil
	}

	err := h.service.MarkAsRead(ctx, req.NotificationId, uint64(req.UserId.Value))
	if err != nil {
		h.log.Error("MarkAsRead failed", zap.Error(err))
		return &pb.MarkAsReadResponse{
			Success: false,
			Error: &commonv1.ErrorDetails{
				Code:    "MARK_READ_FAILED",
				Message: err.Error(),
			},
		}, nil
	}

	return &pb.MarkAsReadResponse{
		Success: true,
	}, nil
}

// MarkAllAsRead marks all user notifications as read
func (h *NotificationGRPCHandler) MarkAllAsRead(ctx context.Context, req *pb.MarkAllAsReadRequest) (*pb.MarkAllAsReadResponse, error) {
	if req.UserId == nil || req.UserId.Value == 0 {
		return &pb.MarkAllAsReadResponse{
			MarkedCount: 0,
		}, nil
	}

	var typeFilter *string
	if req.Type != nil {
		typeStr := req.Type.String()
		typeFilter = &typeStr
	}

	count, err := h.service.MarkAllAsRead(ctx, uint64(req.UserId.Value), typeFilter)
	if err != nil {
		h.log.Error("MarkAllAsRead failed", zap.Error(err))
		return &pb.MarkAllAsReadResponse{
			MarkedCount: 0,
		}, nil
	}

	return &pb.MarkAllAsReadResponse{
		MarkedCount: count,
	}, nil
}

// DeleteNotification deletes a notification
func (h *NotificationGRPCHandler) DeleteNotification(ctx context.Context, req *pb.DeleteNotificationRequest) (*pb.DeleteNotificationResponse, error) {
	if req.UserId == nil || req.UserId.Value == 0 {
		return &pb.DeleteNotificationResponse{
			Success: false,
			Error: &commonv1.ErrorDetails{
				Code:    "INVALID_ARGUMENT",
				Message: "user_id is required",
			},
		}, nil
	}

	err := h.service.DeleteNotification(ctx, req.NotificationId, uint64(req.UserId.Value))
	if err != nil {
		h.log.Error("DeleteNotification failed", zap.Error(err))
		return &pb.DeleteNotificationResponse{
			Success: false,
			Error: &commonv1.ErrorDetails{
				Code:    "DELETE_FAILED",
				Message: err.Error(),
			},
		}, nil
	}

	return &pb.DeleteNotificationResponse{
		Success: true,
	}, nil
}

// GetNotificationSettings returns user's notification settings
func (h *NotificationGRPCHandler) GetNotificationSettings(ctx context.Context, req *pb.GetNotificationSettingsRequest) (*pb.GetNotificationSettingsResponse, error) {
	if req.UserId == nil || req.UserId.Value == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	settings, err := h.service.GetNotificationSettings(ctx, uint64(req.UserId.Value))
	if err != nil {
		h.log.Error("GetNotificationSettings failed", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to get notification settings")
	}

	return &pb.GetNotificationSettingsResponse{
		Settings: toProtoNotificationSettings(settings),
	}, nil
}

// UpdateNotificationSettings updates user's notification settings
func (h *NotificationGRPCHandler) UpdateNotificationSettings(ctx context.Context, req *pb.UpdateNotificationSettingsRequest) (*pb.UpdateNotificationSettingsResponse, error) {
	if req.UserId == nil || req.UserId.Value == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	serviceReq := &service.UpdateNotificationSettingsRequest{
		UserID:       uint64(req.UserId.Value),
		EmailEnabled: req.EmailEnabled,
		SMSEnabled:   req.SmsEnabled,
		PushEnabled:  req.PushEnabled,
		InAppEnabled: req.InAppEnabled,
		TypePreferences: make(map[string]service.ChannelPreferences),
	}

	// Convert type preferences
	for typeStr, pref := range req.TypePreferences {
		serviceReq.TypePreferences[typeStr] = service.ChannelPreferences{
			EmailEnabled: pref.EmailEnabled,
			SMSEnabled:   pref.SmsEnabled,
			PushEnabled:  pref.PushEnabled,
			InAppEnabled: pref.InAppEnabled,
		}
	}

	settings, err := h.service.UpdateNotificationSettings(ctx, serviceReq)
	if err != nil {
		h.log.Error("UpdateNotificationSettings failed", zap.Error(err))
		return &pb.UpdateNotificationSettingsResponse{
			Error: &commonv1.ErrorDetails{
				Code:    "UPDATE_FAILED",
				Message: err.Error(),
			},
		}, nil
	}

	return &pb.UpdateNotificationSettingsResponse{
		Settings: toProtoNotificationSettings(settings),
	}, nil
}

// ============ Helper functions ============

func toProtoNotification(notif *service.Notification) *pb.Notification {
	return &pb.Notification{
		Id:         notif.ID,
		UserId:     &commonv1.UserId{Value: int64(notif.UserID)},
		Channel:    pb.NotificationChannel(pb.NotificationChannel_value[notif.Channel]),
		Type:       pb.NotificationType(pb.NotificationType_value[notif.Type]),
		Priority:   pb.NotificationPriority(pb.NotificationPriority_value[notif.Priority]),
		Subject:    notif.Subject,
		Message:    notif.Message,
		Data:       notif.Data,
		IsRead:     notif.IsRead,
		CreatedAt:  timestamppb.New(notif.CreatedAt),
		ReadAt:     nil,
		SentAt:     nil,
		Status:     pb.NotificationStatus(pb.NotificationStatus_value[notif.Status]),
		ErrorMessage: notif.ErrorMessage,
		ReferenceId:  notif.ReferenceID,
		Metadata:     notif.Metadata,
	}
}

func toProtoNotificationSettings(settings *service.NotificationSettings) *pb.NotificationSettings {
	typePrefs := make(map[pb.NotificationType]*pb.ChannelPreferences)
	for typeStr, pref := range settings.TypePreferences {
		typePrefs[pb.NotificationType(pb.NotificationType_value[typeStr])] = &pb.ChannelPreferences{
			EmailEnabled: pref.EmailEnabled,
			SmsEnabled:   pref.SMSEnabled,
			PushEnabled:  pref.PushEnabled,
			InAppEnabled: pref.InAppEnabled,
		}
	}

	return &pb.NotificationSettings{
		UserId:         &commonv1.UserId{Value: int64(settings.UserID)},
		EmailEnabled:   settings.EmailEnabled,
		SmsEnabled:     settings.SMSEnabled,
		PushEnabled:    settings.PushEnabled,
		InAppEnabled:   settings.InAppEnabled,
		TypePreferences: typePrefs,
		UpdatedAt:      timestamppb.New(settings.UpdatedAt),
	}
}
