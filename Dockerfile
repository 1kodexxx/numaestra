# ── Stage 1: Build React SPA ────────────────────────────────────────────────
FROM node:22-alpine AS frontend
WORKDIR /app

# npm ci кэширует слой зависимостей отдельно от исходников.
COPY frontend/package.json frontend/package-lock.json ./frontend/
RUN cd frontend && npm ci --ignore-scripts

# Копируем исходники фронта и web/ (vite пишет outDir: '../web/out').
COPY frontend/ ./frontend/
COPY web/ ./web/

RUN cd frontend && npm run build
# Результат: /app/web/out/ — index.html + assets/

# ── Stage 2: Build Go binary (embeds web/out) ────────────────────────────────
FROM golang:1.26.4-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Перекрываем placeholder реальным SPA из предыдущего этапа.
COPY --from=frontend /app/web/out/ ./web/out/

RUN CGO_ENABLED=0 GOOS=linux go build -o /numaestra ./cmd/server

# ── Stage 3: Minimal runtime image ──────────────────────────────────────────
FROM alpine:3.19

# ca-certificates нужны для TLS-запросов (Suno, S3, OpenAI по HTTPS).
RUN apk add --no-cache ca-certificates \
    && addgroup -S app && adduser -S app -G app

COPY --from=builder /numaestra /numaestra

USER app

EXPOSE 8080

ENTRYPOINT ["/numaestra"]
