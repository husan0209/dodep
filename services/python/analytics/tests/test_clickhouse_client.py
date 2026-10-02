"""Tests for the ClickHouse access layer."""

from __future__ import annotations

import pytest

from analytics import clickhouse as clickhouse_module
from analytics.clickhouse import (
    ClickHouseClient,
    ClickHouseUnavailableError,
    build_client,
    get_client,
    reset_client,
)
from tests.conftest import FakeDriver, FakeQueryResult


class TestFetchAll:
    """Rows come back as dicts keyed by column name."""

    def test_returns_dicts(self, client_with_driver, fake_driver):
        fake_driver.results = [FakeQueryResult(["a", "b"], [(1, 2), (3, 4)])]
        rows = client_with_driver.fetch_all("SELECT a, b FROM t", {"x": 1})

        assert rows == [{"a": 1, "b": 2}, {"a": 3, "b": 4}]
        query, params = fake_driver.calls[0]
        assert params == {"x": 1}

    def test_empty_result_is_empty_list(self, client_with_driver):
        fake_driver_results = FakeQueryResult(["a"], [])
        client_with_driver._connect().results = [fake_driver_results]
        assert client_with_driver.fetch_all("SELECT a FROM t") == []

    def test_parameters_default_to_empty(self, client_with_driver, fake_driver):
        client_with_driver.fetch_all("SELECT 1")
        assert fake_driver.calls[0][1] == {}

    def test_query_failure_becomes_unavailable(self, client_with_driver, fake_driver):
        fake_driver.raises = RuntimeError("table missing")
        with pytest.raises(ClickHouseUnavailableError, match="query failed"):
            client_with_driver.fetch_all("SELECT * FROM nope")


class TestConnectionHandling:
    """Connection problems surface as ClickHouseUnavailableError, not driver errors."""

    def test_connect_failure_translated(self, monkeypatch):
        client = ClickHouseClient(host="localhost", port=9000, database="test")

        def boom(**kwargs):
            raise OSError("connection refused")

        monkeypatch.setattr(clickhouse_module.clickhouse_connect, "get_client", boom)

        with pytest.raises(ClickHouseUnavailableError, match="cannot connect"):
            client._connect()

    def test_connection_is_cached(self, monkeypatch):
        client = ClickHouseClient(host="localhost", port=9000, database="test")
        driver = FakeDriver()
        monkeypatch.setattr(
            clickhouse_module.clickhouse_connect, "get_client", lambda **kwargs: driver
        )

        assert client._connect() is client._connect()

    def test_ping_false_when_unavailable(self, unavailable_client):
        assert unavailable_client.ping() is False

    def test_ping_true_on_success(self, client_with_driver):
        assert client_with_driver.ping() is True

    def test_ping_false_on_query_error(self, client_with_driver, fake_driver):
        fake_driver.raises = RuntimeError("boom")
        assert client_with_driver.ping() is False

    def test_close_is_safe_without_connection(self):
        client = ClickHouseClient(host="localhost", port=9000, database="test")
        client.close()  # must not raise

    def test_close_releases_driver(self, monkeypatch):
        client = ClickHouseClient(host="localhost", port=9000, database="test")
        driver = FakeDriver()
        monkeypatch.setattr(
            clickhouse_module.clickhouse_connect, "get_client", lambda **kwargs: driver
        )
        client._connect()
        client.close()

        assert driver.closed is True
        assert client._client is None


class TestClientRegistry:
    """The module-level client is cached and resettable."""

    def test_get_client_caches(self):
        class FakeSettings:
            clickhouse_host = "h"
            clickhouse_port = 1
            clickhouse_database = "d"
            clickhouse_user = "u"
            clickhouse_password = ""
            clickhouse_query_timeout = 5

        reset_client()
        first = get_client(FakeSettings())
        second = get_client(FakeSettings())
        assert first is second
        reset_client()

    def test_reset_clears_registry(self):
        class FakeSettings:
            clickhouse_host = "h"
            clickhouse_port = 1
            clickhouse_database = "d"
            clickhouse_user = "u"
            clickhouse_password = ""
            clickhouse_query_timeout = 5

        reset_client()
        get_client(FakeSettings())
        reset_client()
        assert clickhouse_module._cached_client is None

    def test_build_client_maps_settings(self):
        class FakeSettings:
            clickhouse_host = "ch"
            clickhouse_port = 9123
            clickhouse_database = "casino"
            clickhouse_user = "reader"
            clickhouse_password = "secret"
            clickhouse_query_timeout = 42

        client = build_client(FakeSettings())
        assert client.host == "ch"
        assert client.port == 9123
        assert client.database == "casino"
        assert client.query_timeout == 42
