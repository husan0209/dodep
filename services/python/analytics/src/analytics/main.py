"""
Analytics Service - Data analytics and reporting
"""

import csv
import io
import logging
from collections.abc import Iterator
from contextlib import asynccontextmanager
from decimal import Decimal, InvalidOperation
from typing import Any

from fastapi import FastAPI, HTTPException, Query, Request
from fastapi.responses import PlainTextResponse, StreamingResponse
from pydantic_settings import BaseSettings

from analytics import affiliate_reports as rep
from analytics.clickhouse import ClickHouseUnavailableError, get_client
from analytics.metrics import Timer, normalize_route, registry

logger = logging.getLogger(__name__)

# Characters that make a spreadsheet treat a cell as a formula rather than
# text. Affiliate names and campaign labels are attacker-influenced, so an
# unescaped "=cmd|..." would execute when an analyst opens the export.
_FORMULA_TRIGGERS = frozenset("=+-@")


def _csv_cell(value: Any) -> str:
    """Render one CSV field, neutralising spreadsheet formula injection.

    ClickHouse returns Decimal columns as strings, so a negative amount
    arrives as ``"-3.00"``. Escaping that would turn a real figure into
    text and break SUM/AVERAGE in the analyst's spreadsheet, so a value
    that is entirely numeric is left alone even though it starts with
    ``-``. Only non-numeric text gets the ``'`` prefix that forces Excel
    and LibreOffice to treat it as a literal.

    The check looks past leading whitespace and control characters: both
    spreadsheets trim those before evaluating a cell.
    """
    if value is None:
        return ""
    if isinstance(value, str):
        stripped = value.lstrip(" \t\r\n")
        if stripped[:1] in _FORMULA_TRIGGERS:
            try:
                Decimal(stripped)
            except InvalidOperation:
                return f"'{value}"
    return str(value)


def _csv_chunks(rows: list[dict[str, Any]]) -> Iterator[str]:
    """Yield RFC 4180 CSV text for a list of ClickHouse rows."""
    if not rows:
        return
    header = list(rows[0].keys())
    buffer = io.StringIO()
    writer = csv.writer(buffer, lineterminator="\n")
    writer.writerow(header)
    for row in rows:
        writer.writerow([_csv_cell(row.get(column)) for column in header])
    yield buffer.getvalue()


class Settings(BaseSettings):
    """Application settings."""

    app_name: str = "Analytics Service"
    debug: bool = False
    clickhouse_host: str = "localhost"
    clickhouse_port: int = 8123
    clickhouse_username: str = "default"
    clickhouse_password: str = ""
    redis_host: str = "localhost"
    redis_port: int = 6379

    class Config:
        env_file = ".env"


settings = Settings()


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan events."""
    logger.info("Starting Analytics Service...")
    yield
    logger.info("Shutting down Analytics Service...")


app = FastAPI(
    title=settings.app_name,
    description="Analytics service for Opus Casino",
    version="0.1.0",
    lifespan=lifespan,
)


@app.get("/health")
async def health():
    """Health check endpoint."""
    return {"status": "ok"}


@app.get("/ready")
async def ready():
    """Readiness check endpoint."""
    return {"status": "ready"}


@app.middleware("http")
async def metrics_middleware(request: Request, call_next):
    """Record RED metrics for every request (route labels normalized)."""
    registry.track_in_progress(1)
    try:
        with Timer() as timer:
            response = await call_next(request)
        registry.observe_request(
            request.method,
            normalize_route(request.url.path),
            response.status_code,
            timer.elapsed,
        )
        return response
    finally:
        registry.track_in_progress(-1)


@app.get("/metrics")
async def metrics():
    """Prometheus exposition (scraped via ServiceMonitor)."""
    return PlainTextResponse(registry.render(), media_type="text/plain; version=0.0.4")


@app.get("/api/v1/analytics/reports")
async def get_reports(
    date_from: str = Query(..., pattern=r"^\d{4}-\d{2}-\d{2}$"),
    date_to: str = Query(..., pattern=r"^\d{4}-\d{2}-\d{2}$"),
    status: str | None = Query(default=None),
    affiliate_id: str | None = Query(default=None),
    limit: int = 100,
    offset: int = 0,
):
    """Paginated earnings report with optional status/affiliate filters."""
    try:
        start, end = rep.validate_date_range(date_from, date_to)
        query, _ = rep.build_earnings_report_query(status=status, affiliate_id=affiliate_id)
    except ValueError as exc:
        raise HTTPException(status_code=422, detail=str(exc)) from exc
    limit = rep.clamp_limit(limit)
    offset = rep.clamp_offset(offset)
    # Fetch one extra row to compute has_more without an expensive COUNT.
    try:
        rows = _ch().fetch_all(
            query,
            {
                "date_from": str(start),
                "date_to": str(end),
                "status": status,
                "affiliate_id": affiliate_id,
                "limit": limit + 1,
                "offset": offset,
            },
        )
    except ClickHouseUnavailableError as exc:
        raise HTTPException(status_code=503, detail=str(exc)) from exc
    has_more = len(rows) > limit
    return {
        "data": [rep.map_earning_row(r) for r in rows[:limit]],
        "pagination": {"limit": limit, "offset": offset, "has_more": has_more},
    }


@app.get("/api/v1/analytics/dashboard")
async def get_dashboard(
    date_from: str = Query(..., pattern=r"^\d{4}-\d{2}-\d{2}$"),
    date_to: str = Query(..., pattern=r"^\d{4}-\d{2}-\d{2}$"),
):
    """Platform dashboard: totals + top-5 affiliates + funnel for the range."""
    try:
        start, end = rep.validate_date_range(date_from, date_to)
    except ValueError as exc:
        raise HTTPException(status_code=422, detail=str(exc)) from exc
    totals_query, _ = rep.build_platform_totals_query()
    range_top_query, _ = rep.build_top_affiliates_range_query()
    params = {"date_from": str(start), "date_to": str(end)}
    try:
        client = _ch()
        total_rows = client.fetch_all(totals_query, params)
        top_rows = client.fetch_all(
            range_top_query,
            {"date_from": str(start), "date_to": str(end), "limit": 5},
        )
    except ClickHouseUnavailableError as exc:
        raise HTTPException(status_code=503, detail=str(exc)) from exc
    totals = rep.map_totals_row(total_rows[0]) if total_rows else rep.map_totals_row({})
    funnel = rep.map_funnel_row(
        {
            "clicks": totals.get("clicks"),
            "registrations": totals.get("registrations"),
            "ftd_count": totals.get("ftd_count"),
            "active_players": totals.get("active_players"),
        }
    )
    return {
        "data": {
            "range": {"date_from": str(start), "date_to": str(end)},
            "totals": totals,
            "funnel": funnel,
            "top_affiliates": [rep.map_top_affiliates_row(r) for r in top_rows],
        }
    }


@app.post("/api/v1/analytics/export")
async def export_data():
    """Export analytics data."""
    return {"status": "not_implemented"}


def _ch():
    """Resolve the ClickHouse client or raise 503 (never fake data)."""
    try:
        return get_client(
            host=settings.clickhouse_host,
            port=settings.clickhouse_port,
            username=settings.clickhouse_username,
            password=settings.clickhouse_password,
        )
    except ClickHouseUnavailableError as exc:
        raise HTTPException(status_code=503, detail=str(exc)) from exc


@app.get("/api/v1/analytics/affiliate/{affiliate_id}/summary")
async def affiliate_summary(
    affiliate_id: str,
    date_from: str = Query(..., pattern=r"^\d{4}-\d{2}-\d{2}$"),
    date_to: str = Query(..., pattern=r"^\d{4}-\d{2}-\d{2}$"),
):
    """Per-day aggregates for one affiliate in a date range."""
    try:
        start, end = rep.validate_date_range(date_from, date_to)
    except ValueError as exc:
        raise HTTPException(status_code=422, detail=str(exc)) from exc
    query, _ = rep.build_daily_summary_query()
    try:
        rows = _ch().fetch_all(
            query,
            {"affiliate_id": affiliate_id, "date_from": str(start), "date_to": str(end)},
        )
    except ClickHouseUnavailableError as exc:
        raise HTTPException(status_code=503, detail=str(exc)) from exc
    return {"data": [rep.map_summary_row(r) for r in rows]}


@app.get("/api/v1/analytics/affiliate/{affiliate_id}/funnel")
async def affiliate_funnel(affiliate_id: str, days: int = 7):
    """Conversion funnel totals for one affiliate over trailing N days."""
    days = rep.clamp_limit(days, default=7)
    query, _ = rep.build_funnel_query()
    try:
        rows = _ch().fetch_all(query, {"affiliate_id": affiliate_id, "days": days})
    except ClickHouseUnavailableError as exc:
        raise HTTPException(status_code=503, detail=str(exc)) from exc
    if not rows:
        return {"data": rep.map_funnel_row({})}
    return {"data": rep.map_funnel_row(rows[0])}


@app.get("/api/v1/analytics/affiliate/{affiliate_id}/ngr-trend")
async def affiliate_ngr_trend(affiliate_id: str, days: int = 30):
    """Daily NGR/commission trend for one affiliate."""
    days = rep.clamp_limit(days, default=30)
    query, _ = rep.build_ngr_trend_query()
    try:
        rows = _ch().fetch_all(query, {"affiliate_id": affiliate_id, "days": days})
    except ClickHouseUnavailableError as exc:
        raise HTTPException(status_code=503, detail=str(exc)) from exc
    return {
        "data": [
            {
                "report_date": str(r.get("report_date")),
                "ngr_amount": rep.decimal_str(r.get("ngr_amount")),
                "ggr_amount": rep.decimal_str(r.get("ggr_amount")),
                "commission_accrued": rep.decimal_str(r.get("commission_accrued")),
            }
            for r in rows
        ]
    }


@app.get("/api/v1/analytics/affiliate/{affiliate_id}/ltv")
async def affiliate_ltv(affiliate_id: str, limit: int = 100):
    """Lifetime GGR per referred player (LTV ranking)."""
    limit = rep.clamp_limit(limit)
    query, _ = rep.build_ltv_query()
    try:
        rows = _ch().fetch_all(query, {"affiliate_id": affiliate_id, "limit": limit})
    except ClickHouseUnavailableError as exc:
        raise HTTPException(status_code=503, detail=str(exc)) from exc
    return {
        "data": [
            {
                "referred_user_id": r.get("referred_user_id"),
                "lifetime_ggr": rep.decimal_str(r.get("lifetime_ggr")),
                "lifetime_deposits": rep.decimal_str(r.get("lifetime_deposits")),
                "active_days": int(r.get("active_days") or 0),
            }
            for r in rows
        ]
    }


@app.get("/api/v1/analytics/affiliates/top")
async def top_affiliates(days: int = 30, limit: int = 100):
    """Top affiliates by NGR over trailing N days."""
    days = rep.clamp_limit(days, default=30)
    limit = rep.clamp_limit(limit)
    query, _ = rep.build_top_affiliates_query()
    try:
        rows = _ch().fetch_all(query, {"days": days, "limit": limit})
    except ClickHouseUnavailableError as exc:
        raise HTTPException(status_code=503, detail=str(exc)) from exc
    return {"data": [rep.map_top_affiliates_row(r) for r in rows]}


@app.get("/api/v1/analytics/payouts/aging")
async def payout_aging(limit: int = 100):
    """Open payout requests ordered by waiting time (oldest first)."""
    limit = rep.clamp_limit(limit)
    query, _ = rep.build_payout_aging_query()
    try:
        rows = _ch().fetch_all(query, {"limit": limit})
    except ClickHouseUnavailableError as exc:
        raise HTTPException(status_code=503, detail=str(exc)) from exc
    return {
        "data": [
            {
                "payout_id": str(r.get("payout_id")),
                "affiliate_id": str(r.get("affiliate_id")),
                "amount": rep.decimal_str(r.get("amount")),
                "currency": str(r.get("currency")),
                "status": str(r.get("status")),
                "requested_at": str(r.get("requested_at")),
                "wait_hours": int(r.get("wait_hours") or 0),
            }
            for r in rows
        ]
    }


@app.get("/api/v1/analytics/export")
async def export_table(
    table: str = Query(...),
    date_from: str = Query(..., pattern=r"^\d{4}-\d{2}-\d{2}$"),
    date_to: str = Query(..., pattern=r"^\d{4}-\d{2}-\d{2}$"),
    limit: int = 10000,
):
    """CSV export for allowlisted analytics tables in a date range."""
    try:
        start, end = rep.validate_date_range(date_from, date_to)
        query, _ = rep.build_export_query(table)
    except ValueError as exc:
        raise HTTPException(status_code=422, detail=str(exc)) from exc
    limit = rep.clamp_limit(limit, default=10000)
    try:
        rows = _ch().fetch_all(
            query, {"date_from": str(start), "date_to": str(end), "limit": limit}
        )
    except ClickHouseUnavailableError as exc:
        raise HTTPException(status_code=503, detail=str(exc)) from exc

    filename = f"{table}_{start}_{end}.csv"
    return StreamingResponse(
        _csv_chunks(rows),
        media_type="text/csv",
        headers={"Content-Disposition": f'attachment; filename="{filename}"'},
    )
