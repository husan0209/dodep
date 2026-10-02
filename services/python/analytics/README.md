# Analytics Service

Affiliate BI and reporting API over ClickHouse (Этап 13 «Affiliate System +
Analytics Dashboard», Data Engineer profile).

Answers executive/sports dashboards from the materialized ClickHouse tables
in `libs/migrations/clickhouse/003_affiliate_analytics.sql`.

```
main.py                     container entrypoint shim (uvicorn main:app)
src/analytics/config.py     settings (env-driven)
src/analytics/clickhouse.py lazy client, dict rows, outage -> typed error
src/analytics/affiliate_reports.py  query builders, validation, row mapping
src/analytics/metrics.py    Prometheus request counters/latency
src/analytics/main.py       FastAPI app, endpoints, probes
tests/                      88 tests, no live ClickHouse required
```

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| GET | `/health` | liveness (does not need ClickHouse) |
| GET | `/ready` | readiness — 503 while ClickHouse is unreachable |
| GET | `/metrics` | Prometheus exposition |
| GET | `/api/v1/analytics/affiliate/summary` | daily funnel + revenue series |
| GET | `/api/v1/analytics/affiliate/funnel` | click → registration → FTD → active |
| GET | `/api/v1/analytics/affiliate/top` | top affiliates by NGR (`limit`) |
| GET | `/api/v1/analytics/affiliate/ngr-trend` | daily GGR/NGR trend |
| GET | `/api/v1/analytics/affiliate/payouts/aging` | open payouts + waiting hours |
| GET | `/api/v1/analytics/affiliate/ltv` | LTV per referred player |
| GET | `/api/v1/analytics/affiliate/earnings` | paginated earnings (`status`, `limit`, `offset`) |
| GET | `/api/v1/analytics/affiliate/totals` | platform-wide totals |
| GET | `/api/v1/analytics/export/{table}` | CSV export (whitelisted tables) |

All report endpoints accept `date_from` / `date_to` (ISO). Omitted → last 30
days.

## Rules the code enforces

- **Money is never a float.** ClickHouse `Decimal(18,8)` values are rendered as
  decimal **strings** (`"120.50"`), and ratios go through `safe_div()` on
  `Decimal`. CONVENTIONS: NEVER-6.
- **No value interpolation.** Every user-supplied value is bound as a query
  parameter. Identifiers cannot be bound, so export tables come from
  `EXPORTABLE_TABLES` and statuses from `EARNING_STATUSES` — both validated,
  both covered by tests (including injection attempts).
- **Bounded work.** Date range ≤ `max_range_days` (366), `limit` clamped to
  `max_limit` (500), `offset` clamped — a single request cannot ask
  ClickHouse for an unbounded aggregate.
- **Storage outages are explicit.** A ClickHouse failure returns 503, never a
  zero-filled report that looks like "no data".

## Data sources

| Table | Used for |
|---|---|
| `affiliate_daily_aggregates` | summary, funnel, top, NGR trend, totals |
| `affiliate_earnings` | earnings report (status filter, pagination) |
| `affiliate_payouts` | payout aging |
| `referred_player_activity` | LTV |
| `affiliate_attributions` | export only |

## Running locally

```bash
pip install -r requirements.txt
uvicorn main:app --port 8000      # from the service directory
# or: python -m analytics          # equivalent entrypoint
```

Configuration is env-driven: `CLICKHOUSE_HOST`, `CLICKHOUSE_PORT` (default
8123 — the HTTP port), `CLICKHOUSE_DATABASE`, `CLICKHOUSE_USER`,
`CLICKHOUSE_PASSWORD`, `DEFAULT_RANGE_DAYS`, `MAX_LIMIT`.

> Note: `clickhouse-connect` speaks the **HTTP** interface, so the default
> port is 8123, not the native protocol port 9000.

## Development

```bash
pytest -q          # 88 tests, no ClickHouse needed
ruff check .
black --check .
mypy .
bandit -c pyproject.toml -r .    # 0 findings outside tests
```

CI installs this service through two paths (`pip install -r requirements.txt`
and `pip install -e ".[dev]"`), so `requirements.txt` and `pyproject.toml` must
stay identical — `tests/test_dependencies.py` fails the build otherwise.

## Known limitations

- Aggregates are read as-is from the materialized views; there is no caching
  layer in front of ClickHouse, so dashboard traffic is bounded only by the
  range/limit clamps.
- No auth on the endpoints: they are meant to sit behind the existing API
  gateway / network policy. Add service-to-service auth before exposing them.
- Cohort retention and CAC/LTV curves from the ТЗ executive dashboard are not
  implemented yet — LTV per player is there, but no cohort matrix.