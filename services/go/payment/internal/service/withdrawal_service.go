package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/opus-casino/payment/internal/client"
	"github.com/opus-casino/payment/internal/domain"
	"github.com/opus-casino/payment/internal/event"
	"github.com/opus-casino/payment/internal/repository"
	"github.com/rs/zerolog/log"
	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel/trace"
)

// ReviewConfig controls the risk-based manual-review policy.
//
// Withdrawals with fiat value at or above AutoApproveLimitUSD enter
// `pending_review` (funds stay locked, no PSP payout) until a finance
// operator approves or rejects them. Smaller withdrawals keep the
// instant PSP flow.
type ReviewConfig struct {
	Enabled             bool
	AutoApproveLimitUSD decimal.Decimal
}

// DefaultReviewConfig enables review with a $1000 auto-approve threshold.
func DefaultReviewConfig() ReviewConfig {
	return ReviewConfig{
		Enabled:             true,
		AutoApproveLimitUSD: decimal.NewFromInt(1000),
	}
}

// WithdrawalService handles withdrawal business logic
type WithdrawalService struct {
	withdrawalRepo   repository.WithdrawalRepository
	idempotencyRepo  repository.IdempotencyRepository
	exchangeRateRepo repository.ExchangeRateRepository
	dailyLimitsRepo  repository.DailyLimitsRepository
	nowpayments      client.NOWPaymentsAPI
	wallet           client.WalletAPI
	user             client.UserAPI
	producer         *event.Producer
	tracer           trace.Tracer
	ipnCallbackURL   string
	review           ReviewConfig
}

// NewWithdrawalService creates a new withdrawal service
func NewWithdrawalService(
	withdrawalRepo repository.WithdrawalRepository,
	idempotencyRepo repository.IdempotencyRepository,
	exchangeRateRepo repository.ExchangeRateRepository,
	dailyLimitsRepo repository.DailyLimitsRepository,
	nowpayments client.NOWPaymentsAPI,
	wallet client.WalletAPI,
	user client.UserAPI,
	producer *event.Producer,
	tracer trace.Tracer,
	ipnCallbackURL string,
	review ReviewConfig,
) *WithdrawalService {
	return &WithdrawalService{
		withdrawalRepo:   withdrawalRepo,
		idempotencyRepo:  idempotencyRepo,
		exchangeRateRepo: exchangeRateRepo,
		dailyLimitsRepo:  dailyLimitsRepo,
		nowpayments:      nowpayments,
		wallet:           wallet,
		user:             user,
		producer:         producer,
		tracer:           tracer,
		ipnCallbackURL:   ipnCallbackURL,
		review:           review,
	}
}

// InitiateWithdrawalRequest represents a withdrawal request
type InitiateWithdrawalRequest struct {
	UserID         int64
	Amount         decimal.Decimal
	Currency       domain.CryptoCurrency
	Address        string
	IdempotencyKey string
	IPAddress      string
	UserAgent      string
}

// InitiateWithdrawalResponse represents a withdrawal response
type InitiateWithdrawalResponse struct {
	WithdrawalUUID string
	WithdrawalID   string
	Amount         decimal.Decimal
	FiatAmount     decimal.Decimal
	Currency       string
	Address        string
	Status         string
}

// InitiateWithdrawal creates a new withdrawal request
func (s *WithdrawalService) InitiateWithdrawal(ctx context.Context, req InitiateWithdrawalRequest) (*InitiateWithdrawalResponse, error) {
	// Check idempotency
	if existingWithdrawal, err := s.withdrawalRepo.GetByIDempotencyKey(ctx, req.IdempotencyKey); err != nil {
		return nil, fmt.Errorf("check idempotency: %w", err)
	} else if existingWithdrawal != nil {
		log.Info().
			Str("idempotency_key", req.IdempotencyKey).
			Str("withdrawal_id", existingWithdrawal.WithdrawalID).
			Msg("Returning existing withdrawal for idempotency key")
		return s.toResponse(existingWithdrawal), nil
	}

	// Validate KYC level (minimum level 2 for withdrawals)
	if err := s.validateKYCLevel(ctx, req.UserID); err != nil {
		return nil, err
	}

	// Validate currency
	if !req.Currency.IsWithdrawalSupported() {
		return nil, domain.ErrorCurrencyNotSupported(string(req.Currency))
	}

	// Validate withdrawal limits
	if err := s.validateWithdrawalLimits(ctx, req.UserID, req.Amount); err != nil {
		return nil, err
	}

	// Get exchange rate for fiat amount
	fiatAmount, err := s.getFiatAmount(ctx, req.Amount, req.Currency)
	if err != nil {
		return nil, fmt.Errorf("get exchange rate: %w", err)
	}

	// The withdrawal UUID doubles as the wallet reservation reference, so the
	// lock can always be released/settled later even after a crash between
	// the wallet call and the database insert.
	withdrawalUUID := uuid.New()
	lockReference := WithdrawalLockReference(withdrawalUUID)

	// Reserve funds (available -> locked)
	lockResult, err := s.wallet.LockFunds(ctx, client.LockRequest{
		UserID:        req.UserID,
		Currency:      "USD",
		Amount:        fiatAmount,
		Reference:     lockReference,
		ReferenceType: WithdrawalLockReferenceType,
	})
	if err != nil {
		return nil, fmt.Errorf("lock funds: %w", err)
	}

	// Risk-based review: large withdrawals wait for a finance operator.
	// Funds stay locked; no PSP payout exists yet.
	if s.review.Enabled && fiatAmount.GreaterThanOrEqual(s.review.AutoApproveLimitUSD) {
		return s.createReviewWithdrawal(ctx, req, withdrawalUUID, fiatAmount, lockResult.LockID)
	}

	// Create payout in NOWPayments. The provider gets the withdrawal UUID as
	// its reference so IPN callbacks map 1:1 onto our withdrawal.
	npResp, err := s.nowpayments.CreatePayout(ctx, client.CreatePayoutRequest{
		WithdrawalID:   withdrawalUUID.String(),
		Address:        req.Address,
		Currency:       req.Currency.NOWPaymentsCurrency(),
		Amount:         req.Amount,
		IPNCallbackURL: s.getIPNCallbackURL(),
	})

	if err != nil {
		// Compensating transaction: release the reservation
		if unlockErr := s.unlockFunds(ctx, req.UserID, withdrawalUUID); unlockErr != nil {
			log.Error().
				Err(unlockErr).
				Str("lock_reference", lockReference).
				Msg("Failed to release funds after payout failure")
		}
		return nil, domain.ErrorProviderUnavailable("NOWPayments", err)
	}

	// Create withdrawal record
	withdrawal := &domain.Withdrawal{
		UUID:           withdrawalUUID,
		UserID:         req.UserID,
		WithdrawalID:   npResp.WithdrawalID,
		IdempotencyKey: req.IdempotencyKey,
		Amount:         req.Amount,
		FiatAmount:     fiatAmount,
		FiatCurrency:   "USD",
		CryptoCurrency: string(req.Currency),
		Address:        req.Address,
		LockID:         lockResult.LockID,
		Status:         domain.WithdrawalStatusProcessing,
		IPAddress:      req.IPAddress,
		UserAgent:      req.UserAgent,
	}

	if err := s.withdrawalRepo.Create(ctx, withdrawal); err != nil {
		// Compensating transaction: release the reservation
		if unlockErr := s.unlockFunds(ctx, req.UserID, withdrawalUUID); unlockErr != nil {
			log.Error().
				Err(unlockErr).
				Str("lock_reference", lockReference).
				Msg("Failed to release funds after withdrawal create failure")
		}
		return nil, fmt.Errorf("create withdrawal: %w", err)
	}

	// Track daily limit
	if _, err := s.dailyLimitsRepo.Increment(ctx, req.UserID, "withdrawal", fiatAmount); err != nil {
		log.Warn().Err(err).Msg("Failed to track daily withdrawal limit")
	}

	log.Info().
		Int64("user_id", req.UserID).
		Str("withdrawal_id", withdrawal.WithdrawalID).
		Str("amount", req.Amount.String()).
		Msg("Withdrawal created")

	return s.toResponse(withdrawal), nil
}

// GetWithdrawal retrieves a withdrawal by UUID
func (s *WithdrawalService) GetWithdrawal(ctx context.Context, withdrawalUUID string) (*domain.Withdrawal, error) {
	return s.withdrawalRepo.GetByUUID(ctx, withdrawalUUID)
}

// GetWithdrawalByID retrieves a withdrawal by internal ID
func (s *WithdrawalService) GetWithdrawalByID(ctx context.Context, id int64) (*domain.Withdrawal, error) {
	return s.withdrawalRepo.GetByID(ctx, id)
}

// GetWithdrawalByWithdrawalID retrieves a withdrawal by NOWPayments ID
func (s *WithdrawalService) GetWithdrawalByWithdrawalID(ctx context.Context, withdrawalID string) (*domain.Withdrawal, error) {
	return s.withdrawalRepo.GetByWithdrawalID(ctx, withdrawalID)
}

// ListWithdrawals lists withdrawals for a user
func (s *WithdrawalService) ListWithdrawals(ctx context.Context, req ListPaymentsRequest) (*repository.ListResult[domain.Withdrawal], error) {
	filter := repository.ListFilter{
		Limit:  req.Limit,
		Cursor: req.Cursor,
		Status: req.Status,
	}

	return s.withdrawalRepo.ListByUserID(ctx, req.UserID, filter)
}

// ListAllWithdrawals lists the cross-user review queue (admin/ops use-case).
// cursor is an opaque keyset cursor from the previous page; "" starts at the
// oldest pending request.
func (s *WithdrawalService) ListAllWithdrawals(ctx context.Context, limit int, status, cursor string) (*repository.ListResult[domain.Withdrawal], error) {
	return s.withdrawalRepo.ListAll(ctx, repository.ListFilter{
		Limit:  limit,
		Status: status,
		Cursor: cursor,
	})
}

// validateKYCLevel validates minimum KYC level for withdrawals
func (s *WithdrawalService) validateKYCLevel(ctx context.Context, userID int64) error {
	kycLevel, err := s.user.GetKYCLevel(ctx, userID)
	if err != nil {
		return fmt.Errorf("get KYC level: %w", err)
	}

	if kycLevel < 2 {
		return domain.ErrorKYCRequiredLevel(kycLevel)
	}

	return nil
}

// validateWithdrawalLimits validates daily withdrawal limits
func (s *WithdrawalService) validateWithdrawalLimits(ctx context.Context, userID int64, amount decimal.Decimal) error {
	// Get KYC level
	kycLevel, err := s.user.GetKYCLevel(ctx, userID)
	if err != nil {
		return fmt.Errorf("get KYC level: %w", err)
	}

	// Get daily limit for KYC level
	limit := s.getWithdrawalLimit(kycLevel)

	// Get today's cumulative withdrawals
	used, err := s.dailyLimitsRepo.Get(ctx, userID, "withdrawal")
	if err != nil {
		return fmt.Errorf("get daily withdrawals: %w", err)
	}

	// Check if exceeds limit
	if used.Add(amount).GreaterThan(decimal.NewFromFloat(limit)) {
		return domain.ErrorDailyLimitExceeded(limit, used.InexactFloat64(), amount.InexactFloat64())
	}

	return nil
}

// getWithdrawalLimit returns the daily withdrawal limit for a KYC level
func (s *WithdrawalService) getWithdrawalLimit(kycLevel int) float64 {
	limits := map[int]float64{
		0: 0,
		1: 500,
		2: 5000,
		3: 25000,
	}
	if limit, ok := limits[kycLevel]; ok {
		return limit
	}
	return 0
}

// getFiatAmount converts crypto amount to fiat
func (s *WithdrawalService) getFiatAmount(ctx context.Context, amount decimal.Decimal, currency domain.CryptoCurrency) (decimal.Decimal, error) {
	rate, err := s.exchangeRateRepo.Get(ctx, currency.NOWPaymentsCurrency(), "USD")
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get cached exchange rate")
	}

	if rate == nil {
		estResp, err := s.nowpayments.GetEstimatedPrice(ctx, amount, currency.NOWPaymentsCurrency(), "USD")
		if err != nil {
			return decimal.Zero, fmt.Errorf("get estimated price: %w", err)
		}
		rate = &estResp.EstimatedAmount

		if err := s.exchangeRateRepo.Set(ctx, currency.NOWPaymentsCurrency(), "USD", *rate, 60); err != nil {
			log.Warn().Err(err).Msg("Failed to cache exchange rate")
		}
	}

	return amount.Mul(*rate), nil
}

// getIPNCallbackURL returns the webhook callback URL configured via env/config
func (s *WithdrawalService) getIPNCallbackURL() string {
	return s.ipnCallbackURL
}

// toResponse converts a withdrawal to response
func (s *WithdrawalService) toResponse(withdrawal *domain.Withdrawal) *InitiateWithdrawalResponse {
	return &InitiateWithdrawalResponse{
		WithdrawalUUID: withdrawal.UUID.String(),
		WithdrawalID:   withdrawal.WithdrawalID,
		Amount:         withdrawal.Amount,
		FiatAmount:     withdrawal.FiatAmount,
		Currency:       withdrawal.CryptoCurrency,
		Address:        withdrawal.Address,
		Status:         string(withdrawal.Status),
	}
}

// WithdrawalLockReferenceType scopes the wallet reservation for withdrawals.
const WithdrawalLockReferenceType = "withdrawal"

// WithdrawalLockReference returns the stable wallet reservation reference
// for a withdrawal. It must stay derivable from the withdrawal UUID alone:
// compensations (release on failure/rejection) can then run long after the
// original process died.
func WithdrawalLockReference(withdrawalUUID uuid.UUID) string {
	return WithdrawalLockReferenceType + ":" + withdrawalUUID.String()
}

// unlockFunds releases the wallet reservation of a withdrawal.
func (s *WithdrawalService) unlockFunds(ctx context.Context, userID int64, withdrawalUUID uuid.UUID) error {
	return s.wallet.UnlockFunds(ctx, client.UnlockRequest{
		UserID:        userID,
		Reference:     WithdrawalLockReference(withdrawalUUID),
		ReferenceType: WithdrawalLockReferenceType,
	})
}

// ============ Manual review flow ============

// createReviewWithdrawal persists a pending_review withdrawal. Funds are
// already reserved; no PSP payout is created until approval.
func (s *WithdrawalService) createReviewWithdrawal(
	ctx context.Context,
	req InitiateWithdrawalRequest,
	withdrawalUUID uuid.UUID,
	fiatAmount decimal.Decimal,
	lockID string,
) (*InitiateWithdrawalResponse, error) {
	withdrawal := &domain.Withdrawal{
		UUID:           withdrawalUUID,
		UserID:         req.UserID,
		WithdrawalID:   "",
		IdempotencyKey: req.IdempotencyKey,
		Amount:         req.Amount,
		FiatAmount:     fiatAmount,
		FiatCurrency:   "USD",
		CryptoCurrency: string(req.Currency),
		Address:        req.Address,
		LockID:         lockID,
		Status:         domain.WithdrawalStatusPendingReview,
		IPAddress:      req.IPAddress,
		UserAgent:      req.UserAgent,
	}

	if err := s.withdrawalRepo.Create(ctx, withdrawal); err != nil {
		// Compensating transaction: release the reservation
		if unlockErr := s.unlockFunds(ctx, req.UserID, withdrawalUUID); unlockErr != nil {
			log.Error().
				Err(unlockErr).
				Str("lock_reference", WithdrawalLockReference(withdrawalUUID)).
				Msg("Failed to release funds after review-withdrawal create failure")
		}
		return nil, fmt.Errorf("create review withdrawal: %w", err)
	}

	if _, err := s.dailyLimitsRepo.Increment(ctx, req.UserID, "withdrawal", fiatAmount); err != nil {
		log.Warn().Err(err).Msg("Failed to track daily withdrawal limit")
	}

	s.publishAudit(ctx, withdrawal, "", string(withdrawal.Status), "withdrawal.review_requested")
	s.publishReviewEvent(ctx, event.EventTypeWithdrawalReviewRequested, withdrawal, "", "")

	log.Info().
		Int64("user_id", req.UserID).
		Str("withdrawal_uuid", withdrawal.UUID.String()).
		Str("fiat_amount", fiatAmount.String()).
		Msg("Withdrawal queued for manual review")

	return s.toResponse(withdrawal), nil
}

// DecideWithdrawalRequest carries an operator decision.
type DecideWithdrawalRequest struct {
	// WithdrawalUUID identifies the withdrawal (v4 string).
	WithdrawalUUID string
	// AdminID is the operator identity (or "system" for policy decisions).
	AdminID string
	// IdempotencyKey scopes wallet-compensation retries (UUID).
	IdempotencyKey string
	// Reason is required for rejections, optional for approvals.
	Reason string
}

// ApproveWithdrawal approves a pending_review withdrawal and executes the
// PSP payout. Saga: claim (pending->approved) -> PSP payout -> processing.
// On PSP failure funds are unlocked and the withdrawal is marked failed.
// Repeats on an already-decided withdrawal return the current state.
func (s *WithdrawalService) ApproveWithdrawal(ctx context.Context, req DecideWithdrawalRequest) (*domain.Withdrawal, error) {
	if req.AdminID == "" {
		return nil, domain.WithDetails(
			fmt.Errorf("approved_by is required"),
			domain.ErrCodeInvalidAmount,
			map[string]interface{}{"field": "approved_by"},
		)
	}

	w, err := s.withdrawalRepo.GetByUUID(ctx, req.WithdrawalUUID)
	if err != nil {
		return nil, err
	}

	// Idempotent replay: a withdrawal already decided through review
	// returns as-is. Instant-flow rows (no decision recorded) are NOT
	// reviewable: approving them would be a no-op lie.
	switch w.Status {
	case domain.WithdrawalStatusApproved,
		domain.WithdrawalStatusProcessing,
		domain.WithdrawalStatusSending,
		domain.WithdrawalStatusSent,
		domain.WithdrawalStatusFinished:
		if w.DecidedBy == "" {
			break
		}
		return w, nil
	case domain.WithdrawalStatusPendingReview:
		// proceed below
	default:
		return nil, domain.ErrorInvalidStatusTransition(string(w.Status), string(domain.WithdrawalStatusApproved))
	}

	if w.Status != domain.WithdrawalStatusPendingReview {
		return nil, domain.ErrorInvalidStatusTransition(string(w.Status), string(domain.WithdrawalStatusApproved))
	}

	// Atomic claim: exactly one approver wins the race; the loser reloads
	// the winner's state and returns it (idempotent).
	if err := s.withdrawalRepo.UpdateStatus(ctx, w.ID, domain.WithdrawalStatusPendingReview, domain.WithdrawalStatusApproved); err != nil {
		if current, reloadErr := s.withdrawalRepo.GetByUUID(ctx, req.WithdrawalUUID); reloadErr == nil && current.Status != domain.WithdrawalStatusPendingReview {
			return current, nil
		}
		return nil, err
	}
	if err := s.withdrawalRepo.RecordDecision(ctx, w.ID, req.AdminID, req.Reason); err != nil {
		return nil, fmt.Errorf("record approval decision: %w", err)
	}

	// Execute the PSP payout (stable provider id = withdrawal UUID).
	npResp, err := s.nowpayments.CreatePayout(ctx, client.CreatePayoutRequest{
		WithdrawalID:   w.UUID.String(),
		Address:        w.Address,
		Currency:       domain.CryptoCurrency(w.CryptoCurrency).NOWPaymentsCurrency(),
		Amount:         w.Amount,
		IPNCallbackURL: s.getIPNCallbackURL(),
	})
	if err != nil {
		// Compensating transaction: release the reservation, mark failed.
		if unlockErr := s.unlockFunds(ctx, w.UserID, w.UUID); unlockErr != nil {
			log.Error().
				Err(unlockErr).
				Str("lock_reference", WithdrawalLockReference(w.UUID)).
				Msg("Failed to release funds after payout failure")
		}
		_ = s.withdrawalRepo.UpdateStatus(ctx, w.ID, domain.WithdrawalStatusApproved, domain.WithdrawalStatusFailed)
		s.publishAudit(ctx, w, string(domain.WithdrawalStatusApproved), string(domain.WithdrawalStatusFailed), "withdrawal.approve_failed")
		return nil, domain.ErrorProviderUnavailable("NOWPayments", err)
	}

	if err := s.withdrawalRepo.SetProviderWithdrawalID(ctx, w.ID, npResp.WithdrawalID); err != nil {
		return nil, fmt.Errorf("attach provider payout id: %w", err)
	}
	if err := s.withdrawalRepo.UpdateStatus(ctx, w.ID, domain.WithdrawalStatusApproved, domain.WithdrawalStatusProcessing); err != nil {
		return nil, fmt.Errorf("move approved withdrawal to processing: %w", err)
	}

	updated, err := s.withdrawalRepo.GetByUUID(ctx, req.WithdrawalUUID)
	if err != nil {
		return nil, err
	}
	s.publishAudit(ctx, updated, string(domain.WithdrawalStatusPendingReview), string(updated.Status), "withdrawal.approved")
	s.publishReviewEvent(ctx, event.EventTypeWithdrawalApproved, updated, req.AdminID, "")

	log.Info().
		Int64("user_id", w.UserID).
		Str("withdrawal_uuid", w.UUID.String()).
		Str("approved_by", req.AdminID).
		Msg("Withdrawal approved, payout executing")

	return updated, nil
}

// RejectWithdrawal declines a pending_review withdrawal and releases the
// locked funds. Unlock runs BEFORE the status flip so a crash between the
// two is retried safely (wallet unlock is idempotent on the decision key).
// Repeats on an already-rejected withdrawal return the current state.
func (s *WithdrawalService) RejectWithdrawal(ctx context.Context, req DecideWithdrawalRequest) (*domain.Withdrawal, error) {
	if req.AdminID == "" {
		return nil, domain.WithDetails(
			fmt.Errorf("rejected_by is required"),
			domain.ErrCodeInvalidAmount,
			map[string]interface{}{"field": "rejected_by"},
		)
	}
	if req.Reason == "" {
		return nil, domain.WithDetails(
			fmt.Errorf("reason is required"),
			domain.ErrCodeInvalidAmount,
			map[string]interface{}{"field": "reason"},
		)
	}

	w, err := s.withdrawalRepo.GetByUUID(ctx, req.WithdrawalUUID)
	if err != nil {
		return nil, err
	}

	// Idempotent replay.
	if w.Status == domain.WithdrawalStatusRejected {
		return w, nil
	}
	if w.Status != domain.WithdrawalStatusPendingReview {
		return nil, domain.ErrorInvalidStatusTransition(string(w.Status), string(domain.WithdrawalStatusRejected))
	}

	// Compensation first (idempotent), then the terminal flip.
	if err := s.unlockFunds(ctx, w.UserID, w.UUID); err != nil {
		return nil, fmt.Errorf("release reserved funds: %w", err)
	}
	if err := s.withdrawalRepo.UpdateStatus(ctx, w.ID, domain.WithdrawalStatusPendingReview, domain.WithdrawalStatusRejected); err != nil {
		if current, reloadErr := s.withdrawalRepo.GetByUUID(ctx, req.WithdrawalUUID); reloadErr == nil && current.Status != domain.WithdrawalStatusPendingReview {
			return current, nil
		}
		return nil, err
	}
	if err := s.withdrawalRepo.RecordDecision(ctx, w.ID, req.AdminID, req.Reason); err != nil {
		return nil, fmt.Errorf("record rejection decision: %w", err)
	}

	updated, err := s.withdrawalRepo.GetByUUID(ctx, req.WithdrawalUUID)
	if err != nil {
		return nil, err
	}
	s.publishAudit(ctx, updated, string(domain.WithdrawalStatusPendingReview), string(updated.Status), "withdrawal.rejected")
	s.publishReviewEvent(ctx, event.EventTypeWithdrawalRejected, updated, req.AdminID, req.Reason)

	log.Info().
		Int64("user_id", w.UserID).
		Str("withdrawal_uuid", w.UUID.String()).
		Str("rejected_by", req.AdminID).
		Msg("Withdrawal rejected, funds released")

	return updated, nil
}

// CancelWithdrawal lets the player cancel a withdrawal that has not been
// sent to the PSP yet (pending_review). Funds are released first, then the
// terminal flip — same crash-safe ordering as rejection.
func (s *WithdrawalService) CancelWithdrawal(ctx context.Context, userID int64, withdrawalUUID string) (*domain.Withdrawal, error) {
	w, err := s.withdrawalRepo.GetByUUID(ctx, withdrawalUUID)
	if err != nil {
		return nil, err
	}
	if w.UserID != userID {
		return nil, domain.WithDetails(
			fmt.Errorf("withdrawal belongs to another user"),
			domain.ErrCodeWithdrawalNotFound,
			map[string]interface{}{"withdrawal_uuid": withdrawalUUID},
		)
	}
	if w.Status == domain.WithdrawalStatusCancelled {
		return w, nil
	}
	if w.Status != domain.WithdrawalStatusPendingReview {
		return nil, domain.ErrorInvalidStatusTransition(string(w.Status), string(domain.WithdrawalStatusCancelled))
	}

	if err := s.unlockFunds(ctx, w.UserID, w.UUID); err != nil {
		return nil, fmt.Errorf("release reserved funds: %w", err)
	}
	if err := s.withdrawalRepo.UpdateStatus(ctx, w.ID, domain.WithdrawalStatusPendingReview, domain.WithdrawalStatusCancelled); err != nil {
		return nil, err
	}

	updated, err := s.withdrawalRepo.GetByUUID(ctx, withdrawalUUID)
	if err != nil {
		return nil, err
	}
	s.publishAudit(ctx, updated, string(domain.WithdrawalStatusPendingReview), string(updated.Status), "withdrawal.cancelled")
	return updated, nil
}

// publishReviewEvent emits review lifecycle events; broker failures are
// logged but never fail the financial decision (outbox pattern: the DB
// row is the source of truth).
func (s *WithdrawalService) publishReviewEvent(ctx context.Context, eventType string, w *domain.Withdrawal, decidedBy, reason string) {
	if s.producer == nil {
		return
	}
	evt := event.NewWithdrawalReviewEvent(
		eventType, w.UserID, w.UUID.String(), w.Amount, w.FiatAmount, w.CryptoCurrency, decidedBy, reason,
	)
	if err := s.producer.Publish(ctx, w.UUID.String(), evt); err != nil {
		log.Warn().Err(err).Str("event_type", eventType).Msg("Failed to publish review event")
	}
}

// publishAudit emits a status-change audit record.
func (s *WithdrawalService) publishAudit(ctx context.Context, w *domain.Withdrawal, from, to, op string) {
	if s.producer == nil {
		return
	}
	evt := event.NewPaymentAuditEvent(w.UserID, op, w.UUID.String(), "withdrawal", w.UUID.String())
	evt.PreviousStatus = from
	evt.NewStatus = to
	evt.Amount = &w.FiatAmount
	evt.Currency = w.FiatCurrency
	if err := s.producer.Publish(ctx, w.UUID.String(), evt); err != nil {
		log.Warn().Err(err).Str("operation", op).Msg("Failed to publish audit event")
	}
}
