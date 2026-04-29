#!/usr/bin/env sh
# postgres-backup.sh — Periodic PostgreSQL backups to MinIO / S3
# Usage inside container: /scripts/postgres-backup.sh [--cron]
set -eu

CRON_MODE=false
for arg in "$@"; do
  case $arg in --cron) CRON_MODE=true ;; esac
done

BACKUP_DIR="${BACKUP_DIR:-/tmp/backups}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-7}"
mkdir -p "$BACKUP_DIR"

do_backup() {
  TIMESTAMP=$(date +%Y%m%d_%H%M%S)
  BACKUP_FILE="$BACKUP_DIR/opus_casino_${TIMESTAMP}.sql.gz"

  echo "[$(date)] Starting backup → $BACKUP_FILE"
  PGPASSWORD="$PGPASSWORD" pg_dump \
    -h "${POSTGRES_HOST:-postgres}" \
    -U "${POSTGRES_USER:-postgres}" \
    "${POSTGRES_DB:-opus_casino}" \
    | gzip > "$BACKUP_FILE"

  echo "[$(date)] Uploading to s3://${BACKUP_S3_BUCKET}/postgres/$TIMESTAMP/"
  # Use mc (MinIO client) or aws cli depending on what's available
  if command -v mc > /dev/null 2>&1; then
    mc alias set minio "${S3_ENDPOINT}" "${S3_ACCESS_KEY}" "${S3_SECRET_KEY}" --quiet
    mc cp "$BACKUP_FILE" "minio/${BACKUP_S3_BUCKET}/postgres/$(basename $BACKUP_FILE)"
  elif command -v aws > /dev/null 2>&1; then
    aws s3 cp "$BACKUP_FILE" \
      "s3://${BACKUP_S3_BUCKET}/postgres/$(basename $BACKUP_FILE)" \
      --endpoint-url "$S3_ENDPOINT"
  fi

  # Remove local temp files older than 1 day
  find "$BACKUP_DIR" -name "*.sql.gz" -mtime +1 -delete 2>/dev/null || true

  echo "[$(date)] Backup complete."
}

if [ "$CRON_MODE" = "true" ]; then
  echo "Running in cron mode (every 6 hours)"
  while true; do
    do_backup
    sleep 21600  # 6 hours
  done
else
  do_backup
fi
