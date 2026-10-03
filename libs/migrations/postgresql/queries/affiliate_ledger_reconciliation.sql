-- Affiliate ledger reconciliation (production safety net)
--
-- WHY THIS EXISTS
--   GormAffiliateRepository.ReconcileLedger (services/go/affiliate/internal/
--   repository/ledger.go) is on-demand and per-affiliate: it is reachable only
--   through GET /admin/affiliates/:id/ledger/reconciliation, i.e. only runs when
--   somebody thinks to run it. Affiliate money must never drift silently
--   (architecture-overview: "RULE 1: MONEY MUST NEVER BE LOST OR DUPLICATED";
--   discrepancy > $0.01 => P1). This script is the scheduled, fleet-wide check.
--
-- DERIVED MODEL — must stay byte-identical to ReconcileLedger in ledger.go:
--   pending   = SUM(earnings.commission_amount) WHERE status IN ('accrued','pending')
--   available = SUM(earnings) WHERE status IN ('available','paid')
--               - SUM(payouts.amount)
--                 WHERE status IN ('requested','reviewing','approved','processing','paid')
--   paid      = SUM(payouts.amount) WHERE status = 'paid'
--   reversed  = SUM(earnings.commission_amount) WHERE status = 'reversed'
--   adjusted  = SUM(adjustments.amount WHERE type='credit')
--               - SUM(adjustments.amount WHERE type='debit')
--
-- If you change the derived model in Go, change it here in the same commit.
--
-- HOW TO RUN
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f libs/migrations/postgresql/queries/affiliate_ledger_reconciliation.sql
--
-- Exit code 1 when any divergence exceeds $0.01 => wire into alerting (P1).
-- Read-only: SELECT only. Never mutates balances.
--
-- STATUS COVERAGE — every affiliate_payout_status must be explicitly decided:
--   moves money out of `available`:
--       'requested','reviewing','approved','processing','paid'
--   intentionally excluded (payout never left the platform, money stays available):
-- EXCLUDED_STATUSES: 'rejected','failed'
-- The exclusion is deliberate, not an omission: a rejected/failed payout is not
-- posted to the ledger (see UpdatePayoutStatus in gorm_repository.go — postings
-- happen only on PayoutStatusPaid), so subtracting it would understate available.
-- If payout posting ever moves earlier in the lifecycle, remove it from this list.
--
-- affiliate_earning_status coverage is explicit in section 1:
--   'accrued','pending'   -> pending bucket
--   'available','paid'    -> earned term of available bucket
--   'reversed'            -> reversed bucket

\set ON_ERROR_STOP on

-- ============================================================================
-- 1. Fleet-wide divergence report (the one to alert on)
-- ============================================================================
WITH derived AS (
    SELECT
        p.id AS affiliate_id,
        p.affiliate_code,
        p.currency,
        COALESCE(e.pending, 0)   AS derived_pending,
        COALESCE(e.earned, 0) - COALESCE(po.payouted, 0) AS derived_available,
        COALESCE(po.paid, 0)     AS derived_paid,
        COALESCE(e.reversed, 0)  AS derived_reversed,
        COALESCE(a.credit, 0) - COALESCE(a.debit, 0)   AS derived_adjusted
    FROM affiliate_profiles p
    LEFT JOIN (
        SELECT affiliate_id,
               SUM(commission_amount) FILTER (WHERE status IN ('accrued','pending')) AS pending,
               SUM(commission_amount) FILTER (WHERE status IN ('available','paid'))  AS earned,
               SUM(commission_amount) FILTER (WHERE status = 'reversed')           AS reversed
        FROM affiliate_earnings
        GROUP BY affiliate_id
    ) e ON e.affiliate_id = p.id
    LEFT JOIN (
        SELECT affiliate_id,
               SUM(amount) FILTER (
                   WHERE status IN ('requested','reviewing','approved','processing','paid')
               ) AS payouted,
               SUM(amount) FILTER (WHERE status = 'paid') AS paid
        FROM affiliate_payouts
        GROUP BY affiliate_id
    ) po ON po.affiliate_id = p.id
    LEFT JOIN (
        SELECT affiliate_id,
               SUM(amount) FILTER (WHERE type = 'credit') AS credit,
               SUM(amount) FILTER (WHERE type = 'debit')  AS debit
        FROM affiliate_adjustments
        GROUP BY affiliate_id
    ) a ON a.affiliate_id = p.id
),
materialized AS (
    SELECT
        affiliate_id,
        currency,
        SUM(balance) FILTER (WHERE account_type = 'pending')   AS m_pending,
        SUM(balance) FILTER (WHERE account_type = 'available') AS m_available,
        SUM(balance) FILTER (WHERE account_type = 'paid')      AS m_paid,
        SUM(balance) FILTER (WHERE account_type = 'reversed')  AS m_reversed,
        SUM(balance) FILTER (WHERE account_type = 'adjusted')  AS m_adjusted
    FROM affiliate_ledger_accounts
    GROUP BY affiliate_id, currency
)
SELECT
    d.affiliate_id,
    d.affiliate_code,
    d.currency,
    m.m_pending,
    d.derived_pending,
    m.m_available,
    d.derived_available,
    m.m_paid,
    d.derived_paid,
    m.m_reversed,
    d.derived_reversed,
    m.m_adjusted,
    d.derived_adjusted
FROM derived d
LEFT JOIN materialized m
       ON m.affiliate_id = d.affiliate_id
      AND m.currency = d.currency
-- Divergence > 0.01 in ANY bucket is a P1. LEAST/GREATEST keep NULL (no ledger
-- row yet) from silently passing the comparison.
WHERE COALESCE(ABS(COALESCE(m.m_pending,0)   - d.derived_pending),   0) > 0.01
   OR COALESCE(ABS(COALESCE(m.m_available,0) - d.derived_available), 0) > 0.01
   OR COALESCE(ABS(COALESCE(m.m_paid,0)      - d.derived_paid),      0) > 0.01
   OR COALESCE(ABS(COALESCE(m.m_reversed,0)  - d.derived_reversed),  0) > 0.01
   OR COALESCE(ABS(COALESCE(m.m_adjusted,0)  - d.derived_adjusted),  0) > 0.01
ORDER BY d.affiliate_id;

-- ============================================================================
-- 2. Double-entry invariant: every ledger transaction must sum to zero.
--    affiliate_ledger_entries is written as debit+credit pairs
--    (see postLedgerTx in gorm_repository.go), so any non-zero net means a
--    broken posting — e.g. a payout that debited without crediting.
-- ============================================================================
-- Note: 021_affiliates.sql defines transaction_id as a plain UUID column with
-- NO separate transactions table, so entries are grouped by that UUID directly.
SELECT
    transaction_id,
    affiliate_id,
    SUM(amount * CASE WHEN direction = 'debit' THEN 1 ELSE -1 END) AS net,
    COUNT(*) AS entries
FROM affiliate_ledger_entries
WHERE created_at >= now() - INTERVAL '7 days'
GROUP BY transaction_id, affiliate_id
HAVING SUM(amount * CASE WHEN direction = 'debit' THEN 1 ELSE -1 END) <> 0
ORDER BY affiliate_id;

-- ============================================================================
-- 3. Idle release queue: earnings past hold_until still in 'accrued'.
--    Must normally be empty — the hold-release worker (AFF-RB-01) picks these.
-- ============================================================================
SELECT
    affiliate_id,
    status,
    COUNT(*) AS stale_earnings,
    SUM(commission_amount) AS stuck_amount,
    MIN(hold_until) AS oldest_hold_until
FROM affiliate_earnings
WHERE status IN ('accrued','pending')
  AND hold_until < now() - INTERVAL '2 hours'
GROUP BY affiliate_id, status
ORDER BY oldest_hold_until;

-- ============================================================================
-- 4. Payout money left the ledger but business row disagrees.
--    A 'paid' payout with no matching 'paid' ledger credit is the signature of a
--    PSP payout that succeeded while our ledger posting failed.
-- ============================================================================
SELECT
    po.affiliate_id,
    po.id AS payout_id,
    po.amount,
    po.status,
    po.requested_at
FROM affiliate_payouts po
WHERE po.status = 'paid'
  AND NOT EXISTS (
      SELECT 1
      FROM affiliate_ledger_accounts la
      WHERE la.affiliate_id = po.affiliate_id
        AND la.account_type = 'paid'
        AND la.balance >= po.amount
  )
ORDER BY po.requested_at;

-- ============================================================================
-- 5. Backlog guard: unpublished outbox rows older than 10 minutes.
--    See runbook AFF-RB-01; alerting rule affiliate-outbox-stuck.
-- ============================================================================
SELECT
    COUNT(*) AS stuck_events,
    MIN(created_at) AS oldest_unpublished
FROM affiliate_outbox
WHERE published_at IS NULL
  AND created_at < now() - INTERVAL '10 minutes';

-- ============================================================================
-- NOTES / KNOWN LIMITS
-- ============================================================================
-- 1. Section 1 compares only the profile's primary currency
--    (matching ReconcileLedger, which also reads profile.Currency). Affiliates
--    paid in a second currency are NOT covered by this check — that is a gap in
--    the Go implementation too, not just here.
-- 2. Only section 1 is a hard divergence gate. Sections 2-5 are investigation
--    queries — they inform the alert payload, they are not pass/fail by
--    themselves. A non-empty section 3 or 5 during low-traffic hours can be
--    benign; a non-empty section 2 is never benign.