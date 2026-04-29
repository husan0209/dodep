# Wallet Core Service

**Wallet Core** — это микросервис управления кошельками и финансовыми операциями для платформы Opus Casino.

## 📋 Описание

Wallet Core отвечает за:
- Управление кошельками пользователей (main, bonus, free_spins, cashback)
- Балансы (available, locked, bonus)
- Финансовые транзакции (депозиты, выводы, ставки, выигрыши)
- Lock/Unlock средств для_pending ставок
- Трансферы между кошельками
- Idempotency финансовых операций
- Event publishing для аналитики

## 🚀 Быстрый старт

### Требования

- Rust >= 1.75
- PostgreSQL >= 16
- Redis/DragonflyDB >= 7
- Docker (опционально)

### Запуск в development

```bash
# Скопировать .env.example в .env
cp .env.example .env

# Запустить миграции
sqlx migrate run --database-url postgres://postgres:postgres@localhost:5432/opus_casino

# Запустить сервис
cargo run
```

### Запуск в Docker

```bash
# Собрать образ
docker build -t opus-casino/wallet-core:latest .

# Запустить контейнер
docker run -d \
  --name wallet-core \
  -p 50053:50053 \
  -p 3003:3003 \
  -p 9003:9003 \
  --env-file .env \
  opus-casino/wallet-core:latest
```

## 📡 API

### gRPC сервисы

Wallet Core реализует `WalletCoreService` из `wallet/v1/wallet.proto`:

| Метод | Описание |
|-------|----------|
| `GetBalance` | Получить баланс кошелька |
| `GetWallets` | Получить все кошельки пользователя |
| `Credit` | Зачислить средства (депозит, выигрыш) |
| `Debit` | Списать средства (ставка, вывод) |
| `Lock` | Заблокировать средства (для pending ставки) |
| `Unlock` | Разблокировать средства (отмена ставки) |
| `Transfer` | Перевод между кошельками |
| `GetTransactions` | История транзакций |

### HTTP endpoints

| Endpoint | Метод | Описание |
|----------|-------|----------|
| `/health` | GET | Health check (DB + Redis) |
| `/ready` | GET | Readiness probe |
| `/live` | GET | Liveness probe |
| `/metrics` | GET | Prometheus metrics |

## 🔧 Конфигурация

### Переменные окружения

| Переменная | Описание | Default |
|------------|----------|---------|
| `APP__ENV` | Окружение (development/production) | development |
| `DATABASE__HOST` | PostgreSQL host | localhost |
| `DATABASE__PORT` | PostgreSQL port | 5432 |
| `DATABASE__DATABASE` | Database name | opus_casino |
| `DATABASE__USERNAME` | Database user | postgres |
| `DATABASE__PASSWORD` | Database password | postgres |
| `REDIS__HOST` | Redis host | localhost |
| `REDIS__PORT` | Redis port | 6379 |
| `GRPC__ADDR` | gRPC listen address | 0.0.0.0:50053 |
| `HTTP__ADDR` | HTTP listen address | 0.0.0.0:3003 |
| `METRICS__ADDR` | Metrics listen address | 0.0.0.0:9003 |
| `TRACING__OTLP_ENDPOINT` | OpenTelemetry endpoint | http://localhost:4317 |

Полный список в `.env.example`.

## 🗄️ База данных

### Таблицы

- `wallets` — кошельки пользователей
- `transactions` — история транзакций
- `fund_locks` — заблокированные средства
- `ledger_entries` — double-entry bookkeeping

### Миграции

Миграции централизованы в `libs/migrations/postgresql/015_wallet_core.sql` и применяются автоматически при запуске.

```bash
# Применить миграции (sqlx)
sqlx migrate run --source ../../../libs/migrations/postgresql

# Создать новую миграцию (централизованно)
sqlx migrate add <name> --source ../../../libs/migrations/postgresql
```

## 📊 Метрики

Wallet Core экспортирует метрики в Prometheus формате:

- `wallet_operations_total` — количество операций
- `wallet_operation_duration_seconds` — длительность операций
- `wallet_balance_total` — общий баланс
- `db_pool_connections` — соединения с БД

## 🏷️ Tracing

Wallet Core использует OpenTelemetry для distributed tracing:

- Trace ID передается через gRPC metadata
- Spans создаются для каждого запроса
- Интеграция с Jaeger/Tempo

## 🧪 Тестирование

```bash
# Unit тесты
cargo test

# Integration тесты (требуется Docker)
cargo test --test integration

# Coverage
cargo tarpaulin --out Html
```

## 📦 Деплой

### Helm

```bash
# Установить
helm install wallet-core ./infra/helm/charts/wallet-core \
  --namespace wallet-core \
  --create-namespace \
  -f values.yaml

# Обновить
helm upgrade wallet-core ./infra/helm/charts/wallet-core \
  --namespace wallet-core \
  -f values.yaml
```

### ArgoCD

Wallet Core деплоится через ArgoCD с использованием App of Apps pattern.

## 🔐 Безопасность

- Все финансовые операции идемпотентны
- Optimistic locking для предотвращения race conditions
- Audit log всех операций
- Rate limiting на уровне API gateway

## 📄 Лицензия

Proprietary — все права защищены
