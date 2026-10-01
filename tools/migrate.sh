#!/usr/bin/env bash
# migrate.sh — Run all PostgreSQL and ClickHouse migrations in order
# Usage: bash tools/migrate.sh [--env production|development]
set -euo pipefail

# ── Parse args ────────────────────────────────────────────────────────
ENV="development"
for arg in "$@"; do
  case $arg in
    --env) shift; ENV="$1"; shift ;;
    --env=*) ENV="${arg#*=}"; shift ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# ── Load environment ──────────────────────────────────────────────────
if [ "$ENV" = "production" ]; then
  ENV_FILE="$PROJECT_ROOT/.env.production"
else
  ENV_FILE="$PROJECT_ROOT/.env"
fi

if [ -f "$ENV_FILE" ]; then
  set -a; source "$ENV_FILE"; set +a
  echo "Loaded env from $ENV_FILE"
fi

# Defaults (dev)
POSTGRES_HOST="${POSTGRES_HOST:-localhost}"
POSTGRES_PORT="${POSTGRES_PORT:-5433}"
POSTGRES_DB="${POSTGRES_DB:-opus_casino}"
POSTGRES_USER="${POSTGRES_USER:-postgres}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-changeme}"
CLICKHOUSE_HOST="${CLICKHOUSE_HOST:-localhost}"
CLICKHOUSE_PORT="${CLICKHOUSE_PORT:-8123}"
CLICKHOUSE_DB="${CLICKHOUSE_DB:-opus_casino}"
CLICKHOUSE_USER="${CLICKHOUSE_USER:-default}"
CLICKHOUSE_PASSWORD="${CLICKHOUSE_PASSWORD:-changeme}"

MIGRATIONS_PG="$PROJECT_ROOT/libs/migrations/postgresql"
MIGRATIONS_CH="$PROJECT_ROOT/libs/migrations/clickhouse"

echo "═══════════════════════════════════════════"
echo "  Opus Casino — Migrations ($ENV)"
echo "  PG:         $POSTGRES_HOST:$POSTGRES_PORT/$POSTGRES_DB"
echo "  ClickHouse: $CLICKHOUSE_HOST:$CLICKHOUSE_PORT/$CLICKHOUSE_DB"
echo "═══════════════════════════════════════════"

# ── Wait for PostgreSQL ────────────────────────────────────────────────
echo "Waiting for PostgreSQL..."
for i in $(seq 1 30); do
  if PGPASSWORD="$POSTGRES_PASSWORD" psql -h "$POSTGRES_HOST" -p "$POSTGRES_PORT" \
       -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "SELECT 1" > /dev/null 2>&1; then
    echo "PostgreSQL ready."
    break
  fi
  echo "  Attempt $i/30, retrying in 3s..."
  sleep 3
done

# ── Run PostgreSQL migrations ──────────────────────────────────────────
echo ""
echo "── PostgreSQL migrations ─────────────────────────────────────"

# Create schema_migrations tracking table if not exists
PGPASSWORD="$POSTGRES_PASSWORD" psql -h "$POSTGRES_HOST" -p "$POSTGRES_PORT" \
  -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "
    CREATE TABLE IF NOT EXISTS schema_migrations (
      filename   TEXT PRIMARY KEY,
      applied_at TIMESTAMPTZ DEFAULT NOW()
    );"

# Apply each migration file in version order.
#
# Two defects are fixed here:
#
# 1. `ls *.sql | sort` also picked up rollback scripts (*.down.sql). Because
#    "014_payments_core.down.sql" sorts BEFORE "014_payments_core.sql", the
#    runner executed DROP TABLE payments/withdrawals immediately before the
#    CREATE that defines them. Rollback scripts are never applied here.
#
# 2. psql without ON_ERROR_STOP exits 0 even when a statement fails, so broken
#    migrations were recorded in schema_migrations and silently skipped on
#    every later run. Failures now abort the run and are NOT recorded.
#
# Ordering is by filename, which keeps duplicate version prefixes (010, 011,
# 022 …) in a deterministic, name-based order. Duplicate prefixes are a
# pre-existing repo condition — verify the resulting order by hand when adding
# a migration that depends on an earlier one.
pg_files() {
  ls "$1"/*.sql 2>/dev/null \
    | grep -v '\.down\.sql$' \
    | awk -F/ '{print $NF"\t"$0}' \
    | sort -t"$(printf '\t')" -k1,1 \
    | cut -f2-
}

for f in $(pg_files "$MIGRATIONS_PG"); do
  filename=$(basename "$f")

  already=$(PGPASSWORD="$POSTGRES_PASSWORD" psql -h "$POSTGRES_HOST" -p "$POSTGRES_PORT" \
    -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tAc \
    "SELECT COUNT(*) FROM schema_migrations WHERE filename='$filename'")

  if [ "$already" = "1" ]; then
    echo "  [skip]    $filename"
    continue
  fi

  echo "  [apply]   $filename"
  if ! PGPASSWORD="$POSTGRES_PASSWORD" psql -v ON_ERROR_STOP=1 \
       -h "$POSTGRES_HOST" -p "$POSTGRES_PORT" \
       -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f "$f"; then
    echo ""
    echo "  ✗ FAILED: $filename"
    echo "    Not recorded in schema_migrations — fix and re-run to retry."
    exit 1
  fi

  PGPASSWORD="$POSTGRES_PASSWORD" psql -v ON_ERROR_STOP=1 \
    -h "$POSTGRES_HOST" -p "$POSTGRES_PORT" \
    -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c \
    "INSERT INTO schema_migrations (filename) VALUES ('$filename') ON CONFLICT DO NOTHING;"
  echo "  [done]    $filename"
done

# ── Wait for ClickHouse ────────────────────────────────────────────────
echo ""
echo "── ClickHouse migrations ─────────────────────────────────────"
for i in $(seq 1 20); do
  if curl -sf "http://$CLICKHOUSE_HOST:$CLICKHOUSE_PORT/ping" > /dev/null 2>&1; then
    echo "ClickHouse ready."
    break
  fi
  echo "  Attempt $i/20, retrying in 3s..."
  sleep 3
done

CH_AUTH=""
if [ -n "$CLICKHOUSE_PASSWORD" ]; then
  CH_AUTH="--user $CLICKHOUSE_USER --password $CLICKHOUSE_PASSWORD"
fi

# ClickHouse DDL is idempotent (CREATE ... IF NOT EXISTS / ON CLUSTER), so it is
# re-applied on every run rather than tracked in schema_migrations.
for f in $(pg_files "$MIGRATIONS_CH"); do
  filename=$(basename "$f")
  echo "  [apply]   $filename (ClickHouse — idempotent DDL)"
  if ! curl -sf \
    "http://$CLICKHOUSE_HOST:$CLICKHOUSE_PORT/" \
    --user "$CLICKHOUSE_USER:$CLICKHOUSE_PASSWORD" \
    --data-binary @"$f" > /dev/null; then
    echo "  ✗ FAILED: $filename"
    exit 1
  fi
  echo "  [done]    $filename"
done

echo ""
echo "═══════════════════════════════════════════"
echo "  All migrations complete."
echo "═══════════════════════════════════════════"
