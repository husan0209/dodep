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
from fastapi.responses import JSONResponse
from prometheus_client import make_asgi_app

from internal.api.routes import router as api_router
from internal.consumers.redpanda_consumer import RedpandaConsumer
from internal.models.fraud_detector import FraudDetector
from src.config import settings
from src.data.clickhouse import ClickHouseClient

# Configure structured logging
structlog.configure(
    processors=[
        structlog.processors.TimeStamper(fmt="iso"),
        structlog.processors.add_log_level,
        structlog.processors.dict_tracebacks,
        structlog.processors.JSONRenderer(),
    ],
    wrapper_class=structlog.make_filtering_bound_logger(
        logging.getLevelNamesMapping().get(settings.log_level.upper(), logging.INFO)
    ),
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

    # Initialize ClickHouse client. Detection endpoints use in-memory
    # detectors, so an unavailable analytics store must not take the whole
    # API down; training jobs fail loudly on their own connection instead.
    app.state.clickhouse_client = None
    try:
        app.state.clickhouse_client = ClickHouseClient(
            host=settings.clickhouse_host,
            port=settings.clickhouse_port,
            database=settings.clickhouse_database,
            user=settings.clickhouse_user,
            password=settings.clickhouse_password,
        )
        logger.info("clickhouse.connected")
    except Exception as exc:
        logger.error("clickhouse.unavailable", error=str(exc))

    # Initialize fraud detector
    app.state.fraud_detector = FraudDetector(model_path=settings.model_path)
    logger.info("fraud_detector.initialized")

    # Start Redpanda consumer. The API must stay usable for batch scoring
    # and model management even when the streaming bus is unavailable.
    app.state.consumer = None
    try:
        app.state.consumer = RedpandaConsumer(
            brokers=settings.redpanda_brokers,
            fraud_detector=app.state.fraud_detector,
        )
        await app.state.consumer.start()
        logger.info("redpanda_consumer.started")
    except Exception as exc:
        logger.warning("redpanda_consumer.unavailable", error=str(exc))

    yield

    # Shutdown
    logger.info("fraud_ml.shutdown")
    if app.state.consumer is not None:
        await app.state.consumer.stop()
    if app.state.clickhouse_client is not None:
        app.state.clickhouse_client.close()


# Create FastAPI application
app = FastAPI(
    title="Fraud ML Service",
    description="Machine Learning service for fraud detection",
    version="1.0.0",
    lifespan=lifespan,
)

# Include API routes
app.include_router(api_router, prefix="/api/v1")

# Mount Prometheus metrics endpoint
metrics_app = make_asgi_app()
app.mount("/metrics", metrics_app)


@app.get("/health")
async def health_check():
    """Health check endpoint."""
    return {"status": "healthy"}


@app.get("/ready")
async def readiness_check():
    """
    Readiness probe (HTTP 503 when not ready — k8s semantics).

    Ready only when the fraud detector is initialized; otherwise serving
    requests would return 503 anyway.
    """
    if getattr(app.state, "fraud_detector", None) is None:
        return JSONResponse(
            status_code=503,
            content={"status": "not_ready", "reason": "fraud_detector_missing"},
        )
    return {"status": "ready"}


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(
        "main:app",
        host=settings.http_host,
        port=settings.http_port,
        reload=settings.app_env == "development",
    )
