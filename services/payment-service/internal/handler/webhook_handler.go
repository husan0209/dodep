package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/platform/services/payment-service/internal/service"
	"go.uber.org/zap"
)

// WebhookHandler handles webhook HTTP requests
type WebhookHandler struct {
	webhookSvc *service.WebhookService
	logger     *zap.Logger
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(webhookSvc *service.WebhookService, logger *zap.Logger) *WebhookHandler {
	return &WebhookHandler{
		webhookSvc: webhookSvc,
		logger:     logger,
	}
}

// ProcessNOWPaymentsWebhook handles POST /api/v1/payments/webhooks/nowpayments
func (h *WebhookHandler) ProcessNOWPaymentsWebhook(c *fiber.Ctx) error {
	payload := c.Body()
	signature := c.Get("x-nowpayments-sig")

	if signature == "" {
		h.logger.Warn("Missing webhook signature")
		return c.Status(401).JSON(ErrorResponse{
			Code:    5011,
			Message: "missing signature",
		})
	}

	// Try to process as deposit first
	result, err := h.webhookSvc.ProcessDepositWebhook(c.Context(), service.ProcessWebhookRequest{
		Payload:   payload,
		Signature: signature,
	})

	if err != nil {
		// Try as withdrawal
		result, err = h.webhookSvc.ProcessWithdrawalWebhook(c.Context(), service.ProcessWebhookRequest{
			Payload:   payload,
			Signature: signature,
		})
		if err != nil {
			h.logger.Error("Failed to process webhook", zap.Error(err))
			return c.Status(500).JSON(ErrorResponse{
				Code:    5000,
				Message: "webhook processing failed",
			})
		}
	}

	h.logger.Info("Webhook processed",
		zap.String("type", result.Type),
		zap.String("id", result.ID),
		zap.String("status", result.Status),
	)

	return c.Status(200).JSON(fiber.Map{
		"processed": true,
		"type":      result.Type,
		"id":        result.ID,
		"status":    result.Status,
	})
}
