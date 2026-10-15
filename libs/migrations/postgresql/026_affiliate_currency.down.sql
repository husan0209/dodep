-- Migration 026 DOWN: affiliate_adjustments.currency
-- Rollback for 026_affiliate_currency.sql. Removes the mirrored column.
-- WARNING: dropping the column discards stored currencies; backfilled values
-- cannot be recovered by re-running 026 (profiles still hold them, but rows
-- created after the drop default to 'USD' until the next backfill).

ALTER TABLE affiliate_adjustments DROP COLUMN IF EXISTS currency;
