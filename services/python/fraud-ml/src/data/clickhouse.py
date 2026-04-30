"""
ClickHouse data access with Polars integration.
"""
import structlog
import polars as pl
import clickhouse_connect

logger = structlog.get_logger()


class ClickHouseClient:
    """ClickHouse client with Polars DataFrame support."""
    
    def __init__(
        self,
        host: str,
        port: int,
        database: str,
        user: str = "default",
        password: str = "",
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
        """Execute query and return Polars DataFrame."""
        try:
            result = self.client.query(query, parameters=params)
            df = pl.from_arrow(result.to_arrow())
            logger.debug(
                "clickhouse.query_executed",
                rows=len(df) if df is not None else 0,
                columns=len(df.columns) if df is not None else 0,
            )
            return df
        except Exception as e:
            logger.error("clickhouse.query_failed", error=str(e), query=query)
            raise
    
    def get_daily_betting_stats(self, date: str) -> pl.DataFrame:
        """Get daily betting statistics."""
        return self.query_to_polars("""
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
        """, {"date": date})
    
    def get_user_cohort_retention(self, cohort_month: str, months_forward: int = 6) -> pl.DataFrame:
        """Calculate retention for a registration cohort."""
        return self.query_to_polars("""
            SELECT
                toStartOfMonth(first_event) as cohort_month,
                dateDiff('month', toStartOfMonth(first_event), toStartOfMonth(event_time)) as months_since,
                uniq(user_id) as active_users
            FROM (
                SELECT user_id, min(event_time) as first_event, event_time
                FROM user_events
                WHERE event_type IN ('bet_placed', 'deposit', 'game_started')
                GROUP BY user_id, event_time
            )
            WHERE toStartOfMonth(first_event) = {cohort:String}
              AND dateDiff('month', toStartOfMonth(first_event), toStartOfMonth(event_time)) <= {months:UInt32}
            GROUP BY cohort_month, months_since
            ORDER BY months_since
        """, {"cohort": cohort_month, "months": months_forward})
    
    def close(self):
        """Close ClickHouse connection."""
        self.client.close()
        logger.info("clickhouse.disconnected")
