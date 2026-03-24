package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/opus-casino/notification/internal/repository"
)

// NotificationService handles notification business logic
type NotificationService struct {
	repo *repository.NotificationRepository
	log  *zap.Logger
}

// NewNotificationService creates a new notification service
func NewNotificationService(repo *repository.NotificationRepository, log *zap.Logger) *NotificationService {
	return &NotificationService{
		repo: repo,
		log:  log,
	}
}

// Notification represents a notification
type Notification struct {
	ID           string                 `json:"id"`
	UserID       uint64                 `json:"user_id"`
	Channel      string                 `json:"channel"`
	Type         string                 `json:"type"`
	Priority     string                 `json:"priority"`
	Subject      string                 `json:"subject"`
	Message      string                 `json:"message"`
	Data         map[string]string      `json:"data"`
	IsRead       bool                   `json:"is_read"`
	CreatedAt    time.Time              `json:"created_at"`
	ReadAt       *time.Time             `json:"read_at"`
	SentAt       *time.Time             `json:"sent_at"`
	Status       string                 `json:"status"`
	ErrorMessage string                 `json:"error_message"`
	ReferenceID  string                 `json:"reference_id"`
	Metadata     map[string]string      `json:"metadata"`
}

// NotificationSettings represents user's notification settings
type NotificationSettings struct {
	UserID           uint64                        `json:"user_id"`
	EmailEnabled     bool                          `json:"email_enabled"`
	SMSEnabled       bool                          `json:"sms_enabled"`
	PushEnabled      bool                          `json:"push_enabled"`
	InAppEnabled     bool                          `json:"in_app_enabled"`
	TypePreferences  map[string]ChannelPreferences `json:"type_preferences"`
	UpdatedAt        time.Time                     `json:"updated_at"`
}

// ChannelPreferences represents channel preferences for a notification type
type ChannelPreferences struct {
	EmailEnabled bool `json:"email_enabled"`
	SMSEnabled   bool `json:"sms_enabled"`
	PushEnabled  bool `json:"push_enabled"`
	InAppEnabled bool `json:"in_app_enabled"`
}

// SendNotificationRequest represents a request to send a notification
type SendNotificationRequest struct {
	UserID     uint64
	Channel    string
	Type       string
	Subject    string
	Message    string
	Data       map[string]string
	TemplateID string
	Priority   string
	SendAt     *time.Time
	ReferenceID string
}

// SendNotificationResult represents the result of sending a notification
type SendNotificationResult struct {
	NotificationID string
	Error          error
}

// SendNotification sends a notification to a user
func (s *NotificationService) SendNotification(ctx context.Context, req *SendNotificationRequest) (*SendNotificationResult, error) {
	notif := &repository.Notification{
		ID:           uuid.New().String(),
		UserID:       req.UserID,
		Channel:      req.Channel,
		Type:         req.Type,
		Priority:     req.Priority,
		Subject:      req.Subject,
		Message:      req.Message,
		Data:         req.Data,
		IsRead:       false,
		CreatedAt:    time.Now(),
		Status:       "pending",
		ReferenceID:  req.ReferenceID,
		Metadata:     make(map[string]string),
	}

	// Check if user wants this type of notification
	settings, err := s.repo.GetNotificationSettings(ctx, req.UserID)
	if err != nil {
		s.log.Error("Failed to get notification settings", zap.Uint64("user_id", req.UserID), zap.Error(err))
	}

	// Check if channel is enabled
	if !s.isChannelEnabled(settings, req.Channel, req.Type) {
		s.log.Info("Notification channel disabled for user",
			zap.Uint64("user_id", req.UserID),
			zap.String("channel", req.Channel))
		return &SendNotificationResult{
			NotificationID: notif.ID,
			Error:          errors.New("notification channel disabled"),
		}, nil
	}

	// Create notification in database
	if err := s.repo.CreateNotification(ctx, notif); err != nil {
		s.log.Error("Failed to create notification", zap.Error(err))
		return nil, err
	}

	// Send notification based on channel
	var sendErr error
	switch req.Channel {
	case "email":
		sendErr = s.sendEmail(ctx, notif)
	case "sms":
		sendErr = s.sendSMS(ctx, notif)
	case "push":
		sendErr = s.sendPush(ctx, notif)
	case "in_app":
		sendErr = s.sendInApp(ctx, notif)
	default:
		sendErr = errors.New("unknown notification channel")
	}

	// Update notification status
	if sendErr != nil {
		notif.Status = "failed"
		notif.ErrorMessage = sendErr.Error()
		s.log.Error("Failed to send notification",
			zap.String("notification_id", notif.ID),
			zap.String("channel", req.Channel),
			zap.Error(sendErr))
	} else {
		notif.Status = "sent"
		now := time.Now()
		notif.SentAt = &now
		s.log.Info("Notification sent",
			zap.String("notification_id", notif.ID),
			zap.String("channel", req.Channel),
			zap.Uint64("user_id", req.UserID))
	}

	// Update status in database
	if err := s.repo.UpdateNotificationStatus(ctx, notif.ID, notif.Status, notif.ErrorMessage); err != nil {
		s.log.Error("Failed to update notification status", zap.Error(err))
	}

	// Increment unread count for in-app notifications
	if req.Channel == "in_app" && sendErr == nil {
		if err := s.repo.IncrementUnreadCount(ctx, req.UserID); err != nil {
			s.log.Error("Failed to increment unread count", zap.Error(err))
		}
	}

	return &SendNotificationResult{
		NotificationID: notif.ID,
		Error:          sendErr,
	}, nil
}

// SendBulkNotificationRequest represents a request to send bulk notifications
type SendBulkNotificationRequest struct {
	UserIDs      []uint64
	UserSegment  *string
	Channel      string
	Type         string
	Subject      string
	Message      string
	Data         map[string]string
	TemplateID   string
	Priority     string
}

// SendBulkNotificationResult represents the result of sending bulk notifications
type SendBulkNotificationResult struct {
	QueuedCount int32
	FailedCount int32
	BatchID     string
}

// SendBulkNotification sends notifications to multiple users
func (s *NotificationService) SendBulkNotification(ctx context.Context, req *SendBulkNotificationRequest) (*SendBulkNotificationResult, error) {
	batchID := uuid.New().String()
	queuedCount := int32(0)
	failedCount := int32(0)

	for _, userID := range req.UserIDs {
		sendReq := &SendNotificationRequest{
			UserID:     userID,
			Channel:    req.Channel,
			Type:       req.Type,
			Subject:    req.Subject,
			Message:    req.Message,
			Data:       req.Data,
			TemplateID: req.TemplateID,
			Priority:   req.Priority,
		}

		result, err := s.SendNotification(ctx, sendReq)
		if err != nil || result.Error != nil {
			failedCount++
		} else {
			queuedCount++
		}
	}

	return &SendBulkNotificationResult{
		QueuedCount: queuedCount,
		FailedCount: failedCount,
		BatchID:     batchID,
	}, nil
}

// GetNotification returns a notification by ID
func (s *NotificationService) GetNotification(ctx context.Context, id string) (*Notification, error) {
	notif, err := s.repo.GetNotification(ctx, id)
	if err != nil {
		s.log.Error("Failed to get notification", zap.String("notification_id", id), zap.Error(err))
		return nil, err
	}

	return toNotification(notif), nil
}

// GetUserNotificationsRequest represents a request to get user notifications
type GetUserNotificationsRequest struct {
	UserID     uint64
	Type       *string
	IsRead     *bool
	DateFrom   *time.Time
	DateTo     *time.Time
	Limit      int32
	Offset     int32
}

// GetUserNotificationsResult represents the result of getting user notifications
type GetUserNotificationsResult struct {
	Notifications []Notification
	TotalCount    int64
	UnreadCount   int32
}

// GetUserNotifications returns user's notifications
func (s *NotificationService) GetUserNotifications(ctx context.Context, req *GetUserNotificationsRequest) (*GetUserNotificationsResult, error) {
	notifications, total, err := s.repo.GetUserNotifications(
		ctx, req.UserID, req.Type, req.IsRead, req.DateFrom, req.DateTo, req.Limit, req.Offset)
	if err != nil {
		s.log.Error("Failed to get user notifications", zap.Error(err))
		return nil, err
	}

	result := make([]Notification, len(notifications))
	for i, notif := range notifications {
		result[i] = *toNotification(&notif)
	}

	// Get unread count
	unreadCount, err := s.repo.GetUnreadCount(ctx, req.UserID)
	if err != nil {
		s.log.Error("Failed to get unread count", zap.Error(err))
	}

	return &GetUserNotificationsResult{
		Notifications: result,
		TotalCount:    total,
		UnreadCount:   unreadCount,
	}, nil
}

// MarkAsRead marks a notification as read
func (s *NotificationService) MarkAsRead(ctx context.Context, id string, userID uint64) error {
	if err := s.repo.MarkAsRead(ctx, id, userID); err != nil {
		s.log.Error("Failed to mark notification as read", zap.Error(err))
		return err
	}

	// Decrement unread count
	if err := s.repo.IncrementUnreadCount(ctx, userID); err != nil {
		s.log.Error("Failed to decrement unread count", zap.Error(err))
	}

	s.log.Info("Notification marked as read", zap.String("notification_id", id))
	return nil
}

// MarkAllAsRead marks all user notifications as read
func (s *NotificationService) MarkAllAsRead(ctx context.Context, userID uint64, typeFilter *string) (int32, error) {
	count, err := s.repo.MarkAllAsRead(ctx, userID, typeFilter)
	if err != nil {
		s.log.Error("Failed to mark all notifications as read", zap.Error(err))
		return 0, err
	}

	// Reset unread count
	if err := s.repo.SetUnreadCount(ctx, userID, 0); err != nil {
		s.log.Error("Failed to reset unread count", zap.Error(err))
	}

	s.log.Info("All notifications marked as read", zap.Uint64("user_id", userID), zap.Int32("count", count))
	return count, nil
}

// DeleteNotification deletes a notification
func (s *NotificationService) DeleteNotification(ctx context.Context, id string, userID uint64) error {
	if err := s.repo.DeleteNotification(ctx, id, userID); err != nil {
		s.log.Error("Failed to delete notification", zap.Error(err))
		return err
	}

	s.log.Info("Notification deleted", zap.String("notification_id", id))
	return nil
}

// GetNotificationSettings returns user's notification settings
func (s *NotificationService) GetNotificationSettings(ctx context.Context, userID uint64) (*NotificationSettings, error) {
	settings, err := s.repo.GetNotificationSettings(ctx, userID)
	if err != nil {
		s.log.Error("Failed to get notification settings", zap.Error(err))
		return nil, err
	}

	return toNotificationSettings(settings), nil
}

// UpdateNotificationSettingsRequest represents a request to update notification settings
type UpdateNotificationSettingsRequest struct {
	UserID          uint64
	EmailEnabled    *bool
	SMSEnabled      *bool
	PushEnabled     *bool
	InAppEnabled    *bool
	TypePreferences map[string]ChannelPreferences
}

// UpdateNotificationSettings updates user's notification settings
func (s *NotificationService) UpdateNotificationSettings(ctx context.Context, req *UpdateNotificationSettingsRequest) (*NotificationSettings, error) {
	settings := &repository.NotificationSettings{
		UserID:           req.UserID,
		EmailEnabled:     getBoolOrDefault(req.EmailEnabled, true),
		SMSEnabled:       getBoolOrDefault(req.SMSEnabled, false),
		PushEnabled:      getBoolOrDefault(req.PushEnabled, true),
		InAppEnabled:     getBoolOrDefault(req.InAppEnabled, true),
		TypePreferences:  make(map[string]repository.ChannelPreferences),
		UpdatedAt:        time.Now(),
	}

	// Convert type preferences
	for t, pref := range req.TypePreferences {
		settings.TypePreferences[t] = repository.ChannelPreferences{
			EmailEnabled: pref.EmailEnabled,
			SMSEnabled:   pref.SMSEnabled,
			PushEnabled:  pref.PushEnabled,
			InAppEnabled: pref.InAppEnabled,
		}
	}

	if err := s.repo.UpdateNotificationSettings(ctx, settings); err != nil {
		s.log.Error("Failed to update notification settings", zap.Error(err))
		return nil, err
	}

	s.log.Info("Notification settings updated", zap.Uint64("user_id", req.UserID))
	return toNotificationSettings(settings), nil
}

// ProcessEvent processes an event from Redpanda and sends appropriate notifications
func (s *NotificationService) ProcessEvent(ctx context.Context, eventType string, data map[string]string) error {
	s.log.Info("Processing event", zap.String("event_type", eventType))

	switch eventType {
	case "bet.settled":
		return s.processBetSettled(ctx, data)
	case "payment.deposit_confirmed":
		return s.processDepositConfirmed(ctx, data)
	case "payment.withdrawal_processed":
		return s.processWithdrawalProcessed(ctx, data)
	case "bonus.activated":
		return s.processBonusActivated(ctx, data)
	case "kyc.status_changed":
		return s.processKYCStatusChanged(ctx, data)
	default:
		s.log.Debug("Unknown event type", zap.String("event_type", eventType))
		return nil
	}
}

// ============ Helper functions ============

func (s *NotificationService) isChannelEnabled(settings *repository.NotificationSettings, channel string, notifType string) bool {
	if settings == nil {
		return true // Default to enabled if no settings
	}

	// Check global channel setting
	switch channel {
	case "email":
		return settings.EmailEnabled
	case "sms":
		return settings.SMSEnabled
	case "push":
		return settings.PushEnabled
	case "in_app":
		return settings.InAppEnabled
	}

	// Check type-specific preferences
	if typePref, ok := settings.TypePreferences[notifType]; ok {
		switch channel {
		case "email":
			return typePref.EmailEnabled
		case "sms":
			return typePref.SMSEnabled
		case "push":
			return typePref.PushEnabled
		case "in_app":
			return typePref.InAppEnabled
		}
	}

	return true
}

func (s *NotificationService) sendEmail(ctx context.Context, notif *repository.Notification) error {
	// TODO: Implement email sending via SendGrid/SES
	s.log.Debug("Sending email",
		zap.Uint64("user_id", notif.UserID),
		zap.String("subject", notif.Subject))
	return nil
}

func (s *NotificationService) sendSMS(ctx context.Context, notif *repository.Notification) error {
	// TODO: Implement SMS sending via Twilio
	s.log.Debug("Sending SMS", zap.Uint64("user_id", notif.UserID))
	return nil
}

func (s *NotificationService) sendPush(ctx context.Context, notif *repository.Notification) error {
	// TODO: Implement push notification via Firebase
	s.log.Debug("Sending push notification", zap.Uint64("user_id", notif.UserID))
	return nil
}

func (s *NotificationService) sendInApp(ctx context.Context, notif *repository.Notification) error {
	// In-app notifications are already stored in DB
	// Real-time delivery via WebSocket will be handled by WebSocket Gateway
	s.log.Debug("Creating in-app notification", zap.Uint64("user_id", notif.UserID))
	return nil
}

// Event processors
func (s *NotificationService) processBetSettled(ctx context.Context, data map[string]string) error {
	userID := getUint64FromData(data, "user_id")
	if userID == 0 {
		return errors.New("user_id not found in event data")
	}

	// Send in-app notification about bet result
	req := &SendNotificationRequest{
		UserID:   userID,
		Channel:  "in_app",
		Type:     "bet_settled",
		Subject:  "Ваша ставка рассчитана",
		Message:  "Ставка #" + data["bet_id"] + ": " + data["result"],
		Data:     data,
		Priority: "normal",
	}

	_, err := s.SendNotification(ctx, req)
	return err
}

func (s *NotificationService) processDepositConfirmed(ctx context.Context, data map[string]string) error {
	userID := getUint64FromData(data, "user_id")
	if userID == 0 {
		return errors.New("user_id not found in event data")
	}

	req := &SendNotificationRequest{
		UserID:   userID,
		Channel:  "in_app",
		Type:     "deposit_confirmed",
		Subject:  "Депозит подтверждён",
		Message:  "Сумма: " + data["amount"] + " " + data["currency"],
		Data:     data,
		Priority: "high",
	}

	_, err := s.SendNotification(ctx, req)
	return err
}

func (s *NotificationService) processWithdrawalProcessed(ctx context.Context, data map[string]string) error {
	userID := getUint64FromData(data, "user_id")
	if userID == 0 {
		return errors.New("user_id not found in event data")
	}

	req := &SendNotificationRequest{
		UserID:   userID,
		Channel:  "in_app",
		Type:     "withdrawal_processed",
		Subject:  "Вывод средств обработан",
		Message:  "Сумма: " + data["amount"] + " " + data["currency"],
		Data:     data,
		Priority: "high",
	}

	_, err := s.SendNotification(ctx, req)
	return err
}

func (s *NotificationService) processBonusActivated(ctx context.Context, data map[string]string) error {
	userID := getUint64FromData(data, "user_id")
	if userID == 0 {
		return errors.New("user_id not found in event data")
	}

	req := &SendNotificationRequest{
		UserID:   userID,
		Channel:  "in_app",
		Type:     "bonus_activated",
		Subject:  "Бонус активирован",
		Message:  data["bonus_name"],
		Data:     data,
		Priority: "normal",
	}

	_, err := s.SendNotification(ctx, req)
	return err
}

func (s *NotificationService) processKYCStatusChanged(ctx context.Context, data map[string]string) error {
	userID := getUint64FromData(data, "user_id")
	if userID == 0 {
		return errors.New("user_id not found in event data")
	}

	req := &SendNotificationRequest{
		UserID:   userID,
		Channel:  "in_app",
		Type:     "kyc_status",
		Subject:  "Статус KYC изменён",
		Message:  "Новый статус: " + data["status"],
		Data:     data,
		Priority: "high",
	}

	_, err := s.SendNotification(ctx, req)
	return err
}

// Utility functions
func toNotification(notif *repository.Notification) *Notification {
	return &Notification{
		ID:           notif.ID,
		UserID:       notif.UserID,
		Channel:      notif.Channel,
		Type:         notif.Type,
		Priority:     notif.Priority,
		Subject:      notif.Subject,
		Message:      notif.Message,
		Data:         notif.Data,
		IsRead:       notif.IsRead,
		CreatedAt:    notif.CreatedAt,
		ReadAt:       notif.ReadAt,
		SentAt:       notif.SentAt,
		Status:       notif.Status,
		ErrorMessage: notif.ErrorMessage,
		ReferenceID:  notif.ReferenceID,
		Metadata:     notif.Metadata,
	}
}

func toNotificationSettings(settings *repository.NotificationSettings) *NotificationSettings {
	typePrefs := make(map[string]ChannelPreferences)
	for t, pref := range settings.TypePreferences {
		typePrefs[t] = ChannelPreferences{
			EmailEnabled: pref.EmailEnabled,
			SMSEnabled:   pref.SMSEnabled,
			PushEnabled:  pref.PushEnabled,
			InAppEnabled: pref.InAppEnabled,
		}
	}

	return &NotificationSettings{
		UserID:          settings.UserID,
		EmailEnabled:    settings.EmailEnabled,
		SMSEnabled:      settings.SMSEnabled,
		PushEnabled:     settings.PushEnabled,
		InAppEnabled:    settings.InAppEnabled,
		TypePreferences: typePrefs,
		UpdatedAt:       settings.UpdatedAt,
	}
}

func getBoolOrDefault(ptr *bool, defaultValue bool) bool {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

func getUint64FromData(data map[string]string, key string) uint64 {
	if val, ok := data[key]; ok {
		var result uint64
		// Simple string to uint64 conversion
		// TODO: Use proper parsing
		_ = result
	}
	return 0
}
