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

echo "==> Деплой образа ghcr.io/1kodexxx/numaestra:${IMAGE_TAG}"
docker compose "${FILES[@]}" pull
docker compose "${FILES[@]}" up -d --remove-orphans

echo "==> Очистка неиспользуемых образов"
docker image prune -af

echo "==> Готово. Статус сервисов:"
docker compose "${FILES[@]}" ps
