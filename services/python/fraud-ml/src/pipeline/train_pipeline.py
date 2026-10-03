"""
Training pipeline for fraud detection model.

Weekly job: extract features -> train -> quality gates -> export ONNX
(with parity validation) -> write model card -> optional S3 upload.

Quality gates block promotion: if any gate fails, no artifacts are
uploaded and the pipeline raises (see src/evaluation/metrics.py).
"""

import json
from datetime import datetime
from pathlib import Path

import numpy as np
import structlog

from ..config import settings
from ..evaluation.metrics import calculate_metrics, validate_quality_gates
from ..export.onnx_export import validate_onnx_model
from ..features.extraction import FeatureExtractor
from ..features.transformation import FeatureTransformer
from ..models.fraud_model import MODEL_CARD_FILE, FraudModel

logger = structlog.get_logger()

PARITY_SAMPLE_ROWS = 200


class TrainPipeline:
    """End-to-end training pipeline. Runs weekly."""

    def __init__(
        self,
        ch_client,
        feature_store,
        s3_client=None,
        config=None,
    ):
        self.ch_client = ch_client
        self.feature_store = feature_store
        self.s3_client = s3_client
        self.config = config or settings
        self.extractor = FeatureExtractor(ch_client)
        self.transformer = FeatureTransformer()
        self.model = FraudModel()

    def run(self) -> dict:
        """Execute full training pipeline."""
        run_id = datetime.utcnow().strftime("%Y%m%d_%H%M%S")
        logger.info("pipeline.start", run_id=run_id)

        try:
            # 1. Extract features
            logger.info("pipeline.extract_features")
            df = self.extractor.extract_training_data(
                lookback_days=self.config.lookback_days,
                as_of=datetime.utcnow(),
            )
            df = self.extractor.compute_derived_features(df)

            # Cache features (Series.to_list(), not tolist(), on polars)
            self.feature_store.cache(
                df,
                user_ids=df["user_id"].unique().to_list(),
                as_of=datetime.utcnow(),
            )

            logger.info(
                "pipeline.features_extracted",
                rows=len(df),
                fraud_count=df["is_fraud"].sum() if "is_fraud" in df.columns else 0,
            )

            # 2. Validate features
            self.transformer.validate_features(df)

            # 3. Train model (temporal split; threshold selected on validation)
            logger.info("pipeline.train")
            metrics = self.model.train(
                df,
                max_fpr=self.config.max_fpr,
                target_recall=self.config.target_recall,
            )
            logger.info("pipeline.trained", **_log_safe(metrics))

            # 4. Quality gate — blocks export/upload on failure
            passed, failures = validate_quality_gates(
                metrics,
                auc_threshold=self.config.model_quality_threshold,
                precision_threshold=self.config.precision_threshold,
                max_fpr=self.config.max_fpr,
                min_recall_at_max_fpr=self.config.min_recall_at_max_fpr,
            )

            if not passed:
                logger.error("pipeline.quality_gate_failed", failures=failures)
                raise ValueError(f"Quality gate failed: {failures}")

            logger.info("pipeline.quality_gate_passed")

            # 5. Persist artifacts: booster + model card (threshold, features)
            model_dir = Path(f"{self.config.model_path}/{run_id}")
            self.model.save(model_dir, metrics=metrics)

            # 6. Export ONNX with serving metadata (threshold, feature order)
            onnx_path = self.model.export_onnx(model_dir)
            logger.info("pipeline.exported_onnx", path=str(onnx_path))

            # 7. Validate ONNX: structure + probability parity vs source model
            #    on real (held-out) feature rows.
            parity_sample = (
                df.select(self.model.FEATURE_COLUMNS)
                .tail(PARITY_SAMPLE_ROWS)
                .to_numpy()
                .astype(np.float64, copy=False)
            )
            validation = validate_onnx_model(
                onnx_path,
                reference_model=self.model,
                reference_data=parity_sample,
            )
            if not validation["valid"]:
                raise ValueError(f"ONNX validation failed: {validation.get('error')}")
            logger.info(
                "pipeline.onnx_validated",
                max_abs_diff=validation.get("max_abs_diff"),
                label_agreement=validation.get("label_agreement"),
            )

            # 8. Upload to S3
            s3_key = None
            if self.s3_client:
                s3_key = f"ml-models/fraud/{run_id}/fraud_model.onnx"
                self.s3_client.upload_file(
                    str(onnx_path),
                    self.config.s3_bucket,
                    s3_key,
                )
                logger.info("pipeline.uploaded_s3", key=s3_key)

            # 9. Finalize model card with run info + gates result
            metrics["run_id"] = run_id
            metrics["timestamp"] = datetime.utcnow().isoformat()
            metrics["s3_key"] = s3_key
            metrics["quality_gates"] = {"passed": True, "failures": []}
            self._write_run_info(model_dir, metrics, validation)

            logger.info("pipeline.complete", run_id=run_id, threshold=self.model.threshold)
            return {
                "run_id": run_id,
                "s3_key": s3_key,
                "model_path": str(model_dir),
                "onnx_path": str(onnx_path),
                **metrics,
            }

        except Exception as e:
            logger.error("pipeline.failed", error=str(e))
            raise

    def _write_run_info(self, model_dir: Path, metrics: dict, onnx_validation: dict) -> None:
        """Append pipeline-level info to the model card written by save()."""
        card_path = model_dir / MODEL_CARD_FILE
        card = json.loads(card_path.read_text(encoding="utf-8"))
        card["metrics"] = metrics
        card["onnx_validation"] = {
            k: onnx_validation.get(k)
            for k in ("max_abs_diff", "label_agreement", "opset", "output_names")
        }
        card_path.write_text(json.dumps(card, indent=2, default=str), encoding="utf-8")


def _log_safe(metrics: dict) -> dict:
    """Keep log kwargs scalar for structlog JSON rendering."""
    return {k: v for k, v in metrics.items() if isinstance(v, (int, float, str, bool)) or v is None}


# Re-exported for callers that only need metric computation
__all__ = ["TrainPipeline", "calculate_metrics"]
