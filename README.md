# Numaestra — запуск каркаса локально

Этого достаточно, чтобы поднять сервис и убедиться, что он стартует, отвечает по HTTP
и корректно завершается. Бизнес-логики пока нет — это проверка фундамента.

## 1. Предустановки

- Go 1.22+ — https://go.dev/dl/ (на Windows просто запусти установщик `.msi`)
- Docker Desktop (для Postgres и Redis) — https://www.docker.com/products/docker-desktop/
- Проверь после установки:

```powershell
go version
docker --version
```

## 2. Поднять Postgres и Redis

Из корня проекта:

```powershell
docker compose up -d
docker compose ps
```

Оба контейнера (`numaestra-postgres`, `numaestra-redis`) должны быть `healthy` через
несколько секунд.

## 3. Подтянуть зависимости Go

```powershell
go mod tidy
```

Это скачает `chi`, `uuid`, `asynq`, `pgx` и сгенерирует `go.sum`. Нужен обычный доступ
в интернет — если стоит корпоративный/строгий firewall и `go mod tidy` падает с ошибкой
вида `unrecognized import path` или `403`, см. раздел "Если go mod tidy не работает" ниже.

## 4. Настроить переменные окружения (опционально)

Дефолты в `internal/config/config.go` уже совпадают с docker-compose, так что можно
просто запускать без `.env`. Если хочешь переопределить — скопируй `.env.example` в `.env`
и подгружай переменные перед запуском (например, через `Set-Content`/`$env:` в PowerShell
или пакет типа `direnv` в bash).

## 5. Запустить сервер

```powershell
go run ./cmd/server
```

Должен появиться неоновый ASCII-баннер, а затем строки логов о подключении к Postgres
и старте HTTP-сервера на `:8080`.

## 6. Проверить, что сервер отвечает

В отдельном окне PowerShell:

```powershell
curl.exe http://localhost:8080/healthz
```

(Просто `curl` в PowerShell — это алиас на `Invoke-WebRequest` с другим синтаксисом,
поэтому используй `curl.exe` или `Invoke-WebRequest -Uri http://localhost:8080/healthz`.)

Ожидаемый ответ — `ok` и строка лога `http запрос обработан` в окне с сервером.

## 7. Проверить graceful shutdown

Вернись в окно с сервером и нажми `Ctrl+C`. В логах должна появиться последовательность:

```
получен сигнал завершения, начинаем graceful shutdown
сервис Numaestra остановлен корректно
```

Если вместо этого процесс падает с паникой или просто молча убивается — значит,
что-то в shutdown-логике сломалось при переносе кода, проверяй `cmd/server/main.go`.

## 8. Остановить инфраструктуру

```powershell
docker compose down
```

Добавь `-v`, если хочешь снести и данные Postgres (`docker compose down -v`).

## Если `go mod tidy` не работает

Если сеть режет доступ к `proxy.golang.org` (бывает в офисных/учебных сетях), попробуй:

```powershell
$env:GOPROXY = "https://goproxy.io,direct"
go mod tidy
```

или, если есть доступ к официальному проксисервису без ограничений, просто:

```powershell
$env:GOPROXY = ""
go mod tidy
```

(пустое значение возвращает дефолтный `https://proxy.golang.org`).

## Структура

Всё, что уже реализовано (без бизнес-логики, чистый каркас):

- `internal/domain` — сущности `Order`, `SunoAccount`, порты `AccountRepository`,
  `OrderRepository`, `QueuePublisher`, `MusicProvider`.
- `internal/repository/suno` — адаптер `MusicProvider` поверх `pkg/suno.Client`.
- `internal/repository/queue` — адаптер `QueuePublisher` поверх Asynq.
- `pkg/suno` — провайдеро-независимый контракт + `MockClient` для разработки без
  реального Suno/реселлера.
- `pkg/logger`, `pkg/banner` — неоновый логгер и ASCII-баннер.
- `cmd/server/main.go` — точка входа, DI, graceful shutdown.

Чего ещё нет (следующие шаги): миграции и реализации репозиториев на Postgres,
use-case слой, HTTP-хендлеры заказов, интеграция Robokassa, Asynq-воркер.
