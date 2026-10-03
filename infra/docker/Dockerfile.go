# Multi-stage Dockerfile for Go services
# Opus Casino - Auth, User, Payment, Bonus, Casino, Notification, KYC
#
# The build context is the repository root, because every service module
# carries
#     replace github.com/opus-casino/proto => ../../../libs/proto
# and that relative path only resolves if the module keeps its place under
# services/go/. The tree is therefore mirrored into /src instead of being
# flattened, and the build runs from the module's own directory.
# SERVICE_PATH selects the module, exactly as ci-go.yml passes it.

# =============================================================================
# Stage 1: Builder
# =============================================================================
# Must be at least the highest `go` directive across services/go/*/go.mod,
# currently 1.26.0. An older builder fails before compiling anything:
#   go.mod requires go >= 1.26.0 (running go 1.22.12)
FROM golang:1.26-alpine3.22 AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    gcc \
    musl-dev \
    curl

# Pinned, not "latest": the remote buf plugins must resolve to the same
# stubs every run, or the image and CI disagree. Matches ci-go.yml.
ARG BUF_VERSION=1.73.0

# Arguments
# SERVICE_PATH: module directory under services/go/ (default: auth)
# CMD_PATH: main package path inside that module, module-relative. Left empty
#   by default so the main package is discovered rather than assumed: it sits
#   at the module root in auth, user, bonus, casino, notification and kyc, but
#   under cmd/server in payment. `./.` there fails with "no Go files in /src".
# VERSION: stamped into the binary
ARG SERVICE_PATH=services/go/auth
ARG CMD_PATH=
ARG VERSION=dev

WORKDIR /src

# Resolve dependencies before the source is copied, so a source-only change
# keeps the module cache layer warm.
#
# The glob is deliberate: a module with no dependencies has no go.sum at all
# (kyc is one), and naming the file explicitly aborts the build with
#   failed to calculate checksum ... "/services/go/kyc/go.sum": not found
# go.mod always matches, so the COPY still succeeds without it.
COPY ${SERVICE_PATH}/go.* ./
COPY libs/proto/go.mod libs/proto/go.sum libs/proto/

# libs/proto/gen is gitignored, so a fresh checkout has no Go stubs and no
# module that imports github.com/opus-casino/proto/gen/go can compile.
# Generating them here is the same step CI - Go runs.
RUN curl -fsSL -o /tmp/buf.tar.gz \
        "https://github.com/bufbuild/buf/releases/download/v${BUF_VERSION}/buf-Linux-x86_64.tar.gz" \
    && tar -xzf /tmp/buf.tar.gz -C /tmp \
    && install -m 0755 /tmp/buf/bin/buf /usr/local/bin/buf \
    && buf --version

COPY libs/proto/ libs/proto/
RUN cd libs/proto && buf generate --template buf.gen.yaml --output gen/go \
    && test -f gen/go/common/v1/money.pb.go

# Mirror the repository layout: services/go/<service> must stay three levels
# below the root for ../../../libs/proto to resolve.
COPY ${SERVICE_PATH}/ ${SERVICE_PATH}/

# Discover the main package and build in one layer: an ARG is not exported to
# later RUN steps, so a value computed in a previous layer would arrive empty.
# The shell variable is deliberately not called CMD_PATH: Docker substitutes
# ARG references textually before the shell runs, so an assignment to that name
# would be overwritten by the (empty) ARG expansion and the build would target
# the module root.
RUN cd ${SERVICE_PATH} \
    && MAIN_PKG="${CMD_PATH}" \
    && go mod download \
    && if [ -z "${MAIN_PKG}" ]; then \
           MAIN_PKG="$(go list -f '{{if eq .Name "main"}}{{.Dir}}{{end}}' ./... \
               | grep . \
               | head -n 1)"; \
       fi \
    && test -n "${MAIN_PKG}" \
       || { echo "no main package found in ${SERVICE_PATH}"; exit 1; } \
    && echo "main package: ${MAIN_PKG}" \
    && CGO_ENABLED=0 GOOS=linux go build \
        -ldflags="-w -s -X main.Version=${VERSION}" \
        -o /build/app \
        "${MAIN_PKG}"

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

# Run the application
ENTRYPOINT ["/usr/local/bin/app"]

# =============================================================================
# Stage 3: Debug (for development)
# =============================================================================
FROM alpine:3.22 AS debug

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
