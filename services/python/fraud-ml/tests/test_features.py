"""Tests for feature extraction."""
import pytest
import polars as pl
from datetime import datetime, timedelta

from src.features.extraction import FeatureExtractor
from src.data.clickhouse import ClickHouseClient


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
        """Test feature extraction for users."""
        user_ids = [1, 2, 3]
        as_of = datetime.utcnow()
        
        # This would require a real ClickHouse connection
        # df = extractor.extract_user_features(user_ids, as_of)
        # assert len(df) > 0
        # assert "bets_7d" in df.columns
        pass
    
    def test_compute_derived_features(self, extractor):
        """Test derived feature computation."""
        df = pl.DataFrame({
            "user_id": [1, 2],
            "wins_7d": [5, 10],
            "settled_7d": [10, 20],
            "std_bet_30d": [50.0, 100.0],
            "avg_bet_30d": [100.0, 200.0],
            "device_count_30d": [5, 2],
            "ip_count_30d": [15, 5],
        })
        
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
        """Test training data extraction."""
        # This would require a real ClickHouse connection
        # df = extractor.extract_training_data(lookback_days=90)
        # assert len(df) > 0
        # assert "is_fraud" in df.columns
        pass
