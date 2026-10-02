"""Shared fixtures: a fake ClickHouse driver and TestClients.

The fixtures drive the real ASGI app object (not a stripped-down copy) so
the exception handlers under test are the ones that actually run.
"""

from __future__ import annotations

from typing import Any

import pytest
from fastapi.testclient import TestClient

from analytics import clickhouse as clickhouse_module
from analytics import main as main_module
from analytics.clickhouse import ClickHouseClient, ClickHouseUnavailableError


class FakeQueryResult:
    """Mirrors the parts of QueryResult the client uses."""

    def __init__(self, columns: list[str], rows: list[tuple[Any, ...]]):
        self.column_names = [type("C", (), {"name": name}) for name in columns]
        self.result_rows = rows


class FakeDriver:
    """Stands in for clickhouse_connect.Client at the driver level."""

    def __init__(
        self, results: list[FakeQueryResult] | None = None, raises: Exception | None = None
    ):
        self.results = list(results or [])
        self.raises = raises
        self.calls: list[tuple[str, dict[str, Any]]] = []
        self.closed = False

    def query(self, query, parameters=None):
        self.calls.append((query, dict(parameters or {})))
        if self.raises is not None:
            raise self.raises
        if self.results:
            return self.results.pop(0)
        return FakeQueryResult([], [])

    def close(self):
        self.closed = True


def _stub_client(monkeypatch, driver: FakeDriver) -> ClickHouseClient:
    """Wire a ClickHouseClient to the fake driver for the whole app."""
    client = ClickHouseClient(host="localhost", port=9000, database="test")
    monkeypatch.setattr(client, "_connect", lambda: driver)
    monkeypatch.setattr(clickhouse_module, "_cached_client", client)
    monkeypatch.setattr(main_module, "get_client", lambda _settings: client)
    return client


@pytest.fixture
def fake_driver() -> FakeDriver:
    return FakeDriver()


@pytest.fixture
def client_with_driver(monkeypatch, fake_driver) -> ClickHouseClient:
    """Client wired to the fake driver (bypasses connect())."""
    return _stub_client(monkeypatch, fake_driver)


@pytest.fixture
def unavailable_client(monkeypatch) -> ClickHouseClient:
    """Client whose connect always fails, as if ClickHouse were down."""
    client = ClickHouseClient(host="localhost", port=9000, database="test")

    def boom():
        raise ClickHouseUnavailableError("cannot connect")

    monkeypatch.setattr(client, "_connect", boom)
    return client


@pytest.fixture
def api(monkeypatch, fake_driver) -> TestClient:
    """TestClient over the production app with ClickHouse stubbed out."""
    _stub_client(monkeypatch, fake_driver)
    # TestClient(app) without a `with` block skips the lifespan (and thus the
    # startup ping), which is exactly what we want: no socket, no probe.
    return TestClient(main_module.app)


@pytest.fixture
def broken_api(monkeypatch) -> TestClient:
    """TestClient where ClickHouse is down: endpoints must answer 503."""
    driver = FakeDriver(raises=ConnectionError("connection refused"))
    _stub_client(monkeypatch, driver)
    return TestClient(main_module.app)
