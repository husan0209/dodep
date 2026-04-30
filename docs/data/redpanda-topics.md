# Redpanda Topics Documentation

**Author:** DATA_ENGINEER
**Updated:** 2026-03-24
**Cluster:** 3 brokers, Schema Registry (Protobuf)

## Кластер

| Параметр           | Значение                                                |
| ------------------ | ------------------------------------------------------- |
| Брокеры            | 3 (StatefulSet)                                         |
| Replication factor | 3                                                       |
| Schema Registry    | Port 8081                                               |
| Bootstrap servers  | `redpanda-{0,1,2}.redpanda.data.svc.cluster.local:9092` |

## Топики

### Операционные (7 дней retention)

| Топик                 | Partitions | Key      | Описание                     |
| --------------------- | ---------- | -------- | ---------------------------- |
| `bets.bet.placed`     | 32         | user_id  | Ставка размещена             |
| `bets.bet.settled`    | 32         | bet_id   | Ставка рассчитана            |
| `bets.bet.cashout`    | 32         | user_id  | Cashout ставки               |
| `payments.initiated`  | 16         | user_id  | Платёж инициирован           |
| `payments.completed`  | 16         | user_id  | Платёж завершён              |
| `users.registered`    | 8          | user_id  | Пользователь зарегистрирован |
| `users.updated`       | 8          | user_id  | Профиль обновлён             |
| `users.kyc_verified`  | 8          | user_id  | KYC верифицирован            |
| `events.odds_updated` | 32         | event_id | Коэффициенты обновлены       |
| `notifications.send`  | 8          | user_id  | Уведомление для отправки     |

### Аналитические (30 дней retention)

| Топик                    | Partitions | Key     | Описание                     |
| ------------------------ | ---------- | ------- | ---------------------------- |
| `analytics.events`       | 16         | user_id | Все пользовательские события |
| `casino.round.completed` | 16         | user_id | Раунд казино завершён        |
| `fraud.signals`          | 8          | user_id | Сигналы fraud detection      |

### Аудит (infinite retention, compact)

| Топик       | Partitions | Key       | Описание                 |
| ----------- | ---------- | --------- | ------------------------ |
| `audit.log` | 8          | entity_id | Все изменения для аудита |

## Конфигурация топиков

### Операционные топики

```yaml
partitions: 32
replication_factor: 3
retention_ms: 604800000 # 7 days
min.insync.replicas: 2
cleanup.policy: delete
compression.type: zstd
```

### Аналитические топики

```yaml
partitions: 16
replication_factor: 3
retention_ms: 2592000000 # 30 days
min.insync.replicas: 2
cleanup.policy: delete
compression.type: zstd
```

### Аудит топик

```yaml
partitions: 8
replication_factor: 3
retention_ms: -1 # infinite
cleanup.policy: compact
min.insync.replicas: 2
compression.type: zstd
```

## Event Schema (Protobuf)

Все события определены в `libs/proto/events/v1/events.proto`.

### BetPlacedEvent

```protobuf
message BetPlacedEvent {
  string event_id = 1;
  google.protobuf.Timestamp timestamp = 2;
  string user_id = 3;
  string bet_id = 4;
  common.v1.BetType bet_type = 5;
  common.v1.Money stake = 6;
  string odds = 7;
  common.v1.Money potential_win = 8;
  string event_id_external = 9;
  string market_id = 10;
  string selection_id = 11;
  string currency = 12;
  string device_id = 13;
  string ip_address = 14;
  string client_request_id = 15;
  map<string, string> metadata = 16;
}
```

### BetSettledEvent

```protobuf
message BetSettledEvent {
  string event_id = 1;
  google.protobuf.Timestamp timestamp = 2;
  string user_id = 3;
  string bet_id = 4;
  BetResult result = 5;
  common.v1.Money actual_win = 6;
  common.v1.Money pnl = 7;
  string settled_by = 8;
  string client_request_id = 9;
}
```

### DepositCompletedEvent

```protobuf
message DepositCompletedEvent {
  string event_id = 1;
  google.protobuf.Timestamp timestamp = 2;
  string user_id = 3;
  string payment_id = 4;
  common.v1.Money amount = 5;
  string currency = 6;
  string payment_method = 7;
  string provider = 8;
  string client_request_id = 9;
}
```

## Consumer Groups

| Сервис               | Group ID                   | Топики                                              |
| -------------------- | -------------------------- | --------------------------------------------------- |
| ClickHouse           | `clickhouse_bet_events`    | bets.bet.placed, bets.bet.settled, bets.bet.cashout |
| ClickHouse           | `clickhouse_user_events`   | analytics.events                                    |
| ClickHouse           | `clickhouse_casino_rounds` | casino.round.completed                              |
| Notification Service | `notification-service`     | notifications.send                                  |
| Analytics Service    | `analytics-service`        | analytics.events                                    |
| Fraud Engine         | `fraud-engine`             | fraud.signals, bets.bet.placed                      |
| Payment Service      | `payment-service`          | payments.initiated, payments.completed              |

## Producer Config (Rust, rdkafka)

```rust
let producer: FutureProducer = ClientConfig::new()
    .set("bootstrap.servers", BROKERS)
    .set("acks", "all")
    .set("enable.idempotence", "true")
    .set("compression.type", "zstd")
    .set("linger.ms", "5")
    .set("batch.size", "65536")        // 64KB
    .set("retries", "3")
    .set("retry.backoff.ms", "100")
    .set("max.in.flight.requests.per.connection", "5")
    .create()?;
```

## Consumer Config (Go, franz-go)

```go
cl, err := kgo.NewClient(
    kgo.SeedBrokers(BROKERS...),
    kgo.ConsumerGroup(groupID),
    kgo.ConsumeTopics(topics...),
    kgo.DisableAutoCommit(),
    kgo.RequireStableFetchOffsets(),
    kgo.FetchMaxWait(500*time.Millisecond),
)
```

**Правила:**

- ❌ Никогда auto-commit → ручной commit после обработки
- ✅ Process first → commit offset
- ✅ On error → НЕ commit → automatic retry
- ✅ Idempotent consumers → check `processed:{event_id}` в DragonflyDB

## Dead Letter Queue

После 3 неудачных попыток обработки → publish в `{topic}.dlq` с error header.

```yaml
topics_with_dlq:
  - bets.bet.placed.dlq
  - payments.completed.dlq
  - notifications.send.dlq
```

**Алерт:** Любое сообщение в DLQ → P2 alert.

## Мониторинг алертов

| Алерт                       | Условие                  | Severity |
| --------------------------- | ------------------------ | -------- |
| Consumer lag high           | > 10K сообщений, > 5 мин | P2       |
| Produce latency             | p99 > 100ms              | P2       |
| Consumer errors             | > 10/min                 | P1       |
| DLQ message                 | Любое сообщение          | P2       |
| Under-replicated partitions | > 0                      | P1       |

## Ordering Guarantees

- ✅ Порядок сохраняется ВНУТРИ одного partition
- ✅ Key = user_id → события пользователя упорядочены
- ❌ Cross-topic ordering НЕ гарантирован
- ❌ Cross-partition ordering НЕ гарантирован
