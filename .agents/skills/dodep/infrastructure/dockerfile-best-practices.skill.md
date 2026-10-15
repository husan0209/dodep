## #40 dockerfile-best-practices.skill.md

```markdown
# dockerfile-best-practices.skill.md

## РОЛЬ
Ты создаёшь Dockerfile для микросервисов гемблинг-платформы.
Образы должны быть минимальными, безопасными и быстро собираться.

## КОНТЕКСТ
- Языки: Rust, Go, Python, Node.js
- Base images: Distroless (Google) или scratch
- Registry: AWS ECR / GCP Artifact Registry
- Multi-stage builds обязательно
- Размер образа: Rust < 30MB, Go < 20MB, Node.js < 150MB

## RUST DOCKERFILE

```dockerfile
# === Stage 1: Build ===
FROM rust:1.77-bookworm AS builder

WORKDIR /app

# 1. Кэширование зависимостей (cargo chef)
RUN cargo install cargo-chef
COPY . .
RUN cargo chef prepare --recipe-path recipe.json

FROM rust:1.77-bookworm AS cacher
WORKDIR /app
RUN cargo install cargo-chef
COPY --from=builder /app/recipe.json recipe.json
RUN cargo chef cook --release --recipe-path recipe.json

# 2. Build application
FROM rust:1.77-bookworm AS build
WORKDIR /app
COPY --from=cacher /app/target target
COPY --from=cacher /usr/local/cargo /usr/local/cargo
COPY . .
RUN cargo build --release --bin betting-engine

# === Stage 2: Runtime ===
FROM gcr.io/distroless/cc-debian12:nonroot

COPY --from=build /app/target/release/betting-engine /app/betting-engine

# Metadata
LABEL maintainer="platform-team@example.com"
LABEL service="betting-engine"

EXPOSE 8080 9000 9090

USER nonroot:nonroot

ENTRYPOINT ["/app/betting-engine"]
GO DOCKERFILE
Dockerfile

# === Stage 1: Build ===
FROM golang:1.22-bookworm AS builder

WORKDIR /app

# Кэширование зависимостей
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s -X main.version=${VERSION}" \
    -o /app/auth-service ./cmd/auth-service

# === Stage 2: Runtime ===
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /app/auth-service /app/auth-service

LABEL maintainer="platform-team@example.com"
LABEL service="auth-service"

EXPOSE 8080 9000 9090

USER nonroot:nonroot

ENTRYPOINT ["/app/auth-service"]
PYTHON DOCKERFILE (ML service)
Dockerfile

# === Stage 1: Build ===
FROM python:3.12-slim-bookworm AS builder

WORKDIR /app

RUN pip install --no-cache-dir poetry && \
    poetry config virtualenvs.create false

COPY pyproject.toml poetry.lock ./
RUN poetry install --no-dev --no-interaction --no-ansi

COPY . .

# === Stage 2: Runtime ===
FROM python:3.12-slim-bookworm

RUN groupadd -r appuser && useradd -r -g appuser -s /bin/false appuser

WORKDIR /app

# Только нужные пакеты
COPY --from=builder /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin
COPY --from=builder /app /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    libgomp1 && \
    rm -rf /var/lib/apt/lists/*

LABEL maintainer="platform-team@example.com"
LABEL service="fraud-ml-service"

EXPOSE 8080

USER appuser

CMD ["python", "-m", "uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8080"]
NEXT.JS DOCKERFILE
Dockerfile

# === Stage 1: Dependencies ===
FROM node:20-alpine AS deps
WORKDIR /app
COPY package.json pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile

# === Stage 2: Build ===
FROM node:20-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .

ENV NEXT_TELEMETRY_DISABLED=1
RUN corepack enable && pnpm build

# === Stage 3: Runtime ===
FROM node:20-alpine AS runner
WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

RUN addgroup --system --gid 1001 nodejs && \
    adduser --system --uid 1001 nextjs

# Только production файлы
COPY --from=builder /app/public ./public
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static

LABEL maintainer="platform-team@example.com"
LABEL service="web-frontend"

EXPOSE 3000

USER nextjs

CMD ["node", "server.js"]
ПРАВИЛА
text

1. Multi-stage ВСЕГДА — build tools не в runtime образе
2. Distroless / scratch для Go и Rust — минимальная поверхность атаки
3. Non-root user ВСЕГДА — USER nonroot или custom user
4. .dockerignore — исключить .git, node_modules, target/
5. Конкретные версии base image — НЕ :latest
6. COPY перед ADD — ADD только для tar archives
7. Один процесс на контейнер — НЕ supervisor/systemd
8. HEALTHCHECK в Dockerfile (backup к K8s probes)
9. LABEL с metadata (service, maintainer, version)
10. No secrets в image — ни ENV, ни COPY credentials
.dockerignore
text

# .dockerignore
.git
.github
.gitignore
*.md
LICENSE
docker-compose*.yml
Makefile

# Rust
target/
!target/release

# Go
vendor/

# Node
node_modules/
.next/
coverage/

# Python
__pycache__/
*.pyc
.venv/
.pytest_cache/

# IDE
.idea/
.vscode/
*.swp

# Env files
.env*
!.env.example
АНТИПАТТЕРНЫ
Dockerfile

# ❌ ПЛОХО: один stage, build tools в runtime
FROM rust:1.77
COPY . .
RUN cargo build --release
CMD ["./target/release/app"]
# Образ: ~2GB, содержит компилятор, исходники

# ✅ ПРАВИЛЬНО: multi-stage, distroless runtime
# Образ: ~25MB, только бинарник

# ❌ ПЛОХО: COPY . . перед зависимостями
COPY . .
RUN go mod download
# Каждое изменение кода → пересборка зависимостей

# ✅ ПРАВИЛЬНО: сначала зависимости
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Зависимости кэшируются

# ❌ ПЛОХО: apt-get без cleanup
RUN apt-get update && apt-get install -y curl vim wget
# Лишние пакеты + кэш apt в образе

# ✅ ПРАВИЛЬНО:
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl && rm -rf /var/lib/apt/lists/*

# ❌ ПЛОХО: ENV с секретами
ENV DATABASE_PASSWORD=secret123

# ✅ ПРАВИЛЬНО: секреты через Vault / K8s Secrets в runtime

# ❌ ПЛОХО: root user
USER root
# или просто не указывать USER

# ✅ ПРАВИЛЬНО:
USER nonroot:nonroot
# или
USER 1000:1000
SECURITY SCANNING
YAML

# В CI: сканирование образа перед push
- name: Scan image
  uses: aquasecurity/trivy-action@v0.18
  with:
    image-ref: ${{ env.IMAGE }}
    severity: 'CRITICAL,HIGH'
    exit-code: '1'            # fail build on critical/high
    ignore-unfixed: true