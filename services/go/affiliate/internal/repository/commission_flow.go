package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/opus-casino/affiliate/internal/domain"
)

// DailyAggregateDeltas are increments applied to one affiliate_daily_aggregates
// row. Money fields use decimal (NEVER float64); counts are int64.
type DailyAggregateDeltas struct {
	Clicks             int64
	Registrations      int64
	FTDCount           int64
	ActivePlayers      int64
	GGRAmount          decimal.Decimal
	NGRAmount          decimal.Decimal
	CommissionAccrued  decimal.Decimal
	CommissionReversed decimal.Decimal
}

type affiliateDailyAggregateModel struct {
	ID                 uuid.UUID `gorm:"type:uuid;primaryKey"`
	AffiliateID        uuid.UUID
	ReportDate         time.Time `gorm:"type:date"`
	Clicks             int64
	Registrations      int64
	FTDCount           int64
	ActivePlayers      int64
	GGRAmount          decimal.Decimal
	NGRAmount          decimal.Decimal
	CommissionAccrued  decimal.Decimal
	CommissionReversed decimal.Decimal
	CreatedAt          time.Time
}

func (affiliateDailyAggregateModel) TableName() string { return "affiliate_daily_aggregates" }

// UpsertDailyAggregate increments the daily aggregate row for one affiliate,
// creating it when absent. Additive counters only — ClickHouse owns
// corrections; Postgres aggregates feed the partner dashboard.
func (r *GormAffiliateRepository) UpsertDailyAggregate(ctx context.Context, affiliateID uuid.UUID, reportDate time.Time, deltas DailyAggregateDeltas) error {
	day := time.Date(reportDate.Year(), reportDate.Month(), reportDate.Day(), 0, 0, 0, 0, time.UTC)
	err := r.db.WithContext(ctx).Exec(`
		INSERT INTO affiliate_daily_aggregates
			(id, affiliate_id, report_date, clicks, registrations, ftd_count, active_players,
			 ggr_amount, ngr_amount, commission_accrued, commission_reversed, created_at)
		VALUES
			(gen_random_uuid(), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())
		ON CONFLICT (affiliate_id, report_date) DO UPDATE SET
			clicks = affiliate_daily_aggregates.clicks + EXCLUDED.clicks,
			registrations = affiliate_daily_aggregates.registrations + EXCLUDED.registrations,
			ftd_count = affiliate_daily_aggregates.ftd_count + EXCLUDED.ftd_count,
			active_players = affiliate_daily_aggregates.active_players + EXCLUDED.active_players,
			ggr_amount = affiliate_daily_aggregates.ggr_amount + EXCLUDED.ggr_amount,
			ngr_amount = affiliate_daily_aggregates.ngr_amount + EXCLUDED.ngr_amount,
			commission_accrued = affiliate_daily_aggregates.commission_accrued + EXCLUDED.commission_accrued,
			commission_reversed = affiliate_daily_aggregates.commission_reversed + EXCLUDED.commission_reversed
	`,
		affiliateID, day,
		deltas.Clicks, deltas.Registrations, deltas.FTDCount, deltas.ActivePlayers,
		deltas.GGRAmount, deltas.NGRAmount,
		deltas.CommissionAccrued, deltas.CommissionReversed,
	).Error
	if err != nil {
		return fmt.Errorf("upsert daily aggregate: %w", err)
	}
	return nil
}

// GetEarningByID returns one earning by primary key, or (nil, nil) when absent.
func (r *GormAffiliateRepository) GetEarningByID(ctx context.Context, earningID uuid.UUID) (*domain.AffiliateEarning, error) {
	var model affiliateEarningModel
	err := r.db.WithContext(ctx).Where("id = ?", earningID).First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get earning by id: %w", err)
	}
	return earningModelToDomain(model), nil
}

// GetEarningByIdempotencyKey supports idempotent accrual replay: a retry with
// the same key returns the already-created earning instead of a duplicate.
func (r *GormAffiliateRepository) GetEarningByIdempotencyKey(ctx context.Context, key string) (*domain.AffiliateEarning, error) {
	var model affiliateEarningModel
	err := r.db.WithContext(ctx).Where("idempotency_key = ?", key).First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get earning by idempotency key: %w", err)
	}
	return earningModelToDomain(model), nil
}

// GetPayoutByIdempotencyKey supports idempotent payout-request replay.
func (r *GormAffiliateRepository) GetPayoutByIdempotencyKey(ctx context.Context, key string) (*domain.AffiliatePayout, error) {
	var model affiliatePayoutModel
	err := r.db.WithContext(ctx).Where("idempotency_key = ?", key).First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get payout by idempotency key: %w", err)
	}
	return payoutModelToDomain(model), nil
}

// reversibleEarningStatuses are the only states a reversal may leave.
var reversibleEarningStatuses = []string{
	string(domain.EarningStatusAccrued),
	string(domain.EarningStatusPending),
	string(domain.EarningStatusAvailable),
}

// ReverseEarning flips one earning to reversed with a guarded conditional
// update (lost update safe), appends the reversed outbox event, mirrors the
// movement in the affiliate ledger, and bumps the daily aggregate. The
// human-readable reason travels in the outbox payload for audit.
func (r *GormAffiliateRepository) ReverseEarning(ctx context.Context, earningID uuid.UUID, reason string) (*domain.AffiliateEarning, error) {
	var result *domain.AffiliateEarning
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var model affiliateEarningModel
		if err := tx.Where("id = ?", earningID).First(&model).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("earning %s: %w", earningID, errEarningMissing)
			}
			return fmt.Errorf("load earning for reversal: %w", err)
		}
		if !isReversibleStatus(model.Status) {
			return fmt.Errorf("earning %s status %q: %w", earningID, model.Status, errEarningTerminal)
		}
		fromAccount := earningStatusAccount(model.Status)

		res := tx.Model(&affiliateEarningModel{}).
			Where("id = ? AND status IN ?", earningID, reversibleEarningStatuses).
			Update("status", string(domain.EarningStatusReversed))
		if res.Error != nil {
			return fmt.Errorf("reverse earning: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("earning %s: %w", earningID, errEarningTerminal)
		}
		model.Status = string(domain.EarningStatusReversed)

		currency, err := r.profileCurrencyTx(tx, model.AffiliateID)
		if err != nil {
			return err
		}

		if err := r.postLedgerTx(tx, model.AffiliateID, currency, []ledgerPosting{
			{accountType: fromAccount, direction: "debit", amount: model.CommissionAmount, referenceType: "earning_reversal", referenceID: model.ID.String(), idempotencyKey: "reverse:" + model.ID.String() + ":out"},
			{accountType: "reversed", direction: "credit", amount: model.CommissionAmount, referenceType: "earning_reversal", referenceID: model.ID.String(), idempotencyKey: "reverse:" + model.ID.String() + ":in"},
		}); err != nil {
			return err
		}

		if err := r.appendOutboxTx(tx, "affiliate_earning", model.ID.String(), "affiliate.commission.reversed", model.AffiliateID.String(), map[string]any{
			"earning_id":        model.ID.String(),
			"affiliate_id":      model.AffiliateID.String(),
			"referred_user_id":  model.ReferredUserID,
			"commission_amount": model.CommissionAmount.String(),
			"previous_status":   fromAccount,
			"status":            string(domain.EarningStatusReversed),
			"reason":            reason,
			"idempotency_key":   model.IdempotencyKey,
		}); err != nil {
			return err
		}

		day := time.Date(model.CreatedAt.Year(), model.CreatedAt.Month(), model.CreatedAt.Day(), 0, 0, 0, 0, time.UTC)
		if err := upsertDailyAggregateTx(tx, model.AffiliateID, day, DailyAggregateDeltas{
			CommissionReversed: model.CommissionAmount,
		}); err != nil {
			return err
		}

		result = earningModelToDomain(model)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func isReversibleStatus(status string) bool {
	for _, s := range reversibleEarningStatuses {
		if s == status {
			return true
		}
	}
	return false
}

// earningStatusAccount maps an earning lifecycle status to its ledger account.
// accrued and pending both sit in the pending account until hold release.
func earningStatusAccount(status string) string {
	if status == string(domain.EarningStatusAvailable) {
		return "available"
	}
	return "pending"
}

func upsertDailyAggregateTx(tx *gorm.DB, affiliateID uuid.UUID, day time.Time, deltas DailyAggregateDeltas) error {
	return tx.Exec(`
		INSERT INTO affiliate_daily_aggregates
			(id, affiliate_id, report_date, clicks, registrations, ftd_count, active_players,
			 ggr_amount, ngr_amount, commission_accrued, commission_reversed, created_at)
		VALUES
			(gen_random_uuid(), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())
		ON CONFLICT (affiliate_id, report_date) DO UPDATE SET
			clicks = affiliate_daily_aggregates.clicks + EXCLUDED.clicks,
			registrations = affiliate_daily_aggregates.registrations + EXCLUDED.registrations,
			ftd_count = affiliate_daily_aggregates.ftd_count + EXCLUDED.ftd_count,
			active_players = affiliate_daily_aggregates.active_players + EXCLUDED.active_players,
			ggr_amount = affiliate_daily_aggregates.ggr_amount + EXCLUDED.ggr_amount,
			ngr_amount = affiliate_daily_aggregates.ngr_amount + EXCLUDED.ngr_amount,
			commission_accrued = affiliate_daily_aggregates.commission_accrued + EXCLUDED.commission_accrued,
			commission_reversed = affiliate_daily_aggregates.commission_reversed + EXCLUDED.commission_reversed
	`,
		affiliateID, day,
		deltas.Clicks, deltas.Registrations, deltas.FTDCount, deltas.ActivePlayers,
		deltas.GGRAmount, deltas.NGRAmount,
		deltas.CommissionAccrued, deltas.CommissionReversed,
	).Error
}
