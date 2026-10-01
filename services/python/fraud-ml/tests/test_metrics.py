"""Tests for evaluation metrics."""

import numpy as np
import pytest

from src.evaluation.metrics import (
    calculate_metrics,
    compare_models,
    select_threshold_constrained,
    validate_quality_gates,
)


def _make_predictions(n_samples: int = 5000, positive_rate: float = 0.05, seed: int = 42):
    """Imbalanced predictions with real signal (fraud scores shifted up)."""
    rng = np.random.default_rng(seed)
    y_true = (rng.random(n_samples) < positive_rate).astype(int)
    scores = rng.normal(0.3, 0.1, n_samples)
    scores[y_true == 1] = rng.normal(0.7, 0.1, y_true.sum())
    return y_true, np.clip(scores, 0, 1)


def _brute_force_best(y_true, y_pred, max_fpr):
    """Reference implementation: scan every distinct threshold."""
    best_recall, best_fpr = 0.0, 1.0
    for thr in np.unique(y_pred):
        pred = (y_pred >= thr).astype(int)
        fp = int(np.sum((y_true == 0) & (pred == 1)))
        tn = int(np.sum((y_true == 0) & (pred == 0)))
        tp = int(np.sum((y_true == 1) & (pred == 1)))
        fn = int(np.sum((y_true == 1) & (pred == 0)))
        fpr = fp / (fp + tn)
        recall = tp / (tp + fn)
        if fpr <= max_fpr + 1e-12 and recall > best_recall:
            best_recall, best_fpr = recall, fpr
    return best_recall, best_fpr


class TestSelectThresholdConstrained:
    """Neyman-Pearson threshold selection."""

    def test_respects_fpr_budget_and_maximizes_recall(self):
        y_true, y_pred = _make_predictions()
        selection = select_threshold_constrained(y_true, y_pred, max_fpr=0.05)

        assert selection["fpr"] <= 0.05 + 1e-12
        assert 0.0 <= selection["threshold"] <= 1.0
        assert 0.0 <= selection["recall"] <= 1.0

        # Must match the brute-force optimum over all distinct thresholds.
        best_recall, _ = _brute_force_best(y_true, y_pred, max_fpr=0.05)
        assert selection["recall"] == pytest.approx(best_recall, abs=1e-9)

    def test_tighter_budget_never_increases_recall(self):
        y_true, y_pred = _make_predictions()
        loose = select_threshold_constrained(y_true, y_pred, max_fpr=0.10)
        tight = select_threshold_constrained(y_true, y_pred, max_fpr=0.01)

        assert tight["fpr"] <= 0.01 + 1e-12
        assert loose["fpr"] <= 0.10 + 1e-12
        assert tight["recall"] <= loose["recall"]
        assert tight["threshold"] >= loose["threshold"]

    def test_perfect_separation(self):
        y_true = np.array([0] * 95 + [1] * 5)
        y_pred = np.array([0.1] * 95 + [0.9] * 5)
        selection = select_threshold_constrained(y_true, y_pred, max_fpr=0.05)

        assert selection["recall"] == pytest.approx(1.0)
        assert selection["fpr"] == pytest.approx(0.0)
        assert selection["threshold"] == pytest.approx(0.9)

    def test_inverted_scorer_stays_within_budget(self):
        """A model that ranks fraud lowest must flag nothing, not everything.

        The FPR budget is a hard constraint: when no threshold separates the
        classes within budget, the selector must fall back to the
        no-positives point instead of clamping the threshold down to the top
        score (which would flag a negative and blow the budget).
        """
        y_true = np.array([0] * 90 + [1] * 10)
        y_pred = np.concatenate([np.full(90, 0.9), np.full(10, 0.1)])
        selection = select_threshold_constrained(y_true, y_pred, max_fpr=0.05)

        assert selection["fpr"] <= 0.05 + 1e-12
        assert selection["false_positives"] == 0
        assert selection["recall"] == 0.0

    def test_budget_infeasible_for_any_split(self):
        """Tiny budget: selector returns a feasible point, never over budget."""
        rng = np.random.default_rng(3)
        y_true = rng.integers(0, 2, 200)
        y_pred = rng.random(200)

        for budget in (0.0 + 1e-6, 0.01, 0.05):
            selection = select_threshold_constrained(y_true, y_pred, max_fpr=budget)
            assert selection["fpr"] <= budget + 1e-12

    def test_confusion_counts_consistent(self):
        y_true, y_pred = _make_predictions()
        selection = select_threshold_constrained(y_true, y_pred, max_fpr=0.05)

        total = (
            selection["true_positives"]
            + selection["false_positives"]
            + selection["true_negatives"]
            + selection["false_negatives"]
        )
        assert total == len(y_true)
        assert selection["fnr"] == pytest.approx(1.0 - selection["recall"])

    def test_invalid_inputs_raise(self):
        with pytest.raises(ValueError, match="single class"):
            select_threshold_constrained(np.zeros(10), np.random.rand(10))

        with pytest.raises(ValueError, match="same length"):
            select_threshold_constrained(np.array([0, 1]), np.array([0.5]))

        with pytest.raises(ValueError, match="Empty"):
            select_threshold_constrained(np.array([]), np.array([]))

        with pytest.raises(ValueError, match="max_fpr"):
            select_threshold_constrained(
                np.array([0, 1, 0, 1]), np.array([0.1, 0.9, 0.2, 0.8]), max_fpr=0.0
            )

    def test_invalid_labels_raise(self):
        with pytest.raises(ValueError, match="binary"):
            select_threshold_constrained(np.array([0, 2, 1, 0]), np.array([0.1, 0.4, 0.6, 0.2]))


class TestCalculateMetrics:
    """Comprehensive metric calculation."""

    @pytest.fixture
    def sample_predictions(self):
        return _make_predictions()

    def test_calculate_metrics(self, sample_predictions):
        y_true, y_pred_proba = sample_predictions
        metrics = calculate_metrics(y_true, y_pred_proba)

        for key in (
            "auc_roc",
            "avg_precision",
            "brier_score",
            "precision_at_90_recall",
            "f1_score",
            "threshold",
            "recall_at_max_fpr",
            "precision_at_max_fpr",
            "fpr_at_max_fpr",
            "threshold_at_target_recall",
        ):
            assert key in metrics, f"missing metric: {key}"

        assert 0 <= metrics["auc_roc"] <= 1
        assert 0 <= metrics["precision_at_90_recall"] <= 1
        # Operating point must respect the FPR budget.
        assert metrics["fpr_at_max_fpr"] <= metrics["max_fpr"] + 1e-12
        # Positive signal should beat random ranking.
        assert metrics["auc_roc"] > 0.8
        assert metrics["brier_score"] < 0.5
        # Distribution bookkeeping.
        assert metrics["samples_positive"] + metrics["samples_negative"] == metrics["samples_total"]
        assert metrics["threshold_at_target_recall"] in (0, pytest.approx(0, abs=1)) or (
            0 < metrics["threshold_at_target_recall"] <= 1
        )

    def test_metrics_reject_single_class(self):
        with pytest.raises(ValueError, match="single class"):
            calculate_metrics(np.zeros(100), np.random.rand(100))

    def test_target_recall_reached(self):
        y_true, y_pred = _make_predictions()
        metrics = calculate_metrics(y_true, y_pred, target_recall=0.90)
        # By construction the target-recall point reaches >= 0.90 recall.
        assert metrics["recall_at_90_recall"] >= 0.90


class TestQualityGates:
    """Quality gate validation."""

    @pytest.fixture
    def metrics(self):
        y_true, y_pred = _make_predictions()
        return calculate_metrics(y_true, y_pred)

    def test_quality_gates_pass(self, metrics):
        passed, failures = validate_quality_gates(
            metrics,
            auc_threshold=0.5,
            precision_threshold=0.01,
            max_fpr=0.05,
            min_recall_at_max_fpr=0.1,
        )
        assert passed is True
        assert failures == []

    def test_quality_gates_fail(self, metrics):
        passed, failures = validate_quality_gates(
            metrics,
            auc_threshold=0.99,
            precision_threshold=0.99,
            max_fpr=0.05,
            min_recall_at_max_fpr=0.1,
        )
        assert passed is False
        assert len(failures) > 0

    def test_fpr_budget_gate(self):
        metrics = {
            "auc_roc": 0.95,
            "fpr_at_max_fpr": 0.20,  # violates 5% budget
            "recall_at_max_fpr": 0.90,
            "precision_at_90_recall": 0.80,
        }
        passed, failures = validate_quality_gates(metrics)
        assert passed is False
        assert any("FPR" in f for f in failures)

    def test_recall_gate(self):
        metrics = {
            "auc_roc": 0.95,
            "fpr_at_max_fpr": 0.04,
            "recall_at_max_fpr": 0.30,  # too low within budget
            "precision_at_90_recall": 0.80,
        }
        passed, failures = validate_quality_gates(metrics)
        assert passed is False
        assert any("Recall" in f for f in failures)


class TestCompareModels:
    """Test model comparison."""

    def test_compare_models(self):
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
        assert comparison["f1_score"]["improved"] is True
