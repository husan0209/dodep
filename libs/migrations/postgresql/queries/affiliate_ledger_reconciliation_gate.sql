-- Affiliate ledger reconciliation — PASS/FAIL GATE
--
-- Companion to affiliate_ledger_reconciliation.sql (the full evidence report).
-- This file exists because the report prints several result sets and therefore
-- cannot be used directly as an exit code.
--
-- Contract: prints exactly ONE row, ONE integer.
--   0 => ledger is balanced across the fleet
--   >0 => number of affiliates with a divergence > 0.01 in any bucket
--
-- The runner (infra/k8s/data/postgresql/affiliate-reconciliation-cronjob.yaml)
-- turns that integer into the job exit code.
--
-- DERIVED MODEL — identical to ReconcileLedger in
-- services/go/affiliate/internal/repository/ledger.go and to section 1 of
-- affiliate_ledger_reconciliation.sql:
--   pending   = SUM(earnings.commission_amount) WHERE status IN ('accrued','pending')
--   available = SUM(earnings) WHERE status IN ('available','paid')
--               - SUM(payouts.amount)
--                 WHERE status IN ('requested','reviewing','approved','processing','paid')
--   paid      = SUM(payouts.amount) WHERE status = 'paid'
--   reversed  = SUM(earnings.commission_amount) WHERE status = 'reversed'
--   adjusted  = SUM(adjustments.amount WHERE type='credit')
--               - SUM(adjustments.amount WHERE type='debit')
--
-- 'rejected' and 'failed' payouts are intentionally excluded: the ledger is
-- posted only on PayoutStatusPaid (see UpdatePayoutStatus in
-- gorm_repository.go). tools/testing/affiliate/consistency-check.js enforces
-- that every enum member is either listed here or declared in the EXCLUDED_STATUSES
-- block of the report file, so a future status cannot be silently forgotten.
--
-- Read-only: SELECT only.

WITH derived AS (
    SELECT
        p.id AS affiliate_id,
        p.currency,
        COALESCE(e.pending, 0) AS derived_pending,
        COALESCE(e.earned, 0) - COALESCE(po.payouted, 0) AS derived_available,
        COALESCE(po.paid, 0) AS derived_paid,
        COALESCE(e.reversed, 0) AS derived_reversed,
        COALESCE(a.credit, 0) - COALESCE(a.debit, 0) AS derived_adjusted
    FROM affiliate_profiles p
    LEFT JOIN (
        SELECT affiliate_id,
               SUM(commission_amount) FILTER (WHERE status IN ('accrued', 'pending')) AS pending,
               SUM(commission_amount) FILTER (WHERE status IN ('available', 'paid'))  AS earned,
               SUM(commission_amount) FILTER (WHERE status = 'reversed')             AS reversed
        FROM affiliate_earnings
        GROUP BY affiliate_id
    ) e  ON e.affiliate_id = p.id
    LEFT JOIN (
        SELECT affiliate_id,
               SUM(amount) FILTER (
                   WHERE status IN ('requested', 'paid')
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
SELECT count(*) AS diverging_affiliates
FROM derived d
LEFT JOIN materialized m
       ON m.affiliate_id = d.affiliate_id
      AND m.currency = d.currency
WHERE COALESCE(ABS(COALESCE(m.m_pending, 0)   - d.derived_pending),   0) > 0.01
   OR COALESCE(ABS(COALESCE(m.m_available, 0) - d.derived_available), 0) > 0.01
   OR COALESCE(ABS(COALESCE(m.m_paid, 0)      - d.derived_paid),      0) > 0.01
   OR COALESCE(ABS(COALESCE(m.m_reversed, 0)  - d.derived_reversed),  0) > 0.01
   OR COALESCE(ABS(COALESCE(m.m_adjusted, 0)  - d.derived_adjusted),  0) > 0.01;