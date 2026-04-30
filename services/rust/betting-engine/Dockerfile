# Dockerfile for Betting Engine (Rust)
# Multi-stage build with distroless base

# Stage 1: Build
FROM rust:1.82-slim AS builder

RUN apt-get update && apt-get install -y \
    protobuf-compiler \
    pkg-config \
    libssl-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy workspace Cargo.toml and service Cargo.toml
COPY services/rust/Cargo.toml ./services/rust/Cargo.toml
COPY services/rust/betting-engine/Cargo.toml ./services/rust/betting-engine/Cargo.toml
COPY services/rust/betting-engine/build.rs ./services/rust/betting-engine/build.rs

# Copy proto files
COPY libs/proto/ ./libs/proto/

# Copy source code
COPY services/rust/betting-engine/src/ ./services/rust/betting-engine/src/

WORKDIR /app/services/rust

# Build release
RUN cargo build --release -p betting-engine

# Stage 2: Runtime (distroless)
FROM gcr.io/distroless/cc-debian12:nonroot

LABEL maintainer="platform-team@opus-casino.com"
LABEL service="betting-engine"
LABEL language="rust"

COPY --from=builder /app/services/rust/target/release/betting-engine /usr/local/bin/betting-engine

EXPOSE 8080 9090

USER nonroot:nonroot

ENTRYPOINT ["betting-engine"]
