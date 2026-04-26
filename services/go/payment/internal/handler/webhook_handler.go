package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/opus-casino/payment/internal/service"
	"github.com/rs/zerolog/log"
)

// WebhookHandler handles webhook HTTP requests
type WebhookHandler struct {
	webhookSvc *service.WebhookService
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(webhookSvc *service.WebhookService) *WebhookHandler {
	return &WebhookHandler{
		webhookSvc: webhookSvc,
	}
}

// ProcessNOWPaymentsWebhook handles POST /api/v1/payments/webhooks/nowpayments
func (h *WebhookHandler) ProcessNOWPaymentsWebhook(c *fiber.Ctx) error {
	payload := c.Body()
	signature := c.Get("x-nowpayments-sig")

	if signature == "" {
		log.Warn().Msg("Missing webhook signature")
		return c.Status(401).JSON(ErrorResponse{
			Error: ErrorDetail{
				Code:    5011,
				Message: "missing signature",
			},
			Meta: Meta{
				RequestID: getRequestID(c),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
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
			log.Error().Err(err).Msg("Failed to process webhook")
			return c.Status(500).JSON(ErrorResponse{
				Error: ErrorDetail{
					Code:    5000,
					Message: "webhook processing failed",
				},
				Meta: Meta{
					RequestID: getRequestID(c),
					Timestamp: time.Now().UTC().Format(time.RFC3339),
				},
			})
		}
	}

	log.Info().
		Str("type", result.Type).
		Str("id", result.ID).
		Str("status", result.Status).
		Msg("Webhook processed")

	return c.Status(200).JSON(fiber.Map{
		"processed": true,
		"type":      result.Type,
		"id":        result.ID,
		"status":    result.Status,
	})
}

// Handlers combines all handlers
type Handlers struct {
	Payment    *PaymentHandler
	Withdrawal *WithdrawalHandler
	Webhook    *WebhookHandler
}

// New creates a new Handlers instance
func New(
	paymentSvc *service.PaymentService,
	withdrawalSvc *service.WithdrawalService,
	webhookSvc *service.WebhookService,
) *Handlers {
	return &Handlers{
		Payment:    NewPaymentHandler(paymentSvc),
		Withdrawal: NewWithdrawalHandler(withdrawalSvc),
		Webhook:    NewWebhookHandler(webhookSvc),
	}
}
