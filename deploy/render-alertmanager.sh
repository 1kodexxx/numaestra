#!/bin/sh
# Рендерит рабочий конфиг Alertmanager из переменных окружения (.env).
#
# Зачем: сам Alertmanager НЕ подставляет переменные окружения в свой YAML,
# поэтому конфиг с секретами нельзя держать в репозитории и нельзя просто
# прокинуть env. Этот скрипт запускается одноразовым init-контейнером
# (alertmanager-config в docker-compose.yml) и генерирует готовый файл в общий
# том перед стартом Alertmanager.
#
# Каналы:
#   * Email — через те же SMTP_*, что и клиентские письма приложения
#     (отдельный почтовый релей не нужен). Включается, если заданы SMTP_HOST и
#     ALERT_EMAIL_TO.
#   * Telegram — опционально, если заданы TELEGRAM_BOT_TOKEN и TELEGRAM_CHAT_ID.
#
# Если не настроен ни один канал, рендерится валидный no-op receiver: Alertmanager
# штатно стартует, алерты просто никуда не уходят (видны в его UI). Это безопаснее
# падения контейнера из-за пустого обязательного поля.
set -eu

OUT="${ALERTMANAGER_CONFIG_OUT:-/etc/alertmanager/alertmanager.yml}"

SMTP_PORT="${SMTP_PORT:-587}"
# Порт 465 — implicit TLS (Alertmanager ждёт STARTTLS), для него require_tls=false;
# порт 587 и прочие — STARTTLS, require_tls=true.
if [ "$SMTP_PORT" = "465" ]; then
  REQUIRE_TLS="false"
else
  REQUIRE_TLS="true"
fi

{
  echo "# СГЕНЕРИРОВАНО render-alertmanager.sh из переменных окружения — не редактировать вручную."
  echo "route:"
  echo "  receiver: default"
  echo "  group_wait: 30s"
  echo "  group_interval: 5m"
  echo "  repeat_interval: 4h"
  echo ""
  echo "receivers:"
  echo "  - name: default"

  if [ -n "${ALERT_EMAIL_TO:-}" ] && [ -n "${SMTP_HOST:-}" ]; then
    echo "    email_configs:"
    echo "      - to: \"${ALERT_EMAIL_TO}\""
    echo "        from: \"${SMTP_FROM_ADDRESS:-alertmanager@numaestra.ru}\""
    echo "        smarthost: \"${SMTP_HOST}:${SMTP_PORT}\""
    if [ -n "${SMTP_USER:-}" ]; then
      echo "        auth_username: \"${SMTP_USER}\""
      echo "        auth_password: \"${SMTP_PASSWORD:-}\""
    fi
    echo "        require_tls: ${REQUIRE_TLS}"
    echo "        send_resolved: true"
  fi

  if [ -n "${TELEGRAM_BOT_TOKEN:-}" ] && [ -n "${TELEGRAM_CHAT_ID:-}" ]; then
    echo "    telegram_configs:"
    echo "      - bot_token: \"${TELEGRAM_BOT_TOKEN}\""
    echo "        chat_id: ${TELEGRAM_CHAT_ID}"
    echo "        parse_mode: HTML"
    echo "        send_resolved: true"
  fi
} > "$OUT"

echo "render-alertmanager: конфиг записан в $OUT"
if [ -z "${ALERT_EMAIL_TO:-}" ] && [ -z "${TELEGRAM_BOT_TOKEN:-}" ]; then
  echo "render-alertmanager: ВНИМАНИЕ — ни email, ни Telegram не настроены; алерты будут видны только в UI Alertmanager."
fi
