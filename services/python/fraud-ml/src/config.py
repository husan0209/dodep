"""
Configuration for Fraud ML Service
"""
from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    """Application settings."""
    
    # Server
    http_port: int = 8000
    http_host: str = "0.0.0.0"
    
    # Environment
    app_env: str = "development"
    log_level: str = "INFO"
    
    # ClickHouse
    clickhouse_host: str = "localhost"
    clickhouse_port: int = 9000
    clickhouse_database: str = "opus_casino"
    clickhouse_user: str = "default"
    clickhouse_password: str = ""
    
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
    
    # Feature extraction
    lookback_days: int = 90
    feature_cache_ttl_hours: int = 24
    
    # Model path
    model_path: str = "/app/models"
    
    # Redpanda (for notifications)
    redpanda_brokers: list[str] = ["localhost:9092"]
    
    class Config:
        env_file = ".env"
        case_sensitive = False


settings = Settings()
