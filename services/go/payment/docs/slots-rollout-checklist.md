# Slots Rollout Checklist

Use this checklist to enable slot providers safely in production.

## 1) Secrets and config

- Set provider secrets via environment variables:
  - `PAYMENT_SLOTS_PROVIDERS_PRAGMATIC_SECRET`
  - `PAYMENT_SLOTS_PROVIDERS_PGSOFT_SECRET`
  - `PAYMENT_SLOTS_PROVIDERS_AMATIC_SECRET`
  - `PAYMENT_SLOTS_PROVIDERS_AMUSNET_SECRET`
- Keep `slots.replay_window_sec` between `120` and `300`.
- Enable only one provider at a time in `production.yaml`.

## 1.1) Network allowlist (recommended)

- Allow inbound traffic to `POST /api/v1/payments/webhooks/slots/:provider` **only from provider IP ranges** at the edge (ingress / WAF / reverse-proxy).
- If IP ranges are not stable, use a dedicated provider gateway IP and allowlist that instead.

## 2) Pre-go-live validation

- Signature validation works (invalid signature is rejected).
- Duplicate callback returns idempotent success without double processing.
- Old timestamp outside replay window is rejected.
- Metrics are visible:
  - `payment_slot_callbacks_total`
  - `payment_provider_latency_seconds`
  - `payment_errors_total`

### 2.1) Suggested PromQL checks

- Callback volume by provider:
  - `sum by (provider) (rate(payment_slot_callbacks_total[5m]))`
- Error volume:
  - `sum by (type) (rate(payment_errors_total[5m]))`
- Provider latency p95:
  - `histogram_quantile(0.95, sum by (le, provider) (rate(payment_provider_latency_seconds_bucket[5m])))`

## 3) Canary order

Recommended order:

1. `pgsoft`
2. `pragmatic`
3. `amatic`
4. `amusnet`

Enable next provider only after:

- stable callback success ratio,
- no balance drift,
- no abnormal `slot_signature_invalid` and replay-window errors.

## 4) Incident rollback

If anomalies are detected:

1. Set `slots.providers.<name>.enabled: false`.
2. Restart payment-service.
3. Keep collecting callbacks for investigation in logs.
4. Re-run signature and idempotency checks in staging before re-enable.

## 5) Step-by-step staged enablement

For each provider in canary order:

1. Set env secret: `PAYMENT_SLOTS_PROVIDERS_<PROVIDER>_SECRET`.
2. Set `slots.providers.<provider>.enabled: true`.
3. Deploy/restart `payment-service`.
4. Watch:
   - `payment_slot_callbacks_total{provider="<provider>",status="processed"}`
   - `payment_slot_callbacks_total{provider="<provider>",status="duplicate"}`
   - `payment_errors_total` for spikes
   - latency p95 for `"slot_callback_<provider>"`
5. Only then proceed to the next provider.
