<div align="center">

# 🎵 Numaestra

**AI-студия персональных песен на заказ.**
Опишите повод — получите 4 готовые версии трека.

Go 1.25 · React 18 · PostgreSQL 16 · Redis 7 · Suno · Robokassa · Docker

</div>

---

Numaestra — full-stack сервис, который превращает бриф клиента в готовую песню:
LLM пишет текст, **Suno** генерирует музыку, **Robokassa** принимает оплату, а
фоновые воркеры оркеструют весь конвейер. Фронтенд — премиальный тёмный SPA с
конструктором промптов, онлайн-плеером и каталогом из 30 категорий «на все случаи
жизни».

## ✨ Возможности

**Для клиента**
- 🎛️ **Конструктор промптов для Suno** — соберите песню с нуля: повод, жанр и
  настроение (мульти-выбор), темп, вокал, детали и свой текст — с живым
  предпросмотром готового промпта.
- 🗂️ **30 категорий** на все случаи (свадьба, день рождения, 8 марта, корпоратив,
  роуст, признание в любви, детская песня…) — каждая со своим квизом.
- 🎧 **Онлайн-плеер** — 4 версии трека, waveform, перемотка, переключение вариантов.
- 📦 Заказ → оплата → отслеживание статуса генерации в реальном времени → скачивание.
- 📱 Адаптивно: 3-колоночный app-shell на десктопе, чистый скролл на мобильных.

**Для администратора (`/admin`)**
- Управление категориями квиза (вопросы, варианты ответов, обложки).
- Заказы: просмотр, возврат оплаты через Robokassa, письмо клиенту.
- Пул Suno-аккаунтов с балансировкой нагрузки.

**Под капотом**
- Идемпотентные платёжные вебхуки, защита от гонки двойной оплаты.
- Пул Suno-аккаунтов с `FOR UPDATE SKIP LOCKED` (без коллизий между воркерами).
- Авто-`failed` при исчерпании ретраев + освобождение аккаунта.
- Cookie-сессия админки на подписанном HMAC-токене (без серверного состояния).
- Rate-limiting, CORS, graceful shutdown, health-checks, Prometheus-метрики.

## 🔄 Как это работает

```
Клиент → бриф/конструктор ─▶ POST /orders ─▶ payment_url (Robokassa)
                                                   │
                              webhook оплаты ◀──────┘
                                     │
                          Asynq/Redis очередь
                                     │
   воркер: захват Suno-аккаунта → LLM пишет текст → Suno генерирует →
   опрос статуса → треки в S3 → уведомление клиента
                                     │
   Клиент ◀── GET /orders/{id} (X-Access-Token) ── статус + треки
```

Архитектура — гексагональная: `domain` → `usecase` → `delivery`/`repository`/`worker`,
внешние интеграции изолированы в `pkg/`.

## 🧱 Технологии

| Слой | Стек |
|------|------|
| **Backend** | Go 1.25, chi/v5, Asynq (Redis), pgx, hexagonal architecture |
| **Frontend** | React 18, TypeScript, Vite 5, Tailwind CSS v4, Feature-Sliced Design |
| **Данные** | PostgreSQL 16, Redis 7 |
| **Интеграции** | Suno API, OpenRouter/OpenAI (LLM), Robokassa, S3, SMTP |
| **Инфра** | Docker Compose, Caddy (авто-TLS), Prometheus + Alertmanager |

UI собран на собственных Material-примитивах (`Button`, `TextField`, `Card`,
`IconButton`, ripple, elevation) поверх кастомной тёмной cyan-темы.

## 🚀 Быстрый старт (всё в Docker)

Образ `app` многоступенчатый: Node собирает SPA → SPA встраивается в Go-бинарник,
который раздаёт и API, и фронтенд на одном порту.

```bash
docker compose up -d --build
```

Открой **http://localhost:8080** — главная страница.
Админка: **http://localhost:8080/admin/login**.

Миграции (включая сидинг 30 категорий) применяются автоматически при старте.
Здоровье сервиса:

```bash
curl http://localhost:8080/healthz
# {"status":"ok","checks":{"postgres":"ok","redis":"ok"}}
```

Остановить: `docker compose down` (с `-v` — снесёт и данные Postgres).

## 🛠️ Локальная разработка

Понадобятся **Go 1.25+**, **Node 20+** и **Docker**.

```bash
# 1. Инфраструктура (Postgres + Redis)
docker compose up -d postgres redis

# 2. Бэкенд + фронтенд одновременно (требует GNU make + bash)
make dev
```

Либо запустить по отдельности:

```bash
go run ./cmd/server   # бэкенд: HTTP :8080 + воркер, ASCII-баннер, авто-миграции
make frontend-dev     # фронтенд: Vite :3000 с hot-reload, проксирует /api → :8080
```

Полезные команды (`make help` — полный список):

```bash
make test            # тесты Go
make test-race       # с детектором гонок
make lint            # golangci-lint
make frontend-build  # сборка SPA в web/out/ (встраивается в бинарник)
make frontend-test   # vitest + Testing Library
```

## ⚙️ Конфигурация

Дефолты в `internal/config/config.go` совпадают с docker-compose, поэтому для dev
`.env` не обязателен. Для переопределения: `cp .env.example .env`.

**Обязательны для прода** (`APP_ENV != dev`, иначе фатальная ошибка при старте):
`ADMIN_TOKEN`, `ADMIN_LOGIN`/`ADMIN_PASSWORD`, `ADMIN_SESSION_SECRET`,
`SUNO_API_KEY`, `OPENAI_API_KEY`, `S3_ACCESS_KEY`/`S3_SECRET_KEY`,
`SESSION_ENCRYPTION_KEY`. Рекомендуется `SMTP_HOST` (без него письма уходят только в лог).

> 💰 **Цена фиксированная и определяется сервером** (`PRICE_KOPECKS`, по умолчанию
> 200000 = 2000 ₽) — клиент не может занизить сумму через тело запроса.

## 🌐 HTTP API

Публичные заказы — префикс `/api/v1/orders`:

| Метод | Маршрут | Доступ | Назначение |
|-------|---------|--------|------------|
| `POST` | `/` | публичный | Создать заказ → `payment_url` + `access_token` |
| `POST` | `/webhook/robokassa` | подпись Robokassa | Подтверждение оплаты (идемпотентно) |
| `GET` | `/` | `X-Access-Token` | Список заказов клиента |
| `GET` | `/{id}` | `X-Access-Token` | Детали заказа и треки |

Каталог — `/api/v1/categories`:

| Метод | Маршрут | Назначение |
|-------|---------|------------|
| `GET` | `/` | Список активных категорий |
| `GET` | `/{id}/wizard` | Вопросы квиза категории с вариантами ответов |

```bash
# Кастомный заказ без категории (из конструктора промптов)
curl -X POST http://localhost:8080/api/v1/orders/ \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","brief":"Повод: жене на юбилей. Жанр: Поп..."}'
```

`access_token` из ответа сохраняйте и передавайте в `X-Access-Token`.
Публичные маршруты защищены rate-limiting по IP и CORS-заголовками.

## 🔐 Админка

`/admin` — категории, заказы (возврат оплаты, обратная связь), пул Suno-аккаунтов.
Вход по `ADMIN_LOGIN`/`ADMIN_PASSWORD` → httpOnly + Secure + SameSite=Strict cookie
с подписанным (HMAC-SHA256) токеном: не читается из JS, не требует Redis/БД для сессии.
`/admin/login` жёстко ограничен по частоте (защита от перебора). Для CI/скриптов —
`ADMIN_TOKEN` как `Authorization: Bearer` на тех же `/api/v1/admin/*`.

## 📂 Структура

```
cmd/server          точка входа, DI, HTTP + Asynq worker, graceful shutdown
internal/
  config            конфигурация из переменных окружения
  domain            агрегаты Order, SunoAccount; порты репозиториев/провайдеров
  usecase           оркестрация бизнес-сценариев
  delivery/http     REST-хендлеры, middleware (CORS, rate limit), SPA, webhook
  repository/
    postgres        заказы и аккаунты (pgx, FOR UPDATE SKIP LOCKED)
    queue           публикатор задач поверх Asynq
    suno            адаптер MusicProvider поверх pkg/suno
  worker            обработчики фоновых задач Asynq
migrations          embedded SQL-миграции (схема + сидинг категорий)
pkg/                banner, health, logger, migrate, notify, openai, robokassa, s3, suno
frontend/src/       React SPA (Feature-Sliced Design)
  app               роутер, провайдеры, глобальные стили/токены
  pages             catalog, category, examples, quiz, status, admin
  widgets           navbar, player, contact-modal, side-panel, admin-layout
  features          load-catalog, create-order, poll-order-status, admin-session
  entities          category, order, admin-* (типы + API-клиенты)
  shared            ui-кит, http-клиент, конфиг, утилиты
```

## 🚢 Продакшен (VPS / Docker Compose)

Базовый `docker compose up -d` поднимает `postgres`, `redis`, `app`. Для прода —
три опциональных профиля:

```bash
docker compose --profile proxy up -d        # Caddy + авто Let's Encrypt (TLS на :443)
docker compose --profile backup up -d       # ежедневный pg_dump в ./backups/
docker compose --profile monitoring up -d   # Prometheus (:9090) + Alertmanager (:9093)
```

- **proxy** — `DOMAIN` и `ACME_EMAIL` в `.env`; конфиг `deploy/Caddyfile`.
- **backup** — ротация по `BACKUP_RETENTION_DAYS`; восстановление — `deploy/restore-postgres.sh`.
- **monitoring** — алерты в `deploy/alerts.yml`; не забудьте прописать receiver в
  `deploy/alertmanager.yml` (email/Telegram/Slack).

## ✅ Тестирование

- **Go**: доменные стейт-машины, use-case (in-memory моки), HTTP-хендлеры, middleware,
  адаптеры (`robokassa`, `openai`, `s3`, `suno` через `httptest`), воркер.
  Postgres-репозитории — интеграционные тесты (`make test-integration`, testcontainers).
- **Frontend**: `make frontend-test` (vitest + Testing Library).
- **CI** (`.github/workflows/ci.yml`): на каждый push/PR — build, vet, `-race`-тесты,
  golangci-lint и отдельный job для typecheck/тестов/сборки фронтенда.
