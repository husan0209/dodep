# Slots stabilization → Sports handoff

This document marks the end of the **slots (FSE) integration phase** and defines what to freeze/observe before starting the **sports API** phase.

## 1) Freeze window (48–72h)

During the freeze window:

- Do not change callback validation logic (signature, replay-window, idempotency).
- Do not change wallet/ledger side-effects without a rollback plan.
- Only allow hotfixes for production incidents.

## 2) Stability signals to monitor

- **Callback health**
  - `payment_slot_callbacks_total{status="processed"}`
  - `payment_slot_callbacks_total{status="duplicate"}`
  - `payment_errors_total`
- **Latency**
  - `payment_provider_latency_seconds` for `"slot_callback_<provider>"`
- **Balance drift**
  - Wallet reconciliation checks (no negative balances, no double-credit/double-debit per transaction id)

## 3) Operational readiness checklist

- Provider secrets are stored only in env/secret-manager.
- Provider ingress/WAF allowlists are in place (or explicitly accepted as a risk).
- Runbook exists and on-call knows rollback steps:
  - disable provider
  - restart service
  - keep logs for investigation

## 4) Sports phase kickoff (next phase)

When the freeze window is stable, start sports integration in a separate branch/phase with:

- dedicated API client + signature/auth
- separate webhook/callback surface (no mixing with slots callbacks)
- independent rollout plan and SLOs

