import importlib

for m in ("skl2onnx.xgboost", "skl2onnx.convert", "onnxmltools"):
    try:
        importlib.import_module(m)
        print("OK  ", m)
    except Exception as exc:  # noqa: BLE001
        print("FAIL", m, type(exc).__name__, exc)
