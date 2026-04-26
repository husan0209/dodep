# ⛔ CONVENTIONS.md — ОБЯЗАТЕЛЬНЫЕ ПРАВИЛА ДЛЯ ВСЕХ AI-АГЕНТОВ
# ============================================================
# ЭТОТ ФАЙЛ — ЗАКОН. Нарушение = код не будет принят.
# Прочитай ПЕРЕД написанием первой строки кода.
# ============================================================

## 🔴 ЗАПРЕЩЁННЫЕ ДЕЙСТВИЯ (NEVER DO)

```
NEVER-1: Не создавай файлы/директории ВНЕ этой структуры
NEVER-2: Не создавай новый go.mod с другим namespace (только github.com/opus-casino/*)
NEVER-3: Не создавай новые proto-каталоги (используй ТОЛЬКО libs/proto/)
NEVER-4: Не дублируй существующий сервис (СНАЧАЛА проверь что уже есть)
NEVER-5: Не добавляй секреты/пароли в код или yaml (используй env variables)
NEVER-6: Не используй float/double для денег (ТОЛЬКО decimal/NUMERIC)
NEVER-7: Не принимай user_id из body — ВСЕГДА из JWT token
NEVER-8: Не используй GIN (проект на Fiber)
NEVER-9: Не создавай Dockerfile для конкретного сервиса (используй generic)
NEVER-10: Не используй Go версию ниже 1.22
```

---

## 📁 СТРУКТУРА ПРОЕКТА (единый источник истины)

```
opus-casino/                        ← КОРЕНЬ ПРОЕКТА
├── apps/                           ← ФРОНТЕНД ПРИЛОЖЕНИЯ
│   ├── web/                        ← Next.js 14
│   ├── mobile/                     ← Flutter
│   └── admin/                      ← React + Ant Design
│
├── services/                       ← ВСЕ БЭКЕНД СЕРВИСЫ
│   ├── rust/                       ← Cargo workspace (Cargo.toml в корне)
│   │   ├── Cargo.toml              ← workspace manifest
│   │   ├── betting-engine/
│   │   ├── wallet-core/
│   │   └── websocket-gateway/
│   ├── go/                         ← ВСЕ Go сервисы (каждый со своим go.mod)
│   │   ├── auth/
│   │   ├── user/
│   │   ├── payment/
│   │   ├── bonus/
│   │   ├── casino/
│   │   ├── notification/
│   │   ├── kyc/
│   │   └── affiliate/
│   └── python/                     ← Python ML сервисы
│       ├── fraud-ml/
│       └── analytics/
│
├── libs/                           ← ОБЩИЕ БИБЛИОТЕКИ
│   ├── proto/                      ← ЕДИНСТВЕННЫЙ proto каталог (buf v2)
│   │   ├── buf.yaml
│   │   ├── buf.gen.yaml
│   │   ├── common/v1/
│   │   ├── auth/v1/
│   │   ├── user/v1/
│   │   ├── wallet/v1/
│   │   ├── betting/v1/
│   │   ├── payment/v1/
│   │   ├── bonus/v1/
│   │   ├── casino/v1/
│   │   ├── notification/v1/
│   │   ├── kyc/v1/
│   │   ├── fraud/v1/
│   │   └── events/v1/
│   ├── shared/
│   └── migrations/
│
├── infra/
│   ├── docker/                     ← Dockerfiles (ТОЛЬКО generic)
│   │   ├── Dockerfile.go           ← для ВСЕХ Go сервисов
│   │   ├── Dockerfile.rust         ← для ВСЕХ Rust сервисов
│   │   ├── Dockerfile.python       ← для Python сервисов
│   │   └── Dockerfile.web          ← для Next.js
│   ├── k8s/
│   ├── terraform/
│   ├── helm/
│   └── argocd/
│
└── docs/
```

### ⚠️ ПРАВИЛО: Если директория не указана выше — НЕ СОЗДАВАЙ ЕЁ.
### ⚠️ ПРАВИЛО: Если нужен новый сервис — создавай в `services/go/<name>/` или `services/rust/<name>/`.
### ⚠️ ПРАВИЛО: НЕЛЬЗЯ создавать сервис напрямую в `services/` (например `services/my-service/`).

---

## 🏷️ NAMING CONVENTIONS

### Go Modules
```
✅ ПРАВИЛЬНО:  module github.com/opus-casino/{service-name}
❌ НЕПРАВИЛЬНО: module github.com/platform/services/{service-name}
❌ НЕПРАВИЛЬНО: module github.com/opus-casino/services/{service-name}

Примеры:
  services/go/auth/go.mod       → module github.com/opus-casino/auth
  services/go/payment/go.mod    → module github.com/opus-casino/payment
  services/go/affiliate/go.mod  → module github.com/opus-casino/affiliate
```

### Proto Imports
```
✅ ПРАВИЛЬНО:  github.com/opus-casino/proto
❌ НЕПРАВИЛЬНО: github.com/platform/proto

Replace директива (в go.mod):
  replace github.com/opus-casino/proto => ../../libs/proto
```

### Go Version
```
Минимальная версия: go 1.22
Все go.mod файлы ДОЛЖНЫ содержать: go 1.22 или выше
Пример: go 1.22, go 1.23, go 1.24 — всё ОК
```

### Dockerfile Names
```
✅ ПРАВИЛЬНО:  infra/docker/Dockerfile.go     (generic для всех Go сервисов)
✅ ПРАВИЛЬНО:  infra/docker/Dockerfile.rust   (generic для всех Rust сервисов)
❌ НЕПРАВИЛЬНО: infra/docker/Dockerfile.auth   (специфичный для сервиса)
❌ НЕПРАВИЛЬНО: services/go/auth/Dockerfile    (внутри сервиса)
```

### HTTP Ports (для локальной разработки)
```
betting-engine:      8080 (HTTP) / 50054 (gRPC)
wallet-core:         8081 (HTTP) / 50053 (gRPC)
websocket-gateway:   8082 (HTTP)
auth:                8083 (HTTP) / 50051 (gRPC)
payment:             8084 (HTTP) / 50055 (gRPC)
user:                8085 (HTTP) / 50052 (gRPC)
casino:              8086 (HTTP) / 50057 (gRPC)
notification:        8087 (HTTP) / 50058 (gRPC)
bonus:               8088 (HTTP) / 50056 (gRPC)
kyc:                 8089 (HTTP) / 50059 (gRPC)
affiliate:           8090 (HTTP) / 50061 (gRPC)
```

---

## 🏗️ СТРУКТУРА Go-СЕРВИСА

```
services/go/{service-name}/
├── go.mod                          ← module github.com/opus-casino/{service-name}
├── go.sum
├── main.go                         ← Точка входа
├── routes.go                       ← HTTP routes (Fiber)
├── internal/
│   ├── config/                     ← Конфигурация (env/yaml)
│   ├── service/                    ← Бизнес-логика
│   ├── repository/                 ← Доступ к данным (pgx/v5)
│   ├── handler/                    ← gRPC handlers (если есть)
│   ├── domain/                     ← Типы и value objects
│   └── client/                     ← gRPC клиенты к другим сервисам
└── tests/                          ← Интеграционные тесты
```

### Go-сервис main.go шаблон (ОБЯЗАТЕЛЬНАЯ структура)
```go
package main

import (
    "context"
    "fmt"
    "net"
    "os"
    "os/signal"
    "syscall"

    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/logger"
    "github.com/gofiber/fiber/v2/middleware/recover"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/redis/go-redis/v9"
    "go.uber.org/zap"
    "google.golang.org/grpc"
    "google.golang.org/grpc/reflection"
)

func main() {
    // 1. Logger
    // 2. Config
    // 3. DB Pool (GORM + pgx driver)
    // 4. Redis/DragonflyDB (go-redis/v9)
    // 5. Repository → Service
    // 6. gRPC server (goroutine)
    // 7. Fiber HTTP server (goroutine)
    // 8. Health endpoints: GET /health, GET /ready
    // 9. Graceful shutdown (SIGINT/SIGTERM)
}
```

---

## 🏗️ СТРУКТУРА Rust-СЕРВИСА

```
services/rust/{service-name}/
├── Cargo.toml                      ← Часть workspace
└── src/
    ├── main.rs                     ← Bootstrap + Axum server
    ├── config.rs                   ← Configuration
    ├── state.rs                    ← AppState
    ├── router.rs                   ← Route definitions
    ├── api/                        ← HTTP handlers
    ├── domain/                     ← Core types
    ├── services/                   ← Business logic
    ├── repositories/               ← Data access (sqlx)
    ├── grpc/                       ← gRPC server + clients (tonic)
    ├── events/                     ← Redpanda producer/consumer
    └── errors.rs                   ← Error types
```

---

## ✅ CHECKLIST ПЕРЕД СОЗДАНИЕМ ФАЙЛА

Перед созданием ЛЮБОГО нового файла или директории, ответь на эти вопросы:

```
□ Файл находится внутри services/go/, services/rust/, services/python/, 
  apps/, libs/, infra/, или docs/?
  → Если НЕТ — СТОП! Ты создаёшь файл в неправильном месте.

□ Это новый Go-сервис? Его go.mod использует github.com/opus-casino/?
  → Если НЕТ — СТОП! Неправильный namespace.

□ Это proto-файл? Он находится в libs/proto/{service}/v1/?
  → Если НЕТ — СТОП! Нельзя создавать proto в другом месте.

□ Это Dockerfile? Он generic (Dockerfile.go, Dockerfile.rust)?
  → Если НЕТ — СТОП! Не создавай специфичные Dockerfiles.

□ Такой сервис уже существует?
  → Проверь: ls services/go/, ls services/rust/, ls services/python/
  → Если ДА — модифицируй существующий, не создавай новый.

□ Файл содержит пароли, токены, секреты?
  → Если ДА — СТОП! Используй переменные окружения.
```

---

## 🔗 PROTO КОНТРАКТЫ

Единственный источник proto-определений: `libs/proto/`

```
Proto package naming:    {service}.v1        (например: auth.v1)
Proto файл location:     libs/proto/{service}/v1/{service}.proto
Go package:              github.com/opus-casino/proto/gen/go/{service}/v1
Buf module:              buf.build/opus-casino/api
```

### Запрещено:
```
❌ Создавать proto файлы ВНЕ libs/proto/
❌ Создавать второй proto каталог (proto/, platform-proto/, и т.д.)
❌ Использовать другой package naming (не {service}.v1)
```

---

## 🔒 БЕЗОПАСНОСТЬ

```
1. user_id → ВСЕГДА из JWT middleware, НИКОГДА из request body
2. Пароли → Argon2id (НИКОГДА bcrypt, md5, sha256)
3. JWT → Ed25519 подпись, TTL 15 минут
4. Секреты → env variables или Vault (НИКОГДА в коде или yaml)
5. Деньги → decimal/NUMERIC(18,8) (НИКОГДА float64/f64)
6. SQL → параметризованные запросы (НИКОГДА строковые конкатенации)
```

---

## 📊 ЗАВИСИМОСТИ (что использовать)

### Go
| Категория | Использовать | НЕ использовать |
|-----------|-------------|-----------------|
| HTTP фреймворк | Fiber v2 | Gin, Echo, Chi |
| ORM | GORM (gorm.io/gorm) | raw database/sql |
| DB драйвер | pgx/v5 (через GORM или напрямую для raw SQL) | database/sql без pgx |
| Логирование | zap | logrus, log |
| Redis | go-redis/v9 | redigo |
| gRPC | google.golang.org/grpc | — |
| UUID | google/uuid | satori/uuid |
| Config | viper | envconfig |

> **Примечание по GORM vs pgx:**
> - GORM — основной ORM для CRUD операций (7 из 8 сервисов уже используют)
> - pgx/v5 — для raw SQL, complex queries, и performance-critical операций
> - Оба допустимы. НЕ переписывай GORM на pgx без причины.
> - Правила работы с GORM: см. `.qwen/skills/go/go-database.skill.md`

### Rust
| Категория | Использовать | НЕ использовать |
|-----------|-------------|-----------------|
| HTTP фреймворк | Axum | Actix, Rocket |
| Async runtime | Tokio | async-std |
| DB драйвер | SQLx | diesel |
| Redis | fred | redis-rs |
| gRPC | Tonic | — |
| Serialization | serde | — |

---

## 🎯 ОТВЕТСТВЕННОСТЬ

Если ты агент и нарушил эти правила — это баг в твоей работе.
Перечитай CONVENTIONS.md перед каждой задачей.
