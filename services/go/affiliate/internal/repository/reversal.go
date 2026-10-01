package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opus-casino/affiliate/internal/domain"
)

// ReverseEarning cancels an accrued commission and moves the money to the
// `reversed` ledger account.
//
// Business rules:
//   - only earned money can be reversed (accrued, pending or available);
//     a payout that is already paid is settled and cannot be clawed back
//     through this path (that is a recovery/refund decision, not a reversal);
//   - the money is debited from the account that currently holds it
//     (pending while held, available after hold release);
//   - reversal is idempotent: replaying it returns the same earning.
func (r *GormAffiliateRepository) ReverseEarning(
	ctx context.Context,
	earningID uuid.UUID,
	reason string,
	reversedBy string,
) (*domain.AffiliateEarning, error) {
	var result domain.AffiliateEarning

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var model affiliateEarningModel
		if err := tx.Clauses(lockForUpdate).
			Where("id = ?", earningID).
			First(&model).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return domain.ErrEarningNotFound
			}
			return fmt.Errorf("reverse earning: load: %w", err)
		}

		earning := *earningModelToDomain(model)
		if earning.Status == domain.EarningStatusReversed {
			result = earning
			return nil
		}

		sourceAccount := LedgerAccountPending
		switch earning.Status {
		case domain.EarningStatusAccrued, domain.EarningStatusPending:
			sourceAccount = LedgerAccountPending
		case domain.EarningStatusAvailable:
			sourceAccount = LedgerAccountAvailable
		default:
			return fmt.Errorf("reverse earning: %w (status %s)",
				domain.ErrEarningNotReversible, earning.Status)
		}

		if err := tx.Model(&affiliateEarningModel{}).
			Where("id = ?", earningID).
			Update("status", string(domain.EarningStatusReversed)).Error; err != nil {
			return fmt.Errorf("reverse earning: update status: %w", err)
		}

		postingKey := ledgerIdempotencyKey("reversal", earning.AffiliateID, "", earningID)
		if err := r.postLedgerTx(tx, earningID, earning.AffiliateID, []ledgerPosting{
			{
				AccountType:    sourceAccount,
				Direction:      LedgerDirectionDebit,
				Amount:         earning.CommissionAmount,
				ReferenceType:  LedgerRefReversal,
				ReferenceID:    earningID.String(),
				IdempotencyKey: postingKey,
				Metadata:       map[string]any{"reason": reason, "reversed_by": reversedBy},
			},
			{
				AccountType:    LedgerAccountReversed,
				Direction:      LedgerDirectionCredit,
				Amount:         earning.CommissionAmount,
				ReferenceType:  LedgerRefReversal,
				ReferenceID:    earningID.String(),
				IdempotencyKey: postingKey + ":credit",
				Metadata:       map[string]any{"reason": reason, "reversed_by": reversedBy},
			},
		}); err != nil {
			return err
		}

		if err := r.appendOutboxTx(tx, "affiliate_earning", earningID.String(),
			"affiliate.commission.reversed", earning.AffiliateID.String(), map[string]any{
				"earning_id":        earningID.String(),
				"affiliate_id":      earning.AffiliateID.String(),
				"referred_user_id":  earning.ReferredUserID,
				"commission_amount": earning.CommissionAmount.String(),
				"reason":            reason,
				"reversed_by":       reversedBy,
				"reversed_at":       time.Now().UTC(),
			}); err != nil {
			return err
		}

		earning.Status = domain.EarningStatusReversed
		result = earning
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}