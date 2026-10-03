# ClickHouse Schema Documentation

**Author:** DATA_ENGINEER
**Updated:** 2026-03-24
**Cluster:** main_cluster (3 shards × 2 replicas + 3 Keeper nodes)

## Кластер

| Компонент         | Pods | CPU      | RAM   | Storage   |
| ----------------- | ---- | -------- | ----- | --------- |
| ClickHouse Server | 6    | 16 cores | 128GB | 500GB gp3 |
| ClickHouse Keeper | 3    | 2 cores  | 8GB   | 50GB gp3  |

**Replicated path:** `/clickhouse/tables/{shard}/{table_name}`

## Таблицы

### platform_logs

Логи всех сервисов (от Vector).

| Колонка    | Тип                    | Описание                        |
| ---------- | ---------------------- | ------------------------------- |
| timestamp  | DateTime64(3)          | Время события                   |
| level      | Enum8                  | debug, info, warn, error, fatal |
| service    | LowCardinality(String) | Имя сервиса                     |
| message    | String                 | Текст лога                      |
| trace_id   | String                 | OpenTelemetry trace ID          |
| span_id    | String                 | OpenTelemetry span ID           |
| user_id    | Nullable(UInt64)       | ID пользователя                 |
| request_id | String                 | ID запроса                      |
| metadata   | String                 | JSON дополнительные данные      |

- **Engine:** ReplicatedMergeTree
- **Partition:** toYYYYMMDD(timestamp)
- **ORDER BY:** (service, level, timestamp)
- **TTL:** 30 дней
- **Индексы:** bloom_filter по trace_id, set по level

### bet_events

События ставок (из Redpanda).

| Колонка    | Тип                            | Описание                                 |
| ---------- | ------------------------------ | ---------------------------------------- |
| event_time | DateTime64(3)                  | Время события                            |
| user_id    | UInt64                         | ID пользователя                          |
| bet_id     | UInt64                         | ID ставки                                |
| action     | Enum8                          | placed, settled, cashout, void, rejected |
| bet_type   | LowCardinality(String)         | single, accumulator, system              |
| sport      | LowCardinality(String)         | Вид спорта                               |
| league     | LowCardinality(String)         | Лига                                     |
| stake      | Decimal64(8)                   | Сумма ставки                             |
| odds       | Decimal64(6)                   | Коэффициент                              |
| currency   | LowCardinality(FixedString(3)) | Валюта                                   |
| result     | Nullable(Enum8)                | win, loss                                |
| pnl        | Nullable(Decimal64(8))         | Прибыль/убыток                           |
| country    | LowCardinality(FixedString(2)) | Страна                                   |
| device     | LowCardinality(String)         | Устройство                               |
| session_id | String                         | ID сессии                                |
| ip         | IPv4                           | IP адрес                                 |

- **Engine:** ReplicatedMergeTree
- **Partition:** toYYYYMM(event_time)
- **ORDER BY:** (user_id, event_time)
- **TTL:** 3 года
- **Источник:** Redpanda topics `bets.bet.placed`, `bets.bet.settled`, `bets.bet.cashout`

### user_events

Поведенческие события пользователей.

| Колонка    | Тип                            | Описание                                                                        |
| ---------- | ------------------------------ | ------------------------------------------------------------------------------- |
| event_time | DateTime64(3)                  | Время события                                                                   |
| user_id    | UInt64                         | ID пользователя                                                                 |
| event_type | LowCardinality(String)         | login, logout, deposit, withdrawal, bet, game_start, profile_update, kyc_submit |
| properties | String                         | JSON дополнительные данные                                                      |
| ip         | IPv4                           | IP адрес                                                                        |
| user_agent | String                         | User-Agent                                                                      |
| country    | LowCardinality(FixedString(2)) | Страна                                                                          |
| device     | LowCardinality(String)         | Устройство                                                                      |
| session_id | String                         | ID сессии                                                                       |

- **Engine:** ReplicatedMergeTree
- **Partition:** toYYYYMM(event_time)
- **ORDER BY:** (event_type, user_id, event_time)
- **TTL:** 1 год
- **Источник:** Redpanda topic `analytics.events`

### casino_rounds

Раунды казино игр.

| Колонка       | Тип                            | Описание                                    |
| ------------- | ------------------------------ | ------------------------------------------- |
| event_time    | DateTime64(3)                  | Время раунда                                |
| user_id       | UInt64                         | ID пользователя                             |
| round_id      | String                         | ID раунда                                   |
| game_id       | UInt32                         | ID игры                                     |
| game_name     | String                         | Название игры                               |
| provider      | LowCardinality(String)         | Провайдер                                   |
| category      | LowCardinality(String)         | Категория (slots, table_games, live_casino) |
| bet_amount    | Decimal64(8)                   | Сумма ставки                                |
| win_amount    | Decimal64(8)                   | Сумма выигрыша                              |
| currency      | LowCardinality(FixedString(3)) | Валюта                                      |
| rounds_played | UInt32                         | Количество раундов                          |
| session_id    | String                         | ID сессии                                   |
| device        | LowCardinality(String)         | Устройство                                  |
| country       | LowCardinality(FixedString(2)) | Страна                                      |

- **Engine:** ReplicatedMergeTree
- **Partition:** toYYYYMM(event_time)
- **ORDER BY:** (game_id, user_id, event_time)
- **TTL:** 2 года
- **Источник:** Redpanda topic `casino.round.completed`

### fraud_signals

Сигналы fraud detection.

| Колонка     | Тип                            | Описание                    |
| ----------- | ------------------------------ | --------------------------- |
| signal_time | DateTime64(3)                  | Время сигнала               |
| user_id     | UInt64                         | ID пользователя             |
| signal_type | LowCardinality(String)         | Тип сигнала                 |
| severity    | Enum8                          | low, medium, high, critical |
| score       | Float64                        | Fraud score (0-1)           |
| details     | String                         | JSON детали                 |
| ip          | IPv4                           | IP адрес                    |
| device      | LowCardinality(String)         | Устройство                  |
| country     | LowCardinality(FixedString(2)) | Страна                      |

- **Engine:** ReplicatedMergeTree
- **Partition:** toYYYYMM(signal_time)
- **ORDER BY:** (user_id, signal_time)
- **TTL:** 2 года

### financial_reports

Агрегированные финансовые отчёты.

| Колонка           | Тип                            | Описание              |
| ----------------- | ------------------------------ | --------------------- |
| report_date       | Date                           | Дата отчёта           |
| currency          | LowCardinality(FixedString(3)) | Валюта                |
| total_deposits    | Decimal64(8)                   | Сумма депозитов       |
| total_withdrawals | Decimal64(8)                   | Сумма выводов         |
| total_bets        | Decimal64(8)                   | Сумма ставок          |
| total_wins        | Decimal64(8)                   | Сумма выигрышей       |
| ggr               | Decimal64(8)                   | Gross Gaming Revenue  |
| bonus_cost        | Decimal64(8)                   | Стоимость бонусов     |
| net_revenue       | Decimal64(8)                   | Чистый доход          |
| unique_depositors | UInt32                         | Уникальные депозиторы |
| unique_bettors    | UInt32                         | Уникальные ставящие   |
| new_registrations | UInt32                         | Новые регистрации     |

- **Engine:** ReplicatedSummingMergeTree
- **Partition:** toYYYYMM(report_date)
- **ORDER BY:** (report_date, currency)
- **TTL:** 5 лет

### affiliate_daily_aggregates

Дневные агрегаты партнёров (дашборд кабинета). Миграция `003_affiliate_analytics.sql`.

| Колонка             | Тип                 | Описание                  |
| ------------------- | ------------------- | ------------------------- |
| affiliate_id        | String              | ID партнёра               |
| report_date         | Date                | Дата отчёта               |
| clicks              | UInt64              | Клики                     |
| registrations       | UInt64              | Регистрации               |
| ftd_count           | UInt64              | Первые депозиты           |
| active_players      | UInt64              | Активные игроки           |
| ggr_amount          | Decimal(18, 8)      | GGR                       |
| ngr_amount          | Decimal(18, 8)      | NGR                       |
| commission_accrued  | Decimal(18, 8)      | Начислено                 |
| commission_released | Decimal(18, 8)      | Высвобождено из hold      |
| commission_reversed | Decimal(18, 8)      | Сторнировано              |
| currency            | LowCardinality(String) | Валюта                 |

- **Engine:** SummingMergeTree
- **Partition:** toYYYYMM(report_date)
- **ORDER BY:** (affiliate_id, report_date)
- **MV:** `affiliate_daily_summary_mv` (клики из `affiliate_clicks_raw`)

### affiliate_earnings / affiliate_payouts

Финансовые записи партнёрки (аудит начислений и выплат).

- **affiliate_earnings:** earning_id, affiliate_id, referred_user_id, source_type/source_id, period_start/end, ggr/ngr_amount Decimal(18,8), commission_rate Decimal(10,4), commission_amount, status, hold_until — MergeTree, ORDER BY (affiliate_id, created_at, earning_id).
- **affiliate_payouts:** payout_id, affiliate_id, amount, currency, status, requested_at, approved_at Nullable, provider_reference Nullable — MergeTree, ORDER BY (affiliate_id, requested_at, payout_id).

### affiliate_clicks_raw / affiliate_attributions

Сырые клики (TTL 90 дней) и атрибуции `click -> registration -> FTD`.

- **affiliate_clicks_raw:** click_id, affiliate_id, link_id Nullable, ip_hash, user_agent_hash, device_fingerprint Nullable, country_code, landing_page, campaign Nullable — MergeTree, ORDER BY (affiliate_id, created_at, click_id).
- **affiliate_attributions:** attribution_id, affiliate_id, referred_user_id, click_id, attribution_model, attributed_at, ftd_at Nullable, is_ftd_qualified — MergeTree, ORDER BY (affiliate_id, attributed_at, attribution_id).

### referred_player_activity / affiliate_funnel_hourly / affiliate_fraud_signals

- **referred_player_activity:** активность приведённых игроков по дням (bets, ggr, deposits, withdrawals, bonuses) — SummingMergeTree, ORDER BY (affiliate_id, referred_user_id, activity_date). Источник LTV и когорт.
- **affiliate_funnel_hourly:** почасовая воронка clicks/registrations/ftds/active + конверсии — SummingMergeTree, TTL 30 дней.
- **affiliate_fraud_signals:** fraud-сигналы партнёрки (signal_type, severity, details JSON) — MergeTree, TTL 365 дней.

## Kafka Engine (Redpanda ingestion)

| Queue Table         | Topics                                              | Consumer Group           | Format      |
| ------------------- | --------------------------------------------------- | ------------------------ | ----------- |
| bet_events_queue    | bets.bet.placed, bets.bet.settled, bets.bet.cashout | clickhouse_bet_events    | JSONEachRow |
| user_events_queue   | analytics.events                                    | clickhouse_user_events   | JSONEachRow |
| casino_rounds_queue | casino.round.completed                              | clickhouse_casino_rounds | JSONEachRow |

**Bootstrap servers:** `redpanda-{0,1,2}.redpanda.data.svc.cluster.local:9092`

## Pre-aggregated Views

### hourly_bet_stats

Часовая статистика ставок по видам спорта и странам.

- **Engine:** ReplicatedAggregatingMergeTree
- **Partition:** toYYYYMM(hour)
- **ORDER BY:** (hour, sport, country)
- **Метрики:** bets_count, unique_users (uniq), total_stake, total_payouts

### daily_ggr

Ежедневный GGR по видам спорта.

- **Engine:** ReplicatedAggregatingMergeTree
- **Partition:** toYYYYMM(date)
- **ORDER BY:** (date, sport)
- **Метрики:** bets_count, unique_bettors (uniq), total_stake, total_payouts, ggr

## Аналитические запросы

### Daily GGR Dashboard

```sql
SELECT
    toDate(event_time) AS date,
    sport,
    sum(stake) AS total_stake,
    sumIf(abs(pnl), result = 'win') AS total_payouts,
    sum(stake) - sumIf(abs(pnl), result = 'win') AS ggr
FROM bet_events
WHERE action = 'settled'
    AND event_time >= now() - INTERVAL 30 DAY
GROUP BY date, sport
ORDER BY date DESC, ggr DESC;
```

### User Cohort Retention

```sql
SELECT
    toStartOfWeek(registration_date) AS cohort_week,
    dateDiff('week', registration_date, event_time) AS week_number,
    uniq(user_id) AS active_users
FROM user_events
WHERE event_type = 'login'
GROUP BY cohort_week, week_number
ORDER BY cohort_week, week_number;
```

### RTP Monitoring (Casino)

```sql
SELECT
    game_name,
    sum(bet_amount) AS total_bets,
    sum(win_amount) AS total_wins,
    (sum(win_amount) / sum(bet_amount)) * 100 AS rtp_percent
FROM casino_rounds
WHERE event_time >= now() - INTERVAL 1 HOUR
GROUP BY game_name
HAVING total_bets > 10000
    AND (rtp_percent < 85 OR rtp_percent > 115)
ORDER BY rtp_percent;
```

## Anti-patterns

- ❌ `SELECT DISTINCT` на больших таблицах → используй `uniqExact`
- ❌ `JOIN` на больших таблицах → денормализуй при вставке
- ❌ `UPDATE/DELETE` → append-only + TTL
- ❌ `String` для low-cardinality → `LowCardinality(String)`
- ❌ `Nullable` без необходимости → используй дефолтные значения
