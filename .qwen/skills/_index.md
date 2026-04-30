# AI Agent Skills Index — Gambling Platform

## Как использовать
1. Определи свою РОЛЬ (какой сервис/задача)
2. Найди подходящий agent profile в `_agents.md`
3. Загрузи ТОЛЬКО перечисленные skills для этой роли (5-10 файлов максимум)

---

## Каталог skills

### 🏗️ ARCHITECTURE (загружают ВСЕ агенты)

| # | Файл | Описание | Когда нужен |
|---|------|----------|-------------|
| 1 | `architecture/architecture-overview.skill.md` | Общая архитектура платформы | ВСЕГДА |
| 2 | `architecture/microservices-patterns.skill.md` | Паттерны микросервисов | При создании нового сервиса |
| 3 | `architecture/event-driven-design.skill.md` | Event Sourcing, CQRS | При работе с событиями |
| 4 | `architecture/data-consistency.skill.md` | Транзакции, idempotency | При финансовых операциях |
| 5 | `architecture/api-design-guidelines.skill.md` | REST/gRPC стандарты | При создании API |

---

### 🦀 RUST (критический путь)

| # | Файл | Описание | Когда нужен |
|---|------|----------|-------------|
| 6 | `rust/rust-general.skill.md` | Общие правила Rust | Любой Rust код |
| 7 | `rust/rust-axum-handlers.skill.md` | HTTP handlers на Axum | REST API на Rust |
| 8 | `rust/rust-tokio-async.skill.md` | Async patterns с Tokio | Async код |
| 9 | `rust/rust-sqlx-database.skill.md` | SQLx queries | Работа с БД из Rust |
| 10 | `rust/rust-tonic-grpc.skill.md` | gRPC сервисы на tonic | gRPC на Rust |
| 11 | `rust/rust-websocket.skill.md` | WebSocket (tokio-tungstenite) | WS gateway |
| 12 | `rust/rust-error-handling.skill.md` | Обработка ошибок | Любой Rust код |
| 13 | `rust/rust-testing.skill.md` | Тесты | Тесты Rust |
| 14 | `rust/rust-performance.skill.md` | Оптимизация производительности | Performance-critical код |

---

### 🐹 GO (бизнес-логика)

| # | Файл | Описание | Когда нужен |
|---|------|----------|-------------|
| 15 | `go/go-general.skill.md` | Общие правила Go | Любой Go код |
| 16 | `go/go-fiber-handlers.skill.md` | HTTP handlers на Fiber | REST API на Go |
| 17 | `go/go-grpc-services.skill.md` | gRPC сервисы | gRPC на Go |
| 18 | `go/go-database.skill.md` | GORM/sqlc | Работа с БД из Go |
| 19 | `go/go-error-handling.skill.md` | Обработка ошибок | Любой Go код |
| 20 | `go/go-testing.skill.md` | Тесты | Тесты Go |
| 21 | `go/go-middleware.skill.md` | Middleware (auth, logging) | Auth, logging, etc |

---

### 🐍 PYTHON (ML/Analytics)

| # | Файл | Описание | Когда нужен |
|---|------|----------|-------------|
| 22 | `python/python-ml-pipeline.skill.md` | ML пайплайны (XGBoost, LightGBM) | ML задачи |
| 23 | `python/python-data-processing.skill.md` | Обработка данных (Pandas, Polars) | Data задачи |
| 24 | `python/python-fastapi-internal.skill.md` | FastAPI для внутренних ML-сервисов | FastAPI задачи |

---

### ⚛️ FRONTEND

| # | Файл | Описание | Когда нужен |
|---|------|----------|-------------|
| 24 | `frontend/frontend-design.skill.md` | Frontend design excellence (Anthropic) | UI/UX дизайн |
| 25 | `frontend/nextjs-general.skill.md` | Общие правила Next.js 14 | Веб-разработка |
| 26 | `frontend/nextjs-components.skill.md` | Компоненты Next.js | UI компоненты |
| 27 | `frontend/nextjs-state-management.skill.md` | State management (Zustand, TanStack) | Управление состоянием |
| 28 | `frontend/nextjs-api-integration.skill.md` | Интеграция с API | Работа с API |
| 29 | `frontend/flutter-general.skill.md` | Общие правила Flutter | Мобильная разработка |
| 30 | `frontend/flutter-architecture.skill.md` | Архитектура Flutter приложения | Структура моб. приложения |
| 31 | `frontend/react-admin-panel.skill.md` | React Admin Panel (Ant Design) | Админ-панель |
| 32 | `frontend/typescript-shared.skill.md` | Общие TypeScript паттерны | Любой TS код |

---

### 🗄️ DATA

| # | Файл | Описание | Когда нужен |
|---|------|----------|-------------|
| 33 | `data/postgresql-schemas.skill.md` | PostgreSQL схемы и миграции | Проектирование БД |
| 34 | `data/postgresql-queries.skill.md` | PostgreSQL запросы и оптимизация | Написание запросов |
| 35 | `data/clickhouse-analytics.skill.md` | ClickHouse для аналитики | OLAP/аналитика |
| 36 | `data/dragonflydb-caching.skill.md` | DragonflyDB кэширование | Кэш/сессии |
| 37 | `data/redpanda-events.skill.md` | Redpanda брокер сообщений | Event-driven задачи |

---

### 🏭 INFRASTRUCTURE

| # | Файл | Описание | Когда нужен |
|---|------|----------|-------------|
| 38 | `infrastructure/terraform-iac.skill.md` | Terraform IaC | Инфраструктура как код |
| 39 | `infrastructure/kubernetes-manifests.skill.md` | Kubernetes манифесты | K8s деплой |
| 40 | `infrastructure/dockerfile-best-practices.skill.md` | Best practices Dockerfile | Контейнеризация |
| 41 | `infrastructure/helm-charts.skill.md` | Helm Charts | Пакетирование K8s |
| 42 | `infrastructure/github-actions-ci.skill.md` | GitHub Actions CI/CD | CI/CD пайплайны |
| 43 | `infrastructure/istio-service-mesh.skill.md` | Istio Service Mesh | Service mesh |

---

### 🔒 SECURITY

| # | Файл | Описание | Когда нужен |
|---|------|----------|-------------|
| 44 | `security/security-general.skill.md` | Общие правила безопасности | Всё с security |
| 45 | `security/authentication-patterns.skill.md` | Аутентификация (JWT, OAuth) | Auth задачи |
| 46 | `security/encryption-patterns.skill.md` | Шифрование (AES, TLS) | Шифрование данных |
| 47 | `security/input-validation.skill.md` | Валидация ввода | Обработка пользовательского ввода |

---

### 📊 OBSERVABILITY

| # | Файл | Описание | Когда нужен |
|---|------|----------|-------------|
| 48 | `observability/logging-standards.skill.md` | Стандарты логирования (Vector) | Настройка логов |
| 49 | `observability/metrics-instrumentation.skill.md` | Метрики (VictoriaMetrics) | Инструментация метрик |
| 50 | `observability/tracing-opentelemetry.skill.md` | Трейсинг (OpenTelemetry, Jaeger) | Distributed tracing |
| 51 | `observability/alerting-rules.skill.md` | Правила алертинга (Grafana) | Настройка алертов |

---

### 🎰 DOMAIN-SPECIFIC (гемблинг)

| # | Файл | Описание | Когда нужен |
|---|------|----------|-------------|
| 52 | `domain-specific/betting-engine-logic.skill.md` | Логика движка ставок | Ставки, матчинг |
| 53 | `domain-specific/wallet-financial-ops.skill.md` | Финансовые операции кошелька | Депозиты, выводы, балансы |
| 54 | `domain-specific/casino-integration.skill.md` | Интеграция казино провайдеров | Подключение игр |
| 55 | `domain-specific/bonus-system-logic.skill.md` | Бонусная система | Бонусы, вейджер, промо |
| 56 | `domain-specific/kyc-aml-compliance.skill.md` | KYC/AML верификация (Sumsub) | Верификация, комплаенс |
| 57 | `domain-specific/responsible-gambling.skill.md` | Ответственная игра | Лимиты, самоисключение |
| 58 | `domain-specific/odds-calculation.skill.md` | Расчёт коэффициентов | Коэффициенты, маржа |
| 59 | `domain-specific/fraud-detection.skill.md` | Детекция мошенничества | Антифрод, risk scoring |

---

### 📋 PROTOBUF

| # | Файл | Описание | Когда нужен |
|---|------|----------|-------------|
| 60 | `protobuf/protobuf-style-guide.skill.md` | Стандарты Protobuf/gRPC | Любые proto файлы |
