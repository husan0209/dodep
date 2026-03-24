"""Tests for ONNX export."""
import pytest
import polars as pl
import numpy as np
from pathlib import Path

from src.models.fraud_model import FraudModel
from src.export.onnx_export import validate_onnx_model


class TestOnnxExport:
    """Test ONNX model export."""
    
    @pytest.fixture
    def trained_model(self):
        """Create trained model."""
        np.random.seed(42)
        n_samples = 1000
        
        data = {
            "bets_7d": np.random.randint(0, 50, n_samples),
            "bets_24h": np.random.randint(0, 20, n_samples),
            "avg_bet_30d": np.random.uniform(10, 500, n_samples),
            "std_bet_30d": np.random.uniform(5, 200, n_samples),
            "max_bet_30d": np.random.uniform(100, 5000, n_samples),
            "deposits_24h": np.random.randint(0, 10, n_samples),
            "total_deposit_30d": np.random.uniform(0, 10000, n_samples),
            "device_count_30d": np.random.randint(1, 10, n_samples),
            "ip_count_30d": np.random.randint(1, 50, n_samples),
            "country_count_30d": np.random.randint(1, 5, n_samples),
            "wins_7d": np.random.randint(0, 20, n_samples),
            "settled_7d": np.random.randint(0, 50, n_samples),
            "account_age_days": np.random.randint(1, 1000, n_samples),
            "win_rate_7d": np.random.uniform(0, 1, n_samples),
            "bet_cv_30d": np.random.uniform(0, 5, n_samples),
            "deposit_bet_ratio": np.random.uniform(0, 10, n_samples),
            "multi_device": np.random.randint(0, 2, n_samples),
            "multi_ip": np.random.randint(0, 2, n_samples),
            "high_roller": np.random.randint(0, 2, n_samples),
            "rapid_bettor": np.random.randint(0, 2, n_samples),
            "is_fraud": np.random.choice([0, 1], n_samples, p=[0.95, 0.05]),
        }
        
        df = pl.DataFrame(data)
        model = FraudModel()
        model.train(df)
        return model
    
    def test_onnx_export(self, trained_model, tmp_path):
        """Test ONNX export."""
        onnx_path = trained_model.export_onnx(tmp_path)
        
        assert onnx_path.exists()
        assert onnx_path.suffix == ".onnx"
    
    def test_onnx_validation(self, trained_model, tmp_path):
        """Test ONNX model validation."""
        onnx_path = trained_model.export_onnx(tmp_path)
        validation = validate_onnx_model(onnx_path)
        
        assert validation["valid"] is True
        assert "input_shape" in validation
        assert "output_shape" in validation
    
    def test_onnx_inference(self, trained_model, tmp_path):
        """Test ONNX model inference."""
        import onnxruntime as ort
        
        onnx_path = trained_model.export_onnx(tmp_path)
        session = ort.InferenceSession(str(onnx_path))
        input_name = session.get_inputs()[0].name
        
        # Test inference
        test_input = np.random.randn(5, 18).astype(np.float32)
        output = session.run(None, {input_name: test_input})
        
        assert len(output) == 1
        assert output[0].shape == (5, 2)  # 5 samples, 2 classes
