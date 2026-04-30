"""
Fraud Detection Models
Implements multiple ML models for different fraud types
"""

import logging
import numpy as np
import pandas as pd
from pathlib import Path
from typing import Dict, List, Optional, Tuple
from datetime import datetime, timedelta
from dataclasses import dataclass, asdict

import joblib
from sklearn.ensemble import IsolationForest, RandomForestClassifier
from sklearn.preprocessing import StandardScaler
from xgboost import XGBClassifier

logger = logging.getLogger(__name__)


@dataclass
class FraudPrediction:
    """Fraud prediction result"""
    user_id: int
    fraud_type: str
    risk_score: float
    is_fraud: bool
    confidence: float
    features: Dict[str, float]
    timestamp: datetime
    explanation: Optional[str] = None


class BetAnomalyDetector:
    """
    Detects anomalous betting patterns using Isolation Forest
    """
    
    def __init__(self):
        self.model = IsolationForest(
            n_estimators=100,
            contamination=0.05,
            random_state=42,
            n_jobs=-1
        )
        self.scaler = StandardScaler()
        self.is_fitted = False
        
    def extract_features(self, bets: pd.DataFrame) -> pd.DataFrame:
        """Extract features from betting history"""
        if bets.empty:
            return pd.DataFrame()
            
        features = pd.DataFrame()
        
        # Bet amount statistics
        features['bet_amount_mean'] = bets['stake'].mean()
        features['bet_amount_std'] = bets['stake'].std()
        features['bet_amount_max'] = bets['stake'].max()
        features['bet_amount_min'] = bets['stake'].min()
        
        # Betting frequency
        features['bets_per_hour'] = len(bets) / max(
            (bets['placed_at'].max() - bets['placed_at'].min()).total_seconds() / 3600, 1
        )
        
        # Win/loss ratio
        features['win_rate'] = (bets['status'] == 'won').mean()
        
        # Odds statistics
        if 'odds' in bets.columns:
            features['avg_odds'] = bets['odds'].mean()
            features['max_odds'] = bets['odds'].max()
            
        # Time patterns
        features['night_betting_ratio'] = (
            bets['placed_at'].dt.hour.isin(range(0, 6))
        ).mean()
        
        # Rapid betting (multiple bets within short time)
        bet_diffs = bets['placed_at'].diff().dt.total_seconds()
        features['rapid_bet_ratio'] = (bet_diffs < 10).mean()
        
        return features.fillna(0)
    
    def fit(self, X: pd.DataFrame):
        """Train the anomaly detector"""
        X_scaled = self.scaler.fit_transform(X)
        self.model.fit(X_scaled)
        self.is_fitted = True
        logger.info(f"BetAnomalyDetector fitted on {len(X)} samples")
        
    def predict(self, features: pd.DataFrame) -> Tuple[np.ndarray, np.ndarray]:
        """
        Predict anomalies
        Returns: (predictions, scores) where -1 = anomaly, 1 = normal
        """
        if not self.is_fitted:
            raise ValueError("Model not fitted")
            
        X_scaled = self.scaler.transform(features)
        predictions = self.model.predict(X_scaled)
        scores = -self.model.score_samples(X_scaled)  # Higher = more anomalous
        
        return predictions, scores
    
    def save(self, path: Path):
        """Save model to disk"""
        joblib.dump({'model': self.model, 'scaler': self.scaler}, path)
        
    def load(self, path: Path):
        """Load model from disk"""
        data = joblib.load(path)
        self.model = data['model']
        self.scaler = data['scaler']
        self.is_fitted = True


class BonusAbuseDetector:
    """
    Detects bonus abuse patterns
    """
    
    def __init__(self):
        self.model = XGBClassifier(
            n_estimators=100,
            max_depth=6,
            learning_rate=0.1,
            random_state=42,
            n_jobs=-1
        )
        self.scaler = StandardScaler()
        self.is_fitted = False
        
    def extract_features(self, user_data: Dict) -> pd.DataFrame:
        """Extract features for bonus abuse detection"""
        features = pd.DataFrame([user_data])
        
        # Bonus-related features
        features['bonuses_claimed_24h'] = features.get('bonuses_claimed_24h', 0)
        features['bonus_amount_total'] = features.get('bonus_amount_total', 0)
        features['wagering_completed'] = features.get('wagering_completed', 0)
        features['withdrawal_after_bonus'] = features.get('withdrawal_after_bonus', False)
        
        # Account age
        features['account_age_days'] = features.get('account_age_days', 0)
        
        # Deposit pattern
        features['deposits_count'] = features.get('deposits_count', 0)
        features['deposit_bonus_ratio'] = (
            features['bonus_amount_total'] / 
            (features.get('total_deposits', 1) or 1)
        )
        
        return features
    
    def fit(self, X: pd.DataFrame, y: pd.Series):
        """Train the bonus abuse detector"""
        X_scaled = self.scaler.fit_transform(X)
        self.model.fit(X_scaled, y)
        self.is_fitted = True
        logger.info(f"BonusAbuseDetector fitted on {len(X)} samples")
        
    def predict_proba(self, features: pd.DataFrame) -> np.ndarray:
        """Predict probability of bonus abuse"""
        if not self.is_fitted:
            raise ValueError("Model not fitted")
            
        X_scaled = self.scaler.transform(features)
        return self.model.predict_proba(X_scaled)[:, 1]
    
    def get_feature_importance(self) -> Dict[str, float]:
        """Get feature importance scores"""
        if not self.is_fitted:
            return {}
        return dict(zip(
            ['bonuses_claimed_24h', 'bonus_amount_total', 'wagering_completed',
             'withdrawal_after_bonus', 'account_age_days', 'deposits_count',
             'deposit_bonus_ratio'],
            self.model.feature_importances_
        ))
    
    def save(self, path: Path):
        """Save model to disk"""
        joblib.dump({'model': self.model, 'scaler': self.scaler}, path)
        
    def load(self, path: Path):
        """Load model from disk"""
        data = joblib.load(path)
        self.model = data['model']
        self.scaler = data['scaler']
        self.is_fitted = True


class PaymentFraudDetector:
    """
    Detects payment fraud patterns
    """
    
    def __init__(self):
        self.model = RandomForestClassifier(
            n_estimators=100,
            max_depth=10,
            random_state=42,
            n_jobs=-1,
            class_weight='balanced'
        )
        self.scaler = StandardScaler()
        self.is_fitted = False
        
    def extract_features(self, transactions: pd.DataFrame) -> pd.DataFrame:
        """Extract features from transaction history"""
        if transactions.empty:
            return pd.DataFrame()
            
        features = pd.DataFrame()
        
        # Transaction statistics
        features['transaction_count_24h'] = len(transactions)
        features['total_amount_24h'] = transactions['amount'].sum()
        features['avg_transaction_amount'] = transactions['amount'].mean()
        
        # Payment method diversity
        features['unique_payment_methods'] = transactions['payment_method'].nunique()
        
        # Failed transactions
        features['failed_tx_ratio'] = (transactions['status'] == 'failed').mean()
        
        # Time patterns
        features['night_tx_ratio'] = (
            transactions['created_at'].dt.hour.isin(range(0, 6))
        ).mean()
        
        # Rapid transactions
        tx_diffs = transactions['created_at'].diff().dt.total_seconds()
        features['rapid_tx_ratio'] = (tx_diffs < 60).mean()
        
        # Amount patterns
        features['amount_std'] = transactions['amount'].std()
        features['round_amount_ratio'] = (
            (transactions['amount'] % 100 == 0).mean()
        )
        
        return features.fillna(0)
    
    def fit(self, X: pd.DataFrame, y: pd.Series):
        """Train the payment fraud detector"""
        X_scaled = self.scaler.fit_transform(X)
        self.model.fit(X_scaled, y)
        self.is_fitted = True
        logger.info(f"PaymentFraudDetector fitted on {len(X)} samples")
        
    def predict_proba(self, features: pd.DataFrame) -> np.ndarray:
        """Predict probability of payment fraud"""
        if not self.is_fitted:
            raise ValueError("Model not fitted")
            
        X_scaled = self.scaler.transform(features)
        return self.model.predict_proba(X_scaled)[:, 1]
    
    def save(self, path: Path):
        """Save model to disk"""
        joblib.dump({'model': self.model, 'scaler': self.scaler}, path)
        
    def load(self, path: Path):
        """Load model from disk"""
        data = joblib.load(path)
        self.model = data['model']
        self.scaler = data['scaler']
        self.is_fitted = True


class AccountTakeoverDetector:
    """
    Detects account takeover attempts
    """
    
    def __init__(self):
        self.model = XGBClassifier(
            n_estimators=100,
            max_depth=6,
            learning_rate=0.1,
            random_state=42,
            n_jobs=-1
        )
        self.scaler = StandardScaler()
        self.is_fitted = False
        
        # User behavior baseline
        self.user_baselines: Dict[int, Dict] = {}
        
    def extract_features(self, login_data: Dict, user_id: int) -> pd.DataFrame:
        """Extract features for account takeover detection"""
        features = pd.DataFrame([login_data])
        
        # Get user baseline
        baseline = self.user_baselines.get(user_id, {})
        
        # Location features
        features['new_country'] = login_data.get('country') != baseline.get('country', login_data.get('country'))
        features['new_ip'] = login_data.get('ip') != baseline.get('ip', login_data.get('ip'))
        features['new_device'] = login_data.get('device_fingerprint') != baseline.get('device_fingerprint', login_data.get('device_fingerprint'))
        
        # Time features
        features['unusual_hour'] = login_data.get('hour', 12) in range(0, 6)
        features['time_since_last_login'] = login_data.get('hours_since_last_login', 24)
        
        # Failed attempts
        features['failed_attempts_24h'] = login_data.get('failed_attempts_24h', 0)
        
        # Multiple locations
        features['locations_24h'] = login_data.get('locations_24h', 1)
        
        return features
    
    def update_baseline(self, user_id: int, login_data: Dict):
        """Update user behavior baseline"""
        self.user_baselines[user_id] = {
            'country': login_data.get('country'),
            'ip': login_data.get('ip'),
            'device_fingerprint': login_data.get('device_fingerprint'),
            'typical_hours': login_data.get('hour', 12),
        }
    
    def fit(self, X: pd.DataFrame, y: pd.Series):
        """Train the account takeover detector"""
        X_scaled = self.scaler.fit_transform(X)
        self.model.fit(X_scaled, y)
        self.is_fitted = True
        logger.info(f"AccountTakeoverDetector fitted on {len(X)} samples")
        
    def predict_proba(self, features: pd.DataFrame) -> np.ndarray:
        """Predict probability of account takeover"""
        if not self.is_fitted:
            raise ValueError("Model not fitted")
            
        X_scaled = self.scaler.transform(features)
        return self.model.predict_proba(X_scaled)[:, 1]
    
    def save(self, path: Path):
        """Save model to disk"""
        joblib.dump({
            'model': self.model,
            'scaler': self.scaler,
            'baselines': self.user_baselines
        }, path)
        
    def load(self, path: Path):
        """Load model from disk"""
        data = joblib.load(path)
        self.model = data['model']
        self.scaler = data['scaler']
        self.user_baselines = data.get('baselines', {})
        self.is_fitted = True


class FraudDetector:
    """
    Main fraud detection orchestrator
    Combines multiple specialized detectors
    """
    
    def __init__(self, model_path: str = "/app/models"):
        self.model_path = Path(model_path)
        self.model_path.mkdir(parents=True, exist_ok=True)
        
        # Initialize detectors
        self.bet_anomaly_detector = BetAnomalyDetector()
        self.bonus_abuse_detector = BonusAbuseDetector()
        self.payment_fraud_detector = PaymentFraudDetector()
        self.account_takeover_detector = AccountTakeoverDetector()
        
        logger.info("FraudDetector initialized")
        
    def detect_bet_anomaly(self, user_id: int, bets: pd.DataFrame) -> FraudPrediction:
        """Detect betting anomalies for a user"""
        features = self.bet_anomaly_detector.extract_features(bets)
        
        if features.empty or not self.bet_anomaly_detector.is_fitted:
            return FraudPrediction(
                user_id=user_id,
                fraud_type="bet_anomaly",
                risk_score=0.0,
                is_fraud=False,
                confidence=0.0,
                features={},
                timestamp=datetime.now()
            )
        
        predictions, scores = self.bet_anomaly_detector.predict(features)
        is_anomaly = predictions[0] == -1
        risk_score = min(1.0, scores[0] / 10)  # Normalize to 0-1
        
        return FraudPrediction(
            user_id=user_id,
            fraud_type="bet_anomaly",
            risk_score=risk_score,
            is_fraud=is_anomaly,
            confidence=1.0 - risk_score if is_anomaly else risk_score,
            features=features.iloc[0].to_dict(),
            timestamp=datetime.now(),
            explanation="Anomalous betting pattern detected" if is_anomaly else None
        )
    
    def detect_bonus_abuse(self, user_id: int, user_data: Dict) -> FraudPrediction:
        """Detect bonus abuse for a user"""
        features = self.bonus_abuse_detector.extract_features(user_data)
        
        if not self.bonus_abuse_detector.is_fitted:
            return FraudPrediction(
                user_id=user_id,
                fraud_type="bonus_abuse",
                risk_score=0.0,
                is_fraud=False,
                confidence=0.0,
                features={},
                timestamp=datetime.now()
            )
        
        proba = self.bonus_abuse_detector.predict_proba(features)[0]
        is_abuse = proba > 0.5
        
        return FraudPrediction(
            user_id=user_id,
            fraud_type="bonus_abuse",
            risk_score=proba,
            is_fraud=is_abuse,
            confidence=proba if is_abuse else 1 - proba,
            features=features.iloc[0].to_dict(),
            timestamp=datetime.now(),
            explanation="Potential bonus abuse detected" if is_abuse else None
        )
    
    def detect_payment_fraud(self, user_id: int, transactions: pd.DataFrame) -> FraudPrediction:
        """Detect payment fraud for a user"""
        features = self.payment_fraud_detector.extract_features(transactions)
        
        if features.empty or not self.payment_fraud_detector.is_fitted:
            return FraudPrediction(
                user_id=user_id,
                fraud_type="payment_fraud",
                risk_score=0.0,
                is_fraud=False,
                confidence=0.0,
                features={},
                timestamp=datetime.now()
            )
        
        proba = self.payment_fraud_detector.predict_proba(features)[0]
        is_fraud = proba > 0.5
        
        return FraudPrediction(
            user_id=user_id,
            fraud_type="payment_fraud",
            risk_score=proba,
            is_fraud=is_fraud,
            confidence=proba if is_fraud else 1 - proba,
            features=features.iloc[0].to_dict(),
            timestamp=datetime.now(),
            explanation="Potential payment fraud detected" if is_fraud else None
        )
    
    def detect_account_takeover(self, user_id: int, login_data: Dict) -> FraudPrediction:
        """Detect account takeover attempt"""
        features = self.account_takeover_detector.extract_features(login_data, user_id)
        
        if not self.account_takeover_detector.is_fitted:
            return FraudPrediction(
                user_id=user_id,
                fraud_type="account_takeover",
                risk_score=0.0,
                is_fraud=False,
                confidence=0.0,
                features={},
                timestamp=datetime.now()
            )
        
        proba = self.account_takeover_detector.predict_proba(features)[0]
        is_takeover = proba > 0.5
        
        # Update baseline if not fraud
        if not is_takeover:
            self.account_takeover_detector.update_baseline(user_id, login_data)
        
        return FraudPrediction(
            user_id=user_id,
            fraud_type="account_takeover",
            risk_score=proba,
            is_fraud=is_takeover,
            confidence=proba if is_takeover else 1 - proba,
            features=features.iloc[0].to_dict(),
            timestamp=datetime.now(),
            explanation="Potential account takeover detected" if is_takeover else None
        )
    
    def save_models(self):
        """Save all models to disk"""
        self.bet_anomaly_detector.save(self.model_path / "bet_anomaly.joblib")
        self.bonus_abuse_detector.save(self.model_path / "bonus_abuse.joblib")
        self.payment_fraud_detector.save(self.model_path / "payment_fraud.joblib")
        self.account_takeover_detector.save(self.model_path / "account_takeover.joblib")
        logger.info("All models saved")
        
    def load_models(self):
        """Load all models from disk"""
        bet_path = self.model_path / "bet_anomaly.joblib"
        if bet_path.exists():
            self.bet_anomaly_detector.load(bet_path)
            
        bonus_path = self.model_path / "bonus_abuse.joblib"
        if bonus_path.exists():
            self.bonus_abuse_detector.load(bonus_path)
            
        payment_path = self.model_path / "payment_fraud.joblib"
        if payment_path.exists():
            self.payment_fraud_detector.load(payment_path)
            
        ato_path = self.model_path / "account_takeover.joblib"
        if ato_path.exists():
            self.account_takeover_detector.load(ato_path)
            
        logger.info("All models loaded")
