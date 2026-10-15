## #37 redpanda-events.skill.md

```markdown
# redpanda-events.skill.md

## РОЛЬ
Ты — Backend Developer, работающий с Redpanda (Kafka-совместимый брокер)
для event-driven архитектуры гемблинг-платформы.

## КОНТЕКСТ
- Redpanda: Kafka API, без JVM/ZooKeeper
- 3 брокера, replication factor 3
- Schema Registry (Protobuf)
- Источники: все микросервисы
- Потребители: аналитика (ClickHouse), уведомления, fraud engine

## TOPIC NAMING
{domain}.{entity}.{action}

Примеры:
bets.bet.placed
bets.bet.settled
bets.bet.cashout
payments.deposit.initiated
payments.deposit.completed
payments.withdrawal.requested
payments.withdrawal.approved
users.user.registered
users.user.updated
users.user.blocked
users.kyc.verified
users.kyc.rejected
casino.round.started
casino.round.completed
notifications.send
fraud.signal.detected
audit.action.recorded
odds.update.published

text


## TOPIC КОНФИГУРАЦИЯ

```yaml
# Операционные топики — 7 дней retention
bets.bet.placed:
  partitions: 32           # по user_id для ordering
  replication_factor: 3
  retention_ms: 604800000  # 7 days
  min_insync_replicas: 2
  cleanup_policy: delete
  compression: zstd

# Аналитические — 30 дней
analytics.events:
  partitions: 16
  replication_factor: 3
  retention_ms: 2592000000  # 30 days
  cleanup_policy: delete

# Audit — compacted (хранить последнее состояние навсегда)
audit.action.recorded:
  partitions: 8
  replication_factor: 3
  retention_ms: -1          # бесконечно
  cleanup_policy: compact
EVENT SCHEMA (Protobuf)
protobuf

// events/v1/bet_events.proto
syntax = "proto3";
package events.v1;

import "google/protobuf/timestamp.proto";
import "common/v1/money.proto";

message BetPlacedEvent {
  string event_id = 1;            // unique event UUID
  google.protobuf.Timestamp timestamp = 2;
  int64 user_id = 3;
  int64 bet_id = 4;
  string bet_type = 5;            // single, accumulator, system
  common.v1.Money stake = 6;
  double total_odds = 7;
  common.v1.Money potential_win = 8;
  repeated Selection selections = 9;
  string currency = 10;
  string device = 11;
  string ip_address = 12;
  string idempotency_key = 13;
}

message BetSettledEvent {
  string event_id = 1;
  google.protobuf.Timestamp timestamp = 2;
  int64 user_id = 3;
  int64 bet_id = 4;
  string result = 5;              // won, lost, void
  common.v1.Money actual_win = 6;
  common.v1.Money pnl = 7;        // profit/loss
}

message Selection {
  int64 event_id = 1;
  int64 market_id = 2;
  int64 outcome_id = 3;
  double odds = 4;
  string event_name = 5;
  string market_name = 6;
  string outcome_name = 7;
}
PRODUCER (Rust)
Rust

// Rust — Redpanda producer
use rdkafka::producer::{FutureProducer, FutureRecord};
use rdkafka::ClientConfig;
use prost::Message;

pub struct EventProducer {
    producer: FutureProducer,
}

impl EventProducer {
    pub fn new(brokers: &str) -> Result<Self> {
        let producer: FutureProducer = ClientConfig::new()
            .set("bootstrap.servers", brokers)
            .set("message.timeout.ms", "5000")
            .set("acks", "all")                    // wait for all replicas
            .set("enable.idempotence", "true")     // exactly-once semantics
            .set("compression.type", "zstd")
            .set("linger.ms", "5")                 // batch for 5ms
            .set("batch.size", "65536")            // 64KB batches
            .set("retries", "3")
            .set("retry.backoff.ms", "100")
            .create()?;

        Ok(Self { producer })
    }

    pub async fn publish_bet_placed(
        &self,
        event: &BetPlacedEvent,
    ) -> Result<()> {
        let key = event.user_id.to_string();  // partition by user_id
        let payload = event.encode_to_vec();

        let record = FutureRecord::to("bets.bet.placed")
            .key(&key)
            .payload(&payload)
            .headers(OwnedHeaders::new()
                .insert(Header {
                    key: "event_type",
                    value: Some("BetPlaced"),
                })
                .insert(Header {
                    key: "event_id",
                    value: Some(&event.event_id),
                })
                .insert(Header {
                    key: "timestamp",
                    value: Some(&Utc::now().to_rfc3339()),
                })
            );

        self.producer
            .send(record, Duration::from_secs(5))
            .await
            .map_err(|(err, _)| Error::EventPublish(err.to_string()))?;

        Ok(())
    }
}
CONSUMER (Go)
Go

// Go — Redpanda consumer
package events

import (
    "context"
    "github.com/twmb/franz-go/pkg/kgo"
    "google.golang.org/protobuf/proto"
)

type EventConsumer struct {
    client *kgo.Client
}

func NewEventConsumer(brokers []string, group string, topics []string) (*EventConsumer, error) {
    client, err := kgo.NewClient(
        kgo.SeedBrokers(brokers...),
        kgo.ConsumerGroup(group),
        kgo.ConsumeTopics(topics...),
        kgo.FetchMinBytes(1),
        kgo.FetchMaxWait(500*time.Millisecond),
        kgo.RequireStableFetchOffsets(),     // exactly-once
        kgo.DisableAutoCommit(),             // manual commit
    )
    if err != nil {
        return nil, err
    }
    return &EventConsumer{client: client}, nil
}

func (c *EventConsumer) Start(ctx context.Context, handler EventHandler) error {
    for {
        fetches := c.client.PollFetches(ctx)
        if errs := fetches.Errors(); len(errs) > 0 {
            for _, err := range errs {
                log.Error("fetch error",
                    "topic", err.Topic,
                    "partition", err.Partition,
                    "error", err.Err,
                )
            }
        }

        fetches.EachRecord(func(record *kgo.Record) {
            if err := c.processRecord(ctx, record, handler); err != nil {
                log.Error("process error",
                    "topic", record.Topic,
                    "offset", record.Offset,
                    "error", err,
                )
                // НЕ коммитим offset при ошибке → retry
                return
            }
        })

        // Коммит offset после успешной обработки
        if err := c.client.CommitUncommittedOffsets(ctx); err != nil {
            log.Error("commit error", "error", err)
        }
    }
}

func (c *EventConsumer) processRecord(
    ctx context.Context, 
    record *kgo.Record, 
    handler EventHandler,
) error {
    // Извлечь event_type из headers
    var eventType string
    for _, h := range record.Headers {
        if h.Key == "event_type" {
            eventType = string(h.Value)
        }
    }

    switch record.Topic {
    case "bets.bet.placed":
        var event BetPlacedEvent
        if err := proto.Unmarshal(record.Value, &event); err != nil {
            return fmt.Errorf("unmarshal BetPlacedEvent: %w", err)
        }
        return handler.HandleBetPlaced(ctx, &event)

    case "bets.bet.settled":
        var event BetSettledEvent
        if err := proto.Unmarshal(record.Value, &event); err != nil {
            return fmt.Errorf("unmarshal BetSettledEvent: %w", err)
        }
        return handler.HandleBetSettled(ctx, &event)

    case "payments.deposit.completed":
        var event DepositCompletedEvent
        if err := proto.Unmarshal(record.Value, &event); err != nil {
            return fmt.Errorf("unmarshal: %w", err)
        }
        return handler.HandleDepositCompleted(ctx, &event)

    default:
        log.Warn("unknown topic", "topic", record.Topic)
        return nil
    }
}
EVENT HANDLER PATTERN
Go

// handler interface
type EventHandler interface {
    HandleBetPlaced(ctx context.Context, event *BetPlacedEvent) error
    HandleBetSettled(ctx context.Context, event *BetSettledEvent) error
    HandleDepositCompleted(ctx context.Context, event *DepositCompletedEvent) error
}

// notification handler — отправляет уведомления
type NotificationHandler struct {
    notifService *NotificationService
}

func (h *NotificationHandler) HandleBetSettled(
    ctx context.Context, event *BetSettledEvent,
) error {
    if event.Result == "won" && event.ActualWin.Amount > 100 {
        return h.notifService.SendPush(ctx, event.UserId, Notification{
            Title:   "Congratulations! 🎉",
            Body:    fmt.Sprintf("You won %s!", FormatMoney(event.ActualWin)),
            Channel: "bet_results",
        })
    }
    return nil
}

// analytics handler — пишет в ClickHouse
type AnalyticsHandler struct {
    clickhouse *sql.DB
}

func (h *AnalyticsHandler) HandleBetPlaced(
    ctx context.Context, event *BetPlacedEvent,
) error {
    _, err := h.clickhouse.ExecContext(ctx,
        `INSERT INTO bet_events (event_time, user_id, bet_id, action, 
         sport, stake, odds, currency, device, ip)
         VALUES (?, ?, ?, 'placed', ?, ?, ?, ?, ?, ?)`,
        event.Timestamp.AsTime(),
        event.UserId,
        event.BetId,
        event.Selections[0].EventName, // simplified
        event.Stake.Amount,
        event.TotalOdds,
        event.Currency,
        event.Device,
        event.IpAddress,
    )
    return err
}
IDEMPOTENT CONSUMERS
Go

// ✅ ПРАВИЛЬНО: idempotent consumer с DragonflyDB
func (h *Handler) HandleBetPlaced(ctx context.Context, event *BetPlacedEvent) error {
    // 1. Check if already processed
    processedKey := fmt.Sprintf("processed:bet_placed:%s", event.EventId)
    wasSet, err := h.cache.SetNX(ctx, processedKey, "1", 24*time.Hour).Result()
    if err != nil {
        return err
    }
    if !wasSet {
        // Already processed — skip
        log.Info("duplicate event, skipping", "event_id", event.EventId)
        return nil
    }

    // 2. Process event
    if err := h.processEvent(ctx, event); err != nil {
        // Rollback: удалить ключ чтобы retry сработал
        h.cache.Del(ctx, processedKey)
        return err
    }

    return nil
}
DEAD LETTER QUEUE
Go

// ✅ Если обработка падает N раз → отправить в DLQ
func (c *EventConsumer) processWithRetry(
    ctx context.Context, record *kgo.Record, handler EventHandler,
) error {
    var retryCount int
    for _, h := range record.Headers {
        if h.Key == "retry_count" {
            retryCount, _ = strconv.Atoi(string(h.Value))
        }
    }

    err := c.processRecord(ctx, record, handler)
    if err != nil {
        if retryCount >= 3 {
            // Send to DLQ
            dlqRecord := &kgo.Record{
                Topic:   record.Topic + ".dlq",
                Key:     record.Key,
                Value:   record.Value,
                Headers: append(record.Headers, kgo.RecordHeader{
                    Key:   "error",
                    Value: []byte(err.Error()),
                }),
            }
            c.client.Produce(ctx, dlqRecord, nil)
            log.Error("sent to DLQ", "topic", record.Topic, "error", err)
            return nil  // не retry больше
        }
        return err  // retry
    }
    return nil
}
АНТИПАТТЕРНЫ
Go

// ❌ ПЛОХО: fire-and-forget без acks
kgo.ProducerBatchCompression(kgo.NoCompression())
// Потеря сообщений при падении брокера

// ✅ ПРАВИЛЬНО: acks=all + idempotence
// В конфиге producer: acks=all, enable.idempotence=true

// ❌ ПЛОХО: один consumer group на всё
group = "main-consumer"  // все сервисы в одной группе

// ✅ ПРАВИЛЬНО: отдельный consumer group на каждый сервис
group = "notification-service"
group = "analytics-service"
group = "fraud-engine"

// ❌ ПЛОХО: auto-commit offsets
kgo.EnableAutoCommit()  // может коммитнуть до обработки

// ✅ ПРАВИЛЬНО: manual commit после успешной обработки

// ❌ ПЛОХО: огромные сообщения
// Вложить весь PDF документ в event ❌

// ✅ ПРАВИЛЬНО: event содержит ссылку
message KYCDocumentUploaded {
    int64 user_id = 1;
    string document_url = 2;  // ссылка на S3
    string document_type = 3;
}

// ❌ ПЛОХО: бизнес-логика зависит от порядка между разными топиками
// "Дождаться deposit.completed перед обработкой bet.placed"

// ✅ ПРАВИЛЬНО: каждый consumer обрабатывает свой топик независимо
// Порядок гарантирован только внутри одной партиции одного топика
ORDERING GUARANTEES
text

Гарантии Redpanda (Kafka):
- Порядок сохраняется ВНУТРИ одной партиции
- Key определяет партицию

Правило: events одного пользователя → одна партиция
  Key = user_id
  → Все ставки пользователя в порядке
  → deposit → bet → settlement в порядке

НЕ гарантировано:
  Порядок между разными user_id
  Порядок между разными топиками
МОНИТОРИНГ
YAML

# Ключевые метрики Redpanda
consumer_lag:
  description: "Разница между latest offset и consumer offset"
  alert: "lag > 10000 messages для > 5 минут"
  severity: P2

produce_latency_p99:
  description: "Задержка записи"
  alert: "> 100ms"
  severity: P2

consumer_errors:
  description: "Ошибки обработки"
  alert: "rate > 10/min"
  severity: P1

dlq_messages:
  description: "Сообщения в Dead Letter Queue"
  alert: "любое сообщение в DLQ"
  severity: P2