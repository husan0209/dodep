"""Smoke tests for the Analytics Service HTTP surface."""

from fastapi.testclient import TestClient

from src.analytics.main import app, settings


def test_health_endpoint_reports_ok() -> None:
    """GET /health returns the liveness payload the platform probes."""
    client = TestClient(app)

    response = client.get("/health")

    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


def test_ready_endpoint_reports_ready() -> None:
    """GET /ready returns the readiness payload."""
    client = TestClient(app)

    response = client.get("/ready")

    assert response.status_code == 200
    assert response.json() == {"status": "ready"}


def test_placeholder_endpoints_are_wired_up() -> None:
    """The report/dashboard/export routes resolve, even while still stubs."""
    client = TestClient(app)

    reports = client.get("/api/v1/analytics/reports")
    dashboard = client.get("/api/v1/analytics/dashboard")
    export = client.post("/api/v1/analytics/export")

    assert reports.status_code == 200
    assert reports.json() == {"status": "not_implemented"}
    assert dashboard.status_code == 200
    assert dashboard.json() == {"status": "not_implemented"}
    assert export.status_code == 200
    assert export.json() == {"status": "not_implemented"}


def test_lifespan_starts_and_stops_cleanly() -> None:
    """Entering the TestClient context runs the lifespan handler both ways."""
    with TestClient(app) as client:
        response = client.get("/health")

    assert response.status_code == 200


def test_settings_expose_documented_defaults() -> None:
    """Settings fall back to localhost defaults when no env overrides them."""
    assert settings.app_name == "Analytics Service"
    assert settings.clickhouse_host == "localhost"
    assert settings.clickhouse_port == 8123
    assert settings.redis_host == "localhost"
    assert settings.redis_port == 6379