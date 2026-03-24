# Финальный стек технологий гемблинг-платформы 10M+ пользователей

---

## ОБЩАЯ КАРТА

```
┌─────────────────────────────────────────────────────────────┐
│                    TECHNOLOGY STACK MAP                       │
│                                                               │
│  ┌─── CLIENT ───┐  ┌─── EDGE ───┐  ┌──── BACKEND ────┐     │
│  │ Next.js 14   │  │ CloudFlare │  │ Rust (критич.)  │     │
│  │ Flutter      │  │ Workers    │  │ Go (бизнес)     │     │
│  │ TypeScript   │  │ WAF + CDN  │  │ Python (ML/Ana) │     │
│  └──────────────┘  └────────────┘  └─────────────────┘     │
│                                                               │
│  ┌─── DATA ─────────────────────────────────────────────┐   │
│  │ PostgreSQL 16 + Citus    (основная БД)               │   │
│  │ DragonflyDB              (кэш)                       │   │
│  │ ClickHouse               (аналитика)                  │   │
│  │ Redpanda                 (брокер сообщений)           │   │
│  │ S3 / MinIO               (файлы, бэкапы)             │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                               │
│  ┌─── INFRASTRUCTURE ──────────────────────────────────┐    │
│  │ Kubernetes (EKS / GKE)   (оркестрация)              │    │
│  │ Istio                    (service mesh)              │    │
│  │ Argo Rollouts            (canary deploys)            │    │
│  │ Terraform                (IaC)                       │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

---

## 1. BACKEND

| Зона | Язык | Где используется | Почему |
|------|------|-----------------|--------|
| **Критический путь** | **Rust** | Betting Engine, Matching Engine, Risk Engine, Wallet Core, WebSocket Gateway, Odds Calculator, Fraud Engine (real-time), Audit Log | Нет GC, 150 MB RAM, p99 < 5ms, lock-free структуры, io_uring |
| **Бизнес-логика** | **Go** | Auth, User Profile, Payment, Bonus, Casino Orchestration, Notifications, KYC/AML, Responsible Gambling, Affiliate, CMS, Scheduler, Feature Flags | Горутины, быстрая разработка, 500 MB RAM, < 100ms |
| **ML / Аналитика** | **Python** | Fraud ML модели, аналитические пайплайны, отчёты, ретрейнинг моделей | Библиотеки ML (XGBoost, LightGBM, PyTorch), batch-обработка |

```
Распределение кода:

Rust  ████████████████░░░░░░░░░░░░░░  35% кода — 80% нагрузки
Go    ████████████████████░░░░░░░░░░  50% кода — 18% нагрузки  
Python████████░░░░░░░░░░░░░░░░░░░░░░  15% кода —  2% нагрузки
```

### Ключевые фреймворки

```yaml
Rust:
  web:        Axum (tokio-based, максимальная производительность)
  async:      Tokio (io_uring backend)
  serialization: serde + rkyv (zero-copy)
  websocket:  tokio-tungstenite
  grpc:       tonic
  orm:        SQLx (compile-time checked queries)
  
Go:
  web:        Fiber / Echo (быстрые HTTP фреймворки)
  grpc:       google.golang.org/grpc
  orm:        GORM / sqlc (type-safe SQL)
  validation: go-playground/validator
  
Python:
  ml:         XGBoost, LightGBM, scikit-learn
  data:       Pandas, Polars (быстрее Pandas)
  api:        FastAPI (только для внутренних ML-сервисов)
```

---

## 2. FRONTEND

| Платформа | Технология | Обоснование |
|-----------|-----------|-------------|
| **Web** | **Next.js 14 + TypeScript** | Server Components, SSR/SSG, SEO, edge rendering |
| **Mobile** | **Flutter (Dart)** | Одна кодобаза → iOS + Android, нативная производительность 60fps |
| **Admin Panel** | **React + TypeScript + Ant Design** | Быстрая разработка внутренних инструментов |

```yaml
Frontend Libraries:
  state:          Zustand (лёгкий) / TanStack Query (серверный стейт)
  websocket:      собственный клиент с auto-reconnect
  charts:         Recharts / Lightweight Charts (TradingView)
  i18n:           next-intl (20+ языков)
  analytics:      собственная (ClickHouse), НЕ Google Analytics
  ui:             Tailwind CSS + Radix UI (accessible)
  testing:        Playwright (e2e) + Vitest (unit)
  bundler:        Turbopack (Next.js built-in)
```

---

## 3. БАЗЫ ДАННЫХ

```
┌──────────────────────────────────────────────────────────────┐
│                     DATA ARCHITECTURE                         │
│                                                                │
│  ┌─────────────────────┐      ┌─────────────────────┐        │
│  │   PostgreSQL 16     │      │    DragonflyDB      │        │
│  │   + Citus           │      │                     │        │
│  │                     │      │  • Sessions         │        │
│  │  • Users            │      │  • Live odds cache  │        │
│  │  • Bets (ledger)    │      │  • Rate limiting    │        │
│  │  • Transactions     │      │  • Leaderboards     │        │
│  │  • KYC data         │      │  • Feature flags    │        │
│  │  • Game configs     │      │  • Idempotency keys │        │
│  │                     │      │                     │        │
│  │  Шардинг: user_id   │      │  4M ops/sec/server  │        │
│  │  ACID гарантии      │      │  < 1ms latency      │        │
│  └─────────────────────┘      └─────────────────────┘        │
│                                                                │
│  ┌─────────────────────┐      ┌─────────────────────┐        │
│  │   ClickHouse        │      │    S3 / MinIO       │        │
│  │                     │      │                     │        │
│  │  • Event analytics  │      │  • KYC documents    │        │
│  │  • Bet history      │      │  • Database backups │        │
│  │  • Fraud logs       │      │  • Static assets    │        │
│  │  • Financial reports│      │  • Audit trail      │        │
│  │  • User behavior    │      │  • Log archives     │        │
│  │                     │      │                     │        │
│  │  Миллиарды строк    │      │  Unlimited storage  │        │
│  │  Колоночное хранение│      │  99.999999999%      │        │
│  └─────────────────────┘      └─────────────────────┘        │
└──────────────────────────────────────────────────────────────┘
```

| БД | Роль | Объём | Почему не альтернатива |
|----|------|-------|----------------------|
| **PostgreSQL 16 + Citus** | Основная OLTP | ~2 TB | ACID для денег. Citus = горизонтальный шардинг без смены БД. Не MongoDB — нужна консистентность |
| **DragonflyDB** | Кэш + сессии | ~200 GB RAM | Multi-threaded = 25x Redis на одном сервере. API-совместим с Redis. Не Redis — single-threaded bottleneck |
| **ClickHouse** | Аналитика OLAP | ~50 TB | 1 млрд строк/сек сканирование. Колоночное сжатие 10:1. Не Elasticsearch — дороже и медленнее для аналитики |
| **S3 / MinIO** | Объектное хранилище | Unlimited | KYC документы, бэкапы, аудит. MinIO для on-premise |

---

## 4. КОММУНИКАЦИЯ МЕЖДУ СЕРВИСАМИ

```
┌────────────────────────────────────────────────────────┐
│              COMMUNICATION PROTOCOLS                     │
│                                                          │
│  Синхронная (запрос-ответ):                             │
│    gRPC + Protobuf    — между микросервисами            │
│    REST + JSON        — только для внешнего API          │
│    WebSocket          — real-time клиент ↔ сервер       │
│                                                          │
│  Асинхронная (события):                                 │
│    Redpanda (Kafka API) — events, commands, CDC         │
│                                                          │
│  ┌─────────┐  gRPC   ┌─────────┐  Redpanda ┌────────┐ │
│  │ Betting │ ──────▶ │  Risk   │ ────────▶ │Analyt. │ │
│  │ Engine  │ ◀────── │ Engine  │           │Service │ │
│  └─────────┘  < 2ms  └─────────┘           └────────┘ │
└────────────────────────────────────────────────────────┘
```

| Протокол | Где | Почему |
|----------|-----|--------|
| **gRPC + Protobuf** | Между всеми сервисами | В 3-5x меньше CPU на сериализацию, строгие контракты, кодогенерация |
| **REST + JSON** | Внешний API для клиентов | Универсальность, простота для фронтенда |
| **WebSocket** | Live odds, live casino, чат | Bidirectional, low-latency push |
| **Redpanda** | Event bus | Kafka API без зависимости от JVM/ZooKeeper, в 5x меньше серверов |

---

## 5. ИНФРАСТРУКТУРА

```yaml
# Полный инфраструктурный стек
orchestration:
  runtime:          Kubernetes (EKS или GKE)
  service_mesh:     Istio (mTLS, traffic management, observability)
  ingress:          Envoy (через Istio) + CloudFlare
  
deployment:
  ci_cd:            GitHub Actions
  gitops:           ArgoCD (декларативные деплои)
  canary:           Argo Rollouts
  iac:              Terraform + Terragrunt
  secrets:          HashiCorp Vault
  
networking:
  cdn:              CloudFlare (CDN + WAF + Workers + DDoS)
  dns:              CloudFlare DNS (30s TTL, geo-routing)
  load_balancer:    AWS ALB / GCP Cloud LB
  api_gateway:      Kong Gateway (rate limiting, auth, logging)
  
security:
  waf:              CloudFlare WAF
  certificates:     cert-manager (auto Let's Encrypt)
  mtls:             Istio (автоматически между сервисами)
  secrets_mgmt:     HashiCorp Vault
  image_scanning:   Trivy
  sast:             Semgrep
  dast:             OWASP ZAP
  
container:
  base_images:      Distroless (Google) — минимальная поверхность атаки
  registry:         AWS ECR / GCP Artifact Registry
```

---

## 6. OBSERVABILITY

```
┌─────────────────────────────────────────────────────┐
│              OBSERVABILITY STACK                      │
│                                                       │
│  Metrics:     VictoriaMetrics    (НЕ Prometheus)     │
│  Dashboards:  Grafana                                 │
│  Logging:     Vector → ClickHouse (НЕ ELK)          │
│  Tracing:     Jaeger + OpenTelemetry                 │
│  Profiling:   Pyroscope (continuous)                  │
│  Alerting:    Grafana Alerting → PagerDuty           │
│  Status Page: Statuspage.io                           │
│  Chaos:       Litmus Chaos (K8s native)              │
│  Load Test:   k6 (Grafana)                           │
│  Error Track: Sentry                                  │
└─────────────────────────────────────────────────────┘
```

| Компонент | Выбор | Вместо чего | Почему |
|-----------|-------|-------------|--------|
| **Метрики** | VictoriaMetrics | Prometheus | Меньше RAM в 7x, long-term storage, совместимый API |
| **Логи** | Vector → ClickHouse | ELK Stack | Vector на Rust (быстрый), ClickHouse дешевле Elasticsearch в 10x |
| **Трейсинг** | Jaeger + OpenTelemetry | Zipkin, Datadog | Open-source, vendor-neutral, стандарт индустрии |
| **Профилирование** | Pyroscope | - | Continuous profiling без overhead |
| **Ошибки** | Sentry | - | Real-time error tracking с контекстом |

---

## 7. БЕЗОПАСНОСТЬ

```yaml
security_stack:
  authentication:
    password_hashing:   Argon2id
    tokens:             Ed25519 signed JWT (15min) + opaque refresh (7d)
    2fa:                TOTP + WebAuthn (FIDO2) + SMS fallback
    
  encryption:
    in_transit:         TLS 1.3 (минимум)
    at_rest:            AES-256-GCM
    key_management:     HashiCorp Vault + HSM (AWS CloudHSM)
    
  api_protection:
    rate_limiting:      Token Bucket (per IP, per user, per action)
    ddos:               CloudFlare (L3-L7)
    waf:                CloudFlare WAF + custom rules
    bot_detection:      CloudFlare Bot Management
    
  compliance:
    kyc_provider:       Sumsub
    aml_screening:      ComplyAdvantage
    rng_certification:  GLI / eCOGRA
    auditor:            Big 4 (ежегодно)
```

---

## 8. ВНЕШНИЕ СЕРВИСЫ

| Функция | Сервис | Альтернатива |
|---------|--------|-------------|
| **KYC верификация** | Sumsub | Onfido, Jumio |
| **Платежи (фиат)** | Stripe + локальные PSP | Adyen |
| **Платежи (крипто)** | собственный + NOWPayments | CoinPayments |
| **Email** | Amazon SES | Postmark |
| **SMS** | Twilio | Vonage |
| **Push-уведомления** | Firebase Cloud Messaging | OneSignal |
| **Спортивные данные** | Sportradar | Betgenius, LSports |
| **Casino провайдеры** | через агрегатор (SoftSwiss, Slotegrator) | прямая интеграция |
| **Геолокация** | MaxMind GeoIP2 | IP2Location |

---

## 9. ИТОГОВАЯ СВОДНАЯ ТАБЛИЦА

```
╔═══════════════════╦════════════════════════════════════╗
║   КАТЕГОРИЯ       ║   ТЕХНОЛОГИЯ                       ║
╠═══════════════════╬════════════════════════════════════╣
║                   ║                                    ║
║   BACKEND         ║   Rust + Go + Python               ║
║   WEB FRAMEWORK   ║   Axum (Rust) / Fiber (Go)        ║
║   FRONTEND WEB    ║   Next.js 14 + TypeScript          ║
║   FRONTEND MOBILE ║   Flutter (Dart)                   ║
║   ADMIN PANEL     ║   React + Ant Design               ║
║                   ║                                    ║
║   БД ОСНОВНАЯ     ║   PostgreSQL 16 + Citus            ║
║   КЭШ             ║   DragonflyDB                      ║
║   АНАЛИТИКА       ║   ClickHouse                       ║
║   БРОКЕР          ║   Redpanda                         ║
║   ФАЙЛЫ           ║   S3 / MinIO                       ║
║                   ║                                    ║
║   API ВНЕШНЕЕ     ║   REST + JSON                      ║
║   API ВНУТРЕННЕЕ  ║   gRPC + Protobuf                  ║
║   REAL-TIME       ║   WebSocket (Rust + io_uring)      ║
║                   ║                                    ║
║   ОРКЕСТРАЦИЯ     ║   Kubernetes (EKS/GKE)             ║
║   SERVICE MESH    ║   Istio                            ║
║   CI/CD           ║   GitHub Actions + ArgoCD          ║
║   ДЕПЛОЙ          ║   Argo Rollouts (canary)           ║
║   IaC             ║   Terraform                        ║
║   SECRETS         ║   HashiCorp Vault                  ║
║                   ║                                    ║
║   CDN + WAF       ║   CloudFlare                       ║
║   API GATEWAY     ║   Kong                             ║
║                   ║                                    ║
║   МЕТРИКИ         ║   VictoriaMetrics + Grafana        ║
║   ЛОГИ            ║   Vector → ClickHouse              ║
║   ТРЕЙСИНГ        ║   Jaeger + OpenTelemetry           ║
║   ОШИБКИ          ║   Sentry                           ║
║   ПРОФИЛИРОВАНИЕ  ║   Pyroscope                        ║
║   АЛЕРТИНГ        ║   Grafana Alerting → PagerDuty     ║
║   НАГРУЗОЧНЫЕ     ║   k6                               ║
║   CHAOS           ║   Litmus Chaos                     ║
║                   ║                                    ║
║   AUTH             ║   Argon2id + JWT (Ed25519)        ║
║   ШИФРОВАНИЕ      ║   AES-256-GCM + TLS 1.3           ║
║   КЛЮЧИ           ║   Vault + HSM                      ║
║   KYC             ║   Sumsub                           ║
║   AML             ║   ComplyAdvantage                  ║
║                   ║                                    ║
║   КОНТЕЙНЕРЫ      ║   Distroless images                ║
║   СКАНИРОВАНИЕ    ║   Trivy + Semgrep + OWASP ZAP     ║
║                   ║                                    ║
╚═══════════════════╩════════════════════════════════════╝
```

---

## 10. СТОИМОСТЬ ИНФРАСТРУКТУРЫ (оценка)

```
При 10M+ пользователей, 500K DAU:

┌────────────────────────┬──────────────┐
│ Компонент              │  $/месяц     │
├────────────────────────┼──────────────┤
│ Kubernetes (40 nodes)  │  $25,000     │
│ PostgreSQL + Citus     │  $8,000      │
│ DragonflyDB (5 nodes)  │  $5,000      │
│ ClickHouse cluster     │  $4,000      │
│ Redpanda cluster       │  $3,000      │
│ CloudFlare Enterprise  │  $5,000      │
│ S3 storage             │  $2,000      │
│ Monitoring stack       │  $3,000      │
│ HashiCorp Vault        │  $2,000      │
│ External APIs          │  $5,000      │
│ Bandwidth              │  $8,000      │
│ Резерв DR (2nd region) │  $15,000     │
├────────────────────────┼──────────────┤
│ ИТОГО                  │  ~$85,000    │
└────────────────────────┴──────────────┘

Для сравнения: на Python/Node.js стеке 
это было бы $250,000-400,000/мес
Экономия Rust+Go: ~$150-300K/мес
```

Этот стек даёт **99.99% uptime**, **p99 < 10ms** на критическом пути и масштабируется горизонтально до **50M+ пользователей** без изменения архитектуры.
Надо составлять ТЗ под этого проекта сможем его делить на 18 этапов или на 12