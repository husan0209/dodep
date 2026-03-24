# Docker Best Practices for Opus Casino

## Общие принципы

### 1. Multi-stage сборка

Все Dockerfile используют multi-stage сборку для минимизации размера финального образа:

```dockerfile
FROM golang:1.21 AS builder
# ... build ...

FROM gcr.io/distroless/static-debian12
# ... runtime ...
```

### 2. Distroless образы

Для production используем distroless образы:
- **gcr.io/distroless/static-debian12** - для Go сервисов
- **gcr.io/distroless/cc-debian12** - для Rust сервисов
- **python:3.11-slim** - для Python сервисов (нет distroless для ML)

### 3. Безопасность

- Запуск от non-root пользователя
- Минимальные capabilities
- Read-only root filesystem (где возможно)
- Регулярное обновление базовых образов

### 4. Оптимизация размера

| Сервис | Base | Размер |
|--------|------|--------|
| Rust | distroless/cc | ~50 MB |
| Go | distroless/static | ~20 MB |
| Python | python:3.11-slim | ~150 MB |
| Next.js | node:20-alpine | ~200 MB |

### 5. Кэширование слоёв

Порядок инструкций важен для кэширования:

```dockerfile
# Копируем манифесты зависимостей первыми
COPY package.json ./
RUN npm install

# Затем копируем исходный код
COPY . .
RUN npm run build
```

## Сборка образов

### Rust сервисы

```bash
# Betting Engine
docker build -f infra/docker/Dockerfile.rust \
  --build-arg SERVICE_NAME=betting-engine \
  --build-arg PROFILE=release \
  -t opus-casino/betting-engine:latest .

# Wallet Core
docker build -f infra/docker/Dockerfile.rust \
  --build-arg SERVICE_NAME=wallet-core \
  --build-arg PROFILE=release \
  -t opus-casino/wallet-core:latest .

# WebSocket Gateway
docker build -f infra/docker/Dockerfile.rust \
  --build-arg SERVICE_NAME=websocket-gateway \
  --build-arg PROFILE=release \
  -t opus-casino/websocket-gateway:latest .
```

### Go сервисы

```bash
# Auth Service
docker build -f infra/docker/Dockerfile.go \
  --build-arg SERVICE_PATH=services/go/auth \
  --build-arg VERSION=1.0.0 \
  -t opus-casino/auth:latest .

# User Service
docker build -f infra/docker/Dockerfile.go \
  --build-arg SERVICE_PATH=services/go/user \
  -t opus-casino/user:latest .
```

### Python сервисы

```bash
# Fraud ML
docker build -f infra/docker/Dockerfile.python \
  --build-arg SERVICE_NAME=fraud-ml \
  -t opus-casino/fraud-ml:latest .

# Analytics
docker build -f infra/docker/Dockerfile.python \
  --build-arg SERVICE_NAME=analytics \
  -t opus-casino/analytics:latest .
```

### Frontend

```bash
# Next.js Web
docker build -f infra/docker/Dockerfile.frontend \
  -t opus-casino/web:latest .
```

## Запуск локально

### Development режим

```bash
# Rust (debug)
docker run -it --rm \
  -p 8080:8080 \
  -v $(pwd)/services/rust/betting-engine/src:/build/src \
  opus-casino/betting-engine:debug

# Go (debug)
docker run -it --rm \
  -p 8080:8080 \
  -v $(pwd)/services/go/auth:/app \
  opus-casino/auth:debug

# Python (dev)
docker run -it --rm \
  -p 8000:8000 \
  -v $(pwd)/services/python/fraud-ml:/app \
  opus-casino/fraud-ml:dev

# Next.js (dev)
docker run -it --rm \
  -p 3000:3000 \
  -v $(pwd)/apps/web:/app/apps/web \
  opus-casino/web:dev
```

## Health Checks

Все образы включают health checks:

```dockerfile
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/app", "--health-check"] || exit 1
```

Проверка здоровья:

```bash
docker inspect --format='{{.State.Health.Status}}' <container>
```

## Environment Variables

### Rust сервисы

| Переменная | Описание | Default |
|------------|----------|---------|
| `RUST_LOG` | Уровень логирования | `info` |
| `RUST_BACKTRACE` | Включить backtrace | `1` |
| `PORT` | Порт сервиса | `8080` |

### Go сервисы

| Переменная | Описание | Default |
|------------|----------|---------|
| `GIN_MODE` | Режим Gin | `release` |
| `TZ` | Timezone | `UTC` |
| `PORT` | Порт сервиса | `8080` |

### Python сервисы

| Переменная | Описание | Default |
|------------|----------|---------|
| `PYTHONUNBUFFERED` | Буферизация вывода | `1` |
| `TZ` | Timezone | `UTC` |
| `PORT` | Порт сервиса | `8000` |

### Frontend

| Переменная | Описание | Default |
|------------|----------|---------|
| `NODE_ENV` | Режим Node | `production` |
| `PORT` | Порт сервиса | `3000` |
| `NEXT_TELEMETRY_DISABLED` | Отключить телеметрию | `1` |

## Сканирование уязвимостей

```bash
# Trivy
trivy image opus-casino/betting-engine:latest

# Docker Scout
docker scout cves opus-casino/betting-engine:latest

# Grype
grype opus-casino/betting-engine:latest
```

## Оптимизация производительности

### 1. Использование .dockerignore

```
# .dockerignore
node_modules
.git
*.md
.env
.env.local
.next
dist
build
target
__pycache__
*.pyc
coverage
```

### 2. Alpine vs Slim

- **Alpine** - минимальный размер, но могут быть проблемы с совместимостью
- **Slim** - больше размер, но лучшая совместимость

### 3. BuildKit

Включите BuildKit для ускорения сборки:

```bash
export DOCKER_BUILDKIT=1
docker build ...
```

## Push в Registry

```bash
# ECR (AWS)
aws ecr get-login-password --region us-east-1 | \
  docker login --username AWS --password-stdin <account>.dkr.ecr.us-east-1.amazonaws.com

docker tag opus-casino/betting-engine:latest \
  <account>.dkr.ecr.us-east-1.amazonaws.com/opus-casino/betting-engine:latest

docker push <account>.dkr.ecr.us-east-1.amazonaws.com/opus-casino/betting-engine:latest
```
