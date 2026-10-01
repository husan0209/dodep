-- Migration 005: Affiliate lifecycle ingestion (released / adjusted / payout.updated)
-- Closes the ingestion gaps left by 004_affiliate_ingestion.sql:
--   - affiliate.commission.released  -> emitted on every hold release
--     (gorm_repository.go:939) but had no queue, so released_commission /
--     hold-release freshness were invisible in analytics.
--   - affiliate.adjustment.created   -> manual financial corrections
--     (gorm_repository.go:1476); the only audit trail of admin money edits.
--   - affiliate.payout.updated       -> non-terminal payout transitions
--     (approved/reviewing/failed) (gorm_repository.go:781). 004 subscribes to
--     requested/paid/rejected only, so intermediate states never landed.
--
-- Conventions (mirror 004):
--   - ON CLUSTER 'main_cluster', plain CREATE (re-run fails loudly instead of
--     double-consuming; Kafka engine tables hold no state).
--   - Payloads carry NO event timestamps -> Kafka _timestamp for all time fields.
--   - Append-only. New target tables are added by this migration so the
--     released/adjusted history is immutable and reconcilable.
--   - Money arrives as String (Go decimal) -> toDecimal64OrZero(..., 8).
--   - Partial payloads are expected: commission.released has no GGR/NGR/rate,
--     payout.updated has no amount/currency. Totals must join back to the
--     003/004 tables rather than being summed from these rows alone.
--
-- ClickHouse rule applied: no UPDATE/DELETE, no JOINs on large tables,
-- LowCardinality only for genuinely low-cardinality columns.

-- ============================================================
-- TARGET TABLES (append-only event history)
-- ============================================================

-- Hold releases: one row per earning that moved accrued -> available.
CREATE TABLE IF NOT EXISTS affiliate_commission_releases ON CLUSTER 'main_cluster'
(
    earning_id        String,
    affiliate_id      String,
    referred_user_id  Int64,
    commission_amount Decimal(18, 8),
    released_at       DateTime64(3)
)
ENGINE = ReplicatedMergeTree('/clickhouse/tables/{shard}/affiliate_commission_releases', '{replica}')
PARTITION BY toYYYYMM(released_at)
ORDER BY (affiliate_id, released_at, earning_id)
TTL released_at + INTERVAL 3 YEAR
SETTINGS index_granularity = 8192;

-- Manual adjustments: append-only audit of admin-created credits/debits.
CREATE TABLE IF NOT EXISTS affiliate_adjustments_audit ON CLUSTER 'main_cluster'
(
    adjustment_id   String,
    affiliate_id    String,
    adjustment_type LowCardinality(String), -- credit | debit
    amount          Decimal(18, 8),
    reason          String,
    created_by      String,
    created_at      DateTime64(3)
)
ENGINE = ReplicatedMergeTree('/clickhouse/tables/{shard}/affiliate_adjustments_audit', '{replica}')
PARTITION BY toYYYYMM(created_at)
ORDER BY (affiliate_id, created_at, adjustment_id)
TTL created_at + INTERVAL 5 YEAR
SETTINGS index_granularity = 8192;

-- Non-terminal payout transitions (approved / reviewing / failed).
-- Kept separate from affiliate_payouts so the "one row per lifecycle event"
-- table in 004 keeps requested/paid/rejected semantics intact.
CREATE TABLE IF NOT EXISTS affiliate_payout_transitions ON CLUSTER 'main_cluster'
(
    payout_id           String,
    affiliate_id        String,
    status              LowCardinality(String),
    approved_by         String,
    provider_reference  String,
    rejection_reason    String,
    transitioned_at     DateTime64(3)
)
ENGINE = ReplicatedMergeTree('/clickhouse/tables/{shard}/affiliate_payout_transitions', '{replica}')
PARTITION BY toYYYYMM(transitioned_at)
ORDER BY (affiliate_id, transitioned_at, payout_id)
TTL transitioned_at + INTERVAL 3 YEAR
SETTINGS index_granularity = 8192;

-- ============================================================
-- KAFKA ENGINE TABLES
-- ============================================================

CREATE TABLE affiliate_commission_releases_queue ON CLUSTER 'main_cluster'
(
    earning_id        String,
    affiliate_id      String,
    referred_user_id  Int64,
    commission_amount String,
    status            String
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'redpanda-0.redpanda.data.svc.cluster.local:9092,redpanda-1.redpanda.data.svc.cluster.local:9092,redpanda-2.redpanda.data.svc.cluster.local:9092',
    kafka_topic_list = 'affiliate.commission.released',
    kafka_group_name = 'clickhouse_affiliate_releases',
    kafka_format = 'JSONEachRow',
    kafka_num_consumers = 1,
    kafka_max_block_size = 65536;

CREATE TABLE affiliate_adjustments_queue ON CLUSTER 'main_cluster'
(
    adjustment_id   String,
    affiliate_id    String,
    adjustment_type String,
    amount          String,
    reason          String,
    created_by      String
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'redpanda-0.redpanda.data.svc.cluster.local:9092,redpanda-1.redpanda.data.svc.cluster.local:9092,redpanda-2.redpanda.data.svc.cluster.local:9092',
    kafka_topic_list = 'affiliate.adjustment.created',
    kafka_group_name = 'clickhouse_affiliate_adjustments',
    kafka_format = 'JSONEachRow',
    kafka_num_consumers = 1,
    kafka_max_block_size = 65536;

CREATE TABLE affiliate_payout_transitions_queue ON CLUSTER 'main_cluster'
(
    payout_id          String,
    affiliate_id       String,
    status             String,
    approved_by        String,
    provider_reference String,
    rejection_reason   String
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'redpanda-0.redpanda.data.svc.cluster.local:9092,redpanda-1.redpanda.data.svc.cluster.local:9092,redpanda-2.redpanda.data.svc.cluster.local:9092',
    kafka_topic_list = 'affiliate.payout.updated',
    kafka_group_name = 'clickhouse_affiliate_payout_transitions',
    kafka_format = 'JSONEachRow',
    kafka_num_consumers = 1,
    kafka_max_block_size = 65536;

-- ============================================================
-- MATERIALIZED VIEWS
-- ============================================================

CREATE MATERIALIZED VIEW affiliate_commission_releases_mv ON CLUSTER 'main_cluster'
TO affiliate_commission_releases AS
SELECT
    earning_id,
    affiliate_id,
    referred_user_id,
    toDecimal64OrZero(ifNull(nullIf(commission_amount, ''), '0'), 8) AS commission_amount,
    toDateTime64(_timestamp, 3) AS released_at
FROM affiliate_commission_releases_queue;

CREATE MATERIALIZED VIEW affiliate_adjustments_mv ON CLUSTER 'main_cluster'
TO affiliate_adjustments_audit AS
SELECT
    adjustment_id,
    affiliate_id,
    adjustment_type,
    toDecimal64OrZero(ifNull(nullIf(amount, ''), '0'), 8) AS amount,
    reason,
    created_by,
    toDateTime64(_timestamp, 3) AS created_at
FROM affiliate_adjustments_queue;

CREATE MATERIALIZED VIEW affiliate_payout_transitions_mv ON CLUSTER 'main_cluster'
TO affiliate_payout_transitions AS
SELECT
    payout_id,
    affiliate_id,
    status,
    approved_by,
    nullIf(provider_reference, '') AS provider_reference,
    nullIf(rejection_reason, '') AS rejection_reason,
    toDateTime64(_timestamp, 3) AS transitioned_at
FROM affiliate_payout_transitions_queue;

-- ============================================================
-- AGGREGATES (SummingMergeTree — dashboards read these, not raw events)
-- ============================================================

-- Daily released commission per affiliate: drives the "released" panel in
-- grafana-dashboard-affiliate and the AFF-RB-01 staleness check.
CREATE TABLE IF NOT EXISTS affiliate_daily_releases ON CLUSTER 'main_cluster'
(
    affiliate_id        String,
    report_date         Date,
    released_amount     Decimal(18, 8),
    released_earnings   UInt64
)
ENGINE = ReplicatedSummingMergeTree('/clickhouse/tables/{shard}/affiliate_daily_releases', '{replica}')
PARTITION BY toYYYYMM(report_date)
ORDER BY (affiliate_id, report_date)
TTL report_date + INTERVAL 3 YEAR
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW affiliate_daily_releases_mv ON CLUSTER 'main_cluster'
TO affiliate_daily_releases AS
SELECT
    affiliate_id,
    toDate(released_at) AS report_date,
    sum(commission_amount) AS released_amount,
    count() AS released_earnings
FROM affiliate_commission_releases
GROUP BY affiliate_id, report_date;

-- Daily manual adjustments per affiliate (net = credits - debits).
CREATE TABLE IF NOT EXISTS affiliate_daily_adjustments ON CLUSTER 'main_cluster'
(
    affiliate_id    String,
    report_date     Date,
    credited_amount Decimal(18, 8),
    debited_amount  Decimal(18, 8),
    adjustment_count UInt64
)
ENGINE = ReplicatedSummingMergeTree('/clickhouse/tables/{shard}/affiliate_daily_adjustments', '{replica}')
PARTITION BY toYYYYMM(report_date)
ORDER BY (affiliate_id, report_date)
TTL report_date + INTERVAL 5 YEAR
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW affiliate_daily_adjustments_mv ON CLUSTER 'main_cluster'
TO affiliate_daily_adjustments AS
SELECT
    affiliate_id,
    toDate(created_at) AS report_date,
    sumIf(amount, adjustment_type = 'credit') AS credited_amount,
    sumIf(amount, adjustment_type = 'debit') AS debited_amount,
    count() AS adjustment_count
FROM affiliate_adjustments_audit
GROUP BY affiliate_id, report_date;

-- ============================================================
-- Example analytics queries (reference, not executed)
-- ============================================================

-- 1. Hold-release staleness (AFF-RB-01): affiliates with no release for 48h
--    while accruals continue. Subquery, not a JOIN, per ClickHouse doctrine.
-- SELECT a.affiliate_id, max(a.released_at) AS last_release
-- FROM affiliate_daily_releases a
-- WHERE a.report_date >= today() - 30
-- GROUP BY a.affiliate_id
-- HAVING last_release < now() - INTERVAL 48 HOUR
-- ORDER BY last_release;
--
-- 2. Accrued vs released drift per affiliate (30d) — must stay >= 0.
-- SELECT r.affiliate_id, r.released_amount, e.accrued
-- FROM affiliate_daily_releases r
-- LEFT JOIN (SELECT affiliate_id, sum(commission_amount) AS accrued
--            FROM affiliate_earnings
--            WHERE status = 'accrued' AND created_at >= now() - INTERVAL 30 DAY
--            GROUP BY affiliate_id) e USING (affiliate_id)
-- WHERE r.report_date >= today() - 30;
--
-- 3. Manual corrections by admin actor (audit / fraud review).
-- SELECT created_by, count() AS adjustments, sum(amount) AS total
-- FROM affiliate_adjustments_audit
-- WHERE created_at >= now() - INTERVAL 90 DAY
-- GROUP BY created_by
-- ORDER BY total DESC;
--
-- 4. Payouts stuck in a non-terminal state (ageing, cf. AFF-RB-02).
-- SELECT status, count() AS payouts, max(dateDiff('hour', transitioned_at, now())) AS max_age_hours
-- FROM affiliate_payout_transitions
-- WHERE transitioned_at >= now() - INTERVAL 30 DAY
-- GROUP BY status
-- ORDER BY max_age_hours DESC;
--
-- KNOWN GAP (unchanged from 004): no affiliate.player.ftd event is emitted by
-- the service yet, so ftd_at/is_ftd_qualified stay unset. Add a queue+MV here
-- once the backend ships FTD emission.