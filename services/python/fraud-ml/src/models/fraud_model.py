"""
Fraud detection model using XGBoost.
"""
from pathlib import Path

import numpy as np
import polars as pl
import structlog
import xgboost as xgb
from sklearn.metrics import (
    average_precision_score,
    classification_report,
    precision_recall_curve,
    roc_auc_score,
)

from ..features.transformation import MODEL_FEATURES

logger = structlog.get_logger()


class FraudModel:
    """XGBoost fraud detection model."""

    FEATURE_COLUMNS = MODEL_FEATURES
    TARGET_COLUMN = "is_fraud"

    def __init__(self):
        self.model = None
        self.params = {
            "objective": "binary:logistic",
            "eval_metric": ["auc", "aucpr"],
            "max_depth": 6,
            "learning_rate": 0.05,
            "subsample": 0.8,
            "colsample_bytree": 0.8,
            "min_child_weight": 5,
            "scale_pos_weight": 10,  # fraud is rare (~1%)
            "tree_method": "hist",
            "n_estimators": 500,
            "early_stopping_rounds": 50,
            "random_state": 42,
        }
        logger.info("fraud_model.initialized", params=self.params)

    def train(self, df: pl.DataFrame) -> dict:
        """
        Train model with time-based split.

        Returns evaluation metrics.
        """
        logger.info("fraud_model.training_start", rows=len(df))

        # Sort by time for proper split
        if "event_time" in df.columns:
            df = df.sort("event_time")

        features = df.select(self.FEATURE_COLUMNS).to_numpy()
        labels = df.select(self.TARGET_COLUMN).to_numpy().ravel()

        # Time-based split: train on older, test on newer
        split_idx = int(len(features) * 0.8)
        x_train, x_test = features[:split_idx], features[split_idx:]
        y_train, y_test = labels[:split_idx], labels[split_idx:]

        logger.info(
            "fraud_model.data_split",
            train_size=len(x_train),
            test_size=len(x_test),
            fraud_rate_train=float(y_train.mean()),
            fraud_rate_test=float(y_test.mean()),
        )

        # Train
        self.model = xgb.XGBClassifier(**self.params)
        self.model.fit(
            x_train, y_train,
            eval_set=[(x_test, y_test)],
            verbose=False,
        )

        # Evaluate
        y_pred_proba = self.model.predict_proba(x_test)[:, 1]
        metrics = self._evaluate(y_test, y_pred_proba)

        logger.info(
            "fraud_model.training_complete",
            auc=round(metrics["auc_roc"], 4),
            precision_at_90_recall=round(metrics["precision_at_90_recall"], 4),
        )

        return metrics

    def _evaluate(self, y_true: np.ndarray, y_pred_proba: np.ndarray) -> dict:
        """Calculate evaluation metrics."""
        auc = roc_auc_score(y_true, y_pred_proba)
        ap = average_precision_score(y_true, y_pred_proba)

        # Find threshold for 90% recall
        precision, recall, thresholds = precision_recall_curve(y_true, y_pred_proba)
        idx_90_recall = np.argmin(np.abs(recall - 0.90))
        threshold_90 = thresholds[min(idx_90_recall, len(thresholds) - 1)]
        precision_at_90_recall = precision[idx_90_recall]

        # Apply threshold for classification report
        y_pred = (y_pred_proba >= threshold_90).astype(int)
        report = classification_report(y_true, y_pred, output_dict=True)

        return {
            "auc_roc": float(auc),
            "avg_precision": float(ap),
            "precision_at_90_recall": float(precision_at_90_recall),
            "threshold_90_recall": float(threshold_90),
            "samples_total": len(y_true),
            "samples_positive": int(y_true.sum()),
            "positive_rate": float(y_true.mean()),
            "f1_score": float(report["weighted avg"]["f1-score"]),
            "precision": float(report["weighted avg"]["precision"]),
            "recall": float(report["weighted avg"]["recall"]),
        }

    def predict(self, x: np.ndarray) -> np.ndarray:
        """Predict fraud probability."""
        if self.model is None:
            raise ValueError("Model not trained")
        proba: np.ndarray = self.model.predict_proba(x)[:, 1]
        return proba

    def predict_with_threshold(
        self,
        x: np.ndarray,
        threshold: float = 0.5
    ) -> np.ndarray:
        """Predict fraud class with custom threshold."""
        proba = self.predict(x)
        return (proba >= threshold).astype(int)

    def get_feature_importance(self) -> dict:
        """Get feature importance scores."""
        if self.model is None:
            return {}

        importance = self.model.feature_importances_
        return dict(zip(self.FEATURE_COLUMNS, importance.tolist(), strict=True))

    def save(self, path: Path):
        """Save model in XGBoost native format."""
        if self.model is None:
            raise ValueError("Model not trained")
        path.mkdir(parents=True, exist_ok=True)
        model_path = path / "fraud_model.json"
        self.model.save_model(str(model_path))
        logger.info("fraud_model.saved", path=str(model_path))

    def export_onnx(self, path: Path) -> Path:
        """Export to ONNX for serving in Rust."""
        # XGBClassifier is not a scikit-learn estimator, so it cannot be
        # converted by skl2onnx.convert_sklearn; onnxmltools ships the
        # XGBoost converter (and has to supply its own FloatTensorType).
        import onnxmltools
        from onnxmltools.convert.common.data_types import FloatTensorType

        if self.model is None:
            raise ValueError("Model not trained")

        initial_type = [
            ("features", FloatTensorType([None, len(self.FEATURE_COLUMNS)]))
        ]

        onnx_model = onnxmltools.convert_xgboost(self.model, initial_types=initial_type)

        path.mkdir(parents=True, exist_ok=True)
        onnx_path = path / "fraud_model.onnx"
        with open(onnx_path, "wb") as f:
            f.write(onnx_model.SerializeToString())

        logger.info("fraud_model.exported_onnx", path=str(onnx_path))
        return onnx_path

    def load(self, path: Path):
        """Load model from file."""
        model_path = path / "fraud_model.json"
        if not model_path.exists():
            raise FileNotFoundError(f"Model not found: {model_path}")

        self.model = xgb.XGBClassifier()
        self.model.load_model(str(model_path))
        logger.info("fraud_model.loaded", path=str(model_path))
