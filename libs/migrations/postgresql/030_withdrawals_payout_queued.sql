-- Migration: 030_withdrawals_payout_queued.sql
-- Description: exactly-once claim marker for the manual-review dispatcher.
--
-- The payment service dispatches finance-operator decisions: an approved
-- review row triggers the NOWPayments payout, a rejected row returns the
-- locked funds to the player's wallet. Both paths touch money, so the
-- dispatcher must process every decided row EXACTLY once, even with several
-- payment replicas running.
--
-- payout_queued_at is that marker: claimed rows are excluded from future
-- claims (WHERE payout_queued_at IS NULL) and the claim itself is a
-- conditional UPDATE ... FOR UPDATE SKIP LOCKED, so concurrent dispatchers
-- take disjoint rows. A claim is never released — an ambiguous failure keeps
-- the marker and is escalated to operators instead of being retried.

ALTER TABLE withdrawals
    ADD COLUMN IF NOT EXISTS payout_queued_at TIMESTAMPTZ;

-- Dispatcher poll: decided rows that have not been claimed yet.
CREATE INDEX IF NOT EXISTS idx_withdrawals_dispatch_queue
    ON withdrawals(decided_at ASC, id ASC)
    WHERE payout_queued_at IS NULL
      AND decided_by IS NOT NULL
      AND status IN ('approved', 'rejected');
