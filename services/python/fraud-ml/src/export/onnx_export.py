"""
ONNX model export and validation.
"""
from pathlib import Path

import numpy as np
import onnx
import onnxruntime as ort
import structlog

logger = structlog.get_logger()


def validate_onnx_model(onnx_path: Path) -> dict:
    """
    Validate ONNX model.

    Returns:
        Dict with validation results
    """
    logger.info("onnx.validation_start", path=str(onnx_path))

    if not onnx_path.exists():
        logger.error("onnx.file_not_found", path=str(onnx_path))
        return {"valid": False, "error": "File not found"}

    try:
        # Load and check model structure
        onnx_model = onnx.load(str(onnx_path))
        onnx.checker.check_model(onnx_model)
        logger.info("onnx.structure_valid")

        # Get model info
        input_name = onnx_model.graph.input[0].name
        input_shape = [d.dim_value for d in onnx_model.graph.input[0].type.tensor_type.shape.dim]

        # Classifier graphs exported by the XGBoost converter emit two outputs,
        # "label" then "probabilities"; callers care about the probabilities.
        graph_outputs = list(onnx_model.graph.output)
        output_name = graph_outputs[0].name
        prob_output = next(
            (o for o in graph_outputs if "probabilit" in o.name.lower()),
            graph_outputs[-1],
        )
        output_shape = [d.dim_value for d in prob_output.type.tensor_type.shape.dim]

        # Create inference session
        session = ort.InferenceSession(str(onnx_path))
        logger.info("onnx.session_created")

        # Test inference with random input
        test_input = np.random.randn(1, input_shape[1]).astype(np.float32)
        probs = session.run([prob_output.name], {input_name: test_input})[0]

        logger.info(
            "onnx.inference_test",
            input_shape=test_input.shape,
            output_shape=probs.shape,
        )

        return {
            "valid": True,
            "input_name": input_name,
            "output_name": output_name,
            "prob_output_name": prob_output.name,
            "input_shape": input_shape,
            "output_shape": output_shape,
            "test_output_shape": list(probs.shape),
        }

    except onnx.checker.ValidationError as e:
        logger.error("onnx.validation_failed", error=str(e))
        return {"valid": False, "error": f"ONNX validation: {str(e)}"}
    except Exception as e:
        logger.error("onnx.validation_error", error=str(e))
        return {"valid": False, "error": str(e)}


def optimize_onnx_model(onnx_path: Path, output_path: Path | None = None) -> Path:
    """
    Optimize ONNX model for inference.

    Returns path to optimized model.
    """
    from onnxruntime.transformers.optimizer import optimize_model

    if output_path is None:
        output_path = onnx_path.parent / f"{onnx_path.stem}_optimized.onnx"

    logger.info("onnx.optimization_start", path=str(onnx_path))

    try:
        optimized = optimize_model(
            str(onnx_path),
            model_type="tree_ensemble",
            num_heads=0,
            hidden_size=0,
        )
        optimized.save_model_to_file(str(output_path))

        logger.info("onnx.optimization_complete", path=str(output_path))
        return output_path

    except Exception as e:
        logger.error("onnx.optimization_failed", error=str(e))
        return onnx_path


def compare_predictions(
    original_model,
    onnx_path: Path,
    test_data: np.ndarray,
    tolerance: float = 0.01
) -> dict:
    """
    Compare predictions between original and ONNX model.

    Returns:
        Dict with comparison results
    """
    logger.info("onnx.comparison_start", samples=len(test_data))

    # Original model predictions
    original_preds = original_model.predict(test_data)

    # ONNX model predictions - the graph exposes "label" and "probabilities",
    # so pick the probability output and reduce it to the positive class to
    # match original_model.predict().
    session = ort.InferenceSession(str(onnx_path))
    input_name = session.get_inputs()[0].name
    output_names = [o.name for o in session.get_outputs()]
    prob_name = next((n for n in output_names if "probabilit" in n.lower()), output_names[-1])
    raw = session.run([prob_name], {input_name: test_data.astype(np.float32)})[0]
    onnx_preds = raw[:, 1] if raw.ndim == 2 else raw

    # Compare
    max_diff = np.max(np.abs(original_preds - onnx_preds))
    mean_diff = np.mean(np.abs(original_preds - onnx_preds))
    matches = np.sum(np.abs(original_preds - onnx_preds) < tolerance)
    match_rate = matches / len(test_data)

    logger.info(
        "onnx.comparison_complete",
        max_diff=float(max_diff),
        mean_diff=float(mean_diff),
        match_rate=float(match_rate),
    )

    return {
        "max_diff": float(max_diff),
        "mean_diff": float(mean_diff),
        "match_rate": float(match_rate),
        "within_tolerance": match_rate == 1.0,
    }
