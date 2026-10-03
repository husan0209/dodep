"""
Feature transformation and registry.
"""
import polars as pl
import structlog

logger = structlog.get_logger()


def _feature(kind: str, source: str, description: str) -> dict[str, str]:
    """Build one registry entry.

    The table below is a flat list of identical-shaped dicts; spelling the keys out on
    every row pushed most lines past the configured 100-column limit (ruff E501), so the
    shape is factored out here and each feature fits on a single readable line.
    """
    return {"type": kind, "source": source, "description": description}


# Feature registry - defines all features used by the model
FEATURE_REGISTRY = {
    # Betting behavior
    "bets_7d": _feature("numeric", "extraction", "Bets in last 7 days"),
    "bets_24h": _feature("numeric", "extraction", "Bets in last 24 hours"),
    "avg_bet_30d": _feature("numeric", "extraction", "Average bet amount (30d)"),
    "std_bet_30d": _feature("numeric", "extraction", "Std dev of bet amounts (30d)"),
    "max_bet_30d": _feature("numeric", "extraction", "Maximum bet amount (30d)"),

    # Deposit behavior
    "deposits_24h": _feature("numeric", "extraction", "Deposits in last 24 hours"),
    "total_deposit_30d": _feature("numeric", "extraction", "Total deposited (30d)"),

    # Session behavior
    "device_count_30d": _feature("numeric", "extraction", "Unique devices (30d)"),
    "ip_count_30d": _feature("numeric", "extraction", "Unique IPs (30d)"),
    "country_count_30d": _feature("numeric", "extraction", "Unique countries (30d)"),

    # Win rate
    "wins_7d": _feature("numeric", "extraction", "Wins in last 7 days"),
    "settled_7d": _feature("numeric", "extraction", "Settled bets (7d)"),

    # Account
    "account_age_days": _feature("numeric", "extraction", "Account age in days"),

    # Derived features
    "win_rate_7d": _feature("numeric", "derived", "Win rate (7d)"),
    "bet_cv_30d": _feature("numeric", "derived", "Coefficient of variation (bet consistency)"),
    "deposit_bet_ratio": _feature("numeric", "derived", "Deposit to bet ratio"),
    "multi_device": _feature("binary", "derived", "Multi-device indicator (>3)"),
    "multi_ip": _feature("binary", "derived", "Multi-IP indicator (>10)"),
    "high_roller": _feature("binary", "derived", "High roller indicator"),
    "rapid_bettor": _feature("binary", "derived", "Rapid bettor indicator"),
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
