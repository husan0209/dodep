# Backup & PITR Runbook — PostgreSQL + Citus (WAL-G → S3)

**Author:** DATA_ENGINEER
**Updated:** 2026-10-15
**Scope:** Этап 3.1 (backup/PITR) + evidence для Этапов 17-18
**Манифесты:** `infra/k8s/data/postgresql/postgresql-backup.yaml` (CronJobs),
`infra/k8s/data/postgresql/postgresql-replication.yaml` (standby),
`infra/k8s/data/postgresql/pgbouncer-tuning.yaml` (pooling)

---

## 1. Цели: RTO / RPO

| Сценарий | RTO | RPO | Метод |
|---|---|---|---|
| Потеря coordinator pod (PVC жив) | < 5 мин | 0 (WAL replay на месте) | K8s пересоздаёт pod, standby → чтение |
| Потеря coordinator + PVC (AZ failure) | < 30 мин | < 5 мин | Promote standby `pg-coordinator-replica` |
| Логическая порча (oops-DELETE/UPDATE) | < 30 мин | до секунды (PITR на метку) | WAL-G `backup-fetch` + `wal-fetch` до `--target-time` |
| Полная потеря кластера (регион) | < 2 ч | < 24 ч (last full) + WAL | Restore full + WAL replay в новый кластер |
| Потеря worker ноды Citus | < 15 мин | 0 | Shard repair: `citus_copy_shard_placement` (factor 2) |

**PITR SLA:** point-in-time recovery < 30 минут (чеклист 3.1).

---

## 2. Как работает backup

```
primary (pg-coordinator)
  │ archive_command = 'wal-g wal-push %p'   # непрерывный WAL → S3 (включить вручную, §3)
  │ archive_timeout = 60
  ▼
s3://opus-casino-pg-backups/postgres/prod/wal/     # WAL сегменты, RPO ~1 мин
s3://opus-casino-pg-backups/postgres/prod/basebackups_005/  # daily full (CronJob 02:00 UTC)
```

- Full backup: CronJob `pg-walg-full-backup` (02:00 UTC) делает `pg_basebackup`
  с primary по сети (`replication_user`, пароль — Secret `postgresql-secrets`
  → `replication-password`) во временный volume, затем `wal-g backup-push`.
- Verify + retention: CronJob `pg-walg-backup-verify` (06:00 UTC):
  `wal-g backup-list` + `wal-verify integrity` + `delete retain FULL 30`.
- Monthly/yearly: тегируются вручную (`wal-g backup-mark`) — 12 monthly + 1 yearly
  поверх 30 daily (политика из `docs/data/README.md`).
- Workers: отдельные бэкапы НЕ нужны — шарды восстанавливаются Citus repair
  с surviving реплик; reference tables живут на coordinator (входят в backup).

---

## 3. Включение архивации на primary (одноразово, после deploy)

```bash
# Выполнить на pg-coordinator-0 (все значения — env/Secret refs, не литеры):
kubectl -n data exec -it pg-coordinator-0 -- psql -U opus_admin -d opus_platform -c "
  ALTER SYSTEM SET archive_mode = 'on';
  ALTER SYSTEM SET archive_command = 'wal-g wal-push %p';
  ALTER SYSTEM SET archive_timeout = 60;
  ALTER SYSTEM SET max_wal_senders = 10;
  ALTER SYSTEM SET max_replication_slots = 10;
  ALTER SYSTEM SET wal_keep_segments = 64;"
kubectl -n data rollout restart statefulset/pg-coordinator  # или pg_ctl reload, если без рестарта:
# kubectl -n data exec -it pg-coordinator-0 -- psql -U opus_admin -c "SELECT pg_reload_conf();"

# Создание replication_user + слот (если ещё нет):
kubectl -n data exec -it pg-coordinator-0 -- psql -U opus_admin -d opus_platform -c "
  DO \$\$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'replication_user') THEN
      CREATE ROLE replication_user WITH REPLICATION LOGIN PASSWORD '<из Vault>';
    END IF;
  END \$\$;"
# Пароль replication_user положить в Secret:
kubectl -n data create secret generic postgresql-secrets \
  --from-literal=postgres-password='<из Vault>' \
  --from-literal=replication-password='<из Vault>' \
  --dry-run=client -o yaml | kubectl apply -f -
# S3 credentials (IRSA предпочтительнее; static — только через Vault → Secret):
kubectl -n data create secret generic s3-backup-credentials \
  --from-literal=aws-access-key-id='<из Vault>' \
  --from-literal=aws-secret-access-key='<из Vault>' \
  --dry-run=client -o yaml | kubectl apply -f -
```

Проверка непрерывной архивации:

```bash
kubectl -n data exec -it pg-coordinator-0 -- psql -U opus_admin -c \
  "SELECT last_archived_wal, last_archived_time, last_failed_wal FROM pg_stat_archiver;"
# last_archived_time должен быть свежее 2 минут. last_failed_wal — пуст.
```

---

## 4. Restore: полный (disaster recovery)

```bash
# 1. Остановить запись (scale приложений в 0 или заблокировать pgbouncer):
kubectl -n platform scale deploy --all --replicas=0  # согласовать с DEVOPS

# 2. Подготовить чистый PGDATA и скачать последний full:
export WALG_S3_PREFIX=s3://opus-casino-pg-backups/postgres/prod
export AWS_REGION=eu-central-1   # credentials — только из env/Secret, см. §3
wal-g backup-fetch /var/lib/postgresql/data/pgdata LATEST

# 3. Настроить recovery и стартовать:
cat > /var/lib/postgresql/data/pgdata/postgresql.auto.conf <<EOF
restore_command = 'wal-g wal-fetch %f %p'
standby_mode = off
EOF
# Для PG12+: touch standby.signal НЕ нужен (это promote), для replay достаточно restore_command.
pg_ctl -D /var/lib/postgresql/data/pgdata start
# Проверить: SELECT pg_is_in_recovery(); → f после завершения replay.

# 4. Citus: перерегистрировать workers (метаданные восстановлены из backup):
SELECT master_add_node('pg-worker-0.pg-worker', 5432);
SELECT master_add_node('pg-worker-1.pg-worker', 5432);
SELECT master_add_node('pg-worker-2.pg-worker', 5432);
SELECT * FROM citus_repair_shard_placement_on_node(...) -- при битых шардах, см. §6
```

---

## 5. Restore: PITR на метку времени (логическая порча)

```bash
# Цель: состояние на 2026-10-14T12:00:00Z (за 1 мин до oops-транзакции).
export WALG_S3_PREFIX=s3://opus-casino-pg-backups/postgres/prod
wal-g backup-fetch /tmp/pitr-data LATEST

cat > /tmp/pitr-data/recovery.signal <<EOF
EOF
cat > /tmp/pitr-data/postgresql.auto.conf <<EOF
restore_command = 'wal-g wal-fetch %f %p'
recovery_target_time = '2026-10-14T12:00:00Z'
recovery_target_action = 'promote'
recovery_target_timeline = 'latest'
EOF
pg_ctl -D /tmp/pitr-data start
# Дождаться promote, затем выгрузить нужные таблицы и влить в prod:
pg_dump -h /tmp/pitr -U opus_admin -t wallets --data-only | psql -h pg-coordinator -U opus_admin
```

PITR тест (обязателен раз в квартал, evidence для Этапа 18):

```bash
# 1. Зафиксировать метку, 2. сделать тестовую запись, 3. восстановиться на метку, 4. сверить.
psql -h pg-coordinator -U opus_admin -c "SELECT now();"  # T0
psql -h pg-coordinator -U opus_admin -c "CREATE TABLE pitr_probe(id int); INSERT INTO pitr_probe VALUES (1);"
# ... выполнить restore §5 с recovery_target_time = T0 ...
psql -h <restored> -c "\dt pitr_probe"  # таблицы быть НЕ должно → PITR точен
```

---

## 6. Failover на standby (потеря primary)

```bash
# Standby pg-coordinator-replica-0 живой и отстаёт < 1 WAL-сегмент:
kubectl -n data exec -it pg-coordinator-replica-0 -- psql -U opus_admin -c \
  "SELECT now() - pg_last_xact_replay_timestamp() AS replay_lag;"
# Promote:
kubectl -n data exec -it pg-coordinator-replica-0 -- pg_ctl -D /var/lib/postgresql/data/pgdata promote
# Переключить pgbouncer DATABASE_URL на pg-coordinator-replica (Kustomize patch),
# затем пересоздать старый primary как новый standby (pg_basebackup в обратную сторону).
```

---

## 7. PgBouncer: проверка 10K соединений (живой кластер)

```bash
# Admin console:
psql -h pgbouncer -p 5432 -U opus_admin pgbouncer -c "SHOW POOLS;"
psql -h pgbouncer -p 5432 -U opus_admin pgbouncer -c "SHOW CLIENTS;" | wc -l
# Нагрузка (k6, сценарий держит N открытых транзакций через pgbouncer):
k6 run --vus 2000 --duration 5m tools/testing/k6/scenarios/pgbouncer-conns.js
# Критерии: 0 ошибок connect, server_conns ≤ 120 (SHOW POOLS, столбец sv_active+sv_idle),
# p99 wait_time < 50ms. Математика лимитов — в pgbouncer-tuning.yaml.
```

---

## 8. Мониторинг и алерты (связь с Этапом 2)

| Сигнал | Источник | Порог |
|---|---|---|
| `pg_stat_archiver.last_failed_wal != ''` | postgres-exporter | P1 немедленно |
| Возраст `last_archived_time` > 5 мин | postgres-exporter | P1 |
| CronJob `pg-walg-full-backup` failed | kube-state-metrics | P2 |
| `wal-g wal-verify` errors | CronJob логи → ClickHouse | P2 |
| Standby replay lag > 60s | `pg_last_xact_replay_timestamp()` | P2 |
| PgBouncer `cl_waiting` > 100 | SHOW POOLS → metrics | P2 |

---

## 9. Остатки (не закрыто этим runbook — нужен живой кластер / Vault)

- S3 бакет `opus-casino-pg-backups` и IAM/IRSA роль — создаёт DEVOPS (Terraform,
  вне разрешённых DATA_ENGINEER директорий); static ключи — только временный путь.
- Первый реальный full backup + первый PITR-тест — только на живом кластере.
- k6-прогоны (PgBouncer 10K, ClickHouse <1s) — только на живом кластере.
