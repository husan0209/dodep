"""
Feature transformation and registry.
"""
import polars as pl
import structlog

logger = structlog.get_logger()


def _f(source: str, description: str, kind: str = "numeric") -> dict[str, str]:
    """Build a feature-registry entry.

    Splitting this out keeps every row of FEATURE_REGISTRY readable and well
    under the 100-character line limit enforced by ruff.
    """
    return {"type": kind, "source": source, "description": description}


# Feature registry - defines all features used by the model
FEATURE_REGISTRY = {
    # Betting behavior
    "bets_7d": _f("extraction", "Bets in last 7 days"),
    "bets_24h": _f("extraction", "Bets in last 24 hours"),
    "avg_bet_30d": _f("extraction", "Average bet amount (30d)"),
    "std_bet_30d": _f("extraction", "Std dev of bet amounts (30d)"),
    "max_bet_30d": _f("extraction", "Maximum bet amount (30d)"),
    # Deposit behavior
    "deposits_24h": _f("extraction", "Deposits in last 24 hours"),
    "total_deposit_30d": _f("extraction", "Total deposited (30d)"),
    # Session behavior
    "device_count_30d": _f("extraction", "Unique devices (30d)"),
    "ip_count_30d": _f("extraction", "Unique IPs (30d)"),
    "country_count_30d": _f("extraction", "Unique countries (30d)"),
    # Win rate
    "wins_7d": _f("extraction", "Wins in last 7 days"),
    "settled_7d": _f("extraction", "Settled bets (7d)"),
    # Account
    "account_age_days": _f("extraction", "Account age in days"),
    # Derived features
    "win_rate_7d": _f("derived", "Win rate (7d)"),
    "bet_cv_30d": _f("derived", "Coefficient of variation (bet consistency)"),
    "deposit_bet_ratio": _f("derived", "Deposit to bet ratio"),
    "multi_device": _f("derived", "Multi-device indicator (>3)", kind="binary"),
    "multi_ip": _f("derived", "Multi-IP indicator (>10)", kind="binary"),
    "high_roller": _f("derived", "High roller indicator", kind="binary"),
    "rapid_bettor": _f("derived", "Rapid bettor indicator", kind="binary"),
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

