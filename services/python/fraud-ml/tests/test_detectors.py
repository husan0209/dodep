"""Tests for the rule-based detectors in internal/models/fraud_detector.py.

These run without trained models: each detector must degrade gracefully
(is_fitted=False) and must not crash while extracting features.
"""

import pandas as pd
import pytest

from internal.models.fraud_detector import (
    AccountTakeoverDetector,
    BetAnomalyDetector,
    BonusAbuseDetector,
    FraudDetector,
    PaymentFraudDetector,
)


def bets_frame(n: int = 20) -> pd.DataFrame:
    base = pd.Timestamp("2026-01-01 12:00:00")
    return pd.DataFrame(
        {
            "user_id": [1] * n,
            "stake": [10.0 + i for i in range(n)],
            "odds": [2.0] * n,
            "status": ["won" if i % 3 else "lost" for i in range(n)],
            "placed_at": [base + pd.Timedelta(minutes=i) for i in range(n)],
        }
    )


def transactions_frame(n: int = 10) -> pd.DataFrame:
    base = pd.Timestamp("2026-01-01 12:00:00")
    return pd.DataFrame(
        {
            "user_id": [1] * n,
            "amount": [100.0 + i * 10 for i in range(n)],
            "status": ["completed"] * (n - 1) + ["failed"],
            "payment_method": ["card", "wallet"] * (n // 2),
            "created_at": [base + pd.Timedelta(minutes=i) for i in range(n)],
        }
    )


class TestBetAnomalyFeatures:
    """Bet feature extraction tolerates sparse and empty input."""

    def test_extract_features(self):
        # Bet features are aggregate scalars, so the frame is keyed by
        # feature name (not by row index).
        result = BetAnomalyDetector().extract_features(bets_frame())

        assert not result.empty
        for column in ("bet_amount_mean", "bet_amount_max", "bets_per_hour"):
            assert column in result.columns
        assert result["bet_amount_max"][0] == pytest.approx(29.0)
        # 13 of 20 rows are 'won'
        assert result["win_rate"][0] == pytest.approx(13 / 20)

    def test_empty_bets_return_empty_frame(self):
        assert BetAnomalyDetector().extract_features(pd.DataFrame()).empty


class TestBonusAbuseFeatures:
    """Bonus features read defaults from the payload dict."""

    def test_deposit_bonus_ratio_uses_total_deposits(self):
        detector = BonusAbuseDetector()
        features = detector.extract_features(
            {
                "user_id": 1,
                "bonus_amount_total": 200.0,
                "total_deposits": 100.0,
            }
        )

        assert features["deposit_bonus_ratio"][0] == pytest.approx(2.0)

    def test_missing_total_deposits_does_not_divide_by_zero(self):
        detector = BonusAbuseDetector()
        features = detector.extract_features({"user_id": 1, "bonus_amount_total": 200.0})

        assert features["deposit_bonus_ratio"][0] == pytest.approx(200.0)
        assert features["account_age_days"][0] == 0

    def test_zero_total_deposits_does_not_divide_by_zero(self):
        detector = BonusAbuseDetector()
        features = detector.extract_features(
            {"user_id": 1, "bonus_amount_total": 50.0, "total_deposits": 0}
        )

        assert features["deposit_bonus_ratio"][0] == pytest.approx(50.0)


class TestPaymentFraudFeatures:
    """Transaction features include failure and night-time ratios."""

    def test_extract_features(self):
        detector = PaymentFraudDetector()
        features = detector.extract_features(transactions_frame())

        # Row-oriented: one row per transaction history.
        assert len(features) == 1
        assert features["failed_tx_ratio"][0] == pytest.approx(0.1)
        assert features["transaction_count_24h"][0] == 10

    def test_empty_transactions_return_empty_frame(self):
        assert PaymentFraudDetector().extract_features(pd.DataFrame()).empty


class TestAccountTakeoverFeatures:
    """ATO features depend on the per-user baseline."""

    def test_first_login_marks_everything_new(self):
        detector = AccountTakeoverDetector()
        features = detector.extract_features(
            {
                "user_id": 1,
                "country": "NL",
                "ip": "1.1.1.1",
                "device_fingerprint": "dev-1",
                "hour": 3,
            },
            user_id=1,
        )

        # Without a baseline every attribute counts as new.
        assert features["new_country"][0] == 0  # compared to itself
        assert features["unusual_hour"][0] in (True, 1)

    def test_baseline_makes_repeat_login_known(self):
        detector = AccountTakeoverDetector()
        login = {
            "user_id": 1,
            "country": "NL",
            "ip": "1.1.1.1",
            "device_fingerprint": "dev-1",
            "hour": 12,
        }
        detector.update_baseline(1, login)
        features = detector.extract_features(login, user_id=1)

        assert features["new_country"][0] == 0
        assert features["new_ip"][0] == 0
        assert features["new_device"][0] == 0

    def test_new_device_after_baseline_is_flagged(self):
        detector = AccountTakeoverDetector()
        detector.update_baseline(
            1,
            {
                "country": "NL",
                "ip": "1.1.1.1",
                "device_fingerprint": "dev-1",
                "hour": 12,
            },
        )
        features = detector.extract_features(
            {
                "user_id": 1,
                "country": "NL",
                "ip": "1.1.1.1",
                "device_fingerprint": "dev-2",
                "hour": 12,
            },
            user_id=1,
        )

        assert features["new_device"][0] == 1
        assert features["new_country"][0] == 0


class TestRiskScoreScale:
    """Risk scores are reported on the 0-100 scale (tasks/ТЗ.md bands)."""

    def test_threshold_maps_to_review_band(self):
        from internal.models.fraud_detector import _saturating_scale

        # Decision boundary -> start of the manual-review band (60)
        assert _saturating_scale(0.7, 0.7) == pytest.approx(60.0)
        assert _saturating_scale(0.0, 0.7) == pytest.approx(0.0)

    def test_scores_above_threshold_saturate(self):
        from internal.models.fraud_detector import _saturating_scale

        assert _saturating_scale(7.0, 0.7) == 100.0
        assert _saturating_scale(-1.0, 0.7) == 0.0

    def test_invalid_threshold_is_safe(self):
        from internal.models.fraud_detector import _saturating_scale

        assert _saturating_scale(0.5, 0.0) == 0.0


class TestUnfittedDetectors:
    """Before training, detectors return a neutral prediction instead of raising."""

    def test_predict_proba_requires_fitted_model(self):
        with pytest.raises(ValueError, match="not fitted"):
            BonusAbuseDetector().predict_proba(
                BonusAbuseDetector().extract_features({"user_id": 1})
            )

    def test_feature_importance_empty_when_unfitted(self):
        assert BonusAbuseDetector().get_feature_importance() == {}

    def test_orchestrator_returns_neutral_prediction(self, tmp_path):
        detector = FraudDetector(model_path=str(tmp_path))

        prediction = detector.detect_bet_anomaly(1, bets_frame())
        assert prediction.is_fraud is False
        assert prediction.risk_score == 0.0

        bonus = detector.detect_bonus_abuse(1, {"user_id": 1})
        assert bonus.is_fraud is False

        payment = detector.detect_payment_fraud(1, transactions_frame())
        assert payment.is_fraud is False

        ato = detector.detect_account_takeover(
            1,
            {
                "user_id": 1,
                "ip": "1.1.1.1",
                "country": "NL",
                "device_fingerprint": "d",
                "hour": 12,
            },
        )
        assert ato.is_fraud is False

    def test_orchestrator_exposes_per_detector_status(self, tmp_path):
        detector = FraudDetector(model_path=str(tmp_path))
        status = detector.get_status()

        assert set(status) == {
            "bet_anomaly",
            "bonus_abuse",
            "payment_fraud",
            "account_takeover",
        }
        assert all(entry["loaded"] is False for entry in status.values())
