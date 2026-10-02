"""Analytics service configuration."""

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Application settings (env-driven)."""

    model_config = SettingsConfigDict(
        env_file=".env",
        case_sensitive=False,
        # No model_* keys here, but keep the guard so adding one stays safe.
        protected_namespaces=(),
    )

    app_name: str = "Analytics Service"
    version: str = "1.0.0"
    debug: bool = False
    log_level: str = "INFO"

    http_host: str = "0.0.0.0"  # nosec B104 - container bind, NetworkPolicy governs
    http_port: int = 8000

    # ClickHouse
    clickhouse_host: str = "localhost"
    clickhouse_port: int = 8123
    clickhouse_database: str = "opus_casino"
    clickhouse_user: str = "default"
    clickhouse_password: str = ""  # nosec B107 - injected via env in deployments
    clickhouse_query_timeout: int = 30

    # Reporting defaults
    default_range_days: int = 30
    max_range_days: int = 366
    default_limit: int = 100
    max_limit: int = 500


settings = Settings()
