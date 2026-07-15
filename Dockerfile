# ── Stage 1: Build React SPA ────────────────────────────────────────────────
FROM node:22-alpine AS frontend
WORKDIR /app

# npm ci кэширует слой зависимостей отдельно от исходников.
COPY frontend/package.json frontend/package-lock.json ./frontend/
RUN cd frontend && npm ci --ignore-scripts

# Копируем исходники фронта и web/ (vite пишет outDir: '../web/out').
COPY frontend/ ./frontend/
COPY web/ ./web/

# Яндекс.Метрика: VITE_*-переменные вшиваются на этапе СБОРКИ, а не в рантайме.
# frontend/.env gitignored (нет в CI) и .dockerignore исключает .env*, поэтому ID
# счётчика прокидываем build-аргументом. Дефолт = публичный ID (он и так виден в
# JS сайта). Переопределить: build-arg VITE_YM_COUNTER_ID=... (cd.yml тянет из
# GitHub-переменной VITE_YM_COUNTER_ID, если задана).
ARG VITE_YM_COUNTER_ID=110231053
ENV VITE_YM_COUNTER_ID=$VITE_YM_COUNTER_ID

RUN cd frontend && npm run build
# Результат: /app/web/out/ — index.html + assets/

# ── Stage 2: Build Go binary (embeds web/out) ────────────────────────────────
FROM golang:1.26.5-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Перекрываем placeholder реальным SPA из предыдущего этапа.
COPY --from=frontend /app/web/out/ ./web/out/

RUN CGO_ENABLED=0 GOOS=linux go build -o /numaestra ./cmd/server

# ── Stage 3: Minimal runtime image ──────────────────────────────────────────
FROM alpine:3.19

# ca-certificates — для TLS (Suno, S3, OpenAI по HTTPS).
# ffmpeg — обработка демо-фрагмента (обрезка «сочного» участка + водяной знак).
# Если ffmpeg недоступен, демо безопасно деградирует до полного клипа (Фаза 1).
RUN apk add --no-cache ca-certificates ffmpeg \
    && addgroup -S app && adduser -S app -G app

COPY --from=builder /numaestra /numaestra

USER app

EXPOSE 8080

ENTRYPOINT ["/numaestra"]
