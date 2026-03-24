# SKILL #3 — event-driven-design.skill.md

```markdown
# event-driven-design.skill.md
# GAMBLING PLATFORM — EVENT-DRIVEN DESIGN
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: All Backend Agents, Data Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

The platform uses event-driven architecture for cross-service
communication, analytics pipelines, and audit logging.

Broker: Redpanda (Kafka API compatible, no JVM).
Format: Protobuf (with Schema Registry).
Delivery: At-least-once (consumers must be idempotent).

# ============================================================
# SECTION 2: TOPIC CATALOG
# ============================================================

```text
DOMAIN EVENTS (facts about what happened):
  bets.placed           — bet created and funds locked
  bets.settled          — bet result determined, payout processed
  bets.cashout          — bet cashed out early
  payments.initiated    — payment request created
  payments.completed    — payment confirmed by PSP
  payments.failed       — payment failed
  users.registered      — new user account created
  users.updated         — user profile changed
  users.blocked         — user account blocked
  users.self_excluded   — user self-excluded
  kyc.verified          — KYC level upgraded
  kyc.rejected          — KYC verification failed
  bonuses.claimed       — user claimed a bonus
  bonuses.completed     — wagering requirement met
  events.odds_updated   — odds changed for an event
  events.resulted       — sporting event result confirmed
  casino.game_started   — casino game session started
  casino.game_ended     — casino game session ended

COMMANDS (fire-and-forget tasks):
  notifications.send    — send notification via channel
  audit.log             — append to immutable audit log

INTERNAL (operational):
  fraud.signals         — fraud detection signals
  analytics.events      — raw analytics events
TOPIC CONFIGURATION
text

PARTITIONING:
  bets.*:             32 partitions, key = user_id
  payments.*:         16 partitions, key = user_id
  users.*:            16 partitions, key = user_id
  events.odds_updated: 32 partitions, key = event_id
  events.resulted:    8 partitions, key = event_id
  notifications.send: 8 partitions, key = user_id
  analytics.events:   16 partitions, key = event_type
  audit.log:          8 partitions, key = service_name

WHY key matters:
  Same key → same partition → ordered processing
  All bets for user X processed in order (no race conditions)
  All odds for event Y processed in order (consistent state)

REPLICATION: factor = 3 (survive 1 broker failure)
MIN IN-SYNC REPLICAS: 2 (producer acks = all)
RETENTION:
  Operational: 7 days
  Audit: 90 days
  Analytics: 30 days
============================================================
SECTION 3: EVENT SCHEMA
============================================================
PROTOBUF EVENT STRUCTURE
protobuf

// proto/events/v1/bet_events.proto
syntax = "proto3";
package events.v1;

import "google/protobuf/timestamp.proto";

message BetPlacedEvent {
  int64 bet_id = 1;
  int64 user_id = 2;
  string bet_type = 3;          // "single", "accumulator"
  string stake = 4;             // decimal as string "50.00"
  string odds = 5;              // combined odds "2.50"
  string potential_win = 6;     // "125.00"
  string currency = 7;          // "USD"
  int32 selections_count = 8;
  string country = 9;           // user country
  string device = 10;           // "web", "ios", "android"
  google.protobuf.Timestamp placed_at = 11;
}

message BetSettledEvent {
  int64 bet_id = 1;
  int64 user_id = 2;
  string result = 3;            // "won", "lost", "void", "cashout"
  string stake = 4;
  string payout = 5;            // "0.00" for loss
  string currency = 6;
  google.protobuf.Timestamp settled_at = 7;
}
MESSAGE HEADERS (every message)
text

REQUIRED HEADERS:
  trace_id:        OpenTelemetry trace ID (propagation)
  correlation_id:  business flow ID (e.g., bet_id for bet flow)
  produced_at:     ISO 8601 timestamp
  producer:        service name ("betting-engine")
  event_type:      fully qualified type ("events.v1.BetPlacedEvent")
  event_version:   schema version ("1.0")
  idempotency_key: unique per event instance
============================================================
SECTION 4: PRODUCER PATTERNS
============================================================
Rust

// Rust producer pattern
pub struct EventProducer {
    client: rdkafka::producer::FutureProducer,
}

impl EventProducer {
    /// Publish event with headers and key.
    /// Non-blocking, returns after broker acknowledgment.
    pub async fn publish<T: prost::Message>(
        &self,
        topic: &str,
        key: &str,
        event: &T,
        trace_id: &str,
    ) -> Result<(), EventError> {
        let payload = event.encode_to_vec();
        
        let record = FutureRecord::to(topic)
            .key(key)
            .payload(&payload)
            .headers(
                OwnedHeaders::new()
                    .insert(Header { key: "trace_id", value: Some(trace_id.as_bytes()) })
                    .insert(Header { key: "producer", value: Some(b"betting-engine") })
                    .insert(Header { key: "produced_at", value: Some(Utc::now().to_rfc3339().as_bytes()) })
                    .insert(Header { key: "event_type", value: Some(std::any::type_name::<T>().as_bytes()) })
            );
        
        self.client.send(record, Duration::from_secs(5)).await
            .map_err(|(err, _)| EventError::PublishFailed(err.to_string()))?;
        
        Ok(())
    }
}
text

PRODUCER RULES:
  1. acks = all (wait for all replicas)
  2. retries = 3 (with backoff)
  3. enable.idempotence = true (exactly-once at producer level)
  4. Events published AFTER database commit (not inside transaction)
  5. If publish fails → log error, do NOT rollback DB transaction
     (eventual consistency via outbox pattern if needed)
  6. NEVER block main operation on event publish failure
============================================================
SECTION 5: CONSUMER PATTERNS
============================================================
Go

// Go consumer pattern
func (c *AnalyticsConsumer) Start(ctx context.Context) error {
    topics := []string{"bets.placed", "bets.settled", "payments.completed"}
    
    client, _ := kgo.NewClient(
        kgo.SeedBrokers(c.cfg.Brokers...),
        kgo.ConsumerGroup("analytics-service"),
        kgo.ConsumeTopics(topics...),
        kgo.DisableAutoCommit(),
    )
    defer client.Close()

    for {
        fetches := client.PollFetches(ctx)
        if ctx.Err() != nil {
            return ctx.Err()
        }
        
        fetches.EachPartition(func(p kgo.FetchTopicPartition) {
            for _, record := range p.Records {
                if err := c.processRecord(ctx, record); err != nil {
                    log.Error().Err(err).
                        Str("topic", record.Topic).
                        Int64("offset", record.Offset).
                        Msg("Failed to process record")
                    
                    c.retryCount++
                    if c.retryCount >= 3 {
                        c.sendToDeadLetter(ctx, record)
                        c.retryCount = 0
                    }
                    return
                }
                c.retryCount = 0
            }
            
            // Commit AFTER successful processing
            client.CommitRecords(ctx, p.Records...)
        })
    }
}

func (c *AnalyticsConsumer) processRecord(ctx context.Context, record *kgo.Record) error {
    // Idempotency: check if already processed
    eventID := getHeader(record, "idempotency_key")
    if c.cache.HasProcessed(ctx, eventID) {
        return nil // already handled
    }
    
    // Process based on topic
    switch record.Topic {
    case "bets.placed":
        var event BetPlacedEvent
        proto.Unmarshal(record.Value, &event)
        return c.handleBetPlaced(ctx, &event)
    case "bets.settled":
        // ...
    }
    
    // Mark as processed
    c.cache.MarkProcessed(ctx, eventID, 24*time.Hour)
    return nil
}
text

CONSUMER RULES:
  1. IDEMPOTENT: same event processed twice = same result
  2. Commit offset AFTER successful processing (at-least-once)
  3. Dead letter topic after 3 failed retries
  4. One consumer group per service (independent consumption)
  5. Monitor consumer lag — alert if > 1000 messages behind
  6. NEVER assume event ordering across partitions
  7. ALWAYS handle unknown event types gracefully (log + skip)
  8. Process events in batches where possible (ClickHouse inserts)
============================================================
SECTION 6: OUTBOX PATTERN
============================================================
text

PROBLEM: DB commit succeeds but event publish fails → inconsistency.

SOLUTION: Outbox pattern.
  1. In same DB transaction: insert record + insert into outbox table
  2. Separate process reads outbox and publishes to Redpanda
  3. After successful publish: mark outbox entry as sent

WHEN TO USE:
  ✅ Critical events where loss is unacceptable (payments, bets)
  ✅ When eventual consistency must be guaranteed

WHEN NOT NEEDED:
  ❌ Analytics events (loss acceptable)
  ❌ Notification triggers (can be resent)

OUTBOX TABLE:
  CREATE TABLE outbox (
    id BIGSERIAL PRIMARY KEY,
    topic VARCHAR(100) NOT NULL,
    key VARCHAR(255) NOT NULL,
    payload BYTEA NOT NULL,
    headers JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    retries INT NOT NULL DEFAULT 0
  );
  
  INDEX: idx_outbox_unpublished ON outbox(created_at) WHERE published_at IS NULL;
============================================================
SECTION 7: ANTI-PATTERNS
============================================================
text

❌ NEVER publish event inside DB transaction (commit first, then publish)
❌ NEVER rely on event ordering across different partitions
❌ NEVER put large payloads in events (> 1MB) — use reference + S3
❌ NEVER use events for request-response (use gRPC)
❌ NEVER skip Schema Registry (breaking changes undetected)
❌ NEVER commit offset before processing (at-most-once = data loss)
❌ NEVER create topic per user/entity (topic explosion)
❌ NEVER ignore consumer lag metrics (can indicate stuck consumer)
❌ NEVER process events synchronously if batch is possible
============================================================
SECTION 8: TESTING
============================================================
text

MUST TEST:
  ✅ Producer publishes to correct topic with correct key
  ✅ Consumer processes event and commits offset
  ✅ Consumer idempotency: duplicate event = no double processing
  ✅ Dead letter: event goes to DLQ after 3 failures
  ✅ Schema compatibility: old consumer reads new event (added field)
  ✅ Consumer lag alert triggers when lag > threshold
  ✅ Outbox pattern: event published even if publish initially fails
  ✅ Event headers contain all required fields