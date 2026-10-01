-- ClickHouse Schema for Affiliate Analytics
-- Optimized for high-volume aggregations and time-series queries

-- ==============================================================================
-- 1. Affiliate Clicks (raw events)
-- ==============================================================================
CREATE TABLE IF NOT EXISTS affiliate_clicks_raw (
    click_id String,
    affiliate_id String,
    link_id Nullable(String),
    ip_hash String,
    user_agent_hash String,
    device_fingerprint Nullable(String),
    country_code LowCardinality(String),
    landing_page String,
    campaign Nullable(String),
    created_at DateTime64(3)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(created_at)
ORDER BY (affiliate_id, created_at, click_id)
TTL created_at + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

-- ==============================================================================
-- 2. Affiliate Attributions (registrations from clicks)
-- ==============================================================================
CREATE TABLE IF NOT EXISTS affiliate_attributions (
    attribution_id String,
    affiliate_id String,
    referred_user_id Int64,
    click_id String,
    attribution_model LowCardinality(String),
    attributed_at DateTime64(3),
    ftd_at Nullable(DateTime64(3)),
    is_ftd_qualified UInt8,
    created_at DateTime64(3)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(created_at)
ORDER BY (affiliate_id, attributed_at, attribution_id)
SETTINGS index_granularity = 8192;

-- ==============================================================================
-- 3. Affiliate Daily Aggregates (materialized for fast dashboard queries)
-- ==============================================================================
CREATE TABLE IF NOT EXISTS affiliate_daily_aggregates (
    affiliate_id String,
    report_date Date,
    clicks UInt64,
    registrations UInt64,
    ftd_count UInt64,
    active_players UInt64,
    ggr_amount Decimal(18, 8),
    ngr_amount Decimal(18, 8),
    commission_accrued Decimal(18, 8),
    commission_released Decimal(18, 8),
    commission_reversed Decimal(18, 8),
    currency LowCardinality(String),
    updated_at DateTime64(3)
)
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(report_date)
ORDER BY (affiliate_id, report_date)
SETTINGS index_granularity = 8192;

-- ==============================================================================
-- 4. Affiliate Earnings (financial records)
-- ==============================================================================
CREATE TABLE IF NOT EXISTS affiliate_earnings (
    earning_id String,
    affiliate_id String,
    referred_user_id Int64,
    source_type LowCardinality(String),
    source_id String,
    period_start DateTime64(3),
    period_end DateTime64(3),
    ggr_amount Decimal(18, 8),
    ngr_amount Decimal(18, 8),
    commission_rate Decimal(10, 4),
    commission_amount Decimal(18, 8),
    status LowCardinality(String),
    hold_until DateTime64(3),
    created_at DateTime64(3)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(created_at)
ORDER BY (affiliate_id, created_at, earning_id)
SETTINGS index_granularity = 8192;

-- ==============================================================================
-- 5. Affiliate Payouts
-- ==============================================================================
CREATE TABLE IF NOT EXISTS affiliate_payouts (
    payout_id String,
    affiliate_id String,
    amount Decimal(18, 8),
    currency LowCardinality(String),
    status LowCardinality(String),
    requested_at DateTime64(3),
    approved_at Nullable(DateTime64(3)),
    provider_reference Nullable(String),
    created_at DateTime64(3)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(created_at)
ORDER BY (affiliate_id, requested_at, payout_id)
SETTINGS index_granularity = 8192;

-- ==============================================================================
-- 6. Referred Player Activity (for LTV and retention calculations)
-- ==============================================================================
CREATE TABLE IF NOT EXISTS referred_player_activity (
    referred_user_id Int64,
    affiliate_id String,
    activity_date Date,
    bets_count UInt64,
    bets_amount Decimal(18, 8),
    wins_amount Decimal(18, 8),
    ggr Decimal(18, 8),
    deposits_count UInt64,
    deposits_amount Decimal(18, 8),
    withdrawals_count UInt64,
    withdrawals_amount Decimal(18, 8),
    bonuses_received Decimal(18, 8),
    currency LowCardinality(String)
)
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(activity_date)
ORDER BY (affiliate_id, referred_user_id, activity_date)
SETTINGS index_granularity = 8192;

-- ==============================================================================
-- 7. Affiliate Fraud Signals (for risk scoring)
-- ==============================================================================
CREATE TABLE IF NOT EXISTS affiliate_fraud_signals (
    signal_id String,
    affiliate_id String,
    referred_user_id Int64,
    signal_type LowCardinality(String),
    severity LowCardinality(String),
    details String, -- JSON
    detected_at DateTime64(3)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(detected_at)
ORDER BY (affiliate_id, detected_at, signal_id)
TTL detected_at + INTERVAL 365 DAY
SETTINGS index_granularity = 8192;

-- ==============================================================================
-- 8. Conversion Funnel (hourly aggregates)
-- ==============================================================================
CREATE TABLE IF NOT EXISTS affiliate_funnel_hourly (
    affiliate_id String,
    hour DateTime,
    clicks UInt64,
    registrations UInt64,
    ftds UInt64,
    active_players UInt64,
    conversion_click_to_reg Float32,
    conversion_reg_to_ftd Float32,
    conversion_ftd_to_active Float32
)
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(hour)
ORDER BY (affiliate_id, hour)
TTL hour + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;

-- ==============================================================================
-- VIEWS for Dashboard Queries
-- ==============================================================================

-- Daily summary view
CREATE MATERIALIZED VIEW IF NOT EXISTS affiliate_daily_summary_mv
TO affiliate_daily_aggregates AS
SELECT
    affiliate_id,
    toDate(created_at) as report_date,
    count() as clicks,
    0 as registrations,
    0 as ftd_count,
    0 as active_players,
    toDecimal64(0, 8) as ggr_amount,
    toDecimal64(0, 8) as ngr_amount,
    toDecimal64(0, 8) as commission_accrued,
    toDecimal64(0, 8) as commission_released,
    toDecimal64(0, 8) as commission_reversed,
    'USD' as currency,
    now64(3) as updated_at
FROM affiliate_clicks_raw
GROUP BY affiliate_id, report_date;

-- ==============================================================================
-- QUERIES for Analytics
-- ==============================================================================

-- Top affiliates by NGR (last 30 days)
-- SELECT
--     affiliate_id,
--     sum(ngr_amount) as total_ngr,
--     sum(commission_accrued) as total_commission
-- FROM affiliate_daily_aggregates
-- WHERE report_date >= today() - 30
-- GROUP BY affiliate_id
-- ORDER BY total_ngr DESC
-- LIMIT 100;

-- Affiliate conversion funnel (last 7 days)
-- SELECT
--     affiliate_id,
--     sum(clicks) as total_clicks,
--     sum(registrations) as total_registrations,
--     sum(ftd_count) as total_ftds,
--     total_registrations / total_clicks as click_to_reg_rate,
--     total_ftds / total_registrations as reg_to_ftd_rate
-- FROM affiliate_daily_aggregates
-- WHERE report_date >= today() - 7
-- GROUP BY affiliate_id
-- ORDER BY total_ftds DESC;

-- Player LTV by affiliate
-- SELECT
--     affiliate_id,
--     referred_user_id,
--     sum(ggr) as lifetime_ggr,
--     count() as active_days
-- FROM referred_player_activity
-- GROUP BY affiliate_id, referred_user_id
-- ORDER BY lifetime_ggr DESC
-- LIMIT 1000;

-- Cohort retention (weekly)
-- SELECT
--     affiliate_id,
--     toStartOfWeek(activity_date) as week,
--     uniqExact(referred_user_id) as active_players
-- FROM referred_player_activity
-- GROUP BY affiliate_id, week
-- ORDER BY affiliate_id, week;
