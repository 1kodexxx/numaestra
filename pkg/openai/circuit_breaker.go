package openai

import (
	"context"
	"time"

	"github.com/numaestra/numaestra/pkg/circuitbreaker"
)

// withBreaker оборачивает APIClient circuit breaker'ом.
// 3 последовательных ошибки → размыкание на 20 секунд.
// Пробный запрос (Half-Open) выполняется автоматически по истечении timeout.
type withBreaker struct {
	inner   APIClient
	breaker *circuitbreaker.Breaker
}

// NewClientWithBreaker создаёт APIClient с защитой circuit breaker.
func NewClientWithBreaker(baseURL, apiKey string) APIClient {
	return &withBreaker{
		inner:   NewClient(baseURL, apiKey),
		breaker: circuitbreaker.New("llm", 3, 20*time.Second),
	}
}

func (w *withBreaker) GenerateLyrics(ctx context.Context, facts string) (string, error) {
	var result string
	err := w.breaker.Do(func() error {
		var err error
		result, err = w.inner.GenerateLyrics(ctx, facts)
		return err
	})
	return result, err
}
