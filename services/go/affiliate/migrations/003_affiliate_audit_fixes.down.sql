-- 003 rollback: indexes are dropped; the currency column is kept on purpose.
-- Dropping a money column would destroy audit evidence, so DOWN only removes
-- what is safe to remove (backward-compatible down migration).

DROP INDEX IF EXISTS idx_affiliate_earnings_hold_release;
DROP INDEX IF EXISTS idx_affiliate_earnings_reversed;

-- NOTE: affiliate_adjustments.currency is intentionally NOT dropped here.
-- If a full revert is ever required, run:
--   ALTER TABLE affiliate_adjustments DROP COLUMN IF EXISTS currency;
