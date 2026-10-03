"""Smoke tests for Analytics Service."""

from fastapi.testclient import TestClient

from src.analytics.main import app

client = TestClient(app)


def test_health():
    """Health endpoint returns ok."""
    resp = client.get("/health")
    assert resp.status_code == 200
    assert resp.json() == {"status": "ok"}


def test_ready():
    """Readiness endpoint returns ready."""
    resp = client.get("/ready")
    assert resp.status_code == 200
    assert resp.json() == {"status": "ready"}
