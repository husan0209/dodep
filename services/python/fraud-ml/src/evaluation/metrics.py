"""
Model evaluation metrics for fraud detection.
"""
import structlog
import numpy as np
from sklearn.metrics import (
    roc_auc_score,
    average_precision_score,
    precision_recall_curve,
    classification_report,
    confusion_matrix,
    roc_curve,
)

logger = structlog.get_logger()


def calculate_metrics(
    y_true: np.ndarray,
    y_pred_proba: np.ndarray,
    target_recall: float = 0.90
) -> dict:
    """
    Calculate comprehensive fraud detection metrics.
    
    Args:
        y_true: True labels (0 or 1)
        y_pred_proba: Predicted probabilities
        target_recall: Target recall for threshold calculation
    
    Returns:
        Dictionary with all metrics
    """
    # Basic metrics
    auc = roc_auc_score(y_true, y_pred_proba)
    ap = average_precision_score(y_true, y_pred_proba)
    
    # Find threshold for target recall
    precision, recall, thresholds = precision_recall_curve(y_true, y_pred_proba)
    idx = np.argmin(np.abs(recall - target_recall))
    threshold = thresholds[min(idx, len(thresholds) - 1)]
    precision_at_recall = precision[idx]
    
    # Apply threshold
    y_pred = (y_pred_proba >= threshold).astype(int)
    cm = confusion_matrix(y_true, y_pred)
    
    # Classification report
    report = classification_report(y_true, y_pred, output_dict=True)
    
    # ROC curve points (for plotting)
    fpr, tpr, roc_thresholds = roc_curve(y_true, y_pred_proba)
    
    metrics = {
        # Discrimination
        "auc_roc": float(auc),
        "avg_precision": float(ap),
        
        # Threshold-based
        f"precision_at_{target_recall*100}_recall": float(precision_at_recall),
        "threshold": float(threshold),
        
        # Confusion matrix
        "true_positives": int(cm[1, 1]),
        "false_positives": int(cm[0, 1]),
        "true_negatives": int(cm[0, 0]),
        "false_negatives": int(cm[1, 0]),
        
        # Classification metrics
        "precision": float(report["weighted avg"]["precision"]),
        "recall": float(report["weighted avg"]["recall"]),
        "f1_score": float(report["weighted avg"]["f1-score"]),
        
        # Class distribution
        "samples_total": len(y_true),
        "samples_positive": int(y_true.sum()),
        "samples_negative": int(len(y_true) - y_true.sum()),
        "positive_rate": float(y_true.mean()),
        
        # ROC curve (sampled for storage)
        "roc_curve": {
            "fpr": fpr[::10].tolist(),  # Sample every 10th point
            "tpr": tpr[::10].tolist(),
            "thresholds": roc_thresholds[::10].tolist(),
        },
    }
    
    logger.info(
        "metrics.calculated",
        auc=round(auc, 4),
        precision_at_recall=round(precision_at_recall, 4),
        f1_score=round(metrics["f1_score"], 4),
    )
    
    return metrics


def compare_models(
    metrics_a: dict,
    metrics_b: dict,
    metric_names: list[str] | None = None
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
    precision_threshold: float = 0.50
) -> tuple[bool, list[str]]:
    """
    Validate model meets quality gates.
    
    Returns:
        (passed, list of failures)
    """
    failures = []
    
    if metrics["auc_roc"] < auc_threshold:
        failures.append(
            f"AUC {metrics['auc_roc']:.4f} below threshold {auc_threshold}"
        )
    
    if metrics["precision_at_90_recall"] < precision_threshold:
        failures.append(
            f"Prec@90R {metrics['precision_at_90_recall']:.4f} below threshold {precision_threshold}"
        )
    
    passed = len(failures) == 0
    
    logger.info(
        "quality_gates.validated",
        passed=passed,
        failures=failures,
    )
    
    return passed, failures
