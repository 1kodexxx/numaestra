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

- **Go 1.25+** — https://go.dev/dl/ (на Windows запусти установщик `.msi`)
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
  -d '{"email":"user@example.com","brief":"Песня на юбилей","plan":"standard"}'
```

**Цену определяет сервер по тарифу (`plan`)** — сумма НЕ принимается из тела запроса,
иначе её можно было бы занизить и пройти сверку в вебхуке оплаты. Доступные тарифы и
цены задаются переменными `PRICE_*` (см. `.env.example`). Ответ содержит итоговую
`amount_kopecks` и `access_token` — токен нужно сохранить и передавать в заголовке
`X-Access-Token` для доступа к заказу.

Вебхук Robokassa идемпотентен (повторные доставки уже оплаченного заказа возвращают
`OK{InvId}`), а постановка задачи генерации защищена от гонки двойной оплаты
(условный апдейт `WHERE payment_status='pending'` + дедупликация задачи по `TaskID`).
При исчерпании ретраев фоновой задачи заказ автоматически переводится в `failed`,
а занятый Suno-аккаунт освобождается.

Публичные маршруты защищены rate-limiting по IP, для всех ответов выставляются CORS-заголовки.

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

## Тестирование

Юнит-тесты покрывают доменные стейт-машины, use-case (с in-memory моками репозиториев),
HTTP-хендлеры, middleware, адаптер Suno, воркер, а также клиентов `robokassa`, `openai`,
`s3`, `suno` (через `httptest`). Репозитории на Postgres требуют живой БД и в юнит-наборе
не покрыты — для них предполагаются интеграционные тесты.
