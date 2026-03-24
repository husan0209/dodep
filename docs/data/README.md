# Data Layer — Architecture Overview

**Author:** DATA_ENGINEER
**Updated:** 2026-03-24
**Status:** ✅ Complete (Stage 3)

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
