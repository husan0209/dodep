-- Migration: 022_payment_review.sql
-- Description: Manual-review queue for withdrawals + saved payment methods.
-- Reference design (risk-based review, AGA AML best practices):
--   * withdrawals below the auto-approve threshold keep the instant flow;
--   * withdrawals at/above the threshold enter `pending_review` with funds
--     locked in the wallet; a finance operator approves (PSP payout is
--     executed) or rejects (funds are unlocked) the request.
--
-- The status column is converted from the `withdrawal_status` ENUM to TEXT
-- with an explicit CHECK constraint instead of ALTER TYPE ... ADD VALUE,
-- which cannot run inside a transaction block. All literals are cast to
-- ::text explicitly: without the cast PostgreSQL coerces them to the
-- still-existing `withdrawal_status` enum and fails on the new labels.

-- ============================================================
-- 1. Withdrawals: review columns
-- ============================================================
ALTER TABLE withdrawals
    ADD COLUMN IF NOT EXISTS lock_id VARCHAR(100),
    ADD COLUMN IF NOT EXISTS decided_by VARCHAR(100),
    ADD COLUMN IF NOT EXISTS decided_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS decision_reason TEXT;

-- Allow rows without a PSP payout id (pending_review has no payout yet).
-- Empty string means "no provider id"; the partial unique index below
-- keeps real provider ids unique.
ALTER TABLE withdrawals
    ALTER COLUMN withdrawal_id DROP NOT NULL;

ALTER TABLE withdrawals
    DROP CONSTRAINT IF EXISTS withdrawals_withdrawal_id_key;

DROP INDEX IF EXISTS idx_withdrawals_withdrawal_id;

CREATE UNIQUE INDEX IF NOT EXISTS idx_withdrawals_withdrawal_id
    ON withdrawals(withdrawal_id)
    WHERE withdrawal_id IS NOT NULL AND withdrawal_id <> '';

-- Drop indexes on `status` BEFORE the type change: rebuilding them would
-- re-resolve their predicates against the enum type.
DROP INDEX IF EXISTS idx_withdrawals_status;
DROP INDEX IF EXISTS idx_withdrawals_review_queue;

-- Convert status to TEXT. (The withdrawal_status ENUM is left in place for
-- other consumers; this table no longer depends on it.)
ALTER TABLE withdrawals
    ALTER COLUMN status TYPE TEXT USING status::text;

-- The old default is still typed as the enum; re-cast it so inserts that
-- omit `status` do not depend on the enum type.
ALTER TABLE withdrawals
    ALTER COLUMN status SET DEFAULT 'processing'::text;

ALTER TABLE withdrawals
    DROP CONSTRAINT IF EXISTS withdrawals_status_check;

ALTER TABLE withdrawals
    ADD CONSTRAINT withdrawals_status_check CHECK (status::text IN (
        'pending_review',
        'approved',
        'rejected',
        'processing',
        'sending',
        'sent',
        'finished',
        'failed',
        'cancelled'
    ));

-- Re-create the queue indexes (explicit ::text casts keep the predicates
-- resolvable regardless of any enum type with the same labels).
CREATE INDEX IF NOT EXISTS idx_withdrawals_status
    ON withdrawals(status)
    WHERE status::text IN ('pending_review', 'approved', 'processing', 'sending');

CREATE INDEX IF NOT EXISTS idx_withdrawals_review_queue
    ON withdrawals(status, created_at ASC)
    WHERE status::text = 'pending_review';

-- ============================================================
-- 2. Saved payment methods (details encrypted by the application)
-- ============================================================
CREATE TABLE IF NOT EXISTS payment_methods (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           BIGINT NOT NULL,
    method_type       VARCHAR(30) NOT NULL,
    provider          VARCHAR(100) NOT NULL,
    nickname          VARCHAR(100) NOT NULL DEFAULT '',
    display_value     VARCHAR(255) NOT NULL DEFAULT '',
    details_encrypted TEXT NOT NULL,
    is_default        BOOLEAN NOT NULL DEFAULT FALSE,
    is_active         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_payment_methods_user
    ON payment_methods(user_id, is_active)
    WHERE is_active = TRUE;