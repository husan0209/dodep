"""
Fraud ML Service - Main Application
Detects fraudulent activity using machine learning models

Architecture:
- Python trains models weekly, exports to ONNX
- Rust serves models in production (< 5ms inference)
- This FastAPI service is for training and batch scoring
"""
import logging
from contextlib import asynccontextmanager

import structlog
from fastapi import FastAPI
from prometheus_client import make_asgi_app

from src.config import settings
from src.data.clickhouse import ClickHouseClient

# NOTE: the API router, the Redpanda consumer and the batch FraudDetector
# orchestrator are not implemented under src/ yet (only stale copies live in the
# undeployed internal/ tree), so they are intentionally not wired in here.
# main.py is the container entrypoint (CMD uvicorn main:app) and the image only
# ships src/ + main.py, so every import below must resolve inside src/.

# structlog wants a numeric level; logging.INFO and friends are plain ints.
LOG_LEVEL = logging.getLevelNamesMapping().get(settings.log_level.upper(), logging.INFO)

# Configure structured logging
structlog.configure(
    processors=[
        structlog.processors.TimeStamper(fmt="iso"),
        structlog.processors.add_log_level,
        structlog.processors.dict_tracebacks,
        structlog.processors.JSONRenderer(),
    ],
    wrapper_class=structlog.make_filtering_bound_logger(LOG_LEVEL),
    context_class=dict,
    logger_factory=structlog.PrintLoggerFactory(),
    cache_logger_on_first_use=True,
)

logger = structlog.get_logger()


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan manager."""
    # Startup
    logger.info(
        "fraud_ml.startup",
        version="1.0.0",
        environment=settings.app_env,
    )

    # Initialize ClickHouse client
    app.state.clickhouse_client = ClickHouseClient(
        host=settings.clickhouse_host,
        port=settings.clickhouse_port,
        database=settings.clickhouse_database,
        user=settings.clickhouse_user,
        password=settings.clickhouse_password,
    )
    logger.info("clickhouse.connected")

    yield

    # Shutdown
    logger.info("fraud_ml.shutdown")
    app.state.clickhouse_client.close()


# Create FastAPI application
app = FastAPI(
    title="Fraud ML Service",
    description="Machine Learning service for fraud detection",
    version="1.0.0",
    lifespan=lifespan,
)

# Mount Prometheus metrics endpoint
metrics_app = make_asgi_app()
app.mount("/metrics", metrics_app)


@app.get("/health")
async def health_check():
    """Health check endpoint."""
    return {"status": "healthy"}


@app.get("/ready")
async def readiness_check():
    """Readiness check endpoint."""
    return {"status": "ready"}


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(
        # The ASGI app lives in this module (services/python/fraud-ml/main.py);
        # there is no src/main.py.
        "main:app",
        host=settings.http_host,
        port=settings.http_port,
        reload=settings.app_env == "development",
    )
