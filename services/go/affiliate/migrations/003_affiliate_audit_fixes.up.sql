-- 003: affiliate financial-audit fixes (additive only, backward compatible).
--
-- Gap: affiliate_adjustments was created without the currency column the
-- GORM model writes, so every manual adjustment failed on insert. Currency
-- is mandatory for a multi-currency ledger and for reconciliation.

ALTER TABLE affiliate_adjustments
    ADD COLUMN IF NOT EXISTS currency CHAR(3) NOT NULL DEFAULT 'USD';

-- Backfill from the owning affiliate profile where possible.
UPDATE affiliate_adjustments adj
SET currency = p.currency
FROM affiliate_profiles p
WHERE adj.affiliate_id = p.id
  AND (adj.currency IS NULL OR adj.currency = 'USD')
  AND p.currency IS NOT NULL
  AND p.currency <> 'USD';

-- Reversals and hold releases are reflected in reporting.
CREATE INDEX IF NOT EXISTS idx_affiliate_earnings_hold_release
    ON affiliate_earnings(status, hold_until)
    WHERE status IN ('accrued', 'pending');

CREATE INDEX IF NOT EXISTS idx_affiliate_earnings_reversed
    ON affiliate_earnings(affiliate_id, status)
    WHERE status = 'reversed';
