# Bonus Service — API Documentation

> **Source of truth:** `libs/proto/bonus/v1/bonus.proto` (gRPC contract, buf v2).
> This document maps the proto contract to the HTTP layer.
> Backend implementation: `services/go/bonus/` (Fiber + gRPC, GORM).

## Base URLs

```
HTTP: bonus.platform:8088
gRPC: bonus.platform:50056
```

Ports per `CONVENTIONS.md` (bonus: 8088 HTTP / 50056 gRPC).

---

## Authentication

- All `/api/v1/bonuses/*` endpoints require `Authorization: Bearer <access_token>`
  (JWT, issued by auth-service).
- `user_id` is **always taken from the JWT claims, never from the request body**
  (`CONVENTIONS.md` NEVER-7).
- Mutating endpoints (`POST .../activate`, `POST .../cancel`) additionally require
  `X-Idempotency-Key: <uuid>` header.

---

## Bonus Lifecycle

```
pending → active → completed
pending → active → cancelled
pending → cancelled
active → expired (TTL exceeded before wagering complete)
```

- Welcome bonus: 100% match on first deposit, capped at `WELCOME_BONUS_MAX_AMOUNT`
  (default 200 USD), wagering 30x, expiry 30 days (all via env, decimal-safe).
- Award is idempotent per user: re-award returns the existing bonus, no duplicate row.
- Wagering completion credits the bonus amount to the user's **MAIN** (real-money)
  wallet via wallet-core `Credit` with `reference_type="bonus"` and a stable
  idempotency key `bonus-wagering-<bonus_id>` — retries never double-credit.
- Money values are decimal strings (`"100.00"`), never float (`CONVENTIONS.md` NEVER-6).

---

## gRPC Service `bonus.v1.BonusService`

Proto package: `bonus.v1` · Go package: `github.com/opus-casino/proto/gen/go/bonus/v1`

| RPC | Request | Response | Notes |
|-----|---------|----------|-------|
| `GetAvailableBonuses` | `GetAvailableBonusesRequest{user_id, bonus_type?}` | `GetAvailableBonusesResponse{bonuses}` | Welcome offer template while user has no active bonus |
| `ActivateBonus` | `ActivateBonusRequest{user_id, bonus_id, promo_code?}` | `ActivateBonusResponse{bonus, error}` | `pending` → `active` |
| `GetActiveBonuses` | `GetActiveBonusesRequest{user_id, bonus_type?}` | `GetActiveBonusesResponse{bonuses}` | 0 or 1 in current model |
| `GetBonus` | `GetBonusRequest{user_id, bonus_id}` | `GetBonusResponse{bonus}` | Owner only |
| `GetWageringProgress` | `GetWageringProgressRequest{user_id, bonus_id}` | `GetWageringProgressResponse{progress}` | Required/wagered/percentage/completed |
| `ClaimFreeSpins` | `ClaimFreeSpinsRequest{...}` | `ClaimFreeSpinsResponse{..., error}` | Not supported in this release (returns error) |
| `GetPromotions` | `GetPromotionsRequest{pagination}` | `GetPromotionsResponse{}` | No promotion engine in MVP (empty) |
| `GetPromotion` | `GetPromotionRequest{promotion_id}` | — | Not found (no promotion engine in MVP) |

---

## REST Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health`, `/ready` | Liveness / readiness (ready checks DB) |
| `GET` | `/metrics` | Prometheus exposition (see [Metrics](#metrics)) |
| `GET` | `/api/v1/bonuses/?limit=&offset=` | List own bonuses, newest first |
| `GET` | `/api/v1/bonuses/active` | Current active bonus or `{"bonus": null}` |
| `GET` | `/api/v1/bonuses/:id` | Bonus detail (owner only, else 404) |
| `GET` | `/api/v1/bonuses/:id/wagering` | `{required, completed, remaining, progress_percentage, is_completed}` |
| `POST` | `/api/v1/bonuses/:id/activate` | Activate pending bonus (idempotency key required) |
| `POST` | `/api/v1/bonuses/:id/cancel` | Cancel pending/active bonus (idempotency key required) |

Error mapping: `404` bonus not found (incl. other user's bonus),
`409` bonus not active / already exists, `400` invalid amount or missing idempotency key,
`401` missing/invalid token.

---

## Events (Redpanda)

Consumes `payments.completed` (consumer group `bonus-service`):
first-deposit events trigger `AwardWelcomeBonus`. Malformed messages are
skipped without redelivery; service errors return for redelivery.

---

## Metrics

Prometheus exposition is served on the service HTTP port at `GET /metrics`
(no auth; the endpoint is restricted by the Helm NetworkPolicy to the
monitoring namespace and carries aggregate counters only — no PII).

RED (traffic) metrics — `route` is always the registered route pattern, never a
raw path, so path parameters cannot inflate cardinality:

| Metric | Labels | Meaning |
|--------|--------|---------|
| `bonus_http_requests_total` | `method`, `route`, `status` | Served HTTP requests |
| `bonus_http_request_duration_seconds` | `method`, `route` | HTTP latency histogram |
| `bonus_grpc_requests_total` | `method`, `code` | Served gRPC calls |
| `bonus_grpc_request_duration_seconds` | `method` | gRPC latency histogram |

Business metrics for the money-critical flow:

| Metric | Labels | Meaning |
|--------|--------|---------|
| `bonus_bonuses_awarded_total` | `type`, `currency` | Welcome bonuses created |
| `bonus_bonuses_award_skipped_total` | `reason` | Non-creating awards (`already_awarded`, `invalid_amount`) |
| `bonus_wagers_recorded_total` | `result` | Wager outcomes (`recorded`, `no_active_bonus`, `expired`, `credit_failed`, `invalid_amount`, `repository_failure`, `completed`) |
| `bonus_wagering_completed_total` | `type`, `currency` | Bonuses whose wagering was met (counted **after** a successful credit) |
| `bonus_wagering_progress_ratio` | `type` | Distribution of wagered/required at completion |
| `bonus_conversion_credits_total` | `result` | Wallet credits for conversions (`credited`, `skipped_no_wallet`, `failed`) |
| `bonus_payment_events_total` | `result` | Consumed `payments.completed` events (`awarded`, `skipped_not_first`, `malformed`, `failed`) |

Alerting notes:

- `bonus_conversion_credits_total{result="failed"}` increasing without retries
  succeeding is a P1 money-loss signal — a player has completed wagering but
  was not credited.
- `bonus_conversion_credits_total{result="skipped_no_wallet"} > 0` in a
  non-development environment means `WALLET_GRPC_ADDR` / client wiring is broken
  (service currently logs the skip and does not credit).
- `bonus_payment_events_total{result="malformed"}` above 0 indicates a
  payment-service schema change (JSON contract drift).
- `bonus_wagers_recorded_total{result="repository_failure"}` points at database
  trouble, not at players.

---

## Tracing

OpenTelemetry tracing is **opt-in**: without `OTEL_EXPORTER_OTLP_ENDPOINT` the
service installs a no-op tracer provider (zero overhead, no collector needed for
local runs or tests).

| Variable | Default | Description |
|----------|---------|-------------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | _(unset → tracing off)_ | OTLP/gRPC collector, e.g. `otel-collector.monitoring:4317` |
| `OTEL_SERVICE_NAME` | `bonus-service` | `service.name` on every span |
| `OTEL_TRACES_SAMPLER` | `parentbased_always_on` | `always_on` / `always_off` / `traceidratio` |
| `OTEL_TRACES_SAMPLER_ARG` | `1.0` | Ratio for `traceidratio`; invalid values fall back to `1.0` |
| `OTEL_EXPORTER_OTLP_INSECURE` | `true` | Plaintext collector transport (mesh-internal) |

Span layout:

| Span | Kind | Created by | Key attributes |
|------|------|-----------|----------------|
| `GET /api/v1/bonuses/:id` | SERVER | `TracingMiddleware` | `http.request.method`, `http.route`, `url.path`, `http.response.status_code` |
| `GetBonus` | SERVER | `UnaryTracingInterceptor` | `rpc.system`, `rpc.service`, `rpc.grpc.status_code` |
| `wallet.v1.WalletCoreService/Credit` | CLIENT | `client.CreditBonusConversion` | `peer.service`, `wallet.amount`, `wallet.currency` |
| `payments.completed process` | CONSUMER | `PaymentConsumer.processRecord` | `messaging.destination.name`, `messaging.destination.partition`, `messaging.kafka.offset`, `event.*`, `bonus.*` |

Notes:

- W3C `traceparent` from the edge (Istio) is extracted, never restarted: a
  conversion credit shows up under the same trace as the HTTP request that
  triggered it.
- Span names are set to `<METHOD> <route pattern>` after routing resolves, so
  per-ID URLs aggregate into one span — same cardinality rule as the metrics.
- Spans are batched and flushed on shutdown with a 5s budget; a collector
  outage never blocks termination.

Every request also carries `X-Request-ID` (incoming value kept, otherwise
generated, echoed in the response, capped at 128 chars) so logs can be
correlated by `request_id` and `trace_id`.

---

## Rate limits & hardening

| Protection | Value | Notes |
|------------|-------|-------|
| Read budget | 300 req/min per user (or IP) | Partner API group |
| Write budget | 30 req/min per user (or IP) | `activate`, `cancel` |
| Body size | 1 MiB | Rejected before parsing (`413`/`body size exceeds`) |
| Map bound | 50 000 keys | Evicts oldest quarter; limiter fails **open** |
| Headers | `X-RateLimit-Limit/Remaining/Reset`, `Retry-After` | Same contract as admin-bff |

Responses on throttling: `429` with
`{"error":"RATE_LIMIT_EXCEEDED","message":...,"limit":N,"retry_after_seconds":S}`.

Security headers on every response (including errors):
`X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
`Referrer-Policy: no-referrer`, `Permissions-Policy: camera=(), microphone=(), geolocation=()`,
`Strict-Transport-Security: max-age=63072000; includeSubDomains`,
`X-DNS-Prefetch-Control: off`.

Limiter state is **per pod**: it is a first line of defence against abuse and
client loops, not a global quota — global limits are enforced at the edge
(CloudFlare) and in the mesh (Istio). Swapping the store to DragonflyDB only
requires replacing the map in `internal/ratelimit`.

Alert on sustained throttling with:

```promql
sum by (route) (rate(bonus_http_requests_total{status="429"}[5m])) > 0
```

---

## Configuration (env)

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8088` | HTTP port |
| `GRPC_PORT` | `50056` | gRPC port |
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/opus_casino?sslmode=disable` | PostgreSQL DSN |
| `KAFKA_BROKERS` | `localhost:9092` | Redpanda brokers (CSV) |
| `WALLET_GRPC_ADDR` | `wallet-core:50053` | wallet-core gRPC address |
| `JWT_SECRET_KEY` | `change-me-in-production` | JWT HMAC secret (must override) |
| `WELCOME_BONUS_PERCENTAGE` | `100` | Match percent |
| `WELCOME_BONUS_MAX_AMOUNT` | `200` | Cap, decimal string |
| `WELCOME_BONUS_WAGERING_REQUIREMENT` | `30` | Wagering multiplier |
| `WELCOME_BONUS_EXPIRY_DAYS` | `30` | TTL, days |
| `CORS_ORIGINS` | `http://localhost:3000` | CSV allowlist (`*` is stripped) |
