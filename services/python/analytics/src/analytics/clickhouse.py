"""Thin ClickHouse client wrapper.

``clickhouse-connect`` is imported lazily so pure-logic modules and unit
tests never require a live database or the driver package.
Connection failures surface as :class:`ClickHouseUnavailableError` and
are mapped to HTTP 503 by the API layer (never fake data, never 500).
"""

from __future__ import annotations

from typing import Any


class ClickHouseUnavailableError(RuntimeError):
    """Raised when ClickHouse cannot be reached or queried."""


class ClickHouseClient:
    """Minimal wrapper around a clickhouse-connect client."""

    def __init__(
        self,
        host: str,
        port: int,
        username: str = "default",
        password: str = "",
    ) -> None:
        try:
            import clickhouse_connect
        except ImportError as exc:
            raise ClickHouseUnavailableError(
                "clickhouse-connect is not installed; cannot query analytics storage"
            ) from exc
        try:
            self._client = clickhouse_connect.get_client(
                host=host, port=port, username=username, password=password
            )
        except Exception as exc:
            raise ClickHouseUnavailableError(
                f"cannot connect to ClickHouse at {host}:{port}: {exc}"
            ) from exc

    def fetch_all(self, query: str, parameters: dict[str, Any]) -> list[dict[str, Any]]:
        """Run a parameterized query and return rows as dicts."""
        try:
            result = self._client.query(query, parameters=parameters)
        except Exception as exc:
            raise ClickHouseUnavailableError(f"ClickHouse query failed: {exc}") from exc
        columns = list(result.column_names)
        # strict=True: a row whose width differs from the header means the
        # driver and query disagree. Silent zip truncation would hand the
        # caller a dict with missing keys and a KeyError far from the cause.
        return [dict(zip(columns, row, strict=True)) for row in result.result_rows]


_client: ClickHouseClient | None = None


def get_client(
    host: str,
    port: int,
    username: str = "default",
    password: str = "",
) -> ClickHouseClient:
    """Process-wide singleton client (FastAPI lifespan creates it eagerly)."""
    global _client
    if _client is None:
        _client = ClickHouseClient(host=host, port=port, username=username, password=password)
    return _client


def reset_client() -> None:
    """Drop the singleton (tests and lifespan shutdown)."""
    global _client
    _client = None
