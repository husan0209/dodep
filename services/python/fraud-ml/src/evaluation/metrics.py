"""
Model evaluation metrics for fraud detection.

Operating-point selection follows the Neyman-Pearson approach used by
production fraud systems: pick the threshold that *maximizes recall subject
to a hard false-positive-rate constraint* (FPR <= max_fpr) instead of a
fixed 0.5 cut-off.

Rationale:
- Fraud base rate is ~1%, so a 0.5 threshold misses nearly everything;
- an unconstrained "recall = 90%" threshold can flood the manual review
  queue with false positives (tasks/ТЗ.md requires FPR < 5% acceptance
  and defines hold-for-review / block score bands);
- the review queue has finite capacity, so FPR is the real budget.

Threshold is selected on the *validation* split (see FraudModel.train)
and only *reported* on the test split — never chosen on test data.
"""

import numpy as np
import structlog
from sklearn.metrics import (
    average_precision_score,
    brier_score_loss,
    classification_report,
    precision_recall_curve,
    roc_auc_score,
    roc_curve,
)

logger = structlog.get_logger()

__all__ = [
    "select_threshold_constrained",
    "calculate_metrics",
    "compare_models",
    "validate_quality_gates",
]


def _validate_inputs(y_true: np.ndarray, y_pred_proba: np.ndarray) -> None:
    """Fail fast on inputs that would silently produce garbage metrics."""
    y_true = np.asarray(y_true)
    y_pred_proba = np.asarray(y_pred_proba)

    if y_true.shape[0] != y_pred_proba.shape[0]:
        raise ValueError(
            f"y_true ({y_true.shape[0]}) and y_pred_proba ({y_pred_proba.shape[0]}) "
            "must have the same length"
        )
    if y_true.shape[0] == 0:
        raise ValueError("Empty inputs: cannot calculate metrics")
    unique = np.unique(y_true)
    if not np.isin(unique, [0, 1]).all():
        raise ValueError(f"y_true must be binary (0/1), got {unique.tolist()}")
    if unique.size < 2:
        raise ValueError(
            "y_true contains a single class; ROC/PR metrics are undefined. "
            "Check label extraction (fraud_signals join)."
        )


def _counts(y_true: np.ndarray, y_pred: np.ndarray) -> dict:
    """Confusion-matrix counts with guarded precision/recall."""
    tp = int(np.sum((y_true == 1) & (y_pred == 1)))
    fp = int(np.sum((y_true == 0) & (y_pred == 1)))
    tn = int(np.sum((y_true == 0) & (y_pred == 0)))
    fn = int(np.sum((y_true == 1) & (y_pred == 0)))

    precision = tp / (tp + fp) if (tp + fp) > 0 else 0.0
    recall = tp / (tp + fn) if (tp + fn) > 0 else 0.0
    fpr = fp / (fp + tn) if (fp + tn) > 0 else 0.0
    fnr = fn / (fn + tp) if (fn + tp) > 0 else 0.0
    f1 = 2 * precision * recall / (precision + recall) if (precision + recall) > 0 else 0.0

    return {
        "true_positives": tp,
        "false_positives": fp,
        "true_negatives": tn,
        "false_negatives": fn,
        "precision": float(precision),
        "recall": float(recall),
        "fpr": float(fpr),
        "fnr": float(fnr),
        "f1_score": float(f1),
    }


def select_threshold_constrained(
    y_true: np.ndarray,
    y_pred_proba: np.ndarray,
    max_fpr: float = 0.05,
) -> dict:
    """
    Select the operating threshold maximizing recall subject to FPR <= max_fpr.

    ROC points are generated with thresholds in decreasing order, so both
    FPR and TPR are non-decreasing along the curve; the feasible set is a
    prefix, and the last feasible point maximizes recall under the budget.

    Returns:
        dict with threshold, recall, precision, fpr, fnr and confusion counts.
    """
    _validate_inputs(y_true, y_pred_proba)
    if not 0.0 < max_fpr <= 1.0:
        raise ValueError(f"max_fpr must be in (0, 1], got {max_fpr}")

    y_true = np.asarray(y_true).astype(int)
    fpr, tpr, thresholds = roc_curve(y_true, np.asarray(y_pred_proba))

    feasible = np.nonzero(fpr <= max_fpr)[0]
    # Index 0 always exists: roc_curve starts with fpr = 0, tpr = 0, so the
    # feasible set is never empty.
    best = int(feasible[-1])
    threshold = float(thresholds[best])

    # Do NOT clamp the threshold into the observed score range: when no
    # score exceeds it, `score >= threshold` correctly yields no positives
    # (fpr = 0). Clamping to max(score) would flag that single top-scored
    # row and could violate the FPR budget.
    scores = np.asarray(y_pred_proba)
    y_pred = (scores >= threshold).astype(int)
    counts = _counts(y_true, y_pred)

    return {
        "threshold": threshold,
        "max_fpr": float(max_fpr),
        # Recall/precision come from the confusion counts (not the ROC
        # array) so they always match the reported threshold.
        "recall": counts["recall"],
        "precision": counts["precision"],
        "fpr": counts["fpr"],
        "fnr": counts["fnr"],
        **{
            k: counts[k]
            for k in (
                "true_positives",
                "false_positives",
                "true_negatives",
                "false_negatives",
            )
        },
    }


def _threshold_at_recall(
    y_true: np.ndarray,
    y_pred_proba: np.ndarray,
    target_recall: float,
) -> tuple[float, float, float]:
    """
    Lowest-complexity helper: highest threshold still achieving
    recall >= target_recall (max precision among feasible thresholds).

    Returns (threshold, precision, recall).
    """
    precision, recall, thresholds = precision_recall_curve(y_true, y_pred_proba)
    # precision/recall have len == len(thresholds) + 1; the last element
    # corresponds to "predict everything positive" and has no threshold.
    candidates = np.nonzero(recall[:-1] >= target_recall)[0]
    if candidates.size == 0:
        # Model cannot reach the target recall — fall back to the
        # highest-recall point (lowest threshold).
        idx = int(np.argmax(recall[:-1]))
    else:
        idx = int(candidates[-1])
    return float(thresholds[idx]), float(precision[idx]), float(recall[idx])


def calculate_metrics(
    y_true: np.ndarray,
    y_pred_proba: np.ndarray,
    target_recall: float = 0.90,
    max_fpr: float = 0.05,
) -> dict:
    """
    Calculate comprehensive fraud detection metrics.

    Args:
        y_true: True labels (0 or 1).
        y_pred_proba: Predicted fraud probabilities.
        target_recall: Recall target for the reporting operating point
            (precision_at_{pct}_recall key).
        max_fpr: Hard FPR budget for the production operating point.

    Returns:
        Dictionary with discrimination, operating-point and distribution
        metrics. `threshold` is the FPR-constrained production threshold.
    """
    _validate_inputs(y_true, y_pred_proba)
    y_true = np.asarray(y_true).astype(int)
    y_pred_proba = np.asarray(y_pred_proba)

    # Discrimination (threshold-independent)
    auc = roc_auc_score(y_true, y_pred_proba)
    ap = average_precision_score(y_true, y_pred_proba)
    brier = brier_score_loss(y_true, y_pred_proba)

    # Production operating point: max recall under FPR budget
    op = select_threshold_constrained(y_true, y_pred_proba, max_fpr=max_fpr)

    # Reporting point: precision achieved at the target recall
    thr_target, precision_target, recall_target = _threshold_at_recall(
        y_true, y_pred_proba, target_recall
    )
    target_counts = _counts(y_true, (y_pred_proba >= thr_target).astype(int))

    # Classification report at the production threshold
    y_pred = (y_pred_proba >= op["threshold"]).astype(int)
    report = classification_report(y_true, y_pred, output_dict=True, zero_division=0)

    # ROC curve (sampled for storage)
    fpr_curve, tpr_curve, roc_thresholds = roc_curve(y_true, y_pred_proba)

    metrics = {
        # Discrimination
        "auc_roc": float(auc),
        "avg_precision": float(ap),
        "brier_score": float(brier),
        # Production operating point (FPR-constrained)
        "threshold": op["threshold"],
        "max_fpr": op["max_fpr"],
        "recall_at_max_fpr": op["recall"],
        "precision_at_max_fpr": op["precision"],
        "fpr_at_max_fpr": op["fpr"],
        "fnr_at_max_fpr": op["fnr"],
        "true_positives": op["true_positives"],
        "false_positives": op["false_positives"],
        "true_negatives": op["true_negatives"],
        "false_negatives": op["false_negatives"],
        # Target-recall reporting point
        f"precision_at_{target_recall * 100:g}_recall": precision_target,
        f"recall_at_{target_recall * 100:g}_recall": recall_target,
        f"fpr_at_{target_recall * 100:g}_recall": target_counts["fpr"],
        "threshold_at_target_recall": thr_target,
        # Weighted classification metrics at the production threshold
        "precision": float(report["weighted avg"]["precision"]),
        "recall": float(report["weighted avg"]["recall"]),
        "f1_score": float(report["weighted avg"]["f1-score"]),
        # Class distribution
        "samples_total": int(len(y_true)),
        "samples_positive": int(y_true.sum()),
        "samples_negative": int(len(y_true) - y_true.sum()),
        "positive_rate": float(y_true.mean()),
        # ROC curve (sampled for storage)
        "roc_curve": {
            "fpr": fpr_curve[::10].tolist(),
            "tpr": tpr_curve[::10].tolist(),
            "thresholds": roc_thresholds[::10].tolist(),
        },
    }

    logger.info(
        "metrics.calculated",
        auc=round(auc, 4),
        ap=round(ap, 4),
        threshold=round(op["threshold"], 4),
        recall_at_max_fpr=round(op["recall"], 4),
        fpr_at_max_fpr=round(op["fpr"], 4),
    )

    return metrics


def compare_models(
    metrics_a: dict,
    metrics_b: dict,
    metric_names: list[str] | None = None,
) -> dict:
    """
    Compare two models' metrics.

    Returns dict with improvements.
    """
    if metric_names is None:
        metric_names = ["auc_roc", "precision_at_90_recall", "f1_score"]

    comparison = {}
    for metric in metric_names:
        if metric in metrics_a and metric in metrics_b:
            diff = metrics_b[metric] - metrics_a[metric]
            pct_diff = (diff / metrics_a[metric] * 100) if metrics_a[metric] > 0 else 0
            comparison[metric] = {
                "model_a": metrics_a[metric],
                "model_b": metrics_b[metric],
                "diff": diff,
                "pct_diff": pct_diff,
                "improved": diff > 0,
            }

    return comparison


def validate_quality_gates(
    metrics: dict,
    auc_threshold: float = 0.90,
    precision_threshold: float = 0.50,
    max_fpr: float = 0.05,
    min_recall_at_max_fpr: float = 0.70,
) -> tuple[bool, list[str]]:
    """
    Validate model meets quality gates before it can be promoted.

    Gates (tasks/ТЗ.md, Этап 11):
    - AUC-ROC >= auc_threshold (discrimination);
    - FPR at the production operating point <= max_fpr (review-queue budget);
    - recall at that operating point >= min_recall_at_max_fpr (detection rate);
    - precision@target recall >= precision_threshold (reporting quality).

    Returns:
        (passed, list of failures)
    """
    failures = []

    if metrics["auc_roc"] < auc_threshold:
        failures.append(f"AUC {metrics['auc_roc']:.4f} below threshold {auc_threshold}")

    if metrics.get("fpr_at_max_fpr", metrics.get("fpr", 1.0)) > max_fpr:
        failures.append(f"FPR {metrics.get('fpr_at_max_fpr', 'n/a'):.4f} above budget {max_fpr}")

    recall_at_budget = metrics.get("recall_at_max_fpr", 0.0)
    if recall_at_budget < min_recall_at_max_fpr:
        failures.append(
            f"Recall@FPR<={max_fpr} {recall_at_budget:.4f} below "
            f"minimum {min_recall_at_max_fpr}"
        )

    precision_key = "precision_at_90_recall"
    if metrics.get(precision_key, 0.0) < precision_threshold:
        failures.append(
            f"Prec@90R {metrics.get(precision_key, 0.0):.4f} below "
            f"threshold {precision_threshold}"
        )

    passed = len(failures) == 0

    logger.info(
        "quality_gates.validated",
        passed=passed,
        failures=failures,
    )

    return passed, failures
