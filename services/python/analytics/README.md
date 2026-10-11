# Analytics Service

Platform-wide read API over ClickHouse analytics tables. Owned tables: none —
this service only **reads** tables written by product services (affiliate,
betting, payments). It never writes to product tables.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health`, `/ready` | Liveness / readiness |
| GET | `/api/v1/analytics/affiliate/{id}/summary?date_from&date_to` | Per-day clicks/registrations/FTD/GGR/NGR/commission |
| GET | `/api/v1/analytics/affiliate/{id}/funnel?days=7` | Conversion funnel + rates |
| GET | `/api/v1/analytics/affiliate/{id}/ngr-trend?days=30` | Daily NGR/commission trend |
| GET | `/api/v1/analytics/affiliate/{id}/ltv?limit=100` | LTV ranking of referred players |
| GET | `/api/v1/analytics/reports?date_from&date_to&status?&affiliate_id?&limit&offset` | Paginated earnings report (`has_more` without COUNT) |
| GET | `/api/v1/analytics/dashboard?date_from&date_to` | Platform totals + funnel + top-5 affiliates |
| GET | `/api/v1/analytics/affiliates/top?days=30&limit=100` | Top affiliates by NGR |
| GET | `/api/v1/analytics/payouts/aging?limit=100` | Open payouts, oldest first |
| GET | `/api/v1/analytics/export?table&date_from&date_to` | CSV export (allowlisted tables only) |

Conventions:

- Money is formatted as 8-decimal strings via `Decimal` — never floats.
- All ClickHouse queries are parameterized (`%(name)s`); values are never
  interpolated into SQL text.
- Export is restricted to `EXPORTABLE_TABLES` in `affiliate_reports.py`.
- ClickHouse unreachable → HTTP 503. No fake data, no silent empty results
  on the summary endpoints (empty range returns `{"data": []}`).
- Date ranges: `YYYY-MM-DD`, `from <= to`, max 366 days, no future `to`.

## Tables consumed (read-only, owned by Affiliate Service)

`affiliate_daily_aggregates`, `affiliate_earnings`, `affiliate_payouts`,
`referred_player_activity` — see
`services/go/affiliate/clickhouse/affiliate_analytics.sql`.

## Docker

The generic `infra/docker/Dockerfile.python` works for this service:

- `services/python/analytics/requirements.txt` exists (runtime-minimal pins).
- Root `main.py` re-exports the app for `uvicorn main:app` (`SERVICE_NAME=analytics`).

## Helm

Chart: `infra/helm/charts/analytics/` (mirrors `fraud-ml`, minus ML specifics).

- Internal-only `ClusterIP` service on port 8000, no public ingress by default.
- ConfigMap: non-secret env (`APP_ENV`, ClickHouse host/port, Redis host).
- Secret: ClickHouse username/password (override via
  `--set analytics.clickhouse.password=...`, never commit real credentials).
- NetworkPolicy: ingress from `istio-system`/`monitoring`, egress to `data`
  namespace (ClickHouse 8123/9000, Redis 6379).
- Production overlays in `values-production.yaml` (3 replicas, PDB minAvailable 2).

## Observability

- `GET /metrics` — Prometheus exposition (scraped via ServiceMonitor on the
  `http` service port): `analytics_http_requests_total`,
  `analytics_http_request_duration_seconds`, `analytics_http_requests_in_progress`.
- Implemented in `src/analytics/metrics.py` on stdlib only (no extra
  dependency). Route labels are cardinality-guarded: UUIDs, numeric IDs and
  ISO dates collapse to `:param`.
- Structured logging: stdlib `logging` (JSON wiring is the deploy concern).

## Load / smoke test

k6 scenario: `tools/testing/k6/scenarios/analytics.js`.

- Health probes expect 200; reporting endpoints accept 200 **or** 503
  (graceful degradation without ClickHouse is by design, not a failure).
- Validation probes: reversed date range → 422, `export?table=users` → 422.
- Run: `k6 run -e BASE_URL=http://localhost:8000 tools/testing/k6/scenarios/analytics.js`

## Tests

Stdlib only, no live services required:

```
py -m unittest discover -s tests -v
```
