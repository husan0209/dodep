package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/opus-casino/payment/internal/client"
	"github.com/opus-casino/payment/internal/domain"
	"github.com/opus-casino/payment/internal/observability"
	"github.com/opus-casino/payment/internal/repository"
	"github.com/rs/zerolog"
)

// ReviewDispatcher executes manual-review decisions on the shared withdrawals
// table. Finance operators decide rows in the admin panel; this dispatcher is
// the payment service's side of that contract:
//
//   - approved  → create the NOWPayments payout (approved → sending) and
//     record the payout id, so webhook processing continues as usual;
//   - rejected  → return the locked funds to the player's wallet
//     (rejected → cancelled).
//
// Exactly-once semantics come from ClaimDecidedWithdrawals: a decided row is
// claimed by exactly one dispatcher (conditional UPDATE + SKIP LOCKED) and the
// claim is never released. A failed step keeps the claim and is surfaced for
// operators — money movement is never silently retried after an ambiguous
// failure.
type ReviewDispatcher struct {
	repo           repository.WithdrawalRepository
	wallet         client.WalletAPI
	nowpayments    client.NOWPaymentsAPI
	log            zerolog.Logger
	batchSize      int
	ipnCallbackURL string
}

// NewReviewDispatcher creates a dispatcher. batchSize <= 0 defaults to 20.
func NewReviewDispatcher(
	repo repository.WithdrawalRepository,
	wallet client.WalletAPI,
	nowpayments client.NOWPaymentsAPI,
	log zerolog.Logger,
	batchSize int,
	ipnCallbackURL string,
) *ReviewDispatcher {
	if batchSize <= 0 {
		batchSize = 20
	}
	if batchSize > 200 {
		batchSize = 200
	}
	return &ReviewDispatcher{
		repo:           repo,
		wallet:         wallet,
		nowpayments:    nowpayments,
		log:            log,
		batchSize:      batchSize,
		ipnCallbackURL: ipnCallbackURL,
	}
}

// RunOnce claims and processes one batch of decided withdrawals.
func (d *ReviewDispatcher) RunOnce(ctx context.Context) error {
	withdrawals, err := d.repo.ClaimDecidedWithdrawals(ctx, d.batchSize)
	if err != nil {
		return fmt.Errorf("claim decided withdrawals: %w", err)
	}

	for _, w := range withdrawals {
		switch w.Status {
		case domain.WithdrawalStatusApproved:
			d.processApproved(ctx, w)
		case domain.WithdrawalStatusRejected:
			d.processRejected(ctx, w)
		default:
			// Defensive: the claim query only returns approved/rejected rows.
			d.log.Warn().
				Int64("withdrawal_id", w.ID).
				Str("status", string(w.Status)).
				Msg("Skipping claimed withdrawal with unexpected status")
		}
	}

	return nil
}

// processApproved creates the payout for an approved review row. The row is
// already claimed, so a failure is terminal for this pass and logged for
// operators.
func (d *ReviewDispatcher) processApproved(ctx context.Context, w *domain.Withdrawal) {
	payoutID := uuid.New().String()
	npResp, err := d.nowpayments.CreatePayout(ctx, client.CreatePayoutRequest{
		WithdrawalID:   payoutID,
		Address:        w.Address,
		Currency:       domain.CryptoCurrency(w.CryptoCurrency).NOWPaymentsCurrency(),
		Amount:         w.Amount,
		IPNCallbackURL: d.ipnCallbackURL,
	})
	if err != nil {
		observability.RecordError("payout_creation_failed", observability.OperationWithdrawal)
		d.log.Error().
			Err(err).
			Int64("withdrawal_id", w.ID).
			Str("decided_by", w.DecidedBy).
			Msg("Approved withdrawal: payout creation failed — operator action required")
		return
	}

	if err := d.repo.UpdatePayoutID(ctx, w.ID, npResp.WithdrawalID); err != nil {
		d.log.Error().
			Err(err).
			Int64("withdrawal_id", w.ID).
			Msg("Approved withdrawal: failed to persist payout id — operator action required")
		return
	}

	if err := d.repo.UpdateStatus(ctx, w.ID, domain.WithdrawalStatusApproved, domain.WithdrawalStatusSending); err != nil {
		d.log.Error().
			Err(err).
			Int64("withdrawal_id", w.ID).
			Msg("Approved withdrawal: failed to move to sending — operator action required")
		return
	}

	observability.RecordWithdrawalTransition(observability.WithdrawalStatusProcessing, w.CryptoCurrency, observability.KYCLevelUnknown)

	d.log.Info().
		Int64("withdrawal_id", w.ID).
		Str("payout_id", npResp.WithdrawalID).
		Str("decided_by", w.DecidedBy).
		Msg("Approved withdrawal: payout created")
}

// processRejected returns the locked funds for a rejected review row. The
// wallet unlock is idempotent on the lock id, but the claim marker guarantees
// the unlock is attempted exactly once regardless.
func (d *ReviewDispatcher) processRejected(ctx context.Context, w *domain.Withdrawal) {
	if err := d.wallet.UnlockFunds(ctx, w.LockID, w.IdempotencyKey+"_unlock"); err != nil {
		observability.RecordError("compensation_failed", observability.OperationWithdrawal)
		d.log.Error().
			Err(err).
			Int64("withdrawal_id", w.ID).
			Str("lock_id", w.LockID).
			Msg("Rejected withdrawal: fund return failed — operator action required")
		return
	}

	if err := d.repo.UpdateStatus(ctx, w.ID, domain.WithdrawalStatusRejected, domain.WithdrawalStatusCancelled); err != nil {
		d.log.Error().
			Err(err).
			Int64("withdrawal_id", w.ID).
			Msg("Rejected withdrawal: failed to mark cancelled after fund return")
		return
	}

	observability.RecordWithdrawalTransition(observability.WithdrawalStatusCancelled, w.CryptoCurrency, observability.KYCLevelUnknown)

	d.log.Info().
		Int64("withdrawal_id", w.ID).
		Str("decided_by", w.DecidedBy).
		Str("reason", w.DecisionReason).
		Msg("Rejected withdrawal: funds returned to player wallet")
}
