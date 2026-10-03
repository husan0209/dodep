"""Tests for fraud detection model."""

import json

import numpy as np
import polars as pl
import pytest

from src.features.transformation import MODEL_FEATURES
from src.models.fraud_model import MODEL_CARD_FILE, FraudModel


class TestFraudModel:
    """Test fraud detection model."""

    @pytest.fixture
    def sample_data(self, fraud_data):
        return fraud_data

    @pytest.fixture
    def model(self):
        return FraudModel()

    def test_model_initialization(self, model):
        assert model.model is None
        assert len(model.FEATURE_COLUMNS) == len(MODEL_FEATURES)
        assert model.TARGET_COLUMN == "is_fraud"
        assert model.threshold == 0.5  # placeholder until train()/load()

    def test_model_training(self, model, sample_data):
        metrics = model.train(sample_data)

        for key in (
            "auc_roc",
            "precision_at_90_recall",
            "recall_at_max_fpr",
            "fpr_at_max_fpr",
            "threshold",
            "threshold_selected_on",
            "split",
            "validation_selection",
            "test_at_production_threshold",
        ):
            assert key in metrics, f"missing metric: {key}"

        # Synthetic signal is learnable — must clearly beat random ranking.
        assert metrics["auc_roc"] > 0.7
        assert metrics["samples_total"] > 0
        assert metrics["threshold_selected_on"] == "validation"
        assert 0.0 < metrics["threshold"] <= 1.0
        # FPR-constrained selection respects the budget on its own split.
        assert metrics["fpr_at_max_fpr"] <= metrics["max_fpr"] + 1e-12

        # Temporal split with embargo: partitions are disjoint and cover
        # less than the full dataset (embargo rows are dropped).
        split = metrics["split"]
        assert split["train"] > 0 and split["valid"] > 0 and split["test"] > 0
        assert split["train"] + split["valid"] + split["test"] <= len(sample_data)

    def test_scale_pos_weight_computed_from_data(self, model, sample_data):
        metrics = model.train(sample_data)
        spw = metrics["params"]["scale_pos_weight"]
        # ~10% fraud rate => ratio of negatives to positives ~9, not the
        # legacy hard-coded 10; just assert it was computed and is sane.
        assert isinstance(spw, float)
        assert 1.0 < spw < 50.0

    def test_model_prediction(self, model, sample_data):
        model.train(sample_data)

        features = sample_data.select(model.FEATURE_COLUMNS).to_numpy()
        predictions = model.predict(features)

        assert len(predictions) == len(sample_data)
        assert all(0 <= p <= 1 for p in predictions)

    def test_predict_with_production_threshold(self, model, sample_data):
        model.train(sample_data)
        features = sample_data.select(model.FEATURE_COLUMNS).to_numpy()

        labels = model.predict_with_threshold(features)
        proba = model.predict(features)

        # Consistency: labels are exactly proba >= threshold.
        assert np.array_equal(labels, (proba >= model.threshold).astype(int))
        # Explicit threshold overrides the production one.
        strict = model.predict_with_threshold(features, threshold=0.99)
        assert strict.sum() <= labels.sum()

    def test_temporal_split_is_time_ordered(self, sample_data):
        model = FraudModel()
        mask_train, mask_valid, mask_test = model._temporal_split(sample_data)

        times = sample_data["event_time"].to_numpy()
        assert times[mask_train].max() <= times[mask_valid].min()
        assert times[mask_valid].max() <= times[mask_test].min()
        # No row appears in two splits.
        assert not (mask_train & mask_valid).any()
        assert not (mask_valid & mask_test).any()
        assert not (mask_train & mask_test).any()

    def test_model_feature_importance(self, model, sample_data):
        model.train(sample_data)
        importance = model.get_feature_importance()

        assert len(importance) == len(model.FEATURE_COLUMNS)
        assert all(0 <= v <= 1 for v in importance.values())
        # rapid_bettor/devices drive the synthetic fraud signal — the model
        # must assign at least some importance to signal-bearing features.
        assert sum(importance.values()) > 0

    def test_model_save_load(self, model, sample_data, tmp_path):
        model.train(sample_data)
        model.save(tmp_path, metrics={"auc_roc": 0.99})

        assert (tmp_path / "fraud_model.json").exists()
        assert (tmp_path / MODEL_CARD_FILE).exists()

        new_model = FraudModel()
        new_model.load(tmp_path)

        # Threshold and settings restored from the model card.
        assert new_model.threshold == model.threshold
        assert new_model.max_fpr == model.max_fpr
        assert new_model.target_recall == model.target_recall

        features = sample_data.select(model.FEATURE_COLUMNS).to_numpy()
        orig_preds = model.predict(features)
        new_preds = new_model.predict(features)
        assert np.allclose(orig_preds, new_preds)

    def test_load_rejects_feature_order_mismatch(self, model, sample_data, tmp_path):
        model.train(sample_data)
        model.save(tmp_path)

        card_path = tmp_path / MODEL_CARD_FILE
        card = json.loads(card_path.read_text(encoding="utf-8"))
        card["feature_columns"] = list(reversed(card["feature_columns"]))
        card_path.write_text(json.dumps(card), encoding="utf-8")

        with pytest.raises(ValueError, match="feature order"):
            FraudModel().load(tmp_path)

    def test_load_without_card_falls_back_to_default_threshold(self, model, sample_data, tmp_path):
        model.train(sample_data)
        model.save(tmp_path)
        (tmp_path / MODEL_CARD_FILE).unlink()

        new_model = FraudModel()
        new_model.load(tmp_path)
        assert new_model.threshold == 0.5  # documented fallback

    def test_reproducibility_with_fixed_seed(self, sample_data):
        first = FraudModel()
        first.train(sample_data)

        second = FraudModel()
        second.train(sample_data)

        features = sample_data.select(first.FEATURE_COLUMNS).to_numpy()
        assert np.allclose(first.predict(features), second.predict(features), atol=1e-6)
        assert first.threshold == pytest.approx(second.threshold, abs=1e-6)

    def test_train_rejects_missing_columns(self, model, sample_data):
        broken = sample_data.drop("bets_7d")
        with pytest.raises(ValueError, match="Missing columns"):
            model.train(broken)

    def test_train_rejects_single_class_train_split(self):
        model = FraudModel()
        n = 100
        data = {f: np.zeros(n) for f in MODEL_FEATURES}
        data["is_fraud"] = np.zeros(n, dtype=int)
        data["event_time"] = pl.Series("event_time", list(range(n)), dtype=pl.Int64)
        with pytest.raises(ValueError, match="single-class"):
            model.train(pl.DataFrame(data))
