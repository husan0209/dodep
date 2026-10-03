-- Migration: 023_wallet_lock_settlement.sql
-- Description: Align Wallet Core identity/reference typing with the platform
--              and add first-class settlement (consume) for fund locks.
--
-- Why this migration exists
-- -------------------------
-- 1. `wallets/transactions/fund_locks.user_id` were declared UUID while the
--    whole platform identifies users by `users.id BIGSERIAL` and every Go /
--    Rust client sends the decimal form ("42"). wallet-core parses those
--    values as Uuid, so *every* gRPC call from other services failed with
--    INVALID_ARGUMENT "Invalid user_id". user_id becomes BIGINT.
--
-- 2. `reference_id` was UUID, but references are business identifiers owned
--    by other services (withdrawal UUID, bet id, idempotency key). Clients
--    cannot always produce a UUID, so reference_id becomes VARCHAR(255).
--
-- 3. Withdrawals locked funds on request and then called Debit on payout
--    success. Lock already moved the amount from available to locked, so the
--    subsequent Debit charged the player a *second* time. Locks could only be
--    unlocked, never settled. `consumed_at` + `settlement_transaction_id`
--    record the settlement so a lock is consumed exactly once.
--
-- 4. `wallets_available_ge_locked CHECK (balance_available >= balance_locked)`
--    is wrong: available and locked are disjoint buckets, so it blocks legal
--    states such as available=50 / locked=200.
--
-- 5. `fund_locks_reference_unique UNIQUE (reference_id)` blocked a second
--    lock for the same reference forever (even after release). Replaced by a
--    partial unique index over ACTIVE locks only, scoped per user, which is
--    what makes locking idempotent.

-- ============================================================================
-- 1. Guard: UUID-keyed rows cannot be mapped onto BIGINT users
-- ============================================================================
DO $$
DECLARE
    wallets_count   BIGINT;
    transactions_count BIGINT;
    locks_count     BIGINT;
BEGIN
    SELECT COUNT(*) INTO wallets_count FROM wallets;
    SELECT COUNT(*) INTO transactions_count FROM transactions;
    SELECT COUNT(*) INTO locks_count FROM fund_locks;

    IF wallets_count > 0 OR transactions_count > 0 OR locks_count > 0 THEN
        RAISE EXCEPTION
            'wallet schema contains % wallets / % transactions / % fund_locks with UUID user_id; '
            'they cannot be mapped to users.id (BIGINT). Export the balances, truncate these tables '
            'and re-import them under the correct BIGINT user_id before retrying.',
            wallets_count, transactions_count, locks_count;
    END IF;
END $$;

-- ============================================================================
-- 2. user_id: UUID -> BIGINT
-- ============================================================================
-- The reconciliation views select wallets.user_id, so PostgreSQL refuses to
-- retype the column while they exist. They are recreated at the end.
DROP VIEW IF EXISTS wallet_reconciliation_alerts;
DROP VIEW IF EXISTS wallet_reconciliation;

ALTER TABLE wallets
    ALTER COLUMN user_id TYPE BIGINT USING user_id::text::bigint;

ALTER TABLE transactions
    ALTER COLUMN user_id TYPE BIGINT USING user_id::text::bigint;

ALTER TABLE fund_locks
    ALTER COLUMN user_id TYPE BIGINT USING user_id::text::bigint;

-- ============================================================================
-- 3. reference_id: UUID -> VARCHAR(255) (business references, not UUIDs)
-- ============================================================================
ALTER TABLE transactions
    ALTER COLUMN reference_id TYPE VARCHAR(255) USING reference_id::text;

ALTER TABLE ledger_entries
    ALTER COLUMN reference_id TYPE VARCHAR(255) USING reference_id::text;

ALTER TABLE fund_locks
    ALTER COLUMN reference_id TYPE VARCHAR(255) USING reference_id::text;

-- ============================================================================
-- 4. Fund locks: settlement columns + idempotent locking index
-- ============================================================================
ALTER TABLE fund_locks
    ADD COLUMN IF NOT EXISTS consumed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS settlement_transaction_id UUID REFERENCES transactions(id) ON DELETE SET NULL;

ALTER TABLE fund_locks
    ADD CONSTRAINT fund_locks_settlement_state CHECK (
        (consumed_at IS NULL     AND settlement_transaction_id IS NULL) OR
        (consumed_at IS NOT NULL AND settlement_transaction_id IS NOT NULL)
    );

-- released_at and consumed_at are mutually exclusive terminal states.
ALTER TABLE fund_locks
    ADD CONSTRAINT fund_locks_terminal_state CHECK (
        NOT (released_at IS NOT NULL AND consumed_at IS NOT NULL)
    );

ALTER TABLE fund_locks
    DROP CONSTRAINT IF EXISTS fund_locks_reference_unique;

-- At most ONE active lock per (user, reference_type, reference_id):
-- a retried Lock with the same business reference reuses the existing lock
-- instead of reserving the funds twice.
CREATE UNIQUE INDEX IF NOT EXISTS idx_fund_locks_active_reference
    ON fund_locks(user_id, reference_type, reference_id)
    WHERE is_active = TRUE;

CREATE INDEX IF NOT EXISTS idx_fund_locks_consumed
    ON fund_locks(consumed_at DESC)
    WHERE consumed_at IS NOT NULL;

-- ============================================================================
-- 5. Drop the invalid balance invariant
-- ============================================================================
ALTER TABLE wallets
    DROP CONSTRAINT IF EXISTS wallets_available_ge_locked;

-- ============================================================================
-- 5b. Double-entry bookkeeping needs TWO ledger rows per transaction
-- ============================================================================
-- UNIQUE (idempotency_key) contradicts double-entry: one transaction writes a
-- debit and a credit carrying the same idempotency key, so every credit/debit
-- failed with 23505. Correctness is enforced per transaction instead: exactly
-- one debit and one credit.
ALTER TABLE ledger_entries
    DROP CONSTRAINT IF EXISTS ledger_entries_idempotency_unique;

ALTER TABLE ledger_entries
    ADD CONSTRAINT ledger_entries_pair_unique UNIQUE (transaction_id, entry_type);

-- ============================================================================
-- 6. Reference indexes (now VARCHAR)
-- ============================================================================
DROP INDEX IF EXISTS idx_transactions_reference;
CREATE INDEX IF NOT EXISTS idx_transactions_reference
    ON transactions(reference_type, reference_id)
    WHERE reference_id IS NOT NULL;

DROP INDEX IF EXISTS idx_fund_locks_reference;
CREATE INDEX IF NOT EXISTS idx_fund_locks_reference
    ON fund_locks(reference_type, reference_id);

-- ============================================================================
-- 7. Reconciliation view must expose BIGINT user_id
-- ============================================================================
CREATE OR REPLACE VIEW wallet_reconciliation AS
SELECT
    w.id AS wallet_id,
    w.user_id,
    w.wallet_type,
    w.currency,
    (w.balance_available + w.balance_locked + w.balance_bonus) AS actual_balance,
    COALESCE(
        SUM(CASE WHEN le.entry_type = 'credit' THEN le.amount ELSE 0 END) -
        SUM(CASE WHEN le.entry_type = 'debit' THEN le.amount ELSE 0 END),
        0
    ) AS expected_balance,
    ABS(
        (w.balance_available + w.balance_locked + w.balance_bonus) -
        COALESCE(
            SUM(CASE WHEN le.entry_type = 'credit' THEN le.amount ELSE 0 END) -
            SUM(CASE WHEN le.entry_type = 'debit' THEN le.amount ELSE 0 END),
            0
        )
    ) AS discrepancy
FROM wallets w
LEFT JOIN ledger_entries le ON le.account_id = 'user_wallet:' || w.user_id::text || ':' || w.wallet_type::text || ':' || w.currency
GROUP BY w.id, w.user_id, w.wallet_type, w.currency;

CREATE OR REPLACE VIEW wallet_reconciliation_alerts AS
SELECT * FROM wallet_reconciliation WHERE discrepancy > 0.01;