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
`POSTGRES_PASSWORD`, `SESSION_ENCRYPTION_KEY`, `ADMIN_TOKEN`, `ADMIN_LOGIN`,
`ADMIN_PASSWORD`, `ADMIN_SESSION_SECRET`, `SUNO_API_KEY`, `OPENAI_API_KEY`,
`S3_*`, `ROBOKASSA_*`, `DOMAIN`, `ACME_EMAIL`. Также добавьте строку:

```env
# Профили compose: proxy = Caddy с авто-TLS. docker compose читает это нативно.
COMPOSE_PROFILES=proxy
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
6. Прописать receiver алертов (`ALERT_EMAIL_TO` и/или Telegram) — профиль `monitoring`
   подключать на ≥4 ГБ RAM или на отдельном хосте (на 2 ГБ не запускать).

---

## Примечания по нагрузке

Узкое место — провайдер генерации (Sunor), а не сервер: бэкенд лёгкий, аудио хранится
во внешнем S3. Масштабирование — добавлением Suno-ключей и батч-поллингом, а не железом.
Подробнее — в истории обсуждений / README.
