-- Migration 020: Resolve affiliate table/type collisions between the admin BFF
-- (013_admin_bff.sql) and the affiliate service (021_affiliates.sql).
--
-- ORDERING: this file is numbered 020 so it sorts AFTER 013_admin_bff.sql
-- (which creates the colliding objects) and BEFORE 021_affiliates.sql (which
-- must be able to create them). tools/migrate.sh applies files in filename
-- order; as 022 it would have run too late and 021 would still fail.
--
-- PROBLEM
-- 013_admin_bff.sql (Admin BFF, edited 2026-09-29) creates:
--   TYPE affiliate_status  = ('pending','active','suspended','rejected')
--   TYPE payout_status     = ('pending','approved','rejected','paid')
--   TYPE fraud_flag_status = ('open','under_review','resolved','dismissed')
--   TABLE affiliates, affiliate_payouts, fraud_flags, postback_logs
--
-- 021_affiliates.sql (affiliate-service, consolidated 2026-10-01) needs:
--   TYPE affiliate_status = ('pending_review','active','suspended','rejected','closed')
--   TABLE affiliate_payouts (UUID affiliate_id, method_id, idempotency_key, ...)
--
-- On a database migrated in file order (013 then 021):
--   1. `CREATE TYPE IF NOT EXISTS`-style guards in 021 see affiliate_status
--      already exists and SKIP it -> the affiliate service then stores
--      'pending_review'/'closed', which the admin enum does not accept.
--   2. `CREATE TABLE IF NOT EXISTS affiliate_payouts` is skipped because 013
--      already created a table with the SAME NAME but a different shape
--      (VARCHAR affiliate_id, no method_id/idempotency_key, NUMERIC(18,2)).
--      affiliate-service queries then fail at runtime.
-- Neither failure is loud: both objects exist, so a smoke check passes while
-- every payout write breaks.
--
-- RESOLUTION
-- Ownership (per CONVENTIONS / services/go/affiliate/README.md):
--   - affiliate_profiles / affiliate_payouts / affiliate_fraud_flags and their
--     enums belong to the affiliate service. The admin BFF is a read-mostly
--     projection (admin-bff/internal/models/affiliate_stats.go) and must NOT
--     own them.
--   - `affiliates`, `postback_logs` and the postback/deal-type fields are
--     admin-side only (deal_type, cpa_amount, postback_configs) and stay.
--
-- This migration renames the colliding admin-side objects out of the way so
-- that 021 can create the canonical affiliate-service objects.
--
-- This migration is written to be safe in either order:
--   - if 013 has not run yet, every step is a no-op (objects missing) and the
--     canonical names stay free for 021;
--   - if 013 already ran, the collision is repaired.
-- Renames are reversible with the paired DOWN section.

BEGIN;

-- ============================================================
-- 1. RENAME admin-side affiliate objects (if present)
-- ============================================================

-- Types: rename first so nothing depends on the old name mid-migration.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'affiliate_status') THEN
        -- Guard against double-apply.
        IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'admin_affiliate_status') THEN
            ALTER TYPE affiliate_status RENAME TO admin_affiliate_status;
        END IF;
    END IF;

    IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'payout_status') THEN
        IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'admin_payout_status') THEN
            ALTER TYPE payout_status RENAME TO admin_payout_status;
        END IF;
    END IF;

    IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'fraud_flag_status') THEN
        IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'admin_fraud_flag_status') THEN
            ALTER TYPE fraud_flag_status RENAME TO admin_fraud_flag_status;
        END IF;
    END IF;
END $$;

-- Tables: affiliates / postback_logs do not collide by name, but they are
-- moved under an admin_ prefix so the affiliate namespace is unambiguous.
DO $$
BEGIN
    IF to_regclass('public.affiliate_payouts') IS NOT NULL
       AND to_regclass('public.admin_affiliate_payouts') IS NULL THEN
        EXECUTE 'ALTER TABLE affiliate_payouts RENAME TO admin_affiliate_payouts';
    END IF;

    IF to_regclass('public.fraud_flags') IS NOT NULL
       AND to_regclass('public.admin_fraud_flags') IS NULL THEN
        EXECUTE 'ALTER TABLE fraud_flags RENAME TO admin_fraud_flags';
    END IF;

    IF to_regclass('public.affiliates') IS NOT NULL
       AND to_regclass('public.admin_affiliates') IS NULL THEN
        EXECUTE 'ALTER TABLE affiliates RENAME TO admin_affiliates';
    END IF;

    IF to_regclass('public.postback_logs') IS NOT NULL
       AND to_regclass('public.admin_postback_logs') IS NULL THEN
        EXECUTE 'ALTER TABLE postback_logs RENAME TO admin_postback_logs';
    END IF;
END $$;

-- Index names from 013 are not schema-qualified, so rename them to stay
-- consistent with their tables (PostgreSQL keeps the old index names otherwise).
DO $$
BEGIN
    IF to_regclass('public.idx_affiliate_payouts_affiliate') IS NOT NULL THEN
        EXECUTE 'ALTER INDEX idx_affiliate_payouts_affiliate RENAME TO idx_admin_affiliate_payouts_affiliate';
    END IF;
    IF to_regclass('public.idx_affiliate_payouts_status') IS NOT NULL THEN
        EXECUTE 'ALTER INDEX idx_affiliate_payouts_status RENAME TO idx_admin_affiliate_payouts_status';
    END IF;
    IF to_regclass('public.idx_fraud_flags_affiliate') IS NOT NULL THEN
        EXECUTE 'ALTER INDEX idx_fraud_flags_affiliate RENAME TO idx_admin_fraud_flags_affiliate';
    END IF;
    IF to_regclass('public.idx_fraud_flags_status') IS NOT NULL THEN
        EXECUTE 'ALTER INDEX idx_fraud_flags_status RENAME TO idx_admin_fraud_flags_status';
    END IF;
    IF to_regclass('public.idx_affiliates_user_id') IS NOT NULL THEN
        EXECUTE 'ALTER INDEX idx_affiliates_user_id RENAME TO idx_admin_affiliates_user_id';
    END IF;
    IF to_regclass('public.idx_affiliates_status') IS NOT NULL THEN
        EXECUTE 'ALTER INDEX idx_affiliates_status RENAME TO idx_admin_affiliates_status';
    END IF;
    IF to_regclass('public.idx_postback_logs_affiliate') IS NOT NULL THEN
        EXECUTE 'ALTER INDEX idx_postback_logs_affiliate RENAME TO idx_admin_postback_logs_affiliate';
    END IF;
    IF to_regclass('public.idx_postback_logs_status') IS NOT NULL THEN
        EXECUTE 'ALTER INDEX idx_postback_logs_status RENAME TO idx_admin_postback_logs_status';
    END IF;
END $$;

-- ============================================================
-- 2. NO compatibility views — deliberately.
-- ============================================================
-- An earlier draft created views named "affiliates" / "affiliate_payouts" /
-- "fraud_flags" / "postback_logs" so admin-bff/internal/models/affiliates.go
-- (which pins TableName() to those strings) would keep compiling. That is
-- WRONG and is not shipped here:
--
--   * admin-bff/internal/models/affiliate_stats.go ALSO pins
--     AffiliatePayoutAmount.TableName() == "affiliate_payouts" and intends the
--     AFFILIATE-SERVICE table (it is a read-only projection of 021). A view of
--     that name would capture the admin-side table and silently feed the admin
--     "paid payouts" aggregate from admin rows instead of affiliate rows.
--   * GORM Updates() cannot write through a simple view, so the four write
--     paths (approve/reject payout, resolve/dismiss fraud flag) would break.
--
-- Instead the physical tables keep the admin_ names and the Go models must be
-- updated to match (one-line TableName() changes):
--
--   models/affiliates.go        Affiliate       -> "admin_affiliates"
--                               AffiliatePayout -> "admin_affiliate_payouts"
--                               FraudFlag       -> "admin_fraud_flags"
--                               PostbackLog     -> "admin_postback_logs"
--
-- models/affiliate_stats.go is intentionally UNCHANGED: AffiliateProfile,
-- AffiliateAttribution, AffiliateDailyAggregate, AffiliateEarning and
-- AffiliatePayoutAmount keep pointing at the affiliate-service tables created
-- by 021 (affiliate_profiles, affiliate_attributions, affiliate_daily_aggregates,
-- affiliate_earnings, affiliate_payouts).
--
-- After this migration the two namespaces are disjoint and the only remaining
-- coupling is models/affiliate_stats.go:AffiliatePayoutAmount, which now
-- correctly resolves to the affiliate-service table because 021 was no longer
-- blocked from creating it.

COMMIT;

-- ============================================================
-- DOWN (paired)
-- ============================================================
-- BEGIN;
-- ALTER INDEX IF EXISTS idx_admin_affiliates_status RENAME TO idx_affiliates_status;
-- ALTER INDEX IF EXISTS idx_admin_affiliates_user_id RENAME TO idx_affiliates_user_id;
-- ALTER INDEX IF EXISTS idx_admin_affiliate_payouts_status RENAME TO idx_affiliate_payouts_status;
-- ALTER INDEX IF EXISTS idx_admin_affiliate_payouts_affiliate RENAME TO idx_affiliate_payouts_affiliate;
-- ALTER INDEX IF EXISTS idx_admin_fraud_flags_status RENAME TO idx_fraud_flags_status;
-- ALTER INDEX IF EXISTS idx_admin_fraud_flags_affiliate RENAME TO idx_fraud_flags_affiliate;
-- ALTER INDEX IF EXISTS idx_admin_postback_logs_status RENAME TO idx_postback_logs_status;
-- ALTER INDEX IF EXISTS idx_admin_postback_logs_affiliate RENAME TO idx_postback_logs_affiliate;
--
-- ALTER TABLE admin_postback_logs RENAME TO postback_logs;
-- ALTER TABLE admin_fraud_flags RENAME TO fraud_flags;
-- ALTER TABLE admin_affiliates RENAME TO affiliates;
-- ALTER TABLE admin_affiliate_payouts RENAME TO affiliate_payouts;
--
-- ALTER TYPE admin_fraud_flag_status RENAME TO fraud_flag_status;
-- ALTER TYPE admin_payout_status RENAME TO payout_status;
-- ALTER TYPE admin_affiliate_status RENAME TO affiliate_status;
-- COMMIT;