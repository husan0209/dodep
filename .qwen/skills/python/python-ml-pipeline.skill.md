SKILL #22 — python-ml-pipeline.skill.md
Markdown

# python-ml-pipeline.skill.md
# GAMBLING PLATFORM — PYTHON ML PIPELINE
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: Python ML Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

Python handles ML model training, evaluation, and export.
Models are trained weekly, exported to ONNX, served in Rust.
Python does NOT serve production traffic — only batch jobs.

Models: fraud detection, user segmentation, churn prediction.
Libraries: XGBoost, LightGBM, scikit-learn, Polars, ONNX.

# ============================================================
# SECTION 2: PROJECT STRUCTURE
# ============================================================

```text
services/fraud-ml-service/
├── pyproject.toml
├── Dockerfile
├── README.md
├── src/
│   ├── __init__.py
│   ├── config.py                # Configuration
│   ├── features/                # Feature engineering
│   │   ├── __init__.py
│   │   ├── extraction.py        # Raw feature extraction from ClickHouse
│   │   ├── transformation.py    # Feature transforms
│   │   └── registry.py          # Feature registry (name, type, source)
│   ├── models/                  # Model training
│   │   ├── __init__.py
│   │   ├── fraud_model.py       # Fraud detection model
│   │   ├── churn_model.py       # Churn prediction model
│   │   └── base.py              # Base model class
│   ├── evaluation/              # Model evaluation
│   │   ├── __init__.py
│   │   ├── metrics.py           # Custom metrics
│   │   └── reports.py           # Generate evaluation reports
│   ├── export/                  # Model export
│   │   ├── __init__.py
│   │   └── onnx_export.py       # Export to ONNX format
│   ├── pipeline/                # Orchestration
│   │   ├── __init__.py
│   │   ├── train_pipeline.py    # Full training pipeline
│   │   └── scheduler.py         # Scheduled retraining
│   └── data/                    # Data access
│       ├── __init__.py
│       ├── clickhouse.py        # ClickHouse queries
│       └── feature_store.py     # Feature caching
├── tests/
│   ├── test_features.py
│   ├── test_models.py
│   └── test_export.py
├── notebooks/                   # Exploration (not production)
│   ├── eda_fraud.ipynb
│   └── feature_analysis.ipynb
└── models/                      # Saved model artifacts
    └── .gitkeep
============================================================
SECTION 3: FEATURE ENGINEERING
============================================================
Python

# src/features/extraction.py

import polars as pl
from datetime import datetime, timedelta

class FeatureExtractor:
    """Extract raw features from ClickHouse."""
    
    def __init__(self, ch_client):
        self.ch = ch_client
    
    def extract_user_features(
        self, 
        user_ids: list[int], 
        as_of: datetime
    ) -> pl.DataFrame:
        """Extract features for a batch of users as of a specific time."""
        
        query = """
        SELECT
            user_id,
            
            -- Betting behavior
            countIf(event_type = 'bet_placed' AND event_time >= {cutoff_7d}) as bets_7d,
            countIf(event_type = 'bet_placed' AND event_time >= {cutoff_24h}) as bets_24h,
            avgIf(toDecimal64(JSONExtractFloat(properties, 'amount'), 2),
                  event_type = 'bet_placed' AND event_time >= {cutoff_30d}) as avg_bet_30d,
            stddevPopIf(toDecimal64(JSONExtractFloat(properties, 'amount'), 2),
                        event_type = 'bet_placed' AND event_time >= {cutoff_30d}) as std_bet_30d,
            maxIf(toDecimal64(JSONExtractFloat(properties, 'amount'), 2),
                  event_type = 'bet_placed' AND event_time >= {cutoff_30d}) as max_bet_30d,
            
            -- Deposit behavior
            countIf(event_type = 'deposit' AND event_time >= {cutoff_24h}) as deposits_24h,
            sumIf(toDecimal64(JSONExtractFloat(properties, 'amount'), 2),
                  event_type = 'deposit' AND event_time >= {cutoff_30d}) as total_deposit_30d,
            
            -- Session behavior
            uniqIf(JSONExtractString(properties, 'device_id'),
                   event_time >= {cutoff_30d}) as device_count_30d,
            uniqIf(JSONExtractString(properties, 'ip'),
                   event_time >= {cutoff_30d}) as ip_count_30d,
            uniqIf(country, event_time >= {cutoff_30d}) as country_count_30d,
            
            -- Win rate
            countIf(event_type = 'bet_settled' AND 
                    JSONExtractString(properties, 'result') = 'won' AND
                    event_time >= {cutoff_7d}) as wins_7d,
            countIf(event_type = 'bet_settled' AND event_time >= {cutoff_7d}) as settled_7d,
            
            -- Account age
            dateDiff('day', min(event_time), {as_of}) as account_age_days
            
        FROM user_events
        WHERE user_id IN ({user_ids})
          AND event_time <= {as_of}
        GROUP BY user_id
        """
        
        result = self.ch.query(query, {
            "user_ids": ",".join(str(uid) for uid in user_ids),
            "as_of": as_of.isoformat(),
            "cutoff_24h": (as_of - timedelta(hours=24)).isoformat(),
            "cutoff_7d": (as_of - timedelta(days=7)).isoformat(),
            "cutoff_30d": (as_of - timedelta(days=30)).isoformat(),
        })
        
        return pl.from_arrow(result.to_arrow())
    
    def compute_derived_features(self, df: pl.DataFrame) -> pl.DataFrame:
        """Compute derived features from raw features."""
        return df.with_columns([
            # Win rate
            (pl.col("wins_7d") / pl.col("settled_7d").clip(lower_bound=1))
                .alias("win_rate_7d"),
            
            # Coefficient of variation (bet amount consistency)
            (pl.col("std_bet_30d") / pl.col("avg_bet_30d").clip(lower_bound=0.01))
                .alias("bet_cv_30d"),
            
            # Deposit to bet ratio
            (pl.col("total_deposit_30d") / pl.col("avg_bet_30d").clip(lower_bound=0.01) / 
             pl.col("bets_7d").clip(lower_bound=1) * 7)
                .alias("deposit_bet_ratio"),
            
            # Multi-device indicator
            (pl.col("device_count_30d") > 3).cast(pl.Int8).alias("multi_device"),
            
            # Multi-IP indicator
            (pl.col("ip_count_30d") > 10).cast(pl.Int8).alias("multi_ip"),
        ]).fill_null(0)
============================================================
SECTION 4: MODEL TRAINING
============================================================
Python

# src/models/fraud_model.py

import xgboost as xgb
from sklearn.model_selection import TimeSeriesSplit
from sklearn.metrics import (
    roc_auc_score, precision_recall_curve, 
    average_precision_score, classification_report
)
import polars as pl
import numpy as np
from pathlib import Path

class FraudModel:
    """XGBoost fraud detection model."""
    
    FEATURE_COLUMNS = [
        "bets_7d", "bets_24h", "avg_bet_30d", "std_bet_30d", "max_bet_30d",
        "deposits_24h", "total_deposit_30d",
        "device_count_30d", "ip_count_30d", "country_count_30d",
        "wins_7d", "settled_7d", "account_age_days",
        "win_rate_7d", "bet_cv_30d", "deposit_bet_ratio",
        "multi_device", "multi_ip",
    ]
    
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
        }
    
    def train(self, df: pl.DataFrame) -> dict:
        """Train model with time-based split."""
        
        # Sort by time for proper split
        df = df.sort("event_time")
        
        X = df.select(self.FEATURE_COLUMNS).to_numpy()
        y = df.select(self.TARGET_COLUMN).to_numpy().ravel()
        
        # Time-based split: train on older, test on newer
        split_idx = int(len(X) * 0.8)
        X_train, X_test = X[:split_idx], X[split_idx:]
        y_train, y_test = y[:split_idx], y[split_idx:]
        
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
        
        return {
            "auc_roc": float(auc),
            "avg_precision": float(ap),
            "precision_at_90_recall": float(precision_at_90_recall),
            "threshold_90_recall": float(threshold_90),
            "samples_total": len(y_true),
            "samples_positive": int(y_true.sum()),
            "positive_rate": float(y_true.mean()),
        }
    
    def save(self, path: Path):
        """Save model in XGBoost native + ONNX format."""
        path.mkdir(parents=True, exist_ok=True)
        self.model.save_model(str(path / "fraud_model.json"))
    
    def export_onnx(self, path: Path):
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
        
        return onnx_path
============================================================
SECTION 5: TRAINING PIPELINE
============================================================
Python

# src/pipeline/train_pipeline.py

import structlog
from datetime import datetime
from pathlib import Path

logger = structlog.get_logger()

class TrainPipeline:
    """End-to-end training pipeline. Runs weekly."""
    
    def __init__(self, extractor, model, s3_client, config):
        self.extractor = extractor
        self.model = model
        self.s3 = s3_client
        self.config = config
    
    def run(self) -> dict:
        """Execute full pipeline."""
        run_id = datetime.utcnow().strftime("%Y%m%d_%H%M%S")
        logger.info("pipeline.start", run_id=run_id)
        
        # 1. Extract features
        logger.info("pipeline.extract_features")
        df = self.extractor.extract_training_data(
            lookback_days=90,
            as_of=datetime.utcnow(),
        )
        df = self.extractor.compute_derived_features(df)
        logger.info("pipeline.features_extracted", rows=len(df))
        
        # 2. Train model
        logger.info("pipeline.train")
        metrics = self.model.train(df)
        logger.info("pipeline.trained", **metrics)
        
        # 3. Validate metrics
        if metrics["auc_roc"] < 0.90:
            logger.error("pipeline.quality_gate_failed",
                         auc=metrics["auc_roc"], threshold=0.90)
            raise ValueError(f"AUC {metrics['auc_roc']:.4f} below threshold 0.90")
        
        if metrics["precision_at_90_recall"] < 0.50:
            logger.error("pipeline.quality_gate_failed",
                         precision=metrics["precision_at_90_recall"])
            raise ValueError("Precision at 90% recall below 0.50")
        
        # 4. Export ONNX
        model_dir = Path(f"models/{run_id}")
        onnx_path = self.model.export_onnx(model_dir)
        logger.info("pipeline.exported_onnx", path=str(onnx_path))
        
        # 5. Upload to S3
        s3_key = f"ml-models/fraud/{run_id}/fraud_model.onnx"
        self.s3.upload_file(str(onnx_path), self.config.s3_bucket, s3_key)
        logger.info("pipeline.uploaded_s3", key=s3_key)
        
        # 6. Notify Rust service to reload model
        # (via Redpanda event or config update)
        
        logger.info("pipeline.complete", run_id=run_id, **metrics)
        return {"run_id": run_id, "s3_key": s3_key, **metrics}
============================================================
SECTION 6: ANTI-PATTERNS
============================================================
text

❌ NEVER use Pandas for large datasets → USE Polars (10x faster)
❌ NEVER train on shuffled time-series data → USE time-based split
❌ NEVER deploy model without quality gate (AUC threshold)
❌ NEVER serve model directly from Python in production → export ONNX, serve in Rust
❌ NEVER hardcode feature columns → USE registry/config
❌ NEVER skip class imbalance handling (fraud is ~1%)
❌ NEVER log PII in training pipeline (user IDs OK, emails NOT OK)
❌ NEVER use pickle for model serialization → USE ONNX or native format
❌ NEVER skip monitoring model drift after deployment
❌ NEVER train on production database → USE ClickHouse (analytics replica)