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

# Apply each migration file in sorted order
for f in $(ls "$MIGRATIONS_PG"/*.sql 2>/dev/null | sort); do
  filename=$(basename "$f")
  already=$(PGPASSWORD="$POSTGRES_PASSWORD" psql -h "$POSTGRES_HOST" -p "$POSTGRES_PORT" \
    -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tAc \
    "SELECT COUNT(*) FROM schema_migrations WHERE filename='$filename'")

  if [ "$already" = "1" ]; then
    echo "  [skip]    $filename"
  else
    echo "  [apply]   $filename"
    PGPASSWORD="$POSTGRES_PASSWORD" psql -h "$POSTGRES_HOST" -p "$POSTGRES_PORT" \
      -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f "$f"
    PGPASSWORD="$POSTGRES_PASSWORD" psql -h "$POSTGRES_HOST" -p "$POSTGRES_PORT" \
      -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c \
      "INSERT INTO schema_migrations (filename) VALUES ('$filename') ON CONFLICT DO NOTHING;"
    echo "  [done]    $filename"
  fi
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

for f in $(ls "$MIGRATIONS_CH"/*.sql 2>/dev/null | sort); do
  filename=$(basename "$f")
  echo "  [apply]   $filename (ClickHouse — idempotent DDL)"
  curl -sf \
    "http://$CLICKHOUSE_HOST:$CLICKHOUSE_PORT/" \
    --user "$CLICKHOUSE_USER:$CLICKHOUSE_PASSWORD" \
    --data-binary @"$f" > /dev/null
  echo "  [done]    $filename"
done

echo ""
echo "═══════════════════════════════════════════"
echo "  All migrations complete."
echo "═══════════════════════════════════════════"
