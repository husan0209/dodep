# Architecture Decision Record

## ADR-001: Выбор микросервисной архитектуры

**Статус:** Принят  
**Дата:** 2026-03-24  
**Автор:** Platform Team  
**Стейкхолдеры:** CTO, Tech Leads, DevOps Team

---

## Контекст

Требуется создать гемблинг-платформу для 10M+ пользователей со следующими требованиями:

### Бизнес-требования
- **500K DAU** (Daily Active Users)
- **p99 < 10ms** на критическом пути (placement ставки)
- **99.99% uptime** (максимум 52 минуты простоя в год)
- Поддержка multiple jurisdictions (разные лицензии)
- Масштабирование по разным бизнес-доменам

### Технические требования
- Горизонтальное масштабирование
- Изоляция отказов (failure isolation)
- Независимые деплои сервисов
- Поддержка разных технологий для разных задач

### Ограничения
- Команда 40-55 человек к запуску
- Бюджет на инфраструктуру $50K-100K/месяц в production
- Compliance: GDPR, gambling licenses (UKGC, MGA, Curacao)

---

## Решение

Выбрана **микросервисная архитектура** с разделением по бизнес-доменам и технологическому стеку.

### Архитектурное разделение

```
┌─────────────────────────────────────────────────────────────────┐
│                        CLIENT LAYER                              │
│                                                                  │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │   Next.js 14    │  │    Flutter      │  │   React Admin   │ │
│  │   (Web App)     │  │   (Mobile)      │  │    (Dashboard)  │ │
│  │   TypeScript    │  │     Dart        │  │   TypeScript    │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                         EDGE LAYER                               │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │              CloudFlare                                   │  │
│  │  • DDoS Protection (L3/L4/L7)                             │  │
│  │  • WAF (OWASP Top-10, custom rules)                       │  │
│  │  • CDN (static assets, edge caching)                      │  │
│  │  • Rate Limiting (per IP, per user)                       │  │
│  │  • Bot Management                                          │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      BACKEND LAYER                               │
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │    RUST      │  │     GO       │  │   PYTHON     │         │
│  │  (Critical)  │  │  (Business)  │  │    (ML)      │         │
│  │              │  │              │  │              │         │
│  │ • Betting    │  │ • Auth       │  │ • Fraud      │         │
│  │ • Wallet     │  │ • User       │  │ • Analytics  │         │
│  │ • WebSocket  │  │ • Payment    │  │              │         │
│  │ • Odds       │  │ • Bonus      │  │              │         │
│  │              │  │ • Casino     │  │              │         │
│  │              │  │ • KYC        │  │              │         │
│  └──────────────┘  └──────────────┘  └──────────────┘         │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                   EVENT STREAMING                         │  │
│  │                   Redpanda (Kafka)                        │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                       DATA LAYER                                 │
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │  PostgreSQL  │  │  DragonflyDB │  │  ClickHouse  │         │
│  │  + Citus     │  │  (Redis API) │  │              │         │
│  │              │  │              │  │              │         │
│  │ • Users      │  │ • Sessions   │  │ • Analytics  │         │
│  │ • Wallets    │  │ • Cache      │  │ • Reports    │         │
│  │ • Bets       │  │ • Rate Limit │  │ • ML Data    │         │
│  │ • Trans.     │  │ • Pub/Sub    │  │ • Logs       │         │
│  └──────────────┘  └──────────────┘  └──────────────┘         │
└─────────────────────────────────────────────────────────────────┘
```

### Сервисы и их ответственность

| Сервис | Язык | Ответственность | Критичность |
|--------|------|-----------------|-------------|
| Betting Engine | Rust | Размещение, расчёт, валидация ставок | 🔴 Critical |
| Wallet Core | Rust | Балансы, транзакции, financial ops | 🔴 Critical |
| WebSocket Gateway | Rust | Real-time соединения, odds push | 🔴 Critical |
| Auth Service | Go | Аутентификация, авторизация, JWT | 🟡 High |
| User Service | Go | Профили пользователей, preferences | 🟡 High |
| Payment Service | Go | Депозиты, выводы, платежные провайдеры | 🟡 High |
| Bonus Service | Go | Бонусы, фриспины, wagering | 🟢 Medium |
| Casino Service | Go | Интеграция с game providers | 🟢 Medium |
| Notification Service | Go | Email, SMS, push, in-app | 🟢 Medium |
| KYC Service | Go | Верификация, AML проверки | 🟡 High |
| Fraud ML | Python | Детекция мошенничества, scoring | 🟡 High |
| Analytics | Python | Отчёты, пайплайны данных | 🟢 Medium |

### Коммуникация между сервисами

```
┌─────────────────────────────────────────────────────────────┐
│                    COMMUNICATION PATTERNS                    │
│                                                              │
│  Sync (HTTP/gRPC):                                          │
│  ┌──────────┐    gRPC    ┌──────────┐                       │
│  │  Client  │ ──────────▶ │  Service │                       │
│  └──────────┘            └──────────┘                       │
│                                                              │
│  Async (Events):                                             │
│  ┌──────────┐   Redpanda   ┌──────────┐                     │
│  │ Producer │ ────────────▶ │ Consumer │                     │
│  └──────────┘              └──────────┘                     │
│                                                              │
│  Real-time (WebSocket):                                      │
│  ┌──────────┐  WebSocket   ┌──────────┐                     │
│  │  Client  │ ◀──────────▶ │ Gateway  │                     │
│  └──────────┘              └──────────┘                     │
└─────────────────────────────────────────────────────────────┘
```

---

## Альтернативы

### Альтернатива 1: Монолит

**Описание:** Единое приложение со всеми модулями.

**Преимущества:**
- ✅ Проще в разработке на старте
- ✅ Нет distributed system complexity
- ✅ Легче деплой (один артефакт)
- ✅ Нет network latency между модулями

**Недостатки:**
- ❌ Единая точка отказа
- ❌ Сложнее горизонтальное масштабирование
- ❌ Все сервисы масштабируются вместе (неэффективно)
- ❌ Сложнее обновлять (риск regression)
- ❌ Требует одинаковый tech stack для всех модулей
- ❌ Не подходит для команды 40+ человек

**Почему не выбрали:**
Невозможно достичь 99.99% uptime и независимого масштабирования.

---

### Альтернатива 2: Модульный монолит

**Описание:** Монолит с чёткой модульной границей.

**Преимущества:**
- ✅ Лучшая модульность чем у монолита
- ✅ In-process коммуникация (быстрее)
- ✅ Проще транзакции
- ✅ Можно выделить модули в сервисы позже

**Недостатки:**
- ❌ Всё ещё единый деплой
- ❌ Сложнее масштабировать команды
- ❌ Риск coupling между модулями
- ❌ Требует дисциплины от разработчиков

**Почему не выбрали:**
Не даёт преимуществ для параллельной разработки multiple teams.

---

### Альтернатива 3: Event Sourcing + CQRS

**Описание:** Полное event sourcing для всех доменов.

**Преимущества:**
- ✅ Полный audit log из коробки
- ✅ Temporal queries (состояние на любой момент)
- ✅ Отличная производительность чтения

**Недостатки:**
- ❌ Значительная сложность реализации
- ❌ Eventual consistency для всех операций
- ❌ Сложнее отладка
- ❌ Требует другую ментальную модель

**Почему не выбрали:**
Избыточно для большинства use cases. Применим выборочно (Wallet Core).

---

## Последствия

### Положительные

1. **Независимое масштабирование**
   - Betting Engine можно масштабировать отдельно от Bonus Service
   - Экономия 30-40% на инфраструктуре

2. **Технологическая гибкость**
   - Rust для performance-critical paths
   - Go для business logic (быстрая разработка)
   - Python для ML (экосистема)

3. **Изоляция отказов**
   - Падение Bonus Service не влияет на ставки
   - Circuit breaker предотвращает cascade failures

4. **Параллельная разработка**
   - 6-8 команд могут работать независимо
   - Меньше merge conflicts
   - Быстрее time to market

5. **Независимые деплои**
   - Можно деплоить Auth Service 10 раз в день
   - Не затрагивая другие сервисы

---

### Отрицательные

1. **Distributed system complexity**
   - Нужен service mesh (Istio)
   - Distributed tracing обязательен
   - Сложнее отладка проблем

2. **Data consistency**
   - Нет distributed transactions
   - Нужен Saga pattern для cross-service ops
   - Eventual consistency для некоторых операций

3. **Операционные расходы**
   - Нужно поддерживать 12+ сервисов
   - Больше Kubernetes workload'ов
   - Сложнее monitoring и alerting

4. **Network overhead**
   - gRPC вызовы добавляют latency
   - Нужно кэшировать агрессивнее
   - Batch запросы где возможно

5. **Компетенции команды**
   - Нужно знать 3+ языка
   - Разные best practices
   - Сложнее онбординг

---

## Compliance последствия

### GDPR

- ✅ Данные пользователей в PostgreSQL (EU region)
- ✅ Право на удаление (User Service)
- ✅ Audit log всех операций
- ⚠️ Cross-service data требует внимания

### Gambling Licenses (UKGC, MGA)

- ✅ Изоляция critical paths (Betting, Wallet)
- ✅ Audit log всех ставок
- ✅ Responsible gambling (отдельный сервис)
- ✅ KYC/AML compliance (KYC Service)

### PCI DSS (платежи)

- ✅ Payment Service изолирован
- ✅ Нет хранения PAN (tokenization)
- ✅ mTLS между сервисами
- ✅ Vault для secrets management

---

## Метрики успеха

| Метрика | Цель | Как измеряем |
|---------|------|--------------|
| p99 latency (critical) | < 10ms | VictoriaMetrics |
| Uptime | 99.99% | Grafana SLO |
| Deployment frequency | 10+/day | ArgoCD |
| Change failure rate | < 5% | GitHub + ArgoCD |
| Mean time to recovery | < 5 min | PagerDuty |

---

## Ссылки

- [Microservices Patterns (Chris Richardson)](https://microservices.io/patterns/)
- [Building Microservices (Sam Newman)](https://www.oreilly.com/library/view/building-microservices/9781491950340/)
- [Istio Documentation](https://istio.io/latest/docs/)
- [ADR Template](https://github.com/joelparkerhenderson/architecture-decision-record)

---

## История изменений

| Дата | Изменение | Автор |
|------|-----------|-------|
| 2026-03-24 | Initial draft | Platform Team |
| 2026-03-24 | Review & approve | CTO |
