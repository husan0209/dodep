# SKILL #24 — python-fastapi-internal.skill.md

```markdown
# python-fastapi-internal.skill.md
# GAMBLING PLATFORM — PYTHON FASTAPI INTERNAL SERVICES
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: Python ML Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

FastAPI used ONLY for internal ML-related endpoints.
NOT exposed to public internet. Called by other services via gRPC or HTTP.

Use cases:
- Model retraining trigger API
- Feature computation API (batch)
- Report generation trigger
- Model health/status endpoint

# ============================================================
# SECTION 2: PROJECT SETUP
# ============================================================

```toml
# pyproject.toml
[project]
name = "fraud-ml-service"
version = "0.1.0"
requires-python = ">=3.12"
dependencies = [
    "fastapi>=0.115",
    "uvicorn[standard]>=0.30",
    "pydantic>=2.9",
    "pydantic-settings>=2.5",
    "structlog>=24.4",
    "polars>=1.7",
    "xgboost>=2.1",
    "scikit-learn>=1.5",
    "onnx>=1.16",
    "skl2onnx>=1.17",
    "clickhouse-connect>=0.7",
    "boto3>=1.35",
    "prometheus-client>=0.21",
    "opentelemetry-api>=1.27",
    "opentelemetry-sdk>=1.27",
    "opentelemetry-instrumentation-fastapi>=0.48",
    "sentry-sdk[fastapi]>=2.14",
    "httpx>=0.27",
]

[project.optional-dependencies]
dev = [
    "pytest>=8.3",
    "pytest-asyncio>=0.24",
    "ruff>=0.6",
    "mypy>=1.11",
]
============================================================
SECTION 3: APPLICATION STRUCTURE
============================================================
Python

# src/app.py
from contextlib import asynccontextmanager
from fastapi import FastAPI
from prometheus_client import make_asgi_app

from src.config import Settings
from src.routes import training, reports, health
from src.dependencies import init_dependencies

@asynccontextmanager
async def lifespan(app: FastAPI):
    """Startup and shutdown logic."""
    settings = Settings()
    deps = await init_dependencies(settings)
    app.state.deps = deps
    yield
    # Cleanup
    await deps.close()

def create_app() -> FastAPI:
    app = FastAPI(
        title="Fraud ML Service",
        description="Internal ML service for fraud detection",
        version="0.1.0",
        docs_url="/docs" if Settings().environment == "dev" else None,
    )
    
    app.include_router(health.router, tags=["health"])
    app.include_router(training.router, prefix="/api/v1", tags=["training"])
    app.include_router(reports.router, prefix="/api/v1", tags=["reports"])
    
    # Prometheus metrics
    metrics_app = make_asgi_app()
    app.mount("/metrics", metrics_app)
    
    return app

app = create_app()
============================================================
SECTION 4: CONFIGURATION
============================================================
Python

# src/config.py
from pydantic_settings import BaseSettings

class Settings(BaseSettings):
    service_name: str = "fraud-ml-service"
    environment: str = "dev"
    
    clickhouse_host: str = "localhost"
    clickhouse_port: int = 9000
    clickhouse_database: str = "analytics"
    
    s3_bucket: str = "platform-ml-models"
    s3_region: str = "us-east-1"
    
    model_quality_min_auc: float = 0.90
    model_quality_min_precision: float = 0.50
    
    otel_endpoint: str = "http://otel-collector:4317"
    sentry_dsn: str = ""
    
    class Config:
        env_prefix = "APP_"
        env_file = ".env"
============================================================
SECTION 5: ROUTE HANDLERS
============================================================
Python

# src/routes/training.py
from fastapi import APIRouter, BackgroundTasks, Depends, HTTPException
from pydantic import BaseModel
from datetime import datetime

import structlog

logger = structlog.get_logger()
router = APIRouter()

class TrainRequest(BaseModel):
    lookback_days: int = 90
    force: bool = False

class TrainResponse(BaseModel):
    run_id: str
    status: str
    message: str

@router.post("/training/trigger", response_model=TrainResponse)
async def trigger_training(
    req: TrainRequest,
    background_tasks: BackgroundTasks,
    deps = Depends(get_deps),
):
    """Trigger model retraining. Runs in background."""
    run_id = datetime.utcnow().strftime("%Y%m%d_%H%M%S")
    
    # Don't allow concurrent training
    if deps.training_lock.locked():
        raise HTTPException(409, "Training already in progress")
    
    background_tasks.add_task(
        run_training_pipeline,
        deps=deps,
        run_id=run_id,
        lookback_days=req.lookback_days,
    )
    
    return TrainResponse(
        run_id=run_id,
        status="started",
        message="Training pipeline started in background",
    )

@router.get("/training/status")
async def training_status(deps = Depends(get_deps)):
    """Get latest training run status."""
    latest = deps.training_store.get_latest()
    if not latest:
        return {"status": "no_runs"}
    return latest

async def run_training_pipeline(deps, run_id: str, lookback_days: int):
    """Background task for training."""
    async with deps.training_lock:
        try:
            pipeline = deps.create_pipeline()
            result = pipeline.run(lookback_days=lookback_days)
            deps.training_store.save(run_id, "completed", result)
            logger.info("training.complete", run_id=run_id, **result)
        except Exception as e:
            deps.training_store.save(run_id, "failed", {"error": str(e)})
            logger.error("training.failed", run_id=run_id, error=str(e))
Python

# src/routes/reports.py
router = APIRouter()

class ReportRequest(BaseModel):
    report_type: str  # "daily_financial", "rtp_monitoring"
    date: str         # "2025-01-15"

@router.post("/reports/generate")
async def generate_report(
    req: ReportRequest,
    background_tasks: BackgroundTasks,
    deps = Depends(get_deps),
):
    """Trigger report generation."""
    date = datetime.strptime(req.date, "%Y-%m-%d")
    
    background_tasks.add_task(
        deps.report_generator.generate_daily_financial_report,
        date=date,
    )
    
    return {"status": "generating", "report_type": req.report_type, "date": req.date}
Python

# src/routes/health.py
router = APIRouter()

@router.get("/healthz")
async def liveness():
    return {"status": "ok"}

@router.get("/readyz")
async def readiness(deps = Depends(get_deps)):
    ch_ok = deps.clickhouse.ping()
    return {
        "status": "ok" if ch_ok else "degraded",
        "clickhouse": "ok" if ch_ok else "error",
    }
============================================================
SECTION 6: DOCKERFILE
============================================================
Dockerfile

FROM python:3.12-slim

WORKDIR /app

# Install system deps
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential && rm -rf /var/lib/apt/lists/*

# Install Python deps
COPY pyproject.toml .
RUN pip install --no-cache-dir .

# Copy source
COPY src/ src/

EXPOSE 8080

USER nobody

CMD ["uvicorn", "src.app:app", "--host", "0.0.0.0", "--port", "8080", "--workers", "2"]
============================================================
SECTION 7: ANTI-PATTERNS
============================================================
text

❌ NEVER expose FastAPI to public internet (internal only, behind Istio)
❌ NEVER run ML inference in FastAPI request path → export ONNX, serve in Rust
❌ NEVER use sync blocking calls in async handlers → USE httpx, asyncio
❌ NEVER skip type hints → USE Pydantic models for all request/response
❌ NEVER use print() → USE structlog
❌ NEVER return 200 with error body → USE proper HTTPException codes
❌ NEVER run long tasks in request handler → USE BackgroundTasks
❌ NEVER skip health endpoints (K8s needs them)
❌ NEVER hardcode secrets → USE environment variables / Vault