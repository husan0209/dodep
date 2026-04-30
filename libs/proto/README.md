# Protobuf Contracts — Opus Casino

gRPC контракты и event-схемы для микросервисной платформы Opus Casino.

## 📁 Структура

```
libs/proto/
├── buf.yaml                 # Buf lint/breaking config
├── buf.gen.yaml             # Codegen configuration
├── common/v1/               # Общие типы
│   ├── types.proto          # Базовые типы (UserId, Money, etc.)
│   ├── money.proto          # Финансовые типы
│   ├── pagination.proto     # Пагинация
│   └── error.proto          # Error codes
│
├── auth/v1/auth.proto       # Auth Service
├── user/v1/user.proto       # User Service
├── wallet/v1/wallet.proto   # Wallet Core
├── betting/v1/betting.proto # Betting Engine
├── payment/v1/payment.proto # Payment Service
├── bonus/v1/bonus.proto     # Bonus Service
├── casino/v1/casino.proto   # Casino Service
├── notification/v1/notification.proto
├── kyc/v1/kyc.proto         # KYC Service
├── fraud/v1/fraud.proto     # Fraud Detection
└── events/v1/events.proto   # Platform Events (Redpanda)
```

## 🚀 Быстрый старт

### Требования

- **Buf CLI** >= 1.28: `brew install bufbuild/buf/buf` или [установить](https://docs.buf.build/installation)
- **Protobuf compiler** >= 3.21: `brew install protobuf`
- **Go** >= 1.21 (для Go codegen)
- **Rust** >= 1.75 (для Rust codegen)
- **Node.js** >= 20 (для TypeScript codegen)

### Генерация кода

```bash
# Перейти в директорию proto
cd libs/proto

# Установить зависимости (если нужно)
buf dep update

# Сгенерировать код для всех языков
buf generate

# Сгенерировать только для Go
buf generate --template buf.gen.yaml --path buf.build/opus-casino/api --output gen/go

# Проверить lint
buf lint

# Проверить breaking changes
buf breaking --against '.git#branch=main'
```

## 📦 Codegen

### Go

```bash
buf generate --template buf.gen.yaml
```

Код генерируется в `libs/proto/gen/go/`:

```go
import (
    pb "github.com/opus-casino/proto/gen/go/wallet/v1"
)

client := pb.NewWalletCoreServiceClient(conn)
resp, err := client.GetBalance(ctx, &pb.GetBalanceRequest{
    UserId: &commonv1.UserId{Value: user_id},
    WalletType: commonv1.WalletType_WALLET_TYPE_MAIN,
})
```

### Rust

```bash
buf generate --template buf.gen.yaml
```

Требуется установить локальные плагины:

```bash
cargo install protoc-gen-prost
cargo install protoc-gen-tonic
```

Код генерируется в `libs/proto/gen/rust/`.

### TypeScript

```bash
buf generate --template buf.gen.yaml
```

Код генерируется в `libs/proto/gen/typescript/`.

Использование:

```typescript
import { WalletCoreServiceClient } from './gen/go/wallet/v1/wallet_connect';
import { GetBalanceRequest } from './gen/go/wallet/v1/wallet_pb';

const client = new WalletCoreServiceClient('https://api.example.com');
const request = new GetBalanceRequest();
// ...
```

## 🏗 Архитектура

### Версионирование

- **Пакеты:** `{service}/v1/` (например, `wallet/v1/`)
- **Breaking changes:** новая мажорная версия (`v2/`)
- **Backward compatible:** новые поля с новыми номерами

### Naming conventions

```protobuf
// Packages: lower_snake_case с версией
package wallet.v1;

// Services: PascalCase + "Service" suffix
service WalletService { ... }

// RPCs: PascalCase, verb first
rpc GetBalance(...) returns (...);
rpc PlaceBet(...) returns (...);

// Messages: PascalCase
message GetBalanceRequest { ... }

// Fields: lower_snake_case
string user_id = 1;

// Enums: UPPER_SNAKE_CASE с префиксом
enum BetStatus {
  BET_STATUS_UNSPECIFIED = 0;
  BET_STATUS_PENDING = 1;
}
```

### Общие типы

Все сервисы используют общие типы из `common/v1/`:

- `UserId`, `BetId`, `TransactionId` — сильные типы для ID
- `Money` — деньги (amount как string для точности)
- `PaginationRequest/Response` — cursor-based пагинация
- `ErrorDetails` — стандартизированные ошибки

### Event-контракты

События для Redpanda определены в `events/v1/events.proto`:

| Топик | Событие | Key |
|-------|---------|-----|
| `bets.bet.placed` | `BetPlacedEvent` | user_id |
| `bets.bet.settled` | `BetSettledEvent` | bet_id |
| `users.user.registered` | `UserRegisteredEvent` | user_id |
| `payments.deposit.completed` | `DepositCompletedEvent` | deposit_id |
| `casino.game.round.completed` | `CasinoRoundCompletedEvent` | game_session_id |

## 📊 Сервисы

| Сервис | Порт | Описание |
|--------|------|----------|
| `AuthService` | 50051 | Аутентификация, сессии, 2FA |
| `UserService` | 50052 | Профили пользователей, лимиты |
| `WalletCoreService` | 50053 | Кошельки, транзакции, баланс |
| `BettingEngineService` | 50054 | Ставки, коэффициенты |
| `PaymentService` | 50055 | Депозиты, withdrawals |
| `BonusService` | 50056 | Бонусы, вагеринг |
| `CasinoService` | 50057 | Интеграция с провайдерами |
| `NotificationService` | 50058 | Уведомления |
| `KYCService` | 50059 | KYC верификация |
| `FraudService` | 50060 | Fraud детекция |

## 🔍 Buf Lint правила

Включены правила:

- `DEFAULT` — стандартные правила Buf
- `COMMENTS` — требуются документация
- `FILE_LOWER_SNAKE_CASE` — имена файлов

Исключения:

- `PACKAGE_VERSION_SUFFIX` — разрешаем `v1` без суффикса
- `SERVICE_SUFFIX` — разрешаем `Service` суффикс

## 🧪 Тестирование

### Проверка breaking changes

```bash
# Против main ветки
buf breaking --against '.git#branch=main,subdir=libs/proto'

# Против удалённого репозитория
buf breaking --against 'https://github.com/opus-casino/proto.git#branch=main'
```

### Валидация

```bash
# Проверить все proto файлы
buf lint

# Проверить конкретный файл
buf lint --path wallet/v1/wallet.proto
```

## 📝 Best Practices

### ✅ Делать

- Использовать `common.v1` типы для совместимости
- Добавлять документацию к каждому RPC и полю
- Использовать `idempotency_key` для финансовых операций
- Версионировать пакеты при breaking changes
- Резервировать номера удалённых полей

### ❌ Не делать

- Не использовать `float/double` для денег
- Не удалять поля без резервирования
- Не менять номера полей
- Не использовать `google.protobuf.Struct` вместо типизированных сообщений

## 🔗 Ссылки

- [Buf Documentation](https://docs.buf.build/)
- [Protobuf Language Guide](https://protobuf.dev/programming-guides/proto3/)
- [gRPC Documentation](https://grpc.io/docs/)
- [Buf Build Registry](https://buf.build/opus-casino/api)

## 📄 Лицензия

Proprietary — все права защищены
