"""Tests for fraud detection model."""

import numpy as np
import polars as pl
import pytest

from src.features.transformation import MODEL_FEATURES
from src.models.fraud_model import FraudModel


class TestFraudModel:
    """Test fraud detection model."""

    @pytest.fixture
    def sample_data(self):
        """Create sample training data with a learnable fraud signal."""
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
        }

        # The label must be a function of the features. With a uniformly random
        # label the rows carry no signal, so any trained model scores AUC ~0.5
        # and "better than random" is unsatisfiable by construction.
        # Fraud correlates with multi-device + multi-IP + high-roller behaviour,
        # which is the pattern the feature set is designed to capture.
        risk = data["multi_device"] + data["multi_ip"] + data["high_roller"] + data["rapid_bettor"]
        data["is_fraud"] = (risk >= 3).astype(int)
        # Keep the positive class in the low single digits of percent.
        if data["is_fraud"].mean() > 0.10:
            data["is_fraud"] = (risk >= 4).astype(int)

        return pl.DataFrame(data)

    @pytest.fixture
    def model(self):
        """Create fraud model."""
        return FraudModel()

    def test_model_initialization(self, model):
        """Test model initializes correctly."""
        assert model.model is None
        # Compare against the source of truth instead of a hardcoded count:
        # a literal 18 silently rotted when features were added (it is 20 now).
        assert model.FEATURE_COLUMNS == MODEL_FEATURES
        assert len(model.FEATURE_COLUMNS) > 0
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
        x = sample_data.select(model.FEATURE_COLUMNS).to_numpy()
        predictions = model.predict(x)

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
        x = sample_data.select(model.FEATURE_COLUMNS).to_numpy()
        orig_preds = model.predict(x)
        new_preds = new_model.predict(x)

        assert np.allclose(orig_preds, new_preds)
