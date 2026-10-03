"""
Fraud Detection Models
Implements multiple ML models for different fraud types
"""

import logging
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path

import joblib
import numpy as np
import pandas as pd
from sklearn.ensemble import IsolationForest, RandomForestClassifier
from sklearn.preprocessing import StandardScaler
from xgboost import XGBClassifier

from ..config import settings as detector_settings

logger = logging.getLogger(__name__)


def _saturating_scale(raw_score: float, threshold: float) -> float:
    """
    Map an unbounded anomaly score onto the 0-100 risk scale.

    `threshold` (the detector's decision boundary) maps to 60 — the start
    of the "hold for manual review" band in tasks/ТЗ.md. Scores below it
    interpolate into the 0-60 monitoring range, scores above saturate at
    100 (block band).
    """
    if threshold <= 0:
        return 0.0
    score_100 = raw_score / threshold * 60
    return float(min(100.0, max(0.0, score_100)))


@dataclass
class FraudPrediction:
    """
    Fraud prediction result

    `risk_score` is on a 0-100 scale; `confidence` stays in 0-1.
    """

    user_id: int
    fraud_type: str
    risk_score: float
    is_fraud: bool
    confidence: float
    features: dict[str, float]
    timestamp: datetime
    explanation: str | None = None


class BetAnomalyDetector:
    """
    Detects anomalous betting patterns using Isolation Forest
    """

    def __init__(self):
        self.model = IsolationForest(
            n_estimators=100, contamination=0.05, random_state=42, n_jobs=-1
        )
        self.scaler = StandardScaler()
        self.is_fitted = False

    def extract_features(self, bets: pd.DataFrame) -> pd.DataFrame:
        """
        Extract features from betting history.

        Returns a single-row frame: assigning scalars onto an empty
        DataFrame silently produces zero rows, which then breaks the
        downstream `.iloc[0]` access and the model input shape.
        """
        if bets.empty:
            return pd.DataFrame()

        span_hours = max(
            (bets["placed_at"].max() - bets["placed_at"].min()).total_seconds() / 3600,
            1,
        )
        bet_diffs = bets["placed_at"].diff().dt.total_seconds()

        row: dict[str, float] = {
            # Bet amount statistics
            "bet_amount_mean": bets["stake"].mean(),
            "bet_amount_std": bets["stake"].std(),
            "bet_amount_max": bets["stake"].max(),
            "bet_amount_min": bets["stake"].min(),
            # Betting frequency
            "bets_per_hour": len(bets) / span_hours,
            # Win/loss ratio
            "win_rate": (bets["status"] == "won").mean(),
            # Time patterns
            "night_betting_ratio": bets["placed_at"].dt.hour.isin(range(0, 6)).mean(),
            # Rapid betting (multiple bets within a short window)
            "rapid_bet_ratio": (bet_diffs < 10).mean(),
        }

        # Odds statistics are optional (void/cancelled bets may omit them)
        if "odds" in bets.columns:
            row["avg_odds"] = bets["odds"].mean()
            row["max_odds"] = bets["odds"].max()

        return pd.DataFrame([row]).fillna(0)

    def fit(self, x_matrix: pd.DataFrame):
        """Train the anomaly detector"""
        scaled = self.scaler.fit_transform(x_matrix)
        self.model.fit(scaled)
        self.is_fitted = True
        logger.info(f"BetAnomalyDetector fitted on {len(x_matrix)} samples")

    def predict(self, features: pd.DataFrame) -> tuple[np.ndarray, np.ndarray]:
        """
        Predict anomalies
        Returns: (predictions, scores) where -1 = anomaly, 1 = normal
        """
        if not self.is_fitted:
            raise ValueError("Model not fitted")

        scaled = self.scaler.transform(features)
        predictions = self.model.predict(scaled)
        scores = -self.model.score_samples(scaled)  # Higher = more anomalous

        return predictions, scores

    def save(self, path: Path):
        """Save model to disk"""
        joblib.dump({"model": self.model, "scaler": self.scaler}, path)

    def load(self, path: Path):
        """Load model from disk"""
        data = joblib.load(path)
        self.model = data["model"]
        self.scaler = data["scaler"]
        self.is_fitted = True


class BonusAbuseDetector:
    """
    Detects bonus abuse patterns
    """

    def __init__(self):
        self.model = XGBClassifier(
            n_estimators=100, max_depth=6, learning_rate=0.1, random_state=42, n_jobs=-1
        )
        self.scaler = StandardScaler()
        self.is_fitted = False

    def extract_features(self, user_data: dict) -> pd.DataFrame:
        """Extract features for bonus abuse detection"""
        features = pd.DataFrame([user_data])

        # Read defaults from the source dict: DataFrame.get() returns a
        # Series, and arithmetic on it fails ("truth value is ambiguous").
        features["bonuses_claimed_24h"] = user_data.get("bonuses_claimed_24h", 0)
        features["bonus_amount_total"] = user_data.get("bonus_amount_total", 0)
        features["wagering_completed"] = user_data.get("wagering_completed", 0)
        features["withdrawal_after_bonus"] = user_data.get("withdrawal_after_bonus", False)

        # Account age
        features["account_age_days"] = user_data.get("account_age_days", 0)

        # Deposit pattern
        features["deposits_count"] = user_data.get("deposits_count", 0)
        # Zero deposits must not divide by zero.
        total_deposits = user_data.get("total_deposits") or 1
        features["deposit_bonus_ratio"] = user_data.get("bonus_amount_total", 0) / total_deposits

        return features

    def fit(self, x_matrix: pd.DataFrame, y: pd.Series):
        """Train the bonus abuse detector"""
        scaled = self.scaler.fit_transform(x_matrix)
        self.model.fit(scaled, y)
        self.is_fitted = True
        logger.info(f"BonusAbuseDetector fitted on {len(x_matrix)} samples")

    def predict_proba(self, features: pd.DataFrame) -> np.ndarray:
        """Predict probability of bonus abuse"""
        if not self.is_fitted:
            raise ValueError("Model not fitted")

        scaled = self.scaler.transform(features)
        probabilities: np.ndarray = self.model.predict_proba(scaled)
        return probabilities[:, 1]

    def get_feature_importance(self) -> dict[str, float]:
        """Get feature importance scores"""
        if not self.is_fitted:
            return {}
        feature_names = [
            "bonuses_claimed_24h",
            "bonus_amount_total",
            "wagering_completed",
            "withdrawal_after_bonus",
            "account_age_days",
            "deposits_count",
            "deposit_bonus_ratio",
        ]
        return dict(zip(feature_names, self.model.feature_importances_, strict=True))

    def save(self, path: Path):
        """Save model to disk"""
        joblib.dump({"model": self.model, "scaler": self.scaler}, path)

    def load(self, path: Path):
        """Load model from disk"""
        data = joblib.load(path)
        self.model = data["model"]
        self.scaler = data["scaler"]
        self.is_fitted = True


class PaymentFraudDetector:
    """
    Detects payment fraud patterns
    """

    def __init__(self):
        self.model = RandomForestClassifier(
            n_estimators=100, max_depth=10, random_state=42, n_jobs=-1, class_weight="balanced"
        )
        self.scaler = StandardScaler()
        self.is_fitted = False

    def extract_features(self, transactions: pd.DataFrame) -> pd.DataFrame:
        """
        Extract features from transaction history.

        Returns a single-row frame (same scalar-assignment caveat as the
        bet detector).
        """
        if transactions.empty:
            return pd.DataFrame()

        tx_diffs = transactions["created_at"].diff().dt.total_seconds()

        row: dict[str, float] = {
            # Transaction statistics
            "transaction_count_24h": len(transactions),
            "total_amount_24h": transactions["amount"].sum(),
            "avg_transaction_amount": transactions["amount"].mean(),
            # Payment method diversity
            "unique_payment_methods": transactions["payment_method"].nunique(),
            # Failed transactions
            "failed_tx_ratio": (transactions["status"] == "failed").mean(),
            # Time patterns
            "night_tx_ratio": transactions["created_at"].dt.hour.isin(range(0, 6)).mean(),
            # Rapid transactions
            "rapid_tx_ratio": (tx_diffs < 60).mean(),
            # Amount patterns
            "amount_std": transactions["amount"].std(),
            "round_amount_ratio": (transactions["amount"] % 100 == 0).mean(),
        }

        return pd.DataFrame([row]).fillna(0)

    def fit(self, x_matrix: pd.DataFrame, y: pd.Series):
        """Train the payment fraud detector"""
        scaled = self.scaler.fit_transform(x_matrix)
        self.model.fit(scaled, y)
        self.is_fitted = True
        logger.info(f"PaymentFraudDetector fitted on {len(x_matrix)} samples")

    def predict_proba(self, features: pd.DataFrame) -> np.ndarray:
        """Predict probability of payment fraud"""
        if not self.is_fitted:
            raise ValueError("Model not fitted")

        scaled = self.scaler.transform(features)
        probabilities: np.ndarray = self.model.predict_proba(scaled)
        return probabilities[:, 1]

    def save(self, path: Path):
        """Save model to disk"""
        joblib.dump({"model": self.model, "scaler": self.scaler}, path)

    def load(self, path: Path):
        """Load model from disk"""
        data = joblib.load(path)
        self.model = data["model"]
        self.scaler = data["scaler"]
        self.is_fitted = True


class AccountTakeoverDetector:
    """
    Detects account takeover attempts
    """

    def __init__(self):
        self.model = XGBClassifier(
            n_estimators=100, max_depth=6, learning_rate=0.1, random_state=42, n_jobs=-1
        )
        self.scaler = StandardScaler()
        self.is_fitted = False

        # User behavior baseline
        self.user_baselines: dict[int, dict] = {}

    def extract_features(self, login_data: dict, user_id: int) -> pd.DataFrame:
        """Extract features for account takeover detection"""
        features = pd.DataFrame([login_data])

        # Get user baseline
        baseline = self.user_baselines.get(user_id, {})

        # Location features: first-seen values count as "new"; without a
        # baseline everything is new, which is the conservative default.
        features["new_country"] = login_data.get("country") != baseline.get(
            "country", login_data.get("country")
        )
        features["new_ip"] = login_data.get("ip") != baseline.get("ip", login_data.get("ip"))
        features["new_device"] = login_data.get("device_fingerprint") != baseline.get(
            "device_fingerprint", login_data.get("device_fingerprint")
        )

        # Time features
        features["unusual_hour"] = login_data.get("hour", 12) in range(0, 6)
        features["time_since_last_login"] = login_data.get("hours_since_last_login", 24)

        # Failed attempts
        features["failed_attempts_24h"] = login_data.get("failed_attempts_24h", 0)

        # Multiple locations
        features["locations_24h"] = login_data.get("locations_24h", 1)

        return features

    def update_baseline(self, user_id: int, login_data: dict):
        """Update user behavior baseline"""
        self.user_baselines[user_id] = {
            "country": login_data.get("country"),
            "ip": login_data.get("ip"),
            "device_fingerprint": login_data.get("device_fingerprint"),
            "typical_hours": login_data.get("hour", 12),
        }

    def fit(self, x_matrix: pd.DataFrame, y: pd.Series):
        """Train the account takeover detector"""
        scaled = self.scaler.fit_transform(x_matrix)
        self.model.fit(scaled, y)
        self.is_fitted = True
        logger.info(f"AccountTakeoverDetector fitted on {len(x_matrix)} samples")

    def predict_proba(self, features: pd.DataFrame) -> np.ndarray:
        """Predict probability of account takeover"""
        if not self.is_fitted:
            raise ValueError("Model not fitted")

        scaled = self.scaler.transform(features)
        probabilities: np.ndarray = self.model.predict_proba(scaled)
        return probabilities[:, 1]

    def save(self, path: Path):
        """Save model to disk"""
        joblib.dump(
            {"model": self.model, "scaler": self.scaler, "baselines": self.user_baselines}, path
        )

    def load(self, path: Path):
        """Load model from disk"""
        data = joblib.load(path)
        self.model = data["model"]
        self.scaler = data["scaler"]
        self.user_baselines = data.get("baselines", {})
        self.is_fitted = True


class FraudDetector:
    """
    Main fraud detection orchestrator
    Combines multiple specialized detectors

    Risk scores are reported on a 0-100 scale (tasks/ТЗ.md, Этап 11:
    0-30 allow, 30-60 enhanced monitoring, 60-80 manual review,
    80-100 block + alert). Detector probabilities (0-1) are scaled on the
    way out, and per-detector decision thresholds come from settings
    instead of a hard-coded 0.5.
    """

    def __init__(self, model_path: str = "/app/models", settings=None):
        self.model_path = Path(model_path)
        self.model_path.mkdir(parents=True, exist_ok=True)
        self.settings = settings or detector_settings

        # Initialize detectors
        self.bet_anomaly_detector = BetAnomalyDetector()
        self.bonus_abuse_detector = BonusAbuseDetector()
        self.payment_fraud_detector = PaymentFraudDetector()
        self.account_takeover_detector = AccountTakeoverDetector()

        logger.info("FraudDetector initialized")

    def detect_bet_anomaly(self, user_id: int, bets: pd.DataFrame) -> FraudPrediction:
        """Detect betting anomalies for a user"""
        features = self.bet_anomaly_detector.extract_features(bets)

        # extract_features returns a single-row frame; `features.empty`
        # alone is not enough because a 1x0 frame is also "empty".
        if not self.bet_anomaly_detector.is_fitted or features.empty or not len(features.columns):
            return FraudPrediction(
                user_id=user_id,
                fraud_type="bet_anomaly",
                risk_score=0.0,
                is_fraud=False,
                confidence=0.0,
                features={},
                timestamp=datetime.now(),
            )

        predictions, scores = self.bet_anomaly_detector.predict(features)
        is_anomaly = predictions[0] == -1
        # score_samples is unbounded; map the decision threshold to a 0-100
        # scale instead of dividing by a magic constant.
        threshold = self.settings.bet_anomaly_threshold
        raw = float(scores[0])
        risk_score = _saturating_scale(raw, threshold)

        return FraudPrediction(
            user_id=user_id,
            fraud_type="bet_anomaly",
            risk_score=risk_score,
            is_fraud=is_anomaly,
            confidence=risk_score / 100 if is_anomaly else 1 - risk_score / 100,
            features=features.iloc[0].to_dict(),
            timestamp=datetime.now(),
            explanation="Anomalous betting pattern detected" if is_anomaly else None,
        )

    def detect_bonus_abuse(self, user_id: int, user_data: dict) -> FraudPrediction:
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
                timestamp=datetime.now(),
            )

        proba = float(self.bonus_abuse_detector.predict_proba(features)[0])
        is_abuse = proba >= self.settings.bonus_abuse_threshold

        return FraudPrediction(
            user_id=user_id,
            fraud_type="bonus_abuse",
            risk_score=proba * 100,
            is_fraud=is_abuse,
            confidence=proba if is_abuse else 1 - proba,
            features=features.iloc[0].to_dict(),
            timestamp=datetime.now(),
            explanation="Potential bonus abuse detected" if is_abuse else None,
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
                timestamp=datetime.now(),
            )

        proba = float(self.payment_fraud_detector.predict_proba(features)[0])
        is_fraud = proba >= self.settings.payment_fraud_threshold

        return FraudPrediction(
            user_id=user_id,
            fraud_type="payment_fraud",
            risk_score=proba * 100,
            is_fraud=is_fraud,
            confidence=proba if is_fraud else 1 - proba,
            features=features.iloc[0].to_dict(),
            timestamp=datetime.now(),
            explanation="Potential payment fraud detected" if is_fraud else None,
        )

    def detect_account_takeover(self, user_id: int, login_data: dict) -> FraudPrediction:
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
                timestamp=datetime.now(),
            )

        proba = float(self.account_takeover_detector.predict_proba(features)[0])
        is_takeover = proba >= self.settings.account_takeover_threshold

        # Update baseline if not fraud
        if not is_takeover:
            self.account_takeover_detector.update_baseline(user_id, login_data)

        return FraudPrediction(
            user_id=user_id,
            fraud_type="account_takeover",
            risk_score=proba * 100,
            is_fraud=is_takeover,
            confidence=proba if is_takeover else 1 - proba,
            features=features.iloc[0].to_dict(),
            timestamp=datetime.now(),
            explanation="Potential account takeover detected" if is_takeover else None,
        )

    def save_models(self):
        """Save all models to disk"""
        self.bet_anomaly_detector.save(self.model_path / "bet_anomaly.joblib")
        self.bonus_abuse_detector.save(self.model_path / "bonus_abuse.joblib")
        self.payment_fraud_detector.save(self.model_path / "payment_fraud.joblib")
        self.account_takeover_detector.save(self.model_path / "account_takeover.joblib")
        logger.info("All models saved")

    def get_status(self) -> dict[str, dict[str, bool]]:
        """
        Report per-detector load state (mirrors GET /api/v1/models/status).
        """
        return {
            "bet_anomaly": {"loaded": self.bet_anomaly_detector.is_fitted},
            "bonus_abuse": {"loaded": self.bonus_abuse_detector.is_fitted},
            "payment_fraud": {"loaded": self.payment_fraud_detector.is_fitted},
            "account_takeover": {"loaded": self.account_takeover_detector.is_fitted},
        }

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
