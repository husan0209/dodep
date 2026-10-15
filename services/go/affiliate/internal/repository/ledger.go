package repository

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// errEarningMissing / errEarningTerminal are repository-level sentinels for
// the reversal path. The service pre-validates and maps them to its own
// typed errors (ErrEarningNotFound / ErrEarningNotReversible).
var (
	errEarningMissing  = errors.New("affiliate earning not found")
	errEarningTerminal = errors.New("affiliate earning is in a terminal status")
)

type affiliateLedgerEntryModel struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	TransactionID  uuid.UUID
	AffiliateID    uuid.UUID
	AccountID      uuid.UUID
	Direction      string
	Amount         decimal.Decimal
	BalanceAfter   decimal.Decimal
	ReferenceType  string
	ReferenceID    string
	IdempotencyKey string
	Metadata       map[string]any `gorm:"serializer:json"`
	CreatedAt      time.Time
}

func (affiliateLedgerEntryModel) TableName() string { return "affiliate_ledger_entries" }

// ledgerPosting is one side of a double-entry movement inside the affiliate
// ledger. Affiliate money NEVER touches the player wallet: pending,
// available, paid, reversed and adjusted are separate accounts.
type ledgerPosting struct {
	accountType    string
	direction      string // "debit" | "credit"
	amount         decimal.Decimal
	referenceType  string
	referenceID    string
	idempotencyKey string
}

// postLedgerTx writes a balanced set of postings in the caller's transaction.
// Zero-amount postings are skipped: affiliate_ledger_entries has a
// CHECK (amount > 0) constraint and a zero movement carries no information.
// Accounts are created on demand so backfilled affiliates stay consistent.
func (r *GormAffiliateRepository) postLedgerTx(tx *gorm.DB, affiliateID uuid.UUID, currency string, postings []ledgerPosting) error {
	transactionID := uuid.New()
	now := time.Now().UTC()

	for _, p := range postings {
		if p.amount.LessThanOrEqual(decimal.Zero) {
			continue
		}
		if p.direction != "debit" && p.direction != "credit" {
			return fmt.Errorf("invalid ledger direction %q", p.direction)
		}

		var account affiliateLedgerAccountModel
		err := tx.Where("affiliate_id = ? AND account_type = ? AND currency = ?", affiliateID, p.accountType, currency).First(&account).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				account = affiliateLedgerAccountModel{
					ID:          uuid.New(),
					AffiliateID: affiliateID,
					AccountType: p.accountType,
					Currency:    currency,
					Balance:     decimal.Zero,
					CreatedAt:   now,
					UpdatedAt:   now,
				}
				if err := tx.Create(&account).Error; err != nil {
					return fmt.Errorf("create ledger account %s: %w", p.accountType, err)
				}
			} else {
				return fmt.Errorf("load ledger account %s: %w", p.accountType, err)
			}
		}

		balanceAfter := account.Balance
		if p.direction == "credit" {
			balanceAfter = balanceAfter.Add(p.amount)
		} else {
			balanceAfter = balanceAfter.Sub(p.amount)
		}

		if err := tx.Model(&affiliateLedgerAccountModel{}).
			Where("id = ?", account.ID).
			Updates(map[string]any{"balance": balanceAfter, "updated_at": now}).Error; err != nil {
			return fmt.Errorf("update ledger account %s: %w", p.accountType, err)
		}

		entry := affiliateLedgerEntryModel{
			ID:             uuid.New(),
			TransactionID:  transactionID,
			AffiliateID:    affiliateID,
			AccountID:      account.ID,
			Direction:      p.direction,
			Amount:         p.amount,
			BalanceAfter:   balanceAfter,
			ReferenceType:  p.referenceType,
			ReferenceID:    p.referenceID,
			IdempotencyKey: p.idempotencyKey,
			Metadata:       map[string]any{"currency": currency},
			CreatedAt:      now,
		}
		if err := tx.Create(&entry).Error; err != nil {
			return fmt.Errorf("create ledger entry: %w", err)
		}
	}
	return nil
}

// profileCurrencyTx resolves the affiliate's ledger currency inside a tx.
func (r *GormAffiliateRepository) profileCurrencyTx(tx *gorm.DB, affiliateID uuid.UUID) (string, error) {
	var model affiliateProfileModel
	if err := tx.Where("id = ?", affiliateID).First(&model).Error; err != nil {
		return "", fmt.Errorf("load profile currency: %w", err)
	}
	if model.Currency == "" {
		return "USD", nil
	}
	return model.Currency, nil
}
