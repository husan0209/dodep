# Multi-stage Dockerfile for Go services
# Opus Casino - Auth, User, Payment, Bonus, Casino, Notification, KYC
#
# Build context is the REPOSITORY ROOT, not the service directory, because every
# service go.mod carries `replace github.com/opus-casino/proto => ../../../libs/proto`.
# Copying the service directory alone breaks that relative path.
#
# Build args (paths relative to the repository root):
#   SERVICE_PATH - module directory, e.g. services/go/auth
#   CMD_PATH     - main package relative to that directory (payment uses ./cmd/server)
#   VERSION      - stamped into the binary

# =============================================================================
# Stage 1: Builder
# =============================================================================
# The Go version must be >= the `go` directive of every module in the matrix
# (1.25.0 after the dependency bump). A 1.22 base refused to load the graph.
FROM golang:1.25-alpine AS builder

# Install build dependencies. `make` drives the repo's own
# `make -C libs/proto gen-go` target and `curl` fetches the pinned buf release;
# alpine's base image ships neither.
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    curl \
    make

# Set working directory
WORKDIR /src

ARG SERVICE_PATH=services/go/auth
ARG CMD_PATH=.
ARG VERSION=dev

# Dependency layer: the module manifests the build resolves, plus the replaced
# proto module. Copied with their full relative paths so the
# `../../../libs/proto` replacement keeps resolving.
#
# `go mod download` is run inside a RUN rather than immediately after the COPY:
# services/go/kyc is a placeholder with no imports, so `go mod tidy` legitimately
# leaves it without a go.sum, and a bare COPY of a missing go.sum aborts the
# whole build before any code is reached.
COPY ${SERVICE_PATH}/go.mod ${SERVICE_PATH}/
COPY libs/proto/go.mod libs/proto/

# Download dependencies
WORKDIR /src/${SERVICE_PATH}
RUN go mod download

# libs/proto/gen is gitignored (libs/proto/.gitignore), so a checkout has no
# generated stubs and every service fails to build with
# "no required module provides package github.com/opus-casino/proto/gen/go/...".
# The stubs are therefore generated inside the image, from the repo's own
# buf.gen.yaml, which pins the plugin versions used by CI.
WORKDIR /src
COPY . .
RUN BUF_VERSION=1.73.0 \
    && curl -fsSL -o /tmp/buf.tar.gz \
       "https://github.com/bufbuild/buf/releases/download/v${BUF_VERSION}/buf-Linux-x86_64.tar.gz" \
    && tar -xzf /tmp/buf.tar.gz -C /tmp \
    && install -m 0755 /tmp/buf/bin/buf /usr/local/bin/buf \
    && make -C libs/proto gen-go \
    && test -d libs/proto/gen/go

# Build the service. CGO_ENABLED=0 pairs with the distroless/static runtime below.
WORKDIR /src/${SERVICE_PATH}
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

# Run the application. No HEALTHCHECK here: distroless/static has no shell, and
# the binary does not take a "health" subcommand — the probe would have marked
# every container unhealthy.
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

ENV APP_ENV=development
ENV TZ=UTC

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/app"]