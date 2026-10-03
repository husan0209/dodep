"""Tests for feature extraction."""

from datetime import datetime, timedelta
from unittest.mock import MagicMock

import polars as pl
import pytest

from src.features.extraction import FeatureExtractor


class TestFeatureExtractor:
    """Test feature extraction logic."""

    @pytest.fixture
    def mock_ch_client(self):
        """Stand-in for the ClickHouse client.

        The SQL itself needs a live server, so the client is mocked at the
        query boundary. Everything on this side of it still runs for real:
        the empty-input short circuit, cutoff binding and the pass-through of
        the returned frame.
        """
        client = MagicMock()
        client.query_to_polars.return_value = pl.DataFrame(
            {"user_id": [1, 2, 3], "bets_7d": [10, 20, 30]}
        )
        return client

    @pytest.fixture
    def extractor(self, mock_ch_client):
        """Create feature extractor."""
        return FeatureExtractor(mock_ch_client)

    def test_extract_user_features(self, extractor):
        """Extraction binds every cutoff and returns the client's frame."""
        user_ids = [1, 2, 3]
        as_of = datetime(2024, 1, 31, 12, 0, 0)

        result = extractor.extract_user_features(user_ids, as_of)

        assert result.height == 3
        assert result.columns == ["user_id", "bets_7d"]

        query, params = extractor.ch.query_to_polars.call_args.args
        assert params["user_ids"] == "1,2,3"
        assert params["as_of"] == as_of.isoformat()
        assert params["cutoff_24h"] == (as_of - timedelta(hours=24)).isoformat()
        assert params["cutoff_7d"] == (as_of - timedelta(days=7)).isoformat()
        assert params["cutoff_30d"] == (as_of - timedelta(days=30)).isoformat()
        assert "user_id IN ({user_ids})" in query

    def test_extract_user_features_empty_input_skips_query(self, extractor):
        """An empty id list short-circuits rather than sending broken SQL."""
        result = extractor.extract_user_features([], datetime(2024, 1, 31))

        assert result.is_empty()
        extractor.ch.query_to_polars.assert_not_called()

    def test_compute_derived_features(self, extractor):
        """Test derived feature computation."""
        # compute_derived_features reads every raw feature it derives from,
        # so the frame has to carry all of them: omitting total_deposit_30d,
        # bets_7d, max_bet_30d or bets_24d made polars raise ColumnNotFound and
        # the test never reached a single assertion.
        df = pl.DataFrame(
            {
                "user_id": [1, 2],
                "wins_7d": [5, 10],
                "settled_7d": [10, 20],
                "bets_7d": [20, 40],
                "bets_24h": [2, 30],
                "std_bet_30d": [50.0, 100.0],
                "avg_bet_30d": [100.0, 200.0],
                "max_bet_30d": [500.0, 4000.0],
                "total_deposit_30d": [1000.0, 300.0],
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

        # Coefficient of variation: std / avg, guarding avg at 0.01
        assert derived["bet_cv_30d"][0] == pytest.approx(50.0 / 100.0)
        assert derived["bet_cv_30d"][1] == pytest.approx(100.0 / 200.0)

        # Deposit-to-bet ratio, scaled to a weekly basis
        expected_ratio = 1000.0 / 100.0 / 20 * 7
        assert derived["deposit_bet_ratio"][0] == pytest.approx(expected_ratio)

        # High roller: max bet more than 10x the average
        assert derived["high_roller"][0] == 0  # 500 is not > 1000
        assert derived["high_roller"][1] == 1  # 4000 is > 2000

        # Rapid bettor: more than half the week's bets landed in one day
        assert derived["rapid_bettor"][0] == 0  # 2 is not > 10
        assert derived["rapid_bettor"][1] == 1  # 30 is > 20

    def test_extract_training_data(self, extractor):
        """Test training data extraction."""
        # This would require a real ClickHouse connection
        # df = extractor.extract_training_data(lookback_days=90)
        # assert len(df) > 0
        # assert "is_fraud" in df.columns
        pass
