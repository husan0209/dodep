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


def test_report_and_dashboard_routes_validate_their_range() -> None:
    """The report/dashboard routes are wired up and enforce their date range.

    These used to be stubs that ignored their arguments. They now build a real
    ClickHouse query behind a mandatory inclusive date range, so a call without
    a range is rejected by FastAPI's own validation and a reversed or
    malformed range is rejected by the handler. Both cases are decided before
    any database access, so this needs no ClickHouse.
    """
    client = TestClient(app)

    # Missing the required query parameters entirely.
    assert client.get("/api/v1/analytics/reports").status_code == 422
    assert client.get("/api/v1/analytics/dashboard").status_code == 422

    # date_from must be ISO `YYYY-MM-DD` (the route's own pattern).
    malformed = client.get(
        "/api/v1/analytics/reports",
        params={"date_from": "03-02-2026", "date_to": "2026-02-03"},
    )
    assert malformed.status_code == 422

    # A well-formed but reversed range reaches the handler, which refuses it.
    reversed_range = {"date_from": "2026-02-10", "date_to": "2026-02-01"}
    reports = client.get("/api/v1/analytics/reports", params=reversed_range)
    assert reports.status_code == 422
    assert "date_from" in reports.json()["detail"]
    dashboard = client.get("/api/v1/analytics/dashboard", params=reversed_range)
    assert dashboard.status_code == 422
    assert "date_from" in dashboard.json()["detail"]


def test_export_routes_are_wired_up() -> None:
    """Both export routes answer: the POST stub, and the GET CSV endpoint."""
    client = TestClient(app)

    # POST /export is still the placeholder from before.
    export = client.post("/api/v1/analytics/export")
    assert export.status_code == 200
    assert export.json() == {"status": "not_implemented"}

    # GET /export is the real one and needs a table plus a range.
    assert client.get("/api/v1/analytics/export").status_code == 422


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
