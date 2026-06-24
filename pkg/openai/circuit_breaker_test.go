package openai

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/numaestra/numaestra/pkg/circuitbreaker"
)

// stubClient — минимальная реализация APIClient для тестов обёртки.
type stubClient struct {
	result string
	err    error
}

func (s *stubClient) GenerateLyrics(_ context.Context, _ string) (string, error) {
	return s.result, s.err
}

func (s *stubClient) GenerateLyricsVariants(_ context.Context, _ string, count int) ([]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make([]string, count)
	for i := range out {
		out[i] = s.result
	}
	return out, nil
}

var _ APIClient = (*stubClient)(nil)

// newBreakerWith создаёт withBreaker с инжектированным stubClient и кастомным breaker.
func newBreakerWith(inner APIClient, b *circuitbreaker.Breaker) APIClient {
	return &withBreaker{inner: inner, breaker: b}
}

func TestWithBreaker_SuccessPassesThrough(t *testing.T) {
	stub := &stubClient{result: "текст песни"}
	b := circuitbreaker.New("test", 3, 0)
	client := newBreakerWith(stub, b)

	got, err := client.GenerateLyrics(context.Background(), "факты")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if got != "текст песни" {
		t.Errorf("ожидали %q, получили %q", "текст песни", got)
	}
}

func TestWithBreaker_PropagatesInnerError(t *testing.T) {
	innerErr := errors.New("LLM 503")
	stub := &stubClient{err: innerErr}
	b := circuitbreaker.New("test", 10, 0)
	client := newBreakerWith(stub, b)

	_, err := client.GenerateLyrics(context.Background(), "факты")
	if !errors.Is(err, innerErr) {
		t.Errorf("ожидали innerErr, получили: %v", err)
	}
}

func TestWithBreaker_OpensAfterThreshold(t *testing.T) {
	stub := &stubClient{err: errors.New("ошибка")}
	// Таймаут 1 час — цепь останется открытой на протяжении теста.
	b := circuitbreaker.New("test", 3, time.Hour)
	client := newBreakerWith(stub, b)

	// 3 ошибки → цепь размыкается.
	for i := 0; i < 3; i++ {
		client.GenerateLyrics(context.Background(), "x") //nolint:errcheck
	}

	// Следующий запрос должен вернуть ErrCircuitOpen, не обращаясь к stub.
	stub.err = nil
	stub.result = "успех"
	_, err := client.GenerateLyrics(context.Background(), "x")
	if !errors.Is(err, circuitbreaker.ErrCircuitOpen) {
		t.Errorf("ожидали ErrCircuitOpen, получили: %v", err)
	}
}

func TestNewClientWithBreaker_ReturnsAPIClient(t *testing.T) {
	// Smoke-test: убеждаемся, что конструктор возвращает корректный тип.
	client := NewClientWithBreaker("http://localhost", "key")
	if client == nil {
		t.Fatal("NewClientWithBreaker вернул nil")
	}
	if _, ok := client.(*withBreaker); !ok {
		t.Errorf("ожидали *withBreaker, получили %T", client)
	}
}
