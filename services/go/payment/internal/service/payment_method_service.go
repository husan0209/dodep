package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/opus-casino/payment/internal/crypto"
	"github.com/opus-casino/payment/internal/domain"
	"github.com/opus-casino/payment/internal/repository"
	"github.com/rs/zerolog/log"
)

// PaymentMethodService manages saved payout destinations.
// Raw details are encrypted with AES-256-GCM before storage and are never
// returned by any read path (only a masked display value is exposed).
type PaymentMethodService struct {
	methods  repository.PaymentMethodRepository
	fieldEnc *crypto.FieldEncryption
}

// NewPaymentMethodService creates the service. fieldEnc must be Ready;
// construction fails closed otherwise.
func NewPaymentMethodService(methods repository.PaymentMethodRepository, fieldEnc *crypto.FieldEncryption) (*PaymentMethodService, error) {
	if fieldEnc == nil || !fieldEnc.Ready() {
		return nil, fmt.Errorf("payment method encryption is not configured")
	}
	return &PaymentMethodService{methods: methods, fieldEnc: fieldEnc}, nil
}

// SaveMethodInput carries user input for storing a destination.
type SaveMethodInput struct {
	UserID    int64
	Type      domain.PaymentMethodType
	Provider  string
	Nickname  string
	Details   map[string]string
	IsDefault bool
}

// SaveMethod validates, encrypts and persists a payout destination.
func (s *PaymentMethodService) SaveMethod(ctx context.Context, in SaveMethodInput) (*domain.SavedPaymentMethod, error) {
	if in.UserID <= 0 {
		return nil, domain.WithDetails(fmt.Errorf("user id is required"), domain.ErrCodeInvalidAmount, map[string]interface{}{"field": "user_id"})
	}
	if !validMethodType(in.Type) {
		return nil, domain.WithDetails(fmt.Errorf("unsupported payment method type"), domain.ErrCodeCurrencyNotSupported, map[string]interface{}{"type": string(in.Type)})
	}
	if strings.TrimSpace(in.Provider) == "" {
		return nil, domain.WithDetails(fmt.Errorf("provider is required"), domain.ErrCodeInvalidAmount, map[string]interface{}{"field": "provider"})
	}
	if len(in.Details) == 0 {
		return nil, domain.WithDetails(fmt.Errorf("payment details are required"), domain.ErrCodeInvalidAmount, map[string]interface{}{"field": "details"})
	}
	if len(in.Nickname) > 100 {
		return nil, domain.WithDetails(fmt.Errorf("nickname too long"), domain.ErrCodeInvalidAmount, map[string]interface{}{"field": "nickname"})
	}

	raw, err := json.Marshal(in.Details)
	if err != nil {
		return nil, fmt.Errorf("marshal details: %w", err)
	}
	enc, err := s.fieldEnc.Encrypt(string(raw))
	if err != nil {
		return nil, fmt.Errorf("encrypt details: %w", err)
	}

	method := domain.NewSavedPaymentMethod()
	method.UserID = in.UserID
	method.Type = in.Type
	method.Provider = strings.TrimSpace(in.Provider)
	method.Nickname = strings.TrimSpace(in.Nickname)
	method.DetailsEncrypted = enc
	method.DisplayValue = crypto.MaskDetails(firstDetailValue(in.Details))
	method.IsDefault = in.IsDefault

	if err := s.methods.Create(ctx, method); err != nil {
		return nil, err
	}
	if in.IsDefault {
		if err := s.clearOtherDefaults(ctx, in.UserID, method.ID); err != nil {
			log.Warn().Err(err).Msg("Failed to clear other default methods")
		}
	}

	log.Info().
		Int64("user_id", in.UserID).
		Str("method_id", method.ID.String()).
		Msg("Payment method saved")

	return method, nil
}

// GetMethod returns a saved method owned by the user (details stay encrypted).
func (s *PaymentMethodService) GetMethod(ctx context.Context, userID int64, id uuid.UUID) (*domain.SavedPaymentMethod, error) {
	m, err := s.methods.GetByID(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, domain.ErrorPaymentNotFound(0)
	}
	return m, nil
}

// ListMethods paginates saved methods owned by the user.
func (s *PaymentMethodService) ListMethods(ctx context.Context, userID int64, limit, offset int) ([]domain.SavedPaymentMethod, int64, error) {
	return s.methods.ListByUserID(ctx, userID, limit, offset)
}

// DeleteMethod soft-deletes a method owned by the user.
func (s *PaymentMethodService) DeleteMethod(ctx context.Context, userID int64, id uuid.UUID) error {
	return s.methods.Delete(ctx, userID, id)
}

func (s *PaymentMethodService) clearOtherDefaults(ctx context.Context, userID int64, except uuid.UUID) error {
	items, _, err := s.methods.ListByUserID(ctx, userID, 100, 0)
	if err != nil {
		return err
	}
	for _, m := range items {
		if m.ID != except && m.IsDefault {
			m.IsDefault = false
			if err := s.methods.Update(ctx, &m); err != nil {
				return err
			}
		}
	}
	return nil
}

func validMethodType(t domain.PaymentMethodType) bool {
	switch t {
	case domain.PaymentMethodTypeCreditCard,
		domain.PaymentMethodTypeDebitCard,
		domain.PaymentMethodTypeBankTransfer,
		domain.PaymentMethodTypeEWallet,
		domain.PaymentMethodTypeCrypto,
		domain.PaymentMethodTypePrepaid,
		domain.PaymentMethodTypeMobile,
		domain.PaymentMethodTypePIX,
		domain.PaymentMethodTypeUPI:
		return true
	default:
		return false
	}
}

func firstDetailValue(details map[string]string) string {
	for _, k := range []string{"card_number", "account_number", "address", "wallet", "phone", "email"} {
		if v := strings.TrimSpace(details[k]); v != "" {
			return v
		}
	}
	for _, v := range details {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
