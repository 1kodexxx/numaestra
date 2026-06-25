#!/usr/bin/env bash
# Деплой на сервере: тянет образ из GHCR и поднимает стек. Вызывается из CD по SSH
# (см. .github/workflows/cd.yml) ПОСЛЕ git pull, поэтому сам git не трогает —
# иначе обновление файла во время выполнения могло бы сломать запущенный скрипт.
#
# Ручной запуск/откат на конкретный коммит:
#   cd ~/numaestra && IMAGE_TAG=<short-sha> bash deploy/deploy.sh
#
# Профили (например, proxy для Caddy/TLS) задаются переменной COMPOSE_PROFILES
# в .env на сервере — docker compose читает её нативно.
set -euo pipefail

cd "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

IMAGE_TAG="${IMAGE_TAG:-latest}"
export IMAGE_TAG

FILES=(-f docker-compose.yml -f docker-compose.prod.yml)

if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  . ./.env
  set +a
fi

is_profile_enabled() {
  case ",${COMPOSE_PROFILES:-}," in
    *",$1,"*) return 0 ;;
    *) return 1 ;;
  esac
}

if [ "${APP_ENV:-dev}" != "dev" ]; then
  if ! is_profile_enabled "monitoring"; then
    echo "ERROR: COMPOSE_PROFILES должен включать monitoring в non-dev окружении." >&2
    exit 1
  fi
  if ! is_profile_enabled "backup"; then
    echo "ERROR: COMPOSE_PROFILES должен включать backup в non-dev окружении." >&2
    exit 1
  fi

  if [ -z "${ALERT_EMAIL_TO:-}" ] && { [ -z "${TELEGRAM_BOT_TOKEN:-}" ] || [ -z "${TELEGRAM_CHAT_ID:-}" ]; }; then
    echo "ERROR: Настройте ALERT_EMAIL_TO или пару TELEGRAM_BOT_TOKEN+TELEGRAM_CHAT_ID перед прод-деплоем." >&2
    exit 1
  fi

  if [ -z "${BACKUP_S3_BUCKET:-}" ] || [ -z "${BACKUP_S3_ACCESS_KEY:-}" ] || [ -z "${BACKUP_S3_SECRET_KEY:-}" ]; then
    echo "ERROR: Для offsite backup обязательны BACKUP_S3_BUCKET, BACKUP_S3_ACCESS_KEY, BACKUP_S3_SECRET_KEY." >&2
    exit 1
  fi
fi

echo "==> Деплой образа ghcr.io/1kodexxx/numaestra:${IMAGE_TAG}"
docker compose "${FILES[@]}" pull
docker compose "${FILES[@]}" up -d --remove-orphans

echo "==> Очистка неиспользуемых образов"
docker image prune -af

echo "==> Готово. Статус сервисов:"
docker compose "${FILES[@]}" ps
