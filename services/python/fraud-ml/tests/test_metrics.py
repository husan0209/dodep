"""Tests for evaluation metrics."""

import numpy as np
import pytest

from src.evaluation.metrics import (
    calculate_metrics,
    compare_models,
    validate_quality_gates,
)


class TestMetrics:
    """Test evaluation metrics."""

    @pytest.fixture
    def sample_predictions(self):
        """Create sample predictions."""
        np.random.seed(42)
        n_samples = 1000

        # Create imbalanced data (5% fraud)
        y_true = np.random.choice([0, 1], n_samples, p=[0.95, 0.05])

        # Predictions with clear but imperfect separation: fraud scores high,
        # honest players score low, with enough overlap that precision at a 90%
        # recall target lands between the 0.1 threshold the passing case asserts
        # and the 0.99 threshold the failing case asserts.
        #
        # The old `y_pred_proba[y_true == 1] += 0.3` shift left the two
        # populations interleaved, so precision@90R came out at 0.066 and
        # test_quality_gates_pass failed on its own fixture.
        y_pred_proba = np.where(
            y_true == 1,
            np.random.uniform(0.5, 1.0, n_samples),
            np.random.uniform(0.0, 0.6, n_samples),
        )

        return y_true, y_pred_proba

    def test_calculate_metrics(self, sample_predictions):
        """Test metric calculation."""
        y_true, y_pred_proba = sample_predictions
        metrics = calculate_metrics(y_true, y_pred_proba)

        assert "auc_roc" in metrics
        assert "precision_at_90_recall" in metrics
        assert "f1_score" in metrics
        assert 0 <= metrics["auc_roc"] <= 1
        assert 0 <= metrics["precision_at_90_recall"] <= 1

    def test_quality_gates_pass(self, sample_predictions):
        """Test quality gates pass."""
        y_true, y_pred_proba = sample_predictions
        metrics = calculate_metrics(y_true, y_pred_proba)

        # Use lenient thresholds for test
        passed, failures = validate_quality_gates(
            metrics,
            auc_threshold=0.5,
            precision_threshold=0.1,
        )

        # With lenient thresholds, should pass
        assert passed is True
        assert len(failures) == 0

    def test_quality_gates_fail(self, sample_predictions):
        """Test quality gates fail."""
        y_true, y_pred_proba = sample_predictions
        metrics = calculate_metrics(y_true, y_pred_proba)

        # Use strict thresholds
        passed, failures = validate_quality_gates(
            metrics,
            auc_threshold=0.99,
            precision_threshold=0.99,
        )

        # With strict thresholds, should fail
        assert passed is False
        assert len(failures) > 0

    def test_compare_models(self):
        """Test model comparison."""
        metrics_a = {
            "auc_roc": 0.85,
            "precision_at_90_recall": 0.60,
            "f1_score": 0.65,
        }
        metrics_b = {
            "auc_roc": 0.90,
            "precision_at_90_recall": 0.70,
            "f1_score": 0.72,
        }

        comparison = compare_models(metrics_a, metrics_b)

        assert "auc_roc" in comparison
        assert comparison["auc_roc"]["improved"] is True
        assert comparison["auc_roc"]["diff"] > 0
