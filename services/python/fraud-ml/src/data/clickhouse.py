"""
ClickHouse data access with Polars integration.
"""

import clickhouse_connect
import polars as pl
import structlog

logger = structlog.get_logger()


class ClickHouseClient:
    """ClickHouse client with Polars DataFrame support."""

    def __init__(
        self,
        host: str,
        port: int,
        database: str,
        user: str = "default",
        # Empty default is valid for a local dev ClickHouse; real credentials
        # come from CLICKHOUSE_PASSWORD in the environment.
        password: str = "",  # nosec B107
    ):
        self.client = clickhouse_connect.get_client(
            host=host,
            port=port,
            database=database,
            user=user,
            password=password,
            settings={
                "max_execution_time": 300,
                "max_bytes_before_external_group_by": 10000000000,
            },
        )
        logger.info(
            "clickhouse.connected",
            host=host,
            port=port,
            database=database,
        )

    def query_to_polars(self, query: str, params: dict | None = None) -> pl.DataFrame:
        """
        Execute query and return Polars DataFrame.

        Uses client.query_df() (Arrow-backed under the hood) and converts to
        Polars; QueryResult exposes no Arrow accessor in clickhouse-connect
        0.7.x, so this is the supported path.
        """
        try:
            df = pl.from_pandas(self.client.query_df(query, parameters=params))
            logger.debug(
                "clickhouse.query_executed",
                rows=df.height,
                columns=df.width,
            )
            return df
        except Exception as e:
            logger.error("clickhouse.query_failed", error=str(e), query=query)
            raise

    def get_daily_betting_stats(self, date: str) -> pl.DataFrame:
        """Get daily betting statistics."""
        return self.query_to_polars(
            """
            SELECT
                toDate(event_time) as date,
                sport,
                country,
                count() as total_bets,
                uniq(user_id) as unique_users,
                sum(toDecimal64(stake, 2)) as total_stake,
                sum(toDecimal64(pnl, 2)) as total_pnl,
                avg(toDecimal64(odds, 4)) as avg_odds
            FROM bet_events
            WHERE toDate(event_time) = {date:Date}
            GROUP BY date, sport, country
            ORDER BY total_stake DESC
        """,
            {"date": date},
        )

    def get_user_cohort_retention(self, cohort_month: str, months_forward: int = 6) -> pl.DataFrame:
        """Calculate retention for a registration cohort."""
        return self.query_to_polars(
            """
            SELECT
                toStartOfMonth(first_event) as cohort_month,
                dateDiff(
                    'month',
                    toStartOfMonth(first_event),
                    toStartOfMonth(event_time)
                ) as months_since,
                uniq(user_id) as active_users
            FROM (
                SELECT user_id, min(event_time) as first_event, event_time
                FROM user_events
                WHERE event_type IN ('bet_placed', 'deposit', 'game_started')
                GROUP BY user_id, event_time
            )
            WHERE toStartOfMonth(first_event) = {cohort:String}
              AND dateDiff(
                    'month',
                    toStartOfMonth(first_event),
                    toStartOfMonth(event_time)
              ) <= {months:UInt32}
            GROUP BY cohort_month, months_since
            ORDER BY months_since
            """,
            {"cohort": cohort_month, "months": months_forward},
        )

    def close(self):
        """Close ClickHouse connection."""
        self.client.close()
        logger.info("clickhouse.disconnected")
