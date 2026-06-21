# Numaestra

Бэкенд-сервис генерации персональных песен на заказ. Полный цикл:

1. Клиент отправляет бриф (ТЗ на песню) через HTTP API.
2. Оплачивает заказ через **Robokassa**.
3. По вебхуку оплаты заказ ставится в очередь (**Asynq/Redis**).
4. Фоновый воркер захватывает свободный **Suno-аккаунт** из пула, генерирует текст
   через **LLM** (OpenRouter/GPT) и отправляет запрос в **Suno API**.
5. Опрашивает статус генерации, перезаливает готовые треки в **S3** и уведомляет клиента.
6. Клиент забирает треки через защищённый токеном API.

Архитектура — чистая/гексагональная: `domain` → `usecase` → `delivery`/`repository`/`worker`,
внешние интеграции вынесены в `pkg/`.

## 1. Предустановки

- **Go 1.24+** — https://go.dev/dl/ (на Windows запусти установщик `.msi`)
- **Docker Desktop** (для Postgres и Redis) — https://www.docker.com/products/docker-desktop/

```powershell
go version
docker --version
```

## 2. Поднять инфраструктуру

Из корня проекта:

```powershell
docker compose up -d
docker compose ps
```

Контейнеры `numaestra-postgres` и `numaestra-redis` должны стать `healthy` за несколько секунд.

## 3. Зависимости Go

```powershell
go mod tidy
```

Если корпоративный firewall режет `proxy.golang.org`:

```powershell
$env:GOPROXY = "https://goproxy.io,direct"
go mod tidy
```

## 4. Переменные окружения

Дефолты в `internal/config/config.go` совпадают с docker-compose, поэтому для локального
старта `.env` не обязателен. Для переопределения скопируй `.env.example` → `.env`.
Полный список переменных — в `.env.example` (Postgres, Redis, Robokassa, Suno, S3, LLM).

## 5. Запуск сервера

```powershell
go run ./cmd/server
```

Появится неоновый ASCII-баннер и логи о подключении к Postgres, применении миграций
и старте HTTP-сервера на `:8080`.

## 6. Проверка работоспособности

```powershell
curl.exe http://localhost:8080/healthz
```

Ответ — JSON вида:

```json
{"status":"ok","checks":{"postgres":"ok","redis":"ok"}}
```

`HTTP 200` — все зависимости живы; `HTTP 503` — хотя бы одна недоступна.

## 7. Graceful shutdown

`Ctrl+C` в окне сервера → в логах:

```
получен сигнал завершения, начинаем graceful shutdown
сервис Numaestra остановлен корректно
```

## 8. Остановить инфраструктуру

```powershell
docker compose down      # с -v снесёт и данные Postgres
```

## HTTP API

Базовый префикс — `/api/v1/orders`.

| Метод | Маршрут | Доступ | Назначение |
|-------|---------|--------|------------|
| `POST` | `/` | публичный | Создать заказ, получить `payment_url` и `access_token` |
| `POST` | `/webhook/robokassa` | подпись Robokassa | Подтверждение оплаты |
| `GET` | `/` | `X-Access-Token` | Список заказов клиента |
| `GET` | `/{id}` | `X-Access-Token` | Детали заказа и треки |

Создание заказа:

```bash
curl -X POST http://localhost:8080/api/v1/orders/ \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","brief":"Песня на юбилей"}'
```

**Цена фиксированная и определяется сервером** — без тарифов и подписок: один платёж
за 4 версии песни. Сумма НЕ принимается из тела запроса, иначе её можно было бы занизить
и пройти сверку в вебхуке оплаты. Цена задаётся переменной `PRICE_KOPECKS` (см.
`.env.example`, по умолчанию 200000 = 2000 ₽). Ответ содержит итоговую `amount_kopecks`
и `access_token` — токен нужно сохранить и передавать в заголовке `X-Access-Token`
для доступа к заказу.

Вебхук Robokassa идемпотентен (повторные доставки уже оплаченного заказа возвращают
`OK{InvId}`), а постановка задачи генерации защищена от гонки двойной оплаты
(условный апдейт `WHERE payment_status='pending'` + дедупликация задачи по `TaskID`).
При исчерпании ретраев фоновой задачи заказ автоматически переводится в `failed`,
а занятый Suno-аккаунт освобождается.

Публичные маршруты защищены rate-limiting по IP, для всех ответов выставляются CORS-заголовки.

## Админка (`/admin`)

Раздел `/admin` на фронтенде — управление категориями квиза, заказами (просмотр,
возврат оплаты, обратная связь клиенту) и пулом Suno-аккаунтов. Открой
`http://localhost:8080/admin/login` после `go run ./cmd/server` (или `make frontend-dev`
для разработки с hot-reload — Vite проксирует `/api` на бэкенд).

**Вход** — логин/пароль из `ADMIN_LOGIN`/`ADMIN_PASSWORD` (см. `.env.example`).
После входа выставляется httpOnly+Secure+SameSite=Strict cookie с подписанным
(HMAC-SHA256, `ADMIN_SESSION_SECRET`) токеном без сервера состояний — токен не
читается из JS (защита от кражи через XSS) и не требует Redis/БД для сессии.
`/admin/login` жёстко ограничен по частоте запросов (защита от перебора пароля).

Для скриптов/CI вместо логина можно использовать `ADMIN_TOKEN` как `Authorization: Bearer` —
оба способа аутентификации работают параллельно на одних и тех же маршрутах `/api/v1/admin/*`.

| Раздел | Маршруты API | Назначение |
|--------|--------------|------------|
| Категории | `GET/POST /admin/categories/`, `GET/PUT/DELETE /admin/categories/{id}`, `.../questions/...` | Карточки квиза (картинка, заголовок, описание, тэги) + вопросы и варианты ответов |
| Заказы | `GET /admin/orders/`, `GET /admin/orders/{id}`, `POST .../refund`, `POST .../feedback` | Просмотр, возврат оплаты через Robokassa, письмо клиенту с сохранением в БД |
| Suno-аккаунты | `GET/POST /admin/accounts/`, `PATCH /admin/accounts/{id}` | Пул аккаунтов, к которым подключается воркер генерации |

## Разработка

```bash
make help        # список команд
make test        # все тесты
make test-race   # тесты с детектором гонок
make cover       # покрытие
make lint        # golangci-lint
make vet         # go vet
```

CI (`.github/workflows/ci.yml`) на каждый push/PR прогоняет build, vet, тесты с `-race`
и golangci-lint.

## Структура

```
cmd/server          — точка входа, DI, HTTP + Asynq worker, graceful shutdown
internal/
  config            — конфигурация из переменных окружения
  domain            — агрегаты Order, SunoAccount; порты репозиториев и провайдеров
  usecase           — оркестрация бизнес-сценариев
  delivery/http     — REST-хендлеры, middleware (CORS, rate limit), Robokassa webhook
  repository/
    postgres        — репозитории заказов и аккаунтов (pgx, FOR UPDATE SKIP LOCKED)
    queue           — публикатор задач поверх Asynq
    suno            — адаптер MusicProvider поверх pkg/suno
  worker            — обработчики фоновых задач Asynq
migrations          — embedded SQL-миграции
pkg/
  banner, health, logger, migrate, notify, openai, robokassa, s3, suno
```

## Продакшен-деплой (VPS / Docker Compose)

Базовый `docker compose up -d` поднимает только `postgres`, `redis` и `app` — этого
достаточно для dev. Для прода на VPS добавлены три опциональных профиля
(`deploy/`), каждый включается отдельно:

### TLS / reverse-proxy (`profile proxy`)

`app` сам по себе слушает голый HTTP на `:8080`. На VPS перед ним должен стоять
TLS-терминатор. Используется Caddy с автоматическим Let's Encrypt:

```powershell
# В .env: DOMAIN=numaestra.example.com, ACME_EMAIL=you@example.com
docker compose --profile proxy up -d
```

Caddy слушает `:80`/`:443` на хосте и проксирует на `app:8080` внутри docker-сети.
Конфиг — `deploy/Caddyfile`. Порт `8080` самого `app` в `docker-compose.yml` стоит
либо не публиковать на хост вовсе, либо забиндить только на `127.0.0.1`, если
порт пробрасывается напрямую для отладки.

### Бэкапы Postgres (`profile backup`)

```powershell
docker compose --profile backup up -d
```

Сервис `backup` каждые `BACKUP_INTERVAL_SECONDS` (по умолчанию раз в сутки) снимает
`pg_dump` через `deploy/backup-postgres.sh`, кладёт сжатый дамп в `./backups/` на
хосте и удаляет дампы старше `BACKUP_RETENTION_DAYS` (по умолчанию 7 дней).
`./backups/` — это обычная директория хоста, её нужно включить в свой бэкап
VPS/диска (snapshot, rsync на другой сервер и т.п.) — сам контейнер не отправляет
дампы за пределы хоста.

Восстановление:

```bash
POSTGRES_DSN=postgres://numaestra:numaestra@localhost:5432/numaestra \
  ./deploy/restore-postgres.sh ./backups/numaestra-20260621T120000Z.sql.gz
```

### Мониторинг и алерты (`profile monitoring`)

```powershell
docker compose --profile monitoring up -d
```

Поднимает Prometheus (`:9090`) со скрейпом `app:8080/metrics` и Alertmanager
(`:9093`). Правила алертов — `deploy/alerts.yml` (даунтайм сервиса, доля 5xx,
латентность p95, массовые `failed`-заказы, ошибки Suno API).

**Перед продом обязательно** заполни реальный receiver в `deploy/alertmanager.yml`
(email/Telegram/Slack) — Alertmanager не подставляет переменные окружения в
конфиг, плейсхолдеры там нужно заменить руками, иначе алерты будут просто копиться
в UI и никто их не увидит.

### Чек-лист обязательных переменных для prod (`APP_ENV != dev`)

Без них `config.Load()` вернёт фатальную ошибку при старте: `ADMIN_TOKEN`,
`ADMIN_LOGIN`/`ADMIN_PASSWORD`, `ADMIN_SESSION_SECRET`,
`SUNO_API_KEY`, `OPENAI_API_KEY`, `S3_ACCESS_KEY`/`S3_SECRET_KEY`,
`SESSION_ENCRYPTION_KEY`. Дополнительно рекомендуется задать `SMTP_HOST` —
без него уведомления клиентам уходят только в лог (заглушка), без реальной отправки.

## Тестирование

Юнит-тесты покрывают доменные стейт-машины, use-case (с in-memory моками репозиториев),
HTTP-хендлеры, middleware, адаптер Suno, воркер, а также клиентов `robokassa`, `openai`,
`s3`, `suno` (через `httptest`). Репозитории на Postgres требуют живой БД и в юнит-наборе
не покрыты — для них есть интеграционные тесты (`make test-integration`, testcontainers).

Фронтенд: `make frontend-test` (vitest + Testing Library). CI прогоняет typecheck,
тесты и build фронта отдельным job'ом (`.github/workflows/ci.yml`).
