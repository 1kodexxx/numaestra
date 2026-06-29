# 🚀 Деплой Numaestra

CI/CD устроен так: пуш в `main` → **CI** (сборка, тесты, lint, govulncheck, фронтенд) →
при зелёном CI запускается **CD** → собирает Docker-образ → пушит в **GHCR** →
заходит по **SSH** на сервер и обновляет стек. Сборка идёт в GitHub Actions, а не
на сервере — это важно для маленького VPS (на 2 ГБ RAM сборка Go/Node рискует словить OOM).

```
push main ─▶ CI ─▶ CD ─▶ ghcr.io/1kodexxx/numaestra:<sha> ─▶ ssh ─▶ docker compose pull && up
```

---

## 1. Подготовка сервера (один раз)

Нужен VPS с **Ubuntu 22.04/24.04**, Docker и Docker Compose.

```bash
# Docker + compose plugin
curl -fsSL https://get.docker.com | sh

# Swap 4 ГБ — обязательно на 2 ГБ RAM (страхует пики; см. README про 2 ГБ)
sudo fallocate -l 4G /swapfile && sudo chmod 600 /swapfile
sudo mkswap /swapfile && sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
echo 'vm.swappiness=10' | sudo tee /etc/sysctl.d/99-swap.conf && sudo sysctl -p /etc/sysctl.d/99-swap.conf

# Фаервол: наружу только SSH + HTTP/HTTPS. Порты 5432/6379/8080 НЕ открывать.
sudo ufw allow 22,80,443/tcp && sudo ufw enable

# Клонируем репозиторий в ~/numaestra (CD ожидает именно этот путь)
git clone https://github.com/1kodexxx/numaestra.git ~/numaestra
cd ~/numaestra
```

> 🔒 **Приватный репозиторий?** GitHub не поддерживает парольную авторизацию по
> HTTPS для git — клонируйте по SSH через **read-only Deploy Key**:
> ```bash
> ssh-keygen -t ed25519 -f ~/.ssh/github_deploy -C "numaestra-server" -N ""
> cat ~/.ssh/github_deploy.pub   # вставить в GitHub → Settings → Deploy keys → Add deploy key (без write-доступа)
> cat >> ~/.ssh/config <<'EOF'
> Host github.com
>     IdentityFile ~/.ssh/github_deploy
>     IdentitiesOnly yes
> EOF
> chmod 600 ~/.ssh/config
> git clone git@github.com:1kodexxx/numaestra.git ~/numaestra
> ```
> Тот же ключ затем использует `git pull` внутри `deploy/deploy.sh` при автодеплое.

### Логин в GHCR (чтобы сервер мог тянуть образ)

Образы GHCR приватны по умолчанию. Создайте **Personal Access Token (classic)** с правом
`read:packages` (GitHub → Settings → Developer settings → Tokens) и выполните **один раз**:

```bash
echo '<PAT_read_packages>' | docker login ghcr.io -u 1kodexxx --password-stdin
```

Логин сохранится в `~/.docker/config.json` — повторять не нужно.
(Альтернатива: сделать пакет публичным в GitHub Packages — тогда логин не нужен.)

### `.env` на сервере

```bash
cp .env.example .env
nano .env
```

Заполните как минимум (прод требует их при старте, иначе фатальная ошибка):
`POSTGRES_PASSWORD`, `REDIS_PASSWORD`, `SESSION_ENCRYPTION_KEY`, `ADMIN_TOKEN`, `ADMIN_LOGIN`,
`ADMIN_PASSWORD`, `ADMIN_SESSION_SECRET`, `SUNO_API_KEY`,
`S3_*`, `ROBOKASSA_*`, `DOMAIN`, `ACME_EMAIL`. Также добавьте строку:

```env
# Профили compose: proxy = Caddy с авто-TLS. docker compose читает это нативно.
COMPOSE_PROFILES=proxy,monitoring,backup
APP_ENV=prod
```

> ⚠️ Поменяли пароль БД после первого запуска? Просто правка `.env` не перепаролит
> уже созданный том — см. раздел про `ALTER USER` в истории/README.

---

## 2. Секреты GitHub (для CD)

Settings репозитория → **Secrets and variables → Actions** → New repository secret:

| Секрет | Значение |
|---|---|
| `SSH_HOST` | IP или домен сервера |
| `SSH_USERNAME` | пользователь SSH (например, `root` или `deploy`) |
| `SSH_KEY` | **приватный** SSH-ключ (весь файл, включая `BEGIN/END`) |
| `SSH_PORT` | порт SSH (если не 22; иначе можно не задавать) |

Публичную часть ключа добавьте на сервер в `~/.ssh/authorized_keys`.
`GITHUB_TOKEN` для пуша в GHCR создаётся автоматически — отдельно не нужен.

**Включение автодеплоя.** Пока сервер не готов, шаг деплоя пропускается (образ всё
равно собирается и пушится в GHCR). Когда сервер и секреты готовы — создайте
**переменную** (Variables, не Secret) `DEPLOY_ENABLED` = `true`
(Settings → Secrets and variables → Actions → Variables). Чтобы временно
отключить автодеплой — поставьте `false`.

> Если репозиторий **приватный**, серверу нужен доступ к `git pull`: добавьте
> read-only **deploy key** (Settings → Deploy keys) и клонируйте по SSH-ремоуту.
> Для публичного репозитория ничего не требуется.

---

## 3. Первый запуск (вручную, до автодеплоя)

После того как образ хотя бы раз собран в CD (или соберите локально и запушьте):

```bash
cd ~/numaestra
docker compose -f docker-compose.yml -f docker-compose.prod.yml pull
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

Проверка здоровья:

```bash
curl -fsS https://<ваш-домен>/healthz
# {"status":"ok","checks":{"postgres":"ok","redis":"ok"}}
```

Дальше каждый пуш в `main` с зелёным CI деплоится автоматически.

---

## 4. Обновление и откат

- **Обновление** — автоматически после мерджа в `main`. Вручную: вкладка Actions →
  workflow **CD** → *Run workflow*.
- **Откат** на конкретный коммит (образы тегируются коротким sha):

  ```bash
  cd ~/numaestra
  IMAGE_TAG=<short-sha-рабочего-коммита> bash deploy/deploy.sh
  ```

  Список доступных тегов — в GitHub → Packages → `numaestra`.

---

## 5. Go-live чек-лист

1. Сервер поднят: `healthz` отдаёт `postgres ok, redis ok`, TLS выпущен (Caddy).
2. **Smoke-тест Sunor**: один реальный запрос генерации с боевым `SUNO_API_KEY`.
3. **S3**: тестовый объект реально отдаётся по публичной ссылке (200, не 403).
4. **Тестовый заказ** при `ROBOKASSA_IS_TEST=true` — полный цикл до выдачи трека и письма.
5. Переключить Robokassa в боевой режим, провести **один реальный платёж** на минимум.
6. Прописать receiver алертов (`ALERT_EMAIL_TO` и/или Telegram) — без этого `deploy/deploy.sh`
   остановит non-dev деплой. Профиль `monitoring` подключать на ≥4 ГБ RAM или на отдельном хосте.
7. Настроить offsite backup (`BACKUP_S3_BUCKET`, `BACKUP_S3_ACCESS_KEY`, `BACKUP_S3_SECRET_KEY`) —
   без этого `deploy/deploy.sh` остановит non-dev деплой.

---

## 6. Чек-лист безопасности (прод)

Перед открытием трафика и после каждого крупного релиза:

| Область | Что проверить |
|---|---|
| **Сеть** | UFW: только `22/80/443`. Postgres `5432`, Redis `6379`, приложение `8080` не торчат наружу. |
| **TLS** | Caddy с `COMPOSE_PROFILES=proxy`, сертификат Let's Encrypt валиден, HSTS включён. |
| **Секреты** | В `.env` на сервере: уникальные `POSTGRES_PASSWORD`, `REDIS_PASSWORD`, `SESSION_ENCRYPTION_KEY`, `ADMIN_*`, `ROBOKASSA_*`. Файл `chmod 600`, не в git. |
| **Robokassa** | `ROBOKASSA_ALLOWED_IPS` — только боевые подсети Robokassa (без `172.x`); `ROBOKASSA_IS_TEST=false` только после smoke-теста. Caddy передаёт `X-Real-IP`. Для возвратов в админке: `ROBOKASSA_PASS3` (Password3 в кабинете) и включённый Refund API. |
| **CORS** | `CORS_ALLOWED_ORIGINS` — только ваш домен (без `*` в проде). |
| **Rate limit** | Redis с паролем; лимиты на заказы, каталог, отзывы идут через Redis (не in-memory per-pod). |
| **Share-ссылки** | Клиент может отозвать `/s/{id}` на странице статуса; отозванная ссылка отдаёт 404. |
| **Приватные треки** | (Опц., рекомендуется) `S3_PRESIGN_ENABLED=true` + приватный бакет: ссылки на mp3 временные (presigned), прямой GET без подписи → 403. Пошагово: `deploy/S3-PRESIGN.md`. |
| **Согласие 152-ФЗ** | `consent_doc_version` в заказе совпадает с `/legal/consent`; аудит в админке. |
| **Бэкапы** | `scripts/backup.sh` по cron; копия тома **вне** VPS (S3/другой хост). Периодически пробовать restore. |
| **Обновления** | `govulncheck` в CI зелёный; образы тянутся только из GHCR по sha. |
| **Мониторинг** | `/healthz` в uptime-check; алерты на 5xx и падение воркера. |

> **Ограничения без WAF:** при DDoS на уровне L7 помогает Cloudflare (или аналог) перед Caddy.
> Rate limit при недоступности Redis пропускает трафик (fail-open) — поэтому Redis и его пароль обязательны в проде.

---

## 7. Robokassa: диагностика и восстановление

### Симптом

Деньги списаны, в админке заказ в статусе «Ожидание оплаты», генерация не стартовала.

### Диагностика в логах

```bash
docker compose logs app --since 2h | grep -E 'robokassa|allowlist|вебхук'
```

| Запись в логе | Причина |
|---|---|
| `запрос отклонён IP allowlist` + `status=403` | IP вебхука не прошёл `ROBOKASSA_ALLOWED_IPS` (часто до фикса X-Real-IP за Caddy) |
| `неверная подпись` | Неверный `ROBOKASSA_PASS2` или тестовый/боевой режим не совпадает с кабинетом |
| `вебхук robokassa обработан` | Вебхук принят, заказ должен перейти в оплачен |

### Восстановление заказа

1. **Админка** — на странице заказа кнопка «Подтвердить оплату» (только если платёж есть в кабинете Robokassa).
2. **SSH** — повторить ResultURL вручную (подставьте `invoice_id` и сумму заказа):

```bash
cd ~/numaestra
set -a && source .env && set +a

INV=7
OUTSUM=2000.00
SIG=$(echo -n "${OUTSUM}:${INV}:${ROBOKASSA_PASS2}" | md5sum | awk '{print toupper($1)}')

docker compose exec -T app wget -qO- \
  --header="X-Real-IP: 185.59.216.65" \
  --post-data="OutSum=${OUTSUM}&InvId=${INV}&SignatureValue=${SIG}" \
  http://127.0.0.1:8080/api/v1/orders/webhook/robokassa
```

Ожидаемый ответ: `OK{InvId}` (например `OK7`).

### После деплоя с фиксом X-Real-IP

В `ROBOKASSA_ALLOWED_IPS` **нельзя оставлять пустым** — в `APP_ENV=prod` приложение не запустится
(`ROBOKASSA_ALLOWED_IPS обязателен в окружении "prod"`).

Уберите только временный `172.16.0.0/12` (Docker), **заменив** его адресами Robokassa — не удаляйте переменную целиком:

```bash
# Правильно (prod):
ROBOKASSA_ALLOWED_IPS=185.59.216.65,185.59.217.65

# Неправильно — приложение не стартует:
ROBOKASSA_ALLOWED_IPS=

# Неправильно — костыль до фикса X-Real-IP, больше не нужен:
ROBOKASSA_ALLOWED_IPS=172.16.0.0/12,185.59.216.65,185.59.217.65
```

Перезапуск: `docker compose up -d app`.

Ручная проверка вебхука из контейнера (без `172.16` в allowlist) — с заголовком `X-Real-IP`, см. пример выше.

---

## Примечания по нагрузке

Узкое место — провайдер генерации (Sunor), а не сервер: бэкенд лёгкий, аудио хранится
во внешнем S3. Масштабирование — добавлением Suno-ключей и батч-поллингом, а не железом.
Подробнее — в истории обсуждений / README.
