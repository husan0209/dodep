-- Migration: 029_withdrawals_hold_status.sql
-- Description: allow the 'held' status on the shared withdrawals table.
--
-- The manual-review flow (migration 022) converts status to TEXT with an
-- explicit CHECK constraint. Operators need a third decision besides
-- approve/reject: "hold" keeps the row in the review queue while
-- investigations run (funds stay locked), so the label must pass the
-- constraint before the admin BFF can persist it.
--
-- The constraint is replaced (not amended) — CHECK constraints are dropped
-- and re-added; PostgreSQL cannot edit their expression in place.

ALTER TABLE withdrawals
    DROP CONSTRAINT IF EXISTS withdrawals_status_check;

ALTER TABLE withdrawals
    ADD CONSTRAINT withdrawals_status_check CHECK (status::text IN (
        'pending_review',
        'held',
        'approved',
        'rejected',
        'processing',
        'sending',
        'sent',
        'finished',
        'failed',
        'cancelled'
    ));
