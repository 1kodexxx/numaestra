// Package health реализует проверки работоспособности зависимостей сервиса.
//
// Handler возвращает JSON вида:
//
//	{"status":"ok","checks":{"postgres":"ok","redis":"ok"}}
//
// HTTP 200 — все зависимости живы.
// HTTP 503 — хотя бы одна упала; load balancer исключит инстанс из ротации.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/hibiken/asynq"
)

const probeTimeout = 2 * time.Second

// dbPinger abstracts pgxpool.Pool so the checker can be tested without a real database.
type dbPinger interface {
	Ping(ctx context.Context) error
}

// Checker хранит зависимости для проверки.
type Checker struct {
	pg    dbPinger
	redis asynq.RedisClientOpt
}

// New создаёт Checker. redis передаётся как asynq.RedisClientOpt —
// тот же объект, что используется для Asynq, без дублирования конфига.
func New(pg dbPinger, redis asynq.RedisClientOpt) *Checker {
	return &Checker{pg: pg, redis: redis}
}

type checkResult struct {
	Status string            `json:"status"` // "ok" | "degraded"
	Checks map[string]string `json:"checks"`
}

// Handler — HTTP-хендлер для эндпоинта /healthz.
// Подходит для монтирования напрямую: r.Get("/healthz", checker.Handler).
func (c *Checker) Handler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), probeTimeout)
	defer cancel()

	checks := map[string]string{
		"postgres": c.checkPostgres(ctx),
		"redis":    c.checkRedis(),
	}

	overall := "ok"
	for _, v := range checks {
		if v != "ok" {
			overall = "degraded"
			break
		}
	}

	status := http.StatusOK
	if overall != "ok" {
		status = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(checkResult{Status: overall, Checks: checks})
}

func (c *Checker) checkPostgres(ctx context.Context) string {
	if err := c.pg.Ping(ctx); err != nil {
		return "error: " + err.Error()
	}
	return "ok"
}

func (c *Checker) checkRedis() string {
	// asynq.RedisClientOpt не предоставляет метода Ping напрямую,
	// поэтому создаём краткоживущий Inspector и делаем безвредный запрос.
	inspector := asynq.NewInspector(c.redis)
	defer inspector.Close()

	// Queues() делает SMEMBERS в Redis — лёгкая операция, не меняет состояние.
	if _, err := inspector.Queues(); err != nil {
		return "error: " + err.Error()
	}
	return "ok"
}
