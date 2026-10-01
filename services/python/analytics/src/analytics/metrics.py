"""Minimal Prometheus exposition for the Analytics Service.

Stdlib only (threading/time) — no prometheus_client dependency, so this
module is fully unit-testable without third-party packages.

Exposes:
- ``analytics_http_requests_total{method,route,status}`` (counter)
- ``analytics_http_request_duration_seconds`` (histogram, seconds)
- ``analytics_http_requests_in_progress`` (gauge)

Cardinality guard: dynamic path segments (UUIDs, ids, dates) are
collapsed to ``:param`` placeholders by :func:`normalize_route`.
"""

from __future__ import annotations

import re
import threading
import time
from collections.abc import Callable
from typing import Self

# Default histogram buckets (seconds), aligned with architecture-overview.
BUCKETS = (0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0)

_DATE_RE = re.compile(r"^\d{4}-\d{2}-\d{2}$")
_UUID_RE = re.compile(
    r"^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-" r"[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$"
)


def normalize_route(path: str) -> str:
    """Collapse dynamic segments to bound label cardinality.

    - UUID / pure numbers / ISO dates -> ``:param``
    - query strings are dropped
    """
    clean = path.split("?", 1)[0].rstrip("/") or "/"
    parts: list[str] = []
    for segment in clean.split("/"):
        if not segment:
            continue
        if segment.isdigit() or _UUID_RE.match(segment) or _DATE_RE.match(segment):
            parts.append(":param")
        else:
            parts.append(segment)
    return "/" + "/".join(parts)


def _labels_str(labels: dict[str, str]) -> str:
    # Insertion order is preserved (callers put `le` last for histograms,
    # matching canonical Prometheus client output). Dicts are ordered,
    # so rendering stays deterministic.
    return "{" + ",".join(f'{k}="{v}"' for k, v in labels.items()) + "}"


class MetricsRegistry:
    """Thread-safe in-memory metrics store with text exposition."""

    def __init__(self, buckets: tuple[float, ...] = BUCKETS) -> None:
        self._buckets = tuple(sorted(buckets))
        self._lock = threading.Lock()
        self._counters: dict[tuple[str, str, str], int] = {}
        self._histograms: dict[tuple[str, str], list[int]] = {}
        self._hist_sums: dict[tuple[str, str], float] = {}
        self._hist_counts: dict[tuple[str, str], int] = {}
        self._in_progress = 0

    # -- tracking API ----------------------------------------------------

    def track_in_progress(self, delta: int) -> None:
        """Adjust the in-progress gauge (call +1 on start, -1 on finish)."""
        with self._lock:
            self._in_progress = max(0, self._in_progress + delta)

    def observe_request(self, method: str, route: str, status: int, duration_s: float) -> None:
        """Record one finished HTTP request."""
        key = (method.upper(), route, str(status))
        bucket_key = (method.upper(), route)
        bucket_idx = 0
        for i, bound in enumerate(self._buckets):
            if duration_s <= bound:
                bucket_idx = i
                break
        else:
            bucket_idx = len(self._buckets)
        with self._lock:
            self._counters[key] = self._counters.get(key, 0) + 1
            counts = self._histograms.setdefault(bucket_key, [0] * (len(self._buckets) + 1))
            counts[bucket_idx] += 1
            self._hist_sums[bucket_key] = self._hist_sums.get(bucket_key, 0.0) + duration_s
            self._hist_counts[bucket_key] = self._hist_counts.get(bucket_key, 0) + 1

    # -- exposition ------------------------------------------------------

    def render(self) -> str:
        """Render Prometheus text exposition format (version 0.0.4)."""
        lines: list[str] = []
        with self._lock:
            lines.append("# HELP analytics_http_requests_total Total HTTP requests.")
            lines.append("# TYPE analytics_http_requests_total counter")
            for (method, route, status), count in sorted(self._counters.items()):
                lines.append(
                    "analytics_http_requests_total"
                    + _labels_str({"method": method, "route": route, "status": status})
                    + f" {count}"
                )
            lines.append("# HELP analytics_http_requests_in_progress In-flight HTTP requests.")
            lines.append("# TYPE analytics_http_requests_in_progress gauge")
            lines.append(f"analytics_http_requests_in_progress {self._in_progress}")
            lines.append("# HELP analytics_http_request_duration_seconds HTTP request latency.")
            lines.append("# TYPE analytics_http_request_duration_seconds histogram")
            for (method, route), counts in sorted(self._histograms.items()):
                cumulative = 0
                for i, bound in enumerate(self._buckets):
                    cumulative += counts[i]
                    lines.append(
                        "analytics_http_request_duration_seconds_bucket"
                        + _labels_str(
                            {"method": method, "route": route, "le": _format_float(bound)}
                        )
                        + f" {cumulative}"
                    )
                cumulative += counts[len(self._buckets)]
                lines.append(
                    "analytics_http_request_duration_seconds_bucket"
                    + _labels_str({"method": method, "route": route, "le": "+Inf"})
                    + f" {cumulative}"
                )
                lines.append(
                    "analytics_http_request_duration_seconds_sum"
                    + _labels_str({"method": method, "route": route})
                    + f" {_format_float(self._hist_sums.get((method, route), 0.0))}"
                )
                lines.append(
                    "analytics_http_request_duration_seconds_count"
                    + _labels_str({"method": method, "route": route})
                    + f" {self._hist_counts.get((method, route), 0)}"
                )
        return "\n".join(lines) + "\n"


def _format_float(value: float) -> str:
    text = repr(float(value))
    if "." not in text and "e" not in text.lower():
        text += ".0"
    return text


# Process-wide registry used by the FastAPI middleware.
registry = MetricsRegistry()


class Timer:
    """Context manager measuring elapsed seconds (injectable clock for tests)."""

    def __init__(self, clock: Callable[[], float] | None = None) -> None:
        self._clock = clock or time.monotonic
        self.elapsed = 0.0

    def __enter__(self) -> Self:
        self._start = self._clock()
        return self

    def __exit__(self, *args: object) -> None:
        self.elapsed = self._clock() - self._start
