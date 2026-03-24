"""
API Routes for Fraud ML Service
"""

from fastapi import APIRouter, HTTPException, BackgroundTasks
from pydantic import BaseModel, Field
from typing import List, Optional, Dict, Any
from datetime import datetime

router = APIRouter()


# ============ Request/Response Models ============

class BetData(BaseModel):
    """Bet data for anomaly detection"""
    user_id: int
    stake: float
    odds: float
    status: str  # pending, active, won, lost, void
    placed_at: datetime
    sport_id: Optional[int] = None
    event_id: Optional[int] = None


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
    risk_score: float
    is_fraud: bool
    confidence: float
    timestamp: datetime
    explanation: Optional[str] = None


class BatchFraudResponse(BaseModel):
    """Batch fraud detection response"""
    predictions: List[FraudPredictionResponse]
    total_analyzed: int
    fraud_detected: int
    processing_time_ms: float


# ============ Fraud Detection Endpoints ============

@router.post("/detect/bet-anomaly", response_model=FraudPredictionResponse)
async def detect_bet_anomaly(bets: List[BetData]):
    """
    Detect anomalous betting patterns
    
    Analyzes betting history for suspicious patterns including:
    - Unusual bet amounts
    - Rapid betting
    - Night betting patterns
    - Abnormal win rates
    """
    from fastapi import Request
    request: Request = globals().get('request')
    
    if not bets:
        raise HTTPException(status_code=400, detail="No bets provided")
    
    # Get fraud detector from app state
    fraud_detector = request.app.state.fraud_detector if request else None
    if not fraud_detector:
        raise HTTPException(status_code=503, detail="Fraud detector not available")
    
    # Convert to DataFrame
    import pandas as pd
    bets_df = pd.DataFrame([b.model_dump() for b in bets])
    bets_df['placed_at'] = pd.to_datetime(bets_df['placed_at'])
    
    # Detect anomaly
    user_id = bets[0].user_id
    prediction = fraud_detector.detect_bet_anomaly(user_id, bets_df)
    
    return FraudPredictionResponse(
        user_id=prediction.user_id,
        fraud_type=prediction.fraud_type,
        risk_score=prediction.risk_score,
        is_fraud=prediction.is_fraud,
        confidence=prediction.confidence,
        timestamp=prediction.timestamp,
        explanation=prediction.explanation
    )


@router.post("/detect/bonus-abuse", response_model=FraudPredictionResponse)
async def detect_bonus_abuse(user_data: UserData):
    """
    Detect bonus abuse patterns
    
    Analyzes user behavior for bonus abuse including:
    - Multiple bonus claims
    - Low wagering completion
    - Immediate withdrawal after bonus
    - New account with high bonus activity
    """
    from fastapi import Request
    
    request: Request = globals().get('request')
    fraud_detector = request.app.state.fraud_detector if request else None
    if not fraud_detector:
        raise HTTPException(status_code=503, detail="Fraud detector not available")
    
    prediction = fraud_detector.detect_bonus_abuse(
        user_data.user_id,
        user_data.model_dump()
    )
    
    return FraudPredictionResponse(
        user_id=prediction.user_id,
        fraud_type=prediction.fraud_type,
        risk_score=prediction.risk_score,
        is_fraud=prediction.is_fraud,
        confidence=prediction.confidence,
        timestamp=prediction.timestamp,
        explanation=prediction.explanation
    )


@router.post("/detect/payment-fraud", response_model=FraudPredictionResponse)
async def detect_payment_fraud(transactions: List[TransactionData]):
    """
    Detect payment fraud patterns
    
    Analyzes transaction history for fraud including:
    - Rapid transactions
    - Multiple payment methods
    - High failed transaction ratio
    - Unusual transaction amounts
    """
    from fastapi import Request
    
    request: Request = globals().get('request')
    fraud_detector = request.app.state.fraud_detector if request else None
    if not fraud_detector:
        raise HTTPException(status_code=503, detail="Fraud detector not available")
    
    # Convert to DataFrame
    import pandas as pd
    tx_df = pd.DataFrame([t.model_dump() for t in transactions])
    tx_df['created_at'] = pd.to_datetime(tx_df['created_at'])
    
    user_id = transactions[0].user_id
    prediction = fraud_detector.detect_payment_fraud(user_id, tx_df)
    
    return FraudPredictionResponse(
        user_id=prediction.user_id,
        fraud_type=prediction.fraud_type,
        risk_score=prediction.risk_score,
        is_fraud=prediction.is_fraud,
        confidence=prediction.confidence,
        timestamp=prediction.timestamp,
        explanation=prediction.explanation
    )


@router.post("/detect/account-takeover", response_model=FraudPredictionResponse)
async def detect_account_takeover(login_data: LoginData):
    """
    Detect account takeover attempts
    
    Analyzes login patterns for takeover attempts including:
    - New location/device
    - Unusual login time
    - Multiple failed attempts
    - Logins from multiple locations
    """
    from fastapi import Request
    
    request: Request = globals().get('request')
    fraud_detector = request.app.state.fraud_detector if request else None
    if not fraud_detector:
        raise HTTPException(status_code=503, detail="Fraud detector not available")
    
    prediction = fraud_detector.detect_account_takeover(
        login_data.user_id,
        login_data.model_dump()
    )
    
    return FraudPredictionResponse(
        user_id=prediction.user_id,
        fraud_type=prediction.fraud_type,
        risk_score=prediction.risk_score,
        is_fraud=prediction.is_fraud,
        confidence=prediction.confidence,
        timestamp=prediction.timestamp,
        explanation=prediction.explanation
    )


# ============ Batch Detection ============

@router.post("/detect/batch", response_model=BatchFraudResponse)
async def detect_batch(
    bets: Optional[List[BetData]] = None,
    transactions: Optional[List[TransactionData]] = None,
    background_tasks: BackgroundTasks = None
):
    """
    Batch fraud detection for multiple data types
    
    Processes bets and transactions in batch for efficiency
    """
    from fastapi import Request
    import time
    
    request: Request = globals().get('request')
    start_time = time.time()
    
    fraud_detector = request.app.state.fraud_detector if request else None
    if not fraud_detector:
        raise HTTPException(status_code=503, detail="Fraud detector not available")
    
    predictions = []
    fraud_count = 0
    
    # Process bets
    if bets:
        import pandas as pd
        bets_df = pd.DataFrame([b.model_dump() for b in bets])
        bets_df['placed_at'] = pd.to_datetime(bets_df['placed_at'])
        
        user_ids = bets_df['user_id'].unique()
        for user_id in user_ids:
            user_bets = bets_df[bets_df['user_id'] == user_id]
            pred = fraud_detector.detect_bet_anomaly(user_id, user_bets)
            if pred.is_fraud:
                fraud_count += 1
            predictions.append(FraudPredictionResponse(
                user_id=pred.user_id,
                fraud_type=pred.fraud_type,
                risk_score=pred.risk_score,
                is_fraud=pred.is_fraud,
                confidence=pred.confidence,
                timestamp=pred.timestamp,
                explanation=pred.explanation
            ))
    
    # Process transactions
    if transactions:
        import pandas as pd
        tx_df = pd.DataFrame([t.model_dump() for t in transactions])
        tx_df['created_at'] = pd.to_datetime(tx_df['created_at'])
        
        user_ids = tx_df['user_id'].unique()
        for user_id in user_ids:
            user_txs = tx_df[tx_df['user_id'] == user_id]
            pred = fraud_detector.detect_payment_fraud(user_id, user_txs)
            if pred.is_fraud:
                fraud_count += 1
            predictions.append(FraudPredictionResponse(
                user_id=pred.user_id,
                fraud_type=pred.fraud_type,
                risk_score=pred.risk_score,
                is_fraud=pred.is_fraud,
                confidence=pred.confidence,
                timestamp=pred.timestamp,
                explanation=pred.explanation
            ))
    
    processing_time = (time.time() - start_time) * 1000
    
    return BatchFraudResponse(
        predictions=predictions,
        total_analyzed=len(predictions),
        fraud_detected=fraud_count,
        processing_time_ms=processing_time
    )


# ============ Model Management ============

@router.get("/models/status")
async def get_models_status():
    """Get status of all ML models"""
    from fastapi import Request
    
    request: Request = globals().get('request')
    fraud_detector = request.app.state.fraud_detector if request else None
    if not fraud_detector:
        raise HTTPException(status_code=503, detail="Fraud detector not available")
    
    return {
        "bet_anomaly": {
            "loaded": fraud_detector.bet_anomaly_detector.is_fitted
        },
        "bonus_abuse": {
            "loaded": fraud_detector.bonus_abuse_detector.is_fitted
        },
        "payment_fraud": {
            "loaded": fraud_detector.payment_fraud_detector.is_fitted
        },
        "account_takeover": {
            "loaded": fraud_detector.account_takeover_detector.is_fitted
        }
    }


@router.post("/models/reload")
async def reload_models():
    """Reload all ML models from disk"""
    from fastapi import Request
    
    request: Request = globals().get('request')
    fraud_detector = request.app.state.fraud_detector if request else None
    if not fraud_detector:
        raise HTTPException(status_code=503, detail="Fraud detector not available")
    
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
        "avg_processing_time_ms": 0.0
    }
