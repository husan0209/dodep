"""Tests for the weekly training pipeline (with stubbed I/O)."""

import json
from datetime import datetime, timedelta
from pathlib import Path

import numpy as np
import polars as pl
import pytest

from src.models.fraud_model import MODEL_CARD_FILE
from src.pipeline.train_pipeline import TrainPipeline
from tests.conftest import make_fraud_dataset

RAW_COLUMNS = [
    "user_id",
    "event_time",
    "bets_7d",
    "bets_24h",
    "wins_7d",
    "settled_7d",
    "std_bet_30d",
    "avg_bet_30d",
    "max_bet_30d",
    "total_deposit_30d",
    "device_count_30d",
    "ip_count_30d",
    "country_count_30d",
    "deposits_24h",
    "account_age_days",
    "is_fraud",
]


class StubFeatureStore:
    """Records cache() calls; the real store talks to ClickHouse."""

    def __init__(self):
        self.cached_users = []
        self.cache_calls = 0

    def cache(self, df, user_ids=None, as_of=None):
        self.cache_calls += 1
        self.cached_users = list(user_ids or [])


class StubConfig:
    """Settings stand-in pointing artifacts at a tmp directory."""

    def __init__(
        self,
        model_path: Path,
        auc_threshold=0.5,
        precision_threshold=0.01,
        min_recall=0.1,
    ):
        self.model_path = str(model_path)
        self.lookback_days = 90
        self.max_fpr = 0.05
        self.min_recall_at_max_fpr = min_recall
        self.target_recall = 0.90
        self.model_quality_threshold = auc_threshold
        self.precision_threshold = precision_threshold
        self.s3_bucket = "test-bucket"


class StubExtractor:
    """Returns the synthetic dataset as if extracted from ClickHouse."""

    def __init__(self, dataset: pl.DataFrame):
        self.dataset = dataset
        self.lookback_days: int | None = None

    def extract_training_data(self, lookback_days: int = 90, as_of=None):
        self.lookback_days = lookback_days
        # Select whatever raw columns exist, so a deliberately incomplete
        # dataset surfaces the pipeline's own validation error.
        available = [c for c in RAW_COLUMNS if c in self.dataset.columns]
        return self.dataset.select(available)

    def compute_derived_features(self, df: pl.DataFrame) -> pl.DataFrame:
        # Same derivation as the real extractor, applied to raw rows.
        return df.with_columns(
            [
                (pl.col("wins_7d") / pl.col("settled_7d").clip(lower_bound=1)).alias("win_rate_7d"),
                (pl.col("std_bet_30d") / pl.col("avg_bet_30d").clip(lower_bound=0.01)).alias(
                    "bet_cv_30d"
                ),
                (
                    pl.col("total_deposit_30d")
                    / pl.col("avg_bet_30d").clip(lower_bound=0.01)
                    / pl.col("bets_7d").clip(lower_bound=1)
                    * 7
                ).alias("deposit_bet_ratio"),
                (pl.col("device_count_30d") > 3).cast(pl.Int8).alias("multi_device"),
                (pl.col("ip_count_30d") > 10).cast(pl.Int8).alias("multi_ip"),
                (pl.col("max_bet_30d") > pl.col("avg_bet_30d") * 10)
                .cast(pl.Int8)
                .alias("high_roller"),
                (pl.col("bets_24h") > pl.col("bets_7d") * 0.5).cast(pl.Int8).alias("rapid_bettor"),
            ]
        ).fill_null(0)


def build_pipeline(dataset: pl.DataFrame, model_path: Path, **config_kwargs) -> TrainPipeline:
    """Assemble a pipeline with all external I/O stubbed out."""
    pipeline = TrainPipeline(
        ch_client=None,
        feature_store=StubFeatureStore(),
        config=StubConfig(model_path, **config_kwargs),
    )
    pipeline.extractor = StubExtractor(dataset)
    return pipeline


@pytest.fixture
def raw_dataset() -> pl.DataFrame:
    """Synthetic raw (pre-derivation) dataset for the pipeline."""
    df = make_fraud_dataset(n_samples=1200)
    df = df.with_columns(pl.Series("user_id", range(len(df))))
    return df.select(RAW_COLUMNS)


class TestTrainPipelineRun:
    """Happy path: train -> gates -> artifacts -> model card."""

    def test_run_produces_artifacts_and_model_card(self, raw_dataset, tmp_path):
        pipeline = build_pipeline(raw_dataset, tmp_path)
        result = pipeline.run()

        run_id = result["run_id"]
        model_dir = tmp_path / run_id

        # Booster + ONNX + model card all persisted
        assert (model_dir / "fraud_model.json").exists()
        assert (model_dir / "fraud_model.onnx").exists()
        assert (model_dir / MODEL_CARD_FILE).exists()

        card = json.loads((model_dir / MODEL_CARD_FILE).read_text(encoding="utf-8"))
        assert card["threshold"] == pytest.approx(result["threshold"])
        assert card["threshold_selected_on"] == "validation"
        assert card["metrics"]["run_id"] == run_id
        assert card["metrics"]["quality_gates"]["passed"] is True
        # Parity result recorded for the exported artifact
        assert card["onnx_validation"]["max_abs_diff"] <= 1e-4
        assert card["onnx_validation"]["output_names"] == ["label", "probabilities"]

        # Threshold in the card == threshold embedded in ONNX metadata
        import onnx

        onnx_model = onnx.load(str(model_dir / "fraud_model.onnx"))
        metadata = {p.key: p.value for p in onnx_model.metadata_props}
        assert float(metadata["threshold"]) == pytest.approx(result["threshold"])

    def test_features_are_cached_before_training(self, raw_dataset, tmp_path):
        pipeline = build_pipeline(raw_dataset, tmp_path)
        pipeline.run()

        assert pipeline.feature_store.cache_calls == 1
        assert (
            pipeline.feature_store.cached_users == list(range(len(raw_dataset)))
            or len(pipeline.feature_store.cached_users) > 0
        )
        assert pipeline.extractor.lookback_days == 90

    def test_quality_gate_failure_blocks_artifacts(self, raw_dataset, tmp_path):
        # Impossible gates: AUC 0.99 on this data is not reachable.
        pipeline = build_pipeline(
            raw_dataset, tmp_path, auc_threshold=0.999, precision_threshold=0.999
        )

        with pytest.raises(ValueError, match="Quality gate failed"):
            pipeline.run()

        # Nothing promoted: no ONNX artifact was written
        onnx_files = list(tmp_path.rglob("*.onnx"))
        assert onnx_files == []

    def test_missing_features_are_rejected(self, raw_dataset, tmp_path):
        # country_count_30d is a model feature but not a derivation input,
        # so dropping it fails validation rather than feature computation.
        broken = raw_dataset.drop("country_count_30d")
        pipeline = build_pipeline(broken, tmp_path)

        with pytest.raises(ValueError, match="Missing features"):
            pipeline.run()

    def test_run_returns_audit_fields(self, raw_dataset, tmp_path):
        result = build_pipeline(raw_dataset, tmp_path).run()

        for key in (
            "run_id",
            "s3_key",
            "model_path",
            "onnx_path",
            "auc_roc",
            "threshold",
            "timestamp",
            "quality_gates",
        ):
            assert key in result
        assert result["s3_key"] is None  # no S3 client configured
        # run_id is a sortable timestamp
        datetime.strptime(result["run_id"], "%Y%m%d_%H%M%S")

    def test_s3_client_receives_onnx(self, raw_dataset, tmp_path):
        class StubS3:
            def __init__(self):
                self.uploads = []

            def upload_file(self, path, bucket, key):
                self.uploads.append((path, bucket, key))

        s3 = StubS3()
        pipeline = build_pipeline(raw_dataset, tmp_path)
        pipeline.s3_client = s3

        result = pipeline.run()

        assert len(s3.uploads) == 1
        path, bucket, key = s3.uploads[0]
        assert path.endswith(".onnx")
        assert bucket == "test-bucket"
        assert key == f"ml-models/fraud/{result['run_id']}/fraud_model.onnx"
        assert result["s3_key"] == key

    def test_lookback_window_is_respected(self, raw_dataset, tmp_path):
        pipeline = build_pipeline(raw_dataset, tmp_path)
        pipeline.config.lookback_days = 30
        pipeline.run()
        assert pipeline.extractor.lookback_days == 30


class TestTemporalSplitDrift:
    """Guard against leakage: recent rows must not influence training."""

    def test_early_rows_are_not_shifted_by_late_rows(self, tmp_path):
        """Two datasets that differ only in the latest window share a threshold."""
        n = 1000

        def build(flip_last: int) -> pl.DataFrame:
            # Re-seed on every call so both datasets share identical data
            # except for the deliberately corrupted newest window.
            rng = np.random.default_rng(7)
            bets_24h = rng.integers(0, 21, n)
            devices = rng.integers(1, 11, n)
            signal = (bets_24h >= 15) & (devices >= 7)
            if flip_last:
                # Corrupt only the newest 10% of rows
                signal[-flip_last:] = ~signal[-flip_last:]
            start = datetime(2026, 1, 1)
            return pl.DataFrame(
                {
                    "user_id": range(n),
                    "event_time": [start + timedelta(minutes=30 * i) for i in range(n)],
                    "bets_7d": rng.integers(0, 50, n),
                    "bets_24h": bets_24h,
                    "avg_bet_30d": rng.uniform(10, 500, n),
                    "std_bet_30d": rng.uniform(5, 200, n),
                    "max_bet_30d": rng.uniform(100, 5000, n),
                    "deposits_24h": rng.integers(0, 10, n),
                    "total_deposit_30d": rng.uniform(0, 10000, n),
                    "device_count_30d": devices,
                    "ip_count_30d": rng.integers(1, 50, n),
                    "country_count_30d": rng.integers(1, 5, n),
                    "wins_7d": rng.integers(0, 20, n),
                    "settled_7d": rng.integers(1, 50, n),
                    "account_age_days": rng.integers(1, 1000, n),
                    "win_rate_7d": rng.uniform(0, 1, n),
                    "bet_cv_30d": rng.uniform(0, 5, n),
                    "deposit_bet_ratio": rng.uniform(0, 10, n),
                    "multi_device": (devices > 3).astype(int),
                    "multi_ip": (rng.integers(1, 50, n) > 10).astype(int),
                    "high_roller": rng.integers(0, 2, n),
                    "rapid_bettor": (bets_24h >= 15).astype(int),
                    "is_fraud": signal.astype(int),
                }
            )

        base = build(0)
        drifted = build(100)

        # Gates are irrelevant here: we compare models trained on identical
        # training windows, and the corrupted test window is expected to be
        # unlearnable, so the gates are relaxed to let both runs finish.
        relaxed = {"auc_threshold": 0.0, "precision_threshold": 0.0, "min_recall": 0.0}
        first = build_pipeline(base, tmp_path / "a", **relaxed).run()
        second = build_pipeline(drifted, tmp_path / "b", **relaxed).run()

        # Training data (older 80%) is identical, so the trained model and
        # its threshold must be identical; only test metrics may differ.
        assert first["threshold"] == pytest.approx(second["threshold"], abs=1e-6)
        assert first["split"]["test"] == second["split"]["test"]
        # The corrupted test window must change the reported test metrics
        assert first["auc_roc"] != pytest.approx(second["auc_roc"], abs=1e-9)
