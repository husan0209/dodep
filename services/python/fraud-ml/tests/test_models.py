"""Tests for fraud detection model."""

import numpy as np
import polars as pl
import pytest

from src.features.transformation import FEATURE_REGISTRY
from src.models.fraud_model import FraudModel


class TestFraudModel:
    """Test fraud detection model."""

    @pytest.fixture
    def sample_data(self):
        """Create sample training data."""
        np.random.seed(42)
        n_samples = 1000

        labels = np.random.choice([0, 1], n_samples, p=[0.95, 0.05])
        fraud = labels == 1
        n_fraud = int(fraud.sum())

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
            "is_fraud": labels,
        }

        # Give the fraud rows a behavioural signature: brand-new accounts,
        # bursts of bets, many devices and IPs, erratic stake sizes, almost no
        # wins.
        #
        # Without this the label was drawn independently of every feature, so
        # there was nothing to learn and `auc_roc > 0.5` in
        # test_model_training was unreachable - XGBoost returned 0.47.
        data["account_age_days"][fraud] = np.random.randint(1, 30, n_fraud)
        data["bets_24h"][fraud] = np.random.randint(20, 40, n_fraud)
        data["bets_7d"][fraud] = np.random.randint(45, 80, n_fraud)
        data["device_count_30d"][fraud] = np.random.randint(8, 20, n_fraud)
        data["ip_count_30d"][fraud] = np.random.randint(30, 80, n_fraud)
        data["bet_cv_30d"][fraud] = np.random.uniform(3, 5, n_fraud)
        data["win_rate_7d"][fraud] = np.random.uniform(0, 0.2, n_fraud)
        data["multi_device"][fraud] = 1
        data["multi_ip"][fraud] = 1
        data["rapid_bettor"][fraud] = 1

        return pl.DataFrame(data)

    @pytest.fixture
    def model(self):
        """Create fraud model."""
        return FraudModel()

    def test_model_initialization(self, model):
        """Test model initializes correctly."""
        assert model.model is None
        # Derived from the registry instead of a hardcoded count: FEATURE_COLUMNS
        # is MODEL_FEATURES, and the registry grew to 20 entries while this spec
        # still expected 18.
        assert model.FEATURE_COLUMNS == list(FEATURE_REGISTRY)
        assert len(set(model.FEATURE_COLUMNS)) == len(model.FEATURE_COLUMNS)
        assert model.TARGET_COLUMN == "is_fraud"
        assert model.TARGET_COLUMN not in model.FEATURE_COLUMNS

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
