"""
Configuration module for Fraud ML Service

Rule-based detector thresholds (IsolationForest/RandomForest score cut-offs
used by internal.models.fraud_detector). The XGBoost risk model has its own
FPR-constrained threshold, persisted in model_card.json / ONNX metadata.
"""

import json
from typing import Any

from pydantic_settings import BaseSettings, SettingsConfigDict


def parse_brokers(value: Any) -> list[str]:
    """
    Parse a broker list from settings.

    Both forms exist in practice:
      - CSV:  REDPANDA_BROKERS=localhost:9092,kafka:9092  (CI does this)
      - JSON: REDPANDA_BROKERS=["localhost:9092"]

    pydantic-settings JSON-decodes complex (list-typed) fields coming from
    the environment, so a plain CSV value raises SettingsError while the
    module is being imported — taking the whole service down. Keeping the
    field a plain string and parsing it here removes that failure mode.
    """
    if isinstance(value, (list, tuple)):
        return [str(item) for item in value]
    raw = str(value).strip()
    if raw.startswith("["):
        try:
            parsed = json.loads(raw)
        except json.JSONDecodeError:
            parsed = None
        if isinstance(parsed, list):
            return [str(item) for item in parsed]
    return [broker.strip() for broker in raw.split(",") if broker.strip()]


class Settings(BaseSettings):
    """Application settings"""

    model_config = SettingsConfigDict(
        env_file=".env",
        case_sensitive=False,
        protected_namespaces=(),  # model_* keys are intentional here
    )

    # Server
    http_port: int = 8000

    # Database
    clickhouse_host: str = "localhost"
    clickhouse_port: int = 9000
    clickhouse_user: str = "default"
    clickhouse_password: str = ""
    clickhouse_database: str = "opus_casino"

    postgres_host: str = "localhost"
    postgres_port: int = 5432
    postgres_user: str = "postgres"
    postgres_password: str = "postgres"
    postgres_database: str = "opus_casino"

    # Redpanda brokers. Stored as a raw string on purpose: see parse_brokers.
    redpanda_brokers: str = "localhost:9092"

    # ML Models
    model_path: str = "/app/models"
    model_update_freq_hours: int = 24

    # Monitoring
    prometheus_port: int = 9090

    # Environment
    app_env: str = "development"
    log_level: str = "INFO"

    # Fraud detection thresholds (rule-based detectors)
    bet_anomaly_threshold: float = 0.7
    bonus_abuse_threshold: float = 0.6
    payment_fraud_threshold: float = 0.65
    account_takeover_threshold: float = 0.75

    @property
    def redpanda_broker_list(self) -> list[str]:
        """Kafka brokers as a list, from a CSV or JSON env value."""
        return parse_brokers(self.redpanda_brokers)


settings = Settings()
