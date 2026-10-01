package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// lockForUpdate serializes concurrent ledger mutations on the same account.
var lockForUpdate = clause.Locking{Strength: "UPDATE"}

// Ledger account types (affiliate_ledger_account_type enum).
const (
	LedgerAccountPending   = "pending"
	LedgerAccountAvailable = "available"
	LedgerAccountPaid      = "paid"
	LedgerAccountReversed  = "reversed"
	LedgerAccountAdjusted  = "adjusted"
)

// Ledger entry directions (affiliate_ledger_entry_direction enum).
const (
	LedgerDirectionDebit  = "debit"
	LedgerDirectionCredit = "credit"
)

// Reference types used in affiliate_ledger_entries.reference_type.
const (
	LedgerRefEarning    = "earning"
	LedgerRefRelease    = "release"
	LedgerRefPayout     = "payout"
	LedgerRefAdjustment = "adjustment"
	LedgerRefReversal   = "reversal"
)

type affiliateLedgerEntryModel struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	TransactionID  uuid.UUID `gorm:"type:uuid;not null"`
	AffiliateID    uuid.UUID `gorm:"type:uuid;not null"`
	AccountID      uuid.UUID `gorm:"type:uuid;not null"`
	Direction      string    `gorm:"type:affiliate_ledger_entry_direction;not null"`
	Amount         decimal.Decimal
	BalanceAfter   decimal.Decimal
	ReferenceType  string
	ReferenceID    string
	IdempotencyKey string
	Metadata       map[string]any `gorm:"serializer:json"`
	CreatedAt      time.Time
}

func (affiliateLedgerEntryModel) TableName() string { return "affiliate_ledger_entries" }

// ledgerPosting is a single account movement inside one ledger transaction.
type ledgerPosting struct {
	AccountType     string
	Direction       string
	Amount          decimal.Decimal
	ReferenceType   string
	ReferenceID     string
	IdempotencyKey  string
	Metadata        map[string]any
}

// postLedgerTx applies postings as one atomic ledger transaction:
//
//   - all postings share one transaction_id (double-entry pairing);
//   - accounts are locked FOR UPDATE so concurrent mutations serialize;
//   - balance_after is the running balance of the affected account;
//   - duplicate (transaction_id, account_id, direction) is ignored, which
//     makes replaying the same business transaction safe.
//
// The ledger account is resolved per affiliate (one currency per affiliate,
// created with the profile), so postings stay single-currency.
//
// Callers must already run inside a gorm transaction.
func (r *GormAffiliateRepository) postLedgerTx(
	tx *gorm.DB,
	transactionID uuid.UUID,
	affiliateID uuid.UUID,
	postings []ledgerPosting,
) error {
	now := time.Now().UTC()
	for _, p := range postings {
		if p.Amount.LessThanOrEqual(decimal.Zero) {
			return fmt.Errorf("ledger posting: non-positive amount %s", p.Amount)
		}
		if p.Direction != LedgerDirectionDebit && p.Direction != LedgerDirectionCredit {
			return fmt.Errorf("ledger posting: invalid direction %q", p.Direction)
		}

		var account affiliateLedgerAccountModel
		if err := tx.Clauses(lockForUpdate).
			Where("affiliate_id = ? AND account_type = ?", affiliateID, p.AccountType).
			Order("created_at ASC").
			First(&account).Error; err != nil {
			return fmt.Errorf("ledger posting: load account %s: %w", p.AccountType, err)
		}

		// Skip when this exact movement was already recorded.
		var exists int64
		if err := tx.Model(&affiliateLedgerEntryModel{}).
			Where("transaction_id = ? AND account_id = ? AND direction = ?",
				transactionID, account.ID, p.Direction).
			Count(&exists).Error; err != nil {
			return fmt.Errorf("ledger posting: check existing entry: %w", err)
		}
		if exists > 0 {
			continue
		}

		balanceAfter := account.Balance
		if p.Direction == LedgerDirectionCredit {
			balanceAfter = account.Balance.Add(p.Amount)
		} else {
			balanceAfter = account.Balance.Sub(p.Amount)
		}
		if balanceAfter.IsNegative() {
			return fmt.Errorf("ledger posting: %s account would go negative (%s)",
				p.AccountType, balanceAfter)
		}

		entry := affiliateLedgerEntryModel{
			ID:             uuid.New(),
			TransactionID:  transactionID,
			AffiliateID:    affiliateID,
			AccountID:      account.ID,
			Direction:      p.Direction,
			Amount:         p.Amount,
			BalanceAfter:   balanceAfter,
			ReferenceType:  p.ReferenceType,
			ReferenceID:    p.ReferenceID,
			IdempotencyKey: p.IdempotencyKey,
			Metadata:       p.Metadata,
			CreatedAt:      now,
		}
		if err := tx.Create(&entry).Error; err != nil {
			return fmt.Errorf("ledger posting: create entry: %w", err)
		}

		if err := tx.Model(&affiliateLedgerAccountModel{}).
			Where("id = ?", account.ID).
			Updates(map[string]any{
				"balance":    balanceAfter,
				"updated_at": now,
			}).Error; err != nil {
			return fmt.Errorf("ledger posting: update account balance: %w", err)
		}
	}
	return nil
}

// LedgerAccountBalance is a materialized balance of one ledger account.
type LedgerAccountBalance struct {
	AccountType string
	Currency    string
	Balance     decimal.Decimal
}

// LedgerReconciliationReport compares materialized ledger balances with
// balances derived from business tables (earnings, payouts, adjustments).
type LedgerReconciliationReport struct {
	AffiliateID uuid.UUID
	Currency    string
	// Ledger holds materialized account balances.
	Ledger map[string]decimal.Decimal
	// Derived holds balances recomputed from business tables.
	Derived map[string]decimal.Decimal
	// Divergences lists "account_type: ledger=.. derived=.." for mismatches.
	Divergences []string
	// Balanced is true when every account matches.
	Balanced bool
}

// ledgerIdempotencyKey builds a deterministic, unique-per-movement key.
// The upstream idempotency key is preferred when present; otherwise the
// business reference id makes replays idempotent.
func ledgerIdempotencyKey(scope string, affiliateID uuid.UUID, upstreamKey string, referenceID uuid.UUID) string {
	key := scope + ":" + affiliateID.String()
	if upstreamKey != "" {
		return key + ":" + upstreamKey
	}
	return key + ":" + referenceID.String()
}

// GetLedgerBalances returns the materialized balance of every ledger account.
func (r *GormAffiliateRepository) GetLedgerBalances(
	ctx context.Context,
	affiliateID uuid.UUID,
) ([]LedgerAccountBalance, error) {
	var models []affiliateLedgerAccountModel
	if err := r.db.WithContext(ctx).
		Where("affiliate_id = ?", affiliateID).
		Order("account_type ASC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("get ledger balances: %w", err)
	}
	out := make([]LedgerAccountBalance, 0, len(models))
	for _, m := range models {
		out = append(out, LedgerAccountBalance{
			AccountType: m.AccountType,
			Currency:    m.Currency,
			Balance:     m.Balance,
		})
	}
	return out, nil
}

// ReconcileLedger recomputes ledger balances from the business tables and
// compares them with the materialized ones.
//
// Derived model:
//   - pending   = SUM(earnings accrued|pending)
//   - available = SUM(earnings available|paid) - SUM(payouts requested..paid)
//   - paid      = SUM(payouts paid)
//   - reversed  = SUM(earnings reversed)
//   - adjusted  = SUM(adjustments credit) - SUM(adjustments debit)
func (r *GormAffiliateRepository) ReconcileLedger(
	ctx context.Context,
	affiliateID uuid.UUID,
) (*LedgerReconciliationReport, error) {
	db := r.db.WithContext(ctx)

	profile, err := r.GetProfileByID(ctx, affiliateID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, fmt.Errorf("reconcile ledger: affiliate %s not found", affiliateID)
	}
	currency := profile.Currency

	derived := map[string]decimal.Decimal{
		LedgerAccountPending:   decimal.Zero,
		LedgerAccountAvailable: decimal.Zero,
		LedgerAccountPaid:      decimal.Zero,
		LedgerAccountReversed:  decimal.Zero,
		LedgerAccountAdjusted:  decimal.Zero,
	}

	// pgx returns NUMERIC as a string; scan into string and parse with
	// decimal (scanning straight into decimal.Decimal fails).
	scalar := func(query string, args ...any) (decimal.Decimal, error) {
		var row struct {
			Amount string
		}
		// COALESCE(...) needs an explicit alias: GORM maps by column name.
		if err := db.Raw(query, args...).Scan(&row).Error; err != nil {
			return decimal.Zero, err
		}
		if row.Amount == "" {
			return decimal.Zero, nil
		}
		parsed, err := decimal.NewFromString(row.Amount)
		if err != nil {
			return decimal.Zero, fmt.Errorf("parse numeric %q: %w", row.Amount, err)
		}
		return parsed, nil
	}

	var err2 error
	if derived[LedgerAccountPending], err2 = scalar(
		`SELECT COALESCE(SUM(commission_amount),0) AS amount FROM affiliate_earnings
		 WHERE affiliate_id = ? AND status IN ('accrued','pending')`,
		affiliateID,
	); err2 != nil {
		return nil, fmt.Errorf("reconcile pending: %w", err2)
	}

	earned, err := scalar(
		`SELECT COALESCE(SUM(commission_amount),0) AS amount FROM affiliate_earnings
		 WHERE affiliate_id = ? AND status IN ('available','paid')`,
		affiliateID,
	)
	if err != nil {
		return nil, fmt.Errorf("reconcile available: %w", err)
	}
	payouted, err := scalar(
		`SELECT COALESCE(SUM(amount),0) AS amount FROM affiliate_payouts
		 WHERE affiliate_id = ? AND status IN
		       ('requested','reviewing','approved','processing','paid')`,
		affiliateID,
	)
	if err != nil {
		return nil, fmt.Errorf("reconcile payouts: %w", err)
	}
	derived[LedgerAccountAvailable] = earned.Sub(payouted)

	if derived[LedgerAccountPaid], err = scalar(
		`SELECT COALESCE(SUM(amount),0) AS amount FROM affiliate_payouts
		 WHERE affiliate_id = ? AND status = 'paid'`,
		affiliateID,
	); err != nil {
		return nil, fmt.Errorf("reconcile paid: %w", err)
	}

	if derived[LedgerAccountReversed], err = scalar(
		`SELECT COALESCE(SUM(commission_amount),0) AS amount FROM affiliate_earnings
		 WHERE affiliate_id = ? AND status = 'reversed'`,
		affiliateID,
	); err != nil {
		return nil, fmt.Errorf("reconcile reversed: %w", err)
	}

	creditAdj, err := scalar(
		`SELECT COALESCE(SUM(amount),0) AS amount FROM affiliate_adjustments
		 WHERE affiliate_id = ? AND type = 'credit'`,
		affiliateID,
	)
	if err != nil {
		return nil, fmt.Errorf("reconcile adjustments credit: %w", err)
	}
	debitAdj, err := scalar(
		`SELECT COALESCE(SUM(amount),0) AS amount FROM affiliate_adjustments
		 WHERE affiliate_id = ? AND type = 'debit'`,
		affiliateID,
	)
	if err != nil {
		return nil, fmt.Errorf("reconcile adjustments debit: %w", err)
	}
	derived[LedgerAccountAdjusted] = creditAdj.Sub(debitAdj)

	balances, err := r.GetLedgerBalances(ctx, affiliateID)
	if err != nil {
		return nil, err
	}
	ledger := make(map[string]decimal.Decimal, len(balances))
	for _, b := range balances {
		if b.Currency != currency {
			continue
		}
		ledger[b.AccountType] = b.Balance
	}

	report := &LedgerReconciliationReport{
		AffiliateID: affiliateID,
		Currency:    currency,
		Ledger:      ledger,
		Derived:     derived,
		Balanced:    true,
	}
	for accountType, want := range derived {
		got, ok := ledger[accountType]
		if !ok {
			// Account missing entirely is a divergence.
			report.Divergences = append(report.Divergences,
				fmt.Sprintf("%s: ledger=missing derived=%s", accountType, want))
			report.Balanced = false
			continue
		}
		if !got.Equal(want) {
			report.Divergences = append(report.Divergences,
				fmt.Sprintf("%s: ledger=%s derived=%s", accountType, got, want))
			report.Balanced = false
		}
	}
	return report, nil
}
