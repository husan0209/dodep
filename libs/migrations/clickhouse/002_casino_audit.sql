-- Migration 002: Casino provider callback audit log
-- ClickHouse DDL — idempotent (CREATE TABLE IF NOT EXISTS)

CREATE TABLE IF NOT EXISTS casino_callbacks
(
    timestamp         DateTime64(3, 'UTC'),
    provider          LowCardinality(String),
    callback_type     LowCardinality(String),  -- balance, bet, win, rollback, freespins, jackpot
    transaction_id    String,
    round_id          String,
    player_id         String,
    user_id           Int64,
    game_id           String,
    currency          LowCardinality(String),
    amount            Decimal(18, 8),
    new_balance       Decimal(18, 8),
    signature_valid   Bool,
    is_duplicate      Bool,
    processing_ms     UInt32,
    error_code        LowCardinality(String),
    error_message     String,
    ip_address        String,
    session_id        String,
    created_at        DateTime DEFAULT now()
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (provider, timestamp, transaction_id)
TTL timestamp + INTERVAL 3 YEAR
SETTINGS index_granularity = 8192;

-- Materialized view: daily provider stats
CREATE MATERIALIZED VIEW IF NOT EXISTS casino_provider_daily_stats
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (date, provider, callback_type, currency)
POPULATE
AS SELECT
    toDate(timestamp)       AS date,
    provider,
    callback_type,
    currency,
    countIf(error_code = '')           AS success_count,
    countIf(error_code != '')          AS error_count,
    countIf(is_duplicate)              AS duplicate_count,
    sum(if(callback_type = 'bet', amount, 0))  AS total_bet,
    sum(if(callback_type = 'win', amount, 0))  AS total_win,
    avg(processing_ms)                 AS avg_latency_ms,
    max(processing_ms)                 AS max_latency_ms
FROM casino_callbacks
GROUP BY date, provider, callback_type, currency;

-- Signature invalid alerts view
CREATE MATERIALIZED VIEW IF NOT EXISTS casino_signature_violations
ENGINE = MergeTree()
ORDER BY (timestamp, provider)
TTL timestamp + INTERVAL 30 DAY
POPULATE
AS SELECT
    timestamp,
    provider,
    player_id,
    ip_address,
    error_message
FROM casino_callbacks
WHERE signature_valid = false;
