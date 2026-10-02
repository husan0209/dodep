"""Analytics Service - affiliate BI and reporting API.

Endpoints answer from ClickHouse aggregates. If ClickHouse is unreachable
the API returns 503 (and `/ready` reports not-ready) instead of pretending
the numbers are zero.
"""

from __future__ import annotations

import csv
import io
import logging
from collections.abc import Iterator
from contextlib import asynccontextmanager
from decimal import Decimal
from typing import Any

import structlog
from fastapi import APIRouter, FastAPI, Query, Request
from fastapi.responses import JSONResponse, StreamingResponse
from pydantic import BaseModel

from .affiliate_reports import (
    ValidationError,
    build_daily_summary_query,
    build_earnings_report_query,
    build_export_query,
    build_funnel_query,
    build_ltv_query,
    build_ngr_trend_query,
    build_payout_aging_query,
    build_platform_totals_query,
    build_top_affiliates_query,
    clamp_limit,
    clamp_offset,
    decimal_str,
    default_range,
    map_earning_row,
    map_funnel_row,
    map_ltv_row,
    map_payout_aging_row,
    map_summary_row,
    map_top_affiliates_row,
    map_totals_row,
    map_trend_row,
    validate_date_range,
    validate_export_table,
    validate_status,
)
from .clickhouse import ClickHouseUnavailableError, get_client, reset_client
from .config import settings
from .metrics import MetricsMiddleware, render_metrics

structlog.configure(
    processors=[
        structlog.processors.TimeStamper(fmt="iso"),
        structlog.processors.add_log_level,
        structlog.processors.dict_tracebacks,
        structlog.processors.JSONRenderer(),
    ],
    wrapper_class=structlog.make_filtering_bound_logger(
        logging.getLevelNamesMapping().get(settings.log_level.upper(), logging.INFO)
    ),
    context_class=dict,
    logger_factory=structlog.PrintLoggerFactory(),
    cache_logger_on_first_use=True,
)
logger = structlog.get_logger()

router = APIRouter(prefix="/api/v1/analytics")


# ----------------------------------------------------------------------
# Response models
# ----------------------------------------------------------------------
class DailySummary(BaseModel):
    date: str | None
    clicks: int
    registrations: int
    ftd_count: int
    active_players: int
    ggr: str
    ngr: str
    commission_accrued: str
    commission_released: str


class Funnel(BaseModel):
    clicks: int
    registrations: int
    ftd_count: int
    active_players: int
    ggr: str
    ngr: str
    click_to_registration: str
    registration_to_ftd: str
    ftd_to_active: str


class TopAffiliate(BaseModel):
    affiliate_id: str
    ngr: str
    ggr: str
    commission: str
    ftd_count: int
    registrations: int
    clicks: int
    ftd_rate: str


class NgrTrendPoint(BaseModel):
    date: str | None
    ggr: str
    ngr: str
    commission: str


class PayoutAgingItem(BaseModel):
    payout_id: str
    affiliate_id: str
    amount: str
    currency: str
    status: str
    requested_at: str | None
    wait_hours: int


class LtvItem(BaseModel):
    referred_user_id: int
    lifetime_ggr: str
    lifetime_deposits: str
    lifetime_bonuses: str
    active_days: int
    ggr_per_bonus: str


class Earning(BaseModel):
    earning_id: str
    affiliate_id: str
    referred_user_id: int
    source_type: str
    period_start: str | None
    period_end: str | None
    ggr: str
    ngr: str
    commission_rate: str
    commission: str
    status: str


class EarningsReport(BaseModel):
    items: list[Earning]
    total: int
    limit: int
    offset: int
    ngr_by_status: dict[str, str]


class PlatformTotals(BaseModel):
    clicks: int
    registrations: int
    ftd_count: int
    active_players: int
    ggr: str
    ngr: str
    commission_accrued: str
    commission_released: str
    ftd_rate: str
    click_to_registration: str


# ----------------------------------------------------------------------
# Lifespan
# ----------------------------------------------------------------------
@asynccontextmanager
async def lifespan(app: FastAPI):
    """Warm the ClickHouse client and always release it on shutdown."""
    logger.info("analytics.startup", version=settings.version)
    client = get_client(settings)
    app.state.clickhouse_ready = client.ping()
    if not app.state.clickhouse_ready:
        # Not fatal: the API stays up and answers 503 until CH is reachable.
        logger.warning("analytics.clickhouse_unavailable_at_startup")
    try:
        yield
    finally:
        logger.info("analytics.shutdown")
        reset_client()
        app.state.clickhouse_ready = False


app = FastAPI(
    title=settings.app_name,
    description="Affiliate analytics and BI reporting for Opus Casino",
    version=settings.version,
    lifespan=lifespan,
)
app.add_middleware(MetricsMiddleware)


# ----------------------------------------------------------------------
# Dependencies / error handling
# ----------------------------------------------------------------------
@app.exception_handler(ClickHouseUnavailableError)
async def clickhouse_unavailable_handler(
    _request: Request, exc: ClickHouseUnavailableError
) -> JSONResponse:
    """Storage outage -> 503 (never a fabricated zero-filled report)."""
    logger.error("analytics.clickhouse_unavailable", error=str(exc))
    return JSONResponse(status_code=503, content={"detail": "analytics storage unavailable"})


@app.exception_handler(ValidationError)
async def validation_error_handler(_request: Request, exc: ValidationError) -> JSONResponse:
    return JSONResponse(status_code=422, content={"detail": str(exc)})


def _run(query: str, params: dict[str, Any]) -> list[dict[str, Any]]:
    """Execute a query; storage failures propagate to the 503 handler."""
    client = get_client(settings)
    return client.fetch_all(query, params)


def _range(date_from: str | None, date_to: str | None) -> tuple[str, str]:
    """Resolve an explicit or default date range as ISO strings."""
    if date_from and date_to:
        start, end = validate_date_range(date_from, date_to, max_days=settings.max_range_days)
    else:
        start, end = default_range(settings.default_range_days)
    return start.isoformat(), end.isoformat()


# ----------------------------------------------------------------------
# Reports
# ----------------------------------------------------------------------
@router.get("/affiliate/summary", response_model=list[DailySummary])
async def affiliate_summary(
    date_from: str | None = Query(default=None, description="ISO date"),
    date_to: str | None = Query(default=None, description="ISO date"),
) -> list[DailySummary]:
    """Daily funnel and revenue series."""
    start, end = _range(date_from, date_to)
    rows = _run(build_daily_summary_query(), {"date_from": start, "date_to": end})
    return [DailySummary(**map_summary_row(row)) for row in rows]


@router.get("/affiliate/funnel", response_model=Funnel)
async def affiliate_funnel(
    date_from: str | None = Query(default=None),
    date_to: str | None = Query(default=None),
) -> Funnel:
    """Click → registration → FTD → active conversion funnel."""
    start, end = _range(date_from, date_to)
    rows = _run(build_funnel_query(), {"date_from": start, "date_to": end})
    return Funnel(**map_funnel_row(rows[0] if rows else {}))


@router.get("/affiliate/top", response_model=list[TopAffiliate])
async def affiliate_top(
    date_from: str | None = Query(default=None),
    date_to: str | None = Query(default=None),
    limit: int = Query(default=settings.default_limit),
) -> list[TopAffiliate]:
    """Top affiliates by NGR."""
    start, end = _range(date_from, date_to)
    rows = _run(
        build_top_affiliates_query(),
        {"date_from": start, "date_to": end, "limit": clamp_limit(limit, settings.max_limit)},
    )
    return [TopAffiliate(**map_top_affiliates_row(row)) for row in rows]


@router.get("/affiliate/ngr-trend", response_model=list[NgrTrendPoint])
async def affiliate_ngr_trend(
    date_from: str | None = Query(default=None),
    date_to: str | None = Query(default=None),
) -> list[NgrTrendPoint]:
    """Daily GGR/NGR trend for the executive dashboard."""
    start, end = _range(date_from, date_to)
    rows = _run(build_ngr_trend_query(), {"date_from": start, "date_to": end})
    return [NgrTrendPoint(**map_trend_row(row)) for row in rows]


@router.get("/affiliate/payouts/aging", response_model=list[PayoutAgingItem])
async def affiliate_payout_aging(
    date_from: str | None = Query(default=None),
    date_to: str | None = Query(default=None),
    limit: int = Query(default=settings.default_limit),
) -> list[PayoutAgingItem]:
    """Open payouts with waiting time."""
    start, end = _range(date_from, date_to)
    rows = _run(
        build_payout_aging_query(),
        {"date_from": start, "date_to": end, "limit": clamp_limit(limit, settings.max_limit)},
    )
    return [PayoutAgingItem(**map_payout_aging_row(row)) for row in rows]


@router.get("/affiliate/ltv", response_model=list[LtvItem])
async def affiliate_ltv(
    date_from: str | None = Query(default=None),
    date_to: str | None = Query(default=None),
    limit: int = Query(default=settings.default_limit),
) -> list[LtvItem]:
    """Lifetime value per referred player."""
    start, end = _range(date_from, date_to)
    rows = _run(
        build_ltv_query(),
        {"date_from": start, "date_to": end, "limit": clamp_limit(limit, settings.max_limit)},
    )
    return [LtvItem(**map_ltv_row(row)) for row in rows]


@router.get("/affiliate/earnings", response_model=EarningsReport)
async def affiliate_earnings(
    status: str | None = Query(default=None),
    date_from: str | None = Query(default=None),
    date_to: str | None = Query(default=None),
    limit: int = Query(default=settings.default_limit),
    offset: int = Query(default=0),
) -> EarningsReport:
    """Paginated earnings report with optional status filter."""
    start, end = _range(date_from, date_to)
    safe_status = validate_status(status)
    safe_limit = clamp_limit(limit, settings.max_limit)
    safe_offset = clamp_offset(offset)

    predicate = ""
    params: dict[str, Any] = {
        "date_from": start,
        "date_to": end,
        "limit": safe_limit,
        "offset": safe_offset,
    }
    if safe_status:
        predicate = "AND status = %(status)s"
        params["status"] = safe_status

    rows = _run(build_earnings_report_query().format(status_predicate=predicate), params)

    by_status: dict[str, Decimal] = {}
    for row in rows:
        key = str(row.get("status") or "unknown")
        by_status[key] = by_status.get(key, Decimal(0)) + Decimal(str(row.get("ngr_amount") or 0))

    return EarningsReport(
        items=[Earning(**map_earning_row(row)) for row in rows],
        total=len(rows),
        limit=safe_limit,
        offset=safe_offset,
        ngr_by_status={k: decimal_str(v) for k, v in sorted(by_status.items())},
    )


@router.get("/affiliate/totals", response_model=PlatformTotals)
async def affiliate_totals(
    date_from: str | None = Query(default=None),
    date_to: str | None = Query(default=None),
) -> PlatformTotals:
    """Platform-wide affiliate totals for the range."""
    start, end = _range(date_from, date_to)
    rows = _run(build_platform_totals_query(), {"date_from": start, "date_to": end})
    return PlatformTotals(**map_totals_row(rows[0] if rows else {}))


# ----------------------------------------------------------------------
# Export
# ----------------------------------------------------------------------
def _csv_stream(rows: list[dict[str, Any]]) -> Iterator[str]:
    """Yield CSV text for rows (decimals stringified, no float coercion)."""
    buffer = io.StringIO()
    if not rows:
        return
    writer = csv.DictWriter(buffer, fieldnames=list(rows[0].keys()), lineterminator="\n")
    writer.writeheader()
    for row in rows:
        writer.writerow({k: ("" if v is None else v) for k, v in row.items()})
        chunk = buffer.getvalue()
        buffer.seek(0)
        buffer.truncate(0)
        yield chunk


@router.get("/export/{table}")
async def export_table(
    table: str,
    date_from: str | None = Query(default=None),
    date_to: str | None = Query(default=None),
    limit: int = Query(default=settings.default_limit),
) -> StreamingResponse:
    """Stream a CSV export for a whitelisted table."""
    column = validate_export_table(table)
    start, end = _range(date_from, date_to)
    rows = _run(
        build_export_query(table, column),
        {
            "date_from": start,
            "date_to": end,
            "limit": clamp_limit(limit, settings.max_limit),
        },
    )
    return StreamingResponse(
        _csv_stream(rows),
        media_type="text/csv",
        headers={"Content-Disposition": f'attachment; filename="{table}.csv"'},
    )


app.include_router(router)


# ----------------------------------------------------------------------
# Operational endpoints
# ----------------------------------------------------------------------
@app.get("/health")
async def health() -> dict[str, str]:
    """Liveness: the process is up (does not require ClickHouse)."""
    return {"status": "ok"}


@app.get("/ready")
async def ready(request: Request) -> JSONResponse:
    """Readiness: true only when ClickHouse answers."""
    client = get_client(settings)
    if client.ping():
        return JSONResponse(status_code=200, content={"status": "ready"})
    return JSONResponse(
        status_code=503, content={"status": "not_ready", "reason": "clickhouse_unavailable"}
    )


@app.get("/metrics")
async def metrics() -> Any:
    """Prometheus metrics."""
    return render_metrics()


def run() -> None:
    """Entrypoint used by `python -m analytics` and the container image."""
    import uvicorn

    uvicorn.run(
        "analytics.main:app",
        host=settings.http_host,
        port=settings.http_port,
        reload=settings.debug,
    )


if __name__ == "__main__":
    run()
