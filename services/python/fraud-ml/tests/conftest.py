"""Shared fixtures for fraud-ml tests."""

from datetime import datetime, timedelta

import numpy as np
import polars as pl
import pytest

from src.features.transformation import MODEL_FEATURES
from src.models.fraud_model import FraudModel


def make_fraud_dataset(n_samples: int = 1500, seed: int = 42) -> pl.DataFrame:
    """
    Synthetic labeled dataset with a learnable fraud signal.

    Signal (mimics iGaming multi-accounting/velocity patterns):
    burst betting (>15 bets in 24h) combined with many distinct devices
    (>6 in 30d), plus 5% label noise. Rows carry an ascending event_time
    so the temporal split path is exercised.
    """
    rng = np.random.default_rng(seed)

    bets_7d = rng.integers(0, 50, n_samples)
    bets_24h = rng.integers(0, 21, n_samples)
    device_count_30d = rng.integers(1, 11, n_samples)

    signal = (bets_24h >= 15) & (device_count_30d >= 7)
    noise = rng.random(n_samples) < 0.05
    is_fraud = np.where(signal, ~noise, noise & (rng.random(n_samples) < 0.2))
    is_fraud = is_fraud.astype(int)

    start = datetime(2026, 1, 1)
    event_time = [start + timedelta(minutes=30 * i) for i in range(n_samples)]

    data = {
        "event_time": event_time,
        "bets_7d": bets_7d,
        "bets_24h": bets_24h,
        "avg_bet_30d": rng.uniform(10, 500, n_samples),
        "std_bet_30d": rng.uniform(5, 200, n_samples),
        "max_bet_30d": rng.uniform(100, 5000, n_samples),
        "deposits_24h": rng.integers(0, 10, n_samples),
        "total_deposit_30d": rng.uniform(0, 10000, n_samples),
        "device_count_30d": device_count_30d,
        "ip_count_30d": rng.integers(1, 50, n_samples),
        "country_count_30d": rng.integers(1, 5, n_samples),
        "wins_7d": rng.integers(0, 20, n_samples),
        "settled_7d": rng.integers(1, 50, n_samples),
        "account_age_days": rng.integers(1, 1000, n_samples),
        "win_rate_7d": rng.uniform(0, 1, n_samples),
        "bet_cv_30d": rng.uniform(0, 5, n_samples),
        "deposit_bet_ratio": rng.uniform(0, 10, n_samples),
        "multi_device": (device_count_30d > 3).astype(int),
        "multi_ip": (rng.integers(1, 50, n_samples) > 10).astype(int),
        "high_roller": rng.integers(0, 2, n_samples),
        "rapid_bettor": (bets_24h >= 15).astype(int),
        "is_fraud": is_fraud,
    }

    df = pl.DataFrame(data)
    missing = set(MODEL_FEATURES) - set(df.columns)
    assert not missing, f"fixture missing features: {missing}"
    return df


@pytest.fixture(scope="session")
def fraud_data() -> pl.DataFrame:
    """Labeled synthetic dataset (shared across tests to avoid retraining)."""
    return make_fraud_dataset()


@pytest.fixture(scope="session")
def trained_model(fraud_data) -> FraudModel:
    """Model trained once per session on the synthetic dataset."""
    model = FraudModel()
    model.train(fraud_data)
    return model
