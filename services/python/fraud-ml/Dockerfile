# Build stage
FROM python:3.11-slim AS builder

WORKDIR /app

# Install Rust (required for Polars)
RUN apt-get update && apt-get install -y curl
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
ENV PATH="/root/.cargo/bin:${PATH}"

# Install dependencies
COPY pyproject.toml .
RUN pip install --no-cache-dir -e ".[dev]"

# Copy source code.
# internal/ must be copied here as well as src/. main.py imports both trees
# (`from internal.api.routes import ...` and `from src.config import ...`), and
# so do the tests. Leaving internal/ out made this stage fail on
# `from internal.api.routes import router` before the image was ever produced.
COPY src/ ./src/
COPY internal/ ./internal/
COPY tests/ ./tests/
# main.py is needed here too, not only in the runner: tests/test_api.py does
# `from main import app`, so without it this stage failed during collection.
COPY main.py .

# Run tests
RUN pytest tests/ --cov=src/ --cov-report=xml

# Final stage
FROM python:3.11-slim AS runner

WORKDIR /app

ENV PYTHONUNBUFFERED=1
ENV PYTHONDONTWRITEBYTECODE=1

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN useradd -m -u 1000 appuser

# Copy installed packages from builder
COPY --from=builder /usr/local/lib/python3.11/site-packages /usr/local/lib/python3.11/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin

# Copy application code
# main.py resolves `internal.*` at import time, so the runner needs the package
# next to src/ and main.py. Without it `uvicorn main:app` aborts at startup with
# ModuleNotFoundError: No module named 'internal'.
COPY --chown=appuser:appuser src/ ./src/
COPY --chown=appuser:appuser internal/ ./internal/
COPY --chown=appuser:appuser main.py .

# Create models directory
RUN mkdir -p /app/models && chown appuser:appuser /app/models

USER appuser

# Expose ports
EXPOSE 8000

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8000/health || exit 1

# Run the application
CMD ["python", "-m", "uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
