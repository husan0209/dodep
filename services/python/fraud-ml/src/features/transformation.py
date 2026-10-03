"""
Feature transformation and registry.
"""
import polars as pl
import structlog

logger = structlog.get_logger()


def _f(feature_type: str, source: str, description: str) -> dict[str, str]:
    """Build one feature-registry entry.

    The registry used to spell every entry out as an inline dict literal, which
    pushed a dozen lines past the 100-character limit. Same data, one shape.
    """
    return {"type": feature_type, "source": source, "description": description}


# Feature registry - defines all features used by the model
FEATURE_REGISTRY = {
    # Betting behavior
    "bets_7d": _f("numeric", "extraction", "Bets in last 7 days"),
    "bets_24h": _f("numeric", "extraction", "Bets in last 24 hours"),
    "avg_bet_30d": _f("numeric", "extraction", "Average bet amount (30d)"),
    "std_bet_30d": _f("numeric", "extraction", "Std dev of bet amounts (30d)"),
    "max_bet_30d": _f("numeric", "extraction", "Maximum bet amount (30d)"),

    # Deposit behavior
    "deposits_24h": _f("numeric", "extraction", "Deposits in last 24 hours"),
    "total_deposit_30d": _f("numeric", "extraction", "Total deposited (30d)"),

    # Session behavior
    "device_count_30d": _f("numeric", "extraction", "Unique devices (30d)"),
    "ip_count_30d": _f("numeric", "extraction", "Unique IPs (30d)"),
    "country_count_30d": _f("numeric", "extraction", "Unique countries (30d)"),

    # Win rate
    "wins_7d": _f("numeric", "extraction", "Wins in last 7 days"),
    "settled_7d": _f("numeric", "extraction", "Settled bets (7d)"),

    # Account
    "account_age_days": _f("numeric", "extraction", "Account age in days"),

    # Derived features
    "win_rate_7d": _f("numeric", "derived", "Win rate (7d)"),
    "bet_cv_30d": _f("numeric", "derived", "Coefficient of variation (bet consistency)"),
    "deposit_bet_ratio": _f("numeric", "derived", "Deposit to bet ratio"),
    "multi_device": _f("binary", "derived", "Multi-device indicator (>3)"),
    "multi_ip": _f("binary", "derived", "Multi-IP indicator (>10)"),
    "high_roller": _f("binary", "derived", "High roller indicator"),
    "rapid_bettor": _f("binary", "derived", "Rapid bettor indicator"),
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
