# Runbook: Payment Service

Service: `services/go/payment` · HTTP 8084 · gRPC 50055 · metrics 9104
Chart: `infra/helm/charts/payment` · Namespace: `platform`
Alerts: `infra/k8s/monitoring/alerting/payment-alerting-rules.yaml`

Payment moves real money in both directions. Two of the scenarios below
(`PAY-RB-02`, `PAY-RB-03`) mean a player has paid and does not have the money,
or has money locked for a payout that never happened. Treat both as P1
regardless of how few players are affected.

## What the metrics mean

Recorded in `internal/observability/metrics.go`, written from
`internal/service/{payment,withdrawal,webhook}_service.go` and
`internal/client/nowpayments_client.go`.

| Series | Meaning |
|---|---|
| `payment_deposits_total{status}` | `pending` on request, then `completed` / `failed` / `expired` from the webhook |
| `payment_withdrawals_total{status}` | `processing` on request, then `completed` / `failed` / `cancelled` from the webhook |
| `payment_deposit_amount_usd` | Histogram, **one observation per request** |
| `payment_withdrawal_amount_usd` | Histogram, **one observation per request** |
| `payment_provider_latency_seconds{operation}` | NOWPayments call latency including retries |
| `payment_errors_total{error_type,operation}` | Bounded error classes, see below |

Two properties the dashboards rely on:

- **A deposit is counted once per lifecycle event, not once per deposit.**
  Summing `payment_deposits_total` across statuses gives the number of *state
  changes*. Use `completed / pending` for the settle rate, never a plain sum.
- **Idempotent replays are not counted.** A retried request with the same
  `idempotency_key`, and a provider webhook already seen, both return early and
  leave the counters untouched. Without this, client retries would inflate
  volume and completions would exceed the deposits actually opened.

`error_type` values are a fixed set. Raw provider and gRPC text is never used as
a label value — one crafted upstream error would otherwise mint a new time
series per request. See `metricErrorType` (domain errors), `walletErrorType`
(wallet-core gRPC) and `providerErrorType` (NOWPayments).

| `error_type` | `operation` | Meaning |
|---|---|---|
| `deposit_credit_failed` | `webhook` | Crypto arrived, wallet credit failed. **Player paid, no balance.** |
| `withdrawal_unlock_failed` | `webhook` | Payout failed, compensating unlock failed. **Balance stranded.** |
| `withdrawal_debit_not_finalized` | `webhook` | Payout settled, debit not finalized. **Crypto out, balance still locked.** |
| `compensation_failed` | `withdrawal` | Unlock failed in the request path after the payout failed |
| `deposit_status_not_persisted` / `withdrawal_status_not_persisted` | `webhook` | Wallet leg done, status write failed |
| `daily_limit_not_tracked` | `withdrawal` | Withdrawal created, daily cap not charged |
| `limit_exceeded` | `deposit` | Rejected by the daily deposit cap |
| `kyc_required` | `withdrawal` | Rejected: KYC level below 2 |
| `insufficient_balance` / `wallet_locked` / `wallet_unavailable` | `withdrawal` | wallet-core refused the lock |
| `wallet_not_found` / `wallet_rejected_request` | `withdrawal` | wallet-core has no wallet, or rejected the amount |
| `currency_not_supported` | `deposit` / `withdrawal` | Unsupported currency |
| `invalid_amount` / `invalid_address` | `deposit` / `withdrawal` | Rejected by the amount or address check |
| `provider_error` / `provider_5xx` / `provider_error_status` / `rejected` | provider ops | NOWPayments returned an error status (`rejected` = 4xx) |
| `transport_error` / `timeout` / `cancelled` | provider ops | Egress failure, deadline, or client gave up |
| `malformed_response` / `response_read_error` | provider ops | Provider body unreadable or not the expected JSON |
| `invalid_request` | provider ops | The request we built could not be sent |
| `webhook_invalid` / `webhook_malformed` | `webhook` | IPN rejected or unparseable |
| `deposit_lookup_failed` / `withdrawal_lookup_failed` | `webhook` | Webhook names a payment we cannot find |
| `deposit_amount_not_persisted` | `webhook` | Settled, but the actual-amount write failed |
| `persistence_failed` | `deposit` / `withdrawal` | Provider accepted, DB write failed |
| `idempotency_lookup_failed` | `deposit` / `withdrawal` | Redis unavailable for the replay check |
| `internal_error` | any | Unclassified. Investigate the log; do not alert on it directly |

`payment_lookup_failed` and `withdrawal_lookup_failed` are what the code records
(the runbook table uses the shorter `deposit_`/`withdrawal_` prefix for the
scenario names); grep for the exact literal when correlating with logs.

---

## PAY-RB-01: Payment Service Not Ready

**Severity:** P1 · **Alerts:** `payment-no-ready-pods`, `payment-crash-looping`

### Symptoms

`kubectl get pods -n platform -l app.kubernetes.io/name=payment` shows no
`Running`/`Ready`, or repeated restarts.

### Diagnose

```bash
kubectl get pods -n platform -l app.kubernetes.io/name=payment

kubectl describe pod -n platform <pod> | tail -40
kubectl logs -n platform <pod> --tail=200
kubectl logs -n platform <pod> --previous      # crash reason before the restart
```

Common causes, in the order they actually happen:

1. **`PAYMENT_NOWPAYMENTS_API_KEY` / `PAYMENT_NOWPAYMENTS_IPN_SECRET` missing.**
   `config.Validate()` fails at boot: *nowpayments api_key is required*.
2. **Redis unreachable.** The idempotency check is the first thing
   `InitiateDeposit` does, so a Redis outage looks like a total deposit outage.
3. **Postgres unreachable.** `database.host is required`, or a DSN that points
   at a stale primary.
4. **Crash after boot.** `cfg.Validate()` passes but a dependency dies in the
   errgroup; the log line before the exit names it.

### Mitigate

```bash
# Confirm the config error rather than guessing.
kubectl logs -n platform <pod> --tail=200 | grep -i "validate config"

# Roll back to the last known-good image.
kubectl rollout history deployment/payment -n platform
kubectl rollout undo deployment/payment -n platform
kubectl rollout status deployment/payment -n platform --timeout=5m
```

### Verify

```bash
kubectl get endpoints payment -n platform     # expect at least one address
curl -s http://payment.platform.svc.cluster.local:8084/healthz
```

In-flight deposits are **not** lost by a restart. They are `pending` rows whose
IPN callback will arrive later; the provider retries. The risk window is the
in-memory state that does not survive: read `payment_deposits_total{status="pending"}`
before and after, and expect the delta to settle.

### Do not

- Do not scale to zero to "stop the bleeding". A pod with no readiness still
  consumes the IPN queue and delays every callback.
- Do not delete pending payment rows. The provider will keep retrying the
  webhook for them.

---

## PAY-RB-02: Deposit Credited Without Wallet

**Severity:** P1 · **Alert:** `payment-deposit-credit-failed`, `payment-deposit-completion-stalled`

### Symptoms

`sum(rate(payment_errors_total{error_type="deposit_credit_failed"}[10m])) > 0`.
A deposit reached `finished` at NOWPayments, but `CreditWallet` to wallet-core
failed. The player sent crypto and sees no balance. Support sees nothing on
their side.

### Why it is safe to retry

`handleDepositFinished` credits with idempotency key `deposit:<payment_id>`.
wallet-core deduplicates on it, so **the credit itself is safe to replay**. The
webhook returns an error, so NOWPayments will retry on its own and the retry
will converge once wallet-core is healthy.

### Diagnose

```bash
# Which players are affected, and since when.
#   (no user_id label on purpose: the metric must not carry player identity)
#   Correlate by time window against the audit log instead.
kubectl logs -n platform -l app.kubernetes.io/name=payment --tail=500 \
  | grep "Failed to credit wallet"

# Is wallet-core the problem, or is it payment?
kubectl get endpoints wallet-core -n platform
kubectl logs -n platform -l app.kubernetes.io/name=wallet-core --tail=200

# Confirm the money is stranded at the provider and settled, not lost.
#   payment_id is in the log line above; look it up in the provider dashboard.
```

The distinction that matters:

- **`credit wallet: ...` in the payment log** → this scenario. Money is at the
  provider, the balance is missing.
- **`payment-no-ready-pods` also firing** → wallet-core is down, not payment.
  Fix PAY-RB-01 style for wallet-core first; the retries will resolve.

### Mitigate

Preferred: let the provider retry. Once wallet-core is healthy the next IPN
delivery credits the balance.

```bash
# Watch the error rate fall as retries land.
watch -n5 'curl -s http://victoriametrics.platform:8428/api/v1/query \
  --data-urlencode "query=sum(increase(payment_errors_total{error_type=\"deposit_credit_failed\"}[5m]))"'
```

If the provider has stopped retrying and the player is waiting:

```bash
# Replay the single webhook. Do NOT credit manually from the dashboard:
# a manual credit bypasses the wallet idempotency key and will double-credit
# if the provider retry also lands.
kubectl exec -n platform deploy/payment -- \
  curl -sS -X POST http://localhost:8084/api/v1/payments/webhooks/nowpayments \
    -H 'Content-Type: application/json' \
    -H "x-nowpayments-sig: <signature from the stored payload>" \
    --data-binary @/tmp/payload-<payment_id>.json
```

Retrieve the exact payload and signature from the audit log first. Do not
reconstruct the payload — the HMAC covers the raw bytes, and a re-serialized
JSON will not verify.

### Verify

```bash
# 1. The error counter stops rising.
# 2. The deposit row reached the terminal state.
psql "$DATABASE_URL" -c "
  SELECT uuid, payment_id, status, fiat_amount, completed_at
  FROM payments
  WHERE payment_id = '<payment_id>';"
#   expect status = 'finished'

# 3. The balance actually moved, exactly once.
psql "$DATABASE_URL" -c "
  SELECT id, type, amount, reference_id, created_at
  FROM wallet_transactions
  WHERE reference_id = '<payment_id>'
  ORDER BY created_at;"
#   expect exactly one row; two rows means a double credit — escalate to Payments
```

### Do not

- Do not credit the wallet by hand outside `handleDepositFinished`. The
  idempotency key is the only thing preventing a double credit when the
  provider retry lands.
- Do not set `status = 'finished'` in the database to "clear" the alert. The
  money is the problem, not the row.

---

## PAY-RB-03: Withdrawal Failed Without Unlock

**Severity:** P1 · **Alerts:** `payment-withdrawal-unlock-failed`, `payment-compensation-failed`, `payment-withdrawal-debit-not-finalized`

Three variants of the same class: money is stranded and the player cannot use
their balance.

| Variant | `error_type` | What happened | Player's position |
|---|---|---|---|
| A | `withdrawal_unlock_failed` | Payout failed, unlock failed | Balance locked, no crypto sent. **Should be whole.** |
| B | `compensation_failed` | Lock succeeded, payout or DB write failed, unlock failed | Balance locked, no crypto sent. **Should be whole.** |
| C | `withdrawal_debit_not_finalized` | Payout settled, debit not finalized | Crypto received, balance still locked. **Owed a debit.** |

Variant C is the only one where the player has received money. It is also the
easiest to miss, because from the player's side it looks like a success.

### Diagnose

```bash
kubectl logs -n platform -l app.kubernetes.io/name=payment --tail=500 \
  | grep -E "Failed to unlock funds|Failed to finalize debit"
```

Then find the lock:

```bash
psql "$DATABASE_URL" -c "
  SELECT id, user_id, lock_id, amount, status, created_at
  FROM wallet_locks
  WHERE reference_id = '<withdrawal_id>'
  ORDER BY created_at DESC;"
```

An `active` lock for a withdrawal whose provider status is `failed` is exactly
this incident. There is no TTL on these locks — nothing releases them on its own.

### Mitigate

```bash
# Confirm the payout really did not happen before touching the lock.
#   variant C: the provider dashboard shows 'finished' -> do NOT unlock.
#   variants A and B: no payout, or a failed payout -> unlock is correct.
```

For variants A and B, release the lock:

```bash
psql "$DATABASE_URL" -c "
  UPDATE wallet_locks
  SET status = 'released', released_at = now()
  WHERE lock_id = '<lock_id>' AND status = 'active';"
```

For variant C, finalize the debit instead — the player already has the crypto:

```bash
kubectl exec -n platform deploy/payment -- \
  curl -sS -X POST http://localhost:8084/api/v1/payments/webhooks/nowpayments \
    -H 'Content-Type: application/json' \
    -H "x-nowpayments-sig: <signature>" \
    --data-binary @/tmp/payload-<withdrawal_id>.json
```

`FinalizeDebit` uses idempotency key `withdrawal:<withdrawal_id>`, so the retry
is safe.

### Verify

```bash
psql "$DATABASE_URL" -c "
  SELECT l.lock_id, l.status, w.available, w.locked
  FROM wallet_locks l JOIN wallets w ON w.user_id = l.user_id
  WHERE l.lock_id = '<lock_id>';"
#   expect status = 'released' and locked back to its prior value

# The counter must stop rising.
# sum(rate(payment_errors_total{error_type=~"withdrawal_unlock_failed|compensation_failed"}[5m])) -> 0
```

### Do not

- Do not release a lock for variant C. The crypto is gone; releasing the lock
  would let the player spend the same balance twice.
- Do not delete `wallet_locks` rows. wallet-core tracks lock state; a deleted row
  breaks the reconciliation it uses to rebuild balances.

---

## PAY-RB-04: Payment State Out of Sync With Wallet

**Severity:** P2 · **Alerts:** `payment-status-not-persisted`, `payment-daily-limit-not-tracked`

### Symptoms

`payment_deposits_total{status="pending"}` is far above `{status="completed"}`,
or `daily-limit_not_tracked` is non-zero.

### Diagnose

```bash
# Deposits that opened but never reached a terminal state.
psql "$DATABASE_URL" -c "
  SELECT status, count(*), min(created_at) AS oldest
  FROM payments
  WHERE created_at > now() - interval '2 hours'
  GROUP BY status ORDER BY count DESC;"

# The specific rows behind the alert.
kubectl logs -n platform -l app.kubernetes.io/name=payment --tail=1000 \
  | grep -E "Failed to update payment status|Failed to track daily withdrawal"
```

Two distinct causes:

- **`UpdateStatus` failed.** The wallet leg succeeded but the row did not move.
  The webhook returned an error, so the provider retries and the retry
  converges — the wallet idempotency key prevents a second credit. Persistent
  failures mean a database problem, not a payment problem.
- **`dailyLimitsRepo.Increment` failed.** The withdrawal row exists and the
  payout is in flight, but the daily cap was not charged. **The player can
  exceed their daily withdrawal limit today.** This is the more serious of the
  two and is not self-healing.

### Mitigate

```bash
# Reconcile state from the wallet, which is the source of truth for money.
psql "$DATABASE_URL" -c "
  SELECT p.payment_id, p.status AS payment_status, t.id AS txn_id
  FROM payments p
  LEFT JOIN wallet_transactions t ON t.reference_id = p.payment_id
  WHERE p.status = 'pending'
    AND t.type = 'credit'
    AND p.created_at > now() - interval '2 hours';"
```

For each row, replay the webhook (PAY-RB-02) to move the state. Do not update
`payments.status` directly — the transition guard and the audit log both run
inside the service.

For the daily-limit gap, backfill the counter so the cap applies again:

```bash
psql "$DATABASE_URL" -c "
  INSERT INTO daily_limits (user_id, operation_type, amount, updated_at)
  SELECT user_id, 'withdrawal', SUM(fiat_amount), now()
  FROM withdrawals
  WHERE status IN ('processing', 'sent', 'finished')
    AND created_at::date = current_date
  GROUP BY user_id
  ON CONFLICT (user_id, operation_type)
  DO UPDATE SET amount = EXCLUDED.amount, updated_at = now();"
```

Read the current schema before running this; the column set is not asserted
here.

### Verify

```bash
psql "$DATABASE_URL" -c "
  SELECT count(*) FROM payments
  WHERE status = 'pending' AND created_at > now() - interval '2 hours';"
#   trending to 0

psql "$DATABASE_URL" -c "
  SELECT user_id, amount FROM daily_limits
  WHERE operation_type = 'withdrawal' ORDER BY amount DESC LIMIT 10;"
#   compare against wallet_transactions for today
```

---

## PAY-RB-05: Metrics Endpoint Missing

**Severity:** P2 · **Alert:** `payment-metrics-absent`

### Why this one is urgent despite the low severity

`absent(payment_deposits_total)` means the exposition is gone, so **every other
rule in this file is silently evaluating an empty series.** Prometheus reports
no problems. This is the one alert whose failure mode is "all other alerts stop
working".

### Diagnose

```bash
# 1. Is the exposition actually there?
kubectl port-forward -n platform svc/payment 9104:9104 &
curl -s http://localhost:9104/metrics | grep -c '^payment_'

# 2. Is the Service exposing a port named "metrics"?
kubectl get svc payment -n platform -o jsonpath='{.spec.ports[*].name}'

# 3. Is the ServiceMonitor matching anything?
kubectl get servicemonitor -n platform | grep payment
kubectl describe servicemonitor -n platform <name>

# 4. Is the network policy blocking the scrape?
kubectl get networkpolicy -n platform -l app.kubernetes.io/name=payment
```

### Known configuration gap

At the time of writing, `infra/helm/charts/payment` does **not** render a
working scrape path, in three independent ways. All three must be fixed before
these alerts can fire:

1. **The Service has no port named `metrics`.** `values.yaml` sets
   `serviceMonitor.port: metrics`, but the `service` block only declares
   `httpPort`/`grpcPort`. Prometheus matches `ServiceMonitor` endpoints by port
   *name*, so the selector resolves to nothing.
2. **The metrics listener is on a separate port.** The service serves
   `/metrics` on `:9104` (its own `http.ServeMux`), not on the API port `:8084`.
   Even a correctly named port must target `9104`.
3. **The NetworkPolicy does not admit it.** Ingress from the `monitoring`
   namespace allows only `8080` and `9090`. `:9104` is refused, so the scrape
   fails even once the port is named.

Note also that the chart declares `service-chart` as a dependency but its
`values.yaml` is **not** nested under a `service-chart:` key, so the subchart
receives none of these values. `bonus` nests correctly and is the reference to
follow. Fixing this touches the shared `service-chart` templates and other
agents' charts, so it is deliberately left out of the metrics-wiring change.

`infra/docker/prometheus/prometheus.yml` scrapes `payment:9104` via a static
target, which is why the endpoint is reachable in the docker-compose stack but
not in the cluster.

### Mitigate

Until the chart is fixed, verify payment behaviour by hand:

```bash
kubectl port-forward -n platform svc/payment 9104:9104 &
curl -s http://localhost:9104/metrics | grep '^payment_'
```

Treat every alert in this file as unavailable during this window. Manual checks
substitute for `PAY-RB-02` and `PAY-RB-03`:

```bash
kubectl logs -n platform -l app.kubernetes.io/name=payment --tail=200 \
  | grep -cE "Failed to credit wallet|Failed to unlock funds|Failed to finalize debit"
```

### Do not

- Do not silence `payment-metrics-absent` to make it go away. It is the only
  signal that the other 13 rules are blind.

---

## PAY-RB-06: Provider Degraded

**Severity:** P2 · **Alerts:** `payment-provider-latency-high`, `payment-provider-error-rate-high`

### Symptoms

p95 `create_payment`/`create_payout` latency above 5s, or the provider error
rate above 2%.

### Diagnose

```bash
# Latency by operation.
watch -n5 'curl -s "http://victoriametrics.platform:8428/api/v1/query?query=histogram_quantile(0.95,sum+by+(le)(rate(payment_provider_latency_seconds_bucket[5m])))"'

# Error classes.
watch -n5 'curl -s "http://victoriametrics.platform:8428/api/v1/query?query=sum+by+(error_type)(rate(payment_errors_total{operation=~\"create_payment|create_payout\"}[5m]))"'
```

The `operation` label splits the calls, which tells you what is actually
broken:

- Only `get_rate` slow → the exchange-rate path. Cached rates hide this for up
  to 60s, so it presents as intermittent.
- `create_payment` and `create_payout` both slow → the provider itself.
- Errors are `provider_5xx` → the provider is failing. Nothing to fix locally.
- Errors are `transport_error` or `timeout` → our egress. Check the Istio
  sidecar and DNS.

```bash
istioctl proxy-status | grep payment
kubectl logs -n platform <payment-pod> -c istio-proxy --tail=100
```

### Mitigate

The client already retries with exponential backoff
(`nowpayments.retry.*`, defaults 3 attempts, 100ms → 100ms×2^n, capped at 100ms
base / 5s max). Confirm the retry budget is not the bottleneck:

```bash
kubectl get cm payment-config -n platform -o yaml | grep -A5 retry
```

If the provider is returning 5xx, the correct action is to wait it out. Player
impact is deposits and withdrawals failing at the provider, which produces
`provider_error` on `create_payment`/`create_payout`. Those deposits never
reach `pending`, so there is nothing to reconcile — the player was not charged
and no pay address was issued.

If a partial outage affects one currency, the `currency` label on the counters
localises it:

```bash
curl -s "http://victoriametrics.platform:8428/api/v1/query?query=sum+by+(currency)(increase(payment_errors_total{error_type=\"provider_error\"}[15m]))"
```

### Verify

```bash
# Latency back under budget and the error rate under 2%.
# Then confirm deposits are flowing again.
psql "$DATABASE_URL" -c "
  SELECT count(*) FROM payments
  WHERE created_at > now() - interval '10 minutes';"
```

### Adjacent: a spike in business-rule rejections

`payment-limit-rejection-spike` fires when more than 1/s of deposits are refused
for exceeding the daily cap. It usually means a limits change shipped by mistake
— `getDepositLimit` is keyed by KYC level (0 → $500, 1 → $2,000, 2 → $10,000,
3 → $50,000 per day), so a wrong KYC level from User Service collapses every
player onto one cap.

```bash
# Which KYC level is rejecting, and for whom.
curl -s "http://victoriametrics.platform:8428/api/v1/query?query=sum+by+(currency)(increase(payment_errors_total{error_type=\"limit_exceeded\"}[15m]))"

kubectl logs -n platform -l app.kubernetes.io/name=user --tail=200 | grep -i kyc
```

If it clusters on many distinct users, suspect a config or KYC-propagation fault.
If it clusters on a handful of accounts attempting large deposits repeatedly,
that is consistent with attempting to move funds through many profiles — route to
the Fraud/ML on-call rather than tuning the cap. See PAY-RB-04 for repairing the
`daily_limits` table after the fact.

### Do not

- Do not raise `nowpayments.retry.max_retries` to paper over a provider outage.
  Each extra attempt holds a request handler and a wallet lock longer.
- Do not clear the `deposit-completion-stalled` alert by restarting payment. The
  pending deposits are waiting on the provider, not on us.

---

## PAY-RB-07: Webhooks Not Settling

**Severity:** P1 · **Alerts:** `payment-webhook-rejected-spike`, `payment-deposit-completion-stalled`

### Symptoms

A sustained rate of `webhook_invalid` or `webhook_malformed`. Every rejected
callback is a deposit or withdrawal that will not settle. This is P1 even
without any other symptom, because it silently stops all settlement.

### Diagnose

```bash
kubectl logs -n platform -l app.kubernetes.io/name=payment --tail=300 \
  | grep -E "Invalid webhook signature|parse webhook payload"
```

The two error types have different causes and must not be conflated:

**`webhook_invalid`** — the HMAC did not verify. Checked first, before parsing,
so the signature is verified over the exact bytes received.

```bash
# The secret the service is using. Compare against the provider dashboard.
kubectl get secret payment-secrets -n platform -o jsonpath='{.data.nowpayments-ipn-secret}' | base64 -d | wc -c
kubectl get cm payment-config -n platform -o yaml | grep -i ipn
```

Also verify the callback URL is still ours. If it points at an old host or an
expired certificate, the provider's requests fail before they reach us — which
would not show up here at all.

```bash
kubectl get cm payment-config -n platform -o jsonpath='{.data.nowpayments-ipn-callback-url}'
```

**`webhook_malformed`** — the signature verified but the body was not valid
JSON, or `payment_id` was absent. This means the secret is correct and the
provider sent something unexpected. Look for a version change on their side
before assuming our parsing is wrong.

### Mitigate

```bash
# If the secret is wrong, rotate it. The provider retries, so no settlement is
# lost while the old and new secrets overlap.
kubectl patch secret payment-secrets -n platform \
  --type merge -p "{\"stringData\":{\"nowpayments-ipn-secret\":\"<new>\"}}"
kubectl rollout restart deployment/payment -n platform
```

Verify the signature locally before pushing a secret, to avoid a second blind
rotation:

```bash
# HMAC-SHA512 over the raw payload, hex encoded.
printf '%s' "$RAW_PAYLOAD" | openssl dgst -sha512 -hmac "$IPN_SECRET" | sed 's/^.* //'
```

Must match the `x-nowpayments-sig` header byte for byte.

### Verify

```bash
# Rejections stop.
# sum(rate(payment_errors_total{error_type="webhook_invalid"}[5m])) -> 0

# Settlement resumes: pending back to normal.
watch -n10 'curl -s "http://victoriametrics.platform:8428/api/v1/query?query=sum(increase(payment_deposits_total{status=\"pending\"}[10m]))"'
```

Deposits rejected during the window were never charged — the signature check
runs before any state change, so no balance moved and nothing needs crediting.
The provider keeps retrying, so recovery is automatic once the secret is right.

### Do not

- Do not bypass signature verification to "unblock settlement". It is the only
  thing preventing a forged callback from crediting an arbitrary balance
  (`TestProcessDepositWebhook_InvalidSignatureDoesNotCredit` covers this).
- Do not mark rejected webhooks as processed. Doing so suppresses the retry and
  strands the deposit permanently.

---

## Escalation

| Situation | Route to |
|---|---|
| Player paid, no balance (PAY-RB-02) | Payments on-call, then Payments Lead |
| Balance locked for a failed payout (PAY-RB-03 A/B) | Payments on-call |
| Crypto sent, debit not finalized (PAY-RB-03 C) | Payments Lead — reconciliation, do not self-serve |
| Double credit or double debit detected | Payments Lead + Finance, immediately |
| Webhook secret suspected compromised | Security Lead before rotating |

Payments owns the runbook. Add a scenario here when a new alert in
`payment-alerting-rules.yaml` has no matching entry.
