"""
Training pipeline for fraud detection model.
"""
import structlog
from datetime import datetime
from pathlib import Path

from ..config import settings
from ..features.extraction import FeatureExtractor
from ..features.transformation import FeatureTransformer
from ..models.fraud_model import FraudModel
from ..evaluation.metrics import calculate_metrics, validate_quality_gates
from ..export.onnx_export import validate_onnx_model

logger = structlog.get_logger()


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
            
            # Cache features
            self.feature_store.cache(
                df,
                user_ids=df["user_id"].unique().tolist(),
                as_of=datetime.utcnow(),
            )
            
            logger.info(
                "pipeline.features_extracted",
                rows=len(df),
                fraud_count=df["is_fraud"].sum() if "is_fraud" in df.columns else 0,
            )
            
            # 2. Validate features
            self.transformer.validate_features(df)
            
            # 3. Train model
            logger.info("pipeline.train")
            metrics = self.model.train(df)
            logger.info("pipeline.trained", **metrics)
            
            # 4. Quality gate
            passed, failures = validate_quality_gates(
                metrics,
                auc_threshold=self.config.model_quality_threshold,
                precision_threshold=self.config.precision_threshold,
            )
            
            if not passed:
                logger.error(
                    "pipeline.quality_gate_failed",
                    failures=failures,
                )
                raise ValueError(f"Quality gate failed: {failures}")
            
            logger.info("pipeline.quality_gate_passed")
            
            # 5. Export ONNX
            model_dir = Path(f"{self.config.model_path}/{run_id}")
            onnx_path = self.model.export_onnx(model_dir)
            logger.info("pipeline.exported_onnx", path=str(onnx_path))
            
            # 6. Validate ONNX
            validation = validate_onnx_model(onnx_path)
            if not validation["valid"]:
                raise ValueError(f"ONNX validation failed: {validation.get('error')}")
            logger.info("pipeline.onnx_validated")
            
            # 7. Upload to S3
            if self.s3_client:
                s3_key = f"ml-models/fraud/{run_id}/fraud_model.onnx"
                self.s3_client.upload_file(
                    str(onnx_path),
                    self.config.s3_bucket,
                    s3_key,
                )
                logger.info("pipeline.uploaded_s3", key=s3_key)
            
            # 8. Save metrics
            metrics["run_id"] = run_id
            metrics["timestamp"] = datetime.utcnow().isoformat()
            metrics["s3_key"] = s3_key if self.s3_client else None
            
            logger.info("pipeline.complete", run_id=run_id, **metrics)
            return {
                "run_id": run_id,
                "s3_key": s3_key if self.s3_client else None,
                "model_path": str(model_dir),
                "onnx_path": str(onnx_path),
                **metrics,
            }
            
        except Exception as e:
            logger.error("pipeline.failed", error=str(e))
            raise
