-- Migration 026: affiliate_adjustments.currency (centralised mirror)
--
-- Drift fixed: services/go/affiliate/migrations/003_affiliate_audit_fixes.up.sql
-- added `currency` to affiliate_adjustments, but the centralised chain
-- (021_affiliates.sql, which consolidates service 001+002 only) never got it.
-- Any central-chain database therefore rejects the GORM manual-adjustment
-- insert. This file mirrors the service migration; 021 is NOT modified.
--
-- Mirror notes:
--   - ADD COLUMN IF NOT EXISTS keeps re-runs idempotent (same as service 003).
--   - Backfill copies the owning profile currency onto rows still holding the
--     'USD' default, exactly like service 003 (multi-currency ledger +
--     reconciliation requirement).
--   - The two affiliate_earnings covering indexes from service 003
--     (idx_affiliate_earnings_hold_release, idx_affiliate_earnings_reversed)
--     are intentionally NOT mirrored here: out of scope for this drift fix,
--     flagged as remaining work in the wave report.

BEGIN;

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

COMMIT;
