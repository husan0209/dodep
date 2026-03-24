"""
Fraud detection model using XGBoost.
"""
import structlog
import polars as pl
import numpy as np
from pathlib import Path

import xgboost as xgb
from sklearn.model_selection import TimeSeriesSplit
from sklearn.metrics import (
    roc_auc_score,
    average_precision_score,
    precision_recall_curve,
    classification_report,
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
        
        X = df.select(self.FEATURE_COLUMNS).to_numpy()
        y = df.select(self.TARGET_COLUMN).to_numpy().ravel()
        
        # Time-based split: train on older, test on newer
        split_idx = int(len(X) * 0.8)
        X_train, X_test = X[:split_idx], X[split_idx:]
        y_train, y_test = y[:split_idx], y[split_idx:]
        
        logger.info(
            "fraud_model.data_split",
            train_size=len(X_train),
            test_size=len(X_test),
            fraud_rate_train=float(y_train.mean()),
            fraud_rate_test=float(y_test.mean()),
        )
        
        # Train
        self.model = xgb.XGBClassifier(**self.params)
        self.model.fit(
            X_train, y_train,
            eval_set=[(X_test, y_test)],
            verbose=False,
        )
        
        # Evaluate
        y_pred_proba = self.model.predict_proba(X_test)[:, 1]
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
    
    def predict(self, X: np.ndarray) -> np.ndarray:
        """Predict fraud probability."""
        if self.model is None:
            raise ValueError("Model not trained")
        return self.model.predict_proba(X)[:, 1]
    
    def predict_with_threshold(
        self,
        X: np.ndarray,
        threshold: float = 0.5
    ) -> np.ndarray:
        """Predict fraud class with custom threshold."""
        proba = self.predict(X)
        return (proba >= threshold).astype(int)
    
    def get_feature_importance(self) -> dict:
        """Get feature importance scores."""
        if self.model is None:
            return {}
        
        importance = self.model.feature_importances_
        return dict(zip(self.FEATURE_COLUMNS, importance.tolist()))
    
    def save(self, path: Path):
        """Save model in XGBoost native format."""
        path.mkdir(parents=True, exist_ok=True)
        model_path = path / "fraud_model.json"
        self.model.save_model(str(model_path))
        logger.info("fraud_model.saved", path=str(model_path))
    
    def export_onnx(self, path: Path) -> Path:
        """Export to ONNX for serving in Rust."""
        from skl2onnx import convert_sklearn
        from skl2onnx.common.data_types import FloatTensorType
        
        initial_type = [
            ("features", FloatTensorType([None, len(self.FEATURE_COLUMNS)]))
        ]
        
        onnx_model = convert_sklearn(self.model, initial_types=initial_type)
        
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
