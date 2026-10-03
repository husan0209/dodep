"""Tests for the analytics package metadata."""

import analytics


def test_version_is_exposed():
    """The package advertises its version."""
    assert analytics.__version__ == "0.1.0"


def test_version_matches_app():
    """Package version and FastAPI app version must not drift apart."""
    from analytics.main import app

    assert app.version == analytics.__version__
