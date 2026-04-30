"""
Fraud ML Service - Main Application
Detects fraudulent activity using machine learning models

Architecture:
- Python trains models weekly, exports to ONNX
- Rust serves models in production (< 5ms inference)
- This FastAPI service is for training and batch scoring
"""
import structlog
import sys
from contextlib import asynccontextmanager

import clickhouse_connect
from fastapi import FastAPI
from prometheus_client import make_asgi_app

from src.config import settings
from src.api.routes import router as api_router
from src.consumers.redpanda_consumer import RedpandaConsumer
from src.models.fraud_detector import FraudDetector
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
        getattr(sys.modules["logging"], settings.log_level.upper()).getEffectiveLevel()
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
    
    # Initialize ClickHouse client
    app.state.clickhouse_client = ClickHouseClient(
        host=settings.clickhouse_host,
        port=settings.clickhouse_port,
        database=settings.clickhouse_database,
        user=settings.clickhouse_user,
        password=settings.clickhouse_password,
    )
    logger.info("clickhouse.connected")
    
    # Initialize fraud detector
    app.state.fraud_detector = FraudDetector(model_path=settings.model_path)
    logger.info("fraud_detector.initialized")
    
    # Start Redpanda consumer
    app.state.consumer = RedpandaConsumer(
        brokers=settings.redpanda_brokers,
        fraud_detector=app.state.fraud_detector,
    )
    await app.state.consumer.start()
    logger.info("redpanda_consumer.started")
    
    yield
    
    # Shutdown
    logger.info("fraud_ml.shutdown")
    await app.state.consumer.stop()
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
    """Readiness check endpoint."""
    return {"status": "ready"}


if __name__ == "__main__":
    import uvicorn
    
    uvicorn.run(
        "src.main:app",
        host=settings.http_host,
        port=settings.http_port,
        reload=settings.app_env == "development",
    )
