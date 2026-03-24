-- ClickHouse Schema: 001_analytics.sql
-- Description: Analytics tables, Kafka ingestion, Materialized Views
-- Author: DATA_ENGINEER
-- Date: 2026-03-24
-- Cluster: main_cluster (3 shards × 2 replicas)

-- ============================================================
-- LOGS TABLE (from Vector)
-- ============================================================

CREATE TABLE platform_logs ON CLUSTER 'main_cluster'
(
    timestamp     DateTime64(3),
    level         Enum8('debug' = 0, 'info' = 1, 'warn' = 2, 'error' = 3, 'fatal' = 4),
    service       LowCardinality(String),
    message       String,
    trace_id      String,
    span_id       String,
    user_id       Nullable(UInt64),
    request_id    String,
    metadata      String  -- JSON string
)
ENGINE = ReplicatedMergeTree('/clickhouse/tables/{shard}/platform_logs', '{replica}')
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (service, level, timestamp)
TTL timestamp + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;

-- Fast error lookup
ALTER TABLE platform_logs ON CLUSTER 'main_cluster'
    ADD INDEX idx_logs_level level TYPE set(10) GRANULARITY 1;
ALTER TABLE platform_logs ON CLUSTER 'main_cluster'
    ADD INDEX idx_logs_trace trace_id TYPE bloom_filter(0.01) GRANULARITY 1;

-- ============================================================
-- BET EVENTS (analytics)
-- ============================================================

CREATE TABLE bet_events ON CLUSTER 'main_cluster'
(
    event_time    DateTime64(3),
    user_id       UInt64,
    bet_id        UInt64,
    action        Enum8('placed' = 1, 'settled' = 2, 'cashout' = 3, 'void' = 4, 'rejected' = 5),
    bet_type      LowCardinality(String),     -- single, accumulator, system
    sport         LowCardinality(String),
    league        LowCardinality(String),
    stake         Decimal64(8),
    odds          Decimal64(6),
    currency      LowCardinality(FixedString(3)),
    result        Nullable(Enum8('win' = 1, 'loss' = 2)),
    pnl           Nullable(Decimal64(8)),
    country       LowCardinality(FixedString(2)),
    device        LowCardinality(String),     -- mobile, desktop, tablet
    session_id    String,
    ip            IPv4
)
ENGINE = ReplicatedMergeTree('/clickhouse/tables/{shard}/bet_events', '{replica}')
PARTITION BY toYYYYMM(event_time)
ORDER BY (user_id, event_time)
TTL event_time + INTERVAL 3 YEAR
SETTINGS index_granularity = 8192;

-- ============================================================
-- USER EVENTS (behavioral analytics)
-- ============================================================

CREATE TABLE user_events ON CLUSTER 'main_cluster'
(
    event_time    DateTime64(3),
    user_id       UInt64,
    event_type    LowCardinality(String),
      -- login, logout, deposit, withdrawal, bet, game_start,
      -- profile_update, kyc_submit, bonus_claim, etc.
    properties    String,  -- JSON
    ip            IPv4,
    user_agent    String,
    country       LowCardinality(FixedString(2)),
    device        LowCardinality(String),
    session_id    String
)
ENGINE = ReplicatedMergeTree('/clickhouse/tables/{shard}/user_events', '{replica}')
PARTITION BY toYYYYMM(event_time)
ORDER BY (event_type, user_id, event_time)
TTL event_time + INTERVAL 1 YEAR
SETTINGS index_granularity = 8192;

-- ============================================================
-- CASINO ROUNDS (game analytics + RTP monitoring)
-- ============================================================

CREATE TABLE casino_rounds ON CLUSTER 'main_cluster'
(
    event_time      DateTime64(3),
    user_id         UInt64,
    round_id        String,
    game_id         UInt32,
    game_name       LowCardinality(String),
    provider        LowCardinality(String),
    category        LowCardinality(String),   -- slots, table, live
    bet_amount      Decimal64(8),
    win_amount      Decimal64(8),
    currency        LowCardinality(FixedString(3)),
    rounds_played   UInt32,
    session_id      String,
    device          LowCardinality(String),
    country         LowCardinality(FixedString(2))
)
ENGINE = ReplicatedMergeTree('/clickhouse/tables/{shard}/casino_rounds', '{replica}')
PARTITION BY toYYYYMM(event_time)
ORDER BY (user_id, event_time)
TTL event_time + INTERVAL 2 YEAR
SETTINGS index_granularity = 8192;

-- ============================================================
-- FRAUD SIGNALS
-- ============================================================

CREATE TABLE fraud_signals ON CLUSTER 'main_cluster'
(
    event_time      DateTime64(3),
    user_id         UInt64,
    signal_type     LowCardinality(String),
    risk_score      Float32,
    is_fraud        UInt8,
    model_version   LowCardinality(String),
    features        String,  -- JSON
    action_taken    LowCardinality(String),  -- 'none', 'flagged', 'blocked'
    ip              IPv4,
    country         LowCardinality(FixedString(2))
)
ENGINE = ReplicatedMergeTree('/clickhouse/tables/{shard}/fraud_signals', '{replica}')
PARTITION BY toYYYYMM(event_time)
ORDER BY (user_id, event_time)
TTL event_time + INTERVAL 2 YEAR
SETTINGS index_granularity = 8192;

-- ============================================================
-- FINANCIAL REPORTS (aggregated)
-- ============================================================

CREATE TABLE financial_reports ON CLUSTER 'main_cluster'
(
    report_date     Date,
    currency        LowCardinality(FixedString(3)),
    total_deposits  Decimal64(8),
    total_withdrawals Decimal64(8),
    total_bets      Decimal64(8),
    total_payouts   Decimal64(8),
    ggr             Decimal64(8),  -- Gross Gaming Revenue
    bonus_cost      Decimal64(8),
    net_revenue     Decimal64(8),
    unique_depositors UInt32,
    unique_bettors  UInt32,
    new_registrations UInt32
)
ENGINE = ReplicatedSummingMergeTree('/clickhouse/tables/{shard}/financial_reports', '{replica}')
PARTITION BY toYYYYMM(report_date)
ORDER BY (report_date, currency)
TTL report_date + INTERVAL 5 YEAR;

-- ============================================================
-- KAFKA ENGINE TABLES (consume from Redpanda)
-- ============================================================

-- Bet events queue
CREATE TABLE bet_events_queue ON CLUSTER 'main_cluster'
(
    event_time    DateTime64(3),
    user_id       UInt64,
    bet_id        UInt64,
    action        String,
    bet_type      String,
    sport         String,
    league        String,
    stake         Float64,
    odds          Float64,
    currency      String,
    result        Nullable(String),
    pnl           Nullable(Float64),
    country       String,
    device        String,
    session_id    String,
    ip            String
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'redpanda-0.redpanda.data.svc.cluster.local:9092,redpanda-1.redpanda.data.svc.cluster.local:9092,redpanda-2.redpanda.data.svc.cluster.local:9092',
    kafka_topic_list = 'bets.bet.placed,bets.bet.settled,bets.bet.cashout',
    kafka_group_name = 'clickhouse_bet_events',
    kafka_format = 'JSONEachRow',
    kafka_num_consumers = 3,
    kafka_max_block_size = 65536,
    kafka_poll_timeout_ms = 500;

-- User events queue
CREATE TABLE user_events_queue ON CLUSTER 'main_cluster'
(
    event_time    DateTime64(3),
    user_id       UInt64,
    event_type    String,
    properties    String,
    ip            String,
    user_agent    String,
    country       String,
    device        String,
    session_id    String
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'redpanda-0.redpanda.data.svc.cluster.local:9092,redpanda-1.redpanda.data.svc.cluster.local:9092,redpanda-2.redpanda.data.svc.cluster.local:9092',
    kafka_topic_list = 'analytics.events',
    kafka_group_name = 'clickhouse_user_events',
    kafka_format = 'JSONEachRow',
    kafka_num_consumers = 3,
    kafka_max_block_size = 65536;

-- Casino rounds queue
CREATE TABLE casino_rounds_queue ON CLUSTER 'main_cluster'
(
    event_time      DateTime64(3),
    user_id         UInt64,
    round_id        String,
    game_id         UInt32,
    game_name       String,
    provider        String,
    category        String,
    bet_amount      Float64,
    win_amount      Float64,
    currency        String,
    rounds_played   UInt32,
    session_id      String,
    device          String,
    country         String
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'redpanda-0.redpanda.data.svc.cluster.local:9092,redpanda-1.redpanda.data.svc.cluster.local:9092,redpanda-2.redpanda.data.svc.cluster.local:9092',
    kafka_topic_list = 'casino.round.completed',
    kafka_group_name = 'clickhouse_casino_rounds',
    kafka_format = 'JSONEachRow',
    kafka_num_consumers = 2,
    kafka_max_block_size = 65536;

-- ============================================================
-- MATERIALIZED VIEWS (Kafka → ReplicatedMergeTree)
-- ============================================================

-- Bet events: Kafka queue → analytics table
CREATE MATERIALIZED VIEW bet_events_mv ON CLUSTER 'main_cluster'
TO bet_events AS
SELECT
    event_time,
    user_id,
    bet_id,
    multiIf(
        action = 'placed', 'placed',
        action = 'settled', 'settled',
        action = 'cashout', 'cashout',
        action = 'void', 'void',
        'rejected'
    ) AS action,
    bet_type,
    sport,
    league,
    toDecimal64(stake, 8) AS stake,
    toDecimal64(odds, 6) AS odds,
    currency,
    if(result IS NOT NULL, CAST(result AS Enum8('win' = 1, 'loss' = 2)), NULL) AS result,
    toDecimal64OrNull(pnl, 8) AS pnl,
    country,
    device,
    session_id,
    toIPv4OrDefault(ip) AS ip
FROM bet_events_queue;

-- User events: Kafka queue → analytics table
CREATE MATERIALIZED VIEW user_events_mv ON CLUSTER 'main_cluster'
TO user_events AS
SELECT
    event_time,
    user_id,
    event_type,
    properties,
    toIPv4OrDefault(ip) AS ip,
    user_agent,
    country,
    device,
    session_id
FROM user_events_queue;

-- Casino rounds: Kafka queue → analytics table
CREATE MATERIALIZED VIEW casino_rounds_mv ON CLUSTER 'main_cluster'
TO casino_rounds AS
SELECT
    event_time,
    user_id,
    round_id,
    game_id,
    game_name,
    provider,
    category,
    toDecimal64(bet_amount, 8) AS bet_amount,
    toDecimal64(win_amount, 8) AS win_amount,
    currency,
    rounds_played,
    session_id,
    device,
    country
FROM casino_rounds_queue;

-- ============================================================
-- PRE-AGGREGATED MATERIALIZED VIEWS (SummingMergeTree)
-- ============================================================

-- Hourly bet stats by sport + country
CREATE TABLE hourly_bet_stats ON CLUSTER 'main_cluster'
(
    hour          DateTime,
    sport         LowCardinality(String),
    country       LowCardinality(FixedString(2)),
    bets_count    UInt64,
    unique_users  AggregateFunction(uniq, UInt64),
    total_stake   Decimal64(8),
    total_payouts Decimal64(8)
)
ENGINE = ReplicatedAggregatingMergeTree('/clickhouse/tables/{shard}/hourly_bet_stats', '{replica}')
PARTITION BY toYYYYMM(hour)
ORDER BY (hour, sport, country);

CREATE MATERIALIZED VIEW hourly_bet_stats_mv ON CLUSTER 'main_cluster'
TO hourly_bet_stats AS
SELECT
    toStartOfHour(event_time) AS hour,
    sport,
    country,
    count() AS bets_count,
    uniqState(user_id) AS unique_users,
    sum(stake) AS total_stake,
    sumIf(pnl, result = 'win') AS total_payouts
FROM bet_events
WHERE action = 'settled'
GROUP BY hour, sport, country;

-- Daily GGR
CREATE TABLE daily_ggr ON CLUSTER 'main_cluster'
(
    date              Date,
    sport             LowCardinality(String),
    bets_count        UInt64,
    unique_bettors    AggregateFunction(uniq, UInt64),
    total_stake       Decimal64(8),
    total_payouts     Decimal64(8),
    ggr               Decimal64(8)
)
ENGINE = ReplicatedAggregatingMergeTree('/clickhouse/tables/{shard}/daily_ggr', '{replica}')
PARTITION BY toYYYYMM(date)
ORDER BY (date, sport);

CREATE MATERIALIZED VIEW daily_ggr_mv ON CLUSTER 'main_cluster'
TO daily_ggr AS
SELECT
    toDate(event_time) AS date,
    sport,
    count() AS bets_count,
    uniqState(user_id) AS unique_bettors,
    sum(stake) AS total_stake,
    sumIf(abs(pnl), result = 'win') AS total_payouts,
    sum(stake) - sumIf(abs(pnl), result = 'win') AS ggr
FROM bet_events
WHERE action = 'settled'
GROUP BY date, sport;
