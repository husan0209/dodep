# SKILL #23 — python-data-processing.skill.md

```markdown
# python-data-processing.skill.md
# GAMBLING PLATFORM — PYTHON DATA PROCESSING
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: Python ML Agent, Data Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

Python handles batch data processing:
- Analytics aggregation pipelines
- Report generation (daily, monthly, regulatory)
- Data quality checks
- Feature computation for ML

Data source: ClickHouse (read), PostgreSQL (read-only replica).
Output: ClickHouse (aggregated tables), S3 (reports), Redpanda (events).
Library: Polars (NOT Pandas — 10x faster, less memory).

# ============================================================
# SECTION 2: POLARS OVER PANDAS
# ============================================================

```python
# ── WHY Polars ──
# Pandas: single-threaded, eager, copies data
# Polars: multi-threaded, lazy evaluation, zero-copy, Rust backend

import polars as pl

# ✅ GOOD: Polars lazy evaluation
df = (
    pl.scan_parquet("data/bets.parquet")
    .filter(pl.col("placed_at") >= "2025-01-01")
    .group_by("sport_id")
    .agg([
        pl.count().alias("total_bets"),
        pl.col("stake").sum().alias("total_stake"),
        pl.col("stake").mean().alias("avg_stake"),
        pl.col("actual_win").sum().alias("total_payout"),
        (pl.col("stake").sum() - pl.col("actual_win").sum()).alias("ggr"),
    ])
    .sort("ggr", descending=True)
    .collect()  # executes entire chain optimally
)

# ❌ BAD: Pandas equivalent (slower, more memory)
# df = pd.read_parquet("data/bets.parquet")
# df = df[df["placed_at"] >= "2025-01-01"]
# result = df.groupby("sport_id").agg({"stake": ["count", "sum", "mean"]})
============================================================
SECTION 3: CLICKHOUSE DATA ACCESS
============================================================
Python

import clickhouse_connect
import polars as pl
from datetime import datetime, timedelta

class ClickHouseClient:
    def __init__(self, host: str, port: int, database: str):
        self.client = clickhouse_connect.get_client(
            host=host, port=port, database=database,
            settings={"max_execution_time": 300}
        )
    
    def query_to_polars(self, query: str, params: dict = None) -> pl.DataFrame:
        """Execute query and return Polars DataFrame."""
        result = self.client.query(query, parameters=params)
        return pl.from_arrow(result.to_arrow())
    
    def get_daily_betting_stats(self, date: datetime) -> pl.DataFrame:
        """Get daily betting statistics for all sports."""
        return self.query_to_polars("""
            SELECT
                toDate(event_time) as date,
                sport,
                country,
                count() as total_bets,
                uniq(user_id) as unique_users,
                sum(toDecimal64(stake, 2)) as total_stake,
                sum(toDecimal64(pnl, 2)) as total_pnl,
                avg(toDecimal64(odds, 4)) as avg_odds,
                countIf(action = 'settled' AND result = 'won') as wins,
                countIf(action = 'settled' AND result = 'lost') as losses
            FROM bet_events
            WHERE toDate(event_time) = {date:Date}
            GROUP BY date, sport, country
            ORDER BY total_stake DESC
        """, {"date": date.date()})
    
    def get_user_cohort_retention(
        self, cohort_month: str, months_forward: int = 6
    ) -> pl.DataFrame:
        """Calculate retention for a registration cohort."""
        return self.query_to_polars("""
            SELECT
                toStartOfMonth(first_event) as cohort_month,
                dateDiff('month', toStartOfMonth(first_event), 
                         toStartOfMonth(event_time)) as months_since,
                uniq(user_id) as active_users
            FROM (
                SELECT user_id, min(event_time) as first_event, event_time
                FROM user_events
                WHERE event_type IN ('bet_placed', 'deposit', 'game_started')
                GROUP BY user_id, event_time
            )
            WHERE toStartOfMonth(first_event) = {cohort:String}
              AND dateDiff('month', toStartOfMonth(first_event), 
                           toStartOfMonth(event_time)) <= {months:UInt32}
            GROUP BY cohort_month, months_since
            ORDER BY months_since
        """, {"cohort": cohort_month, "months": months_forward})
============================================================
SECTION 4: REPORT GENERATION
============================================================
Python

import structlog
from pathlib import Path
from datetime import datetime

logger = structlog.get_logger()

class ReportGenerator:
    """Generate regulatory and business reports."""
    
    def __init__(self, ch_client, s3_client, config):
        self.ch = ch_client
        self.s3 = s3_client
        self.config = config
    
    def generate_daily_financial_report(self, date: datetime) -> Path:
        """Daily financial summary — required by regulators."""
        logger.info("report.daily_financial.start", date=date.isoformat())
        
        # Fetch data
        betting = self.ch.get_daily_betting_stats(date)
        deposits = self.ch.query_to_polars("""
            SELECT currency, payment_method,
                   count() as txn_count,
                   sum(amount) as total_amount
            FROM payment_events
            WHERE toDate(event_time) = {date:Date} AND type = 'deposit' AND status = 'completed'
            GROUP BY currency, payment_method
        """, {"date": date.date()})
        withdrawals = self.ch.query_to_polars("""
            SELECT currency, payment_method,
                   count() as txn_count,
                   sum(amount) as total_amount
            FROM payment_events
            WHERE toDate(event_time) = {date:Date} AND type = 'withdrawal' AND status = 'completed'
            GROUP BY currency, payment_method
        """, {"date": date.date()})
        
        # Build report
        report = {
            "date": date.strftime("%Y-%m-%d"),
            "generated_at": datetime.utcnow().isoformat(),
            "betting_summary": betting.to_dicts(),
            "deposits": {
                "total": float(deposits["total_amount"].sum()),
                "by_method": deposits.to_dicts(),
            },
            "withdrawals": {
                "total": float(withdrawals["total_amount"].sum()),
                "by_method": withdrawals.to_dicts(),
            },
            "ggr": float(betting["total_pnl"].sum()) if "total_pnl" in betting.columns else 0,
        }
        
        # Save to file
        filename = f"daily_financial_{date.strftime('%Y%m%d')}.json"
        filepath = Path(f"/tmp/reports/{filename}")
        filepath.parent.mkdir(parents=True, exist_ok=True)
        
        import json
        with open(filepath, "w") as f:
            json.dump(report, f, indent=2, default=str)
        
        # Upload to S3
        s3_key = f"reports/financial/daily/{date.strftime('%Y/%m')}/{filename}"
        self.s3.upload_file(str(filepath), self.config.s3_bucket, s3_key)
        
        logger.info("report.daily_financial.complete", s3_key=s3_key)
        return filepath
    
    def generate_rtp_monitoring_report(self, date: datetime) -> pl.DataFrame:
        """Monitor actual RTP vs theoretical for each casino game."""
        return self.ch.query_to_polars("""
            SELECT
                game_id, game_name, provider,
                count() as rounds,
                sum(bet_amount) as total_wagered,
                sum(win_amount) as total_returned,
                (sum(win_amount) / sum(bet_amount)) * 100 as actual_rtp,
                any(theoretical_rtp) as expected_rtp,
                abs((sum(win_amount) / sum(bet_amount)) * 100 - any(theoretical_rtp)) as rtp_deviation
            FROM game_rounds
            WHERE toDate(event_time) = {date:Date}
              AND bet_amount > 0
            GROUP BY game_id, game_name, provider
            HAVING rounds >= 1000
            ORDER BY rtp_deviation DESC
        """, {"date": date.date()})
============================================================
SECTION 5: DATA QUALITY CHECKS
============================================================
Python

class DataQualityChecker:
    """Run data quality checks, alert on anomalies."""
    
    def check_betting_volume(self, ch_client, date: datetime) -> list[str]:
        """Check if betting volume is within expected range."""
        alerts = []
        
        today = ch_client.query_to_polars("""
            SELECT count() as bets FROM bet_events
            WHERE toDate(event_time) = {date:Date}
        """, {"date": date.date()})
        
        avg_7d = ch_client.query_to_polars("""
            SELECT avg(daily_bets) as avg_bets FROM (
                SELECT toDate(event_time) as d, count() as daily_bets
                FROM bet_events
                WHERE toDate(event_time) BETWEEN {from:Date} AND {to:Date}
                GROUP BY d
            )
        """, {
            "from": (date - timedelta(days=8)).date(),
            "to": (date - timedelta(days=1)).date(),
        })
        
        today_count = today["bets"][0]
        avg_count = avg_7d["avg_bets"][0]
        
        if today_count < avg_count * 0.5:
            alerts.append(f"Betting volume {today_count} is <50% of 7d avg {avg_count:.0f}")
        if today_count > avg_count * 2.0:
            alerts.append(f"Betting volume {today_count} is >200% of 7d avg {avg_count:.0f}")
        
        return alerts
============================================================
SECTION 6: ANTI-PATTERNS
============================================================
text

❌ NEVER use Pandas for datasets > 1M rows → USE Polars
❌ NEVER run queries without timeout (ClickHouse can run forever)
❌ NEVER write to production PostgreSQL from batch jobs
❌ NEVER hardcode dates or filters → USE parameters
❌ NEVER skip data quality checks before report generation
❌ NEVER store reports with PII in accessible S3 buckets
❌ NEVER use print() → USE structlog for structured logging
❌ NEVER process unbounded result sets → USE LIMIT or pagination
❌ NEVER ignore timezone in date handling → ALWAYS UTC