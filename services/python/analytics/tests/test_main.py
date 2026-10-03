"""Tests for the Analytics service HTTP surface."""
import pytest
from fastapi.testclient import TestClient

from analytics.main import app, settings


@pytest.fixture
def client():
    """Return a TestClient bound to the Analytics ASGI app."""
    return TestClient(app)


def test_health(client):
    """Health endpoint reports ok."""
    response = client.get("/health")

    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


def test_ready(client):
    """Readiness endpoint reports ready."""
    response = client.get("/ready")

    assert response.status_code == 200
    assert response.json() == {"status": "ready"}


def test_reports_endpoint(client):
    """Reports endpoint is mounted under /api/v1/analytics."""
    response = client.get("/api/v1/analytics/reports")

    assert response.status_code == 200
    assert response.json() == {"status": "not_implemented"}


def test_dashboard_endpoint(client):
    """Dashboard endpoint is mounted under /api/v1/analytics."""
    response = client.get("/api/v1/analytics/dashboard")

    assert response.status_code == 200
    assert response.json() == {"status": "not_implemented"}


def test_export_endpoint(client):
    """Export endpoint is mounted under /api/v1/analytics."""
    response = client.post("/api/v1/analytics/export")

    assert response.status_code == 200
    assert response.json() == {"status": "not_implemented"}


def test_app_metadata():
    """App metadata is derived from Settings."""
    assert app.title == settings.app_name
    assert app.version == "0.1.0"


def test_unknown_route_returns_404(client):
    """Unregistered analytics subpaths are not swallowed."""
    response = client.get("/api/v1/analytics/does-not-exist")

    assert response.status_code == 404
