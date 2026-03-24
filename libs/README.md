# Opus Casino — Protobuf Contracts & Shared Libraries

Этот пакет содержит Protobuf контракты и shared библиотеки для микросервисной платформы Opus Casino.

## 📁 Структура

```
libs/
├── proto/                    # Protobuf контракты
│   ├── buf.yaml              # Buf configuration
│   ├── buf.gen.yaml          # Codegen configuration
│   ├── common/v1/            # Общие типы
│   │   ├── types.proto
│   │   ├── money.proto
│   │   ├── pagination.proto
│   │   └── error.proto
│   ├── auth/v1/auth.proto
│   ├── user/v1/user.proto
│   ├── wallet/v1/wallet.proto
│   ├── betting/v1/betting.proto
│   ├── payment/v1/payment.proto
│   ├── bonus/v1/bonus.proto
│   ├── casino/v1/casino.proto
│   ├── notification/v1/notification.proto
│   ├── kyc/v1/kyc.proto
│   ├── fraud/v1/fraud.proto
│   └── events/v1/events.proto
│
└── shared/                   # Shared libraries
    ├── typescript/           # TypeScript shared library
    │   ├── package.json
    │   ├── tsconfig.json
    │   └── src/
    │       ├── index.ts
    │       ├── types.ts
    │       ├── validators.ts
    │       ├── constants.ts
    │       └── helpers.ts
    │
    ├── rust/                 # Rust shared library
    │   ├── Cargo.toml
    │   └── src/
    │       ├── lib.rs
    │       ├── types.rs
    │       ├── validators.rs
    │       ├── constants.rs
    │       ├── helpers.rs
    │       └── error.rs
    │
    └── go/                   # Go shared library
        ├── go.mod
        ├── lib.go
        ├── types/
        ├── validators/
        ├── constants/
        ├── helpers/
        └── errors/
```

## 🚀 Быстрый старт

### Protobuf

```bash
# Перейти в директорию proto
cd libs/proto

# Сгенерировать код для всех языков
make gen

# Или использовать buf напрямую
buf generate

# Проверить lint
buf lint

# Проверить breaking changes
buf breaking --against '.git#branch=main,subdir=libs/proto'
```

### Shared Libraries

#### TypeScript

```bash
cd libs/shared/typescript

# Установить зависимости
npm install

# Собрать
npm run build

# Тесты
npm test
```

#### Rust

```bash
cd libs/shared/rust

# Собрать
cargo build

# Тесты
cargo test

# Clippy
cargo clippy
```

#### Go

```bash
cd libs/shared/go

# Скачать зависимости
go mod download

# Собрать
go build ./...

# Тесты
go test ./...
```

## 📦 Protobuf Сервисы

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

## 📊 Event Топики

| Топик | Событие | Key |
|-------|---------|-----|
| `bets.bet.placed` | `BetPlacedEvent` | user_id |
| `bets.bet.settled` | `BetSettledEvent` | bet_id |
| `bets.bet.cancelled` | `BetCancelledEvent` | bet_id |
| `users.user.registered` | `UserRegisteredEvent` | user_id |
| `users.user.verified` | `UserVerifiedEvent` | user_id |
| `users.session.created` | `SessionCreatedEvent` | session_id |
| `payments.deposit.initiated` | `DepositInitiatedEvent` | user_id |
| `payments.deposit.completed` | `DepositCompletedEvent` | deposit_id |
| `payments.withdrawal.requested` | `WithdrawalRequestedEvent` | user_id |
| `casino.game.started` | `CasinoGameStartedEvent` | user_id |
| `casino.game.round.completed` | `CasinoRoundCompletedEvent` | game_session_id |
| `bonus.bonus.activated` | `BonusActivatedEvent` | user_id |
| `fraud.alert.created` | `FraudAlertCreatedEvent` | user_id |

## 🔧 Требования

### Для Protobuf

- **Buf CLI** >= 1.28
- **Protobuf compiler** >= 3.21
- **Go** >= 1.21 (для Go codegen)
- **Rust** >= 1.75 (для Rust codegen)
- **Node.js** >= 20 (для TypeScript codegen)

### Для Shared Libraries

- **TypeScript**: Node.js >= 20, npm >= 9
- **Rust**: Rust >= 1.75, Cargo
- **Go**: Go >= 1.21

## 📝 Conventions

### Protobuf

- **Packages:** `lower_snake_case` с версией (`wallet.v1`)
- **Services:** `PascalCase` + "Service" суффикс
- **RPCs:** `PascalCase`, глагол первый (`GetBalance`, `PlaceBet`)
- **Messages:** `PascalCase`
- **Fields:** `lower_snake_case`
- **Enums:** `UPPER_SNAKE_CASE` с префиксом (`BET_STATUS_PENDING`)

### Версионирование

- **Пакеты:** `{service}/v1/` (например, `wallet/v1/`)
- **Breaking changes:** новая мажорная версия (`v2/`)
- **Backward compatible:** новые поля с новыми номерами

## 📚 Документация

- [Protobuf README](proto/README.md)
- [TypeScript Shared Library](shared/typescript/README.md)
- [Rust Shared Library](shared/rust/README.md)
- [Go Shared Library](shared/go/README.md)

## 🧪 Тестирование

### Protobuf

```bash
# Lint
cd libs/proto && buf lint

# Breaking changes
cd libs/proto && buf breaking --against '.git#branch=main,subdir=libs/proto'
```

### TypeScript

```bash
cd libs/shared/typescript
npm run test
```

### Rust

```bash
cd libs/shared/rust
cargo test
```

### Go

```bash
cd libs/shared/go
go test ./...
```

## 📄 Лицензия

Proprietary — все права защищены
