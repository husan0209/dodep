# AGENTS.md — AI Agent Configuration
# Gambling Platform — 10M+ Users

## Быстрый старт для AI-агента

### ⛔ Шаг 0: Прочитай CONVENTIONS.md
**ОБЯЗАТЕЛЬНО** прочитай `CONVENTIONS.md` перед любой работой.
Там описаны запрещённые действия, структура проекта, namespaces.
**Нарушение = код не принимается.**

### Шаг 1: Определи свою роль перед тем как начать работать
Прочитай `.qwen/skills/_agents.md` и найди подходящий профиль.

### Шаг 2: Загрузи skills
Загрузи ТОЛЬКО файлы, перечисленные в твоём профиле.
**НЕ загружай все 60 файлов** — они не поместятся в контекст.

### Шаг 3: Начни работу
Следуй правилам из загруженных skills и CONVENTIONS.md.

---

## Доступные профили

| Профиль | Задачи | Кол-во skills |
|---------|--------|---------------|
| `RUST_CORE_ENGINEER` | Betting Engine, Wallet Core, Risk Engine, Odds Calculator | 8 + доп. |
| `RUST_WEBSOCKET_ENGINEER` | WebSocket Gateway, real-time odds push | 7 |
| `GO_BUSINESS_ENGINEER` | Auth, User, Payment, Bonus, Casino, KYC | 8 + доп. |
| `FRONTEND_WEB_ENGINEER` | Next.js 14 web application | 7 + доп. |
| `FLUTTER_MOBILE_ENGINEER` | Flutter mobile app (iOS + Android) | 5 |
| `ADMIN_PANEL_ENGINEER` | React admin panel | 5 |
| `DATA_ENGINEER` | SQL schemas, migrations, analytics, caching | 7 |
| `DEVOPS_SRE_ENGINEER` | Infrastructure, CI/CD, K8s, monitoring | 9 |
| `SECURITY_ENGINEER` | Security audit, auth, encryption, compliance | 7 |
| `ML_FRAUD_ENGINEER` | Fraud detection ML, analytics pipelines | 6 |
| `OBSERVABILITY_ENGINEER` | Metrics, logging, tracing, alerting | 6 |
| `PROTOBUF_CONTRACTS` | gRPC контракты между сервисами | 4 |

---

## Примеры промптов

### Для AI-агента (Claude, Qwen, Gemini, Kiro, etc.):
```
Я работаю как RUST_CORE_ENGINEER.
Загрузи skills из профиля RUST_CORE_ENGINEER (файл .qwen/skills/_agents.md).
Задача: реализовать endpoint PlaceBet в Betting Engine.
```

### Для Cursor (.cursorrules):
```
Read and follow instructions from:
- .qwen/skills/architecture/architecture-overview.skill.md
- .qwen/skills/rust/rust-general.skill.md
- .qwen/skills/domain-specific/betting-engine-logic.skill.md
(список из профиля)
```

---

## Правила

1. **ВСЕГДА** загружай `architecture/architecture-overview.skill.md`
2. Загружай **5-10 skills** максимум (не больше)
3. Если задача пересекает 2 домена — загрузи skills из обоих
4. `security/security-general.skill.md` — загружай **всегда** при работе с данными

---

## Структура файлов

```
.qwen/skills/
├── _index.md                    ← Полный каталог всех 60 skills
├── _agents.md                   ← Профили ролей (12 профилей)
├── architecture/                ← Архитектурные паттерны (5)
├── rust/                        ← Rust skills (9)
├── go/                          ← Go skills (7)
├── python/                      ← Python ML/Analytics (3)
├── frontend/                    ← Next.js, Flutter, React (8)
├── data/                        ← PostgreSQL, ClickHouse, etc. (5)
├── infrastructure/              ← Terraform, K8s, CI/CD (6)
├── security/                    ← Security patterns (4)
├── observability/               ← Logging, metrics, tracing (4)
├── domain-specific/             ← Гемблинг бизнес-логика (8)
└── protobuf/                    ← Protobuf стандарты (1)
```

---

## Рабочий процесс

```
┌──────────────────────────────────────────────────────────────┐
│           WORKFLOW: Разработчик + AI Agent                    │
│                                                                │
│  1. Разработчик открывает задачу                              │
│     «Реализовать withdrawal endpoint в Payment Service»       │
│                                                                │
│  2. Разработчик говорит агенту:                               │
│     «Ты GO_BUSINESS_ENGINEER. Задача — Payment Service.       │
│      Загрузи: architecture-overview, go-general,              │
│      go-fiber-handlers, go-database, go-error-handling,       │
│      wallet-financial-ops, security-general,                   │
│      input-validation»                                         │
│                                                                │
│  3. Агент читает 8 skill файлов (~3500 строк)                 │
│     и знает:                                                  │
│     ✓ Структуру проекта                                       │
│     ✓ Как писать handlers в Go                                │
│     ✓ Как работать с БД                                       │
│     ✓ Как обрабатывать финансовые операции                    │
│     ✓ Как валидировать input                                  │
│     ✓ Security правила                                        │
│                                                                │
│  4. Агент пишет код по стандартам платформы                    │
│     Не «какой-то рабочий код», а ПРАВИЛЬНЫЙ код               │
│                                                                │
│  5. Code review: 90% принимается сразу                        │
└──────────────────────────────────────────────────────────────┘
```