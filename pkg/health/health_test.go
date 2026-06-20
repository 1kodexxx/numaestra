package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
)

// mockPinger реализует dbPinger для тестов.
type mockPinger struct{ err error }

func (m mockPinger) Ping(_ context.Context) error { return m.err }

var _ dbPinger = mockPinger{}

func TestHealthHandler_AllOK(t *testing.T) {
	mini := miniredis.RunT(t)
	redisOpt := asynq.RedisClientOpt{Addr: mini.Addr()}
	checker := New(mockPinger{}, redisOpt)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	checker.Handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d (%s)", rec.Code, rec.Body.String())
	}

	var result checkResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Status != "ok" {
		t.Errorf("status: got %q, want ok", result.Status)
	}
	if result.Checks["postgres"] != "ok" {
		t.Errorf("postgres check: got %q", result.Checks["postgres"])
	}
	if result.Checks["redis"] != "ok" {
		t.Errorf("redis check: got %q", result.Checks["redis"])
	}
}

func TestHealthHandler_PostgresDown(t *testing.T) {
	mini := miniredis.RunT(t)
	redisOpt := asynq.RedisClientOpt{Addr: mini.Addr()}
	checker := New(mockPinger{err: errors.New("connection refused")}, redisOpt)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	checker.Handler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("ожидали 503, получили %d", rec.Code)
	}

	var result checkResult
	_ = json.NewDecoder(rec.Body).Decode(&result)
	if result.Status != "degraded" {
		t.Errorf("status: got %q, want degraded", result.Status)
	}
	if result.Checks["postgres"] == "ok" {
		t.Error("postgres check должен содержать ошибку")
	}
}

func TestHealthHandler_RedisDown(t *testing.T) {
	// Адрес без сервера — asynq.Inspector вернёт ошибку подключения.
	redisOpt := asynq.RedisClientOpt{Addr: "127.0.0.1:19999"}
	checker := New(mockPinger{}, redisOpt)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	checker.Handler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("ожидали 503, получили %d", rec.Code)
	}

	var result checkResult
	_ = json.NewDecoder(rec.Body).Decode(&result)
	if result.Status != "degraded" {
		t.Errorf("status: got %q, want degraded", result.Status)
	}
	if result.Checks["redis"] == "ok" {
		t.Error("redis check должен содержать ошибку")
	}
}

func TestHealthHandler_ResponseContentType(t *testing.T) {
	mini := miniredis.RunT(t)
	checker := New(mockPinger{}, asynq.RedisClientOpt{Addr: mini.Addr()})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	checker.Handler(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %q, want application/json", ct)
	}
}
