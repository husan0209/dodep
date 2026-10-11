#!/bin/sh
# Affiliate ledger reconciliation runner.
#
# Runs the gate query and turns its single integer into an exit code, so a
# Kubernetes CronJob can page on it. Kept in its own file so the SQL stays
# pure (read-only, psql-only) and this stays pure (shell-only).
#
# Contract of the gate: exactly one row, one integer.
#   0  -> balanced
#   >0 -> that many affiliates diverge by more than 0.01

set -eu

GATE_FILE="${GATE_FILE:-/sql/affiliate_ledger_reconciliation_gate.sql}"
REPORT_FILE="${REPORT_FILE:-/sql/affiliate_ledger_reconciliation.sql}"

: "${PGHOST:?PGHOST is required}"
: "${PGDATABASE:?PGDATABASE is required}"
: "${PGUSER:?PGUSER is required}"

# sslmode is pinned to the cluster setting; PGPASSWORD must come from a
# secretKeyRef, never from a literal in this script.
export PGSSLMODE="${PGSSLMODE:-require}"

echo "[reconcile] target=$PGHOST/$PGDATABASE as $PGUSER"

# --- Gate: the pass/fail signal -------------------------------------------
# -t -A  : tuples only, unaligned -> a single bare integer
# ON_ERROR_STOP: a broken query must fail loudly, never report "balanced".
gate_output="$(psql -v ON_ERROR_STOP=1 -t -A -f "$GATE_FILE")"

# Strip the value column; keep only the count column.
diverging="$(printf '%s' "$gate_output" | tr -d '[:space:]')"

case "$diverging" in
  ''|*[!0-9]*)
    echo "[reconcile] FAIL: gate did not return a single integer (got: '$gate_output')" >&2
    exit 2
    ;;
esac

echo "[reconcile] diverging_affiliates=$diverging"

if [ "$diverging" -ne 0 ]; then
  # Evidence first: the per-affiliate rows, same derived model as the gate.
  echo "[reconcile] ---- divergence detail (materialized vs derived) ----"
  psql -v ON_ERROR_STOP=1 -f "$REPORT_FILE" || \
    echo "[reconcile] warning: report query failed" >&2

  echo "[reconcile] FAIL: affiliate ledger divergence detected" >&2
  echo "[reconcile] action: runbook docs/infra/runbooks/affiliate-service.md" >&2
  echo "[reconcile] do NOT fix balances by hand - investigate, then correct the source" >&2
  exit 1
fi

echo "[reconcile] OK: affiliate ledger balanced across all affiliates"
exit 0