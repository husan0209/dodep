"""Tests for fraud detection model."""
import pytest
import polars as pl
import numpy as np

from src.models.fraud_model import FraudModel


class TestFraudModel:
    """Test fraud detection model."""
    
    @pytest.fixture
    def sample_data(self):
        """Create sample training data."""
        np.random.seed(42)
        n_samples = 1000
        
        data = {
            "bets_7d": np.random.randint(0, 50, n_samples),
            "bets_24h": np.random.randint(0, 20, n_samples),
            "avg_bet_30d": np.random.uniform(10, 500, n_samples),
            "std_bet_30d": np.random.uniform(5, 200, n_samples),
            "max_bet_30d": np.random.uniform(100, 5000, n_samples),
            "deposits_24h": np.random.randint(0, 10, n_samples),
            "total_deposit_30d": np.random.uniform(0, 10000, n_samples),
            "device_count_30d": np.random.randint(1, 10, n_samples),
            "ip_count_30d": np.random.randint(1, 50, n_samples),
            "country_count_30d": np.random.randint(1, 5, n_samples),
            "wins_7d": np.random.randint(0, 20, n_samples),
            "settled_7d": np.random.randint(0, 50, n_samples),
            "account_age_days": np.random.randint(1, 1000, n_samples),
            "win_rate_7d": np.random.uniform(0, 1, n_samples),
            "bet_cv_30d": np.random.uniform(0, 5, n_samples),
            "deposit_bet_ratio": np.random.uniform(0, 10, n_samples),
            "multi_device": np.random.randint(0, 2, n_samples),
            "multi_ip": np.random.randint(0, 2, n_samples),
            "high_roller": np.random.randint(0, 2, n_samples),
            "rapid_bettor": np.random.randint(0, 2, n_samples),
            "is_fraud": np.random.choice([0, 1], n_samples, p=[0.95, 0.05]),
        }
        
        return pl.DataFrame(data)
    
    @pytest.fixture
    def model(self):
        """Create fraud model."""
        return FraudModel()
    
    def test_model_initialization(self, model):
        """Test model initializes correctly."""
        assert model.model is None
        assert len(model.FEATURE_COLUMNS) == 18
        assert model.TARGET_COLUMN == "is_fraud"
    
    def test_model_training(self, model, sample_data):
        """Test model training."""
        metrics = model.train(sample_data)
        
        assert "auc_roc" in metrics
        assert "precision_at_90_recall" in metrics
        assert metrics["auc_roc"] > 0.5  # Better than random
        assert metrics["samples_total"] > 0
    
    def test_model_prediction(self, model, sample_data):
        """Test model prediction."""
        # Train first
        model.train(sample_data)
        
        # Predict
        X = sample_data.select(model.FEATURE_COLUMNS).to_numpy()
        predictions = model.predict(X)
        
        assert len(predictions) == len(sample_data)
        assert all(0 <= p <= 1 for p in predictions)
    
    def test_model_feature_importance(self, model, sample_data):
        """Test feature importance."""
        model.train(sample_data)
        importance = model.get_feature_importance()
        
        assert len(importance) == len(model.FEATURE_COLUMNS)
        assert all(0 <= v <= 1 for v in importance.values())
    
    def test_model_save_load(self, model, sample_data, tmp_path):
        """Test model save and load."""
        # Train
        model.train(sample_data)
        
        # Save
        model.save(tmp_path)
        assert (tmp_path / "fraud_model.json").exists()
        
        # Load
        new_model = FraudModel()
        new_model.load(tmp_path)
        
        # Compare predictions
        X = sample_data.select(model.FEATURE_COLUMNS).to_numpy()
        orig_preds = model.predict(X)
        new_preds = new_model.predict(X)
        
        assert np.allclose(orig_preds, new_preds)
