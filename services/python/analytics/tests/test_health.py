"""Smoke tests for the Analytics Service HTTP surface.

CI runs `pytest -v --cov=src`, and pytest exits with code 5 when no tests are
collected, which fails the job. These tests also pin the health/readiness
contract that the Kubernetes probes depend on.
"""
from fastapi.testclient import TestClient

from src.analytics.main import app

client = TestClient(app)


def test_health_reports_ok() -> None:
    """GET /health returns an ok status."""
    response = client.get("/health")

    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


def test_ready_reports_ready() -> None:
    """GET /ready returns a ready status."""
    response = client.get("/ready")

    assert response.status_code == 200
    assert response.json() == {"status": "ready"}


def test_service_metadata() -> None:
    """The app exposes the expected title and version."""
    assert app.title == "Analytics Service"
    assert app.version == "0.1.0"
