# 🎰 Opus Casino — High-Performance Gambling Platform

[![License: Proprietary](https://img.shields.io/badge/License-Proprietary-red.svg)](LICENSE)
[![Node.js](https://img.shields.io/badge/Node.js-20+-green.svg)](https://nodejs.org/)
[![Rust](https://img.shields.io/badge/Rust-1.75+-orange.svg)](https://www.rust-lang.org/)
[![Go](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org/)
[![Python](https://img.shields.io/badge/Python-3.11+-yellow.svg)](https://www.python.org/)

**Масштабируемая гемблинг-платформа для 10M+ пользователей**

---

## 📋 О Проекте

Opus Casino — это высокопроизводительная микросервисная платформа для онлайн-гемблинга, способная обрабатывать более **500K ежедневных активных пользователей** с p99 latency < 10ms на критическом пути.

### Ключевые Характеристики

- 🎯 **10M+ зарегистрированных пользователей**
- ⚡ **500K+ DAU** (Daily Active Users)
- 🚀 **p99 < 10ms** для критических операций
- 💰 **Real-money transactions** с двойной бухгалтерией
- 🛡️ **99.99% uptime** SLA
- 🌍 **Multi-region** deployment (EU, Asia)

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

### Распределение Кода

| Язык   | % Кода | % Нагрузки | Сервисы                          |
|--------|--------|------------|----------------------------------|
| Rust   | 35%    | 80%        | Betting Engine, Wallet, WebSocket |
| Go     | 50%    | 18%        | Auth, User, Payment, Bonus, Casino |
| Python | 15%    | 2%         | Fraud ML, Analytics              |

---

## 📦 Структура Монорепозитория

```
opus-casino/
├── apps/                        # Приложения
│   ├── web/                     # Next.js 14 — веб-платформа
│   ├── mobile/                  # Flutter — iOS + Android
│   └── admin/                   # React — админ-панель
├── services/                    # Микросервисы
│   ├── rust/                    # Rust (критический путь)
│   │   ├── betting-engine/      # Движок ставок
│   │   ├── wallet-core/         # Управление кошельками
│   │   └── websocket-gateway/   # Real-time gateway
│   ├── go/                      # Go (бизнес-логика)
│   │   ├── auth/                # Аутентификация
│   │   ├── user/                # Профиль пользователя
│   │   ├── payment/             # Платежи
│   │   ├── bonus/               # Бонусная система
│   │   ├── casino/              # Оркестрация казино
│   │   ├── notification/        # Уведомления
│   │   └── kyc/                 # KYC/AML
│   └── python/                  # Python (ML/Analytics)
│       ├── fraud-ml/            # ML детекция мошенничества
│       └── analytics/           # Аналитические пайплайны
├── libs/                        # Общие библиотеки
│   ├── proto/                   # Protobuf контракты (gRPC)
│   ├── shared/                  # Общий код
│   └── migrations/              # DB миграции
├── infra/                       # Инфраструктура
│   ├── k8s/                     # Kubernetes манифесты
│   ├── terraform/               # Terraform модули
│   ├── helm/                    # Helm charts
│   ├── argocd/                  # ArgoCD конфигурация
│   └── docker/                  # Dockerfile'ы
├── tools/                       # Инструменты
│   ├── ci/                      # CI скрипты
│   └── testing/                 # k6, chaos тесты
├── docs/                        # Документация
│   ├── api/                     # API документация
│   ├── infra/                   # Инфраструктура
│   ├── compliance/              # Compliance (AML, KYC, GDPR)
│   └── stages/                  # Этапы разработки
├── .github/                     # GitHub workflows
│   ├── workflows/               # CI/CD пайплайны
│   └── actions/                 # Reusable actions
└── .qwen/                       # AI Agent skills
    └── skills/                  # 60 skill файлов
```

---

## 🛠 Технологический Стек

### Backend

| Зона               | Язык     | Фреймворк           |
|--------------------|----------|---------------------|
| Критический путь   | Rust     | Axum + Tokio        |
| Бизнес-логика      | Go       | Fiber/Echo          |
| ML/Аналитика       | Python   | FastAPI + XGBoost   |

### Frontend

| Платформа | Технология              |
|-----------|-------------------------|
| Web       | Next.js 14 + TypeScript |
| Mobile    | Flutter (Dart)          |
| Admin     | React + Ant Design 5    |

### Базы Данных

| БД                    | Назначение                  |
|-----------------------|-----------------------------|
| PostgreSQL 16 + Citus | Основная OLTP               |
| DragonflyDB           | Кэш + сессии                |
| ClickHouse            | Аналитика OLAP              |
| Redpanda              | Брокер сообщений            |
| S3/MinIO              | Объектное хранилище         |

### Инфраструктура

- **Оркестрация:** Kubernetes (EKS/GKE) + Istio
- **CI/CD:** GitHub Actions + ArgoCD + Argo Rollouts
- **IaC:** Terraform
- **Observability:** VictoriaMetrics + Grafana + Vector + Jaeger
- **Security:** HashiCorp Vault + CloudFlare WAF

---

## 🚀 Быстрый Старт

### Требования

- Node.js >= 20
- Rust >= 1.75
- Go >= 1.21
- Python >= 3.11
- Docker >= 24
- kubectl >= 1.28

### 1. Установка Зависимостей

```bash
npm install
```

### 2. Запуск Инфраструктуры

```bash
make docker-up
# или
docker-compose -f infra/docker/docker-compose.dev.yml up -d
```

### 3. Запуск Сервисов

```bash
npm run dev
# или
make dev
```

### 4. Запуск Frontend

```bash
# Web (Next.js)
cd apps/web && npm run dev
# http://localhost:3000

# Mobile (Flutter)
cd apps/mobile && flutter run

# Admin Panel (React)
cd apps/admin && npm run dev
# http://localhost:3001
```

---

## 📊 Этапы Разработки

| №   | Этап                      | Статус  |
| --- | ------------------------- | ------- |
| 1   | Инфраструктура            | ✅ 100% |
| 2   | Observability             | ✅ 100% |
| 3   | Базы данных               | ✅ 100% |
| 4   | Proto-контракты           | ✅ 100% |
| 5   | Rust Betting Engine       | ✅ 100% |
| 6   | Rust Wallet Core          | ✅ 100% |
| 7   | Rust WebSocket Gateway    | ✅ 100% |
| 8   | Go Auth Service           | ✅ 100% |
| 9   | Go User & Payment         | ✅ 100% |
| 10  | Go Casino & Notifications | ✅ 100% |
| 11  | Python Fraud ML           | ✅ 100% |
| 12  | Next.js Web               | ✅ 100% |
| 13  | Flutter Mobile            | ✅ 100% |
| 14  | React Admin Panel         | ✅ 100% |
| 15  | K8s Production            | ✅ 100% |
| 16  | Terraform Production      | ✅ 100% |
| 17  | CI/CD Production          | ✅ 100% |
| 18  | Security & Testing        | ✅ 100% |

**Общий прогресс: 18/18 (100%)** ✅

---

## 🧪 Тестирование

### Load Testing (k6)

```bash
k6 run --vus 1000 --duration 10m tools/testing/k6/scenarios/10m-users.js
```

### Chaos Engineering (Litmus)

```bash
kubectl apply -f infra/k8s/chaos/litmus-chaos.yaml
```

### Security Scanning

```bash
make security-scan
# Trivy + Semgrep
```

---

## 📚 Документация

- **[ТЗ.md](ТЗ.md)** — Полное техническое задание (3111 строк)
- **[STAGES.md](STAGES.md)** — Матрица координации 18 этапов
- **[STEK.md](stek.md)** — Детальное описание стека технологий
- **[AGENTS.md](AGENTS.md)** — AI Agent конфигурация
- **[docs/](docs/)** — Полная документация

---

## 🔐 Безопасность

- ✅ **mTLS** между всеми сервисами (Istio)
- ✅ **Vault** для управления секретами
- ✅ **WAF** (CloudFlare) для защиты от атак
- ✅ **Rate Limiting** на всех критичных endpoints
- ✅ **Audit Logging** всех действий
- ✅ **KYC/AML** compliance

---

## 📈 Мониторинг

### Grafana Dashboards

- **System Health** — CPU, Memory, Disk, Network
- **API Performance** — p50, p95, p99 latency
- **Business Metrics** — bets, deposits, active users
- **Error Rates** — error rate по сервисам

### Alerting

- **P1 (Critical)** — PagerDuty (phone + SMS)
- **P2 (High)** — PagerDuty (push)
- **P3 (Medium)** — Slack
- **P4 (Low)** — Slack

---

## 🤝 Вклад

Этот проект является частным (proprietary). Вклад возможен только для членов команды.

---

## 📄 Лицензия

**Proprietary** — все права защищены.

---

## 📞 Контакты

- **Документация:** [docs/](docs/)
- **Infrastructure:** [infra/](infra/)
- **AI Agent Skills:** [.qwen/skills/](.qwen/skills/)

---

**Made with ❤️ by Opus Casino Team**
