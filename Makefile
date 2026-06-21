.PHONY: help build run test test-integration test-race cover vet lint tidy fmt docker-up docker-down clean \
        frontend-install frontend-dev frontend-build frontend-typecheck dev

APP_NAME := numaestra
CMD_PATH := ./cmd/server

help: ## Показать список доступных команд
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

build: ## Собрать бинарник в ./bin
	go build -o bin/$(APP_NAME) $(CMD_PATH)

run: ## Запустить сервис локально
	go run $(CMD_PATH)

test: ## Прогнать все тесты
	go test ./...

test-integration: ## Интеграционные тесты Postgres (требует Docker)
	go test -tags=integration -timeout 120s -v ./internal/repository/postgres/...

test-race: ## Прогнать тесты с детектором гонок
	go test -race ./...

cover: ## Тесты с отчётом покрытия
	go test -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out | tail -n 1

vet: ## Статический анализ go vet
	go vet ./...

lint: ## Запустить golangci-lint (должен быть установлен)
	golangci-lint run

tidy: ## Привести в порядок зависимости
	go mod tidy

fmt: ## Форматировать код
	gofmt -s -w .

docker-up: ## Поднять инфраструктуру (Postgres, Redis, app)
	docker compose up -d

docker-down: ## Остановить инфраструктуру
	docker compose down

clean: ## Удалить артефакты сборки
	rm -rf bin coverage.out web/out

# ── Frontend ──────────────────────────────────────────────────────────────────

frontend-install: ## Установить npm-зависимости фронтенда
	cd frontend && npm install

frontend-dev: ## Запустить Vite dev-сервер (порт 3000, проксирует /api → :8080)
	cd frontend && npm run dev

frontend-build: ## Собрать React SPA в web/out/ (встраивается в Go-бинарник)
	cd frontend && npm run build

frontend-typecheck: ## Проверить типы TypeScript без сборки
	cd frontend && npm run typecheck

dev: ## Запустить бэкенд и фронтенд одновременно (требует GNU make + bash)
	$(MAKE) -j2 run frontend-dev
