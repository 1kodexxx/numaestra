# Этап 1: сборка бинарника в полноценном Go-окружении.
# Версия согласована с go.mod (go 1.24).
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Сначала только go.mod/go.sum - Docker закэширует слой с зависимостями
# и не будет перекачивать их при каждом изменении исходного кода.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /numaestra ./cmd/server

# Этап 2: минимальный runtime-образ без компилятора и лишних слоёв
FROM alpine:3.19

# ca-certificates нужны для TLS-запросов (например, к реселлеру Suno по HTTPS)
RUN apk add --no-cache ca-certificates \
    && addgroup -S app && adduser -S app -G app

COPY --from=builder /numaestra /numaestra

USER app

EXPOSE 8080

ENTRYPOINT ["/numaestra"]