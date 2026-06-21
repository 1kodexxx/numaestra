package s3

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/pkg/circuitbreaker"
)

// newResilientTestClient создаёт ResilientClient, указывающий на тестовый сервер.
func newResilientTestClient(srv *httptest.Server, threshold int, breakerTimeout time.Duration) *ResilientClient {
	return &ResilientClient{
		inner:   New(srv.URL, "us-east-1", "test-bucket", "AK", "sk"),
		breaker: circuitbreaker.New("s3-test", threshold, breakerTimeout),
	}
}

// Compile-time: ResilientClient должен реализовывать порт хранилища треков.
var _ domain.TrackStorage = (*ResilientClient)(nil)

func TestResilientClient_UploadFromURL_SuccessFirstAttempt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Write([]byte("audio")) //nolint:errcheck
		case http.MethodPut:
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	c := newResilientTestClient(srv, 3, time.Minute)
	publicURL, err := c.UploadFromURL(context.Background(), srv.URL+"/src.mp3", "tracks/1.mp3", "audio/mpeg")
	if err != nil {
		t.Fatalf("ожидали успех, получили: %v", err)
	}
	wantURL := srv.URL + "/test-bucket/tracks/1.mp3"
	if publicURL != wantURL {
		t.Errorf("publicURL = %q, ожидали %q", publicURL, wantURL)
	}
}

// TestResilientClient_UploadFromURL_RetrySucceedsOnSecond проверяет, что при первом
// сбое PUT клиент повторяет запрос и возвращает результат второй попытки.
// Тест занимает ~1s (задержка экспоненциального backoff между попытками).
func TestResilientClient_UploadFromURL_RetrySucceedsOnSecond(t *testing.T) {
	var putCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Write([]byte("audio")) //nolint:errcheck
		case http.MethodPut:
			n := atomic.AddInt32(&putCount, 1)
			if n == 1 {
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				w.WriteHeader(http.StatusOK)
			}
		}
	}))
	defer srv.Close()

	// threshold=10 — не хотим, чтобы автомат открылся во время теста.
	c := newResilientTestClient(srv, 10, time.Minute)
	_, err := c.UploadFromURL(context.Background(), srv.URL+"/src.mp3", "tracks/2.mp3", "audio/mpeg")
	if err != nil {
		t.Fatalf("ожидали успех на второй попытке, получили: %v", err)
	}
	if putCount != 2 {
		t.Errorf("ожидали 2 PUT (первый упал, второй успешен), получили %d", putCount)
	}
}

// TestResilientClient_CircuitOpensAfterThresholdFailures проверяет, что после
// threshold последовательных ошибок автомат размыкается и inner не вызывается.
// Использует отменённый контекст для ускорения каждого вызова (пропускает backoff).
func TestResilientClient_CircuitOpensAfterThresholdFailures(t *testing.T) {
	innerCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		innerCalls++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	// threshold=3, длинный timeout → автомат не закроется сам во время теста.
	c := newResilientTestClient(srv, 3, time.Minute)

	// 3 вызова с отменённым контекстом: каждый считается как одна ошибка для автомата,
	// но не ждёт задержку backoff (ctx.Done() срабатывает раньше time.After).
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		c.UploadFromURL(ctx, srv.URL+"/src.mp3", "k", "audio/mpeg") //nolint:errcheck
	}

	// Четвёртый вызов с валидным контекстом: автомат должен быть открыт.
	callsBefore := innerCalls
	_, err := c.UploadFromURL(context.Background(), srv.URL+"/src.mp3", "k", "audio/mpeg")
	if !errors.Is(err, circuitbreaker.ErrCircuitOpen) {
		t.Errorf("ожидали ErrCircuitOpen после %d ошибок, получили %v", 3, err)
	}
	if innerCalls != callsBefore {
		t.Errorf("после открытия автомата inner не должен вызываться (было %d, стало %d)", callsBefore, innerCalls)
	}
}
