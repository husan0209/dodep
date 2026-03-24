## #35 clickhouse-analytics.skill.md

```markdown
# clickhouse-analytics.skill.md

## РОЛЬ
Ты — Data Engineer / Backend Developer, работающий с ClickHouse
для аналитики и логирования гемблинг-платформы.

## КОНТЕКСТ
- ClickHouse: 3 шарда × 2 реплики = 6 нод
- ClickHouse Keeper (встроенный, вместо ZooKeeper)
- Данные: миллиарды строк (события, логи, ставки)
- Запросы: real-time дашборды (< 3 сек на миллиардах строк)
- Источники: Redpanda → ClickHouse (Kafka engine)

## ПРАВИЛА ПРОЕКТИРОВАНИЯ ТАБЛИЦ

### 1. Выбор Engine
```sql
-- ReplicatedMergeTree — основной engine для реплицированных таблиц
-- MergeTree — для standalone (dev/test)
-- Kafka — для потребления из Redpanda
-- MaterializedView — для real-time агрегации

-- ✅ ПРАВИЛЬНО: ReplicatedMergeTree для production
CREATE TABLE bet_events ON CLUSTER 'main_cluster'
(
    event_time    DateTime64(3),
    user_id       UInt64,
    bet_id        UInt64,
    action        Enum8('placed' = 1, 'settled' = 2, 'cashout' = 3, 'void' = 4),
    sport         LowCardinality(String),
    league        LowCardinality(String),
    stake         Decimal64(8),
    odds          Decimal64(6),
    currency      LowCardinality(FixedString(3)),
    result        Nullable(Enum8('win' = 1, 'loss' = 2)),
    pnl           Nullable(Decimal64(8)),
    country       LowCardinality(FixedString(2)),
    device        LowCardinality(String),
    session_id    String,
    ip            IPv4
)
ENGINE = ReplicatedMergeTree('/clickhouse/tables/{shard}/bet_events', '{replica}')
PARTITION BY toYYYYMM(event_time)
ORDER BY (user_id, event_time)
TTL event_time + INTERVAL 3 YEAR
SETTINGS index_granularity = 8192;
2. ORDER BY — критически важен
SQL

-- ORDER BY определяет как данные хранятся на диске
-- Выбирай колонки по частоте фильтрации

-- ✅ ПРАВИЛЬНО: часто фильтруем по user_id, потом по времени
ORDER BY (user_id, event_time)
-- Быстро: WHERE user_id = 123 AND event_time > ...
-- Быстро: WHERE user_id = 123

-- ❌ ПЛОХО: event_time первым, если чаще фильтруем по user_id
ORDER BY (event_time, user_id)
-- Медленно: WHERE user_id = 123 (полный скан)

-- Для логов: фильтрация по service + level + time
ORDER BY (service, level, event_time)
3. LowCardinality для строк с малым числом уникальных значений
SQL

-- ✅ ПРАВИЛЬНО: LowCardinality для < 10000 уникальных значений
sport         LowCardinality(String),    -- ~50 видов спорта
country       LowCardinality(FixedString(2)),  -- ~200 стран
device        LowCardinality(String),    -- mobile, desktop, tablet
status        LowCardinality(String),    -- ~10 статусов
currency      LowCardinality(FixedString(3)),  -- ~30 валют

-- ❌ ПЛОХО: LowCardinality для высококардинальных данных
email         LowCardinality(String),    -- миллионы уникальных ❌
session_id    LowCardinality(String),    -- миллионы уникальных ❌
4. Partition By
SQL

-- ✅ Partition по месяцам — стандарт для event-данных
PARTITION BY toYYYYMM(event_time)
-- Каждый месяц = отдельная директория на диске
-- DROP PARTITION мгновенный (для TTL cleanup)

-- ❌ ПЛОХО: partition по дням для маленьких таблиц
-- Слишком много партиций, overhead на merge

-- ❌ ПЛОХО: partition по высококардинальному полю
PARTITION BY user_id  -- миллионы партиций ❌
INGESTION ИЗ REDPANDA
SQL

-- 1. Kafka engine таблица (читает из Redpanda)
CREATE TABLE bet_events_queue ON CLUSTER 'main_cluster'
(
    event_time    DateTime64(3),
    user_id       UInt64,
    bet_id        UInt64,
    action        String,
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
    kafka_broker_list = 'redpanda-0:9092,redpanda-1:9092,redpanda-2:9092',
    kafka_topic_list = 'bets.events',
    kafka_group_name = 'clickhouse_bet_events',
    kafka_format = 'JSONEachRow',
    kafka_num_consumers = 3,
    kafka_max_block_size = 65536;

-- 2. Materialized View: автоматически переливает данные
CREATE MATERIALIZED VIEW bet_events_mv ON CLUSTER 'main_cluster'
TO bet_events AS
SELECT
    event_time,
    user_id,
    bet_id,
    action,
    sport,
    league,
    toDecimal64(stake, 8) AS stake,
    toDecimal64(odds, 6) AS odds,
    currency,
    result,
    toDecimal64OrNull(pnl, 8) AS pnl,
    country,
    device,
    session_id,
    toIPv4OrDefault(ip) AS ip
FROM bet_events_queue;
АНАЛИТИЧЕСКИЕ ЗАПРОСЫ
Дашборд: Revenue
SQL

-- GGR по дням за последний месяц
SELECT
    toDate(event_time) AS day,
    count() AS total_bets,
    sum(stake) AS total_stakes,
    sumIf(pnl, result = 'win') AS total_payouts,
    sum(stake) - abs(sumIf(pnl, result = 'win')) AS ggr,
    uniqExact(user_id) AS unique_players,
    round(ggr / total_stakes * 100, 2) AS margin_pct
FROM bet_events
WHERE event_time >= today() - 30
  AND action = 'settled'
GROUP BY day
ORDER BY day DESC;
-- На 1 млрд строк: < 2 сек
Дашборд: User Cohort Retention
SQL

-- Retention по когортам (месяц регистрации)
WITH registrations AS (
    SELECT
        user_id,
        toStartOfMonth(min(event_time)) AS cohort_month
    FROM bet_events
    WHERE action = 'placed'
    GROUP BY user_id
)
SELECT
    r.cohort_month,
    count(DISTINCT r.user_id) AS cohort_size,
    count(DISTINCT IF(
        toStartOfMonth(b.event_time) = r.cohort_month + INTERVAL 1 MONTH,
        b.user_id, NULL
    )) AS month_1,
    count(DISTINCT IF(
        toStartOfMonth(b.event_time) = r.cohort_month + INTERVAL 2 MONTH,
        b.user_id, NULL
    )) AS month_2,
    count(DISTINCT IF(
        toStartOfMonth(b.event_time) = r.cohort_month + INTERVAL 3 MONTH,
        b.user_id, NULL
    )) AS month_3
FROM registrations r
LEFT JOIN bet_events b ON b.user_id = r.user_id AND b.action = 'placed'
GROUP BY r.cohort_month
ORDER BY r.cohort_month DESC;
Real-time: Active Users
SQL

-- Активные пользователи прямо сейчас (последние 5 минут)
SELECT
    count(DISTINCT user_id) AS active_users,
    countIf(action = 'placed') AS bets_placed,
    sum(stake) AS total_stake
FROM bet_events
WHERE event_time >= now() - INTERVAL 5 MINUTE;
RTP мониторинг для казино
SQL

-- Отклонение RTP от теоретического по играм
SELECT
    game_id,
    game_name,
    theoretical_rtp,
    count() AS rounds,
    sum(bet_amount) AS total_bet,
    sum(win_amount) AS total_win,
    round(total_win / total_bet * 100, 2) AS actual_rtp,
    round(actual_rtp - theoretical_rtp, 2) AS rtp_deviation
FROM casino_rounds
WHERE event_time >= today() - 7
GROUP BY game_id, game_name, theoretical_rtp
HAVING rounds > 1000
ORDER BY abs(rtp_deviation) DESC
LIMIT 20;
-- Если deviation > ±3% → alarm
MATERIALIZED VIEWS ДЛЯ АГРЕГАЦИИ
SQL

-- Pre-aggregated hourly stats (не пересчитывать каждый раз)
CREATE MATERIALIZED VIEW hourly_stats_mv ON CLUSTER 'main_cluster'
ENGINE = ReplicatedSummingMergeTree('/clickhouse/tables/{shard}/hourly_stats', '{replica}')
PARTITION BY toYYYYMM(hour)
ORDER BY (hour, sport, country)
AS SELECT
    toStartOfHour(event_time) AS hour,
    sport,
    country,
    count() AS bets_count,
    uniqExact(user_id) AS unique_users,
    sum(stake) AS total_stake,
    sumIf(stake, result = 'win') AS winning_stakes,
    sumIf(abs(pnl), result = 'win') AS total_payouts
FROM bet_events
WHERE action = 'settled'
GROUP BY hour, sport, country;

-- Запрос к MV — мгновенный
SELECT
    hour,
    sum(bets_count) AS bets,
    sum(unique_users) AS users,
    sum(total_stake) AS stakes
FROM hourly_stats_mv
WHERE hour >= today() - 7
GROUP BY hour
ORDER BY hour;
ЛОГИРОВАНИЕ
SQL

-- Таблица логов (принимает из Vector)
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
    metadata      String   -- JSON string
)
ENGINE = ReplicatedMergeTree('/clickhouse/tables/{shard}/platform_logs', '{replica}')
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (service, level, timestamp)
TTL timestamp + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;

-- Поиск ошибок по сервису
SELECT timestamp, message, trace_id, metadata
FROM platform_logs
WHERE service = 'betting-engine'
  AND level >= 'error'
  AND timestamp >= now() - INTERVAL 1 HOUR
ORDER BY timestamp DESC
LIMIT 100;
АНТИПАТТЕРНЫ
SQL

-- ❌ ПЛОХО: SELECT DISTINCT на огромных таблицах
SELECT DISTINCT user_id FROM bet_events;

-- ✅ ПРАВИЛЬНО: uniqExact или GROUP BY
SELECT count() FROM (SELECT user_id FROM bet_events GROUP BY user_id);

-- ❌ ПЛОХО: JOIN больших таблиц
SELECT * FROM bet_events b JOIN user_events u ON b.user_id = u.user_id;

-- ✅ ПРАВИЛЬНО: используй subquery или IN
SELECT * FROM bet_events
WHERE user_id IN (
    SELECT user_id FROM user_events WHERE event_type = 'deposit'
);

-- ❌ ПЛОХО: UPDATE/DELETE (ClickHouse не для этого)
ALTER TABLE bet_events DELETE WHERE bet_id = 123;
-- Это тяжёлая мутация, не для регулярного использования

-- ✅ ПРАВИЛЬНО: append-only, используй TTL для удаления старых данных

-- ❌ ПЛОХО: String для всего
country String,  -- "US", "UK", "DE"...

-- ✅ ПРАВИЛЬНО: FixedString + LowCardinality
country LowCardinality(FixedString(2)),

-- ❌ ПЛОХО: Nullable без необходимости
user_id Nullable(UInt64),  -- user_id всегда есть

-- ✅ ПРАВИЛЬНО: Nullable только когда значение реально может отсутствовать
result Nullable(Enum8('win' = 1, 'loss' = 2)),  -- NULL для unsettled
PERFORMANCE
text

1. ORDER BY ключ = 80% производительности. Выбирай внимательно
2. LowCardinality: используй для строк с < 10K уникальных значений
3. Partition: по месяцам для event data, по дням для логов
4. TTL: автоматическое удаление старых данных
5. Pre-aggregate: MaterializedView + SummingMergeTree
6. Не делай JOINs: денормализуй данные при вставке
7. PREWHERE: ClickHouse автоматически, но можно указать явно
8. Sampling: для приблизительных запросов на огромных данных
   SELECT count() * 10 FROM table SAMPLE 1/10;