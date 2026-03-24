# Multi-stage Dockerfile for Go services
# Opus Casino - Auth, User, Payment, Bonus, Casino, Notification, KYC

# =============================================================================
# Stage 1: Builder
# =============================================================================
FROM golang:1.21-alpine3.19 AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    gcc \
    musl-dev

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build arguments
ARG SERVICE_PATH=auth
ARG PROFILE=release

# Build the service
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.Version=${VERSION:-dev}" \
    -o /build/app \
    ./${SERVICE_PATH}

# =============================================================================
# Stage 2: Runtime (distroless for security)
# =============================================================================
FROM gcr.io/distroless/static-debian12 AS runtime

# Copy binary from builder
COPY --from=builder /build/app /usr/local/bin/app

# Copy CA certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Set working directory
WORKDIR /

# Environment variables
ENV GIN_MODE=release
ENV TZ=UTC

# Expose default port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/app", "health"] || exit 1

# Run the application
ENTRYPOINT ["/usr/local/bin/app"]

# =============================================================================
# Stage 3: Debug (for development)
# =============================================================================
FROM alpine:3.19 AS debug

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    netcat-openbsd \
    procps

# Copy binary from builder (debug build)
COPY --from=builder /build/app /usr/local/bin/app

WORKDIR /

ENV GIN_MODE=debug
ENV TZ=UTC

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/app"]
