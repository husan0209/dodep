# WebSocket Gateway Dockerfile
# Multi-stage build with distroless runtime

# =============================================================================
# Stage 1: Builder
# =============================================================================
FROM rust:1.82-slim-bookworm AS builder

RUN apt-get update && apt-get install -y \
    pkg-config \
    libssl-dev \
    cmake \
    clang \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

# Copy workspace Cargo manifests
COPY services/rust/Cargo.toml Cargo.toml
COPY services/rust/Cargo.lock Cargo.lock
COPY services/rust/betting-engine/Cargo.toml betting-engine/Cargo.toml
COPY services/rust/wallet-core/Cargo.toml wallet-core/Cargo.toml
COPY services/rust/websocket-gateway/Cargo.toml websocket-gateway/Cargo.toml

# Copy shared libs
COPY libs/proto libs/proto

# Create dummy src for dependency caching
RUN mkdir -p betting-engine/src wallet-core/src websocket-gateway/src
RUN echo "fn main() {}" > betting-engine/src/main.rs
RUN echo "fn main() {}" > wallet-core/src/main.rs
RUN echo "fn main() {}" > websocket-gateway/src/main.rs

RUN cargo fetch --locked

# Remove dummy src
RUN rm -rf betting-engine/src wallet-core/src websocket-gateway/src

# Copy source code
COPY services/rust/betting-engine/src betting-engine/src
COPY services/rust/wallet-core/src wallet-core/src
COPY services/rust/websocket-gateway/src websocket-gateway/src

# Build websocket-gateway
RUN cargo build --package websocket-gateway --release --locked

# =============================================================================
# Stage 2: Runtime (distroless)
# =============================================================================
FROM gcr.io/distroless/cc-debian12:nonroot AS runtime

COPY --from=builder /build/target/release/websocket-gateway /usr/local/bin/app

WORKDIR /

ENV RUST_LOG=info
ENV RUST_BACKTRACE=1

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/app"]
