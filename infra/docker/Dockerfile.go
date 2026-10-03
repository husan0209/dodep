# Multi-stage Dockerfile for Go services
# Opus Casino - Auth, User, Payment, Bonus, Casino, Notification, KYC

# =============================================================================
# Stage 1: Builder
# =============================================================================
# The build context is the REPOSITORY ROOT, not the service directory: every
# service resolves the shared contract module through a
# `replace github.com/opus-casino/proto => ../../../libs/proto` directive, so
# libs/proto must be inside the context for the module graph to resolve.
#
# NOTE: builder must stay >= the highest `go` directive across services
# (currently go 1.25: the CVE-fixed pgx/grpc/fiber releases require it). Go 1.22
# is EOL since Feb 2025 and ships no security fixes - do NOT downgrade.
FROM golang:1.26.8-alpine3.23 AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    gcc \
    musl-dev

# Build arguments (declared before the first COPY that uses them)
# SERVICE_PATH: service directory under services/go/ (e.g. services/go/auth)
# CMD_PATH:     relative path to the main package within that service
# VERSION:      injected into main.Version via -ldflags
ARG SERVICE_PATH=services/go/auth
ARG CMD_PATH=.
ARG VERSION=dev

# Keep the repository layout inside the builder. The services' `replace
# github.com/opus-casino/proto => ../../../libs/proto` directive is resolved
# relative to the module root, so the service must sit at its real depth and
# libs/proto must sit next to services/.
WORKDIR /build

# Copy the module definition first so `go mod download` stays cached until the
# dependency graph actually changes. The glob covers go.mod plus go.sum where
# present: kyc declares no external imports and so commits no go.sum.
COPY ${SERVICE_PATH}/go.* ${SERVICE_PATH}/

# The proto module is a local `replace` target and must exist before download.
COPY libs/proto/ ./libs/proto/

WORKDIR /build/${SERVICE_PATH}

# Download dependencies
RUN go mod download

# Copy source code. Both sides are given explicitly: `COPY . .` would copy the
# whole context (the repository root) into the current directory.
COPY ${SERVICE_PATH}/ /build/${SERVICE_PATH}/

# Build the service
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.Version=${VERSION}" \
    -o /build/app \
    ./${CMD_PATH}

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
ENV APP_ENV=production
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
FROM alpine:3.21 AS debug

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

ENV APP_ENV=development
ENV TZ=UTC

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/app"]
