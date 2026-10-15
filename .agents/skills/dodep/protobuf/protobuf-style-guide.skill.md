## #60 protobuf-style-guide.skill.md

```markdown
# protobuf-style-guide.skill.md

## РОЛЬ
Ты определяешь стандарты для Protobuf схем и gRPC сервисов
гемблинг-платформы. Proto файлы — контракт между сервисами.

## КОНТЕКСТ
- Protobuf 3 (proto3 syntax)
- Buf (buf.build) для lint, breaking change detection, codegen
- Codegen: Rust (tonic/prost), Go (google.golang.org/protobuf), TypeScript
- Schema Registry: Redpanda Schema Registry
- Версионирование: пакет v1, v2 при breaking changes

## СТРУКТУРА РЕПОЗИТОРИЯ
proto/
├── buf.yaml # Buf config
├── buf.gen.yaml # Codegen config
├── buf.lock
│
├── common/
│ └── v1/
│ ├── money.proto
│ ├── pagination.proto
│ ├── timestamp.proto
│ └── error.proto
│
├── auth/
│ └── v1/
│ └── auth.proto
│
├── user/
│ └── v1/
│ └── user.proto
│
├── wallet/
│ └── v1/
│ └── wallet.proto
│
├── betting/
│ └── v1/
│ ├── bet_service.proto
│ ├── odds_service.proto
│ └── event_service.proto
│
├── payment/
│ └── v1/
│ └── payment.proto
│
├── casino/
│ └── v1/
│ └── casino.proto
│
├── notification/
│ └── v1/
│ └── notification.proto
│
├── kyc/
│ └── v1/
│ └── kyc.proto
│
├── bonus/
│ └── v1/
│ └── bonus.proto
│
├── fraud/
│ └── v1/
│ └── fraud.proto
│
├── admin/
│ └── v1/
│ └── admin.proto
│
└── events/
└── v1/
├── bet_events.proto
├── payment_events.proto
├── user_events.proto
└── casino_events.proto

text


## BUF CONFIGURATION

```yaml
# buf.yaml
version: v2
modules:
  - path: .
    name: buf.build/gambling-platform/api
lint:
  use:
    - DEFAULT
    - COMMENTS
  except:
    - PACKAGE_VERSION_SUFFIX
breaking:
  use:
    - FILE

# buf.gen.yaml
version: v2
managed:
  enabled: true
  override:
    - file_option: go_package_prefix
      value: github.com/org/platform/gen/go
plugins:
  - remote: buf.build/protocolbuffers/go
    out: gen/go
    opt: paths=source_relative
  - remote: buf.build/grpc/go
    out: gen/go
    opt: paths=source_relative
  - local: protoc-gen-prost
    out: gen/rust
  - local: protoc-gen-tonic
    out: gen/rust
  - local: protoc-gen-ts
    out: gen/ts
COMMON TYPES
protobuf

// common/v1/money.proto
syntax = "proto3";
package common.v1;

// Money represents a monetary amount with currency.
// Amount is stored as string to avoid floating point issues.
// Examples: "100.00", "0.50", "1234.56"
message Money {
  // Amount as decimal string (e.g., "100.00")
  string amount = 1;
  // ISO 4217 currency code (e.g., "USD", "EUR")
  string currency = 2;
}

// common/v1/pagination.proto
syntax = "proto3";
package common.v1;

// Cursor-based pagination request
message PaginationRequest {
  // Maximum items to return (1-100, default 20)
  int32 page_size = 1;
  // Cursor from previous response (empty for first page)
  string cursor = 2;
}

// Cursor-based pagination response
message PaginationResponse {
  // Cursor for next page (empty if no more pages)
  string next_cursor = 1;
  // Whether there are more items
  bool has_more = 2;
  // Total count (optional, may be expensive)
  optional int64 total_count = 3;
}
SERVICE DEFINITION
protobuf

// wallet/v1/wallet.proto
syntax = "proto3";
package wallet.v1;

import "common/v1/money.proto";
import "google/protobuf/timestamp.proto";

// WalletService manages user wallet operations.
// All financial operations are idempotent via idempotency_key.
service WalletService {
  // GetBalance returns the current balance for a user's wallet.
  rpc GetBalance(GetBalanceRequest) returns (GetBalanceResponse);

  // Debit removes funds from user's available balance.
  // Returns FAILED_PRECONDITION if insufficient funds.
  rpc Debit(DebitRequest) returns (DebitResponse);

  // Credit adds funds to user's available balance.
  rpc Credit(CreditRequest) returns (CreditResponse);

  // Lock moves funds from available to locked (for pending bets).
  rpc Lock(LockRequest) returns (LockResponse);

  // Unlock moves funds from locked back to available.
  rpc Unlock(UnlockRequest) returns (UnlockResponse);

  // Settle finalizes a locked amount (win/loss/void).
  rpc Settle(SettleRequest) returns (SettleResponse);

  // GetTransactions returns paginated transaction history.
  rpc GetTransactions(GetTransactionsRequest) returns (GetTransactionsResponse);
}

message GetBalanceRequest {
  int64 user_id = 1;
  string currency_code = 2;
}

message GetBalanceResponse {
  common.v1.Money available = 1;
  common.v1.Money locked = 2;
  common.v1.Money bonus = 3;
  common.v1.Money total = 4;
  int32 version = 5;
}

message DebitRequest {
  int64 user_id = 1;
  string currency_code = 2;
  common.v1.Money amount = 3;
  // Unique key for idempotency (UUID v4)
  string idempotency_key = 4;
  // Reference to related entity
  string reference_type = 5;  // "bet", "withdrawal", "bonus"
  int64 reference_id = 6;
  string reason = 7;
}

message DebitResponse {
  int64 transaction_id = 1;
  common.v1.Money new_balance = 2;
}
EVENT MESSAGES
protobuf

// events/v1/bet_events.proto
syntax = "proto3";
package events.v1;

import "google/protobuf/timestamp.proto";
import "common/v1/money.proto";

// BetPlacedEvent is published when a bet is successfully placed.
// Topic: bets.bet.placed
// Key: user_id (string)
message BetPlacedEvent {
  // Unique event identifier (UUID v4)
  string event_id = 1;
  google.protobuf.Timestamp timestamp = 2;
  int64 user_id = 3;
  int64 bet_id = 4;
  BetType bet_type = 5;
  common.v1.Money stake = 6;
  double total_odds = 7;
  common.v1.Money potential_win = 8;
  repeated BetSelection selections = 9;
  string currency = 10;
  string device = 11;
  string ip_address = 12;
  string idempotency_key = 13;
}

enum BetType {
  BET_TYPE_UNSPECIFIED = 0;
  BET_TYPE_SINGLE = 1;
  BET_TYPE_ACCUMULATOR = 2;
  BET_TYPE_SYSTEM = 3;
}

message BetSelection {
  int64 event_id = 1;
  int64 market_id = 2;
  int64 outcome_id = 3;
  double odds = 4;
  string event_name = 5;
  string market_name = 6;
  string outcome_name = 7;
}

// BetSettledEvent is published when a bet is settled.
// Topic: bets.bet.settled
message BetSettledEvent {
  string event_id = 1;
  google.protobuf.Timestamp timestamp = 2;
  int64 user_id = 3;
  int64 bet_id = 4;
  BetResult result = 5;
  common.v1.Money actual_win = 6;
  common.v1.Money pnl = 7;
}

enum BetResult {
  BET_RESULT_UNSPECIFIED = 0;
  BET_RESULT_WON = 1;
  BET_RESULT_LOST = 2;
  BET_RESULT_VOID = 3;
  BET_RESULT_HALF_WON = 4;
  BET_RESULT_HALF_LOST = 5;
}
NAMING RULES
protobuf

// Packages: lower_snake_case с версией
package wallet.v1;
package betting.v1;

// Services: PascalCase + "Service" suffix
service WalletService { ... }
service BettingService { ... }

// RPCs: PascalCase, verb first
rpc GetBalance(...)
rpc PlaceBet(...)
rpc CancelWithdrawal(...)

// Messages: PascalCase
message GetBalanceRequest { ... }
message GetBalanceResponse { ... }

// Fields: lower_snake_case
int64 user_id = 1;
string currency_code = 2;
common.v1.Money available_balance = 3;

// Enums: UPPER_SNAKE_CASE с prefix
enum BetStatus {
  BET_STATUS_UNSPECIFIED = 0;  // всегда 0 = UNSPECIFIED
  BET_STATUS_PENDING = 1;
  BET_STATUS_ACTIVE = 2;
  BET_STATUS_WON = 3;
  BET_STATUS_LOST = 4;
  BET_STATUS_VOID = 5;
}

// Request/Response: {RpcName}Request, {RpcName}Response
rpc PlaceBet(PlaceBetRequest) returns (PlaceBetResponse);
FIELD NUMBER RULES
protobuf

// 1-15:   часто используемые поля (1 byte encoding)
// 16-2047: менее частые поля (2 byte encoding)
// НИКОГДА не переиспользовать удалённый номер

message User {
  int64 id = 1;            // часто
  string email = 2;        // часто
  string status = 3;       // часто
  int32 kyc_level = 4;     // часто

  string timezone = 16;    // реже
  string language = 17;    // реже
  map<string, string> metadata = 18;  // реже

  // Зарезервировать удалённые поля
  reserved 10, 11;
  reserved "old_field_name";
}
BACKWARD COMPATIBILITY
protobuf

// ✅ БЕЗОПАСНЫЕ изменения (не breaking):
//   - Добавить новое поле (с новым номером)
//   - Добавить новый RPC
//   - Добавить новое enum value
//   - Добавить новый сервис
//   - Изменить комментарии

// ❌ BREAKING изменения (запрещены без новой версии):
//   - Удалить поле
//   - Изменить тип поля
//   - Изменить номер поля
//   - Переименовать поле (wire format не изменится, но codegen сломается)
//   - Удалить RPC
//   - Изменить stream/unary тип RPC
//   - Удалить enum value

// Если нужен breaking change → новая версия пакета
package wallet.v2;  // v1 продолжает работать
АНТИПАТТЕРНЫ
protobuf

// ❌ ПЛОХО: float/double для денег
message Payment {
  double amount = 1;  // 0.1 + 0.2 = 0.30000000000000004
}

// ✅ ПРАВИЛЬНО: string или int64 (cents)
message Payment {
  string amount = 1;  // "100.50" — парсится как Decimal
  // ИЛИ
  int64 amount_cents = 1;  // 10050 = $100.50
}

// ❌ ПЛОХО: нет UNSPECIFIED в enum
enum Status {
  ACTIVE = 0;  // 0 будет default → все "пустые" значения станут ACTIVE
}

// ✅ ПРАВИЛЬНО:
enum Status {
  STATUS_UNSPECIFIED = 0;  // default = unknown
  STATUS_ACTIVE = 1;
}

// ❌ ПЛОХО: generic Request/Response
message Request { ... }
message Response { ... }

// ✅ ПРАВИЛЬНО: per-RPC messages
message GetBalanceRequest { ... }
message GetBalanceResponse { ... }

// ❌ ПЛОХО: google.protobuf.Struct для всего
message Event {
  google.protobuf.Struct data = 1;  // потеря типизации
}

// ✅ ПРАВИЛЬНО: typed message
message BetPlacedEvent {
  int64 bet_id = 1;
  double stake = 2;
  // ...
}

// ❌ ПЛОХО: timestamp как string или int64
message Event {
  string created_at = 1;  // какой формат? ISO? Unix?
}

// ✅ ПРАВИЛЬНО:
import "google/protobuf/timestamp.proto";
message Event {
  google.protobuf.Timestamp created_at = 1;
}