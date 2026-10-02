"""Tests for the analytics HTTP API (ClickHouse stubbed)."""

from __future__ import annotations

from datetime import date
from decimal import Decimal

import pytest

from tests.conftest import FakeQueryResult

SUMMARY_COLUMNS = [
    "report_date",
    "clicks",
    "registrations",
    "ftd_count",
    "active_players",
    "ggr_amount",
    "ngr_amount",
    "commission_accrued",
    "commission_released",
]

SUMMARY_ROW = (
    date(2026, 1, 5),
    100,
    20,
    5,
    9,
    Decimal("1000.00"),
    Decimal("400.00"),
    Decimal("40.00"),
    Decimal("25.00"),
)


@pytest.fixture
def summary_rows(fake_driver):
    fake_driver.results = [FakeQueryResult(SUMMARY_COLUMNS, [SUMMARY_ROW])]


class TestReports:
    """Happy paths return mapped payloads."""

    def test_summary(self, api, summary_rows):
        resp = api.get("/api/v1/analytics/affiliate/summary")
        assert resp.status_code == 200
        body = resp.json()
        assert len(body) == 1
        assert body[0]["date"] == "2026-01-05"
        assert body[0]["clicks"] == 100
        # money stays a string in JSON
        assert body[0]["ggr"] == "1000.00"

    def test_funnel_empty_aggregate_is_zeroes(self, api):
        resp = api.get("/api/v1/analytics/affiliate/funnel")
        assert resp.status_code == 200
        assert resp.json()["clicks"] == 0
        assert resp.json()["click_to_registration"] == "0.0000"

    def test_top_affiliates_limit_is_bound(self, api, fake_driver):
        fake_driver.results = [
            FakeQueryResult(["affiliate_id", "total_ngr"], [("a1", Decimal("10"))])
        ]
        resp = api.get("/api/v1/analytics/affiliate/top?limit=999999")
        assert resp.status_code == 200
        # value bound, not interpolated
        _, params = fake_driver.calls[0]
        assert params["limit"] == 500

    def test_earnings_rejects_unknown_status(self, api):
        resp = api.get("/api/v1/analytics/affiliate/earnings?status=hacked")
        assert resp.status_code == 422

    def test_earnings_binds_status_parameter(self, api, fake_driver):
        fake_driver.results = [
            FakeQueryResult(
                ["earning_id", "affiliate_id", "ngr_amount", "status"],
                [("e1", "a1", Decimal("10.50"), "accrued")],
            )
        ]
        resp = api.get("/api/v1/analytics/affiliate/earnings?status=accrued")

        assert resp.status_code == 200
        body = resp.json()
        assert body["total"] == 1
        assert body["items"][0]["earning_id"] == "e1"
        assert body["ngr_by_status"]["accrued"] == "10.50"

        query, params = fake_driver.calls[0]
        assert "AND status = %(status)s" in query
        assert params["status"] == "accrued"

    def test_invalid_date_is_422(self, api):
        resp = api.get("/api/v1/analytics/affiliate/summary?date_from=nope&date_to=2026-01-01")
        assert resp.status_code == 422

    def test_inverted_range_is_422(self, api):
        resp = api.get(
            "/api/v1/analytics/affiliate/summary?date_from=2026-02-01&date_to=2026-01-01"
        )
        assert resp.status_code == 422


class TestExport:
    """CSV export is whitelist-guarded and streamed."""

    def test_export_returns_csv(self, api, fake_driver):
        fake_driver.results = [
            FakeQueryResult(
                ["payout_id", "amount"],
                [("p1", Decimal("10.50")), ("p2", Decimal("20"))],
            )
        ]
        resp = api.get("/api/v1/analytics/export/affiliate_payouts")

        assert resp.status_code == 200
        assert resp.headers["content-type"].startswith("text/csv")
        assert "attachment" in resp.headers["content-disposition"]
        text = resp.text
        assert "payout_id,amount" in text
        assert "10.50" in text

    def test_export_rejects_unknown_table(self, api):
        resp = api.get("/api/v1/analytics/export/users")
        assert resp.status_code == 422

    def test_export_injection_attempt_rejected(self, api):
        resp = api.get("/api/v1/analytics/export/affiliate_payouts%3B%20DROP%20TABLE%20x")
        assert resp.status_code in (404, 422)


class TestDependencyOutages:
    """ClickHouse failures surface as 503, never as fake zeroes."""

    @pytest.mark.parametrize(
        "path",
        [
            "/api/v1/analytics/affiliate/summary",
            "/api/v1/analytics/affiliate/funnel",
            "/api/v1/analytics/affiliate/totals",
            "/api/v1/analytics/affiliate/top",
            "/api/v1/analytics/affiliate/earnings",
        ],
    )
    def test_reports_return_503(self, broken_api, path):
        resp = broken_api.get(path)
        assert resp.status_code == 503
        assert "unavailable" in resp.json()["detail"]


class TestEntryPointShim:
    """The container shim must expose the ASGI app."""

    def test_shim_exports_app(self):
        """`uvicorn main:app` must work from the service directory."""
        import importlib
        import sys
        from pathlib import Path

        service_dir = str(Path(__file__).resolve().parents[1])
        if service_dir not in sys.path:
            sys.path.insert(0, service_dir)

        shim = importlib.import_module("main")
        assert shim.app is not None

    def test_shim_and_package_share_one_app(self):
        """No duplicate module instances (Prometheus would raise)."""
        import importlib
        import sys
        from pathlib import Path

        from analytics.main import app as package_app

        service_dir = str(Path(__file__).resolve().parents[1])
        if service_dir not in sys.path:
            sys.path.insert(0, service_dir)

        shim = importlib.import_module("main")
        assert shim.app is package_app
