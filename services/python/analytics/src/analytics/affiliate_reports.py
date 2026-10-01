"""Affiliate reporting queries and row mapping.

Pure functions only — no I/O, no third-party imports.
All SQL uses clickhouse-connect ``%(name)s`` placeholders; values are
never interpolated into the query string (SQL injection prevention).

Tables consumed (owned by the Affiliate Service, see
``services/go/affiliate/clickhouse/affiliate_analytics.sql`` — read-only
dependency, this module never writes to them):

- ``affiliate_daily_aggregates`` (SummingMergeTree, per affiliate/day)
- ``affiliate_earnings`` (financial records)
- ``affiliate_payouts`` (payout requests)
- ``referred_player_activity`` (per player/day, LTV + retention)
- ``affiliate_funnel_hourly`` (hourly funnel, 30d TTL)
"""

from __future__ import annotations

from datetime import UTC, date, datetime, timedelta
from decimal import Decimal, InvalidOperation
from typing import Any

# Tables whose export is allowed via the CSV export endpoint.
# Anything else is rejected (defense in depth: no arbitrary table reads).
EXPORTABLE_TABLES = frozenset(
    {
        "affiliate_daily_aggregates",
        "affiliate_earnings",
        "affiliate_payouts",
        "referred_player_activity",
    }
)

MAX_RANGE_DAYS = 366
MAX_LIMIT = 1000
MAX_OFFSET = 100000

# Canonical earning statuses (affiliate_earnings.status).
EARNING_STATUSES = frozenset({"accrued", "pending", "available", "paid", "reversed"})


def parse_iso_date(value: str) -> date:
    """Parse ``YYYY-MM-DD`` or raise ValueError."""
    parts = value.split("-")
    if len(parts) != 3:
        raise ValueError(f"invalid date, expected YYYY-MM-DD: {value!r}")
    try:
        return date(int(parts[0]), int(parts[1]), int(parts[2]))
    except (ValueError, TypeError) as exc:
        raise ValueError(f"invalid date {value!r}: {exc}") from exc


def validate_date_range(date_from: str, date_to: str) -> tuple[date, date]:
    """Validate an inclusive date range. Raises ValueError on violation."""
    start = parse_iso_date(date_from)
    end = parse_iso_date(date_to)
    if start > end:
        raise ValueError("date_from must not be after date_to")
    if (end - start).days > MAX_RANGE_DAYS:
        raise ValueError(f"date range must not exceed {MAX_RANGE_DAYS} days")
    # UTC, not local: ClickHouse partitions by server (UTC) date, so a
    # local-time "today" would shift the allowed boundary with the host TZ.
    if end > datetime.now(UTC).date() + timedelta(days=1):
        raise ValueError("date_to must not be in the future")
    return start, end


def clamp_limit(limit: int, default: int = 100) -> int:
    """Clamp a user-supplied limit into [1, MAX_LIMIT]."""
    try:
        value = int(limit)
    except (TypeError, ValueError):
        return default
    return max(1, min(value, MAX_LIMIT))


def clamp_offset(offset: int) -> int:
    """Clamp a user-supplied offset into [0, MAX_OFFSET]."""
    try:
        value = int(offset)
    except (TypeError, ValueError):
        return 0
    return max(0, min(value, MAX_OFFSET))


def safe_div(numerator: Any, denominator: Any) -> float:
    """Ratio helper: returns 0.0 on zero/invalid denominator (funnel math)."""
    try:
        denom = float(denominator)
    except (TypeError, ValueError):
        return 0.0
    if denom == 0:
        return 0.0
    try:
        return float(numerator) / denom
    except (TypeError, ValueError, ZeroDivisionError):
        return 0.0


def decimal_str(value: Any) -> str:
    """Money formatter: Decimal-safe string with 8 fractional digits.

    Financial amounts are NEVER floats (CONVENTIONS NEVER-6).
    """
    try:
        quantized = Decimal(str(value)).quantize(Decimal("0.00000001"))
        return format(quantized, "f")
    except (InvalidOperation, ValueError, TypeError):
        return "0.00000000"


def build_daily_summary_query() -> tuple[str, list[str]]:
    """Per-day aggregates for one affiliate in a date range."""
    query = """
        SELECT
            report_date,
            sum(clicks) AS clicks,
            sum(registrations) AS registrations,
            sum(ftd_count) AS ftd_count,
            sum(active_players) AS active_players,
            sum(ggr_amount) AS ggr_amount,
            sum(ngr_amount) AS ngr_amount,
            sum(commission_accrued) AS commission_accrued,
            sum(commission_released) AS commission_released,
            sum(commission_reversed) AS commission_reversed
        FROM affiliate_daily_aggregates
        WHERE affiliate_id = %(affiliate_id)s
          AND report_date BETWEEN %(date_from)s AND %(date_to)s
        GROUP BY report_date
        ORDER BY report_date ASC
    """
    return query, ["affiliate_id", "date_from", "date_to"]


def build_funnel_query() -> tuple[str, list[str]]:
    """Conversion funnel totals for one affiliate over trailing N days."""
    query = """
        SELECT
            sum(clicks) AS clicks,
            sum(registrations) AS registrations,
            sum(ftd_count) AS ftd_count,
            sum(active_players) AS active_players
        FROM affiliate_daily_aggregates
        WHERE affiliate_id = %(affiliate_id)s
          AND report_date >= today() - %(days)s
    """
    return query, ["affiliate_id", "days"]


def build_top_affiliates_query() -> tuple[str, list[str]]:
    """Top affiliates by NGR over trailing N days."""
    query = """
        SELECT
            affiliate_id,
            sum(ngr_amount) AS total_ngr,
            sum(ggr_amount) AS total_ggr,
            sum(commission_accrued) AS total_commission,
            sum(ftd_count) AS total_ftds
        FROM affiliate_daily_aggregates
        WHERE report_date >= today() - %(days)s
        GROUP BY affiliate_id
        ORDER BY total_ngr DESC
        LIMIT %(limit)s
    """
    return query, ["days", "limit"]


def build_top_affiliates_range_query() -> tuple[str, list[str]]:
    """Top affiliates by NGR inside an explicit date range."""
    query = """
        SELECT
            affiliate_id,
            sum(ngr_amount) AS total_ngr,
            sum(ggr_amount) AS total_ggr,
            sum(commission_accrued) AS total_commission,
            sum(ftd_count) AS total_ftds
        FROM affiliate_daily_aggregates
        WHERE report_date BETWEEN %(date_from)s AND %(date_to)s
        GROUP BY affiliate_id
        ORDER BY total_ngr DESC
        LIMIT %(limit)s
    """
    return query, ["date_from", "date_to", "limit"]


def build_ngr_trend_query() -> tuple[str, list[str]]:
    """Daily NGR/commission trend for one affiliate over trailing N days."""
    query = """
        SELECT
            report_date,
            sum(ngr_amount) AS ngr_amount,
            sum(ggr_amount) AS ggr_amount,
            sum(commission_accrued) AS commission_accrued
        FROM affiliate_daily_aggregates
        WHERE affiliate_id = %(affiliate_id)s
          AND report_date >= today() - %(days)s
        GROUP BY report_date
        ORDER BY report_date ASC
    """
    return query, ["affiliate_id", "days"]


def build_payout_aging_query() -> tuple[str, list[str]]:
    """Open payout requests ordered by waiting time (oldest first)."""
    query = """
        SELECT
            payout_id,
            affiliate_id,
            amount,
            currency,
            status,
            requested_at,
            dateDiff('hour', requested_at, now()) AS wait_hours
        FROM affiliate_payouts
        WHERE status NOT IN ('paid', 'rejected', 'failed')
        ORDER BY requested_at ASC
        LIMIT %(limit)s
    """
    return query, ["limit"]


def build_ltv_query() -> tuple[str, list[str]]:
    """Lifetime GGR per referred player for one affiliate (LTV ranking)."""
    query = """
        SELECT
            referred_user_id,
            sum(ggr) AS lifetime_ggr,
            sum(deposits_amount) AS lifetime_deposits,
            count() AS active_days
        FROM referred_player_activity
        WHERE affiliate_id = %(affiliate_id)s
        GROUP BY referred_user_id
        ORDER BY lifetime_ggr DESC
        LIMIT %(limit)s
    """
    return query, ["affiliate_id", "limit"]


def build_export_query(table: str) -> tuple[str, list[str]]:
    """Bounded export query for an allowlisted table in a date range.

    Raises ValueError for non-allowlisted tables. Date column differs
    per table; only tables with a date partition column are supported.
    """
    date_columns = {
        "affiliate_daily_aggregates": "report_date",
        "affiliate_earnings": "created_at",
        "affiliate_payouts": "created_at",
        "referred_player_activity": "activity_date",
    }
    if table not in EXPORTABLE_TABLES or table not in date_columns:
        raise ValueError(f"table is not exportable: {table!r}")
    column = date_columns[table]
    query = f"""
        SELECT *
        FROM {table}
        WHERE {column} BETWEEN %(date_from)s AND %(date_to)s
        ORDER BY {column} ASC
        LIMIT %(limit)s
    """
    return query, ["date_from", "date_to", "limit"]


def map_summary_row(row: dict) -> dict:
    """Map one daily-aggregate row to the API shape (money as strings)."""
    return {
        "report_date": str(row.get("report_date")),
        "clicks": int(row.get("clicks") or 0),
        "registrations": int(row.get("registrations") or 0),
        "ftd_count": int(row.get("ftd_count") or 0),
        "active_players": int(row.get("active_players") or 0),
        "ggr_amount": decimal_str(row.get("ggr_amount")),
        "ngr_amount": decimal_str(row.get("ngr_amount")),
        "commission_accrued": decimal_str(row.get("commission_accrued")),
        "commission_released": decimal_str(row.get("commission_released")),
        "commission_reversed": decimal_str(row.get("commission_reversed")),
    }


def map_funnel_row(row: dict) -> dict:
    """Map funnel totals to counts + conversion rates."""
    clicks = int(row.get("clicks") or 0)
    registrations = int(row.get("registrations") or 0)
    ftds = int(row.get("ftd_count") or 0)
    active = int(row.get("active_players") or 0)
    return {
        "clicks": clicks,
        "registrations": registrations,
        "ftd_count": ftds,
        "active_players": active,
        "click_to_reg_rate": safe_div(registrations, clicks),
        "reg_to_ftd_rate": safe_div(ftds, registrations),
        "ftd_to_active_rate": safe_div(active, ftds),
    }


def map_top_affiliates_row(row: dict) -> dict:
    """Map one top-affiliate row to the API shape."""
    return {
        "affiliate_id": str(row.get("affiliate_id")),
        "total_ngr": decimal_str(row.get("total_ngr")),
        "total_ggr": decimal_str(row.get("total_ggr")),
        "total_commission": decimal_str(row.get("total_commission")),
        "total_ftds": int(row.get("total_ftds") or 0),
    }


def summarize_earnings_by_status(rows: list[dict]) -> dict:
    """Aggregate commission amounts per earning status.

    Used by the partner dashboard (pending / available / paid / reversed).
    Amounts stay Decimal until the final string formatting.
    """
    totals: dict[str, Decimal] = {}
    for row in rows:
        status = str(row.get("status") or "unknown")
        try:
            amount = Decimal(str(row.get("commission_amount") or "0"))
        except (InvalidOperation, ValueError, TypeError):
            amount = Decimal(0)
        totals[status] = totals.get(status, Decimal(0)) + amount
    return {status: decimal_str(amount) for status, amount in sorted(totals.items())}


def build_earnings_report_query(
    status: str | None = None, affiliate_id: str | None = None
) -> tuple[str, list[str]]:
    """Paginated earnings report with optional status/affiliate filters.

    Status is validated against EARNING_STATUSES (query param comes from
    the user). Optional filters only add parameterized predicates —
    no string interpolation of values.
    """
    if status is not None and status not in EARNING_STATUSES:
        raise ValueError(f"invalid earning status: {status!r}")
    conditions = ["period_start >= %(date_from)s", "period_start <= %(date_to)s"]
    params = ["date_from", "date_to"]
    if status is not None:
        conditions.append("status = %(status)s")
        params.append("status")
    if affiliate_id is not None:
        conditions.append("affiliate_id = %(affiliate_id)s")
        params.append("affiliate_id")
    where = " AND ".join(conditions)
    query = f"""
        SELECT
            earning_id,
            affiliate_id,
            referred_user_id,
            source_type,
            period_start,
            period_end,
            ggr_amount,
            ngr_amount,
            commission_rate,
            commission_amount,
            status,
            hold_until,
            created_at
        FROM affiliate_earnings
        WHERE {where}
        ORDER BY period_start DESC
        LIMIT %(limit)s OFFSET %(offset)s
    """
    return query, params + ["limit", "offset"]


def build_platform_totals_query() -> tuple[str, list[str]]:
    """Platform-wide totals across all affiliates in a date range."""
    query = """
        SELECT
            sum(clicks) AS clicks,
            sum(registrations) AS registrations,
            sum(ftd_count) AS ftd_count,
            sum(active_players) AS active_players,
            sum(ggr_amount) AS ggr_amount,
            sum(ngr_amount) AS ngr_amount,
            sum(commission_accrued) AS commission_accrued,
            sum(commission_released) AS commission_released,
            sum(commission_reversed) AS commission_reversed,
            uniqExact(affiliate_id) AS active_affiliates
        FROM affiliate_daily_aggregates
        WHERE report_date BETWEEN %(date_from)s AND %(date_to)s
    """
    return query, ["date_from", "date_to"]


def map_earning_row(row: dict) -> dict:
    """Map one affiliate_earnings row to the API shape."""
    return {
        "earning_id": str(row.get("earning_id")),
        "affiliate_id": str(row.get("affiliate_id")),
        "referred_user_id": row.get("referred_user_id"),
        "source_type": str(row.get("source_type")),
        "period_start": str(row.get("period_start")),
        "period_end": str(row.get("period_end")),
        "ggr_amount": decimal_str(row.get("ggr_amount")),
        "ngr_amount": decimal_str(row.get("ngr_amount")),
        "commission_rate": decimal_str(row.get("commission_rate")),
        "commission_amount": decimal_str(row.get("commission_amount")),
        "status": str(row.get("status")),
        "hold_until": str(row.get("hold_until")),
        "created_at": str(row.get("created_at")),
    }


def map_totals_row(row: dict) -> dict:
    """Map a platform-totals row (no report_date) to the API shape."""
    mapped = map_summary_row(
        {
            "report_date": "",
            "clicks": row.get("clicks"),
            "registrations": row.get("registrations"),
            "ftd_count": row.get("ftd_count"),
            "active_players": row.get("active_players"),
            "ggr_amount": row.get("ggr_amount"),
            "ngr_amount": row.get("ngr_amount"),
            "commission_accrued": row.get("commission_accrued"),
            "commission_released": row.get("commission_released"),
            "commission_reversed": row.get("commission_reversed"),
        }
    )
    mapped.pop("report_date", None)
    mapped["active_affiliates"] = int(row.get("active_affiliates") or 0)
    return mapped
