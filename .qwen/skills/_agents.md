# Agent Role Profiles — какие skills загружать для какой задачи

## КАК ИСПОЛЬЗОВАТЬ

При начале работы скажи агенту:
> «Ты работаешь как [РОЛЬ]. Загрузи skills из профиля [ПРОФИЛЬ].»

Или в системном промпте:
> «Load skills from profile: RUST_CORE_ENGINEER»

## УРОВНИ ЗАГРУЗКИ

| Уровень | Описание | Когда загружать |
|---------|----------|----------------|
| 🔵 **Основные** | Проектные скиллы роли | ВСЕГДА |
| 🟢 **По задаче** | Доменные скиллы | По типу конкретной задачи |
| 🟡 **Справочные** | Cross-cutting методологии | При дебаге, TDD, финансовой логике |

---

## PROFILE: RUST_CORE_ENGINEER

**Задачи:** Betting Engine, Wallet Core, Risk Engine, Odds Calculator

**Загрузить (8 files):**
- `architecture/architecture-overview.skill.md`
- `rust/rust-general.skill.md`
- `rust/rust-axum-handlers.skill.md`
- `rust/rust-sqlx-database.skill.md`
- `rust/rust-error-handling.skill.md`
- `rust/rust-testing.skill.md`
- `security/security-general.skill.md`
- `protobuf/protobuf-style-guide.skill.md`

**Дополнительно по задаче:**
- Betting Engine → + `domain-specific/betting-engine-logic.skill.md` + `domain-specific/odds-calculation.skill.md`
- Wallet Core → + `domain-specific/wallet-financial-ops.skill.md` + `architecture/data-consistency.skill.md`
- Risk/Fraud → + `domain-specific/fraud-detection.skill.md` + `rust/rust-performance.skill.md`

**Справочные (при необходимости):**
- При дебаге → `systematic-debugging/SKILL.md`
- При TDD → `test-driven-development/SKILL.md`
- При финансовой логике → `domain-fintech/SKILL.md`

---

## PROFILE: RUST_WEBSOCKET_ENGINEER

**Задачи:** WebSocket Gateway, real-time odds push

**Загрузить (7 files):**
- `architecture/architecture-overview.skill.md`
- `rust/rust-general.skill.md`
- `rust/rust-websocket.skill.md`
- `rust/rust-tokio-async.skill.md`
- `rust/rust-performance.skill.md`
- `rust/rust-error-handling.skill.md`
- `data/redpanda-events.skill.md`

**Справочные (при необходимости):**
- При дебаге → `systematic-debugging/SKILL.md`
- При финансовой логике → `domain-fintech/SKILL.md`

---

## PROFILE: GO_BUSINESS_ENGINEER

**Задачи:** Auth, User, Payment, Bonus, Casino Orchestration, Notifications, KYC/AML, Responsible Gambling

**Загрузить (8 files):**
- `architecture/architecture-overview.skill.md`
- `go/go-general.skill.md`
- `go/go-fiber-handlers.skill.md`
- `go/go-grpc-services.skill.md`
- `go/go-database.skill.md`
- `go/go-error-handling.skill.md`
- `go/go-testing.skill.md`
- `security/security-general.skill.md`

**Дополнительно по задаче:**
- Auth Service → + `security/authentication-patterns.skill.md` + `security/encryption-patterns.skill.md`
- Payment → + `domain-specific/wallet-financial-ops.skill.md` + `architecture/data-consistency.skill.md`
- KYC → + `domain-specific/kyc-aml-compliance.skill.md` + `security/encryption-patterns.skill.md`
- Bonus → + `domain-specific/bonus-system-logic.skill.md`
- Casino → + `domain-specific/casino-integration.skill.md`
- Notifications → + `data/redpanda-events.skill.md`

**Справочные (при необходимости):**
- При дебаге → `systematic-debugging/SKILL.md`
- При TDD → `test-driven-development/SKILL.md`
- При финансовой логике → `domain-fintech/SKILL.md`

---

## PROFILE: FRONTEND_WEB_ENGINEER

**Задачи:** Next.js 14 web application

**Загрузить (7 files):**
- `architecture/architecture-overview.skill.md`
- `frontend/nextjs-general.skill.md`
- `frontend/nextjs-components.skill.md`
- `frontend/nextjs-state-management.skill.md`
- `frontend/nextjs-api-integration.skill.md`
- `frontend/typescript-shared.skill.md`
- `security/security-general.skill.md`

**Дополнительно по задаче:**
- Sports betting UI → + `domain-specific/betting-engine-logic.skill.md`
- Casino UI → + `domain-specific/casino-integration.skill.md`
- Wallet UI → + `domain-specific/wallet-financial-ops.skill.md`

**Справочные (при необходимости):**
- При дебаге → `systematic-debugging/SKILL.md`
- При TDD → `test-driven-development/SKILL.md`

---

## PROFILE: FLUTTER_MOBILE_ENGINEER

**Задачи:** Flutter mobile app (iOS + Android)

**Загрузить (5 files):**
- `architecture/architecture-overview.skill.md`
- `frontend/flutter-general.skill.md`
- `frontend/flutter-architecture.skill.md`
- `frontend/typescript-shared.skill.md` *(для понимания API types)*
- `security/security-general.skill.md`

**Справочные (при необходимости):**
- При дебаге → `systematic-debugging/SKILL.md`
- При TDD → `test-driven-development/SKILL.md`

---

## PROFILE: ADMIN_PANEL_ENGINEER

**Задачи:** React admin panel (Ant Design)

**Загрузить (5 files):**
- `architecture/architecture-overview.skill.md`
- `frontend/react-admin-panel.skill.md`
- `frontend/typescript-shared.skill.md`
- `frontend/nextjs-api-integration.skill.md` *(API client patterns)*
- `security/security-general.skill.md`

**Справочные (при необходимости):**
- При дебаге → `systematic-debugging/SKILL.md`
- При TDD → `test-driven-development/SKILL.md`

---

## PROFILE: DATA_ENGINEER

**Задачи:** SQL schemas, migrations, analytics, caching

**Загрузить (7 files):**
- `architecture/architecture-overview.skill.md`
- `data/postgresql-schemas.skill.md`
- `data/postgresql-queries.skill.md`
- `data/clickhouse-analytics.skill.md`
- `data/dragonflydb-caching.skill.md`
- `data/redpanda-events.skill.md`
- `architecture/data-consistency.skill.md`

**Справочные (при необходимости):**
- При дебаге → `systematic-debugging/SKILL.md`
- При финансовой логике → `domain-fintech/SKILL.md`

---

## PROFILE: DEVOPS_SRE_ENGINEER

**Задачи:** Infrastructure, CI/CD, K8s, monitoring

**Загрузить (9 files):**
- `architecture/architecture-overview.skill.md`
- `infrastructure/terraform-iac.skill.md`
- `infrastructure/kubernetes-manifests.skill.md`
- `infrastructure/dockerfile-best-practices.skill.md`
- `infrastructure/helm-charts.skill.md`
- `infrastructure/github-actions-ci.skill.md`
- `infrastructure/istio-service-mesh.skill.md`
- `observability/logging-standards.skill.md`
- `observability/alerting-rules.skill.md`

**Справочные (при необходимости):**
- При дебаге → `systematic-debugging/SKILL.md`

---

## PROFILE: SECURITY_ENGINEER

**Задачи:** Security audit, auth, encryption, compliance

**Загрузить (7 files):**
- `architecture/architecture-overview.skill.md`
- `security/security-general.skill.md`
- `security/authentication-patterns.skill.md`
- `security/encryption-patterns.skill.md`
- `security/input-validation.skill.md`
- `domain-specific/kyc-aml-compliance.skill.md`
- `domain-specific/responsible-gambling.skill.md`

**Справочные (при необходимости):**
- При дебаге → `systematic-debugging/SKILL.md`
- При финансовой логике → `domain-fintech/SKILL.md`

---

## PROFILE: ML_FRAUD_ENGINEER

**Задачи:** Fraud detection ML, analytics pipelines

**Загрузить (6 files):**
- `architecture/architecture-overview.skill.md`
- `python/python-ml-pipeline.skill.md`
- `python/python-data-processing.skill.md`
- `domain-specific/fraud-detection.skill.md`
- `data/clickhouse-analytics.skill.md`
- `data/redpanda-events.skill.md`

**Справочные (при необходимости):**
- При дебаге → `systematic-debugging/SKILL.md`

---

## PROFILE: OBSERVABILITY_ENGINEER

**Задачи:** Metrics, logging, tracing, alerting

**Загрузить (6 files):**
- `architecture/architecture-overview.skill.md`
- `observability/logging-standards.skill.md`
- `observability/metrics-instrumentation.skill.md`
- `observability/tracing-opentelemetry.skill.md`
- `observability/alerting-rules.skill.md`
- `infrastructure/kubernetes-manifests.skill.md`

**Справочные (при необходимости):**
- При дебаге → `systematic-debugging/SKILL.md`

---

## PROFILE: PROTOBUF_CONTRACTS

**Задачи:** Определение gRPC контрактов между сервисами

**Загрузить (4 files):**
- `architecture/architecture-overview.skill.md`
- `protobuf/protobuf-style-guide.skill.md`
- `architecture/api-design-guidelines.skill.md`
- `architecture/event-driven-design.skill.md`

**Справочные (при необходимости):**
- При дебаге → `systematic-debugging/SKILL.md`
