#!/bin/sh
# Восстановление дампа в БД numaestra. Использование:
#   ./deploy/restore-postgres.sh ./backups/numaestra-20260621T120000Z.sql.gz
#
# Внимание: команда не сносит существующие данные сама — psql применит дамп
# поверх текущей схемы. Для восстановления "с нуля" сначала останови app/worker
# и пересоздай БД (`docker compose down -v` снесёт volume, либо `DROP DATABASE`).
set -eu

if [ "$#" -ne 1 ]; then
	echo "Использование: $0 <путь-к-дампу.sql.gz>" >&2
	exit 1
fi

DUMP_FILE="$1"
DSN="${POSTGRES_DSN:?переменная POSTGRES_DSN обязательна}"

echo "[restore] применяю $DUMP_FILE к $DSN"
gunzip -c "$DUMP_FILE" | psql "$DSN"
echo "[restore] готово"
