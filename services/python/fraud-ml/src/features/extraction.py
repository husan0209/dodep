"""
Feature extraction from ClickHouse.
"""
import structlog
import polars as pl
from datetime import datetime, timedelta

from .clickhouse import ClickHouseClient

logger = structlog.get_logger()


class FeatureExtractor:
    """Extract features for fraud detection from ClickHouse."""
    
    def __init__(self, ch_client: ClickHouseClient):
        self.ch = ch_client
    
    def extract_user_features(
        self,
        user_ids: list[int],
        as_of: datetime
    ) -> pl.DataFrame:
        """
        Extract features for a batch of users as of a specific time.
        
        Features include:
        - Betting behavior (frequency, amounts, patterns)
        - Deposit behavior
        - Session behavior (devices, IPs, countries)
        - Win/loss patterns
        - Account age
        """
        if not user_ids:
            return pl.DataFrame()
        
        query = """
        SELECT
            user_id,
            
            -- Betting behavior
            countIf(event_type = 'bet_placed' AND event_time >= {cutoff_7d}) as bets_7d,
            countIf(event_type = 'bet_placed' AND event_time >= {cutoff_24h}) as bets_24h,
            avgIf(toDecimal64(JSONExtractFloat(properties, 'amount'), 2),
                  event_type = 'bet_placed' AND event_time >= {cutoff_30d}) as avg_bet_30d,
            stddevPopIf(toDecimal64(JSONExtractFloat(properties, 'amount'), 2),
                        event_type = 'bet_placed' AND event_time >= {cutoff_30d}) as std_bet_30d,
            maxIf(toDecimal64(JSONExtractFloat(properties, 'amount'), 2),
                  event_type = 'bet_placed' AND event_time >= {cutoff_30d}) as max_bet_30d,
            
            -- Deposit behavior
            countIf(event_type = 'deposit' AND event_time >= {cutoff_24h}) as deposits_24h,
            sumIf(toDecimal64(JSONExtractFloat(properties, 'amount'), 2),
                  event_type = 'deposit' AND event_time >= {cutoff_30d}) as total_deposit_30d,
            
            -- Session behavior
            uniqIf(JSONExtractString(properties, 'device_id'),
                   event_time >= {cutoff_30d}) as device_count_30d,
            uniqIf(JSONExtractString(properties, 'ip'),
                   event_time >= {cutoff_30d}) as ip_count_30d,
            uniqIf(country, event_time >= {cutoff_30d}) as country_count_30d,
            
            -- Win rate
            countIf(event_type = 'bet_settled' AND
                    JSONExtractString(properties, 'result') = 'won' AND
                    event_time >= {cutoff_7d}) as wins_7d,
            countIf(event_type = 'bet_settled' AND event_time >= {cutoff_7d}) as settled_7d,
            
            -- Account age
            dateDiff('day', min(event_time), {as_of}) as account_age_days
            
        FROM user_events
        WHERE user_id IN ({user_ids})
          AND event_time <= {as_of}
        GROUP BY user_id
        """
        
        result = self.ch.query_to_polars(query, {
            "user_ids": ",".join(str(uid) for uid in user_ids),
            "as_of": as_of.isoformat(),
            "cutoff_24h": (as_of - timedelta(hours=24)).isoformat(),
            "cutoff_7d": (as_of - timedelta(days=7)).isoformat(),
            "cutoff_30d": (as_of - timedelta(days=30)).isoformat(),
        })
        
        logger.info(
            "features.extracted",
            users=len(result),
            as_of=as_of.isoformat(),
        )
        
        return result
    
    def compute_derived_features(self, df: pl.DataFrame) -> pl.DataFrame:
        """
        Compute derived features from raw features.
        
        Derived features include:
        - Win rate
        - Coefficient of variation (bet consistency)
        - Deposit to bet ratio
        - Multi-device indicator
        - Multi-IP indicator
        """
        if df.is_empty():
            return df
        
        derived = df.with_columns([
            # Win rate
            (pl.col("wins_7d") / pl.col("settled_7d").clip(lower_bound=1))
                .alias("win_rate_7d"),
            
            # Coefficient of variation (bet amount consistency)
            (pl.col("std_bet_30d") / pl.col("avg_bet_30d").clip(lower_bound=0.01))
                .alias("bet_cv_30d"),
            
            # Deposit to bet ratio
            (pl.col("total_deposit_30d") / 
             pl.col("avg_bet_30d").clip(lower_bound=0.01) /
             pl.col("bets_7d").clip(lower_bound=1) * 7)
                .alias("deposit_bet_ratio"),
            
            # Multi-device indicator
            (pl.col("device_count_30d") > 3).cast(pl.Int8).alias("multi_device"),
            
            # Multi-IP indicator
            (pl.col("ip_count_30d") > 10).cast(pl.Int8).alias("multi_ip"),
            
            # High roller indicator
            (pl.col("max_bet_30d") > pl.col("avg_bet_30d") * 10).cast(pl.Int8).alias("high_roller"),
            
            # Rapid bettor indicator
            (pl.col("bets_24h") > pl.col("bets_7d") * 0.5).cast(pl.Int8).alias("rapid_bettor"),
        ]).fill_null(0)
        
        logger.debug(
            "features.derived_computed",
            features=[
                "win_rate_7d", "bet_cv_30d", "deposit_bet_ratio",
                "multi_device", "multi_ip", "high_roller", "rapid_bettor",
            ],
        )
        
        return derived
    
    def extract_training_data(
        self,
        lookback_days: int = 90,
        as_of: datetime | None = None
    ) -> pl.DataFrame:
        """
        Extract training data with labels for model training.
        
        This includes historical features and fraud labels from
        the fraud_signals table.
        """
        if as_of is None:
            as_of = datetime.utcnow()
        
        cutoff = as_of - timedelta(days=lookback_days)
        
        query = """
        SELECT
            ue.user_id,
            ue.event_time,
            
            -- Betting behavior
            countIf(ue.event_type = 'bet_placed') OVER w_7d as bets_7d,
            countIf(ue.event_type = 'bet_placed') OVER w_24h as bets_24h,
            avgIf(toDecimal64(JSONExtractFloat(ue.properties, 'amount'), 2),
                  ue.event_type = 'bet_placed') OVER w_30d as avg_bet_30d,
            stddevPopIf(toDecimal64(JSONExtractFloat(ue.properties, 'amount'), 2),
                        ue.event_type = 'bet_placed') OVER w_30d as std_bet_30d,
            maxIf(toDecimal64(JSONExtractFloat(ue.properties, 'amount'), 2),
                  ue.event_type = 'bet_placed') OVER w_30d as max_bet_30d,
            
            -- Deposit behavior
            countIf(ue.event_type = 'deposit') OVER w_24h as deposits_24h,
            sumIf(toDecimal64(JSONExtractFloat(ue.properties, 'amount'), 2),
                  ue.event_type = 'deposit') OVER w_30d as total_deposit_30d,
            
            -- Session behavior
            uniqIf(JSONExtractString(ue.properties, 'device_id'), true) OVER w_30d as device_count_30d,
            uniqIf(JSONExtractString(ue.properties, 'ip'), true) OVER w_30d as ip_count_30d,
            uniqIf(ue.country, true) OVER w_30d as country_count_30d,
            
            -- Win rate
            countIf(ue.event_type = 'bet_settled' AND
                    JSONExtractString(ue.properties, 'result') = 'won') OVER w_7d as wins_7d,
            countIf(ue.event_type = 'bet_settled') OVER w_7d as settled_7d,
            
            -- Account age
            dateDiff('day', min(ue.event_time) OVER (PARTITION BY ue.user_id), ue.event_time) as account_age_days,
            
            -- Label: fraud signal
            coalesce(max(fs.is_fraud), 0) as is_fraud
            
        FROM user_events ue
        LEFT JOIN fraud_signals fs 
            ON ue.user_id = fs.user_id 
            AND fs.event_time >= ue.event_time 
            AND fs.event_time < ue.event_time + INTERVAL 1 DAY
        WHERE ue.event_time >= {cutoff:DateTime}
          AND ue.event_time <= {as_of:DateTime}
        WINDOW
            w_24h AS (PARTITION BY ue.user_id ORDER BY ue.event_time RANGE BETWEEN INTERVAL 24 HOUR PRECEDING AND CURRENT ROW),
            w_7d AS (PARTITION BY ue.user_id ORDER BY ue.event_time RANGE BETWEEN INTERVAL 7 DAY PRECEDING AND CURRENT ROW),
            w_30d AS (PARTITION BY ue.user_id ORDER BY ue.event_time RANGE BETWEEN INTERVAL 30 DAY PRECEDING AND CURRENT ROW)
        QUALIFY row_number() OVER (PARTITION BY ue.user_id ORDER BY ue.event_time DESC) = 1
        """
        
        df = self.ch.query_to_polars(query, {
            "cutoff": cutoff.isoformat(),
            "as_of": as_of.isoformat(),
        })
        
        logger.info(
            "features.training_data_extracted",
            rows=len(df),
            fraud_count=df["is_fraud"].sum() if "is_fraud" in df.columns else 0,
            lookback_days=lookback_days,
        )
        
        return df
