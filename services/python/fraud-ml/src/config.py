"""
Configuration for Fraud ML Service
"""

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Application settings."""

    model_config = SettingsConfigDict(
        env_file=".env",
        case_sensitive=False,
        # settings carry model_* keys (model_path, model_quality_threshold);
        # pydantic reserves the "model_" namespace for its own models
        protected_namespaces=(),
    )

    # Server
    http_port: int = 8000
    # 0.0.0.0 is required inside Kubernetes (the pod IP must be reachable);
    # in-cluster exposure is controlled by NetworkPolicy, not the bind host.
    http_host: str = "0.0.0.0"  # nosec B104

    # Environment
    app_env: str = "development"
    log_level: str = "INFO"

    # ClickHouse
    clickhouse_host: str = "localhost"
    clickhouse_port: int = 9000
    clickhouse_database: str = "opus_casino"
    clickhouse_user: str = "default"
    # Never hardcode credentials: inject CLICKHOUSE_PASSWORD in the
    # environment (empty is valid for a local dev ClickHouse).
    clickhouse_password: str = ""  # nosec B107

    @property
    def clickhouse_url(self) -> str:
        return f"clickhouse://{self.clickhouse_user}:{self.clickhouse_password}@{self.clickhouse_host}:{self.clickhouse_port}/{self.clickhouse_database}"

    # S3 for model storage
    s3_bucket: str = "opus-casino-models"
    s3_region: str = "us-east-1"
    s3_endpoint: str | None = None  # For MinIO

    # Model quality thresholds
    model_quality_threshold: float = 0.90  # AUC threshold
    precision_threshold: float = 0.50  # precision@90recall threshold
    max_fpr: float = 0.05  # hard FPR budget at production operating point
    min_recall_at_max_fpr: float = 0.70  # min recall within the FPR budget
    target_recall: float = 0.90  # reporting recall target

    # Feature extraction
    lookback_days: int = 90
    feature_cache_ttl_hours: int = 24

    # Model path
    model_path: str = "/app/models"

    # Redpanda (for notifications)
    redpanda_brokers: list[str] = ["localhost:9092"]


settings = Settings()
