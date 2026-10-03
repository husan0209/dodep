"""
ONNX model export and validation.

Export uses onnxmltools' *native* XGBoost converter. Do NOT use
skl2onnx.convert_sklearn() for XGBClassifier — it has no registered shape
calculator for it and fails with MissingShapeCalculator (the converter
class lives in onnxmltools, and onnxmltools >= 1.16 expects its own tensor
types, which skl2onnx rejects). The native path needs neither registration
nor the unsupported `zipmap` option.

Resulting graph:
    inputs:  features  [None, n_features] float32
    outputs: label         [None]           int64
             probabilities [None, 2]        float32 (col 1 = P(fraud))

Validation is a *parity gate*, not a smoke test: ONNX probabilities are
compared against the source model's predict_proba on reference data, and
the production threshold + feature order are embedded into the graph
metadata so the Rust serving side scores with exactly the evaluated
artifacts.
"""

import json
from pathlib import Path

import numpy as np
import onnx
import onnxruntime as ort
import structlog

logger = structlog.get_logger()

ONNX_FILE = "fraud_model.onnx"
DEFAULT_ATOL = 1e-4


def _set_metadata(onnx_model: "onnx.ModelProto", metadata: dict) -> None:
    """Embed JSON-encoded metadata props into the ONNX graph."""
    del onnx_model.metadata_props[:]
    for key, value in metadata.items():
        entry = onnx_model.metadata_props.add()
        entry.key = key
        entry.value = value if isinstance(value, str) else json.dumps(value, default=str)


def export_model_to_onnx(model, path: Path) -> Path:
    """
    Export a trained FraudModel to ONNX with serving metadata.

    Args:
        model: src.models.fraud_model.FraudModel (trained).
        path: output directory.

    Returns:
        Path to the written .onnx file.
    """
    import onnxmltools
    from onnxmltools.convert.common.data_types import FloatTensorType

    if model.model is None:
        raise ValueError("Model not trained")

    n_features = len(model.FEATURE_COLUMNS)
    onnx_model = onnxmltools.convert_xgboost(
        model.model,
        initial_types=[("features", FloatTensorType([None, n_features]))],
    )

    _set_metadata(
        onnx_model,
        {
            "model_type": "xgboost_binary_logistic",
            "feature_columns": list(model.FEATURE_COLUMNS),
            "threshold": float(model.threshold),
            "max_fpr": float(model.max_fpr),
            "target_recall": float(model.target_recall),
            "threshold_selected_on": "validation",
            "positive_class_index": 1,
        },
    )
    onnx.checker.check_model(onnx_model)

    path.mkdir(parents=True, exist_ok=True)
    onnx_path = path / ONNX_FILE
    onnx.save(onnx_model, str(onnx_path))

    logger.info(
        "onnx.exported",
        path=str(onnx_path),
        features=n_features,
        threshold=model.threshold,
    )
    return onnx_path


def _shape_to_list(dims) -> list[int | str | None]:
    """Convert ONNX TensorShapeProto dims to a JSON-friendly list."""
    result: list[int | str | None] = []
    for dim in dims:
        if dim.HasField("dim_value"):
            result.append(int(dim.dim_value))
        elif dim.HasField("dim_param"):
            result.append(dim.dim_param)
        else:
            result.append(None)
    return result


def validate_onnx_model(
    onnx_path: Path,
    reference_model=None,
    reference_data: np.ndarray | None = None,
    atol: float = DEFAULT_ATOL,
) -> dict:
    """
    Validate an exported ONNX model.

    Always checks: file exists, ONNX structure (checker), inference session,
    input/output shapes, embedded metadata.

    When reference_model + reference_data are given, additionally runs a
    parity gate: max |P_onnx - P_sklearn| <= atol over the reference rows.

    Returns:
        Dict with validation results ({"valid": bool, "error": str?} + details).
    """
    logger.info("onnx.validation_start", path=str(onnx_path))

    if not onnx_path.exists():
        logger.error("onnx.file_not_found", path=str(onnx_path))
        return {"valid": False, "error": "File not found"}

    try:
        onnx_model = onnx.load(str(onnx_path))
        onnx.checker.check_model(onnx_model)
        logger.info("onnx.structure_valid")

        graph_input = onnx_model.graph.input[0]
        graph_output = onnx_model.graph.output[0]
        input_name = graph_input.name
        input_shape = _shape_to_list(graph_input.type.tensor_type.shape.dim)
        output_shape = _shape_to_list(graph_output.type.tensor_type.shape.dim)
        output_names = [o.name for o in onnx_model.graph.output]

        opset = {imp.domain or "ai.onnx": imp.version for imp in onnx_model.opset_import}
        metadata = {p.key: p.value for p in onnx_model.metadata_props}

        session = ort.InferenceSession(str(onnx_path))
        logger.info("onnx.session_created")

        # Smoke inference with correctly-shaped random input. Fall back to
        # feature count from metadata when the batch dim is dynamic only.
        n_features = input_shape[1] if len(input_shape) == 2 else None
        if not isinstance(n_features, int) or n_features <= 0:
            n_features = len(json.loads(metadata.get("feature_columns", "[]")))
        if not n_features:
            return {"valid": False, "error": "Cannot determine input feature count"}

        test_input = np.random.randn(1, n_features).astype(np.float32)
        output = session.run(None, {input_name: test_input})
        logger.info(
            "onnx.inference_test",
            input_shape=list(test_input.shape),
            n_outputs=len(output),
        )

        result = {
            "valid": True,
            "input_name": input_name,
            "input_shape": input_shape,
            "output_shape": output_shape,
            "output_names": output_names,
            "opset": opset,
            "metadata": metadata,
        }

        # Parity gate: ONNX probabilities vs source model probabilities.
        if reference_model is not None and reference_data is not None:
            ref = np.asarray(reference_data, dtype=np.float64)
            if ref.ndim == 1:
                ref = ref.reshape(1, -1)
            p_sk = reference_model.predict(ref)
            onnx_out = session.run(None, {input_name: ref.astype(np.float32)})
            probabilities = _extract_probabilities(onnx_out)
            max_abs_diff = float(np.max(np.abs(p_sk - probabilities)))

            threshold = float(metadata.get("threshold", reference_model.threshold))
            p_onnx_fraud = probabilities
            label_sk = (p_sk >= threshold).astype(int)
            label_on = (p_onnx_fraud >= threshold).astype(int)
            label_agreement = float(np.mean(label_sk == label_on))

            result.update(
                {
                    "max_abs_diff": max_abs_diff,
                    "label_agreement": label_agreement,
                    "parity_atol": atol,
                    "parity_ok": max_abs_diff <= atol and label_agreement >= 0.999,
                }
            )
            if not result["parity_ok"]:
                result["valid"] = False
                result["error"] = (
                    f"Parity check failed: max_abs_diff={max_abs_diff:.3e} "
                    f"(atol={atol}), label_agreement={label_agreement:.4f}"
                )
                logger.error(
                    "onnx.parity_failed",
                    **{k: result[k] for k in ("max_abs_diff", "label_agreement")},
                )
                return result
            logger.info(
                "onnx.parity_ok",
                max_abs_diff=max_abs_diff,
                label_agreement=label_agreement,
            )

        return result

    except Exception as exc:
        logger.error("onnx.validation_error", error=str(exc))
        return {"valid": False, "error": str(exc)}


def _extract_probabilities(onnx_output: list) -> np.ndarray:
    """Extract P(positive class) from the ONNX classifier outputs."""
    for item in onnx_output:
        arr = np.asarray(item)
        if arr.ndim == 2 and arr.shape[1] == 2:
            return arr[:, 1]
    raise ValueError(f"Unexpected ONNX outputs: {[np.asarray(o).shape for o in onnx_output]}")
