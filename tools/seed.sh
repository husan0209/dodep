#!/usr/bin/env bash
# seed.sh — Apply seed data for all environments
set -euo pipefail

ENV="development"
for arg in "$@"; do
  case $arg in
    --env) shift; ENV="$1"; shift ;;
    --env=*) ENV="${arg#*=}"; shift ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

if [ "$ENV" = "production" ]; then
  source "$PROJECT_ROOT/.env.production"
else
  [ -f "$PROJECT_ROOT/.env" ] && source "$PROJECT_ROOT/.env"
fi

POSTGRES_HOST="${POSTGRES_HOST:-localhost}"
POSTGRES_PORT="${POSTGRES_PORT:-5433}"
POSTGRES_DB="${POSTGRES_DB:-opus_casino}"
POSTGRES_USER="${POSTGRES_USER:-postgres}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-changeme}"

echo "Applying seed: 010_seed_reference.sql ..."
PGPASSWORD="$POSTGRES_PASSWORD" psql \
  -h "$POSTGRES_HOST" -p "$POSTGRES_PORT" \
  -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  -f "$PROJECT_ROOT/libs/migrations/postgresql/010_seed_reference.sql"

echo "Seed complete."
