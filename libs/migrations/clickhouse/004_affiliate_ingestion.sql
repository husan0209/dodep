-- Migration 004: Affiliate Redpanda ingestion (Kafka engine → 003 tables)
-- Consumes the outbox events published by Affiliate Service
-- (see services/go/affiliate/internal/repository/gorm_repository.go appendOutboxTx
-- payloads) into the analytics tables created by 003_affiliate_analytics.sql.
--
-- Conventions (mirror 001_analytics.sql):
--   - ON CLUSTER 'main_cluster', plain CREATE (re-run fails loudly instead of
--     double-consuming; Kafka engine tables hold no state).
--   - Payloads carry NO event timestamps, so MVs use the Kafka _timestamp
--     virtual column for all time fields.
--   - Append-only semantics (platform doctrine: no UPDATE/DELETE in ClickHouse).
--     Mutable lifecycles (payout requested→paid) land as one row per event;
--     aggregate with the example queries below (status-filtered sums).
--     Paid/rejected payloads carry no amount/currency, so lifecycle rows keep
--     amount=0 and empty currency; totals always join back to requested rows.
--   - Partial payloads: commission events carry no GGR/NGR/source/period
--     (zeros/empty, created_at=_timestamp); enriching the publisher payload is
--     a backend follow-up, the MVs need no change when fields appear — use
--     ifNull/empty-string-tolerant expressions below.
--   - KNOWN GAP: no affiliate.player.ftd event is emitted by the service yet
--     (ftd_at/is_ftd_qualified are never set); add a sixth queue+MV here once
--     the backend ships FTD emission.
--
-- ============================================================
-- KAFKA ENGINE TABLES (consume from Redpanda)
-- ============================================================

-- Clicks queue
CREATE TABLE affiliate_clicks_queue ON CLUSTER 'main_cluster'
(
    click_id     String,
    affiliate_id String,
    country_code String,
    landing_page String
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'redpanda-0.redpanda.data.svc.cluster.local:9092,redpanda-1.redpanda.data.svc.cluster.local:9092,redpanda-2.redpanda.data.svc.cluster.local:9092',
    kafka_topic_list = 'affiliate.click.tracked',
    kafka_group_name = 'clickhouse_affiliate_clicks',
    kafka_format = 'JSONEachRow',
    kafka_num_consumers = 3,
    kafka_max_block_size = 65536,
    kafka_poll_timeout_ms = 500;

-- Attributions queue
CREATE TABLE affiliate_attributions_queue ON CLUSTER 'main_cluster'
(
    attribution_id   String,
    affiliate_id     String,
    referred_user_id Int64,
    click_id         String
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'redpanda-0.redpanda.data.svc.cluster.local:9092,redpanda-1.redpanda.data.svc.cluster.local:9092,redpanda-2.redpanda.data.svc.cluster.local:9092',
    kafka_topic_list = 'affiliate.attribution.created',
    kafka_group_name = 'clickhouse_affiliate_attributions',
    kafka_format = 'JSONEachRow',
    kafka_num_consumers = 2,
    kafka_max_block_size = 65536;

-- Commissions queue
CREATE TABLE affiliate_commissions_queue ON CLUSTER 'main_cluster'
(
    earning_id        String,
    affiliate_id      String,
    referred_user_id  Int64,
    commission_amount String,
    commission_rate   String,
    status            String,
    hold_until        String,
    idempotency_key   String
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'redpanda-0.redpanda.data.svc.cluster.local:9092,redpanda-1.redpanda.data.svc.cluster.local:9092,redpanda-2.redpanda.data.svc.cluster.local:9092',
    kafka_topic_list = 'affiliate.commission.accrued',
    kafka_group_name = 'clickhouse_affiliate_commissions',
    kafka_format = 'JSONEachRow',
    kafka_num_consumers = 2,
    kafka_max_block_size = 65536;

-- Payouts queue (requested + paid + rejected lifecycle events)
CREATE TABLE affiliate_payouts_queue ON CLUSTER 'main_cluster'
(
    payout_id          String,
    affiliate_id       String,
    amount             String,
    currency           String,
    status             String,
    provider_reference String,
    rejection_reason   String
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'redpanda-0.redpanda.data.svc.cluster.local:9092,redpanda-1.redpanda.data.svc.cluster.local:9092,redpanda-2.redpanda.data.svc.cluster.local:9092',
    kafka_topic_list = 'affiliate.payout.requested,affiliate.payout.paid,affiliate.payout.rejected',
    kafka_group_name = 'clickhouse_affiliate_payouts',
    kafka_format = 'JSONEachRow',
    kafka_num_consumers = 2,
    kafka_max_block_size = 65536;

-- Fraud flags queue
CREATE TABLE affiliate_fraud_queue ON CLUSTER 'main_cluster'
(
    fraud_flag_id    String,
    affiliate_id     String,
    referred_user_id Int64,
    flag_type        String,
    severity         String,
    status           String
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'redpanda-0.redpanda.data.svc.cluster.local:9092,redpanda-1.redpanda.data.svc.cluster.local:9092,redpanda-2.redpanda.data.svc.cluster.local:9092',
    kafka_topic_list = 'affiliate.fraud.flagged',
    kafka_group_name = 'clickhouse_affiliate_fraud',
    kafka_format = 'JSONEachRow',
    kafka_num_consumers = 2,
    kafka_max_block_size = 65536;

-- ============================================================
-- MATERIALIZED VIEWS (Kafka → 003 tables)
-- ============================================================

-- Clicks: queue → affiliate_clicks_raw
CREATE MATERIALIZED VIEW affiliate_clicks_mv ON CLUSTER 'main_cluster'
TO affiliate_clicks_raw AS
SELECT
    click_id,
    affiliate_id,
    CAST(NULL AS Nullable(String)) AS link_id,
    '' AS ip_hash,
    '' AS user_agent_hash,
    CAST(NULL AS Nullable(String)) AS device_fingerprint,
    country_code,
    landing_page,
    CAST(NULL AS Nullable(String)) AS campaign,
    toDateTime64(_timestamp, 3) AS created_at
FROM affiliate_clicks_queue;

-- Attributions: queue → affiliate_attributions
CREATE MATERIALIZED VIEW affiliate_attributions_mv ON CLUSTER 'main_cluster'
TO affiliate_attributions AS
SELECT
    attribution_id,
    affiliate_id,
    referred_user_id,
    click_id,
    'last_click' AS attribution_model,
    toDateTime64(_timestamp, 3) AS attributed_at,
    CAST(NULL AS Nullable(DateTime64(3))) AS ftd_at,
    0 AS is_ftd_qualified,
    toDateTime64(_timestamp, 3) AS created_at
FROM affiliate_attributions_queue;

-- Commissions: queue → affiliate_earnings
CREATE MATERIALIZED VIEW affiliate_commissions_mv ON CLUSTER 'main_cluster'
TO affiliate_earnings AS
SELECT
    earning_id,
    affiliate_id,
    referred_user_id,
    '' AS source_type,
    '' AS source_id,
    toDateTime64(_timestamp, 3) AS period_start,
    toDateTime64(_timestamp, 3) AS period_end,
    toDecimal64(0, 8) AS ggr_amount,
    toDecimal64(0, 8) AS ngr_amount,
    toDecimal64OrZero(commission_rate, 4) AS commission_rate,
    toDecimal64OrZero(commission_amount, 8) AS commission_amount,
    status,
    toDateTime64(parseDateTimeBestEffortOrNull(hold_until), 3) AS hold_until,
    toDateTime64(_timestamp, 3) AS created_at
FROM affiliate_commissions_queue;

-- Payouts: queue → affiliate_payouts (one row per lifecycle event)
CREATE MATERIALIZED VIEW affiliate_payouts_mv ON CLUSTER 'main_cluster'
TO affiliate_payouts AS
SELECT
    payout_id,
    affiliate_id,
    toDecimal64OrZero(ifNull(nullIf(amount, ''), '0'), 8) AS amount,
    currency,
    status,
    toDateTime64(_timestamp, 3) AS requested_at,
    CAST(NULL AS Nullable(DateTime64(3))) AS approved_at,
    nullIf(provider_reference, '') AS provider_reference,
    toDateTime64(_timestamp, 3) AS created_at
FROM affiliate_payouts_queue;

-- Fraud flags: queue → affiliate_fraud_signals
CREATE MATERIALIZED VIEW affiliate_fraud_mv ON CLUSTER 'main_cluster'
TO affiliate_fraud_signals AS
SELECT
    fraud_flag_id AS signal_id,
    affiliate_id,
    referred_user_id,
    flag_type AS signal_type,
    severity,
    concat('{"flag_id":"', fraud_flag_id, '","status":"', status, '"}') AS details,
    toDateTime64(_timestamp, 3) AS detected_at
FROM affiliate_fraud_queue;

-- ============================================================
-- Example analytics queries (reference, not executed)
-- ============================================================

-- Commission trend per affiliate (accrued only, avoids double counting)
-- SELECT affiliate_id, toDate(created_at) AS day,
--        sum(commission_amount) AS accrued
-- FROM affiliate_earnings
-- WHERE status = 'accrued'
-- GROUP BY affiliate_id, day ORDER BY day;

-- Paid-out totals: join requested amounts to paid lifecycle rows
-- SELECT r.affiliate_id, sum(r.amount) AS paid_total
-- FROM affiliate_payouts r
-- WHERE r.status = 'requested'
--   AND has((SELECT groupUniqArray(payout_id) FROM affiliate_payouts
--            WHERE status = 'paid'), r.payout_id)
-- GROUP BY r.affiliate_id;

-- Click → registration funnel per affiliate (30d)
-- SELECT c.affiliate_id, count() AS clicks,
--        uniqExact(a.referred_user_id) AS registrations
-- FROM affiliate_clicks_raw c
-- LEFT JOIN affiliate_attributions a
--   ON a.click_id = c.click_id AND a.affiliate_id = c.affiliate_id
-- WHERE c.created_at >= now() - INTERVAL 30 DAY
-- GROUP BY c.affiliate_id;
