"""ClickHouse access layer.

Design notes:
- The client is created lazily and cached, so importing the module (or the
  FastAPI app) never opens a socket: that keeps unit tests and CLI tooling
  working without a database.
- Every query is executed through parameter binding (`%(name)s`) — values
  coming from HTTP query parameters never reach the SQL text.
- Connection failures are translated into `ClickHouseUnavailableError` so the API
  can answer 503 instead of leaking a driver-specific traceback.
"""

from __future__ import annotations

from typing import Any

import clickhouse_connect
import structlog
from clickhouse_connect.driver.client import Client

logger = structlog.get_logger()

# Process-wide client cache. Named distinctly from ClickHouseClient._client
# (the per-instance driver handle) to avoid shadowing confusion.
_cached_client: ClickHouseClient | None = None


class ClickHouseUnavailableError(RuntimeError):
    """Raised when ClickHouse cannot be reached or a query fails."""


class ClickHouseClient:
    """Thin wrapper around clickhouse-connect with dict-based fetching."""

    def __init__(
        self,
        host: str,
        port: int,
        database: str,
        user: str = "default",
        password: str = "",  # nosec B107 - injected via env; empty is valid locally
        query_timeout: int = 30,
    ) -> None:
        self.host = host
        self.port = port
        self.database = database
        self.user = user
        self.password = password
        self.query_timeout = query_timeout
        self._client: Client | None = None

    # ------------------------------------------------------------------
    # Connection handling
    # ------------------------------------------------------------------
    def _connect(self) -> Client:
        if self._client is not None:
            return self._client
        try:
            self._client = clickhouse_connect.get_client(
                host=self.host,
                port=self.port,
                database=self.database,
                user=self.user,
                password=self.password,
                settings={
                    # Analytics queries scan aggregates; bound the time so a
                    # pathological range cannot pin a runner forever.
                    "max_execution_time": self.query_timeout,
                    "max_bytes_before_external_group_by": 10_000_000_000,
                },
            )
        except Exception as exc:  # driver raises a family of exceptions
            logger.error("clickhouse.connect_failed", host=self.host, error=str(exc))
            raise ClickHouseUnavailableError(
                f"cannot connect to ClickHouse at {self.host}:{self.port}"
            ) from exc
        return self._client

    def ping(self) -> bool:
        """True when the server answers a trivial query."""
        try:
            self._connect().query("SELECT 1")
            return True
        except ClickHouseUnavailableError:
            return False
        except Exception as exc:
            logger.warning("clickhouse.ping_failed", error=str(exc))
            return False

    def close(self) -> None:
        if self._client is not None:
            try:
                self._client.close()
            except Exception as exc:  # pragma: no cover - best effort
                logger.warning("clickhouse.close_failed", error=str(exc))
            finally:
                self._client = None

    # ------------------------------------------------------------------
    # Queries
    # ------------------------------------------------------------------
    def fetch_all(self, query: str, params: dict[str, Any] | None = None) -> list[dict[str, Any]]:
        """
        Run a query and return rows as dicts.

        Args:
            query: SQL with %(name)s placeholders (no value interpolation).
            params: bound parameters; `date_from`/`date_to` style keys.

        Returns:
            List of row dicts keyed by column name. Empty list for no matches.
        """
        client = self._connect()
        try:
            result = client.query(query, parameters=params or {})
        except Exception as exc:
            logger.error(
                "clickhouse.query_failed",
                error=str(exc),
                query_head=" ".join(query.split())[:200],
            )
            raise ClickHouseUnavailableError(f"query failed: {exc}") from exc

        columns = [column.name for column in result.column_names]
        rows = result.result_rows
        if not rows:
            return []
        return [dict(zip(columns, row, strict=True)) for row in rows]


def build_client(settings: Any) -> ClickHouseClient:
    """Create a client from application settings."""
    return ClickHouseClient(
        host=settings.clickhouse_host,
        port=settings.clickhouse_port,
        database=settings.clickhouse_database,
        user=settings.clickhouse_user,
        password=settings.clickhouse_password,
        query_timeout=settings.clickhouse_query_timeout,
    )


def get_client(settings: Any) -> ClickHouseClient:
    """Return the process-wide cached client (created on first use)."""
    global _cached_client
    if _cached_client is None:
        _cached_client = build_client(settings)
    return _cached_client


def reset_client() -> None:
    """Drop the cached client (used by tests and on shutdown)."""
    global _cached_client
    if _cached_client is not None:
        _cached_client.close()
    _cached_client = None
