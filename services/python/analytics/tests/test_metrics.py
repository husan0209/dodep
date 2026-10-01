"""Unit tests for the stdlib metrics module.

Run: ``py -m unittest discover -s tests -v`` from the service directory.
"""

import os
import sys
import threading
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "src"))

from analytics.metrics import MetricsRegistry, Timer, normalize_route


class TestNormalizeRoute(unittest.TestCase):
    def test_static_path(self):
        self.assertEqual(normalize_route("/health"), "/health")
        self.assertEqual(
            normalize_route("/api/v1/analytics/affiliates/top"),
            "/api/v1/analytics/affiliates/top",
        )

    def test_uuid_collapsed(self):
        self.assertEqual(
            normalize_route("/api/v1/analytics/affiliate/9b2f4c1a-3d4e-4f5a-8b6c-7d8e9f0a1b2c/summary"),
            "/api/v1/analytics/affiliate/:param/summary",
        )

    def test_numeric_and_date_collapsed(self):
        self.assertEqual(normalize_route("/reports/12345"), "/reports/:param")
        self.assertEqual(normalize_route("/daily/2026-09-30"), "/daily/:param")

    def test_query_dropped_and_slugs_kept(self):
        self.assertEqual(
            normalize_route("/api/v1/analytics/payouts/aging?limit=10"),
            "/api/v1/analytics/payouts/aging",
        )
        self.assertEqual(normalize_route("/r/AFF-SPORTS-01"), "/r/AFF-SPORTS-01")

    def test_root(self):
        self.assertEqual(normalize_route("/"), "/")


class TestRegistry(unittest.TestCase):
    def test_counter_and_render(self):
        reg = MetricsRegistry()
        reg.observe_request("GET", "/health", 200, 0.003)
        reg.observe_request("GET", "/health", 200, 0.007)
        reg.observe_request("GET", "/health", 503, 0.5)
        text = reg.render()
        self.assertIn(
            'analytics_http_requests_total{method="GET",route="/health",status="200"} 2', text
        )
        self.assertIn(
            'analytics_http_requests_total{method="GET",route="/health",status="503"} 1', text
        )
        # Cumulative buckets: both fast requests <= 0.01, one <= 0.005.
        self.assertIn(
            'analytics_http_request_duration_seconds_bucket'
            '{method="GET",route="/health",le="0.005"} 1',
            text,
        )
        self.assertIn(
            'analytics_http_request_duration_seconds_bucket'
            '{method="GET",route="/health",le="+Inf"} 3',
            text,
        )
        self.assertIn(
            'analytics_http_request_duration_seconds_count{method="GET",route="/health"} 3',
            text,
        )

    def test_in_progress_gauge(self):
        reg = MetricsRegistry()
        reg.track_in_progress(1)
        reg.track_in_progress(1)
        reg.track_in_progress(-1)
        self.assertIn("analytics_http_requests_in_progress 1", reg.render())
        reg.track_in_progress(-5)  # never negative
        self.assertIn("analytics_http_requests_in_progress 0", reg.render())

    def test_thread_safety(self):
        reg = MetricsRegistry()
        threads = [
            threading.Thread(
                target=lambda: [
                    reg.observe_request("GET", "/health", 200, 0.01) for _ in range(100)
                ]
            )
            for _ in range(10)
        ]
        for t in threads:
            t.start()
        for t in threads:
            t.join()
        self.assertIn(
            'analytics_http_requests_total{method="GET",route="/health",status="200"} 1000',
            reg.render(),
        )

    def test_timer(self):
        ticks = iter([10.0, 10.25])
        with Timer(clock=lambda: next(ticks)) as timer:
            pass
        self.assertAlmostEqual(timer.elapsed, 0.25)


if __name__ == "__main__":
    unittest.main()
