"""
Fraud detection model using XGBoost.

Design notes (research-backed, see services/python/fraud-ml/README.md):
- **Temporal split with embargo**: data is sorted by event_time and split
  into train / validation / test by time (never random — random splits leak
  future behaviour of the same users into training). An embargo gap of
  `embargo_days` between splits covers the 1-day label horizon used by the
  fraud_signals join, so labels maturing inside the gap cannot leak.
- **Threshold on validation, reported on test**: the production threshold
  (max recall subject to FPR <= max_fpr) is selected on the validation
  split only; test metrics at that threshold are the honest estimate.
- **scale_pos_weight** is computed from class frequencies instead of a
  hard-coded constant (fraud rate moves with lookback window and seasonality).
- Artifacts: `fraud_model.json` (booster) + `model_card.json` (threshold,
  feature order, metrics, params) — consumers (Rust serving, ONNX metadata)
  read the same threshold the model was evaluated with.
"""

import json
from datetime import datetime
from pathlib import Path

import numpy as np
import polars as pl
import structlog
import xgboost as xgb

from ..evaluation.metrics import calculate_metrics, select_threshold_constrained
from ..features.transformation import MODEL_FEATURES

logger = structlog.get_logger()

MODEL_FILE = "fraud_model.json"
MODEL_CARD_FILE = "model_card.json"


class FraudModel:
    """XGBoost fraud detection model."""

    FEATURE_COLUMNS = MODEL_FEATURES
    TARGET_COLUMN = "is_fraud"

    def __init__(self, random_state: int = 42):
        self.model: xgb.XGBClassifier | None = None
        self.random_state = random_state
        # Production threshold; 0.5 until train()/load() set the real one.
        self.threshold: float = 0.5
        self.max_fpr: float = 0.05
        self.target_recall: float = 0.90
        self.embargo_days: int = 1
        self.params = {
            "objective": "binary:logistic",
            "eval_metric": ["aucpr", "auc"],
            "max_depth": 6,
            "learning_rate": 0.05,
            "subsample": 0.8,
            "colsample_bytree": 0.8,
            "min_child_weight": 5,
            "tree_method": "hist",
            "n_estimators": 1000,
            "early_stopping_rounds": 50,
            "random_state": self.random_state,
        }
        logger.info("fraud_model.initialized", params=self.params)

    # ------------------------------------------------------------------
    # Splitting
    # ------------------------------------------------------------------
    def _temporal_split(self, df: pl.DataFrame) -> tuple[np.ndarray, np.ndarray, np.ndarray]:
        """
        Split into train/valid/test by time with an embargo gap.

        Ratios: 60% train, 20% validation, 20% test (by rows of the
        time-ordered data). If `event_time` exists, boundaries are time
        quantiles and rows within `embargo_days` of a boundary are dropped;
        otherwise fall back to positional split without embargo.
        """
        n = len(df)
        if n < 30:
            raise ValueError(f"Not enough samples to split: {n} < 30")

        train_end = int(n * 0.60)
        valid_end = int(n * 0.80)

        if "event_time" in df.columns:
            times = df["event_time"].to_numpy()
            t_train = times[train_end]
            t_valid = times[valid_end]
            if np.issubdtype(times.dtype, np.datetime64):
                delta = np.timedelta64(self.embargo_days, "D")
                t_train_cut = t_train + delta
                t_valid_cut = t_valid + delta
            else:
                t_train_cut = t_train
                t_valid_cut = t_valid

            mask_train = times <= t_train_cut
            mask_valid = (times > t_train_cut) & (times <= t_valid_cut)
            mask_test = times > t_valid_cut

            # Guard: temporal boundaries with duplicate/flat timestamps can
            # starve a split; fall back to positional split in that case.
            if min(mask_train.sum(), mask_valid.sum(), mask_test.sum()) < 10:
                logger.warning("fraud_model.temporal_split_starved", fallback="positional")
            else:
                return mask_train, mask_valid, mask_test

        idx = np.arange(n)
        mask_train = idx < train_end
        mask_valid = (idx >= train_end) & (idx < valid_end)
        mask_test = idx >= valid_end
        return mask_train, mask_valid, mask_test

    # ------------------------------------------------------------------
    # Training
    # ------------------------------------------------------------------
    def train(
        self,
        df: pl.DataFrame,
        max_fpr: float = 0.05,
        target_recall: float = 0.90,
    ) -> dict:
        """
        Train model with a temporal train/valid/test split.

        Early stopping runs on the validation split; the production
        threshold is selected on validation; returned metrics are computed
        on the held-out test split (with the validation-selected threshold
        recorded separately).

        Returns evaluation metrics.
        """
        logger.info("fraud_model.training_start", rows=len(df))
        self.max_fpr = max_fpr
        self.target_recall = target_recall

        # Sort by time for a proper temporal split.
        if "event_time" in df.columns:
            df = df.sort("event_time")

        missing = set(self.FEATURE_COLUMNS + [self.TARGET_COLUMN]) - set(df.columns)
        if missing:
            raise ValueError(f"Missing columns for training: {sorted(missing)}")

        x_matrix = df.select(self.FEATURE_COLUMNS).to_numpy().astype(np.float64, copy=False)
        y = df.select(self.TARGET_COLUMN).to_numpy().ravel().astype(int)

        mask_train, mask_valid, mask_test = self._temporal_split(df)
        x_train, y_train = x_matrix[mask_train], y[mask_train]
        x_valid, y_valid = x_matrix[mask_valid], y[mask_valid]
        x_test, y_test = x_matrix[mask_test], y[mask_test]

        n_pos = int(y_train.sum())
        n_neg = len(y_train) - n_pos
        if n_pos == 0 or n_neg == 0:
            raise ValueError(
                f"Train split is single-class (pos={n_pos}, neg={n_neg}); "
                "check label extraction and split boundaries."
            )
        scale_pos_weight = n_neg / n_pos
        self.params["scale_pos_weight"] = round(scale_pos_weight, 4)

        logger.info(
            "fraud_model.data_split",
            train_size=len(x_train),
            valid_size=len(x_valid),
            test_size=len(x_test),
            fraud_rate_train=float(y_train.mean()),
            fraud_rate_valid=float(y_valid.mean()),
            fraud_rate_test=float(y_test.mean()),
            scale_pos_weight=round(scale_pos_weight, 4),
        )

        # Train with early stopping on the *validation* split.
        self.model = xgb.XGBClassifier(**self.params)
        self.model.fit(
            x_train,
            y_train,
            eval_set=[(x_valid, y_valid)],
            verbose=False,
        )

        # Production threshold: selected on validation only.
        p_valid = self.model.predict_proba(x_valid)[:, 1]
        selection = select_threshold_constrained(y_valid, p_valid, max_fpr=max_fpr)
        self.threshold = selection["threshold"]

        # Honest evaluation on the held-out test split.
        p_test = self.model.predict_proba(x_test)[:, 1]
        metrics = calculate_metrics(y_test, p_test, target_recall=target_recall, max_fpr=max_fpr)

        # Record where the threshold came from and how it behaves on test:
        # apply the *validation-selected* threshold to test scores directly.
        y_pred_test = (p_test >= self.threshold).astype(int)
        tp = int(np.sum((y_test == 1) & (y_pred_test == 1)))
        fp = int(np.sum((y_test == 0) & (y_pred_test == 1)))
        tn = int(np.sum((y_test == 0) & (y_pred_test == 0)))
        fn = int(np.sum((y_test == 1) & (y_pred_test == 0)))
        precision_at_thr = tp / (tp + fp) if (tp + fp) > 0 else 0.0
        recall_at_thr = tp / (tp + fn) if (tp + fn) > 0 else 0.0
        f1_at_thr = (
            2 * precision_at_thr * recall_at_thr / (precision_at_thr + recall_at_thr)
            if (precision_at_thr + recall_at_thr) > 0
            else 0.0
        )
        metrics["threshold"] = self.threshold
        metrics["threshold_selected_on"] = "validation"
        metrics["test_at_production_threshold"] = {
            "threshold": self.threshold,
            "precision": float(precision_at_thr),
            "recall": float(recall_at_thr),
            "f1_score": float(f1_at_thr),
            "true_positives": tp,
            "false_positives": fp,
            "true_negatives": tn,
            "false_negatives": fn,
        }
        metrics["validation_selection"] = {
            "threshold": selection["threshold"],
            "recall": selection["recall"],
            "precision": selection["precision"],
            "fpr": selection["fpr"],
            "max_fpr": max_fpr,
        }
        metrics["split"] = {
            "train": int(mask_train.sum()),
            "valid": int(mask_valid.sum()),
            "test": int(mask_test.sum()),
            "embargo_days": self.embargo_days,
            "sort_key": "event_time" if "event_time" in df.columns else "row_order",
        }
        metrics["params"] = dict(self.params)
        try:
            metrics["best_iteration"] = int(self.model.get_booster().best_iteration) + 1
        except Exception:  # no early-stopping info available
            metrics["best_iteration"] = None

        logger.info(
            "fraud_model.training_complete",
            auc=round(metrics["auc_roc"], 4),
            threshold=round(self.threshold, 4),
            recall_at_max_fpr=round(metrics["recall_at_max_fpr"], 4),
            fpr_at_max_fpr=round(metrics["fpr_at_max_fpr"], 4),
        )

        return metrics

    # ------------------------------------------------------------------
    # Inference
    # ------------------------------------------------------------------
    def predict(self, x_matrix: np.ndarray) -> np.ndarray:
        """Predict fraud probability."""
        if self.model is None:
            raise ValueError("Model not trained")
        return self.model.predict_proba(x_matrix)[:, 1]

    def predict_with_threshold(
        self,
        x_matrix: np.ndarray,
        threshold: float | None = None,
    ) -> np.ndarray:
        """
        Predict fraud class using the production threshold.

        Falls back to the threshold selected during train()/load() when
        not explicitly provided.
        """
        proba = self.predict(x_matrix)
        thr = self.threshold if threshold is None else threshold
        return (proba >= thr).astype(int)

    def get_feature_importance(self) -> dict:
        """Get gain-based feature importance (falls back to weight)."""
        if self.model is None:
            return {}

        booster = self.model.get_booster()
        try:
            scores = booster.get_score(importance_type="gain")
            # Without feature names XGBoost uses f0..fN keys.
            result: dict[str, float] = {}
            for key, value in scores.items():
                if key.startswith("f") and key[1:].isdigit():
                    idx = int(key[1:])
                    if idx < len(self.FEATURE_COLUMNS):
                        result[self.FEATURE_COLUMNS[idx]] = float(value)  # type: ignore[arg-type]
                        continue
                if key in self.FEATURE_COLUMNS:
                    result[key] = float(value)  # type: ignore[arg-type]
            if result:
                # Normalize to 0..1 so consumers can compare across runs.
                total = sum(result.values())
                if total > 0:
                    result = {k: v / total for k, v in result.items()}
                return {name: result.get(name, 0.0) for name in self.FEATURE_COLUMNS}
        except Exception as exc:  # pragma: no cover - defensive
            logger.warning("fraud_model.importance_fallback", error=str(exc))

        return dict(
            zip(
                self.FEATURE_COLUMNS,
                self.model.feature_importances_.tolist(),
                strict=True,
            )
        )

    # ------------------------------------------------------------------
    # Persistence
    # ------------------------------------------------------------------
    def save(self, path: Path, metrics: dict | None = None) -> None:
        """Save model (XGBoost native) + model card (threshold, features)."""
        if self.model is None:
            raise ValueError("Model not trained")
        path.mkdir(parents=True, exist_ok=True)
        model_path = path / MODEL_FILE
        self.model.save_model(str(model_path))

        card = {
            "model_type": "xgboost_binary_logistic",
            "feature_columns": list(self.FEATURE_COLUMNS),
            "target": self.TARGET_COLUMN,
            "threshold": float(self.threshold),
            "threshold_selected_on": "validation",
            "max_fpr": float(self.max_fpr),
            "target_recall": float(self.target_recall),
            "embargo_days": int(self.embargo_days),
            "params": dict(self.params),
            "trained_at": datetime.utcnow().isoformat(),
            "random_state": self.random_state,
        }
        if metrics is not None:
            card["metrics"] = metrics
        card_path = path / MODEL_CARD_FILE
        card_path.write_text(json.dumps(card, indent=2, default=str), encoding="utf-8")

        logger.info(
            "fraud_model.saved",
            path=str(model_path),
            threshold=self.threshold,
            card=str(card_path),
        )

    def load(self, path: Path) -> None:
        """Load model and restore threshold/feature order from the model card."""
        model_path = path / MODEL_FILE
        if not model_path.exists():
            raise FileNotFoundError(f"Model not found: {model_path}")

        self.model = xgb.XGBClassifier()
        self.model.load_model(str(model_path))

        card_path = path / MODEL_CARD_FILE
        if card_path.exists():
            card = json.loads(card_path.read_text(encoding="utf-8"))
            self.threshold = float(card.get("threshold", 0.5))
            self.max_fpr = float(card.get("max_fpr", 0.05))
            self.target_recall = float(card.get("target_recall", 0.90))
            saved_features = card.get("feature_columns")
            if saved_features and saved_features != list(self.FEATURE_COLUMNS):
                # Serving must consume features in exactly the training order.
                raise ValueError(
                    "Model card feature order does not match code registry: "
                    f"card={len(saved_features)} code={len(self.FEATURE_COLUMNS)}"
                )
        else:
            logger.warning(
                "fraud_model.card_missing",
                path=str(card_path),
                fallback_threshold=0.5,
            )

        logger.info(
            "fraud_model.loaded",
            path=str(model_path),
            threshold=self.threshold,
        )

    def export_onnx(self, path: Path) -> Path:
        """Export to ONNX for serving in Rust (see src/export/onnx_export.py)."""
        from ..export.onnx_export import export_model_to_onnx

        return export_model_to_onnx(self, path)
