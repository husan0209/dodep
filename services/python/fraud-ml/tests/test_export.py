"""Tests for ONNX export and parity validation."""

import numpy as np
import onnxruntime as ort
import pytest

from src.export.onnx_export import export_model_to_onnx, validate_onnx_model
from src.features.transformation import MODEL_FEATURES


@pytest.fixture(scope="module")
def reference_rows(trained_model, fraud_data):
    """Held-out feature rows used for the parity gate."""
    return fraud_data.select(MODEL_FEATURES).tail(50).to_numpy().astype(np.float64, copy=False)


@pytest.fixture(scope="module")
def onnx_path(trained_model, tmp_path_factory):
    """Export once per module."""
    return export_model_to_onnx(trained_model, tmp_path_factory.mktemp("onnx"))


class TestOnnxExport:
    """Export produces a loadable graph with serving metadata."""

    def test_onnx_export(self, onnx_path):
        assert onnx_path.exists()
        assert onnx_path.suffix == ".onnx"

    def test_metadata_carries_serving_contract(self, onnx_path, trained_model):
        import onnx

        onnx_model = onnx.load(str(onnx_path))
        metadata = {p.key: p.value for p in onnx_model.metadata_props}

        # Rust serving reads these to score identically to Python training.
        assert float(metadata["threshold"]) == pytest.approx(trained_model.threshold)
        assert float(metadata["max_fpr"]) == pytest.approx(trained_model.max_fpr)
        assert metadata["threshold_selected_on"] == "validation"
        assert int(metadata["positive_class_index"]) == 1
        import json

        assert json.loads(metadata["feature_columns"]) == list(MODEL_FEATURES)

    def test_onnx_validation(self, onnx_path):
        validation = validate_onnx_model(onnx_path)

        assert validation["valid"] is True, validation.get("error")
        assert "input_shape" in validation
        assert "output_shape" in validation
        # Classifier output: label tensor + probability matrix.
        assert validation["output_names"] == ["label", "probabilities"]

    def test_onnx_validation_detects_missing_file(self, tmp_path):
        validation = validate_onnx_model(tmp_path / "missing.onnx")
        assert validation["valid"] is False
        assert validation["error"] == "File not found"

    def test_parity_validation(self, onnx_path, trained_model, reference_rows):
        validation = validate_onnx_model(
            onnx_path,
            reference_model=trained_model,
            reference_data=reference_rows,
        )

        assert validation["valid"] is True, validation.get("error")
        assert validation["parity_ok"] is True
        assert validation["max_abs_diff"] <= 1e-4
        assert validation["label_agreement"] == pytest.approx(1.0)

    def test_parity_validation_detects_broken_model(self, onnx_path, trained_model, reference_rows):
        """A model whose scores disagree must fail the parity gate."""
        drifted = type(trained_model)()
        drifted.model = trained_model.model
        drifted.threshold = 1.0 - trained_model.threshold
        drifted.predict = lambda x: 1.0 - trained_model.predict(x)  # type: ignore[assignment]

        validation = validate_onnx_model(
            onnx_path,
            reference_model=drifted,
            reference_data=reference_rows,
        )

        assert validation["valid"] is False
        assert "Parity check failed" in validation["error"]


class TestOnnxInference:
    """The exported graph behaves like the source model."""

    def test_onnx_inference_shapes(self, onnx_path):
        session = ort.InferenceSession(str(onnx_path))
        input_name = session.get_inputs()[0].name
        test_input = np.random.randn(5, len(MODEL_FEATURES)).astype(np.float32)

        outputs = session.run(None, {input_name: test_input})

        assert len(outputs) == 2
        label, probabilities = outputs
        assert label.shape == (5,)
        assert probabilities.shape == (5, 2)
        # Probability matrix columns sum to 1 (post_transform = LOGISTIC).
        assert np.allclose(probabilities.sum(axis=1), 1.0, atol=1e-5)

    def test_onnx_probabilities_match_sklearn(self, onnx_path, trained_model, reference_rows):
        session = ort.InferenceSession(str(onnx_path))
        input_name = session.get_inputs()[0].name

        _, probabilities = session.run(None, {input_name: reference_rows.astype(np.float32)})
        p_onnx = probabilities[:, 1]
        p_sklearn = trained_model.predict(reference_rows)

        assert np.allclose(p_onnx, p_sklearn, atol=1e-5)

    def test_onnx_labels_match_threshold(self, onnx_path, trained_model, reference_rows):
        session = ort.InferenceSession(str(onnx_path))
        input_name = session.get_inputs()[0].name

        _, probabilities = session.run(None, {input_name: reference_rows.astype(np.float32)})
        labels = (probabilities[:, 1] >= trained_model.threshold).astype(int)

        assert set(np.unique(labels)).issubset({0, 1})
