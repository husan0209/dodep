"""Prometheus instrumentation for the analytics API.

Only request-level metrics live here (counters/latency); business metrics
are derived from ClickHouse on demand, so they are not exported as gauges
that could silently go stale.
"""

from __future__ import annotations

import time

import structlog
from prometheus_client import CONTENT_TYPE_LATEST, Counter, Histogram, generate_latest
from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request
from starlette.responses import Response

logger = structlog.get_logger()

REQUESTS = Counter(
    "analytics_http_requests_total",
    "Total HTTP requests handled by the analytics API",
    ["method", "path", "status"],
)

LATENCY = Histogram(
    "analytics_http_request_duration_seconds",
    "Latency of analytics API requests",
    ["method", "path"],
)


class MetricsMiddleware(BaseHTTPMiddleware):
    """Record request counts and latency, labelled by route path."""

    async def dispatch(self, request: Request, call_next) -> Response:
        start = time.perf_counter()
        status = "500"
        try:
            response: Response = await call_next(request)
            status = str(response.status_code)
            return response
        finally:
            elapsed = time.perf_counter() - start
            # Label by the route template, never the raw path: ids and query
            # strings would explode metric cardinality.
            route = request.scope.get("route")
            path = getattr(route, "path", "unmatched")
            REQUESTS.labels(request.method, path, status).inc()
            LATENCY.labels(request.method, path).observe(elapsed)


def render_metrics() -> Response:
    """Prometheus exposition endpoint response."""
    body: bytes = generate_latest()
    return Response(content=body, media_type=CONTENT_TYPE_LATEST)
