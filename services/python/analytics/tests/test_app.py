"""Tests for the analytics service application wiring and endpoints."""

from analytics.main import (
    app,
    export_data,
    get_dashboard,
    get_reports,
    health,
    lifespan,
    ready,
    settings,
)


class TestSettings:
    """Application settings defaults."""

    def test_defaults(self):
        """Sensible local defaults are used when no env vars are set."""
        assert settings.app_name == "Analytics Service"
        assert settings.debug is False
        assert settings.clickhouse_host == "localhost"
        assert settings.clickhouse_port == 8123
        assert settings.redis_host == "localhost"
        assert settings.redis_port == 6379

    def test_ports_are_positive(self):
        """Ports must be usable as TCP ports."""
        assert 0 < settings.clickhouse_port < 65536
        assert 0 < settings.redis_port < 65536


class TestApp:
    """FastAPI application wiring."""

    def test_metadata(self):
        """App exposes the expected title, description and version."""
        assert app.title == settings.app_name
        assert app.version == "0.1.0"
        assert app.description

    def test_lifespan_is_configured(self):
        """The lifespan context manager is attached to the app."""
        assert app.router.lifespan_context is not None

    def test_expected_routes_registered(self):
        """Every documented endpoint is reachable."""
        routes = {
            (route.path, method)
            for route in app.routes
            for method in getattr(route, "methods", set())
        }
        expected = {
            ("/health", "GET"),
            ("/ready", "GET"),
            ("/api/v1/analytics/reports", "GET"),
            ("/api/v1/analytics/dashboard", "GET"),
            ("/api/v1/analytics/export", "POST"),
        }
        assert expected <= routes


class TestHealthEndpoints:
    """Liveness/readiness probes used by the container healthcheck."""

    async def test_health(self):
        """Health check reports healthy."""
        assert await health() == {"status": "ok"}

    async def test_ready(self):
        """Readiness check reports ready."""
        assert await ready() == {"status": "ready"}


class TestPlaceholderEndpoints:
    """Analytics endpoints are still stubs."""

    async def test_reports(self):
        """Reports endpoint is a stub."""
        assert await get_reports() == {"status": "not_implemented"}

    async def test_dashboard(self):
        """Dashboard endpoint is a stub."""
        assert await get_dashboard() == {"status": "not_implemented"}

    async def test_export(self):
        """Export endpoint is a stub."""
        assert await export_data() == {"status": "not_implemented"}


class TestLifespan:
    """Startup/shutdown must not raise."""

    async def test_startup_and_shutdown(self):
        """Entering and exiting the lifespan completes cleanly."""
        async with lifespan(app):
            pass
