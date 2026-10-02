"""Affiliate reporting: query builders, validation and row mapping.

Rules that shape this module:
- **No floats for money.** ClickHouse hands back `Decimal(18,8)`; values are
  serialized as decimal *strings* (CONVENTIONS: NEVER-6).
- **No value interpolation.** Every user-supplied value is bound as a
  parameter; identifiers (export tables) come from a hardcoded whitelist.
- **Bounded work.** Date ranges and page sizes are clamped so a single
  request cannot ask ClickHouse for an unbounded aggregate.
"""

from __future__ import annotations

from datetime import date, datetime, timedelta
from decimal import Decimal, InvalidOperation
from typing import Any

MAX_LIMIT = 500
MAX_OFFSET = 100_000
MAX_RANGE_DAYS = 366
DEFAULT_LIMIT = 100

#: Tables a client may export, with the date column used for the range filter.
EXPORTABLE_TABLES: dict[str, str] = {
    "affiliate_daily_aggregates": "report_date",
    "affiliate_earnings": "period_start",
    "affiliate_payouts": "requested_at",
    "referred_player_activity": "activity_date",
    "affiliate_attributions": "attributed_at",
}

#: Statuses accepted by the earnings report filter (LowCardinality values).
EARNING_STATUSES: tuple[str, ...] = (
    "pending",
    "accrued",
    "released",
    "reversed",
    "held",
)


class ValidationError(ValueError):
    """Invalid query parameters (mapped to HTTP 422)."""


# ----------------------------------------------------------------------
# Value helpers
# ----------------------------------------------------------------------
def parse_iso_date(value: str | date | datetime | None, field: str = "date") -> date:
    """Parse an ISO-8601 date/datetime into a `date`."""
    if value is None or value == "":
        raise ValidationError(f"{field} is required")
    if isinstance(value, datetime):
        return value.date()
    if isinstance(value, date):
        return value
    try:
        return date.fromisoformat(str(value).strip())
    except ValueError as exc:
        raise ValidationError(f"{field} must be an ISO date (YYYY-MM-DD)") from exc


def validate_date_range(
    date_from: str | date | datetime,
    date_to: str | date | datetime,
    max_days: int = MAX_RANGE_DAYS,
) -> tuple[date, date]:
    """Validate a [date_from, date_to] range and return it as dates."""
    start = parse_iso_date(date_from, "date_from")
    end = parse_iso_date(date_to, "date_to")
    if start > end:
        raise ValidationError("date_from must not be after date_to")
    if (end - start).days > max_days:
        raise ValidationError(f"date range must not exceed {max_days} days")
    return start, end


def default_range(days: int = 30) -> tuple[date, date]:
    """Range ending today, for dashboards that do not pass dates."""
    today = date.today()
    return today - timedelta(days=days - 1), today


def clamp_limit(value: Any, maximum: int = MAX_LIMIT) -> int:
    """Coerce a page size into [1, maximum]."""
    try:
        limit = int(value)
    except (TypeError, ValueError) as exc:
        raise ValidationError("limit must be an integer") from exc
    if limit < 1:
        return 1
    return min(limit, maximum)


def clamp_offset(value: Any, maximum: int = MAX_OFFSET) -> int:
    """Coerce an offset into [0, maximum]."""
    try:
        offset = int(value)
    except (TypeError, ValueError) as exc:
        raise ValidationError("offset must be an integer") from exc
    if offset < 0:
        return 0
    return min(offset, maximum)


def safe_div(numerator: Decimal | int, denominator: Decimal | int) -> Decimal:
    """Decimal division that returns 0 instead of dividing by zero."""
    num = _to_decimal(numerator)
    den = _to_decimal(denominator)
    if den == 0:
        return Decimal(0)
    return num / den


def _to_decimal(value: Any) -> Decimal:
    if isinstance(value, Decimal):
        return value
    if value is None:
        return Decimal(0)
    try:
        return Decimal(str(value))
    except InvalidOperation:
        return Decimal(0)


def decimal_str(value: Any, places: int | None = 2) -> str:
    """
    Render a money value as a decimal string.

    Money never becomes a float: `Decimal(str(value))` with optional
    quantization keeps `2.50` instead of `2.5`.
    """
    amount = _to_decimal(value)
    if places is None:
        return format(amount, "f")
    return format(amount.quantize(Decimal(1).scaleb(-places)), "f")


def _row_date(row: dict[str, Any], column: str) -> str | None:
    value = row.get(column)
    if value is None:
        return None
    if isinstance(value, (datetime,)):
        return value.date().isoformat()
    if isinstance(value, date):
        return value.isoformat()
    return str(value)


def _row_int(row: dict[str, Any], column: str) -> int:
    value = row.get(column)
    if value is None:
        return 0
    try:
        return int(value)
    except (TypeError, ValueError):
        return 0


# ----------------------------------------------------------------------
# Query builders — all values bound as parameters
# ----------------------------------------------------------------------
def build_daily_summary_query() -> str:
    """Per-day affiliate funnel + revenue series."""
    return """
        SELECT
            report_date,
            sum(clicks) AS clicks,
            sum(registrations) AS registrations,
            sum(ftd_count) AS ftd_count,
            sum(active_players) AS active_players,
            sum(ggr_amount) AS ggr_amount,
            sum(ngr_amount) AS ngr_amount,
            sum(commission_accrued) AS commission_accrued,
            sum(commission_released) AS commission_released
        FROM affiliate_daily_aggregates
        WHERE report_date BETWEEN %(date_from)s AND %(date_to)s
        GROUP BY report_date
        ORDER BY report_date
    """


def build_funnel_query() -> str:
    """Aggregated funnel for the requested range."""
    return """
        SELECT
            sum(clicks) AS clicks,
            sum(registrations) AS registrations,
            sum(ftd_count) AS ftd_count,
            sum(active_players) AS active_players,
            sum(ggr_amount) AS ggr_amount,
            sum(ngr_amount) AS ngr_amount
        FROM affiliate_daily_aggregates
        WHERE report_date BETWEEN %(date_from)s AND %(date_to)s
    """


def build_top_affiliates_query() -> str:
    """Top affiliates by NGR for the requested range."""
    return """
        SELECT
            affiliate_id,
            sum(ngr_amount) AS total_ngr,
            sum(ggr_amount) AS total_ggr,
            sum(commission_accrued) AS total_commission,
            sum(ftd_count) AS total_ftd,
            sum(registrations) AS total_registrations,
            sum(clicks) AS total_clicks
        FROM affiliate_daily_aggregates
        WHERE report_date BETWEEN %(date_from)s AND %(date_to)s
        GROUP BY affiliate_id
        ORDER BY total_ngr DESC, affiliate_id ASC
        LIMIT %(limit)s
    """


def build_ngr_trend_query() -> str:
    """Daily GGR/NGR trend (executive dashboard)."""
    return """
        SELECT
            report_date,
            sum(ggr_amount) AS ggr_amount,
            sum(ngr_amount) AS ngr_amount,
            sum(commission_accrued) AS commission_accrued
        FROM affiliate_daily_aggregates
        WHERE report_date BETWEEN %(date_from)s AND %(date_to)s
        GROUP BY report_date
        ORDER BY report_date
    """


def build_payout_aging_query() -> str:
    """Open payouts with their waiting time (hours since request)."""
    return """
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
          AND requested_at BETWEEN %(date_from)s AND %(date_to)s
        ORDER BY requested_at ASC
        LIMIT %(limit)s
    """


def build_ltv_query() -> str:
    """Per-referred-player lifetime value over the range."""
    return """
        SELECT
            referred_user_id,
            sum(ggr) AS lifetime_ggr,
            sum(deposits_amount) AS lifetime_deposits,
            sum(bonuses_received) AS lifetime_bonuses,
            count() AS active_days
        FROM referred_player_activity
        WHERE activity_date BETWEEN %(date_from)s AND %(date_to)s
        GROUP BY referred_user_id
        ORDER BY lifetime_ggr DESC
        LIMIT %(limit)s
    """


def build_earnings_report_query() -> str:
    """
    Paginated earnings report.

    `status` is optional and validated against EARNING_STATUSES: the value
    comes from an HTTP query parameter, so it must never be interpolated.
    """
    return """
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
        WHERE period_start BETWEEN %(date_from)s AND %(date_to)s
          {status_predicate}
        ORDER BY period_start DESC, earning_id ASC
        LIMIT %(limit)s OFFSET %(offset)s
    """


def build_platform_totals_query() -> str:
    """Platform-wide affiliate totals for the range."""
    return """
        SELECT
            sum(clicks) AS clicks,
            sum(registrations) AS registrations,
            sum(ftd_count) AS ftd_count,
            sum(active_players) AS active_players,
            sum(ggr_amount) AS ggr_amount,
            sum(ngr_amount) AS ngr_amount,
            sum(commission_accrued) AS commission_accrued,
            sum(commission_released) AS commission_released
        FROM affiliate_daily_aggregates
        WHERE report_date BETWEEN %(date_from)s AND %(date_to)s
    """


def build_export_query(table: str, date_column: str) -> str:
    """
    CSV export for a whitelisted table.

    `table` and `date_column` must come from EXPORTABLE_TABLES — identifiers
    cannot be bound as parameters, so the whitelist is the only safe path.
    """
    order_column = "report_date" if date_column == "report_date" else date_column
    # nosec marker must sit on the flagged line itself (bandit scans per line).
    head = f"SELECT * FROM {table} WHERE {date_column}"  # nosec B608 - whitelist only
    query = (
        f"{head} BETWEEN %(date_from)s AND %(date_to)s "
        f"ORDER BY {order_column} ASC LIMIT %(limit)s"
    )
    return query


def validate_export_table(table: str) -> str:
    """Return the export date column or reject the table."""
    if table not in EXPORTABLE_TABLES:
        allowed = ", ".join(sorted(EXPORTABLE_TABLES))
        raise ValidationError(f"table must be one of: {allowed}")
    return EXPORTABLE_TABLES[table]


def validate_status(status: str | None) -> str | None:
    """Normalize an optional earnings status filter."""
    if status is None or status == "":
        return None
    normalized = str(status).strip().lower()
    if normalized not in EARNING_STATUSES:
        allowed = ", ".join(EARNING_STATUSES)
        raise ValidationError(f"status must be one of: {allowed}")
    return normalized


# ----------------------------------------------------------------------
# Row mapping — money stays a decimal string
# ----------------------------------------------------------------------
def map_summary_row(row: dict[str, Any]) -> dict[str, Any]:
    return {
        "date": _row_date(row, "report_date"),
        "clicks": _row_int(row, "clicks"),
        "registrations": _row_int(row, "registrations"),
        "ftd_count": _row_int(row, "ftd_count"),
        "active_players": _row_int(row, "active_players"),
        "ggr": decimal_str(row.get("ggr_amount")),
        "ngr": decimal_str(row.get("ngr_amount")),
        "commission_accrued": decimal_str(row.get("commission_accrued")),
        "commission_released": decimal_str(row.get("commission_released")),
    }


def map_funnel_row(row: dict[str, Any]) -> dict[str, Any]:
    clicks = _row_int(row, "clicks")
    registrations = _row_int(row, "registrations")
    ftd = _row_int(row, "ftd_count")
    active = _row_int(row, "active_players")

    return {
        "clicks": clicks,
        "registrations": registrations,
        "ftd_count": ftd,
        "active_players": active,
        "ggr": decimal_str(row.get("ggr_amount")),
        "ngr": decimal_str(row.get("ngr_amount")),
        # Conversion rates as decimal strings (division never yields float).
        "click_to_registration": decimal_str(safe_div(registrations, clicks), places=4),
        "registration_to_ftd": decimal_str(safe_div(ftd, registrations), places=4),
        "ftd_to_active": decimal_str(safe_div(active, ftd), places=4),
    }


def map_top_affiliates_row(row: dict[str, Any]) -> dict[str, Any]:
    clicks = _row_int(row, "total_clicks")
    registrations = _row_int(row, "total_registrations")
    ftd = _row_int(row, "total_ftd")

    return {
        "affiliate_id": str(row.get("affiliate_id") or ""),
        "ngr": decimal_str(row.get("total_ngr")),
        "ggr": decimal_str(row.get("total_ggr")),
        "commission": decimal_str(row.get("total_commission")),
        "ftd_count": ftd,
        "registrations": registrations,
        "clicks": clicks,
        "ftd_rate": decimal_str(safe_div(ftd, registrations), places=4),
    }


def map_earning_row(row: dict[str, Any]) -> dict[str, Any]:
    return {
        "earning_id": str(row.get("earning_id") or ""),
        "affiliate_id": str(row.get("affiliate_id") or ""),
        "referred_user_id": _row_int(row, "referred_user_id"),
        "source_type": str(row.get("source_type") or ""),
        "period_start": _row_date(row, "period_start"),
        "period_end": _row_date(row, "period_end"),
        "ggr": decimal_str(row.get("ggr_amount")),
        "ngr": decimal_str(row.get("ngr_amount")),
        # Rate is a fraction (0..1) in ClickHouse Decimal(10,4).
        "commission_rate": decimal_str(row.get("commission_rate"), places=4),
        "commission": decimal_str(row.get("commission_amount")),
        "status": str(row.get("status") or ""),
    }


def map_totals_row(row: dict[str, Any]) -> dict[str, Any]:
    registrations = _row_int(row, "registrations")
    ftd = _row_int(row, "ftd_count")
    clicks = _row_int(row, "clicks")

    return {
        "clicks": clicks,
        "registrations": registrations,
        "ftd_count": ftd,
        "active_players": _row_int(row, "active_players"),
        "ggr": decimal_str(row.get("ggr_amount")),
        "ngr": decimal_str(row.get("ngr_amount")),
        "commission_accrued": decimal_str(row.get("commission_accrued")),
        "commission_released": decimal_str(row.get("commission_released")),
        "ftd_rate": decimal_str(safe_div(ftd, registrations), places=4),
        "click_to_registration": decimal_str(safe_div(registrations, clicks), places=4),
    }


def map_trend_row(row: dict[str, Any]) -> dict[str, Any]:
    """Daily GGR/NGR point for the executive dashboard."""
    return {
        "date": _row_date(row, "report_date"),
        "ggr": decimal_str(row.get("ggr_amount")),
        "ngr": decimal_str(row.get("ngr_amount")),
        "commission": decimal_str(row.get("commission_accrued")),
    }


def map_payout_aging_row(row: dict[str, Any]) -> dict[str, Any]:
    return {
        "payout_id": str(row.get("payout_id") or ""),
        "affiliate_id": str(row.get("affiliate_id") or ""),
        "amount": decimal_str(row.get("amount")),
        "currency": str(row.get("currency") or ""),
        "status": str(row.get("status") or ""),
        "requested_at": _row_date(row, "requested_at"),
        "wait_hours": _row_int(row, "wait_hours"),
    }


def map_ltv_row(row: dict[str, Any]) -> dict[str, Any]:
    deposits = _to_decimal(row.get("lifetime_deposits"))
    bonuses = _to_decimal(row.get("lifetime_bonuses"))
    ggr = _to_decimal(row.get("lifetime_ggr"))

    return {
        "referred_user_id": _row_int(row, "referred_user_id"),
        "lifetime_ggr": decimal_str(ggr),
        "lifetime_deposits": decimal_str(deposits),
        "lifetime_bonuses": decimal_str(bonuses),
        "active_days": _row_int(row, "active_days"),
        # Bonus efficiency: how much GGR each deposited/bonus unit produced.
        "ggr_per_bonus": decimal_str(safe_div(ggr, bonuses), places=4),
    }


def summarize_earnings_by_status(rows: list[dict[str, Any]]) -> dict[str, Decimal]:
    """Sum NGR per status, as Decimal (no float accumulation)."""
    totals: dict[str, Decimal] = {}
    for row in rows:
        status = str(row.get("status") or "unknown")
        totals[status] = totals.get(status, Decimal(0)) + _to_decimal(row.get("ngr_amount"))
    return totals
