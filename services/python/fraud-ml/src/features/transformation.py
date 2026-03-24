"""
Feature transformation and registry.
"""
import structlog
import polars as pl

logger = structlog.get_logger()


# Feature registry - defines all features used by the model
FEATURE_REGISTRY = {
    # Betting behavior
    "bets_7d": {"type": "numeric", "source": "extraction", "description": "Bets in last 7 days"},
    "bets_24h": {"type": "numeric", "source": "extraction", "description": "Bets in last 24 hours"},
    "avg_bet_30d": {"type": "numeric", "source": "extraction", "description": "Average bet amount (30d)"},
    "std_bet_30d": {"type": "numeric", "source": "extraction", "description": "Std dev of bet amounts (30d)"},
    "max_bet_30d": {"type": "numeric", "source": "extraction", "description": "Maximum bet amount (30d)"},
    
    # Deposit behavior
    "deposits_24h": {"type": "numeric", "source": "extraction", "description": "Deposits in last 24 hours"},
    "total_deposit_30d": {"type": "numeric", "source": "extraction", "description": "Total deposited (30d)"},
    
    # Session behavior
    "device_count_30d": {"type": "numeric", "source": "extraction", "description": "Unique devices (30d)"},
    "ip_count_30d": {"type": "numeric", "source": "extraction", "description": "Unique IPs (30d)"},
    "country_count_30d": {"type": "numeric", "source": "extraction", "description": "Unique countries (30d)"},
    
    # Win rate
    "wins_7d": {"type": "numeric", "source": "extraction", "description": "Wins in last 7 days"},
    "settled_7d": {"type": "numeric", "source": "extraction", "description": "Settled bets (7d)"},
    
    # Account
    "account_age_days": {"type": "numeric", "source": "extraction", "description": "Account age in days"},
    
    # Derived features
    "win_rate_7d": {"type": "numeric", "source": "derived", "description": "Win rate (7d)"},
    "bet_cv_30d": {"type": "numeric", "source": "derived", "description": "Coefficient of variation (bet consistency)"},
    "deposit_bet_ratio": {"type": "numeric", "source": "derived", "description": "Deposit to bet ratio"},
    "multi_device": {"type": "binary", "source": "derived", "description": "Multi-device indicator (>3)"},
    "multi_ip": {"type": "binary", "source": "derived", "description": "Multi-IP indicator (>10)"},
    "high_roller": {"type": "binary", "source": "derived", "description": "High roller indicator"},
    "rapid_bettor": {"type": "binary", "source": "derived", "description": "Rapid bettor indicator"},
}

# Features used by the model
MODEL_FEATURES = list(FEATURE_REGISTRY.keys())


class FeatureTransformer:
    """Transform and validate features."""
    
    def __init__(self):
        self.feature_names = MODEL_FEATURES
    
    def validate_features(self, df: pl.DataFrame) -> pl.DataFrame:
        """Validate that all required features are present."""
        missing = set(self.feature_names) - set(df.columns)
        if missing:
            logger.error(
                "features.missing",
                missing=list(missing),
            )
            raise ValueError(f"Missing features: {missing}")
        
        logger.debug("features.validated", count=len(self.feature_names))
        return df
    
    def select_features(self, df: pl.DataFrame) -> pl.DataFrame:
        """Select only features used by the model."""
        return df.select(self.feature_names)
    
    def get_feature_info(self) -> dict:
        """Get feature registry information."""
        return FEATURE_REGISTRY.copy()
