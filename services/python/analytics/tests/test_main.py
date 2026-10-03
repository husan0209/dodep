"""Smoke tests for the Analytics Service HTTP surface.

CI runs `pytest -v --cov=src` in this directory; with no test files at all
pytest exits with code 5 ("no tests ran"), which failed the job.
"""

from fastapi.testclient import TestClient

# Imported through the installed distribution name, not as `src.analytics`:
# CI runs the `pytest` console script, which does not put the working directory
# on sys.path the way `python -m pytest` does.
from analytics.main import app, settings

client = TestClient(app)


def test_health_reports_ok() -> None:
    response = client.get("/health")

    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


def test_ready_reports_ready() -> None:
    response = client.get("/ready")

    assert response.status_code == 200
    assert response.json() == {"status": "ready"}


def test_reports_endpoint_is_served() -> None:
    response = client.get("/api/v1/analytics/reports")

    assert response.status_code == 200
    assert response.status_code != 404


def test_app_title_comes_from_settings() -> None:
    assert app.title == settings.app_name
