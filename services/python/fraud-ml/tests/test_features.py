"""Tests for feature extraction."""

import polars as pl
import pytest

from src.features.extraction import FeatureExtractor


class TestFeatureExtractor:
    """Test feature extraction logic."""

    @pytest.fixture
    def mock_ch_client(self):
        """Mock ClickHouse client."""
        # In real tests, use testcontainers
        pass

    @pytest.fixture
    def extractor(self, mock_ch_client):
        """Create feature extractor."""
        return FeatureExtractor(mock_ch_client)

    @pytest.mark.skip(
        reason="needs a live ClickHouse; extract_user_features has no offline seam yet"
    )
    def test_extract_user_features(self, extractor):
        """Test feature extraction for users."""
        # This would require a real ClickHouse connection
        # df = extractor.extract_user_features([1, 2, 3], datetime.utcnow())
        # assert len(df) > 0
        # assert "bets_7d" in df.columns

    def test_compute_derived_features(self, extractor):
        """Test derived feature computation."""
        # Every column compute_derived_features() reads must be present: it is
        # called straight after the ClickHouse extraction, which always selects
        # the full set. The previous fixture omitted total_deposit_30d,
        # bets_7d, bets_24h and max_bet_30d, so it tested a column set the
        # function is never actually given.
        df = pl.DataFrame({
            "user_id": [1, 2],
            "bets_7d": [10, 20],
            "bets_24h": [1, 2],
            "wins_7d": [5, 10],
            "settled_7d": [10, 20],
            "std_bet_30d": [50.0, 100.0],
            "avg_bet_30d": [100.0, 200.0],
            "max_bet_30d": [500.0, 400.0],
            "total_deposit_30d": [1000.0, 500.0],
            "device_count_30d": [5, 2],
            "ip_count_30d": [15, 5],
        })

        derived = extractor.compute_derived_features(df)

        assert "win_rate_7d" in derived.columns
        assert "bet_cv_30d" in derived.columns
        assert "deposit_bet_ratio" in derived.columns
        assert "multi_device" in derived.columns
        assert "multi_ip" in derived.columns
        assert "high_roller" in derived.columns
        assert "rapid_bettor" in derived.columns

        # Check win rate calculation
        assert derived["win_rate_7d"][0] == 0.5
        assert derived["win_rate_7d"][1] == 0.5

        # Check multi-device indicator
        assert derived["multi_device"][0] == 1  # 5 > 3
        assert derived["multi_device"][1] == 0  # 2 <= 3

        # Check deposit ratio: total_deposit_30d / avg_bet_30d / bets_7d * 7,
        # evaluated left to right as the expression in the source is written
        #   row 0: (1000 / 100) / 10 * 7 = 7.0
        #   row 1: (500 / 200) / 20 * 7 = 0.875
        assert derived["deposit_bet_ratio"][0] == pytest.approx(7.0)
        assert derived["deposit_bet_ratio"][1] == pytest.approx(0.875)

    @pytest.mark.skip(reason="needs a live ClickHouse; no offline seam yet")
    def test_extract_training_data(self, extractor):
        """Test training data extraction."""
        # df = extractor.extract_training_data(lookback_days=90)
        # assert len(df) > 0
        # assert "is_fraud" in df.columns
