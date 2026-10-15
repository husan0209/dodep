package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/opus-casino/affiliate/internal/domain"
)

// NGRComponents are the settled, finalized deductions subtracted from GGR
// to obtain NGR. Every field is a decimal amount in the affiliate currency.
// Only settled/finalized upstream data may feed this function (ТЗ Фаза 4).
type NGRComponents struct {
	Bonuses           decimal.Decimal
	PaymentFees       decimal.Decimal
	Chargebacks       decimal.Decimal
	FraudWriteoffs    decimal.Decimal
	Taxes             decimal.Decimal
	ManualAdjustments decimal.Decimal
}

// IsZero reports whether all deduction components are zero.
func (c NGRComponents) IsZero() bool {
	return c.Bonuses.IsZero() &&
		c.PaymentFees.IsZero() &&
		c.Chargebacks.IsZero() &&
		c.FraudWriteoffs.IsZero() &&
		c.Taxes.IsZero() &&
		c.ManualAdjustments.IsZero()
}

// Total sums all deduction components.
func (c NGRComponents) Total() decimal.Decimal {
	return c.Bonuses.
		Add(c.PaymentFees).
		Add(c.Chargebacks).
		Add(c.FraudWriteoffs).
		Add(c.Taxes).
		Add(c.ManualAdjustments)
}

// ComputeNGR derives Net Gaming Revenue from Gross Gaming Revenue:
//
//	NGR = GGR - bonuses - payment_fees - chargebacks
//	    - fraud_writeoffs - taxes - manual_adjustments
//
// Any negative component is rejected: deductions must be positive amounts.
// The result is NOT clamped here; clamping to max(0, ...) happens in
// commission computation (domain.CalculateCommission), keeping NGR itself
// auditable even when negative.
func ComputeNGR(ggr decimal.Decimal, c NGRComponents) (decimal.Decimal, error) {
	for name, v := range map[string]decimal.Decimal{
		"bonuses":            c.Bonuses,
		"payment_fees":       c.PaymentFees,
		"chargebacks":        c.Chargebacks,
		"fraud_writeoffs":    c.FraudWriteoffs,
		"taxes":              c.Taxes,
		"manual_adjustments": c.ManualAdjustments,
	} {
		if v.IsNegative() {
			return decimal.Zero, NewValidationError(FieldError{
				Field:   name,
				Message: "deduction component must not be negative",
			})
		}
	}
	return ggr.Sub(c.Total()), nil
}

// DailyAccrualItem is one settled player-period feeding the daily accrual job.
type DailyAccrualItem struct {
	AffiliateID    uuid.UUID
	ReferredUserID int64
	SourceType     string
	SourceID       string
	PeriodStart    time.Time
	PeriodEnd      time.Time
	GGRAmount      decimal.Decimal
	Deductions     NGRComponents
	IdempotencyKey string
}

// AccrueDaily is the Фаза 4 daily accrual job entry point: it settles one
// batch of finalized player activity into accrued (held) commission rows.
// Each item is idempotent by its key — re-running the job for the same day
// returns the existing earning instead of double-accruing. Items are
// processed independently; one bad item never aborts the batch.
func (s *AffiliateService) AccrueDaily(ctx context.Context, items []DailyAccrualItem) ([]*domain.AffiliateEarning, error) {
	results := make([]*domain.AffiliateEarning, 0, len(items))
	for i := range items {
		it := items[i]
		earning, err := s.CalculateCommission(ctx, CalculateCommissionInput{
			AffiliateID:    it.AffiliateID,
			ReferredUserID: it.ReferredUserID,
			SourceType:     it.SourceType,
			SourceID:       it.SourceID,
			PeriodStart:    it.PeriodStart,
			PeriodEnd:      it.PeriodEnd,
			GGRAmount:      it.GGRAmount,
			Deductions:     it.Deductions,
			IdempotencyKey: it.IdempotencyKey,
		})
		if err != nil {
			return results, err
		}
		results = append(results, earning)
	}
	return results, nil
}

// ReverseEarningInput reverses a held or available commission line, e.g. on
// chargeback-heavy traffic or confirmed fraud. Paid earnings are terminal
// and can only be corrected with a manual debit adjustment.
type ReverseEarningInput struct {
	EarningID uuid.UUID
	Reason    string
}

// ReverseEarning flips one earning to reversed, emitting
// affiliate.commission.reversed and mirroring the movement in the affiliate
// ledger (out of pending/available, into reversed).
func (s *AffiliateService) ReverseEarning(ctx context.Context, in ReverseEarningInput) (*domain.AffiliateEarning, error) {
	if in.EarningID == uuid.Nil {
		return nil, NewValidationError(FieldError{Field: "earning_id", Message: "earning_id is required"})
	}
	if strings.TrimSpace(in.Reason) == "" {
		return nil, NewValidationError(FieldError{Field: "reason", Message: "reversal reason is required for audit"})
	}

	earning, err := s.repo.GetEarningByID(ctx, in.EarningID)
	if err != nil {
		return nil, err
	}
	if earning == nil {
		return nil, ErrEarningNotFound
	}
	switch earning.Status {
	case domain.EarningStatusAccrued, domain.EarningStatusPending, domain.EarningStatusAvailable:
		// reversible
	default:
		return nil, ErrEarningNotReversible
	}

	reversed, err := s.repo.ReverseEarning(ctx, in.EarningID, in.Reason)
	if err != nil {
		return nil, err
	}

	s.logger.Info("commission reversed",
		zap.String("earning_id", in.EarningID.String()),
		zap.String("affiliate_id", reversed.AffiliateID.String()),
		zap.String("amount", reversed.CommissionAmount.String()),
		zap.String("reason", in.Reason),
	)
	return reversed, nil
}

// MarkPayoutPaidInput finalizes an approved affiliate payout after the PSP
// leg completes. Only approved payouts may transition to paid; the ledger
// movement available -> paid happens atomically in the repository.
type MarkPayoutPaidInput struct {
	PayoutID          uuid.UUID
	ProviderReference string
}

// MarkPayoutPaid moves an approved payout to paid. It is separate from
// ApproveAffiliatePayout on purpose: approval is the finance decision,
// paid is the PSP settlement fact. Affiliate money stays inside the
// affiliate ledger contour (available -> paid), never touching player wallet.
func (s *AffiliateService) MarkPayoutPaid(ctx context.Context, in MarkPayoutPaidInput) (*domain.AffiliatePayout, error) {
	payout, err := s.repo.GetPayoutByID(ctx, in.PayoutID)
	if err != nil {
		return nil, err
	}
	if payout == nil {
		return nil, domain.ErrPayoutNotFound
	}
	if _, err := s.requireActiveProfile(ctx, payout.AffiliateID); err != nil {
		return nil, err
	}
	if payout.Status != domain.PayoutStatusApproved && payout.Status != domain.PayoutStatusProcessing {
		return nil, domain.ErrInvalidPayoutStatus
	}

	payout.Status = domain.PayoutStatusPaid
	if in.ProviderReference != "" {
		payout.ProviderReference = in.ProviderReference
	}
	if err := s.repo.UpdatePayout(ctx, payout); err != nil {
		return nil, err
	}
	return payout, nil
}

// ReleaseHold is the Фаза 4 hold-release job entry point: every earning in
// accrued/pending whose hold_until has passed becomes available, emitting
// affiliate.commission.released per row. The hold_until check is the gate —
// commission must never become available instantly (ТЗ).
func (s *AffiliateService) ReleaseHold(ctx context.Context, now time.Time) (int64, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	released, err := s.repo.ReleaseEligibleEarnings(ctx, now)
	if err != nil {
		return 0, err
	}
	if released > 0 {
		s.logger.Info("hold released", zap.Int64("earnings", released))
	}
	return released, nil
}
