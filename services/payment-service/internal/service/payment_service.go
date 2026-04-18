package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/platform/services/payment-service/internal/client"
	"github.com/platform/services/payment-service/internal/domain"
	"github.com/platform/services/payment-service/internal/repository"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// PaymentService handles payment business logic
type PaymentService struct {
	paymentRepo      repository.PaymentRepository
	idempotencyRepo  repository.IdempotencyRepository
	exchangeRateRepo repository.ExchangeRateRepository
	dailyLimitsRepo  repository.DailyLimitsRepository
	nowpayments      *client.NOWPaymentsClient
	wallet           *client.WalletClient
	user             *client.UserClient
	logger           *zap.Logger
}

// NewPaymentService creates a new payment service
func NewPaymentService(
	paymentRepo repository.PaymentRepository,
	idempotencyRepo repository.IdempotencyRepository,
	exchangeRateRepo repository.ExchangeRateRepository,
	dailyLimitsRepo repository.DailyLimitsRepository,
	nowpayments *client.NOWPaymentsClient,
	wallet *client.WalletClient,
	user *client.UserClient,
	logger *zap.Logger,
) *PaymentService {
	return &PaymentService{
		paymentRepo:      paymentRepo,
		idempotencyRepo:  idempotencyRepo,
		exchangeRateRepo: exchangeRateRepo,
		dailyLimitsRepo:  dailyLimitsRepo,
		nowpayments:      nowpayments,
		wallet:           wallet,
		user:             user,
		logger:           logger,
	}
}

// InitiateDepositRequest represents a deposit request
type InitiateDepositRequest struct {
	UserID         int64
	Amount         decimal.Decimal
	Currency       domain.CryptoCurrency
	IdempotencyKey string
	IPAddress      string
	UserAgent      string
}

// InitiateDepositResponse represents a deposit response
type InitiateDepositResponse struct {
	PaymentUUID   string
	PaymentID     string
	PayAddress    string
	PayAmount     decimal.Decimal
	PayCurrency   string
	FiatAmount    decimal.Decimal
	ExpiresAt     time.Time
}

// InitiateDeposit creates a new deposit payment
func (s *PaymentService) InitiateDeposit(ctx context.Context, req InitiateDepositRequest) (*InitiateDepositResponse, error) {
	// Check idempotency
	if existingPayment, err := s.paymentRepo.GetByIDempotencyKey(ctx, req.IdempotencyKey); err != nil {
		return nil, fmt.Errorf("check idempotency: %w", err)
	} else if existingPayment != nil {
		s.logger.Info("Returning existing payment for idempotency key",
			zap.String("idempotency_key", req.IdempotencyKey),
			zap.String("payment_id", existingPayment.PaymentID),
		)
		return s.toResponse(existingPayment), nil
	}

	// Validate KYC level and limits
	if err := s.validateDepositLimits(ctx, req.UserID, req.Amount); err != nil {
		return nil, err
	}

	// Validate currency
	if !req.Currency.IsDepositSupported() {
		return nil, domain.ErrorCurrencyNotSupported(string(req.Currency))
	}

	// Get exchange rate
	fiatAmount, err := s.getFiatAmount(ctx, req.Amount, req.Currency)
	if err != nil {
		return nil, fmt.Errorf("get exchange rate: %w", err)
	}

	// Create payment in NOWPayments
	npResp, err := s.nowpayments.CreatePayment(ctx, client.CreatePaymentRequest{
		PriceAmount:    req.Amount,
		PriceCurrency:  "USD",
		PayCurrency:    req.Currency.NOWPaymentsCurrency(),
		IPNCallbackURL: s.getIPNCallbackURL(),
		OrderID:        uuid.New().String(),
	})
	if err != nil {
		return nil, domain.ErrorProviderUnavailable("NOWPayments", err)
	}

	// Create payment record
	payment := &domain.Payment{
		UUID:            uuid.New(),
		UserID:          req.UserID,
		PaymentID:       npResp.PaymentID,
		IdempotencyKey:  req.IdempotencyKey,
		RequestedAmount: req.Amount,
		FiatAmount:      fiatAmount,
		FiatCurrency:    "USD",
		CryptoCurrency:  string(req.Currency),
		PayAddress:      npResp.PayAddress,
		PayAmount:       &npResp.PayAmount,
		Status:          domain.PaymentStatusPending,
		ExpiresAt:       &npResp.ExpiresAt,
		IPAddress:       req.IPAddress,
		UserAgent:       req.UserAgent,
	}

	if err := s.paymentRepo.Create(ctx, payment); err != nil {
		return nil, fmt.Errorf("create payment: %w", err)
	}

	s.logger.Info("Deposit payment created",
		zap.Int64("user_id", req.UserID),
		zap.String("payment_id", payment.PaymentID),
		zap.String("amount", req.Amount.String()),
	)

	return s.toResponse(payment), nil
}

// GetPayment retrieves a payment by UUID
func (s *PaymentService) GetPayment(ctx context.Context, paymentUUID string) (*domain.Payment, error) {
	payment, err := s.paymentRepo.GetByUUID(ctx, paymentUUID)
	if err != nil {
		return nil, err
	}
	return payment, nil
}

// GetPaymentByID retrieves a payment by internal ID
func (s *PaymentService) GetPaymentByID(ctx context.Context, id int64) (*domain.Payment, error) {
	return s.paymentRepo.GetByID(ctx, id)
}

// GetPaymentByPaymentID retrieves a payment by NOWPayments ID
func (s *PaymentService) GetPaymentByPaymentID(ctx context.Context, paymentID string) (*domain.Payment, error) {
	return s.paymentRepo.GetByPaymentID(ctx, paymentID)
}

// ListPaymentsRequest represents a list payments request
type ListPaymentsRequest struct {
	UserID int64
	Limit  int
	Cursor string
	Status string
}

// ListPayments lists payments for a user
func (s *PaymentService) ListPayments(ctx context.Context, req ListPaymentsRequest) (*repository.ListResult[domain.Payment], error) {
	filter := repository.ListFilter{
		Limit:  req.Limit,
		Cursor: req.Cursor,
		Status: req.Status,
	}

	return s.paymentRepo.ListByUserID(ctx, req.UserID, filter)
}

// validateDepositLimits validates KYC level and daily limits
func (s *PaymentService) validateDepositLimits(ctx context.Context, userID int64, amount decimal.Decimal) error {
	// Get KYC level
	kycLevel, err := s.user.GetKYCLevel(ctx, userID)
	if err != nil {
		return fmt.Errorf("get KYC level: %w", err)
	}

	// Get daily limit for KYC level
	limit := s.getDepositLimit(kycLevel)

	// Get today's cumulative deposits
	used, err := s.dailyLimitsRepo.Get(ctx, userID, "deposit")
	if err != nil {
		return fmt.Errorf("get daily deposits: %w", err)
	}

	// Check if exceeds limit
	if used.Add(amount).GreaterThan(decimal.NewFromFloat(limit)) {
		return domain.ErrorDailyLimitExceeded(limit, used.InexactFloat64(), amount.InexactFloat64())
	}

	return nil
}

// getDepositLimit returns the daily deposit limit for a KYC level
func (s *PaymentService) getDepositLimit(kycLevel int) float64 {
	limits := map[int]float64{
		0: 500,
		1: 2000,
		2: 10000,
		3: 50000,
	}
	if limit, ok := limits[kycLevel]; ok {
		return limit
	}
	return 500 // Default to level 0
}

// getFiatAmount converts crypto amount to fiat
func (s *PaymentService) getFiatAmount(ctx context.Context, amount decimal.Decimal, currency domain.CryptoCurrency) (decimal.Decimal, error) {
	// Try cache first
	rate, err := s.exchangeRateRepo.Get(ctx, currency.NOWPaymentsCurrency(), "USD")
	if err != nil {
		s.logger.Warn("Failed to get cached exchange rate", zap.Error(err))
	}

	if rate == nil {
		// Fetch from NOWPayments
		estResp, err := s.nowpayments.GetEstimatedPrice(ctx, amount, currency.NOWPaymentsCurrency(), "USD")
		if err != nil {
			return decimal.Zero, fmt.Errorf("get estimated price: %w", err)
		}

		rate = &estResp.EstimatedAmount

		// Cache the rate
		if err := s.exchangeRateRepo.Set(ctx, currency.NOWPaymentsCurrency(), "USD", *rate, 60); err != nil {
			s.logger.Warn("Failed to cache exchange rate", zap.Error(err))
		}
	}

	return amount.Mul(*rate), nil
}

// getIPNCallbackURL returns the webhook callback URL
func (s *PaymentService) getIPNCallbackURL() string {
	// TODO: Get from config
	return "https://api.platform.com/api/v1/payments/webhooks/nowpayments"
}

// toResponse converts a payment to response
func (s *PaymentService) toResponse(payment *domain.Payment) *InitiateDepositResponse {
	var payAmount decimal.Decimal
	if payment.PayAmount != nil {
		payAmount = *payment.PayAmount
	}

	var expiresAt time.Time
	if payment.ExpiresAt != nil {
		expiresAt = *payment.ExpiresAt
	}

	return &InitiateDepositResponse{
		PaymentUUID: payment.UUID.String(),
		PaymentID:   payment.PaymentID,
		PayAddress:  payment.PayAddress,
		PayAmount:   payAmount,
		PayCurrency: payment.CryptoCurrency,
		FiatAmount:  payment.FiatAmount,
		ExpiresAt:   expiresAt,
	}
}
