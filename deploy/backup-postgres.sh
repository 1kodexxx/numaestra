#!/bin/sh
# Ночной дамп Postgres с ротацией. Запускается контейнером `backup` из
# docker-compose.yml (profile "backup") по расписанию BACKUP_INTERVAL_SECONDS.
set -eu

BACKUP_DIR="${BACKUP_DIR:-/backups}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-7}"
INTERVAL="${BACKUP_INTERVAL_SECONDS:-86400}"

mkdir -p "$BACKUP_DIR"

dump_once() {
	ts=$(date -u +%Y%m%dT%H%M%SZ)
	out="$BACKUP_DIR/numaestra-$ts.sql.gz"
	echo "[backup] $(date -u --iso-8601=seconds) — снимаю дамп $PGDATABASE в $out"
	pg_dump --no-owner --no-privileges | gzip > "$out.tmp"
	mv "$out.tmp" "$out"
	echo "[backup] готово: $(du -h "$out" | cut -f1)"

	find "$BACKUP_DIR" -name 'numaestra-*.sql.gz' -mtime "+$RETENTION_DAYS" -print -delete | \
		while read -r removed; do echo "[backup] удалён старый дамп: $removed"; done
}

while true; do
	dump_once || echo "[backup] ОШИБКА дампа, попробую на следующей итерации"
	sleep "$INTERVAL"
done
