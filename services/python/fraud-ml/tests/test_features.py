"""Tests for feature engineering (no ClickHouse connection required)."""

import polars as pl
import pytest

from src.features.extraction import FeatureExtractor
from src.features.transformation import FEATURE_REGISTRY, MODEL_FEATURES


def complete_feature_frame() -> pl.DataFrame:
    """Frame containing every model feature (for transformer tests)."""
    return pl.DataFrame({name: [1] for name in MODEL_FEATURES})


@pytest.fixture
def extractor():
    """FeatureExtractor with a stubbed ClickHouse client (unused here)."""
    return FeatureExtractor(ch_client=None)


@pytest.fixture
def raw_features():
    """Minimal raw feature frame (all inputs of compute_derived_features)."""
    return pl.DataFrame(
        {
            "user_id": [1, 2],
            "bets_7d": [40, 10],
            "bets_24h": [30, 1],
            "wins_7d": [5, 10],
            "settled_7d": [10, 20],
            "std_bet_30d": [50.0, 100.0],
            "avg_bet_30d": [100.0, 200.0],
            "max_bet_30d": [5000.0, 300.0],
            "total_deposit_30d": [7000.0, 100.0],
            "device_count_30d": [5, 2],
            "ip_count_30d": [15, 5],
        }
    )


class TestFeatureRegistry:
    """Feature registry is the single source of truth for model inputs."""

    def test_model_features_cover_registry(self):
        assert len(MODEL_FEATURES) == len(FEATURE_REGISTRY)
        assert set(MODEL_FEATURES) == set(FEATURE_REGISTRY.keys())
        assert len(MODEL_FEATURES) == len(set(MODEL_FEATURES))

    def test_every_feature_is_documented(self):
        for name, meta in FEATURE_REGISTRY.items():
            assert meta["description"], f"{name} has no description"
            assert meta["source"] in {"extraction", "derived"}


class TestDerivedFeatures:
    """Derived feature computation (win rate, CV, ratios, flags)."""

    def test_compute_derived_features(self, extractor, raw_features):
        derived = extractor.compute_derived_features(raw_features)

        for column in (
            "win_rate_7d",
            "bet_cv_30d",
            "deposit_bet_ratio",
            "multi_device",
            "multi_ip",
            "high_roller",
            "rapid_bettor",
        ):
            assert column in derived.columns

        # Win rate: wins / max(settled, 1)
        assert derived["win_rate_7d"][0] == 0.5
        assert derived["win_rate_7d"][1] == 0.5

        # Coefficient of variation: std / max(avg, 0.01)
        assert derived["bet_cv_30d"][0] == 0.5
        assert derived["bet_cv_30d"][1] == 0.5

        # Multi-device (>3) and multi-IP (>10) indicators
        assert derived["multi_device"][0] == 1  # 5 > 3
        assert derived["multi_device"][1] == 0  # 2 <= 3
        assert derived["multi_ip"][0] == 1  # 15 > 10
        assert derived["multi_ip"][1] == 0  # 5 <= 10

        # High roller: max bet > 10x average
        assert derived["high_roller"][0] == 1  # 5000 > 1000
        assert derived["high_roller"][1] == 0  # 300 <= 2000

        # Rapid bettor: >50% of weekly bets inside 24h
        assert derived["rapid_bettor"][0] == 1  # 30 > 20
        assert derived["rapid_bettor"][1] == 0  # 1 <= 5

    def test_deposit_bet_ratio_uses_weekly_average_stake(self, extractor):
        df = pl.DataFrame(
            {
                "bets_7d": [70, 0],
                "bets_24h": [0, 0],
                "wins_7d": [0, 0],
                "settled_7d": [10, 0],
                "std_bet_30d": [10.0, 10.0],
                "avg_bet_30d": [10.0, 10.0],
                "max_bet_30d": [10.0, 10.0],
                "total_deposit_30d": [100.0, 100.0],
                "device_count_30d": [1, 1],
                "ip_count_30d": [1, 1],
            }
        )
        derived = extractor.compute_derived_features(df)

        # 100 / 10 / 70 * 7 = 1.0
        assert derived["deposit_bet_ratio"][0] == pytest.approx(1.0)
        # bets_7d clipped to 1 -> 100 / 10 / 1 * 7
        assert derived["deposit_bet_ratio"][1] == pytest.approx(70.0)

    def test_zero_denominators_do_not_produce_nulls(self, extractor):
        df = pl.DataFrame(
            {
                "bets_7d": [0, 0],
                "bets_24h": [0, 0],
                "wins_7d": [0, 0],
                "settled_7d": [0, 0],
                "std_bet_30d": [0.0, 0.0],
                "avg_bet_30d": [0.0, 0.0],
                "max_bet_30d": [0.0, 0.0],
                "total_deposit_30d": [0.0, 0.0],
                "device_count_30d": [0, 0],
                "ip_count_30d": [0, 0],
            }
        )
        derived = extractor.compute_derived_features(df)

        for column in ("win_rate_7d", "bet_cv_30d", "deposit_bet_ratio"):
            assert derived[column].null_count() == 0, f"{column} has nulls"

    def test_empty_frame_is_returned_unchanged(self, extractor):
        empty = pl.DataFrame(
            {
                "bets_7d": [],
                "bets_24h": [],
                "wins_7d": [],
                "settled_7d": [],
                "std_bet_30d": [],
                "avg_bet_30d": [],
                "max_bet_30d": [],
                "total_deposit_30d": [],
                "device_count_30d": [],
                "ip_count_30d": [],
            }
        )
        assert extractor.compute_derived_features(empty).is_empty()


class TestFeatureTransformer:
    """Feature presence validation before training."""

    def test_validate_features_accepts_complete_frame(self):
        from src.features.transformation import FeatureTransformer

        transformer = FeatureTransformer()
        df = complete_feature_frame()

        assert transformer.validate_features(df) is not None
        assert transformer.select_features(df).columns == MODEL_FEATURES

    def test_validate_features_reports_missing_columns(self):
        from src.features.transformation import FeatureTransformer

        transformer = FeatureTransformer()
        with pytest.raises(ValueError, match="Missing features"):
            transformer.validate_features(pl.DataFrame({"bets_7d": [1]}))
