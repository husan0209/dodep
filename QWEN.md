# Opus Casino — Контекст проекта

## 📋 Обзор проекта

**Opus Casino** — высокопроизводительная гемблинг-платформа для 10M+ пользователей с микросервисной архитектурой.

**Ключевые характеристики:**
- **Архитектура:** Микросервисы (Rust + Go + Python)
- **Нагрузка:** 500K DAU, p99 < 10ms на критическом пути
- **Доступность:** 99.99% uptime
- **Этапы разработки:** 18 этапов (~14-18 месяцев)

---

## 🏗 Архитектура

```
┌─────────────────────────────────────────────────────────────┐
│                    TECHNOLOGY STACK MAP                       │
│                                                               │
│  ┌─── CLIENT ───┐  ┌─── EDGE ───┐  ┌──── BACKEND ────┐     │
│  │ Next.js 14   │  │ CloudFlare │  │ Rust (критич.)  │     │
│  │ Flutter      │  │ Workers    │  │ Go (бизнес)     │     │
│  │ TypeScript   │  │ WAF + CDN  │  │ Python (ML/Ana) │     │
│  └──────────────┘  └────────────┘  └─────────────────┘     │
└─────────────────────────────────────────────────────────────┘
```

### Распределение кода
- **Rust:** 35% кода — 80% нагрузки (Betting Engine, Wallet Core, WebSocket Gateway)
- **Go:** 50% кода — 18% нагрузки (Auth, User, Payment, Bonus, Casino, KYC)
- **Python:** 15% кода — 2% нагрузки (Fraud ML, Analytics)

---

## 📦 Структура монорепозитория

```
opus-casino/
├── apps/                    # Приложения
│   ├── web/                 # Next.js 14 — основная веб-платформа
│   ├── mobile/              # Flutter — iOS + Android
│   └── admin/               # React — панель администратора
├── services/                # Микросервисы
│   ├── rust/                # Rust сервисы (критический путь)
│   │   ├── betting-engine/  # Движок ставок
│   │   ├── wallet-core/     # Управление кошельками
│   │   └── websocket-gateway/ # Real-time gateway
│   ├── go/                  # Go сервисы (бизнес-логика)
│   │   ├── auth/            # Аутентификация
│   │   ├── user/            # Профиль пользователя
│   │   ├── payment/         # Платежи
│   │   ├── bonus/           # Бонусная система
│   │   ├── casino/          # Оркестрация казино
│   │   ├── notification/    # Уведомления
│   │   └── kyc/             # KYC/AML
│   └── python/              # Python сервисы (ML/Analytics)
│       ├── fraud-ml/        # ML детекция мошенничества
│       └── analytics/       # Аналитические пайплайны
├── libs/                    # Общие библиотеки
│   ├── proto/               # Protobuf контракты (gRPC)
│   ├── shared/              # Общий код
│   └── migrations/          # DB миграции
├── infra/                   # Инфраструктура
│   ├── k8s/                 # Kubernetes манифесты
│   ├── terraform/           # Terraform модули
│   └── docker/              # Dockerfile'ы
├── tools/                   # Инструменты
│   ├── ci/                  # CI скрипты
│   └── testing/             # k6, chaos тесты
└── docs/                    # Документация
```

---

## 🛠 Технологический стек

### Backend
| Зона | Язык | Фреймворк |
|------|------|-----------|
| Критический путь | Rust | Axum + Tokio |
| Бизнес-логика | Go | Fiber/Echo |
| ML/Аналитика | Python | FastAPI + XGBoost |

### Frontend
| Платформа | Технология |
|-----------|-----------|
| Web | Next.js 14 + TypeScript |
| Mobile | Flutter (Dart) |
| Admin | React + Ant Design |

### Базы данных
| БД | Назначение |
|----|-----------|
| PostgreSQL 16 + Citus | Основная OLTP |
| DragonflyDB | Кэш + сессии |
| ClickHouse | Аналитика OLAP |
| Redpanda | Брокер сообщений |
| S3/MinIO | Объектное хранилище |

### Инфраструктура
- **Оркестрация:** Kubernetes (EKS/GKE) + Istio
- **CI/CD:** GitHub Actions + ArgoCD + Argo Rollouts
- **IaC:** Terraform
- **Observability:** VictoriaMetrics + Grafana + Vector + Jaeger
- **Security:** HashiCorp Vault + CloudFlare WAF

---

## 🚀 Building and Running

### Требования
- Node.js >= 20
- Rust >= 1.75
- Go >= 1.21
- Python >= 3.11
- Docker >= 24
- kubectl >= 1.28

### Установка зависимостей
```bash
npm install
```

### Запуск всех сервисов (dev)
```bash
npm run dev
# или
make dev
```

### Сборка всех проектов
```bash
npm run build
# или
make build
```

### Запуск тестов
```bash
npm run test
# или
make test
```

### Линтинг и форматирование
```bash
npm run lint
npm run format
# или
make lint
make format
```

### Docker Compose (локальная инфраструктура)
```bash
make docker-up    # Запуск
make docker-down  # Остановка
```

### Граф зависимостей
```bash
npm run graph
```

### Очистка
```bash
make clean
```

---

## 📊 Этапы разработки (18 этапов)

| Этап | Компонент | Статус |
|------|-----------|--------|
| 1 | Инфраструктура (Terraform, K8s, Istio, CI/CD, Vault, CloudFlare) | ⏳ |
| 2 | Observability (VictoriaMetrics, Grafana, Vector, Jaeger, Sentry) | ⏳ |
| 3 | Базы данных (PostgreSQL+Citus, DragonflyDB, ClickHouse, Redpanda) | ⏳ |
| 4 | Shared Libraries и Proto-контракты | ⏳ |
| 5 | Rust Betting Engine | ⏳ |
| 6 | Rust Wallet Core | ⏳ |
| 7 | Rust WebSocket Gateway | ⏳ |
| 8 | Go Auth Service | ⏳ |
| 9 | Go User & Payment | ⏳ |
| 10 | Go Casino & Notifications | ⏳ |
| 11 | Python Fraud ML & Analytics | ⏳ |
| 12 | Next.js 14 Web Platform | ⏳ |
| 13 | Flutter Mobile App | ⏳ |
| 14 | React Admin Panel | ⏳ |
| 15 | Kubernetes & Istio (production) | ⏳ |
| 16 | Terraform & Cloud | ⏳ |
| 17 | CI/CD Pipeline (production) | ⏳ |
| 18 | Security & Testing | ⏳ |

---

## 🔑 Ключевые файлы конфигурации

| Файл | Описание |
|------|----------|
| `package.json` | Корневой package.json с nx скриптами |
| `nx.json` | Конфигурация Nx monorepo |
| `Makefile` | Make команды для разработки |
| `.env.example` | Шаблон переменных окружения |
| `ТЗ.md` | Полное техническое задание (3111 строк) |
| `stek.md` | Детальное описание стека технологий |
| `skills.md` | Конфигурация AI-агентов (12 профилей) |

---

## 📝 Development Conventions

### Код-стайл
- **Rust:** clippy, rustfmt
- **Go:** golangci-lint, gofmt
- **Python:** ruff, black
- **TypeScript:** eslint, prettier

### Тестирование
- Unit-тесты для всех сервисов
- Integration-тесты с testcontainers
- Нагрузочное тестирование с k6
- Chaos-тестирование с Litmus

### Безопасность
- Сканирование уязвимостей: Trivy, Semgrep
- SAST: Semgrep
- DAST: OWASP ZAP
- mTLS между всеми сервисами (Istio)

### Ветка
- `main` — основная ветка
- Feature-ветки: `feature/<name>`
- Protection rules настроены в GitHub

---

## 📚 Дополнительная документация

- **Полное ТЗ:** `ТЗ.md` (3111 строк)
- **Стек технологий:** `stek.md`
- **AI Agent Skills:** `skills.md` + `.qwen/skills/`
- **README:** `README.md`

---

## 🔐 Переменные окружения

Скопируйте `.env.example` в `.env` и заполните значения:

```bash
# Основные
APP_ENV=development
APP_DEBUG=true
APP_PORT=8080

# PostgreSQL
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_DB=opus_casino
POSTGRES_USER=postgres
POSTGRES_PASSWORD=changeme

# Redis/Dragonfly
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=changeme

# JWT
JWT_SECRET_KEY=changeme-use-32-random-bytes
JWT_ACCESS_EXPIRY=900
JWT_REFRESH_EXPIRY=604800
```

---

## 📞 Контакты и ресурсы

- **Лицензия:** Proprietary — все права защищены
- **Документация:** `docs/`
- **Инфраструктура:** `infra/`
