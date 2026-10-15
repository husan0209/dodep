# Data Layer — Architecture Overview

**Author:** DATA_ENGINEER
**Updated:** 2026-10-15
**Status:** 🟡 Stage 3 — 70% + audit follow-up (replication/backup/PITR артефакты добавлены, живые прогоны pending)

## Статус Этапа 3.1 — Replication / Backup / PITR (обновлено 2026-10-15)

| Пункт чеклиста 3.1 | Статус | Артефакт |
|---|---|---|
| Citus кластер 1 coord + 3 workers | ✅ есть | `infra/k8s/data/postgresql/postgresql.yaml` |
| PgBouncer transaction mode, 10K conns | ✅ конфиг есть, ⏳ живой прогон pending | `postgresql.yaml` + `infra/k8s/data/postgresql/pgbouncer-tuning.yaml` (математика 120<200, `SHOW POOLS` процедура) |
| Streaming replication coordinator | ✅ артефакт добавлен, ⏳ живой тест pending | `infra/k8s/data/postgresql/postgresql-replication.yaml` (standby + `pg-read` Service) |
| Backup WAL-G → S3 | ✅ артефакты добавлены, ⏳ первый backup pending | `infra/k8s/data/postgresql/postgresql-backup.yaml` (CronJobs full 02:00 UTC + verify 06:00 UTC, S3 refs via env) |
| PITR тестирование | ✅ runbook добавлен, ⏳ живой PITR-тест pending | `docs/data/backup-pitr-runbook.md` (RTO/RPO, restore + PITR команды, probe-процедура) |
| pg_partman autopartition | 🟡 конфиг закомментирован в `004_transactions.sql` | Включить после deploy (команда в миграции) |
| Миграция 024 (индексы + RLS) | ✅ добавлена, идемпотентна | `libs/migrations/postgresql/024_stage3_hardening.sql` |

Аудит-дыры, подтверждённые сверкой STAGES 3.1-3.4 с файлами (см. runbook §9 и отчёт в сессии):
`009_outbox` ✅ на месте; `postgresql.yaml` с PgBouncer ✅; `dragonflydb.yaml` ✅;
`clickhouse.yaml` ✅ (TTL 30d/1-3yr/5yr + 5 MV на месте); `redpanda.yaml` + topics Job ✅
(14 топиков + отдельный `affiliate-topics-job.yaml`).

## Evidence checklist для Этапов 17-18 (data scope)

То, что CI/CD и Security инженеры должны приложить как доказательства прохождения
(полный список — внизу файла, раздел «Evidence checklist для Этапов 17-18 (data scope, живой кластер)»).

## Архитектура данных

```
┌─────────────────────────────────────────────────────────────────┐
│                        DATA LAYER                               │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐      │
│  │  PostgreSQL   │  │  DragonflyDB │  │    ClickHouse    │      │
│  │  + Citus      │  │  (Cache)     │  │  (Analytics)     │      │
│  │  OLTP         │  │  3 nodes     │  │  3 shards × 2    │      │
│  │  coordinator  │  │  64GB/node   │  │  + Keeper        │      │
│  │  + 3 workers  │  │              │  │  128GB RAM/node  │      │
│  └──────┬───────┘  └──────────────┘  └────────┬─────────┘      │
│         │                                      │                │
│         │              ┌──────────────┐        │                │
│         └──────────────┤   Redpanda   ├────────┘                │
│                        │  Event Bus   │                         │
│                        │  3 brokers   │                         │
│                        │  + Schema R. │                         │
│                        └──────────────┘                         │
└─────────────────────────────────────────────────────────────────┘
```

## Компоненты

| Компонент | Технология            | Роль                       | Ноды                    | Память     |
| --------- | --------------------- | -------------------------- | ----------------------- | ---------- |
| OLTP БД   | PostgreSQL 16 + Citus | Основная транзакционная БД | 1 coord + 3 workers     | 16GB/node  |
| Кэш       | DragonflyDB           | Сессии, кэш, rate limiting | 3 (master + 2 replicas) | 72GB/node  |
| Аналитика | ClickHouse            | Логи, события, отчёты      | 3 shards × 2 replicas   | 128GB/node |
| Event Bus | Redpanda              | Асинхронные события        | 3 брокера               | 32GB/node  |
| Объекты   | S3/MinIO              | Бэкапы, KYC документы      | —                       | —          |

## Топология K8s

Все компоненты размещены в namespace `data`:

```
data/
├── pg-coordinator          StatefulSet (1 replica)
├── pg-worker               StatefulSet (3 replicas)
├── pgbouncer               Deployment (2 replicas)
├── dragonfly               StatefulSet (3 replicas)
├── redpanda                StatefulSet (3 replicas)
├── clickhouse              StatefulSet (6 replicas)
└── clickhouse-keeper       StatefulSet (3 replicas)
```

## Стратегия шардирования (Citus)

| Таблица             | Тип         | Шард по | Кол-во шардов     |
| ------------------- | ----------- | ------- | ----------------- |
| users               | Distributed | user_id | 32                |
| wallets             | Distributed | user_id | 32                |
| wallet_transactions | Distributed | user_id | 32                |
| bets                | Distributed | user_id | 32                |
| bet_selections      | Distributed | user_id | 32                |
| currencies          | Reference   | —       | — (реплицирована) |
| countries           | Reference   | —       | — (реплицирована) |
| sports              | Reference   | —       | — (реплицирована) |
| game_configs        | Reference   | —       | — (реплицирована) |

## Партиционирование

| Таблица             | Поле партиционирования | Тип     | TTL     |
| ------------------- | ---------------------- | ------- | ------- |
| wallet_transactions | created_at             | Monthly | —       |
| ledger_entries      | created_at             | Monthly | —       |
| bets                | placed_at              | Daily   | —       |
| audit_log           | created_at             | Monthly | —       |
| platform_logs (CH)  | timestamp              | Daily   | 30 дней |
| bet_events (CH)     | event_time             | Monthly | 3 года  |
| user_events (CH)    | event_time             | Monthly | 1 год   |

## Резервное копирование

- **PostgreSQL:** WAL-G → S3 (continuous WAL + daily full)
- **Retention:** 30 daily + 12 monthly + 1 yearly
- **PITR:** Point-in-time recovery < 30 минут
- **ClickHouse:** ReplicatedMergeTree (автоматическая репликация)
- **DragonflyDB:** Snapshots каждые 5 минут → S3
- **Redpanda:** Retention 7-30 дней + репликация factor 3

## Мониторинг

Все компоненты экспортируют Prometheus метрики:

| Компонент   | Endpoint | Port |
| ----------- | -------- | ---- |
| PostgreSQL  | /metrics | 9187 |
| DragonflyDB | /metrics | 6380 |
| ClickHouse  | /metrics | 9363 |
| Redpanda    | /metrics | 9644 |

## Безопасность

- **NetworkPolicy:** Каждый компонент доступен только из namespace `platform` и `data`
- **mTLS:** Istio service mesh шифрует весь трафик между подами
- **Шифрование:** AES-256-GCM для данных в покое (KMS ключи)
- **Аудит:** Все изменения в PostgreSQL записываются в audit_log

## Документация

- [PostgreSQL Schema](./postgresql-schema.md) — Схема БД, миграции, Citus
- [ClickHouse Schema](./clickhouse-schema.md) — Аналитические таблицы, Kafka ingestion
- [DragonflyDB Schema](./dragonfly-schema.md) — Схема кэша, TTL, паттерны
- [Redpanda Topics](./redpanda-topics.md) — Топики, конфигурация, консьюмеры
- [Backup & PITR Runbook](./backup-pitr-runbook.md) — WAL-G → S3, restore, PITR, RTO/RPO

## Evidence checklist для Этапов 17-18 (data scope, живой кластер)

- [ ] `wal-g backup-list` показывает daily full < 24ч свежести (CronJob `pg-walg-full-backup`)
- [ ] `wal-g wal-verify integrity` — 0 errors (CronJob `pg-walg-backup-verify`, логи → ClickHouse)
- [ ] `pg_stat_archiver.last_failed_wal` пуст, `last_archived_time` < 2 мин
- [ ] Standby lag `now() - pg_last_xact_replay_timestamp()` < 60s
- [ ] PITR probe-тест пройден (таблица `pitr_probe` отсутствует после restore на T0)
- [ ] PgBouncer: `SHOW POOLS` server_conns ≤ 120 при k6 2000 VUs, p99 wait < 50ms
- [ ] Dragonfly: `TTL cache:odds:*` > 0, `INFO memory` evictions без OOM, snapshot mtime < 6 мин
- [ ] ClickHouse: MV lag (Kafka consumer) < 1000 msgs, daily GGR dashboard < 3s
- [ ] Redpanda: consumer lag < 10K, 0 under-replicated partitions, DLQ пуст
