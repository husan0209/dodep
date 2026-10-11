# Bonus Service

Welcome-bonus lifecycle for Opus Casino: award on first deposit, track wagering,
convert to real money on completion. Go + Fiber + GORM, gRPC contract in
`libs/proto/bonus/v1/bonus.proto`. Full endpoint reference: `docs/api/bonus.md`.

## What it does

- Awards a 100% welcome match on first deposit (capped, 30x wagering, 30d TTL —
  all via env, decimal-safe, never float).
- Award is idempotent per user (re-award returns the existing row).
- Records wagering from `payments.completed` events (Redpanda, group `bonus-service`).
- On wagering completion credits the bonus amount to the user's MAIN wallet via
  wallet-core `Credit` (`reference_type="bonus"`, stable idempotency key
  `bonus-wagering-<bonus_id>` — retries never double-credit).
- Activate/cancel flows for pending bonuses; expiry sweep on read.

## Layout

```
main.go                    Bootstrap: config → GORM → repo → service → consumer + gRPC + HTTP
middleware.go              JWT auth (user_id from token only) + X-Idempotency-Key enforcement
routes.go                  Thin Fiber handlers (no business logic)
internal/config/           Env-based config (ports 8088/50056 per CONVENTIONS)
internal/domain/           Bonus entity, status machine, sentinel errors (no external deps)
internal/service/          Business logic (depends on repository interface only)
internal/repository/       GORM persistence (fail-fast on nil DB)
internal/handlers/         gRPC BonusService (proto ↔ domain at the boundary)
internal/client/           wallet-core gRPC Credit client
internal/consumer/         Redpanda payments.completed consumer
internal/telemetry/        Prometheus collectors, OTel tracing, HTTP/gRPC middleware
internal/ratelimit/        Fixed-window HTTP rate limiter (per pod, fails open)
internal/testutil/         In-memory repository fake (tests only, never imported by prod code)
tests/integration/         testcontainers (postgres:16-alpine) flow tests
```

## Run locally

```bash
cd services/go/bonus
go build ./...
go test -short ./...      # unit tests (no infra)
go test ./tests/...       # integration tests (needs Docker: spins postgres:16-alpine)
```

Required env for a live run: `DATABASE_URL`, `KAFKA_BROKERS`, `WALLET_GRPC_ADDR`
(default `wallet-core:50053`), `JWT_SECRET_KEY` (must override),
`CORS_ORIGINS` (CSV allowlist, `*` is stripped).

## CI / CD

Pipeline: `.github/workflows/ci-go-bonus.yml` — lint (`gofmt`, `go vet`,
pinned golangci-lint) → tests (`-race`, unit + testcontainers integration,
Codecov) → `govulncheck` → image build (generic `infra/docker/Dockerfile.go`;
`go mod vendor` runs first because go.mod replaces `libs/proto` outside the
docker context) → Trivy (blocking on CRITICAL/HIGH) → `helm lint` + template
render asserts → deploy to dev with `/health` + `/ready` smoke on `:8088`.

## Helm

Chart: `infra/helm/charts/bonus/` (depends on local `service-chart`).
Key values follow `service-chart` keys: `service.port/targetPort` (8088),
`service.grpcPort/grpcTargetPort` (50056, opt-in stanza in the base chart),
`healthCheck.port` (8088), `env` (see Configuration in `docs/api/bonus.md`).
`serviceMonitor` is enabled: the app serves `/metrics` on the HTTP port and
`service.extraPorts` exposes it under the `metrics` port name so the
ServiceMonitor selector matches. Metrics inventory and alerting hints:
`docs/api/bonus.md#metrics`.

Secrets (`bonus-secrets` / `bonus-secrets-production`: `database-url`,
`jwt-secret-key`) are provisioned outside this chart — never put secret
values in values files.

## Performance testing

`tools/testing/k6/scenarios/bonus-service.js` covers the partner cabinet:

```bash
k6 run tools/testing/k6/scenarios/bonus-service.js
# with auth + state-changing flows (dedicated test user only):
k6 run -e BASE_URL=http://bonus:8088 \
       -e AUTH_BASE_URL=http://auth:8083 \
       -e LOGIN_IDENTIFIER=loadtest@example.com -e LOGIN_PASSWORD=... \
       -e TEST_BONUS_ID=<uuid> -e ALLOW_WRITES=true \
       tools/testing/k6/scenarios/bonus-service.js
```

The scenario asserts the auth gate (401 without a token), the idempotency-key
requirement on writes, `X-RateLimit-*` headers on reads, and — with
`STRESS_RATE_LIMIT=true` — that the limiter answers 429 with `Retry-After`.
Thresholds: partner API p95 < 500 ms, unexpected errors < 1%.

CI: the `k6-smoke` job in `.github/workflows/ci-go-bonus.yml` runs inside the
`grafana/k6` container and only executes when `BONUS_K6_BASE_URL` is set for
the branch, so PRs from forks are not blocked by a missing environment.

## Key invariants

- `user_id` always comes from JWT claims, never from the body (NEVER-7).
- All money math in `decimal.Decimal` (NEVER-6); proto amounts are decimal strings.
- Credit happens **before** marking `completed`, guarded by the wallet
  idempotency key — a crash between the two is retried safely, money is never lost.
- Bonus completion without a configured wallet client is logged at error level
  (dev mode), counted as `skipped_no_wallet`, and never silent.
- `/metrics` is unauthenticated by design: aggregate counters only, reachable
  only from the monitoring namespace (NetworkPolicy).
- Tracing is opt-in via `OTEL_EXPORTER_OTLP_ENDPOINT`; spans are extracted from
  the edge W3C context and never restarted. `X-Request-ID` is generated,
  propagated and echoed. Span/metric inventory: `docs/api/bonus.md`.
- Shutdown order is explicit: gRPC health → NOT_SERVING, stop the consumer →
  drain HTTP → graceful gRPC stop → release the wallet-core connection → flush
  spans.
- HTTP hardening: 1 MiB body cap, security headers on every response
  (nosniff, DENY framing, no-referrer, restrictive `Permissions-Policy`, HSTS),
  and per-user rate limits — 300 reads/min, 30 writes/min — answered with
  `429` + `Retry-After` + `X-RateLimit-Limit/Remaining/Reset`. The limiter is
  per pod on purpose (global quotas are enforced at the edge/mesh) and fails
  open, bounded at 50k keys, so it can never become an outage.
