# Dockerfile for Wallet Core Service
# Multi-stage build for minimal production image

# =====================
# Build stage
# =====================
FROM rust:1.75-slim-bookworm AS builder

# Install dependencies
RUN apt-get update && apt-get install -y \
    pkg-config \
    libssl-dev \
    && rm -rf /var/lib/apt/lists/*

# Create new user
RUN useradd -m -u 1000 app

# Set working directory
WORKDIR /app

# Copy source
COPY . .

# Build in release mode
RUN cargo build --release --bin wallet-core

# =====================
# Runtime stage
# =====================
FROM debian:bookworm-slim AS runtime

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    libssl3 \
    && rm -rf /var/lib/apt/lists/* \
    && adduser --disabled-password --gecos '' --uid 1000 app

# Copy binary from builder
COPY --from=builder --chown=app:app /app/target/release/wallet-core /usr/local/bin/wallet-core

# Set working directory
WORKDIR /app

# Switch to non-root user
USER app

# Expose ports
# gRPC: 50053
# HTTP: 3003
# Metrics: 9003
EXPOSE 50053 3003 9003

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/wallet-core", "--health"]

# Run the service
ENTRYPOINT ["/usr/local/bin/wallet-core"]
