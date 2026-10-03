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

    def test_extract_user_features(self, extractor):
        """Requires a live ClickHouse, which this unit suite does not provide."""
        # extractor.extract_user_features(user_ids, as_of) queries
        # user_events / fraud_signals through CHClient, so it cannot run against
        # the stubbed fixture. Skipped rather than left as a body of commented
        # out asserts, which used to report as a pass while testing nothing.
        pytest.skip("extract_user_features requires a live ClickHouse connection")

    def test_compute_derived_features(self, extractor):
        """Test derived feature computation."""
        # compute_derived_features reads bets_7d, bets_24h, total_deposit_30d
        # and max_bet_30d as well; the fixture omitted them and polars aborted
        # with 'unable to find column "total_deposit_30d"'.
        df = pl.DataFrame(
            {
                "user_id": [1, 2],
                "wins_7d": [5, 10],
                "settled_7d": [10, 20],
                "std_bet_30d": [50.0, 100.0],
                "avg_bet_30d": [100.0, 200.0],
                "max_bet_30d": [900.0, 250.0],
                "bets_7d": [30, 20],
                "bets_24h": [2, 1],
                "total_deposit_30d": [3000.0, 500.0],
                "device_count_30d": [5, 2],
                "ip_count_30d": [15, 5],
            }
        )

        derived = extractor.compute_derived_features(df)

        assert "win_rate_7d" in derived.columns
        assert "bet_cv_30d" in derived.columns
        assert "multi_device" in derived.columns
        assert "multi_ip" in derived.columns

        # Check win rate calculation
        assert derived["win_rate_7d"][0] == 0.5
        assert derived["win_rate_7d"][1] == 0.5

        # Check multi-device indicator
        assert derived["multi_device"][0] == 1  # 5 > 3
        assert derived["multi_device"][1] == 0  # 2 <= 3

    def test_extract_training_data(self, extractor):
        """Training data extraction needs a live ClickHouse."""
        # extract_training_data runs the same user_events / fraud_signals join as
        # extract_user_features. Skipped rather than left as commented-out
        # asserts reporting a pass.
        pytest.skip("extract_training_data requires a live ClickHouse connection")
