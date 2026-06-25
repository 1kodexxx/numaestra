#!/bin/sh
# Ночной дамп Postgres с ротацией. Запускается контейнером `backup` из
# docker-compose.yml (profile "backup") по расписанию BACKUP_INTERVAL_SECONDS.
set -eu

BACKUP_DIR="${BACKUP_DIR:-/backups}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-7}"
INTERVAL="${BACKUP_INTERVAL_SECONDS:-86400}"
BACKUP_S3_BUCKET="${BACKUP_S3_BUCKET:-}"
BACKUP_S3_PREFIX="${BACKUP_S3_PREFIX:-postgres}"
BACKUP_S3_ENDPOINT="${BACKUP_S3_ENDPOINT:-}"
BACKUP_S3_REGION="${BACKUP_S3_REGION:-us-east-1}"
BACKUP_S3_ACCESS_KEY="${BACKUP_S3_ACCESS_KEY:-}"
BACKUP_S3_SECRET_KEY="${BACKUP_S3_SECRET_KEY:-}"

mkdir -p "$BACKUP_DIR"

ensure_aws_cli() {
	if command -v aws >/dev/null 2>&1; then
		return 0
	fi
	echo "[backup] aws cli не найден, устанавливаю..."
	apk add --no-cache aws-cli >/dev/null
}

upload_offsite() {
	file="$1"
	[ -n "$BACKUP_S3_BUCKET" ] || return 0
	if [ -z "$BACKUP_S3_ACCESS_KEY" ] || [ -z "$BACKUP_S3_SECRET_KEY" ]; then
		echo "[backup] OFFSITE ПРОПУЩЕН: не заданы BACKUP_S3_ACCESS_KEY/BACKUP_S3_SECRET_KEY"
		return 0
	fi

	ensure_aws_cli

	key="$BACKUP_S3_PREFIX/$(basename "$file")"
	dst="s3://$BACKUP_S3_BUCKET/$key"
	echo "[backup] offsite upload -> $dst"
	if [ -n "$BACKUP_S3_ENDPOINT" ]; then
		AWS_ACCESS_KEY_ID="$BACKUP_S3_ACCESS_KEY" \
		AWS_SECRET_ACCESS_KEY="$BACKUP_S3_SECRET_KEY" \
		AWS_DEFAULT_REGION="$BACKUP_S3_REGION" \
		aws s3 cp "$file" "$dst" --endpoint-url "$BACKUP_S3_ENDPOINT" --only-show-errors
	else
		AWS_ACCESS_KEY_ID="$BACKUP_S3_ACCESS_KEY" \
		AWS_SECRET_ACCESS_KEY="$BACKUP_S3_SECRET_KEY" \
		AWS_DEFAULT_REGION="$BACKUP_S3_REGION" \
		aws s3 cp "$file" "$dst" --only-show-errors
	fi
}

dump_once() {
	ts=$(date -u +%Y%m%dT%H%M%SZ)
	out="$BACKUP_DIR/numaestra-$ts.sql.gz"
	echo "[backup] $(date -u --iso-8601=seconds) — снимаю дамп $PGDATABASE в $out"
	pg_dump --no-owner --no-privileges | gzip > "$out.tmp"
	mv "$out.tmp" "$out"
	echo "[backup] готово: $(du -h "$out" | cut -f1)"
	upload_offsite "$out" || echo "[backup] ОШИБКА offsite upload, локальный бэкап сохранён"

	find "$BACKUP_DIR" -name 'numaestra-*.sql.gz' -mtime "+$RETENTION_DAYS" -print -delete | \
		while read -r removed; do echo "[backup] удалён старый дамп: $removed"; done
}

while true; do
	dump_once || echo "[backup] ОШИБКА дампа, попробую на следующей итерации"
	sleep "$INTERVAL"
done
