# Multi-stage Dockerfile for Go services
# Opus Casino - Auth, User, Payment, Bonus, Casino, Notification, KYC

# =============================================================================
# Stage 1: Builder
# =============================================================================
# NOTE: builder must stay >= the highest `go` directive across services
# (currently go 1.24: grpc v1.80 requires it). Go 1.22 is EOL since Feb 2025
# and ships no security fixes — do NOT downgrade below go 1.24.
FROM golang:1.25.14-alpine3.23 AS builder

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
# SERVICE: directory name under services/go/ (default: auth)
# CMD_PATH: relative path to main package within service dir (default: .)
ARG SERVICE=auth
ARG CMD_PATH=.
ARG VERSION=dev

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
