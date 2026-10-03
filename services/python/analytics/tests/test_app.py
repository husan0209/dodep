"""Smoke tests for the Analytics Service application.

The service had no test files at all, so `pytest -v --cov=src` exited 5 ("no
tests ran") and failed the python-build job before collecting anything. These
tests pin the contract that is cheap to break and cheap to serve: the app
imports, the health and readiness probes answer, and the placeholder endpoints
are wired to the documented paths.
"""

from fastapi.testclient import TestClient

from analytics.main import app

client = TestClient(app)


def test_health_reports_ok():
    response = client.get("/health")

    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


def test_ready_reports_ready():
    response = client.get("/ready")

    assert response.status_code == 200
    assert response.json() == {"status": "ready"}


def test_placeholder_endpoints_are_reachable():
    """The analytics endpoints exist and answer until they are implemented."""
    for method, path in (
        ("GET", "/api/v1/analytics/reports"),
        ("GET", "/api/v1/analytics/dashboard"),
        ("POST", "/api/v1/analytics/export"),
    ):
        response = client.request(method, path)

        assert response.status_code == 200, f"{method} {path} returned {response.status_code}"
        assert response.json() == {"status": "not_implemented"}