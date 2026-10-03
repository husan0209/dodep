"""
API Routes for Fraud ML Service.

Detectors live in `app.state.fraud_detector` (wired in main.py lifespan);
handlers access it through FastAPI's injected `Request` — the previous
`globals().get('request')` pattern always resolved to None, making every
endpoint return 503.
"""

import time
from datetime import datetime
from typing import Any

import pandas as pd
from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel, Field

router = APIRouter()


# ============ Request/Response Models ============


class BetData(BaseModel):
    """Bet data for anomaly detection"""

    user_id: int
    stake: float
    odds: float
    status: str  # pending, active, won, lost, void
    placed_at: datetime
    sport_id: int | None = None
    event_id: int | None = None


class UserData(BaseModel):
    """User data for bonus abuse detection"""

    user_id: int
    bonuses_claimed_24h: int = 0
    bonus_amount_total: float = 0
    wagering_completed: float = 0
    withdrawal_after_bonus: bool = False
    account_age_days: int = 0
    deposits_count: int = 0
    total_deposits: float = 0


class TransactionData(BaseModel):
    """Transaction data for payment fraud detection"""

    user_id: int
    amount: float
    status: str  # pending, completed, failed
    payment_method: str
    created_at: datetime


class LoginData(BaseModel):
    """Login data for account takeover detection"""

    user_id: int
    ip: str
    country: str
    device_fingerprint: str
    hour: int
    hours_since_last_login: int = 24
    failed_attempts_24h: int = 0
    locations_24h: int = 1


class FraudPredictionResponse(BaseModel):
    """Fraud prediction response"""

    user_id: int
    fraud_type: str
    risk_score: float = Field(ge=0.0, le=100.0)
    is_fraud: bool
    confidence: float = Field(ge=0.0, le=1.0)
    timestamp: datetime
    explanation: str | None = None


class BatchFraudResponse(BaseModel):
    """Batch fraud detection response"""

    predictions: list[FraudPredictionResponse]
    total_analyzed: int
    fraud_detected: int
    processing_time_ms: float


# ============ Helpers ============


def _get_detector(request: Request):
    """Resolve the fraud detector from app state or fail with 503."""
    detector = getattr(request.app.state, "fraud_detector", None)
    if detector is None:
        raise HTTPException(status_code=503, detail="Fraud detector not available")
    return detector


def _to_response(prediction: Any) -> FraudPredictionResponse:
    """Map a FraudPrediction dataclass to the API response model."""
    return FraudPredictionResponse(
        user_id=prediction.user_id,
        fraud_type=prediction.fraud_type,
        risk_score=prediction.risk_score,
        is_fraud=prediction.is_fraud,
        confidence=prediction.confidence,
        timestamp=prediction.timestamp,
        explanation=prediction.explanation,
    )


def _require_non_empty(items: list, detail: str) -> None:
    if not items:
        raise HTTPException(status_code=400, detail=detail)


# ============ Fraud Detection Endpoints ============


@router.post("/detect/bet-anomaly", response_model=FraudPredictionResponse)
async def detect_bet_anomaly(request: Request, bets: list[BetData]):
    """
    Detect anomalous betting patterns.

    Analyzes betting history for suspicious patterns including:
    - Unusual bet amounts
    - Rapid betting
    - Night betting patterns
    - Abnormal win rates
    """
    _require_non_empty(bets, "No bets provided")
    fraud_detector = _get_detector(request)

    bets_df = pd.DataFrame([b.model_dump() for b in bets])
    bets_df["placed_at"] = pd.to_datetime(bets_df["placed_at"])

    user_id = bets[0].user_id
    prediction = fraud_detector.detect_bet_anomaly(user_id, bets_df)
    return _to_response(prediction)


@router.post("/detect/bonus-abuse", response_model=FraudPredictionResponse)
async def detect_bonus_abuse(request: Request, user_data: UserData):
    """
    Detect bonus abuse patterns.

    Analyzes user behavior for bonus abuse including:
    - Multiple bonus claims
    - Low wagering completion
    - Immediate withdrawal after bonus
    - New account with high bonus activity
    """
    fraud_detector = _get_detector(request)
    prediction = fraud_detector.detect_bonus_abuse(user_data.user_id, user_data.model_dump())
    return _to_response(prediction)


@router.post("/detect/payment-fraud", response_model=FraudPredictionResponse)
async def detect_payment_fraud(request: Request, transactions: list[TransactionData]):
    """
    Detect payment fraud patterns.

    Analyzes transaction history for fraud including:
    - Rapid transactions
    - Multiple payment methods
    - High failed transaction ratio
    - Unusual transaction amounts
    """
    _require_non_empty(transactions, "No transactions provided")
    fraud_detector = _get_detector(request)

    tx_df = pd.DataFrame([t.model_dump() for t in transactions])
    tx_df["created_at"] = pd.to_datetime(tx_df["created_at"])

    user_id = transactions[0].user_id
    prediction = fraud_detector.detect_payment_fraud(user_id, tx_df)
    return _to_response(prediction)


@router.post("/detect/account-takeover", response_model=FraudPredictionResponse)
async def detect_account_takeover(request: Request, login_data: LoginData):
    """
    Detect account takeover attempts.

    Analyzes login patterns for takeover attempts including:
    - New location/device
    - Unusual login time
    - Multiple failed attempts
    - Logins from multiple locations
    """
    fraud_detector = _get_detector(request)
    prediction = fraud_detector.detect_account_takeover(login_data.user_id, login_data.model_dump())
    return _to_response(prediction)


# ============ Batch Detection ============


@router.post("/detect/batch", response_model=BatchFraudResponse)
async def detect_batch(
    request: Request,
    bets: list[BetData] | None = None,
    transactions: list[TransactionData] | None = None,
):
    """
    Batch fraud detection for multiple data types.

    Processes bets and transactions in batch for efficiency.
    """
    fraud_detector = _get_detector(request)
    start_time = time.time()

    predictions: list[FraudPredictionResponse] = []
    fraud_count = 0

    if bets:
        bets_df = pd.DataFrame([b.model_dump() for b in bets])
        bets_df["placed_at"] = pd.to_datetime(bets_df["placed_at"])

        for user_id in bets_df["user_id"].unique():
            user_bets = bets_df[bets_df["user_id"] == user_id]
            pred = fraud_detector.detect_bet_anomaly(user_id, user_bets)
            if pred.is_fraud:
                fraud_count += 1
            predictions.append(_to_response(pred))

    if transactions:
        tx_df = pd.DataFrame([t.model_dump() for t in transactions])
        tx_df["created_at"] = pd.to_datetime(tx_df["created_at"])

        for user_id in tx_df["user_id"].unique():
            user_txs = tx_df[tx_df["user_id"] == user_id]
            pred = fraud_detector.detect_payment_fraud(user_id, user_txs)
            if pred.is_fraud:
                fraud_count += 1
            predictions.append(_to_response(pred))

    processing_time = (time.time() - start_time) * 1000

    return BatchFraudResponse(
        predictions=predictions,
        total_analyzed=len(predictions),
        fraud_detected=fraud_count,
        processing_time_ms=processing_time,
    )


# ============ Model Management ============


@router.get("/models/status")
async def get_models_status(request: Request):
    """Get status of all ML models"""
    fraud_detector = _get_detector(request)
    return fraud_detector.get_status()


@router.post("/models/reload")
async def reload_models(request: Request):
    """Reload all ML models from disk"""
    fraud_detector = _get_detector(request)
    fraud_detector.load_models()
    return {"status": "models reloaded"}


# ============ Statistics ============


@router.get("/statistics")
async def get_statistics():
    """Get fraud detection statistics"""
    # This would typically query a database for historical stats
    return {
        "total_scans_24h": 0,
        "fraud_detected_24h": 0,
        "false_positive_rate": 0.0,
        "avg_processing_time_ms": 0.0,
    }
