"""Tests for the HTTP layer.

The API previously resolved the fraud detector via
`globals().get('request')`, which is always None, so every endpoint
returned 503. These tests pin the injected-`Request` behaviour.
"""

from datetime import datetime
from types import SimpleNamespace

import pandas as pd
import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient

from internal.api.routes import router


class FakeFraudPrediction:
    """Stand-in for internal.models.fraud_detector.FraudPrediction.

    risk_score follows the 0-100 scale used by the real detectors.
    """

    def __init__(self, user_id: int, fraud_type: str, risk_score: float):
        self.user_id = user_id
        self.fraud_type = fraud_type
        self.risk_score = risk_score
        self.is_fraud = risk_score >= 60.0
        self.confidence = 0.9
        self.features: dict[str, float] = {}
        self.timestamp = datetime(2026, 1, 1, 12, 0, 0)
        self.explanation = "test"


class FakeDetector:
    """Minimal detector implementing the routes' expected interface."""

    def __init__(self):
        self.reloaded = False
        self.bet_anomaly_detector = SimpleNamespace(is_fitted=True)
        self.bonus_abuse_detector = SimpleNamespace(is_fitted=True)
        self.payment_fraud_detector = SimpleNamespace(is_fitted=True)
        self.account_takeover_detector = SimpleNamespace(is_fitted=False)

    def detect_bet_anomaly(self, user_id, bets_df):
        assert isinstance(bets_df, pd.DataFrame)
        return FakeFraudPrediction(user_id, "bet_anomaly", 85.0)

    def detect_bonus_abuse(self, user_id, features):
        return FakeFraudPrediction(user_id, "bonus_abuse", 30.0)

    def detect_payment_fraud(self, user_id, tx_df):
        assert isinstance(tx_df, pd.DataFrame)
        return FakeFraudPrediction(user_id, "payment_fraud", 75.0)

    def detect_account_takeover(self, user_id, features):
        return FakeFraudPrediction(user_id, "account_takeover", 95.0)

    def load_models(self):
        self.reloaded = True

    def get_status(self):
        return {
            "bet_anomaly": {"loaded": self.bet_anomaly_detector.is_fitted},
            "bonus_abuse": {"loaded": self.bonus_abuse_detector.is_fitted},
            "payment_fraud": {"loaded": self.payment_fraud_detector.is_fitted},
            "account_takeover": {"loaded": self.account_takeover_detector.is_fitted},
        }


@pytest.fixture
def client_with_detector():
    app = FastAPI()
    app.include_router(router, prefix="/api/v1")
    app.state.fraud_detector = FakeDetector()
    return TestClient(app)


@pytest.fixture
def client_without_detector():
    """Detector not initialized (startup not run / dependency lost)."""
    app = FastAPI()
    app.include_router(router, prefix="/api/v1")
    return TestClient(app)


BET_PAYLOAD = [
    {
        "user_id": 42,
        "stake": 250.0,
        "odds": 2.1,
        "status": "pending",
        "placed_at": "2026-01-01T10:00:00",
    }
]

TX_PAYLOAD = [
    {
        "user_id": 42,
        "amount": 1000.0,
        "status": "completed",
        "payment_method": "card",
        "created_at": "2026-01-01T10:00:00",
    }
]


class TestDetectEndpoints:
    """Detection endpoints reach the detector when it is available."""

    def test_detect_bet_anomaly(self, client_with_detector):
        resp = client_with_detector.post("/api/v1/detect/bet-anomaly", json=BET_PAYLOAD)

        assert resp.status_code == 200
        body = resp.json()
        assert body["user_id"] == 42
        assert body["fraud_type"] == "bet_anomaly"
        assert body["is_fraud"] is True
        assert 0.0 <= body["risk_score"] <= 100.0

    def test_detect_bonus_abuse(self, client_with_detector):
        resp = client_with_detector.post(
            "/api/v1/detect/bonus-abuse",
            json={"user_id": 7, "bonuses_claimed_24h": 6, "account_age_days": 1},
        )

        assert resp.status_code == 200
        assert resp.json()["fraud_type"] == "bonus_abuse"
        assert resp.json()["is_fraud"] is False

    def test_detect_payment_fraud(self, client_with_detector):
        resp = client_with_detector.post("/api/v1/detect/payment-fraud", json=TX_PAYLOAD)

        assert resp.status_code == 200
        assert resp.json()["fraud_type"] == "payment_fraud"

    def test_detect_account_takeover(self, client_with_detector):
        resp = client_with_detector.post(
            "/api/v1/detect/account-takeover",
            json={
                "user_id": 9,
                "ip": "203.0.113.5",
                "country": "NL",
                "device_fingerprint": "dev-abc",
                "hour": 3,
                "failed_attempts_24h": 6,
                "locations_24h": 3,
            },
        )

        assert resp.status_code == 200
        assert resp.json()["is_fraud"] is True

    def test_detect_rejects_empty_input(self, client_with_detector):
        resp = client_with_detector.post("/api/v1/detect/bet-anomaly", json=[])
        assert resp.status_code == 400
        assert resp.json()["detail"] == "No bets provided"

    def test_detect_rejects_invalid_payload(self, client_with_detector):
        resp = client_with_detector.post(
            "/api/v1/detect/bet-anomaly",
            json=[{"user_id": "not-an-int"}],
        )
        assert resp.status_code == 422


class TestUnavailableDetector:
    """Without a detector the service reports 503 instead of crashing."""

    @pytest.mark.parametrize(
        "path,payload",
        [
            ("/api/v1/detect/bet-anomaly", BET_PAYLOAD),
            ("/api/v1/detect/bonus-abuse", {"user_id": 1}),
            ("/api/v1/detect/payment-fraud", TX_PAYLOAD),
            (
                "/api/v1/detect/account-takeover",
                {
                    "user_id": 1,
                    "ip": "1.1.1.1",
                    "country": "US",
                    "device_fingerprint": "d",
                    "hour": 12,
                },
            ),
        ],
    )
    def test_detect_endpoints_503(self, client_without_detector, path, payload):
        resp = client_without_detector.post(path, json=payload)
        assert resp.status_code == 503
        assert resp.json()["detail"] == "Fraud detector not available"

    def test_models_status_503(self, client_without_detector):
        assert client_without_detector.get("/api/v1/models/status").status_code == 503

    def test_reload_503(self, client_without_detector):
        assert client_without_detector.post("/api/v1/models/reload").status_code == 503


class TestBatchAndModelRoutes:
    """Batch scoring and model management."""

    def test_detect_batch(self, client_with_detector):
        resp = client_with_detector.post(
            "/api/v1/detect/batch",
            json={"bets": BET_PAYLOAD, "transactions": TX_PAYLOAD},
        )

        assert resp.status_code == 200
        body = resp.json()
        assert body["total_analyzed"] == 2
        assert body["fraud_detected"] == 2
        assert body["processing_time_ms"] >= 0
        assert {p["fraud_type"] for p in body["predictions"]} == {
            "bet_anomaly",
            "payment_fraud",
        }

    def test_models_status(self, client_with_detector):
        resp = client_with_detector.get("/api/v1/models/status")

        assert resp.status_code == 200
        body = resp.json()
        assert body["bet_anomaly"]["loaded"] is True
        assert body["account_takeover"]["loaded"] is False

    def test_models_reload(self, client_with_detector):
        resp = client_with_detector.post("/api/v1/models/reload")

        assert resp.status_code == 200
        assert resp.json() == {"status": "models reloaded"}

    def test_statistics(self, client_with_detector):
        resp = client_with_detector.get("/api/v1/statistics")

        assert resp.status_code == 200
        assert "total_scans_24h" in resp.json()


class TestServiceProbes:
    """Health/readiness probes of the real app."""

    def test_health_is_healthy(self):
        from main import app

        client = TestClient(app)
        assert client.get("/health").json() == {"status": "healthy"}

    def test_ready_is_503_before_startup(self):
        # Lifespan is not entered, so the detector is missing.
        from main import app

        client = TestClient(app)
        resp = client.get("/ready")
        assert resp.status_code == 503
        assert resp.json()["status"] == "not_ready"

    def test_app_starts_without_clickhouse_and_kafka(self, monkeypatch):
        """
        Full lifespan runs with both backing services unavailable.

        Startup must degrade gracefully: detection endpoints work off
        in-memory detectors, so a missing ClickHouse/Kafka must not crash
        the process (the training job connects on its own).
        """
        import main
        from internal.consumers import redpanda_consumer as consumer_module
        from src.data import clickhouse as ch_module

        def boom(*args, **kwargs):
            raise ConnectionError("service unavailable")

        monkeypatch.setattr(ch_module.ClickHouseClient, "__init__", boom)
        monkeypatch.setattr(consumer_module, "Consumer", boom)

        with TestClient(main.app) as client:
            # Readiness depends only on the detector, not on the bus/store.
            assert client.get("/ready").status_code == 200
            assert client.get("/health").status_code == 200

            resp = client.post("/api/v1/detect/bonus-abuse", json={"user_id": 1})
            assert resp.status_code == 200
            assert resp.json()["is_fraud"] is False
