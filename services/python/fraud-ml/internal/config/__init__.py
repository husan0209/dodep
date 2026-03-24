"""
Configuration module for Fraud ML Service
"""

from pydantic_settings import BaseSettings
from typing import List


class Settings(BaseSettings):
    """Application settings"""
    
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
    
    # Redpanda
    redpanda_brokers: List[str] = ["localhost:9092"]
    
    # ML Models
    model_path: str = "/app/models"
    model_update_freq_hours: int = 24
    
    # Monitoring
    prometheus_port: int = 9090
    
    # Environment
    app_env: str = "development"
    log_level: str = "INFO"
    
    # Fraud detection thresholds
    bet_anomaly_threshold: float = 0.7
    bonus_abuse_threshold: float = 0.6
    payment_fraud_threshold: float = 0.65
    account_takeover_threshold: float = 0.75
    
    class Config:
        env_file = ".env"
        case_sensitive = False


settings = Settings()
