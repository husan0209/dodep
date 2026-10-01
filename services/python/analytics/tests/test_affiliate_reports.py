"""Unit tests for affiliate reporting logic.

Stdlib only (``unittest``) — no fastapi/pytest/clickhouse-connect needed.
Run: ``py -m unittest discover -s tests -v`` from the service directory.
"""

import os
import sys
import unittest
from datetime import date
from decimal import Decimal

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "src"))

from analytics import affiliate_reports as rep


class TestDateValidation(unittest.TestCase):
    def test_valid_range(self):
        start, end = rep.validate_date_range("2026-09-01", "2026-09-30")
        self.assertEqual(start, date(2026, 9, 1))
        self.assertEqual(end, date(2026, 9, 30))

    def test_reversed_range_rejected(self):
        with self.assertRaises(ValueError):
            rep.validate_date_range("2026-09-30", "2026-09-01")

    def test_bad_format_rejected(self):
        for bad in ("2026/09/01", "2026-13-01", "tomorrow", "", "2026-9-1x"):
            with self.assertRaises(ValueError, msg=bad):
                rep.parse_iso_date(bad)

    def test_too_wide_range_rejected(self):
        with self.assertRaises(ValueError):
            rep.validate_date_range("2024-01-01", "2026-09-30")

    def test_future_end_rejected(self):
        with self.assertRaises(ValueError):
            rep.validate_date_range("2026-01-01", "2999-01-01")


class TestLimits(unittest.TestCase):
    def test_clamp(self):
        self.assertEqual(rep.clamp_limit(5), 5)
        self.assertEqual(rep.clamp_limit(0), 1)
        self.assertEqual(rep.clamp_limit(-3), 1)
        self.assertEqual(rep.clamp_limit(10**9), rep.MAX_LIMIT)
        self.assertEqual(rep.clamp_limit("abc"), 100)
        self.assertEqual(rep.clamp_limit(None), 100)


class TestMoneyMath(unittest.TestCase):
    def test_decimal_str_precision(self):
        self.assertEqual(rep.decimal_str("100"), "100.00000000")
        self.assertEqual(rep.decimal_str(Decimal("1.5")), "1.50000000")
        self.assertEqual(rep.decimal_str(None), "0.00000000")
        self.assertEqual(rep.decimal_str("garbage"), "0.00000000")
        # No float artifacts: 0.1 + 0.2 stays exact via Decimal path
        self.assertEqual(rep.decimal_str("0.30000000"), "0.30000000")

    def test_safe_div(self):
        self.assertAlmostEqual(rep.safe_div(1, 2), 0.5)
        self.assertEqual(rep.safe_div(5, 0), 0.0)
        self.assertEqual(rep.safe_div(5, None), 0.0)
        self.assertEqual(rep.safe_div(None, 4), 0.0)


class TestQueryBuilders(unittest.TestCase):
    def test_no_value_interpolation(self):
        """All values must go through placeholders, never f-string concat."""
        builders = [
            rep.build_daily_summary_query(),
            rep.build_funnel_query(),
            rep.build_top_affiliates_query(),
            rep.build_ngr_trend_query(),
            rep.build_payout_aging_query(),
            rep.build_ltv_query(),
        ]
        for query, params in builders:
            self.assertIn("%(", query)
            self.assertNotIn("{", query.replace("%(", "").replace(")s", ""))
            for param in params:
                self.assertIn(f"%({param})s", query)

    def test_export_allowlist(self):
        query, _ = rep.build_export_query("affiliate_daily_aggregates")
        self.assertIn("affiliate_daily_aggregates", query)
        for bad in ("users", "kyc_documents", "affiliate_daily_aggregates; DROP TABLE x", ""):
            with self.assertRaises(ValueError, msg=bad):
                rep.build_export_query(bad)

    def test_earnings_report_filters(self):
        # No filters: base predicates only, placeholders declared.
        query, params = rep.build_earnings_report_query()
        self.assertIn("period_start >=", query)
        self.assertNotIn("%(status)s", query)
        self.assertNotIn("%(affiliate_id)s", query)
        self.assertEqual(params, ["date_from", "date_to", "limit", "offset"])
        # With filters: parameterized predicates, still no interpolation.
        query, params = rep.build_earnings_report_query(status="paid", affiliate_id="aff-1")
        self.assertIn("status = %(status)s", query)
        self.assertIn("affiliate_id = %(affiliate_id)s", query)
        self.assertEqual(
            params,
            ["date_from", "date_to", "status", "affiliate_id", "limit", "offset"],
        )

    def test_earnings_report_bad_status(self):
        for bad in ("hacked", "PAID", "", "paid OR 1=1"):
            with self.assertRaises(ValueError, msg=bad):
                rep.build_earnings_report_query(status=bad)

    def test_range_queries_use_between(self):
        for builder in (rep.build_platform_totals_query, rep.build_top_affiliates_range_query):
            query, params = builder()
            self.assertIn("BETWEEN %(date_from)s AND %(date_to)s", query)
            self.assertIn("date_from", params)
            self.assertIn("date_to", params)


class TestRowMapping(unittest.TestCase):
    def test_summary_row(self):
        mapped = rep.map_summary_row(
            {
                "report_date": date(2026, 9, 30),
                "clicks": 10,
                "registrations": 4,
                "ftd_count": 2,
                "active_players": 3,
                "ggr_amount": "120.5",
                "ngr_amount": "80.25",
                "commission_accrued": "16.05",
                "commission_released": "10",
                "commission_reversed": None,
            }
        )
        self.assertEqual(mapped["report_date"], "2026-09-30")
        self.assertEqual(mapped["clicks"], 10)
        self.assertEqual(mapped["ngr_amount"], "80.25000000")
        self.assertEqual(mapped["commission_reversed"], "0.00000000")

    def test_funnel_rates(self):
        mapped = rep.map_funnel_row(
            {"clicks": 1000, "registrations": 100, "ftd_count": 20, "active_players": 15}
        )
        self.assertAlmostEqual(mapped["click_to_reg_rate"], 0.1)
        self.assertAlmostEqual(mapped["reg_to_ftd_rate"], 0.2)
        self.assertAlmostEqual(mapped["ftd_to_active_rate"], 0.75)

    def test_funnel_zero_denominator(self):
        mapped = rep.map_funnel_row(
            {"clicks": 0, "registrations": 0, "ftd_count": 0, "active_players": 0}
        )
        self.assertEqual(mapped["click_to_reg_rate"], 0.0)
        self.assertEqual(mapped["reg_to_ftd_rate"], 0.0)

    def test_top_affiliates_row(self):
        mapped = rep.map_top_affiliates_row(
            {
                "affiliate_id": "aff-1",
                "total_ngr": "999.99",
                "total_ggr": "1200",
                "total_commission": "200",
                "total_ftds": 7,
            }
        )
        self.assertEqual(mapped["affiliate_id"], "aff-1")
        self.assertEqual(mapped["total_ngr"], "999.99000000")
        self.assertEqual(mapped["total_ftds"], 7)

    def test_earnings_grouping(self):
        grouped = rep.summarize_earnings_by_status(
            [
                {"status": "pending", "commission_amount": "10.00"},
                {"status": "available", "commission_amount": "5.50"},
                {"status": "pending", "commission_amount": "2.50"},
                {"status": "paid", "commission_amount": None},
                {"status": "pending", "commission_amount": "junk"},
            ]
        )
        self.assertEqual(grouped["pending"], "12.50000000")
        self.assertEqual(grouped["available"], "5.50000000")
        self.assertEqual(grouped["paid"], "0.00000000")

    def test_earning_row(self):
        mapped = rep.map_earning_row(
            {
                "earning_id": "e-1",
                "affiliate_id": "aff-1",
                "referred_user_id": 42,
                "source_type": "casino",
                "period_start": "2026-09-01 00:00:00",
                "period_end": "2026-09-30 23:59:59",
                "ggr_amount": "1000",
                "ngr_amount": "800.5",
                "commission_rate": "0.2",
                "commission_amount": "160.1",
                "status": "pending",
                "hold_until": "2026-10-14 00:00:00",
                "created_at": "2026-10-01 00:00:00",
            }
        )
        self.assertEqual(mapped["earning_id"], "e-1")
        self.assertEqual(mapped["referred_user_id"], 42)
        self.assertEqual(mapped["ngr_amount"], "800.50000000")
        self.assertEqual(mapped["commission_amount"], "160.10000000")
        self.assertEqual(mapped["status"], "pending")

    def test_totals_row(self):
        mapped = rep.map_totals_row(
            {
                "clicks": 5000,
                "registrations": 500,
                "ftd_count": 100,
                "active_players": 80,
                "ggr_amount": "10000",
                "ngr_amount": "7000.25",
                "commission_accrued": "1400.05",
                "commission_released": "1000",
                "commission_reversed": "50.5",
                "active_affiliates": 12,
            }
        )
        self.assertNotIn("report_date", mapped)
        self.assertEqual(mapped["clicks"], 5000)
        self.assertEqual(mapped["ngr_amount"], "7000.25000000")
        self.assertEqual(mapped["active_affiliates"], 12)

    def test_clamp_offset(self):
        self.assertEqual(rep.clamp_offset(0), 0)
        self.assertEqual(rep.clamp_offset(50), 50)
        self.assertEqual(rep.clamp_offset(-5), 0)
        self.assertEqual(rep.clamp_offset(10**9), rep.MAX_OFFSET)
        self.assertEqual(rep.clamp_offset("abc"), 0)


if __name__ == "__main__":
    unittest.main()
