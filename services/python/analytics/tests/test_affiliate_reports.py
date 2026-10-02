"""Tests for query builders, validation and row mapping."""

from datetime import date
from decimal import Decimal

import pytest

from analytics.affiliate_reports import (
    EARNING_STATUSES,
    EXPORTABLE_TABLES,
    MAX_LIMIT,
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
    parse_iso_date,
    safe_div,
    summarize_earnings_by_status,
    validate_date_range,
    validate_export_table,
    validate_status,
)


class TestDateValidation:
    """Range validation protects ClickHouse from unbounded scans."""

    def test_parse_iso_date(self):
        assert parse_iso_date("2026-01-31") == date(2026, 1, 31)
        assert parse_iso_date(date(2026, 1, 31)) == date(2026, 1, 31)

    def test_parse_iso_date_rejects_garbage(self):
        with pytest.raises(ValidationError, match="ISO date"):
            parse_iso_date("31/01/2026")
        with pytest.raises(ValidationError, match="required"):
            parse_iso_date(None)

    def test_valid_range(self):
        assert validate_date_range("2026-01-01", "2026-01-31") == (
            date(2026, 1, 1),
            date(2026, 1, 31),
        )

    def test_inverted_range_rejected(self):
        with pytest.raises(ValidationError, match="must not be after"):
            validate_date_range("2026-02-01", "2026-01-01")

    def test_range_too_wide_rejected(self):
        with pytest.raises(ValidationError, match="must not exceed"):
            validate_date_range("2020-01-01", "2026-01-01", max_days=366)

    def test_default_range_is_bounded(self):
        start, end = default_range(30)
        assert (end - start).days == 29
        assert end == date.today()


class TestPagingClamps:
    """Page size and offset are coerced, never trusted."""

    @pytest.mark.parametrize(
        "raw,expected",
        [(1, 1), (10, 10), (10_000, MAX_LIMIT), (0, 1), (-5, 1), ("7", 7)],
    )
    def test_clamp_limit(self, raw, expected):
        assert clamp_limit(raw) == expected

    def test_clamp_limit_rejects_garbage(self):
        with pytest.raises(ValidationError):
            clamp_limit("many")

    @pytest.mark.parametrize("raw,expected", [(0, 0), (50, 50), (-3, 0), (10**9, 100_000)])
    def test_clamp_offset(self, raw, expected):
        assert clamp_offset(raw) == expected


class TestWhitelists:
    """Identifiers come from allowlists; user input never reaches SQL text."""

    def test_export_table_whitelist(self):
        for table in EXPORTABLE_TABLES:
            assert validate_export_table(table) == EXPORTABLE_TABLES[table]

    @pytest.mark.parametrize(
        "table", ["users", "affiliate_daily_aggregates; DROP TABLE x", "", "SYSTEM TABLES"]
    )
    def test_export_table_rejects_unknown(self, table):
        with pytest.raises(ValidationError, match="must be one of"):
            validate_export_table(table)

    def test_status_whitelist(self):
        for status in EARNING_STATUSES:
            assert validate_status(status.upper()) == status
        assert validate_status(None) is None

    @pytest.mark.parametrize("status", ["paid'; DROP TABLE x", "unknown", "1 OR 1=1"])
    def test_status_rejects_unknown(self, status):
        with pytest.raises(ValidationError, match="must be one of"):
            validate_status(status)

    def test_export_query_uses_whitelisted_identifiers(self):
        query = build_export_query("affiliate_payouts", "requested_at")
        assert "FROM affiliate_payouts" in query
        assert "requested_at BETWEEN %(date_from)s" in query
        # values stay bound
        assert "%(limit)s" in query


class TestQueriesAreParameterized:
    """No user value may be interpolated into SQL."""

    @pytest.mark.parametrize(
        "builder",
        [
            build_daily_summary_query,
            build_funnel_query,
            build_top_affiliates_query,
            build_ngr_trend_query,
            build_payout_aging_query,
            build_ltv_query,
            build_earnings_report_query,
            build_platform_totals_query,
        ],
    )
    def test_placeholders_present(self, builder):
        query = builder()
        assert "%(date_from)s" in query
        assert "%(date_to)s" in query
        assert "{" not in query.replace("{status_predicate}", "")

    def test_earnings_status_predicate_is_injected_by_caller(self):
        query = build_earnings_report_query().format(status_predicate="AND status = %(status)s")
        assert "AND status = %(status)s" in query
        assert "{status_predicate}" not in query


class TestDecimalHandling:
    """Money is rendered as decimal strings, never floats."""

    def test_decimal_str_keeps_scale(self):
        assert decimal_str(Decimal("2.5")) == "2.50"
        assert decimal_str("10") == "10.00"
        assert decimal_str(None) == "0.00"

    def test_decimal_str_full_precision(self):
        assert decimal_str(Decimal("1.23456789"), places=None) == "1.23456789"

    def test_safe_div_handles_zero(self):
        assert safe_div(1, 0) == Decimal(0)
        assert safe_div(10, 4) == Decimal("2.5")

    def test_mapped_money_is_string(self):
        row = {"ggr_amount": Decimal("100.12345678"), "ngr_amount": Decimal("80")}
        mapped = map_summary_row(row)
        assert isinstance(mapped["ggr"], str)
        assert mapped["ggr"] == "100.12"
        assert isinstance(mapped["ngr"], str)


class TestRowMapping:
    """Mappers are tolerant of NULLs and empty aggregates."""

    def test_summary_row(self):
        mapped = map_summary_row(
            {
                "report_date": date(2026, 1, 5),
                "clicks": 10,
                "registrations": 4,
                "ftd_count": 2,
                "active_players": 3,
                "ggr_amount": Decimal("500"),
                "ngr_amount": Decimal("120.5"),
                "commission_accrued": Decimal("12.05"),
                "commission_released": None,
            }
        )
        assert mapped["date"] == "2026-01-05"
        assert mapped["clicks"] == 10
        assert mapped["commission_released"] == "0.00"

    def test_empty_funnel_row_is_all_zeroes(self):
        mapped = map_funnel_row({})
        assert mapped["clicks"] == 0
        # Division by zero must not explode
        assert mapped["click_to_registration"] == "0.0000"

    def test_funnel_conversions(self):
        mapped = map_funnel_row(
            {"clicks": 100, "registrations": 10, "ftd_count": 5, "active_players": 4}
        )
        assert mapped["click_to_registration"] == "0.1000"
        assert mapped["registration_to_ftd"] == "0.5000"
        assert mapped["ftd_to_active"] == "0.8000"

    def test_top_affiliates_row(self):
        mapped = map_top_affiliates_row(
            {
                "affiliate_id": "aff-1",
                "total_ngr": Decimal("1000"),
                "total_ggr": Decimal("5000"),
                "total_commission": Decimal("100"),
                "total_ftd": 8,
                "total_registrations": 16,
                "total_clicks": 320,
            }
        )
        assert mapped["affiliate_id"] == "aff-1"
        assert mapped["ngr"] == "1000.00"
        assert mapped["ftd_rate"] == "0.5000"

    def test_earning_row_rate_keeps_four_places(self):
        mapped = map_earning_row(
            {
                "earning_id": "e1",
                "affiliate_id": "a1",
                "referred_user_id": 42,
                "source_type": "bet",
                "period_start": date(2026, 1, 1),
                "period_end": date(2026, 1, 31),
                "ggr_amount": Decimal("100"),
                "ngr_amount": Decimal("80"),
                "commission_rate": Decimal("0.2500"),
                "commission_amount": Decimal("20"),
                "status": "accrued",
            }
        )
        assert mapped["commission_rate"] == "0.2500"
        assert mapped["referred_user_id"] == 42

    def test_totals_row(self):
        mapped = map_totals_row(
            {
                "clicks": 200,
                "registrations": 20,
                "ftd_count": 5,
                "active_players": 10,
                "ggr_amount": Decimal("1000"),
                "ngr_amount": Decimal("400"),
                "commission_accrued": Decimal("40"),
                "commission_released": Decimal("30"),
            }
        )
        assert mapped["ftd_rate"] == "0.2500"

    def test_payout_aging_row(self):
        mapped = map_payout_aging_row(
            {
                "payout_id": "p1",
                "affiliate_id": "a1",
                "amount": Decimal("250.5"),
                "currency": "EUR",
                "status": "pending",
                "requested_at": date(2026, 1, 2),
                "wait_hours": 30,
            }
        )
        assert mapped["amount"] == "250.50"
        assert mapped["wait_hours"] == 30

    def test_ltv_row(self):
        mapped = map_ltv_row(
            {
                "referred_user_id": 7,
                "lifetime_ggr": Decimal("900"),
                "lifetime_deposits": Decimal("300"),
                "lifetime_bonuses": Decimal("100"),
                "active_days": 12,
            }
        )
        assert mapped["lifetime_ggr"] == "900.00"
        assert mapped["ggr_per_bonus"] == "9.0000"

    def test_ltv_row_without_bonuses(self):
        mapped = map_ltv_row(
            {
                "referred_user_id": 7,
                "lifetime_ggr": Decimal("900"),
                "lifetime_deposits": Decimal("300"),
                "lifetime_bonuses": None,
                "active_days": 12,
            }
        )
        assert mapped["ggr_per_bonus"] == "0.0000"

    def test_summarize_earnings_by_status_uses_decimal(self):
        totals = summarize_earnings_by_status(
            [
                {"status": "accrued", "ngr_amount": Decimal("10.10")},
                {"status": "accrued", "ngr_amount": Decimal("0.20")},
                {"status": "reversed", "ngr_amount": Decimal("5")},
            ]
        )
        assert totals["accrued"] == Decimal("10.30")
        assert isinstance(totals["accrued"], Decimal)
